// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Safe
import { GnosisSafe as Safe } from "safe-contracts/GnosisSafe.sol";
import { Enum } from "safe-contracts/common/Enum.sol";

// Libraries
import { Unauthorized } from "src/libraries/PortalErrors.sol";
import { GameType, Timestamp } from "src/dispute/lib/Types.sol";

// Interfaces
import { IAnchorStateRegistry } from "src/dispute/interfaces/IAnchorStateRegistry.sol";
import { IFaultDisputeGame } from "src/dispute/interfaces/IFaultDisputeGame.sol";
import { ISuperchainConfig } from "src/L1/interfaces/ISuperchainConfig.sol";
import { IOptimismPortal2 } from "src/L1/interfaces/IOptimismPortal2.sol";
import { IDisputeGame } from "src/dispute/interfaces/IDisputeGame.sol";
import { ISemver } from "src/universal/interfaces/ISemver.sol";

/// @title DeputyGuardianModule
/// @notice This module is intended to be enabled on the Security Council Safe, which will own the Guardian role in the
///         SuperchainConfig contract. The DeputyGuardianModule should allow a Deputy Guardian to administer any of the
///         actions that the Guardian is authorized to take. The security council can revoke the Deputy Guardian's
///         authorization at any time by disabling this module.
contract DeputyGuardianModule is ISemver {
    /// @notice Error message for failed transaction execution
    error ExecutionFailed(string);

    /// @notice Emitted when the SuperchainConfig is paused
    event Paused(string identifier);

    /// @notice Emitted when the SuperchainConfig is unpaused
    event Unpaused();

    /// @notice Emitted when a DisputeGame is blacklisted
    event DisputeGameBlacklisted(IDisputeGame indexed game);

    /// @notice Emitted when the respected game type is set
    event RespectedGameTypeSet(GameType indexed gameType, Timestamp indexed updatedAt);

    /// @notice The Safe contract instance
    Safe internal immutable SAFE;

    /// @notice The SuperchainConfig's address
    ISuperchainConfig internal immutable SUPERCHAIN_CONFIG;

    /// @notice The deputy guardian's address
    address internal immutable DEPUTY_GUARDIAN;

    /// @notice Semantic version.
    /// @custom:semver 2.0.1-beta.4
    string public constant version = "2.0.1-beta.4";

    // Constructor to initialize the Safe and baseModule instances
    constructor(Safe _safe, ISuperchainConfig _superchainConfig, address _deputyGuardian) {
        SAFE = _safe;
        SUPERCHAIN_CONFIG = _superchainConfig;
        DEPUTY_GUARDIAN = _deputyGuardian;
    }

    /// @notice Getter function for the Safe contract instance
    /// @return safe_ The Safe contract instance
    function safe() public view returns (Safe safe_) {
        safe_ = SAFE;
    }

    /// @notice Getter function for the SuperchainConfig's address
    /// @return superchainConfig_ The SuperchainConfig's address
    function superchainConfig() public view returns (ISuperchainConfig superchainConfig_) {
        superchainConfig_ = SUPERCHAIN_CONFIG;
    }

    /// @notice Getter function for the deputy guardian's address
    /// @return deputyGuardian_ The deputy guardian's address
    function deputyGuardian() public view returns (address deputyGuardian_) {
        deputyGuardian_ = DEPUTY_GUARDIAN;
    }

    /// @notice Internal function to ensure that only the deputy guardian can call certain functions.
    function _onlyDeputyGuardian() internal view {
        if (msg.sender != DEPUTY_GUARDIAN) {
            revert Unauthorized();
        }
    }

    /// @notice Calls the Security Council Safe's `execTransactionFromModuleReturnData()`, with the arguments
    ///      necessary to call `pause()` on the `SuperchainConfig` contract.
    ///      Only the deputy guardian can call this function.
    function pause() external {
        _onlyDeputyGuardian();

        bytes memory data = abi.encodeCall(SUPERCHAIN_CONFIG.pause, ("Deputy Guardian"));
        (bool success, bytes memory returnData) =
            SAFE.execTransactionFromModuleReturnData(address(SUPERCHAIN_CONFIG), 0, data, Enum.Operation.Call);
        if (!success) {
            revert ExecutionFailed(string(returnData));
        }
        emit Paused("Deputy Guardian");
    }

    /// @notice Calls the Security Council Safe's `execTransactionFromModuleReturnData()`, with the arguments
    ///      necessary to call `unpause()` on the `SuperchainConfig` contract.
    ///      Only the deputy guardian can call this function.
    function unpause() external {
        _onlyDeputyGuardian();

        bytes memory data = abi.encodeCall(SUPERCHAIN_CONFIG.unpause, ());
        (bool success, bytes memory returnData) =
            SAFE.execTransactionFromModuleReturnData(address(SUPERCHAIN_CONFIG), 0, data, Enum.Operation.Call);
        if (!success) {
            revert ExecutionFailed(string(returnData));
        }
        emit Unpaused();
    }

    /// @notice Calls the Security Council Safe's `execTransactionFromModuleReturnData()`, with the arguments
    ///      necessary to call `setAnchorState()` on the `AnchorStateRegistry` contract.
    ///      Only the deputy guardian can call this function.
    /// @param _registry The `IAnchorStateRegistry` contract instance.
    /// @param _game The `IFaultDisputeGame` contract instance.
    function setAnchorState(IAnchorStateRegistry _registry, IFaultDisputeGame _game) external {
        _onlyDeputyGuardian();

        bytes memory data = abi.encodeCall(IAnchorStateRegistry.setAnchorState, (_game));
        (bool success, bytes memory returnData) =
            SAFE.execTransactionFromModuleReturnData(address(_registry), 0, data, Enum.Operation.Call);
        if (!success) {
            revert ExecutionFailed(string(returnData));
        }
    }

    /// @notice Calls the Security Council Safe's `execTransactionFromModuleReturnData()`, with the arguments
    ///      necessary to call `blacklistDisputeGame()` on the `OptimismPortal2` contract.
    ///      Only the deputy guardian can call this function.
    /// @param _portal The `OptimismPortal2` contract instance.
    /// @param _game The `IDisputeGame` contract instance.
    function blacklistDisputeGame(IOptimismPortal2 _portal, IDisputeGame _game) external {
        _onlyDeputyGuardian();

        bytes memory data = abi.encodeCall(IOptimismPortal2.blacklistDisputeGame, (_game));
        (bool success, bytes memory returnData) =
            SAFE.execTransactionFromModuleReturnData(address(_portal), 0, data, Enum.Operation.Call);
        if (!success) {
            revert ExecutionFailed(string(returnData));
        }
        emit DisputeGameBlacklisted(_game);
    }

    /// @notice Calls the Security Council Safe's `execTransactionFromModuleReturnData()`, with the arguments
    ///      necessary to call `setRespectedGameType()` on the `OptimismPortal2` contract.
    ///      Only the deputy guardian can call this function.
    /// @param _portal The `OptimismPortal2` contract instance.
    /// @param _gameType The `GameType` to set as the respected game type.
    function setRespectedGameType(IOptimismPortal2 _portal, GameType _gameType) external {
        _onlyDeputyGuardian();

        bytes memory data = abi.encodeCall(IOptimismPortal2.setRespectedGameType, (_gameType));
        (bool success, bytes memory returnData) =
            SAFE.execTransactionFromModuleReturnData(address(_portal), 0, data, Enum.Operation.Call);
        if (!success) {
            revert ExecutionFailed(string(returnData));
        }
        emit RespectedGameTypeSet(_gameType, Timestamp.wrap(uint64(block.timestamp)));
    }
}
