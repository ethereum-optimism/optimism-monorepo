// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { Initializable } from "@openzeppelin/contracts/proxy/utils/Initializable.sol";

// Libraries
import { GameType, OutputRoot, Claim, GameStatus, Hash } from "src/dispute/lib/Types.sol";
import { Unauthorized } from "src/libraries/errors/CommonErrors.sol";

// Interfaces
import { ISemver } from "interfaces/universal/ISemver.sol";
import { IFaultDisputeGame } from "interfaces/dispute/IFaultDisputeGame.sol";
import { IDisputeGame } from "interfaces/dispute/IDisputeGame.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";

/// @custom:proxied true
/// @title AnchorStateRegistry
/// @notice The AnchorStateRegistry is a contract that stores the latest "anchor" state for each available
///         FaultDisputeGame type. The anchor state is the latest state that has been proposed on L1 and was not
///         challenged within the challenge period. By using stored anchor states, new FaultDisputeGame instances can
///         be initialized with a more recent starting state which reduces the amount of required offchain computation.
contract AnchorStateRegistry is Initializable, ISemver {
    error InvalidGame();
    error AnchorGameIsNewer(uint256 anchorGameL2BlockNumber, uint256 candidateGameL2BlockNumber);

    event AnchorGameSet(IDisputeGame newAnchorGame);
    event RespectedGameTypeSet(GameType gameType);
    event DisputeGameBlacklisted(IDisputeGame game);
    event GameRetirementTimestampSet(uint64 timestamp);

    /// @notice Semantic version.
    // TODO: update?
    /// @custom:semver 2.0.1-beta.6
    string public constant version = "2.0.1-beta.6";

    /// @notice DisputeGameFactory address.
    // TODO: find storage slot
    IDisputeGameFactory public disputeGameFactory;

    /// @notice The delay between when a dispute game is resolved and when it can be considered finalized.
    // TODO: find storage slot
    uint256 public disputeGameFinalityDelaySeconds;

    // TODO: storage slot issue
    // / @notice Returns the anchor state for the given game type.
    // mapping(GameType => OutputRoot) public anchors;

    // TODO: determine if this belongs
    // uint256 __gap0;

    /// @notice Address of the SuperchainConfig contract.
    ISuperchainConfig public superchainConfig;

    /// @notice Returns whether a game is blacklisted.
    mapping(IDisputeGame => bool) public isGameBlacklisted;

    uint64 public gameRetirementTimestamp;

    GameType public respectedGameType;

    IFaultDisputeGame internal _anchorGame;

    constructor() {
        _disableInitializers();
    }

    /// @notice Initializes the contract.
    /// @param _disputeGameFactory DisputeGameFactory address.
    /// @param _disputeGameFinalityDelaySeconds The delay between when a dispute game is resolved and when it can be
    /// considered finalized.
    /// @param _authorizedGame An authorized dispute game.
    /// @param _superchainConfig The address of the SuperchainConfig contract.
    function initialize(
        IDisputeGameFactory _disputeGameFactory,
        uint256 _disputeGameFinalityDelaySeconds,
        IFaultDisputeGame _authorizedGame,
        ISuperchainConfig _superchainConfig
    )
        external
        initializer
    {
        disputeGameFactory = _disputeGameFactory;
        disputeGameFinalityDelaySeconds = _disputeGameFinalityDelaySeconds;
        _anchorGame = _authorizedGame;
        superchainConfig = _superchainConfig;
    }

    /// @notice Returns the output root of the anchor game, or an authorized anchor state if no such game exists.
    function anchors(GameType /* unused */ ) public view returns (OutputRoot memory) {
        if (isGameBlacklisted[_anchorGame]) revert InvalidGame();
        return
            OutputRoot({ l2BlockNumber: _anchorGame.l2BlockNumber(), root: Hash.wrap(_anchorGame.rootClaim().raw()) });
    }

    function setAnchorState(IFaultDisputeGame _game) external {
        uint256 _anchorL2BlockNumber = _anchorGame.l2BlockNumber();
        uint256 _gameL2BlockNumber = _game.l2BlockNumber();
        if (!isGameValid(_game)) revert InvalidGame();
        if (_gameL2BlockNumber <= _anchorL2BlockNumber) {
            revert AnchorGameIsNewer(_anchorL2BlockNumber, _gameL2BlockNumber);
        }
        _anchorGame = _game;
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

    function isGameRetired(IFaultDisputeGame _game) public view returns (bool) {
        // Must be created after the gameRetirementTimestamp.
        return _game.createdAt().raw() <= gameRetirementTimestamp;
    }

    function isGameMaybeValid(IFaultDisputeGame _game) public view returns (bool) {
        // Grab the game and game data.
        (GameType gameType, Claim rootClaim, bytes memory extraData) = _game.gameData();

        // Grab the verified address of the game based on the game data.
        // slither-disable-next-line unused-return
        (IDisputeGame _factoryRegisteredGame,) =
            disputeGameFactory.games({ _gameType: gameType, _rootClaim: rootClaim, _extraData: extraData });

        // Must be a game created by the factory.
        if (address(_factoryRegisteredGame) != address(_game)) return false;

        // Must not be blacklisted.
        if (isGameBlacklisted[_game]) return false;

        // The game type of the dispute game must have been the respected game type when it was created.
        if (!_game.wasRespectedGameTypeWhenCreated()) return false;

        // Must be a game with a status other than `CHALLENGER_WINS`.
        if (_game.status() == GameStatus.CHALLENGER_WINS) {
            return false;
        }

        // Must be created after the gameRetirementTimestamp.
        if (isGameRetired(_game)) return false;

        return true;
    }

    function isGameValid(IFaultDisputeGame _game) public view returns (bool) {
        return isGameMaybeValid(_game) && isGameFinalized(_game);
    }

    function isGameFinalized(IFaultDisputeGame _game) public view returns (bool) {
        // Game status must be CHALLENGER_WINS or DEFENDER_WINS
        if (_game.status() != GameStatus.DEFENDER_WINS && _game.status() != GameStatus.CHALLENGER_WINS) {
            return false;
        }
        // Game resolvedAt timestamp must be non-zero
        // Game resolvedAt timestamp must be more than airgap period seconds ago
        uint256 _resolvedAt = _game.resolvedAt().raw();
        if (_resolvedAt == 0 || _resolvedAt <= block.timestamp - disputeGameFinalityDelaySeconds) {
            return false;
        }
        return true;
    }

    function _guardian() internal view returns (address) {
        return superchainConfig.guardian();
    }
}
