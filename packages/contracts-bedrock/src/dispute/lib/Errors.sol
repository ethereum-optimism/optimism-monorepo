// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

import { GameType, Hash, Claim } from "src/dispute/lib/LibUDT.sol";

////////////////////////////////////////////////////////////////
//                `DisputeGameFactory` Errors                 //
////////////////////////////////////////////////////////////////

/// @notice Thrown when a dispute game is attempted to be created with an unsupported game type.
/// @param gameType The unsupported game type.
error NoImplementation(GameType gameType);

/// @notice Thrown when a dispute game that already exists is attempted to be created.
/// @param uuid The UUID of the dispute game that already exists.
error GameAlreadyExists(Hash uuid);

/// @notice Thrown when the root claim has an unexpected VM status.
///         Some games can only start with a root-claim with a specific status.
/// @param rootClaim is the claim that was unexpected.
error UnexpectedRootClaim(Claim rootClaim);

////////////////////////////////////////////////////////////////
//                 `FaultDisputeGame` Errors                  //
////////////////////////////////////////////////////////////////

/// @notice Thrown when a dispute game has already been initialized.
error AlreadyInitialized();

/// @notice Thrown when a supplied bond is not equal to the required bond amount to cover the cost of the interaction.
error IncorrectBondAmount();

/// @notice Thrown when a credit claim is attempted for a value of 0.
error NoCreditToClaim();

/// @notice Thrown when the transfer of credit to a recipient account reverts.
error BondTransferFailed();

/// @notice Thrown when the `extraData` passed to the CWIA proxy is of improper length, or contains invalid information.
error BadExtraData();

/// @notice Thrown when a defense against the root claim is attempted.
error CannotDefendRootClaim();

/// @notice Thrown when a claim is attempting to be made that already exists.
error ClaimAlreadyExists();

/// @notice Thrown when a disputed claim does not match its index in the game.
error InvalidDisputedClaimIndex();

/// @notice Thrown when an action that requires the game to be `IN_PROGRESS` is invoked when
///         the game is not in progress.
error GameNotInProgress();

/// @notice Thrown when a move is attempted to be made after the clock has timed out.
error ClockTimeExceeded();

/// @notice Thrown when the game is attempted to be resolved too early.
error ClockNotExpired();

/// @notice Thrown when a move is attempted to be made at or greater than the max depth of the game.
error GameDepthExceeded();

/// @notice Thrown when a step is attempted above the maximum game depth.
error InvalidParent();

/// @notice Thrown when an invalid prestate is supplied to `step`.
error InvalidPrestate();

/// @notice Thrown when a step is made that computes the expected post state correctly.
error ValidStep();

/// @notice Thrown when a game is attempted to be initialized with an L1 head that does
///         not contain the disputed output root.
error L1HeadTooOld();

/// @notice Thrown when an invalid local identifier is passed to the `addLocalData` function.
error InvalidLocalIdent();

/// @notice Thrown when resolving claims out of order.
error OutOfOrderResolution();

/// @notice Thrown when resolving a claim that has already been resolved.
error ClaimAlreadyResolved();

/// @notice Thrown when a parent output root is attempted to be found on a claim that is in
///         the output root portion of the tree.
error ClaimAboveSplit();

/// @notice Thrown on deployment if the split depth is greater than or equal to the max
///         depth of the game.
error InvalidSplitDepth();

/// @notice Thrown on deployment if the max clock duration is less than or equal to the clock extension.
error InvalidClockExtension();

/// @notice Thrown on deployment if the PreimageOracle challenge period is too high.
error InvalidChallengePeriod();

/// @notice Thrown on deployment if the max depth is greater than `LibPosition.`
error MaxDepthTooLarge();

/// @notice Thrown when trying to step against a claim for a second time, after it has already been countered with
///         an instruction step.
error DuplicateStep();

/// @notice Thrown when an anchor root is not found for a given game type.
error AnchorRootNotFound();

/// @notice Thrown when an output root proof is invalid.
error InvalidOutputRootProof();

/// @notice Thrown when header RLP is invalid with respect to the block hash in an output root proof.
error InvalidHeaderRLP();

/// @notice Thrown when there is a match between the block number in the output root proof and the block number
///         claimed in the dispute game.
error BlockNumberMatches();

/// @notice Thrown when the L2 block number claim has already been challenged.
error L2BlockNumberChallenged();

////////////////////////////////////////////////////////////////
//              `PermissionedDisputeGame` Errors              //
////////////////////////////////////////////////////////////////

/// @notice Thrown when an unauthorized address attempts to interact with the game.
error BadAuth();

////////////////////////////////////////////////////////////////
//              `AnchorStateRegistry` Errors                  //
////////////////////////////////////////////////////////////////

/// @notice Thrown when attempting to set an anchor state using an unregistered game.
error UnregisteredGame();

/// @notice Thrown when attempting to set an anchor state using an invalid game result.
error InvalidGameStatus();
