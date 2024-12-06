// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// @notice Error for an unauthorized CALLER.
error Unauthorized();

/// @notice Error for when a method is called that only works when using a custom gas token.
error OnlyCustomGasToken();

/// @notice Error for when a method is called that only works when NOT using a custom gas token.
error NotCustomGasToken();

/// @notice Error for when a transfer via call fails.
error TransferFailed();

/// @notice Thrown when attempting to perform an operation and the account is the zero address.
error ZeroAddress();
