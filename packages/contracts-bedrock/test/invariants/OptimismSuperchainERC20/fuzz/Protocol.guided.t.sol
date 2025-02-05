// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import { MockL2ToL2CrossDomainMessenger } from "../helpers/MockL2ToL2CrossDomainMessenger.t.sol";
import { SuperchainERC20 } from "src/L2/SuperchainERC20.sol";
import { ProtocolHandler } from "../handlers/Protocol.t.sol";
import { EnumerableMap } from "@openzeppelin/contracts/utils/structs/EnumerableMap.sol";
import { CompatibleAssert } from "../helpers/CompatibleAssert.t.sol";

contract ProtocolGuided is ProtocolHandler, CompatibleAssert {
    using EnumerableMap for EnumerableMap.Bytes32ToUintMap;
    /// @notice deploy a new supertoken with deploy salt determined by params, to the given (of course mocked) chainId
    /// @custom:property-id 14
    /// @custom:property supertoken total supply starts at zero

    function fuzz_deployNewSupertoken(
        TokenDeployParams memory params,
        uint256 chainId
    )
        external
        validateTokenDeployParams(params)
    {
        chainId = bound(chainId, 0, MAX_CHAINS - 1);
        SuperchainERC20 supertoken = _deploySupertoken(
            remoteTokens[params.remoteTokenIndex],
            WORDS[params.nameIndex],
            WORDS[params.symbolIndex],
            DECIMALS[params.decimalsIndex],
            chainId
        );
        // 14
        compatibleAssert(supertoken.totalSupply() == 0);
    }

    /// @custom:property-id 11
    /// @custom:property relayERC20 increases the token's totalSupply in the destination chain exactly by the input
    /// amount
    /// @custom:property-id 27
    /// @custom:property relayERC20 increases sender's balance in the destination chain exactly by the input amount
    /// @custom:property-id 7
    /// @custom:property calls to relayERC20 always succeed as long as the cross-domain caller is valid
    function fuzz_relayERC20(uint256 messageIndex) external {
        MockL2ToL2CrossDomainMessenger.CrossChainMessage memory messageToRelay = MESSENGER.messageQueue(messageIndex);
        SuperchainERC20 destinationToken = SuperchainERC20(messageToRelay.crossDomainMessageSender);
        uint256 destinationSupplyBefore = destinationToken.totalSupply();
        uint256 destinationBalanceBefore = destinationToken.balanceOf(messageToRelay.recipient);

        try MESSENGER.relayMessageFromQueue(messageIndex) {
            bytes32 deploySalt = MESSENGER.superTokenInitDeploySalts(address(destinationToken));
            (bool success, uint256 currentlyInTransit) = ghost_tokensInTransit.tryGet(deploySalt);
            // if sendERC20 didnt intialize this, then test suite is broken
            compatibleAssert(success);
            ghost_tokensInTransit.set(deploySalt, currentlyInTransit - messageToRelay.amount);
            // 11
            compatibleAssert(destinationSupplyBefore + messageToRelay.amount == destinationToken.totalSupply());
            // 27
            compatibleAssert(
                destinationBalanceBefore + messageToRelay.amount == destinationToken.balanceOf(messageToRelay.recipient)
            );
        } catch {
            // 7
            compatibleAssert(false);
        }
    }
}
