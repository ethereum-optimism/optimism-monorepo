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
    IDisputeGameFactory public disputeGameFactory;

    /// @notice The delay between when a dispute game is resolved and when it can be considered finalized.
    uint256 public disputeGameFinalityDelaySeconds;

    // TODO: storage slot issue
    // / @notice Returns the anchor state for the given game type.
    // mapping(GameType => OutputRoot) public anchors;

    // TODO: ^^
    // uint256 __gap0;

    /// @notice Address of the SuperchainConfig contract.
    ISuperchainConfig public superchainConfig;

    /// @notice Returns whether a game is blacklisted.
    mapping(IDisputeGame => bool) public isGameBlacklisted;

    uint64 public gameRetirementTimestamp;

    GameType public respectedGameType;

    IFaultDisputeGame[] public anchorGameCandidates;

    uint256 internal _anchorGameCandidateIndex;

    IFaultDisputeGame internal _anchorGame;

    uint256 public tryUpdateAnchorStateGas;

    constructor() {
        _disableInitializers();
    }

    /// @notice Initializes the contract.
    /// @param _disputeGameFactory DisputeGameFactory address.
    /// @param _disputeGameFinalityDelaySeconds The delay between when a dispute game is resolved and when it can be
    /// considered finalized.
    /// @param _authorizedGame An authorized dispute game.
    /// @param _superchainConfig The address of the SuperchainConfig contract.
    /// @param _tryUpdateAnchorStateGas The approximate gas limit for the tryUpdateAnchorState loop.
    function initialize(
        IDisputeGameFactory _disputeGameFactory,
        uint256 _disputeGameFinalityDelaySeconds,
        IFaultDisputeGame _authorizedGame,
        ISuperchainConfig _superchainConfig,
        uint256 _tryUpdateAnchorStateGas
    )
        external
        initializer
    {
        disputeGameFactory = _disputeGameFactory;
        disputeGameFinalityDelaySeconds = _disputeGameFinalityDelaySeconds;
        _anchorGame = _authorizedGame;
        superchainConfig = _superchainConfig;
        tryUpdateAnchorStateGas = _tryUpdateAnchorStateGas;
    }

    /// @notice Returns the output root of the anchor game, or an authorized anchor state if no such game exists.
    function anchors(GameType /* unused */ ) public view returns (OutputRoot memory) {
        if (isGameBlacklisted[_anchorGame]) revert InvalidGame();
        return
            OutputRoot({ l2BlockNumber: _anchorGame.l2BlockNumber(), root: Hash.wrap(_anchorGame.rootClaim().raw()) });
    }

    function pokeAnchorState(uint256 _candidateGameIndex) external {
        IFaultDisputeGame _game = anchorGameCandidates[_candidateGameIndex];
        _anchorGameCandidateIndex = _candidateGameIndex + 1;
        _updateAnchorState(_game);
    }

    function _updateAnchorState(IFaultDisputeGame _game) internal {
        uint256 _anchorL2BlockNumber = _anchorGame.l2BlockNumber();
        uint256 _gameL2BlockNumber = _game.l2BlockNumber();
        if (!isGameValid(_game)) revert InvalidGame();
        if (_gameL2BlockNumber <= _anchorL2BlockNumber) {
            revert AnchorGameIsNewer(_anchorL2BlockNumber, _gameL2BlockNumber);
        }
        _updateAnchorStateWithValidNewerGame(_game);
    }

    function _updateAnchorStateWithValidNewerGame(IFaultDisputeGame _game) internal {
        _anchorGame = _game;
        emit AnchorGameSet(_game);
    }

    function _maybeRegisterAnchorGameCandidate() internal {
        IFaultDisputeGame _game = IFaultDisputeGame(msg.sender);
        // game must not be invalid
        if (!isGameMaybeValid(_game)) return;
        // if the game is older than the anchor game, we don't need it
        if (_game.l2BlockNumber() < _anchorGame.l2BlockNumber()) {
            return;
        }
        anchorGameCandidates.push(_game);
    }

    /// @notice Callable by FaultDisputeGame contracts to update the anchor state.
    function tryUpdateAnchorState() external {
        _maybeRegisterAnchorGameCandidate();
        uint256 _anchorGameBlockNumber = _anchorGame.l2BlockNumber();
        uint256 _gasStart = gasleft();
        // TODO: add padding to ensure we don't run out of gas
        while (_gasStart - gasleft() < tryUpdateAnchorStateGas) {
            // if there are no candidates to consider, break
            if (_anchorGameCandidateIndex == anchorGameCandidates.length) break;
            // If the game's l2BlockNumber is older than that of anchorGame, seek ahead.
            IFaultDisputeGame _anchorCandidate = anchorGameCandidates[_anchorGameCandidateIndex];
            if (_anchorCandidate.l2BlockNumber() <= _anchorGameBlockNumber) {
                // We can confidently seek past games that don't increase the anchor game l2BlockNumber.
                _anchorGameCandidateIndex++;
            } else {
                if (_isGameFinalized(_anchorCandidate)) {
                    // If the game is finalized but invalid, we should move past it
                    if (!isGameValid(_anchorCandidate)) {
                        _anchorGameCandidateIndex++;
                    } else {
                        // If the game is finalized and valid, let's use it.
                        _updateAnchorStateWithValidNewerGame(_anchorCandidate);
                        _anchorGameCandidateIndex++;
                        _anchorGameBlockNumber = _anchorCandidate.l2BlockNumber();
                    }
                } else {
                    // If the game is not finalized, we could consider checking some games ahead of it, but for draft
                    // impl we'll pause.
                    break;
                }
            }
        }
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
        if (_game.createdAt().raw() <= gameRetirementTimestamp) return false;

        return true;
    }

    function isGameValid(IFaultDisputeGame _game) public view returns (bool) {
        return isGameMaybeValid(_game) && _isGameFinalized(_game);
    }

    function _isGameFinalized(IFaultDisputeGame _game) internal view returns (bool) {
        // - Game status is CHALLENGER_WINS or DEFENDER_WINS
        if (_game.status() != GameStatus.DEFENDER_WINS && _game.status() != GameStatus.CHALLENGER_WINS) {
            return false;
        }
        // - Game resolvedAt timestamp must be non-zero
        // - Game resolvedAt timestamp must be more than airgap period seconds ago
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
