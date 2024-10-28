// SPDX-License-Identifier: MIT
pragma solidity ^0.8.13;

import { DeploymentSummaryFaultProofs } from "./utils/DeploymentSummaryFaultProofs.sol";
import { KontrolUtils } from "./utils/KontrolUtils.sol";
import { IL1CrossDomainMessenger as L1CrossDomainMessenger } from "src/L1/interfaces/IL1CrossDomainMessenger.sol";
import { ISuperchainConfig as SuperchainConfig } from "src/L1/interfaces/ISuperchainConfig.sol";

contract L1CrossDomainMessengerKontrol is DeploymentSummaryFaultProofs, KontrolUtils {
    L1CrossDomainMessenger l1CrossDomainMessenger;
    SuperchainConfig superchainConfig;

    /// @dev Inlined setUp function for faster Kontrol performance
    ///      Tracking issue: https://github.com/runtimeverification/kontrol/issues/282
    function setUpInlined() public {
        l1CrossDomainMessenger = L1CrossDomainMessenger(l1CrossDomainMessengerProxyAddress);
        superchainConfig = SuperchainConfig(superchainConfigProxyAddress);
    }

    function prove_relayMessage_paused(
        uint256 _nonce,
        address _sender,
        address _target,
        uint256 _value,
        uint256 _gas,
        bytes calldata _message
    )
        external
    {
        setUpInlined();

        // Pause System
        vm.prank(superchainConfig.guardian());
        superchainConfig.pause("identifier");

        vm.expectRevert("CrossDomainMessenger: paused");
        l1CrossDomainMessenger.relayMessage(_nonce, _sender, _target, _value, _gas, _message);
    }
}
