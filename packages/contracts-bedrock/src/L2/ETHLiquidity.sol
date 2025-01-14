// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { SafeSend } from "src/universal/SafeSend.sol";
import { Semver } from "src/universal/Semver.sol";

// Libraries
import { Unauthorized, NotCustomGasToken } from "src/libraries/errors/CommonErrors.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";

// Interfaces
import { IL1Block } from "interfaces/L2/IL1Block.sol";

/// @title ETHLiquidity
/// @notice The ETHLiquidity contract allows other contracts to access ETH liquidity without
///         needing to modify the EVM to generate new ETH.
contract ETHLiquidity is Semver {
    /// @notice Emitted when an address burns ETH liquidity.
    event LiquidityBurned(address indexed caller, uint256 value);

    /// @notice Emitted when an address mints ETH liquidity.
    event LiquidityMinted(address indexed caller, uint256 value);

    /// @notice Semantic version.
    /// @custom:semver 1.0.1
    function _version() internal pure override returns (Versions memory) {
        return Versions({ major: 1, minor: 0, patch: 1, suffix: "" });
    }

    /// @notice Allows an address to lock ETH liquidity into this contract.
    function burn() external payable {
        if (msg.sender != Predeploys.SUPERCHAIN_WETH) revert Unauthorized();
        if (IL1Block(Predeploys.L1_BLOCK_ATTRIBUTES).isCustomGasToken()) revert NotCustomGasToken();
        emit LiquidityBurned(msg.sender, msg.value);
    }

    /// @notice Allows an address to unlock ETH liquidity from this contract.
    /// @param _amount The amount of liquidity to unlock.
    function mint(uint256 _amount) external {
        if (msg.sender != Predeploys.SUPERCHAIN_WETH) revert Unauthorized();
        if (IL1Block(Predeploys.L1_BLOCK_ATTRIBUTES).isCustomGasToken()) revert NotCustomGasToken();
        new SafeSend{ value: _amount }(payable(msg.sender));
        emit LiquidityMinted(msg.sender, _amount);
    }
}
