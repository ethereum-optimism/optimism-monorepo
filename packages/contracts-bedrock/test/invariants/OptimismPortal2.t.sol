// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { StdUtils } from "forge-std/Test.sol";
import { Vm } from "forge-std/Vm.sol";
import { CommonTest } from "test/setup/CommonTest.sol";
import { EIP1967Helper } from "test/mocks/EIP1967Helper.sol";

// Contracts
import { ResourceMetering } from "src/L1/ResourceMetering.sol";

// Libraries
import { Constants } from "src/libraries/Constants.sol";
import { Types } from "src/libraries/Types.sol";
import "src/dispute/lib/Types.sol";
import "src/libraries/PortalErrors.sol";

// Interfaces
import { IOptimismPortal2 } from "src/L1/interfaces/IOptimismPortal2.sol";
import { IResourceMetering } from "src/L1/interfaces/IResourceMetering.sol";
import { IFaultDisputeGame } from "src/dispute/interfaces/IFaultDisputeGame.sol";

contract OptimismPortal2_Depositor is StdUtils, ResourceMetering {
    Vm internal vm;
    IOptimismPortal2 internal portal;
    bool public failedToComplete;

    constructor(Vm _vm, IOptimismPortal2 _portal) {
        vm = _vm;
        portal = _portal;
        initialize();
    }

    function initialize() internal initializer {
        __ResourceMetering_init();
    }

    function resourceConfig() public pure returns (ResourceMetering.ResourceConfig memory) {
        return _resourceConfig();
    }

    function _resourceConfig() internal pure override returns (ResourceMetering.ResourceConfig memory config_) {
        IResourceMetering.ResourceConfig memory rcfg = Constants.DEFAULT_RESOURCE_CONFIG();
        assembly ("memory-safe") {
            config_ := rcfg
        }
    }

    // A test intended to identify any unexpected halting conditions
    function depositTransactionCompletes(
        address _to,
        uint256 _value,
        uint64 _gasLimit,
        bool _isCreation,
        bytes memory _data
    )
        public
        payable
    {
        vm.assume((!_isCreation || _to == address(0)) && _data.length <= 120_000);

        uint256 preDepositvalue = bound(_value, 0, type(uint128).max);
        // Give the depositor some ether
        vm.deal(address(this), preDepositvalue);
        // cache the contract's eth balance
        uint256 preDepositBalance = address(this).balance;
        uint256 value = bound(preDepositvalue, 0, preDepositBalance);

        (, uint64 cachedPrevBoughtGas,) = ResourceMetering(address(portal)).params();
        ResourceMetering.ResourceConfig memory rcfg = resourceConfig();
        uint256 maxResourceLimit = uint64(rcfg.maxResourceLimit);
        uint64 gasLimit = uint64(
            bound(_gasLimit, portal.minimumGasLimit(uint64(_data.length)), maxResourceLimit - cachedPrevBoughtGas)
        );

        try portal.depositTransaction{ value: value }(_to, value, gasLimit, _isCreation, _data) {
            // Do nothing; Call succeeded
        } catch {
            failedToComplete = true;
        }
    }
}

contract OptimismPortal2_Invariant_Harness is CommonTest {
    // Reusable default values for a test withdrawal
    Types.WithdrawalTransaction _defaultTx;

    uint256 _proposedGameIndex;
    uint256 _proposedBlockNumber;
    bytes32 _stateRoot;
    bytes32 _storageRoot;
    bytes32 _outputRoot;
    bytes32 _withdrawalHash;
    bytes[] _withdrawalProof;
    Types.OutputRootProof internal _outputRootProof;

    function setUp() public virtual override {
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
        (_stateRoot, _storageRoot, _outputRoot, _withdrawalHash, _withdrawalProof) =
            ffi.getProveWithdrawalTransactionInputs(_defaultTx);

        // Setup a dummy output root proof for reuse.
        _outputRootProof = Types.OutputRootProof({
            version: bytes32(uint256(0)),
            stateRoot: _stateRoot,
            messagePasserStorageRoot: _storageRoot,
            latestBlockhash: bytes32(uint256(0))
        });

        // Create a dispute game with the output root we've proposed.
        _proposedBlockNumber = 0xFF;
        IFaultDisputeGame game = IFaultDisputeGame(
            payable(
                address(
                    disputeGameFactory.create(
                        optimismPortal2.respectedGameType(), Claim.wrap(_outputRoot), abi.encode(_proposedBlockNumber)
                    )
                )
            )
        );
        _proposedGameIndex = disputeGameFactory.gameCount() - 1;

        // Warp beyond the finalization period for the dispute game and resolve it.
        vm.warp(block.timestamp + (game.maxClockDuration().raw() * 2) + 1 seconds);
        game.resolveClaim(0, 0);
        game.resolve();

        // Fund the portal so that we can withdraw ETH.
        vm.deal(address(optimismPortal2), 0xFFFFFFFF);
    }
}

contract OptimismPortal2_Deposit_Invariant is CommonTest {
    OptimismPortal2_Depositor internal actor;

    function setUp() public override {
        super.setUp();
        // Create a deposit actor.
        actor = new OptimismPortal2_Depositor(vm, optimismPortal2);

        targetContract(address(actor));

        bytes4[] memory selectors = new bytes4[](1);
        selectors[0] = actor.depositTransactionCompletes.selector;
        FuzzSelector memory selector = FuzzSelector({ addr: address(actor), selectors: selectors });
        targetSelector(selector);
    }

    /// @custom:invariant Deposits of any value should always succeed unless
    ///                   `_to` = `address(0)` or `_isCreation` = `true`.
    ///
    ///                   All deposits, barring creation transactions and transactions
    ///                   sent to `address(0)`, should always succeed.
    function invariant_deposit_completes() external view {
        assertEq(actor.failedToComplete(), false);
    }
}

contract OptimismPortal2_CannotTimeTravel is OptimismPortal2_Invariant_Harness {
    function setUp() public override {
        super.setUp();

        // Prove the withdrawal transaction
        optimismPortal2.proveWithdrawalTransaction(_defaultTx, _proposedGameIndex, _outputRootProof, _withdrawalProof);

        // Set the target contract to the portal proxy
        targetContract(address(optimismPortal2));
        // Exclude the proxy admin from the senders so that the proxy cannot be upgraded
        excludeSender(EIP1967Helper.getAdmin(address(optimismPortal2)));
    }

    /// @custom:invariant `finalizeWithdrawalTransaction` should revert if the proof maturity period has not elapsed.
    ///
    ///                   A withdrawal that has been proven should not be able to be finalized
    ///                   until after the proof maturity period has elapsed.
    function invariant_cannotFinalizeBeforePeriodHasPassed() external {
        vm.expectRevert("OptimismPortal: proven withdrawal has not matured yet");
        optimismPortal2.finalizeWithdrawalTransaction(_defaultTx);
    }
}

contract OptimismPortal2_CannotFinalizeTwice is OptimismPortal2_Invariant_Harness {
    function setUp() public override {
        super.setUp();

        // Prove the withdrawal transaction
        optimismPortal2.proveWithdrawalTransaction(_defaultTx, _proposedGameIndex, _outputRootProof, _withdrawalProof);

        // Warp past the proof maturity period.
        vm.warp(block.timestamp + optimismPortal2.proofMaturityDelaySeconds() + 1);

        // Finalize the withdrawal transaction.
        optimismPortal2.finalizeWithdrawalTransaction(_defaultTx);

        // Set the target contract to the portal proxy
        targetContract(address(optimismPortal2));
        // Exclude the proxy admin from the senders so that the proxy cannot be upgraded
        excludeSender(EIP1967Helper.getAdmin(address(optimismPortal2)));
    }

    /// @custom:invariant `finalizeWithdrawalTransaction` should revert if the withdrawal has already been finalized.
    ///
    ///                   Ensures that there is no chain of calls that can be made that allows a withdrawal to be
    ///                   finalized twice.
    function invariant_cannotFinalizeTwice() external {
        vm.expectRevert(AlreadyFinalized.selector);
        optimismPortal2.finalizeWithdrawalTransaction(_defaultTx);
    }
}

contract OptimismPortal_CanAlwaysFinalizeAfterWindow is OptimismPortal2_Invariant_Harness {
    function setUp() public override {
        super.setUp();

        // Prove the withdrawal transaction
        optimismPortal2.proveWithdrawalTransaction(_defaultTx, _proposedGameIndex, _outputRootProof, _withdrawalProof);

        // Warp past the proof maturity period.
        vm.warp(block.timestamp + optimismPortal2.proofMaturityDelaySeconds() + 1);

        // Set the target contract to the portal proxy
        targetContract(address(optimismPortal2));
        // Exclude the proxy admin from the senders so that the proxy cannot be upgraded
        excludeSender(EIP1967Helper.getAdmin(address(optimismPortal2)));
    }

    /// @custom:invariant A withdrawal should **always** be able to be finalized `PROOF_MATURITY_DELAY_SECONDS` after
    ///                   it was successfully proven, if the game has resolved and passed the air-gap.
    ///
    ///                   This invariant asserts that there is no chain of calls that can be made that will prevent a
    ///                   withdrawal from being finalized exactly `PROOF_MATURITY_DELAY_SECONDS` after it was
    ///                   successfully proven and the game has resolved and passed the air-gap.
    function invariant_canAlwaysFinalize() external {
        uint256 bobBalanceBefore = address(bob).balance;

        optimismPortal2.finalizeWithdrawalTransaction(_defaultTx);

        assertEq(address(bob).balance, bobBalanceBefore + _defaultTx.value);
    }
}
