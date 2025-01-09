// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { Initializable } from "@openzeppelin/contracts/proxy/utils/Initializable.sol";

// Libraries
import { GameType, OutputRoot, Claim, GameStatus, Hash } from "src/dispute/lib/Types.sol";

// Interfaces
import { ISemver } from "interfaces/universal/ISemver.sol";
import { IDisputeGame } from "interfaces/dispute/IDisputeGame.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";

/// @custom:proxied true
/// @title AnchorStateRegistry
/// @notice The AnchorStateRegistry is responsible for determining the validity of a dispute game.
/// Other contracts can rely on the assertions made by this contract to ensure that a game is or is
/// not valid. The AnchorStateRegistry also provides the anchor state previously provided by the
/// AnchorStateRegistry that can be used to create new dispute games.
contract AnchorStateRegistry is Initializable, ISemver {
    error AnchorStateRegistry_AnchorGameBlacklisted(IDisputeGame game);
    error AnchorStateRegistry_AnchorGameIsNewer();
    error AnchorStateRegistry_CandidateGameNotValid(string reason);
    error AnchorStateRegistry_OnlyGuardian();

    /// @notice Emitted when a new anchor game is set.
    /// @param newAnchorGame The new anchor game.
    event AnchorGameSet(IDisputeGame newAnchorGame);

    /// @notice Emitted when the respected game type is set.
    /// @param gameType The new respected game type.
    event RespectedGameTypeSet(GameType gameType);

    /// @notice Emitted when a dispute game is blacklisted.
    /// @param game The blacklisted game.
    event DisputeGameBlacklisted(IDisputeGame game);

    /// @notice Emitted when the game retirement timestamp is set.
    /// @param timestamp The new game retirement timestamp.
    event GameRetirementTimestampSet(uint64 timestamp);

    /// @notice Semantic version.
    /// @custom:semver 3.0.0-beta.1
    string public constant version = "3.0.0-beta.1";

    /// @custom:legacy
    /// @custom:spacer anchors
    /// @notice Spacer taking up the legacy `anchors` mapping slot.
    bytes32 private spacer_1_0_32;

    /// @notice Address of the SuperchainConfig contract.
    ISuperchainConfig public superchainConfig;

    /// @notice DisputeGameFactory address.
    IDisputeGameFactory public disputeGameFactory;

    /// @notice Delay between game resolution and finalization.
    uint256 public disputeGameFinalityDelaySeconds;

    /// @notice Timestamp after which games are considered retired.
    uint64 public gameRetirementTimestamp;

    /// @notice The game type that is respected for output proposals.
    GameType public respectedGameType;

    /// @notice The current anchor game.
    IDisputeGame internal anchorGame;

    /// @notice The initial anchor root.
    OutputRoot internal initialAnchorRoot;

    /// @notice Returns whether a game is blacklisted.
    mapping(IDisputeGame => bool) public isGameBlacklisted;

    /// @notice Constructor to disable initializers.
    constructor() {
        _disableInitializers();
    }

    /// @notice Initializes the contract.
    /// @param _superchainConfig The address of the SuperchainConfig contract.
    /// @param _initialAnchorRoot A starting anchor root.
    /// @param _disputeGameFactory DisputeGameFactory address.
    /// @param _disputeGameFinalityDelaySeconds Delay between game resolution and finalization.
    function initialize(
        ISuperchainConfig _superchainConfig,
        OutputRoot memory _initialAnchorRoot,
        IDisputeGameFactory _disputeGameFactory,
        uint256 _disputeGameFinalityDelaySeconds
    )
        external
        initializer
    {
        superchainConfig = _superchainConfig;
        initialAnchorRoot = _initialAnchorRoot;
        disputeGameFactory = _disputeGameFactory;
        disputeGameFinalityDelaySeconds = _disputeGameFinalityDelaySeconds;
    }

    /// @notice Returns the current anchor root.
    /// @return The current anchor root.
    function getAnchorRoot() public view returns (OutputRoot memory) {
        // If we don't have an anchor game yet, return the initial anchor root.
        if (anchorGame == IDisputeGame(address(0))) {
            return initialAnchorRoot;
        }

        // Revert if the anchor game is blacklisted.
        if (isGameBlacklisted[anchorGame]) {
            revert AnchorStateRegistry_AnchorGameBlacklisted(anchorGame);
        }

        // Otherwise, return the anchor root.
        // We don't revert if the anchor game is retired because it's very likely that this
        // scenario could happen in practice. If you want to stop the current anchor game from
        // being used, blacklist it.
        return OutputRoot({ l2BlockNumber: anchorGame.l2BlockNumber(), root: Hash.wrap(anchorGame.rootClaim().raw()) });
    }

    /// @custom:legacy
    /// @notice Returns the current anchor root.
    /// @return The current anchor root.
    function anchors(GameType /* _gameType */ ) external view returns (OutputRoot memory) {
        return getAnchorRoot();
    }

    /// @notice Updates the anchor game.
    /// @param _game New candidate anchor game.
    function setAnchorGame(IDisputeGame _game) external {
        // Check if the candidate game is valid.
        (bool valid, string memory reason) = isGameValid(_game);
        if (!valid) {
            revert AnchorStateRegistry_CandidateGameNotValid(reason);
        }

        // Check if the candidate game is newer than the current anchor game.
        if (_game.l2BlockNumber() <= anchorGame.l2BlockNumber()) {
            revert AnchorStateRegistry_AnchorGameIsNewer();
        }

        // Update the anchor game.
        anchorGame = _game;
        emit AnchorGameSet(_game);
    }

    /// @notice Allows the Guardian to retire all existing games.
    function retireAllExistingGames() external {
        if (msg.sender != superchainConfig.guardian()) revert AnchorStateRegistry_OnlyGuardian();
        gameRetirementTimestamp = uint64(block.timestamp);
        emit GameRetirementTimestampSet(gameRetirementTimestamp);
    }

    /// @notice Allows the Guardian to blacklist a dispute game.
    /// @param _disputeGame Dispute game to blacklist.
    function setGameBlacklisted(IDisputeGame _disputeGame) external {
        if (msg.sender != superchainConfig.guardian()) revert AnchorStateRegistry_OnlyGuardian();
        isGameBlacklisted[_disputeGame] = true;
        emit DisputeGameBlacklisted(_disputeGame);
    }

    /// @notice Allows the Guardian to set the respected game type.
    /// @param _gameType The game type to consult for output proposals.
    function setRespectedGameType(GameType _gameType) external {
        if (msg.sender != superchainConfig.guardian()) revert AnchorStateRegistry_OnlyGuardian();
        respectedGameType = _gameType;
        emit RespectedGameTypeSet(_gameType);
    }

    /// @notice Returns whether a game is retired.
    /// @param _game The game to check.
    /// @return Whether the game is retired.
    function isGameRetired(IDisputeGame _game) public view returns (bool) {
        // Must be created after the gameRetirementTimestamp.
        return _game.createdAt().raw() <= gameRetirementTimestamp;
    }

    /// @notice Returns whether a game is maybe valid.
    /// @param _game The game to check.
    /// @return Whether the game is maybe valid.
    /// @return Reason why the game is not maybe valid.
    function isGameMaybeValid(IDisputeGame _game) public view returns (bool, string memory) {
        // Grab the game and game data.
        (GameType gameType, Claim rootClaim, bytes memory extraData) = _game.gameData();

        // Grab the verified address of the game based on the game data.
        (IDisputeGame _factoryRegisteredGame,) =
            disputeGameFactory.games({ _gameType: gameType, _rootClaim: rootClaim, _extraData: extraData });

        // Must be a game created by the factory.
        if (address(_factoryRegisteredGame) != address(_game)) {
            return (false, "game not factory registered");
        }

        // Must not be blacklisted.
        if (isGameBlacklisted[_game]) {
            return (false, "game blacklisted");
        }

        // Must have been the respected game type when the game was created.
        if (!_game.wasRespectedGameTypeWhenCreated()) {
            return (false, "game respected game type mismatch");
        }

        // Must be a game with a status other than CHALLENGER_WINS.
        if (_game.status() == GameStatus.CHALLENGER_WINS) {
            return (false, "game challenger wins");
        }

        // Must be created after the gameRetirementTimestamp.
        if (isGameRetired(_game)) {
            return (false, "game retired");
        }

        return (true, "");
    }

    /// @notice Returns whether a game is valid.
    /// @param _game The game to check.
    /// @return Whether the game is valid.
    /// @return Reason why the game is not valid.
    function isGameValid(IDisputeGame _game) public view returns (bool, string memory) {
        // Game must be maybe valid.
        (bool maybeValid, string memory notMaybeValidReason) = isGameMaybeValid(_game);
        if (!maybeValid) {
            return (false, notMaybeValidReason);
        }

        // Game must be finalized.
        (bool finalized, string memory notFinalizedReason) = isGameFinalized(_game);
        if (!finalized) {
            return (false, notFinalizedReason);
        }

        return (true, "");
    }

    /// @notice Returns whether a game is finalized.
    /// @param _game The game to check.
    /// @return Whether the game is finalized.
    /// @return Reason why the game is not finalized.
    function isGameFinalized(IDisputeGame _game) public view returns (bool, string memory) {
        // Game status must be CHALLENGER_WINS or DEFENDER_WINS
        if (_game.status() != GameStatus.DEFENDER_WINS && _game.status() != GameStatus.CHALLENGER_WINS) {
            return (false, "game not resolved");
        }

        // Game resolvedAt timestamp must be non-zero
        uint256 _resolvedAt = _game.resolvedAt().raw();
        if (_resolvedAt == 0) {
            return (false, "game not resolved");
        }

        // Game resolvedAt timestamp must be more than airgap period seconds ago
        if (block.timestamp - _resolvedAt <= disputeGameFinalityDelaySeconds) {
            return (false, "game must wait finality delay");
        }

        return (true, "");
    }
}
