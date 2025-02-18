// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { CommonTest } from "test/setup/CommonTest.sol";

// Libraries
import { Constants } from "src/libraries/Constants.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";
import { Types } from "src/libraries/Types.sol";
import { OutputRootWithChainId, SuperRootProof, SUPER_ROOT_VERSION } from "src/libraries/SuperRoot.sol";
import "src/dispute/lib/Types.sol";
import "src/libraries/PortalErrors.sol";

// Interfaces
import { IL1BlockInterop, ConfigType } from "interfaces/L2/IL1BlockInterop.sol";
import { IOptimismPortalInterop } from "interfaces/L1/IOptimismPortalInterop.sol";

contract OptimismPortalInterop_Test is CommonTest {
    /// @notice Marked virtual to be overridden in
    ///         test/kontrol/deployment/DeploymentSummary.t.sol
    function setUp() public virtual override {
        super.enableInterop();
        super.setUp();
    }

    /// @notice Tests that the version function returns a valid string. We avoid testing the
    ///         specific value of the string as it changes frequently.
    function test_version_succeeds() external view {
        assert(bytes(_optimismPortalInterop().version()).length > 0);
    }

    /// @dev Tests that the config for adding a dependency can be set.
    function testFuzz_setConfig_addDependency_succeeds(bytes calldata _value) public {
        vm.expectEmit(address(optimismPortal2));
        emitTransactionDeposited({
            _from: Constants.DEPOSITOR_ACCOUNT,
            _to: Predeploys.L1_BLOCK_ATTRIBUTES,
            _value: 0,
            _mint: 0,
            _gasLimit: 200_000,
            _isCreation: false,
            _data: abi.encodeCall(IL1BlockInterop.setConfig, (ConfigType.ADD_DEPENDENCY, _value))
        });

        vm.prank(address(_optimismPortalInterop().systemConfig()));
        _optimismPortalInterop().setConfig(ConfigType.ADD_DEPENDENCY, _value);
    }

    /// @dev Tests that setting the add dependency config as not the system config reverts.
    function testFuzz_setConfig_addDependencyButNotSystemConfig_reverts(bytes calldata _value) public {
        vm.expectRevert(Unauthorized.selector);
        _optimismPortalInterop().setConfig(ConfigType.ADD_DEPENDENCY, _value);
    }

    /// @dev Tests that the config for removing a dependency can be set.
    function testFuzz_setConfig_removeDependency_succeeds(bytes calldata _value) public {
        vm.expectEmit(address(optimismPortal2));
        emitTransactionDeposited({
            _from: Constants.DEPOSITOR_ACCOUNT,
            _to: Predeploys.L1_BLOCK_ATTRIBUTES,
            _value: 0,
            _mint: 0,
            _gasLimit: 200_000,
            _isCreation: false,
            _data: abi.encodeCall(IL1BlockInterop.setConfig, (ConfigType.REMOVE_DEPENDENCY, _value))
        });

        vm.prank(address(_optimismPortalInterop().systemConfig()));
        _optimismPortalInterop().setConfig(ConfigType.REMOVE_DEPENDENCY, _value);
    }

    /// @dev Tests that setting the remove dependency config as not the system config reverts.
    function testFuzz_setConfig_removeDependencyButNotSystemConfig_reverts(bytes calldata _value) public {
        vm.expectRevert(Unauthorized.selector);
        _optimismPortalInterop().setConfig(ConfigType.REMOVE_DEPENDENCY, _value);
    }

    /// @dev Tests that `proveWithdrawalTransaction` succeeds.
    function test_proveWithdrawalTransaction_validSuperWithdrawalProof_succeeds() external {
        vm.warp(block.timestamp + 1);

        // Craft test withdrawal transaction.
        Types.WithdrawalTransaction memory defaultTx = Types.WithdrawalTransaction({
            nonce: 0,
            sender: alice,
            target: bob,
            value: 100,
            gasLimit: 100_000,
            data: hex"aa" // includes calldata for ERC20 withdrawal test
         });
        (
            bytes32 stateRoot,
            bytes32 storageRoot,
            bytes32 outputRoot,
            bytes32 withdrawalHash,
            bytes[] memory withdrawalProof
        ) = ffi.getProveWithdrawalTransactionInputs(defaultTx);

        // Construct the output root proof
        Types.OutputRootProof memory _outputRootProof = Types.OutputRootProof({
            version: bytes32(uint256(0)),
            stateRoot: stateRoot,
            messagePasserStorageRoot: storageRoot,
            latestBlockhash: bytes32(uint256(0))
        });

        // Construct the super root, comprising of only one chain.
        OutputRootWithChainId memory outputRootWithChain =
            OutputRootWithChainId({ l2ChainId: 10, outputRoot: outputRoot });
        bytes memory rawSuperRoot = abi.encodePacked(
            SUPER_ROOT_VERSION, uint64(block.timestamp), outputRootWithChain.l2ChainId, outputRootWithChain.outputRoot
        );

        // Set up the dummy game, proposing the super root.
        GameType respectedGameType = _optimismPortalInterop().respectedGameType();
        disputeGameFactory.create{ value: disputeGameFactory.initBonds(respectedGameType) }(
            respectedGameType, Claim.wrap(keccak256(rawSuperRoot)), abi.encode(block.timestamp)
        );

        vm.expectEmit(true, true, true, true);
        emit WithdrawalProven(withdrawalHash, alice, bob);
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProvenExtension1(withdrawalHash, address(this));
        _optimismPortalInterop().proveWithdrawalTransaction({
            _tx: defaultTx,
            _disputeGameIndex: disputeGameFactory.gameCount() - 1,
            _superRootProof: SuperRootProof({ rawSuperRoot: rawSuperRoot, l2ChainId: 10, index: 0 }),
            _outputRootProof: _outputRootProof,
            _withdrawalProof: withdrawalProof
        });
    }

    /// @dev Returns the OptimismPortalInterop instance.
    function _optimismPortalInterop() internal view returns (IOptimismPortalInterop) {
        return IOptimismPortalInterop(payable(address(optimismPortal2)));
    }
}
