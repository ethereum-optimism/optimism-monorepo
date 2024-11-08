// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { Test } from "forge-std/Test.sol";
import { Vm } from "forge-std/Vm.sol";
import { CommonTest } from "test/setup/CommonTest.sol";

// Libraries
import { Types } from "src/libraries/Types.sol";
import { SafeCall } from "src/libraries/SafeCall.sol";
import { IL1BlockInterop } from "src/L2/interfaces/IL1BlockInterop.sol";
import { Encoding } from "src/libraries/Encoding.sol";

// Interfaces
import { ICrossDomainMessenger } from "src/universal/interfaces/ICrossDomainMessenger.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

// Free function for setting the prevBaseFee param in the OptimismPortal.
function setPrevBaseFee(Vm _vm, address _op, uint128 _prevBaseFee) {
    _vm.store(address(_op), bytes32(uint256(1)), bytes32((block.number << 192) | _prevBaseFee));
}

contract SetPrevBaseFee_Test is CommonTest {
    function test_setPrevBaseFee_succeeds() external {
        setPrevBaseFee(vm, address(optimismPortal), 100 gwei);
        (uint128 prevBaseFee,, uint64 prevBlockNum) = optimismPortal.params();
        assertEq(uint256(prevBaseFee), 100 gwei);
        assertEq(uint256(prevBlockNum), block.number);
    }
}

// Tests for obtaining pure gas cost estimates for commonly used functions.
// The objective with these benchmarks is to strip down the actual test functions
// so that they are nothing more than the call we want measure the gas cost of.
// In order to achieve this we make no assertions, and handle everything else in the setUp()
// function.
contract GasBenchMark_OptimismPortal is CommonTest {
    // Reusable default values for a test withdrawal
    Types.WithdrawalTransaction _defaultTx;

    uint256 _proposedOutputIndex;
    uint256 _proposedBlockNumber;
    bytes[] _withdrawalProof;
    Types.OutputRootProof internal _outputRootProof;
    bytes32 _outputRoot;

    // Use a constructor to set the storage vars above, so as to minimize the number of ffi calls.
    constructor() {
        super.enableLegacyContracts();
        super.setUp();
        _defaultTx = Types.WithdrawalTransaction({
            nonce: 0,
            sender: alice,
            target: bob,
            value: 100,
            gasLimit: 100_000,
            data: hex""
        });

        // Get withdrawal proof data we can use for testing.
        bytes32 _storageRoot;
        bytes32 _stateRoot;
        (_stateRoot, _storageRoot, _outputRoot,, _withdrawalProof) = ffi.getProveWithdrawalTransactionInputs(_defaultTx);

        // Setup a dummy output root proof for reuse.
        _outputRootProof = Types.OutputRootProof({
            version: bytes32(uint256(0)),
            stateRoot: _stateRoot,
            messagePasserStorageRoot: _storageRoot,
            latestBlockhash: bytes32(uint256(0))
        });
        _proposedBlockNumber = l2OutputOracle.nextBlockNumber();
        _proposedOutputIndex = l2OutputOracle.nextOutputIndex();
    }

    // Get the system into a nice ready-to-use state.
    function setUp() public virtual override {
        // Configure the oracle to return the output root we've prepared.
        vm.warp(l2OutputOracle.computeL2Timestamp(_proposedBlockNumber) + 1);
        vm.prank(l2OutputOracle.PROPOSER());
        l2OutputOracle.proposeL2Output(_outputRoot, _proposedBlockNumber, 0, 0);

        // Warp beyond the finalization period for the block we've proposed.
        vm.warp(
            l2OutputOracle.getL2Output(_proposedOutputIndex).timestamp + l2OutputOracle.FINALIZATION_PERIOD_SECONDS()
                + 1
        );

        // Fund the portal so that we can withdraw ETH.
        vm.deal(address(optimismPortal), 0xFFFFFFFF);
    }

    function test_depositTransaction_benchmark() external {
        optimismPortal.depositTransaction{ value: 100 }(
            address(1), 0, 50000, false, hex"0000111122223333444455556666777788889999aaaabbbbccccddddeeeeffff0000"
        );
    }

    function test_depositTransaction_benchmark_1() external {
        setPrevBaseFee(vm, address(optimismPortal), 1 gwei);
        optimismPortal.depositTransaction{ value: 100 }(
            address(1), 0, 50000, false, hex"0000111122223333444455556666777788889999aaaabbbbccccddddeeeeffff0000"
        );
    }

    function test_proveWithdrawalTransaction_benchmark() external {
        optimismPortal.proveWithdrawalTransaction(_defaultTx, _proposedOutputIndex, _outputRootProof, _withdrawalProof);
    }
}

contract GasBenchMark_L1CrossDomainMessenger is CommonTest {
    function test_sendMessage_benchmark_0() external {
        vm.pauseGasMetering();
        setPrevBaseFee(vm, address(optimismPortal), 1 gwei);
        // The amount of data typically sent during a bridge deposit.
        bytes memory data =
            hex"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff";
        vm.resumeGasMetering();
        l1CrossDomainMessenger.sendMessage(bob, data, uint32(100));
    }

    function test_sendMessage_benchmark_1() external {
        vm.pauseGasMetering();
        setPrevBaseFee(vm, address(optimismPortal), 10 gwei);
        // The amount of data typically sent during a bridge deposit.
        bytes memory data =
            hex"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff";
        vm.resumeGasMetering();
        l1CrossDomainMessenger.sendMessage(bob, data, uint32(100));
    }
}

contract GasBenchMark_L1StandardBridge_Deposit is CommonTest {
    function setUp() public virtual override {
        super.setUp();
        deal(address(L1Token), alice, 100000, true);
        vm.startPrank(alice, alice);
        L1Token.approve(address(l1StandardBridge), type(uint256).max);
    }

    function test_depositETH_benchmark_0() external {
        vm.pauseGasMetering();
        setPrevBaseFee(vm, address(optimismPortal), 1 gwei);
        vm.resumeGasMetering();
        l1StandardBridge.depositETH{ value: 500 }(50000, hex"");
    }

    function test_depositETH_benchmark_1() external {
        vm.pauseGasMetering();
        setPrevBaseFee(vm, address(optimismPortal), 10 gwei);
        vm.resumeGasMetering();
        l1StandardBridge.depositETH{ value: 500 }(50000, hex"");
    }

    function test_depositERC20_benchmark_0() external {
        vm.pauseGasMetering();
        setPrevBaseFee(vm, address(optimismPortal), 1 gwei);
        vm.resumeGasMetering();
        l1StandardBridge.bridgeERC20({
            _localToken: address(L1Token),
            _remoteToken: address(L2Token),
            _amount: 100,
            _minGasLimit: 100_000,
            _extraData: hex""
        });
    }

    function test_depositERC20_benchmark_1() external {
        vm.pauseGasMetering();
        setPrevBaseFee(vm, address(optimismPortal), 10 gwei);
        vm.resumeGasMetering();
        l1StandardBridge.bridgeERC20({
            _localToken: address(L1Token),
            _remoteToken: address(L2Token),
            _amount: 100,
            _minGasLimit: 100_000,
            _extraData: hex""
        });
    }
}

contract GasBenchMark_L1StandardBridge_Finalize is CommonTest {
    function setUp() public virtual override {
        super.setUp();
        deal(address(L1Token), address(l1StandardBridge), 100, true);
        vm.mockCall(
            address(l1StandardBridge.messenger()),
            abi.encodeCall(ICrossDomainMessenger.xDomainMessageSender, ()),
            abi.encode(address(l1StandardBridge.OTHER_BRIDGE()))
        );
        vm.startPrank(address(l1StandardBridge.messenger()));
        vm.deal(address(l1StandardBridge.messenger()), 100);
    }

    function test_finalizeETHWithdrawal_benchmark() external {
        // TODO: Make this more accurate. It is underestimating the cost because it pranks
        // the call coming from the messenger, which bypasses the portal
        // and oracle.
        l1StandardBridge.finalizeETHWithdrawal{ value: 100 }(alice, alice, 100, hex"");
    }
}

contract GasBenchMark_L2OutputOracle is CommonTest {
    uint256 nextBlockNumber;

    function setUp() public override {
        super.enableLegacyContracts();
        super.setUp();
        nextBlockNumber = l2OutputOracle.nextBlockNumber();
        warpToProposeTime(nextBlockNumber);
        address proposer = deploy.cfg().l2OutputOracleProposer();
        vm.startPrank(proposer);
    }

    function test_proposeL2Output_benchmark() external {
        l2OutputOracle.proposeL2Output(nonZeroHash, nextBlockNumber, 0, 0);
    }
}

contract GasBenchMark_L1Block is CommonTest {
    address depositor;
    bytes setValuesCalldata;

    function setUp() public virtual override {
        super.setUp();
        depositor = l1Block.DEPOSITOR_ACCOUNT();
        setValuesCalldata = Encoding.encodeSetL1BlockValuesEcotone(
            type(uint32).max,
            type(uint32).max,
            type(uint64).max,
            type(uint64).max,
            type(uint64).max,
            type(uint256).max,
            type(uint256).max,
            keccak256(abi.encode(1)),
            bytes32(type(uint256).max)
        );
        vm.startPrank(depositor);
    }
}

contract GasBenchMark_L1Block_SetValuesEcotone is GasBenchMark_L1Block {
    function test_setL1BlockValuesEcotone_benchmark() external {
        SafeCall.call({ _target: address(l1Block), _calldata: setValuesCalldata });
    }
}

contract GasBenchMark_L1Block_SetValuesEcotone_Warm is GasBenchMark_L1Block {
    function setUp() public virtual override {
        SafeCall.call({ _target: address(l1Block), _calldata: setValuesCalldata });
    }

    function test_setL1BlockValuesEcotone_benchmark() external {
        SafeCall.call({ _target: address(l1Block), _calldata: setValuesCalldata });
    }
}

contract GasBenchMark_L1BlockInterop is GasBenchMark_L1Block {
    IL1BlockInterop l1BlockInterop;

    function setUp() public virtual override {
        super.setUp();
        l1BlockInterop = IL1BlockInterop(
            DeployUtils.create1({
                _name: "L1BlockInterop",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IL1BlockInterop.__constructor__, ()))
            })
        );
        setValuesCalldata = Encoding.encodeSetL1BlockValuesInterop(
            type(uint32).max,
            type(uint32).max,
            type(uint64).max,
            type(uint64).max,
            type(uint64).max,
            type(uint256).max,
            type(uint256).max,
            keccak256(abi.encode(1)),
            bytes32(type(uint256).max)
        );
    }
}

contract GasBenchMark_L1BlockInterop_SetValuesInterop is GasBenchMark_L1BlockInterop {
    function test_setL1BlockValuesInterop_benchmark() external {
        SafeCall.call({ _target: address(l1BlockInterop), _calldata: setValuesCalldata });
    }
}

contract GasBenchMark_L1BlockInterop_SetValuesInterop_Warm is GasBenchMark_L1BlockInterop {
    function setUp() public virtual override {
        SafeCall.call({ _target: address(l1BlockInterop), _calldata: setValuesCalldata });
    }

    function test_setL1BlockValuesInterop_benchmark() external {
        SafeCall.call({ _target: address(l1BlockInterop), _calldata: setValuesCalldata });
    }
}

contract GasBenchMark_L1BlockInterop_DepositsComplete is GasBenchMark_L1BlockInterop {
    function test_depositsComplete_benchmark() external {
        SafeCall.call({
            _target: address(l1BlockInterop),
            _calldata: abi.encodeCall(IL1BlockInterop.depositsComplete, ())
        });
    }
}

contract GasBenchMark_L1BlockInterop_DepositsComplete_Warm is GasBenchMark_L1BlockInterop {
    function setUp() public virtual override {
        super.setUp();
        // Set the isDeposit flag to true so then we can benchmark when it is reset.
        SafeCall.call({ _target: address(l1BlockInterop), _calldata: setValuesCalldata });
    }

    function test_depositsComplete_benchmark() external {
        SafeCall.call({
            _target: address(l1BlockInterop),
            _calldata: abi.encodeCall(l1BlockInterop.depositsComplete, ())
        });
    }
}
