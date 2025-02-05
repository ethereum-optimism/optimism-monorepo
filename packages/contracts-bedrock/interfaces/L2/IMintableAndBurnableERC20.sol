// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { IERC20 } from "@openzeppelin/contracts/token/ERC20/IERC20.sol";

/// @title IMintableAndBurnableERC20
/// @notice Interface for mintable and burnable ERC20 tokens.
interface IMintableAndBurnableERC20 is IERC20 {
    /// @notice Mints `_amount` of tokens to `_to`.
    /// @param _to      Address to mint tokens to.
    /// @param _amount  Amount of tokens to mint.
    function mint(address _to, uint256 _amount) external;

    /// @notice Burns `_amount` of tokens from `_from`.
    /// @param _from    Address to burn tokens from.
    /// @param _amount  Amount of tokens to burn.
    function burn(address _from, uint256 _amount) external;
}
