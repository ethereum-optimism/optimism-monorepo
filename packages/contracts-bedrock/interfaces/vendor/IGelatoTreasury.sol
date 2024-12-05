// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// @title IGelatoTreasury
/// @notice Interface for the GelatoTreasury contract.
interface IGelatoTreasury {
    function totalDepositedAmount(address _user, address _token) external view returns (uint256);
    function totalWithdrawnAmount(address _user, address _token) external view returns (uint256);
}
