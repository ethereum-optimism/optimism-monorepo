// SPDX-License-Identifier: MIT
pragma solidity 0.8.25;

import { SuperchainERC20 } from "src/L2/SuperchainERC20.sol";

/// @title SuperchainERC20Implementation Mock contract
/// @notice Mock contract just to create tests over an implementation of the SuperchainERC20 abstract contract.
contract MockSuperchainERC20Implementation is SuperchainERC20 {
    function name() public pure override returns (string memory) {
        return "SuperchainERC20";
    }

    function symbol() public pure override returns (string memory) {
        return "SCE";
    }
}
