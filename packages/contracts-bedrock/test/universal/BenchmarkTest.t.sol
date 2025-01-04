// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { Vm } from "forge-std/Vm.sol";
import { CommonTest } from "test/setup/CommonTest.sol";

// Libraries
import { SafeCall } from "src/libraries/SafeCall.sol";
import { IL1BlockInterop } from "interfaces/L2/IL1BlockInterop.sol";
import { Encoding } from "src/libraries/Encoding.sol";

// Interfaces
import { ICrossDomainMessenger } from "interfaces/universal/ICrossDomainMessenger.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

// Free function for setting the prevBaseFee param in the OptimismPortal.
function setPrevBaseFee(Vm _vm, address _op, uint128 _prevBaseFee) {
    _vm.store(address(_op), bytes32(uint256(1)), bytes32((block.number << 192) | _prevBaseFee));
}

contract SetPrevBaseFee_Test is CommonTest {
    function test_setPrevBaseFee_succeeds() external {
        setPrevBaseFee(vm, address(optimismPortal2), 100 gwei);
        (uint128 prevBaseFee,, uint64 prevBlockNum) = optimismPortal2.params();
        assertEq(uint256(prevBaseFee), 100 gwei);
        assertEq(uint256(prevBlockNum), block.number);
    }
}

contract GasBenchMark_L1CrossDomainMessenger is CommonTest {
    /// @notice Benchmark sending a message with typical bridge deposit data at 1 gwei base fee
    /// @dev This test measures gas consumption for a typical cross-domain message
    function test_sendMessage_benchmark_0() external {
        vm.pauseGasMetering();
        setPrevBaseFee(vm, address(optimismPortal2), 1 gwei);
        // The amount of data typically sent during a bridge deposit.
        bytes memory data =
            hex"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff";
        vm.resumeGasMetering();
        l1CrossDomainMessenger.sendMessage(bob, data, uint32(100));
    }

    /// @notice Benchmark sending a message with typical bridge deposit data at 10 gwei base fee
    /// @dev This test measures gas consumption under higher network load conditions
    function test_sendMessage_benchmark_1() external {
        vm.pauseGasMetering();
        setPrevBaseFee(vm, address(optimismPortal2), 10 gwei);
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

    /// @notice Benchmark ETH deposit at 1 gwei base fee
    /// @dev Measures complete deposit flow including portal interaction
    function test_depositETH_benchmark_0() external {
        vm.pauseGasMetering();
        setPrevBaseFee(vm, address(optimismPortal2), 1 gwei);
        vm.resumeGasMetering();
        l1StandardBridge.depositETH{ value: 500 }(50000, hex"");
    }

    /// @notice Benchmark ETH deposit at 10 gwei base fee
    /// @dev Measures deposit performance under network congestion
    function test_depositETH_benchmark_1() external {
        vm.pauseGasMetering();
        setPrevBaseFee(vm, address(optimismPortal2), 10 gwei);
        vm.resumeGasMetering();
        l1StandardBridge.depositETH{ value: 500 }(50000, hex"");
    }

    /// @notice Benchmark ERC20 deposit at 1 gwei base fee
    /// @dev Complete bridge operation including token transfer and message passing
    function test_depositERC20_benchmark_0() external {
        vm.pauseGasMetering();
        setPrevBaseFee(vm, address(optimismPortal2), 1 gwei);
        vm.resumeGasMetering();
        l1StandardBridge.bridgeERC20({
            _localToken: address(L1Token),
            _remoteToken: address(L2Token),
            _amount: 100,
            _minGasLimit: 100_000,
            _extraData: hex""
        });
    }

    /// @notice Benchmark ERC20 deposit at 10 gwei base fee
    /// @dev Measures bridge performance under network congestion
    function test_depositERC20_benchmark_1() external {
        vm.pauseGasMetering();
        setPrevBaseFee(vm, address(optimismPortal2), 10 gwei);
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
        // Setup initial state
        deal(address(L1Token), address(l1StandardBridge), 100, true);

        // Setup the L2 messenger state
        vm.mockCall(
            address(l1StandardBridge.messenger()),
            abi.encodeCall(ICrossDomainMessenger.xDomainMessageSender, ()),
            abi.encode(address(l1StandardBridge.OTHER_BRIDGE()))
        );

        // Setup the portal state to simulate full withdrawal path
        bytes32 withdrawalHash = keccak256(abi.encodePacked("withdrawal"));
        vm.store(
            address(optimismPortal2),
            keccak256(abi.encodePacked(withdrawalHash, uint256(0))),
            bytes32(uint256(1))
        );

        vm.startPrank(address(l1StandardBridge.messenger()));
        vm.deal(address(l1StandardBridge.messenger()), 100);
    }

    /// @notice Benchmark ETH withdrawal finalization with complete portal path
    /// @dev Measures the full cost including portal and oracle interaction
    function test_finalizeETHWithdrawal_benchmark() external {
        // Record initial gas for more accurate measurement
        uint256 startGas = gasleft();

        // Execute the withdrawal through the complete path
        l1StandardBridge.finalizeETHWithdrawal{ value: 100 }(alice, alice, 100, hex"");

        // Calculate and log the actual gas used
        uint256 gasUsed = startGas - gasleft();
        console.log("Actual gas used for ETH withdrawal: ", gasUsed);
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
