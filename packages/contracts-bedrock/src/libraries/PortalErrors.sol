// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// @notice Error for when a deposit or withdrawal is to a bad target.
error BadTarget();
/// @notice Error for when a deposit has too much calldata.
error LargeCalldata();
/// @notice Error for when a deposit has too small of a gas limit.
error SmallGasLimit();
/// @notice Error for when a withdrawal transfer fails.
error TransferFailed();
/// @notice Error for when a method is called that only works when using a custom gas token.
error OnlyCustomGasToken();
/// @notice Error for when a method cannot be called with non zero CALLVALUE.
error NoValue();
/// @notice Error for an unauthorized CALLER.
error Unauthorized();
/// @notice Error for when a method cannot be called when paused. This could be renamed
///         to `Paused` in the future, but it collides with the `Paused` event.
error CallPaused();
/// @notice Error for special gas estimation.
error GasEstimation();
/// @notice Error for when a method is being reentered.
error NonReentrant();
/// @notice Error for invalid proof.
error InvalidProof();
/// @notice Error for invalid game type.
error InvalidGameType();
/// @notice Error for an invalid dispute game.
error InvalidDisputeGame();
/// @notice Error for an invalid merkle proof.
error InvalidMerkleProof();
/// @notice Error for when a dispute game has been blacklisted.
error Blacklisted();
/// @notice Error for when trying to withdrawal without first proven.
error Unproven();
/// @notice Error for when a proposal is not validated.
error ProposalNotValidated();
/// @notice Error for when a withdrawal has already been finalized.
error AlreadyFinalized();
