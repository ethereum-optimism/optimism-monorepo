// SPDX-License-Identifier: MIT
pragma solidity ^0.8.13;

import { DeploymentSummaryFaultProofs } from "./utils/DeploymentSummaryFaultProofs.sol";
import { KontrolUtils } from "./utils/KontrolUtils.sol";
import { Types } from "src/libraries/Types.sol";
import { IOptimismPortal2 as OptimismPortal } from "interfaces/L1/IOptimismPortal2.sol";
import { ISuperchainConfig as SuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import "src/libraries/PortalErrors.sol";

contract OptimismPortal2Kontrol is DeploymentSummaryFaultProofs, KontrolUtils {
    OptimismPortal optimismPortal;
    SuperchainConfig superchainConfig;

    /// @dev Inlined setUp function for faster Kontrol performance
    ///      Tracking issue: https://github.com/runtimeverification/kontrol/issues/282
    function setUpInlined() public {
        optimismPortal = OptimismPortal(payable(optimismPortalProxyAddress));
        superchainConfig = SuperchainConfig(superchainConfigProxyAddress);
    }

    function prove_finalizeWithdrawalTransaction_paused(Types.WithdrawalTransaction calldata _tx) external {
        setUpInlined();

        // Pause Optimism Portal
        vm.prank(optimismPortal.guardian());
        superchainConfig.pause("identifier");

        vm.expectRevert(CallPaused.selector);
        optimismPortal.finalizeWithdrawalTransaction(_tx);
    }

    /// @dev Function containing the logic for prove_proveWithdrawalTransaction_paused
    ///      The reason for this is that we want the _withdrawalProof to range in size from
    ///      0 to 10. These 11 proofs will exercise the same logic, which is contained in this function
    function prove_proveWithdrawalTransaction_paused_internal(
        Types.WithdrawalTransaction memory _tx,
        uint256 _l2OutputIndex,
        Types.OutputRootProof calldata _outputRootProof,
        bytes[] memory _withdrawalProof
    )
        internal
    {
        setUpInlined();

        // Pause Optimism Portal
        vm.prank(optimismPortal.guardian());
        superchainConfig.pause("identifier");

        vm.expectRevert(CallPaused.selector);
        optimismPortal.proveWithdrawalTransaction(_tx, _l2OutputIndex, _outputRootProof, _withdrawalProof);
    }

    /// @custom:kontrol-array-length-equals _withdrawalProof: 10,
    /// @custom:kontrol-bytes-length-equals _withdrawalProof: 600,
    function prove_proveWithdrawalTransaction_paused10(
        Types.WithdrawalTransaction memory _tx,
        uint256 _l2OutputIndex,
        Types.OutputRootProof calldata _outputRootProof,
        bytes[] calldata _withdrawalProof
    )
        external
    {
        prove_proveWithdrawalTransaction_paused_internal(_tx, _l2OutputIndex, _outputRootProof, _withdrawalProof);
    }

    /// @custom:kontrol-array-length-equals _withdrawalProof: 9,
    /// @custom:kontrol-bytes-length-equals _withdrawalProof: 600,
    function prove_proveWithdrawalTransaction_paused9(
        Types.WithdrawalTransaction memory _tx,
        uint256 _l2OutputIndex,
        Types.OutputRootProof calldata _outputRootProof,
        bytes[] calldata _withdrawalProof
    )
        external
    {
        prove_proveWithdrawalTransaction_paused_internal(_tx, _l2OutputIndex, _outputRootProof, _withdrawalProof);
    }

    /// @custom:kontrol-array-length-equals _withdrawalProof: 8,
    /// @custom:kontrol-bytes-length-equals _withdrawalProof: 600,
    function prove_proveWithdrawalTransaction_paused8(
        Types.WithdrawalTransaction memory _tx,
        uint256 _l2OutputIndex,
        Types.OutputRootProof calldata _outputRootProof,
        bytes[] calldata _withdrawalProof
    )
        external
    {
        prove_proveWithdrawalTransaction_paused_internal(_tx, _l2OutputIndex, _outputRootProof, _withdrawalProof);
    }

    /// @custom:kontrol-array-length-equals _withdrawalProof: 7,
    /// @custom:kontrol-bytes-length-equals _withdrawalProof: 600,
    function prove_proveWithdrawalTransaction_paused7(
        Types.WithdrawalTransaction memory _tx,
        uint256 _l2OutputIndex,
        Types.OutputRootProof calldata _outputRootProof,
        bytes[] calldata _withdrawalProof
    )
        external
    {
        prove_proveWithdrawalTransaction_paused_internal(_tx, _l2OutputIndex, _outputRootProof, _withdrawalProof);
    }

    /// @custom:kontrol-array-length-equals _withdrawalProof: 6,
    /// @custom:kontrol-bytes-length-equals _withdrawalProof: 600,
    function prove_proveWithdrawalTransaction_paused6(
        Types.WithdrawalTransaction memory _tx,
        uint256 _l2OutputIndex,
        Types.OutputRootProof calldata _outputRootProof,
        bytes[] calldata _withdrawalProof
    )
        external
    {
        prove_proveWithdrawalTransaction_paused_internal(_tx, _l2OutputIndex, _outputRootProof, _withdrawalProof);
    }

    /// @custom:kontrol-array-length-equals _withdrawalProof: 5,
    /// @custom:kontrol-bytes-length-equals _withdrawalProof: 600,
    function prove_proveWithdrawalTransaction_paused5(
        Types.WithdrawalTransaction memory _tx,
        uint256 _l2OutputIndex,
        Types.OutputRootProof calldata _outputRootProof,
        bytes[] calldata _withdrawalProof
    )
        external
    {
        prove_proveWithdrawalTransaction_paused_internal(_tx, _l2OutputIndex, _outputRootProof, _withdrawalProof);
    }

    /// @custom:kontrol-array-length-equals _withdrawalProof: 4,
    /// @custom:kontrol-bytes-length-equals _withdrawalProof: 600,
    function prove_proveWithdrawalTransaction_paused4(
        Types.WithdrawalTransaction memory _tx,
        uint256 _l2OutputIndex,
        Types.OutputRootProof calldata _outputRootProof,
        bytes[] calldata _withdrawalProof
    )
        external
    {
        prove_proveWithdrawalTransaction_paused_internal(_tx, _l2OutputIndex, _outputRootProof, _withdrawalProof);
    }

    /// @custom:kontrol-array-length-equals _withdrawalProof: 3,
    /// @custom:kontrol-bytes-length-equals _withdrawalProof: 600,
    function prove_proveWithdrawalTransaction_paused3(
        Types.WithdrawalTransaction memory _tx,
        uint256 _l2OutputIndex,
        Types.OutputRootProof calldata _outputRootProof,
        bytes[] calldata _withdrawalProof
    )
        external
    {
        prove_proveWithdrawalTransaction_paused_internal(_tx, _l2OutputIndex, _outputRootProof, _withdrawalProof);
    }

    /// @custom:kontrol-array-length-equals _withdrawalProof: 2,
    /// @custom:kontrol-bytes-length-equals _withdrawalProof: 600,
    function prove_proveWithdrawalTransaction_paused2(
        Types.WithdrawalTransaction memory _tx,
        uint256 _l2OutputIndex,
        Types.OutputRootProof calldata _outputRootProof,
        bytes[] calldata _withdrawalProof
    )
        external
    {
        prove_proveWithdrawalTransaction_paused_internal(_tx, _l2OutputIndex, _outputRootProof, _withdrawalProof);
    }

    /// @custom:kontrol-array-length-equals _withdrawalProof: 1,
    /// @custom:kontrol-bytes-length-equals _withdrawalProof: 600,
    function prove_proveWithdrawalTransaction_paused1(
        Types.WithdrawalTransaction memory _tx,
        uint256 _l2OutputIndex,
        Types.OutputRootProof calldata _outputRootProof,
        bytes[] calldata _withdrawalProof
    )
        external
    {
        prove_proveWithdrawalTransaction_paused_internal(_tx, _l2OutputIndex, _outputRootProof, _withdrawalProof);
    }

    function prove_proveWithdrawalTransaction_paused0(
        Types.WithdrawalTransaction memory _tx,
        uint256 _l2OutputIndex,
        Types.OutputRootProof calldata _outputRootProof
    )
        external
    {
        bytes[] memory _withdrawalProof = new bytes[](0);
        prove_proveWithdrawalTransaction_paused_internal(_tx, _l2OutputIndex, _outputRootProof, _withdrawalProof);
    }
}
