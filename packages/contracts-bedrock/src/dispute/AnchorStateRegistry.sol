// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { Initializable } from "@openzeppelin/contracts/proxy/utils/Initializable.sol";

// Libraries
import { GameType, OutputRoot, Claim, GameStatus, Hash } from "src/dispute/lib/Types.sol";
import { Unauthorized } from "src/libraries/errors/CommonErrors.sol";
import { InvalidAnchorGame } from "src/dispute/lib/Errors.sol";

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
    /// @custom:semver 2.1.0-beta.1
    string public constant version = "2.1.0-beta.1";

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
    event AnchorNotUpdated(IFaultDisputeGame indexed game, string reason);

    /// @notice Emitted when an anchor state is updated.
    event AnchorUpdated(IFaultDisputeGame indexed game);

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
        return _game.gameType().raw() == portal.respectedGameType().raw();
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
        return _game.createdAt().raw() < portal.respectedGameTypeUpdatedAt();
    }

    /// @notice Determines whether a game resolved properly and the game was not subject to any
    ///         invalidation conditions. The root claim of a proper game IS NOT guaranteed to be
    ///         valid. The root claim of a proper game CAN BE incorrect and still be a proper game.
    /// @param _game The game to check.
    /// @return Whether the game is a proper game.
    /// @return Reason why the game is not a proper game.
    function isGameProper(IDisputeGame _game) public view returns (bool, string memory) {
        if (!isGameRegistered(_game)) {
            return (false, "game not factory registered");
        }

        // Must be respected game type.
        if (!isGameRespected(_game)) {
            return (false, "game type not respected");
        }

        // Must not be blacklisted.
        if (isGameBlacklisted(_game)) {
            return (false, "game blacklisted");
        }

        // Must be created after the gameRetirementTimestamp.
        if (isGameRetired(_game)) {
            return (false, "game retired");
        }

        return (true, "");
    }

    /// @notice Callable by FaultDisputeGame contracts to update the anchor state. Pulls the anchor state directly from
    ///         the FaultDisputeGame contract and stores it in the registry if the new anchor state is valid and the
    ///         state is newer than the current anchor state.
    function tryUpdateAnchorState() external {
        // Grab the game.
        IFaultDisputeGame game = IFaultDisputeGame(msg.sender);

        // Check if the game is a proper game.
        (bool isProper, string memory reason) = isGameProper(game);
        if (!isProper) {
            emit AnchorNotUpdated(game, reason);
            return;
        }

        // Must be a game that resolved in favor of the state.
        if (game.status() != GameStatus.DEFENDER_WINS) {
            emit AnchorNotUpdated(game, "game not defender wins");
            return;
        }

        // No need to update anything if the anchor state is already newer.
        (, uint256 anchorL2BlockNumber) = getAnchorRoot();
        if (game.l2BlockNumber() <= anchorL2BlockNumber) {
            emit AnchorNotUpdated(game, "game not newer than anchor state");
            return;
        }

        // Update the anchor game.
        anchorGame = game;
        emit AnchorUpdated(game);
    }

    /// @notice Sets the anchor state given the game.
    /// @param _game The game to set the anchor state for.
    function setAnchorState(IFaultDisputeGame _game) external {
        if (msg.sender != superchainConfig.guardian()) revert Unauthorized();

        // Check if the game is a proper game.
        (bool isProper, string memory reason) = isGameProper(_game);
        if (!isProper) {
            revert InvalidAnchorGame(reason);
        }

        // The game must have resolved in favor of the root claim.
        if (_game.status() != GameStatus.DEFENDER_WINS) {
            revert InvalidAnchorGame("game not defender wins");
        }

        // Update the anchor game.
        anchorGame = _game;
        emit AnchorUpdated(_game);
    }
}
