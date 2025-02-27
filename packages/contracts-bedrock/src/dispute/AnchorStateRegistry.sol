// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { Initializable } from "@openzeppelin/contracts/proxy/utils/Initializable.sol";

// Libraries
import { GameType, OutputRoot, Claim, GameStatus, Hash } from "src/dispute/lib/Types.sol";

// Interfaces
import { ISemver } from "interfaces/universal/ISemver.sol";
import { IFaultDisputeGame } from "interfaces/dispute/IFaultDisputeGame.sol";
import { IDisputeGame } from "interfaces/dispute/IDisputeGame.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { IOptimismPortal2 } from "interfaces/L1/IOptimismPortal2.sol";

/// @custom:proxied true
/// @title AnchorStateRegistry
/// @notice The AnchorStateRegistry is a contract that stores the latest "anchor" state for each available
///         FaultDisputeGame type. The anchor state is the latest state that has been proposed on L1 and was not
///         challenged within the challenge period. By using stored anchor states, new FaultDisputeGame instances can
///         be initialized with a more recent starting state which reduces the amount of required offchain computation.
contract AnchorStateRegistry is Initializable, ISemver {
    /// @notice Semantic version.
    /// @custom:semver 2.2.2
    string public constant version = "2.2.2";

    /// @notice Address of the SuperchainConfig contract.
    ISuperchainConfig public superchainConfig;

    /// @notice Address of the DisputeGameFactory contract.
    IDisputeGameFactory public disputeGameFactory;

    /// @notice Address of the OptimismPortal contract.
    IOptimismPortal2 public portal;

    /// @notice The game whose claim is currently being used as the anchor state.
    IFaultDisputeGame public anchorGame;

    /// @notice The starting anchor root.
    OutputRoot internal startingAnchorRoot;

    /// @notice Emitted when an anchor state is not updated.
    /// @param game Game that was not used as the new anchor game.
    event AnchorNotUpdated(IFaultDisputeGame indexed game);

    /// @notice Emitted when an anchor state is updated.
    /// @param game Game that was used as the new anchor game.
    event AnchorUpdated(IFaultDisputeGame indexed game);

    /// @notice Thrown when an unauthorized caller attempts to set the anchor state.
    error AnchorStateRegistry_Unauthorized();

    /// @notice Thrown when an invalid anchor game is provided.
    error AnchorStateRegistry_InvalidAnchorGame();

    /// @notice Thrown when the anchor root is requested, but the anchor game is blacklisted.
    error AnchorStateRegistry_AnchorGameBlacklisted();

    /// @notice Constructor to disable initializers.
    constructor() {
        _disableInitializers();
    }

    /// @notice Initializes the contract.
    /// @param _superchainConfig The address of the SuperchainConfig contract.
    /// @param _disputeGameFactory The address of the DisputeGameFactory contract.
    /// @param _portal The address of the OptimismPortal contract.
    /// @param _startingAnchorRoot The starting anchor root.
    function initialize(
        ISuperchainConfig _superchainConfig,
        IDisputeGameFactory _disputeGameFactory,
        IOptimismPortal2 _portal,
        OutputRoot memory _startingAnchorRoot
    )
        external
        initializer
    {
        superchainConfig = _superchainConfig;
        disputeGameFactory = _disputeGameFactory;
        portal = _portal;
        startingAnchorRoot = _startingAnchorRoot;
    }

    /// @notice Returns the respected game type.
    /// @return The respected game type.
    function respectedGameType() public view returns (GameType) {
        return portal.respectedGameType();
    }

    /// @custom:legacy
    /// @notice Returns the anchor root. Note that this is a legacy deprecated function and will
    ///         be removed in a future release. Use getAnchorRoot() instead. Anchor roots are no
    ///         longer stored per game type, so this function will return the same root for all
    ///         game types.
    function anchors(GameType /* unused */ ) external view returns (Hash, uint256) {
        return getAnchorRoot();
    }

    /// @notice Returns the current anchor root.
    /// @return The anchor root.
    function getAnchorRoot() public view returns (Hash, uint256) {
        // Return the starting anchor root if there is no anchor game.
        if (address(anchorGame) == address(0)) {
            return (startingAnchorRoot.root, startingAnchorRoot.l2BlockNumber);
        }

        // Otherwise, return the anchor root.
        return (Hash.wrap(anchorGame.rootClaim().raw()), anchorGame.l2BlockNumber());
    }

    /// @notice Determines whether a game is registered in the DisputeGameFactory.
    /// @param _game The game to check.
    /// @return Whether the game is factory registered.
    function isGameRegistered(IDisputeGame _game) public view returns (bool) {
        // Grab the game and game data.
        (GameType gameType, Claim rootClaim, bytes memory extraData) = _game.gameData();

        // Grab the verified address of the game based on the game data.
        (IDisputeGame _factoryRegisteredGame,) =
            disputeGameFactory.games({ _gameType: gameType, _rootClaim: rootClaim, _extraData: extraData });

        // Return whether the game is factory registered.
        return address(_factoryRegisteredGame) == address(_game);
    }

    /// @notice Determines whether a game is of a respected game type.
    /// @param _game The game to check.
    /// @return Whether the game is of a respected game type.
    function isGameRespected(IDisputeGame _game) public view returns (bool) {
        return _game.wasRespectedGameTypeWhenCreated();
    }

    /// @notice Determines whether a game is blacklisted.
    /// @param _game The game to check.
    /// @return Whether the game is blacklisted.
    function isGameBlacklisted(IDisputeGame _game) public view returns (bool) {
        return portal.disputeGameBlacklist(_game);
    }

    /// @notice Determines whether a game is retired.
    /// @param _game The game to check.
    /// @return Whether the game is retired.
    function isGameRetired(IDisputeGame _game) public view returns (bool) {
        // Must be created after the respectedGameTypeUpdatedAt timestamp. Note that this means all
        // games created in the same block as the respectedGameTypeUpdatedAt timestamp are
        // considered retired.
        return _game.createdAt().raw() <= portal.respectedGameTypeUpdatedAt();
    }

    /// @notice Returns whether a game is resolved.
    /// @param _game The game to check.
    /// @return Whether the game is resolved.
    function isGameResolved(IDisputeGame _game) public view returns (bool) {
        return _game.resolvedAt().raw() != 0
            && (_game.status() == GameStatus.DEFENDER_WINS || _game.status() == GameStatus.CHALLENGER_WINS);
    }

    /// @notice **READ THIS FUNCTION DOCUMENTATION CAREFULLY.**
    ///         Determines whether a game resolved properly and the game was not subject to any
    ///         invalidation conditions. The root claim of a proper game IS NOT guaranteed to be
    ///         valid. The root claim of a proper game CAN BE incorrect and still be a proper game.
    ///         DO NOT USE THIS FUNCTION ALONE TO DETERMINE IF A ROOT CLAIM IS VALID.
    /// @dev Note that isGameProper previously checked that the game type was equal to the
    ///      respected game type. However, it should be noted that it is possible for a game other
    ///      than the respected game type to resolve without being invalidated. Since isGameProper
    ///      exists to determine if a game has (or has not) been invalidated, we now allow any game
    ///      type to be considered a proper game. We enforce checks on the game type in
    ///      isGameClaimValid().
    /// @param _game The game to check.
    /// @return Whether the game is a proper game.
    function isGameProper(IDisputeGame _game) public view returns (bool) {
        // Must be registered in the DisputeGameFactory.
        if (!isGameRegistered(_game)) {
            return false;
        }

        // Must not be blacklisted.
        if (isGameBlacklisted(_game)) {
            return false;
        }

        // Must be created at or after the respectedGameTypeUpdatedAt timestamp.
        if (isGameRetired(_game)) {
            return false;
        }

        return true;
    }

    /// @notice Returns whether a game is finalized.
    /// @param _game The game to check.
    /// @return Whether the game is finalized.
    function isGameFinalized(IDisputeGame _game) public view returns (bool) {
        // Game must be resolved.
        if (!isGameResolved(_game)) {
            return false;
        }

        // Game must be beyond the "airgap period" - time since resolution must be at least
        // "dispute game finality delay" seconds in the past.
        if (block.timestamp - _game.resolvedAt().raw() <= portal.disputeGameFinalityDelaySeconds()) {
            return false;
        }

        return true;
    }

    /// @notice Returns whether a game's root claim is valid.
    /// @param _game The game to check.
    /// @return Whether the game's root claim is valid.
    function isGameClaimValid(IDisputeGame _game) public view returns (bool) {
        // Game must be a proper game.
        if (!isGameProper(_game)) {
            return false;
        }

        // Must be respected.
        if (!isGameRespected(_game)) {
            return false;
        }

        // Game must be finalized.
        if (!isGameFinalized(_game)) {
            return false;
        }

        // Game must be resolved in favor of the defender.
        if (_game.status() != GameStatus.DEFENDER_WINS) {
            return false;
        }

        return true;
    }

    /// @notice Updates the anchor game.
    /// @param _game New candidate anchor game.
    function setAnchorState(IDisputeGame _game) public {
        // Convert game to FaultDisputeGame.
        // We can't use FaultDisputeGame in the interface because this function is called from the
        // FaultDisputeGame contract which can't import IFaultDisputeGame by convention. We should
        // likely introduce a new interface (e.g., StateDisputeGame) that can act as a more useful
        // version of IDisputeGame in the future.
        IFaultDisputeGame game = IFaultDisputeGame(address(_game));

        // Check if the candidate game claim is valid.
        if (!isGameClaimValid(game)) {
            revert AnchorStateRegistry_InvalidAnchorGame();
        }

        // Must be newer than the current anchor game.
        (, uint256 anchorL2BlockNumber) = getAnchorRoot();
        if (game.l2BlockNumber() <= anchorL2BlockNumber) {
            revert AnchorStateRegistry_InvalidAnchorGame();
        }

        // Update the anchor game.
        anchorGame = game;
        emit AnchorUpdated(game);
    }
}
