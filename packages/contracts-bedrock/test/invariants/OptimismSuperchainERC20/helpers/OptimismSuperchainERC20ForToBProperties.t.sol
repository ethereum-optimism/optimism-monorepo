// SPDX-License-Identifier: AGPL-3
pragma solidity 0.8.25;

import { OptimismSuperchainERC20 } from "src/L2/OptimismSuperchainERC20.sol";

contract OptimismSuperchainERC20ForToBProperties is OptimismSuperchainERC20 {
    /// @notice This is used by CryticERC20ExternalBasicProperties (only used
    /// in Medusa testing campaign)to know which properties to test, and
    /// remains here so Medusa and Foundry test campaigns can use a single
    /// setup
    bool public constant isMintableOrBurnable = true;
}
