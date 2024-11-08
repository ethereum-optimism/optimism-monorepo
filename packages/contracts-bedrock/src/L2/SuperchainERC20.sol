// SPDX-License-Identifier: MIT
pragma solidity 0.8.25;

import { IERC7802, IERC165 } from "src/L2/interfaces/IERC7802.sol";
import { ISemver } from "src/universal/interfaces/ISemver.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";
import { ERC20 } from "@solady-v0.0.245/tokens/ERC20.sol";
import { IERC20 } from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import { Unauthorized } from "src/libraries/errors/CommonErrors.sol";

/// @title SuperchainERC20
/// @notice A standard ERC20 extension implementing IERC7802 for unified cross-chain fungibility across
///         the Superchain. Allows the SuperchainTokenBridge to mint and burn tokens as needed.
abstract contract SuperchainERC20 is ERC20, IERC7802, ISemver {
    /// @notice Semantic version.
    /// @custom:semver 1.0.0-beta.5
    function version() external view virtual returns (string memory) {
        return "1.0.0-beta.5";
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

    /// @inheritdoc IERC165
    function supportsInterface(bytes4 _interfaceId) public view virtual returns (bool) {
        return _interfaceId == type(IERC7802).interfaceId || _interfaceId == type(IERC20).interfaceId
            || _interfaceId == type(IERC165).interfaceId;
    }
}
