// SPDX-License-Identifier: MIT
pragma solidity 0.8.25;

// Libraries
import { Predeploys } from "src/libraries/Predeploys.sol";
import { Unauthorized } from "src/libraries/errors/CommonErrors.sol";

// Interfaces
import { IERC7802, IERC165 } from "src/L2/interfaces/IERC7802.sol";

/// @title AbstractSuperchainERC20
/// @notice A standard ERC20 extension implementing IERC7802 for unified cross-chain fungibility across
///         the Superchain. Allows the SuperchainTokenBridge to mint and burn tokens as needed.
abstract contract AbstractSuperchainERC20 is IERC7802 {

    error NotImplemented();

    /// @notice Allows the SuperchainTokenBridge to mint tokens.
    /// @param _to     Address to mint tokens to.
    /// @param _amount Amount of tokens to mint.
    function crosschainMint(address _to, uint256 _amount) external {
        if (msg.sender != Predeploys.SUPERCHAIN_TOKEN_BRIDGE) revert Unauthorized();

        _crosschainMint(_to, _amount);

        emit CrosschainMint(_to, _amount, msg.sender);
    }

    /// @notice Allows the SuperchainTokenBridge to burn tokens.
    /// @param _from   Address to burn tokens from.
    /// @param _amount Amount of tokens to burn.
    function crosschainBurn(address _from, uint256 _amount) external {
        if (msg.sender != Predeploys.SUPERCHAIN_TOKEN_BRIDGE) revert Unauthorized();

        _crosschainBurn(_from, _amount);

        emit CrosschainBurn(_from, _amount, msg.sender);
    }

    function _crosschainBurn(address _from, uint256 _amount) internal virtual {
        revert NotImplemented();
    }

    function _crosschainMint(address _to, uint256 _amount) internal virtual {
        revert NotImplemented();
    }

    /// @inheritdoc IERC165
    function supportsInterface(bytes4 _interfaceId) public view virtual returns (bool) {
        return _interfaceId == type(IERC7802).interfaceId || _interfaceId == type(IERC20).interfaceId
            || _interfaceId == type(IERC165).interfaceId;
    }
}
