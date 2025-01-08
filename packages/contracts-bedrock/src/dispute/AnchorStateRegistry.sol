// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { Initializable } from "@openzeppelin/contracts/proxy/utils/Initializable.sol";

// Libraries
import { GameType, OutputRoot, Claim, GameStatus, Hash } from "src/dispute/lib/Types.sol";
import { Unauthorized } from "src/libraries/errors/CommonErrors.sol";
import {
    AnchorStateRegistry_AnchorGameIsNewer,
    AnchorStateRegistry_GameNotResolved,
    AnchorStateRegistry_GameMustWaitFinalityDelay,
    AnchorStateRegistry_GameNotFactoryRegistered,
    AnchorStateRegistry_GameBlacklisted,
    AnchorStateRegistry_GameRespectedGameTypeMismatch,
    AnchorStateRegistry_GameChallengerWins,
    AnchorStateRegistry_GameRetired
} from "src/dispute/lib/Errors.sol";

// Interfaces
import { ISemver } from "interfaces/universal/ISemver.sol";
import { IDisputeGame } from "interfaces/dispute/IDisputeGame.sol";
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
    event AnchorGameSet(IDisputeGame newAnchorGame);
    event RespectedGameTypeSet(GameType gameType);
    event DisputeGameBlacklisted(IDisputeGame game);
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

    /// @notice The delay between when a dispute game is resolved and when it can be considered finalized.
    uint256 public disputeGameFinalityDelaySeconds;

    uint64 public gameRetirementTimestamp;

    GameType public respectedGameType;

    IDisputeGame internal _anchorGame;

    OutputRoot internal _initialAnchorRoot;

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
        _initialAnchorRoot = _startingAnchorRoot;
        disputeGameFactory = _disputeGameFactory;
        disputeGameFinalityDelaySeconds = _disputeGameFinalityDelaySeconds;
        superchainConfig = _superchainConfig;
    }

    /// @notice Returns the output root of the anchor game, or an authorized anchor state if no such game exists.
    function anchors(GameType /* unused */ ) public view returns (OutputRoot memory) {
        return getAnchorState();
    }

    function getAnchorState() public view returns (OutputRoot memory) {
        if (_anchorGame == IDisputeGame(address(0))) return _initialAnchorRoot;
        if (isGameBlacklisted[_anchorGame]) revert AnchorStateRegistry_GameBlacklisted(_anchorGame);
        return
            OutputRoot({ l2BlockNumber: _anchorGame.l2BlockNumber(), root: Hash.wrap(_anchorGame.rootClaim().raw()) });
    }

    function setAnchorState(IDisputeGame _game) external {
        uint256 _anchorL2BlockNumber = _anchorGame.l2BlockNumber();
        uint256 _gameL2BlockNumber = _game.l2BlockNumber();
        assertGameValid(_game);
        if (_gameL2BlockNumber <= _anchorL2BlockNumber) {
            revert AnchorStateRegistry_AnchorGameIsNewer(_anchorL2BlockNumber, _gameL2BlockNumber);
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

    function isGameRetired(IDisputeGame _game) public view returns (bool) {
        // Must be created after the gameRetirementTimestamp.
        return _game.createdAt().raw() <= gameRetirementTimestamp;
    }

    function assertGameMaybeValid(IDisputeGame _game) public view returns (bool) {
        // Grab the game and game data.
        (GameType gameType, Claim rootClaim, bytes memory extraData) = _game.gameData();

        // Grab the verified address of the game based on the game data.
        // slither-disable-next-line unused-return
        (IDisputeGame _factoryRegisteredGame,) =
            disputeGameFactory.games({ _gameType: gameType, _rootClaim: rootClaim, _extraData: extraData });

        // Must be a game created by the factory.
        if (address(_factoryRegisteredGame) != address(_game)) {
            revert AnchorStateRegistry_GameNotFactoryRegistered(_game);
        }

        // Must not be blacklisted.
        if (isGameBlacklisted[_game]) revert AnchorStateRegistry_GameBlacklisted(_game);

        // The game type of the dispute game must have been the respected game type when it was created.
        if (!_game.wasRespectedGameTypeWhenCreated()) revert AnchorStateRegistry_GameRespectedGameTypeMismatch(_game);

        // Must be a game with a status other than `CHALLENGER_WINS`.
        if (_game.status() == GameStatus.CHALLENGER_WINS) {
            revert AnchorStateRegistry_GameChallengerWins(_game);
        }

        // Must be created after the gameRetirementTimestamp.
        if (isGameRetired(_game)) revert AnchorStateRegistry_GameRetired(_game);

        return true;
    }

    function assertGameValid(IDisputeGame _game) public view returns (bool) {
        return assertGameMaybeValid(_game) && assertGameFinalized(_game);
    }

    function assertGameFinalized(IDisputeGame _game) public view returns (bool) {
        // Game status must be CHALLENGER_WINS or DEFENDER_WINS
        if (_game.status() != GameStatus.DEFENDER_WINS && _game.status() != GameStatus.CHALLENGER_WINS) {
            revert AnchorStateRegistry_GameNotResolved(_game);
        }
        uint256 _resolvedAt = _game.resolvedAt().raw();
        // Game resolvedAt timestamp must be non-zero
        if (_resolvedAt == 0) {
            revert AnchorStateRegistry_GameNotResolved(_game);
        }
        // Game resolvedAt timestamp must be more than airgap period seconds ago
        if (block.timestamp - _resolvedAt <= disputeGameFinalityDelaySeconds) {
            revert AnchorStateRegistry_GameMustWaitFinalityDelay(_game);
        }
        return true;
    }

    function _guardian() internal view returns (address) {
        return superchainConfig.guardian();
    }
}
