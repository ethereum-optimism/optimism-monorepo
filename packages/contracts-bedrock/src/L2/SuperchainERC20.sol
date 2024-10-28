// SPDX-License-Identifier: MIT
pragma solidity 0.8.25;

import { ICrosschainERC20 } from "src/L2/interfaces/ICrosschainERC20.sol";
import { ISemver } from "src/universal/interfaces/ISemver.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";
import { ERC20 } from "@solady-v0.0.245/tokens/ERC20.sol";
import { Unauthorized } from "src/libraries/errors/CommonErrors.sol";

/// @title SuperchainERC20
/// @notice SuperchainERC20 is a standard extension of the base ERC20 token contract that unifies ERC20 token
///         bridging to make it fungible across the Superchain. This construction allows the SuperchainTokenBridge to
///         burn and mint tokens.
abstract contract SuperchainERC20 is ERC20, ICrosschainERC20, ISemver {
    /// @notice Semantic version.
    /// @custom:semver 1.0.0-beta.4
    function version() external view virtual returns (string memory) {
        return "1.0.0-beta.4";
    }

    /// @notice Allows the SuperchainTokenBridge to mint tokens.
    /// @param _to     Address to mint tokens to.
    /// @param _amount Amount of tokens to mint.
    function crosschainMint(address _to, uint256 _amount) external {
        if (msg.sender != Predeploys.SUPERCHAIN_TOKEN_BRIDGE) revert Unauthorized();

        _mint(_to, _amount);

        emit CrosschainMint(_to, _amount);
    }

    /// @notice Allows the SuperchainTokenBridge to burn tokens.
    /// @param _from   Address to burn tokens from.
    /// @param _amount Amount of tokens to burn.
    function crosschainBurn(address _from, uint256 _amount) external {
        if (msg.sender != Predeploys.SUPERCHAIN_TOKEN_BRIDGE) revert Unauthorized();

        _burn(_from, _amount);

        emit CrosschainBurn(_from, _amount);
    }
}
