// SPDX-License-Identifier: MIT
pragma solidity ^0.8.13;

import { DeploymentSummaryFaultProofs } from "./utils/DeploymentSummaryFaultProofs.sol";
import { KontrolUtils } from "./utils/KontrolUtils.sol";
import { IL1ERC721Bridge as L1ERC721Bridge } from "src/L1/interfaces/IL1ERC721Bridge.sol";
import { ISuperchainConfig as SuperchainConfig } from "src/L1/interfaces/ISuperchainConfig.sol";
import { ICrossDomainMessenger as CrossDomainMessenger } from "src/universal/interfaces/ICrossDomainMessenger.sol";

contract L1ERC721BridgeKontrol is DeploymentSummaryFaultProofs, KontrolUtils {
    L1ERC721Bridge l1ERC721Bridge;
    SuperchainConfig superchainConfig;

    function setUpInlined() public {
        l1ERC721Bridge = L1ERC721Bridge(l1ERC721BridgeProxyAddress);
        superchainConfig = SuperchainConfig(superchainConfigProxyAddress);
    }

    function prove_finalizeBridgeERC721_paused(
        address _localToken,
        address _remoteToken,
        address _from,
        address _to,
        uint256 _amount,
        bytes calldata _extraData
    )
        public
    {
        setUpInlined();

        // Pause Standard Bridge
        vm.prank(superchainConfig.guardian());
        superchainConfig.pause("identifier");

        vm.mockCall(
            address(l1ERC721Bridge.messenger()),
            abi.encodeCall(CrossDomainMessenger.xDomainMessageSender, ()),
            abi.encode(address(l1ERC721Bridge.otherBridge()))
        );

        vm.prank(address(l1ERC721Bridge.messenger()));
        vm.expectRevert("L1ERC721Bridge: paused");
        l1ERC721Bridge.finalizeBridgeERC721(_localToken, _remoteToken, _from, _to, _amount, _extraData);
    }
}
