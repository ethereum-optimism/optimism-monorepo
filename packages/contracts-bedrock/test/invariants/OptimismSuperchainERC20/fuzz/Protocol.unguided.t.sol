// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import { ProtocolHandler } from "../handlers/Protocol.t.sol";
import { EnumerableMap } from "@openzeppelin/contracts/utils/structs/EnumerableMap.sol";
import { OptimismSuperchainERC20 } from "src/L2/OptimismSuperchainERC20.sol";
import { CompatibleAssert } from "../helpers/CompatibleAssert.t.sol";

// TODO: add fuzz_sendERC20 when we implement non-atomic bridging
contract ProtocolUnguided is ProtocolHandler, CompatibleAssert {
    using EnumerableMap for EnumerableMap.Bytes32ToUintMap;

    /// @custom:property-id 12
    /// @custom:property supertoken total supply only increases on calls to mint() by the L2toL2StandardBridge
    function fuzz_mint(uint256 tokenIndex, address to, address sender, uint256 amount) external {
        address token = allSuperTokens[bound(tokenIndex, 0, allSuperTokens.length)];
        bytes32 salt = MESSENGER.superTokenInitDeploySalts(token);
        amount = bound(amount, 0, type(uint256).max - OptimismSuperchainERC20(token).totalSupply());
        vm.prank(sender);
        try OptimismSuperchainERC20(token).crosschainMint(to, amount) {
            compatibleAssert(sender == BRIDGE);
            (, uint256 currentValue) = ghost_totalSupplyAcrossChains.tryGet(salt);
            ghost_totalSupplyAcrossChains.set(salt, currentValue + amount);
        } catch {
            compatibleAssert(sender != BRIDGE || to == address(0));
        }
    }

    /// @custom:property-id 13
    /// @custom:property supertoken total supply only increases on calls to mint() by the L2toL2StandardBridge
    function fuzz_burn(uint256 tokenIndex, address from, address sender, uint256 amount) external {
        address token = allSuperTokens[bound(tokenIndex, 0, allSuperTokens.length)];
        bytes32 salt = MESSENGER.superTokenInitDeploySalts(token);
        uint256 senderBalance = OptimismSuperchainERC20(token).balanceOf(sender);
        vm.prank(sender);
        try OptimismSuperchainERC20(token).crosschainBurn(from, amount) {
            compatibleAssert(sender == BRIDGE);
            (, uint256 currentValue) = ghost_totalSupplyAcrossChains.tryGet(salt);
            ghost_totalSupplyAcrossChains.set(salt, currentValue - amount);
        } catch {
            compatibleAssert(sender != BRIDGE || senderBalance < amount);
        }
    }

    /// @custom:property-id 25
    /// @custom:property supertokens can't be reinitialized
    function fuzz_initialize(
        address sender,
        uint256 tokenIndex,
        address remoteToken,
        string memory name,
        string memory symbol,
        uint8 decimals
    )
        external
    {
        vm.prank(sender);
        // revert is possible in bound, but is not part of the external call
        try OptimismSuperchainERC20(allSuperTokens[bound(tokenIndex, 0, allSuperTokens.length)]).initialize(
            remoteToken, name, symbol, decimals
        ) {
            compatibleAssert(false);
        } catch { }
    }
}
