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
/// @title GameValidityOracle
/// @notice The GameValidityOracle is responsible for determining the validity of a dispute game.
/// Other contracts can rely on the assertions made by this contract to ensure that a game is or is
/// not valid. The GameValidityOracle also provides the anchor state previously provided by the
/// AnchorStateRegistry that can be used to create new dispute games.
contract GameValidityOracle is Initializable, ISemver {
    error GameValidityOracle_AnchorGameIsNewer(uint256 anchorGameL2BlockNumber, uint256 candidateGameL2BlockNumber);

    // TODO: comment these
    event AnchorGameSet(IDisputeGame newAnchorGame);
    event RespectedGameTypeSet(GameType gameType);
    event DisputeGameBlacklisted(IDisputeGame game);
    event GameRetirementTimestampSet(uint64 timestamp);

    /// @notice Semantic version.
    /// @custom:semver 1.0.0-beta.1
    string public constant version = "1.0.0-beta.1";

    /// @notice Address of the SuperchainConfig contract.
    ISuperchainConfig public superchainConfig;

    /// @notice DisputeGameFactory address.
    IDisputeGameFactory public disputeGameFactory;

    /// @notice The delay between when a dispute game is resolved and when it can be considered finalized.
    uint256 public disputeGameFinalityDelaySeconds;

    uint64 public gameRetirementTimestamp;

    GameType public respectedGameType;

    IDisputeGame internal anchorGame;

    OutputRoot internal initialAnchorRoot;

    /// @notice Returns whether a game is blacklisted.
    mapping(IDisputeGame => bool) public isGameBlacklisted;

    constructor() {
        _disableInitializers();
    }

    /// @notice Initializes the contract.
    /// @param _startingAnchorRoot A starting anchor root.
    /// @param _disputeGameFactory DisputeGameFactory address.
    /// @param _disputeGameFinalityDelaySeconds The delay between when a dispute game is resolved and when it can be
    /// considered finalized.
    /// @param _superchainConfig The address of the SuperchainConfig contract.
    function initialize(
        OutputRoot memory _startingAnchorRoot,
        IDisputeGameFactory _disputeGameFactory,
        uint256 _disputeGameFinalityDelaySeconds,
        ISuperchainConfig _superchainConfig
    )
        external
        initializer
    {
        initialAnchorRoot = _startingAnchorRoot;
        disputeGameFactory = _disputeGameFactory;
        disputeGameFinalityDelaySeconds = _disputeGameFinalityDelaySeconds;
        superchainConfig = _superchainConfig;
    }

    function getAnchorState() public view returns (OutputRoot memory) {
        if (anchorGame == IDisputeGame(address(0))) return initialAnchorRoot;
        if (isGameBlacklisted[anchorGame]) revert GameValidityOracle_GameBlacklisted(anchorGame);
        return
            OutputRoot({ l2BlockNumber: anchorGame.l2BlockNumber(), root: Hash.wrap(anchorGame.rootClaim().raw()) });
    }

    function setAnchorState(IDisputeGame _game) external {
        assertGameValid(_game);

        uint256 anchorL2BlockNumber = anchorGame.l2BlockNumber();
        uint256 gameL2BlockNumber = _game.l2BlockNumber();
        if (gameL2BlockNumber <= anchorL2BlockNumber) {
            revert GameValidityOracle_AnchorGameIsNewer(anchorL2BlockNumber, gameL2BlockNumber);
        }
        anchorGame = _game;
        emit AnchorGameSet(_game);
    }

    function retireAllExistingGames() external {
        if (msg.sender != _guardian()) revert Unauthorized();
        gameRetirementTimestamp = uint64(block.timestamp);
        emit GameRetirementTimestampSet(gameRetirementTimestamp);
    }

    /// @notice Blacklists a dispute game. Should only be used in the event that a dispute game resolves incorrectly.
    /// @param _disputeGame Dispute game to blacklist.
    function setGameBlacklisted(IDisputeGame _disputeGame) external {
        if (msg.sender != _guardian()) revert Unauthorized();
        isGameBlacklisted[_disputeGame] = true;
        emit DisputeGameBlacklisted(_disputeGame);
    }

    /// @notice Sets the respected game type. Changing this value can alter the security properties of the system,
    ///         depending on the new game's behavior.
    /// @param _gameType The game type to consult for output proposals.
    function setRespectedGameType(GameType _gameType) external {
        if (msg.sender != _guardian()) revert Unauthorized();
        respectedGameType = _gameType;
        emit RespectedGameTypeSet(_gameType);
    }

    function isGameRetired(IDisputeGame _game) public view returns (bool) {
        // Must be created after the gameRetirementTimestamp.
        return _game.createdAt().raw() <= gameRetirementTimestamp;
    }

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

    function _guardian() internal view returns (address) {
        return superchainConfig.guardian();
    }
}
