// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// @notice Error for an unauthorized CALLER.
error Unauthorized();

/// @notice Error for when a transfer via call fails.
error TransferFailed();

/// @notice Thrown when attempting to perform an operation and the account is the zero address.
error ZeroAddress();

/// @notice Thrown when a function is called while the contract is paused.
error Paused();
