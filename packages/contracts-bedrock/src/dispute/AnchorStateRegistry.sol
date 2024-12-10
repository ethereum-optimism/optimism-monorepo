// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { Initializable } from "@openzeppelin/contracts/proxy/utils/Initializable.sol";

// Libraries
import { GameType, OutputRoot, Claim, GameStatus, Hash } from "src/dispute/lib/Types.sol";
import { Unauthorized } from "src/libraries/errors/CommonErrors.sol";
import { UnregisteredGame, InvalidGameStatus } from "src/dispute/lib/Errors.sol";

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
    error LatestGameIsNewer(uint256 latestGameBlockNumber, uint256 candidateGameBlockNumber);

    event LatestValidGameSet(IDisputeGame game);
    event RespectedGameTypeSet(GameType _gameType);
    event DisputeGameBlacklisted(IDisputeGame game);

    /// @notice Semantic version.
    /// @custom:semver 2.0.1-beta.6
    string public constant version = "2.0.1-beta.6";

    /// @notice DisputeGameFactory address.
    IDisputeGameFactory internal immutable DISPUTE_GAME_FACTORY;

    /// @notice The delay between when a dispute game is resolved and when it can be considered finalized.
    uint256 internal immutable DISPUTE_GAME_FINALITY_DELAY_SECONDS;

    // / @notice Returns the anchor state for the given game type.
    // mapping(GameType => OutputRoot) public anchors;
    // TODO: upgrade?
    // uint256 __gap0;

    /// @notice Address of the SuperchainConfig contract.
    ISuperchainConfig public superchainConfig;

    /// @notice Returns whether a game is blacklisted.
    mapping(IDisputeGame => bool) isBlacklisted;

    uint64 public validityTimestamp;

    GameType public respectedGameType;

    IFaultDisputeGame[] public maybeValidGames;

    uint256 internal maybeValidGameIndex;

    IFaultDisputeGame public latestValidGame;

    /// @param _disputeGameFactory DisputeGameFactory address.
    constructor(IDisputeGameFactory _disputeGameFactory, uint256 _disputeGameFinalityDelaySeconds) {
        DISPUTE_GAME_FACTORY = _disputeGameFactory;
        DISPUTE_GAME_FINALITY_DELAY_SECONDS = _disputeGameFinalityDelaySeconds;
        _disableInitializers();
    }

    /// @notice Initializes the contract.
    /// @param _authorizedGame An authorized dispute game.
    /// @param _superchainConfig The address of the SuperchainConfig contract.
    /// @param _validityTimestamp The timestamp on which game validity is in part determined.
    function initialize(
        IFaultDisputeGame _authorizedGame,
        ISuperchainConfig _superchainConfig,
        uint64 _validityTimestamp
    )
        external
        initializer
    {
        latestValidGame = _authorizedGame;
        superchainConfig = _superchainConfig;
        validityTimestamp = _validityTimestamp;
    }

    /// @notice Returns the root claim of the result of get latest valid game or an authorized anchor state if no such
    /// game exists.
    function anchors(GameType /* unused */ ) public view returns (OutputRoot memory) {
        return OutputRoot({
            l2BlockNumber: latestValidGame.l2BlockNumber(),
            root: Hash.wrap(latestValidGame.rootClaim().raw())
        });
    }

    function registerMaybeValidGame() public {
        IFaultDisputeGame _game = IFaultDisputeGame(msg.sender);
        // game must not be invalid
        if (isGameInvalid(_game)) revert InvalidGame();
        _latestMaybeValidGame();
        maybeValidGames.push(_game);
    }

    function updateLatestValidGame(IFaultDisputeGame _game) public {
        uint256 _l2BlockNumber = latestValidGame.l2BlockNumber();
        uint256 _gameL2BlockNumber = _game.l2BlockNumber();
        if (_gameL2BlockNumber <= _l2BlockNumber) {
            revert LatestGameIsNewer(_l2BlockNumber, _gameL2BlockNumber);
        }
        if (!isGameValid(_game)) revert InvalidGame();
        latestValidGame = _game;
        emit LatestValidGameSet(_game);
    }

    /// @notice Callable by FaultDisputeGame contracts to update the anchor state. Pulls the anchor state directly from
    ///         the FaultDisputeGame contract and stores it in the registry if the new anchor state is valid and the
    ///         state is newer than the current anchor state.
    function tryUpdateAnchorState() external {
        uint256 _l2BlockNumber = latestValidGame.l2BlockNumber();
        registerMaybeValidGame();

        for (uint256 i = 0; i < 50; i++) {
            // If the game's l2BlockNumber is older than that of latestValidGame, seek ahead.
            IFaultDisputeGame _game = maybeValidGames[maybeValidGameIndex];
            if (_game.l2BlockNumber() < _l2BlockNumber || isGameInvalid(_game)) {
                maybeValidGameIndex++;
            } else {
                // If the game's l2BlockNumber is newer than that of latestValidGame, update the anchor state.
                // It would be more efficient gas-wise to do the update here, but for spec purposes we're exploring
                // calling the `updateLatestValidGame` method instead.
                updateLatestValidGame(_game);
                _l2BlockNumber = _game.l2BlockNumber();
                if (maybeValidGameIndex == maybeValidGames.length) break;
            }
        }
    }

    /// @notice Blacklists a dispute game. Should only be used in the event that a dispute game resolves incorrectly.
    /// @param _disputeGame Dispute game to blacklist.
    function setGameBlacklisted(IDisputeGame _disputeGame) external {
        if (msg.sender != _guardian()) revert Unauthorized();
        isBlacklisted[_disputeGame] = true;
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

    function _guardian() internal view returns (address) {
        return superchainConfig.guardian();
    }

    function isGameInvalid(IFaultDisputeGame _game) public view returns (bool) {
        // Grab the game and game data.
        (GameType gameType, Claim rootClaim, bytes memory extraData) = _game.gameData();

        // Grab the verified address of the game based on the game data.
        // slither-disable-next-line unused-return
        (IDisputeGame _factoryRegisteredGame,) =
            DISPUTE_GAME_FACTORY.games({ _gameType: gameType, _rootClaim: rootClaim, _extraData: extraData });

        // Must be a game created by the factory.
        if (address(_factoryRegisteredGame) != address(_game)) return true;

        // Must not be blacklisted.
        if (isBlacklisted[_game]) return true;

        // The game type of the dispute game must be the respected game type. This was also checked in
        // `proveWithdrawalTransaction`, but we check it again in case the respected game type has changed since
        // the withdrawal was proven.
        if (_game.gameType().raw() != respectedGameType.raw()) return true;

        // Must be created after the validityTimestamp.
        uint64 _createdAt = _game.createdAt().raw();
        if (_createdAt >= validityTimestamp) return true;

        // Must be a game that resolved in favor of the state.
        if (_game.status() != GameStatus.DEFENDER_WINS) {
            return true;
        }
        return false;
    }

    function _isGameFinalized(IFaultDisputeGame _game) internal view returns (bool) {
        // - Game status is CHALLENGER_WINS or DEFENDER_WINS
        if (_game.status() != GameStatus.DEFENDER_WINS && _game.status() != GameStatus.CHALLENGER_WINS) {
            return false;
        }
        // - Game resolvedAt timestamp is not zero
        // - Game resolvedAt timestamp is more than airgap period seconds ago
        uint256 _resolvedAt = _game.resolvedAt().raw();
        if (_resolvedAt == 0 || _resolvedAt <= block.timestamp - DISPUTE_GAME_FINALITY_DELAY_SECONDS) {
            return false;
        }

        return true;
    }

    function isGameValid(IFaultDisputeGame _game) public view returns (bool) {
        return !isGameInvalid(_game) && _isGameFinalized(_game);
    }

    function _latestMaybeValidGame() internal returns (IDisputeGame) {
        return maybeValidGames[maybeValidGameIndex - 1];
    }

    /// @notice Returns the dispute game finality delay.
    /// @return Finality delay in seconds.
    function disputeGameFinalityDelaySeconds() public view returns (uint256) {
        return DISPUTE_GAME_FINALITY_DELAY_SECONDS;
    }

    /// @notice Returns the DisputeGameFactory address.
    /// @return DisputeGameFactory address.
    function disputeGameFactory() external view returns (IDisputeGameFactory) {
        return DISPUTE_GAME_FACTORY;
    }
}
