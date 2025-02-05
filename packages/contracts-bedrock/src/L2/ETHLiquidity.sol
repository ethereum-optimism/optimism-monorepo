// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { SafeSend } from "src/universal/SafeSend.sol";

// Libraries
import { Unauthorized } from "src/libraries/errors/CommonErrors.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";

// Interfaces
import { ISemver } from "interfaces/universal/ISemver.sol";

/// @custom:proxied true
/// @custom:predeploy 0x4200000000000000000000000000000000000025
/// @title ETHLiquidity
/// @notice The ETHLiquidity contract allows other contracts to access ETH liquidity without
///         needing to modify the EVM to generate new ETH.
contract ETHLiquidity is ISemver {
    /// @notice Emitted when an address burns ETH liquidity.
    event LiquidityBurned(address indexed caller, uint256 value);

    /// @notice Emitted when an address mints ETH liquidity.
    event LiquidityMinted(address indexed caller, uint256 value);

    /// @notice Semantic version.
    /// @custom:semver 1.0.0-beta.6
    string public constant version = "1.0.0-beta.6";

    /// @notice Allows an address to lock ETH liquidity into this contract.
    function burn() external payable {
        if (msg.sender != Predeploys.SUPERCHAIN_WETH) revert Unauthorized();
        emit LiquidityBurned(msg.sender, msg.value);
    }

    /// @notice Allows an address to unlock ETH liquidity from this contract.
    /// @param _amount The amount of liquidity to unlock.
    function mint(uint256 _amount) external {
        if (msg.sender != Predeploys.SUPERCHAIN_WETH) revert Unauthorized();
        new SafeSend{ value: _amount }(payable(msg.sender));
        emit LiquidityMinted(msg.sender, _amount);
    }
}
