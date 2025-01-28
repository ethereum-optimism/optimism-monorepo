// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import {
    OptimismPortal2,
    IDisputeGameFactory,
    ISystemConfig,
    ISuperchainConfig,
    GameType
} from "src/L1/OptimismPortal2.sol";

// Libraries
import { Unauthorized } from "src/libraries/PortalErrors.sol";
import { Types } from "src/libraries/Types.sol";

// Interfaces
import { ISharedLockbox } from "interfaces/L1/ISharedLockbox.sol";
import { ISuperchainConfigInterop } from "interfaces/L1/ISuperchainConfigInterop.sol";

/// @custom:proxied true
/// @title OptimismPortalInterop
/// @notice The OptimismPortal is a low-level contract responsible for passing messages between L1
///         and L2. Messages sent directly to the OptimismPortal have no form of replayability.
///         Users are encouraged to use the L1CrossDomainMessenger for a higher-level interface.
contract OptimismPortalInterop is OptimismPortal2 {
    /// @notice Emitted when the contract migrates the ETH liquidity to the SharedLockbox.
    /// @param amount Amount of ETH migrated.
    event ETHMigrated(uint256 amount);

    /// @notice Error thrown when the withdrawal target is the SharedLockbox.
    error MessageTargetSharedLockbox();

    /// @notice Storage slot that the OptimismPortalStorage struct is stored at.
    /// keccak256(abi.encode(uint256(keccak256("optimismPortal.storage")) - 1)) & ~bytes32(uint256(0xff));
    bytes32 internal constant OPTIMISM_PORTAL_STORAGE_SLOT =
        0x554bed1aae13f6a1ca3b124bc567e2e458d6903a211d2d3a4ec21fca3b2b6c00;

    /// @notice Storage struct for the OptimismPortal specific storage data.
    /// @custom:storage-location erc7201:OptimismPortal.storage
    struct OptimismPortalStorage {
        /// @notice The address of the SharedLockbox.
        address sharedLockbox;
        /// @notice A flag indicating whether the contract has migrated the ETH liquidity to the SharedLockbox.
        bool migrated;
    }

    /// @notice Returns the storage for the OptimismPortalStorage.
    function _storage() private pure returns (OptimismPortalStorage storage storage_) {
        assembly {
            storage_.slot := OPTIMISM_PORTAL_STORAGE_SLOT
        }
    }

    constructor(
        uint256 _proofMaturityDelaySeconds,
        uint256 _disputeGameFinalityDelaySeconds
    )
        OptimismPortal2(_proofMaturityDelaySeconds, _disputeGameFinalityDelaySeconds)
    { }

    /// @custom:semver +interop-beta.12
    function version() public pure override returns (string memory) {
        return string.concat(super.version(), "+interop-beta.12");
    }

    /// @notice Initializer.
    /// @param _disputeGameFactory Contract of the DisputeGameFactory.
    /// @param _systemConfig Contract of the SystemConfig.
    /// @param _superchainConfig Contract of the SuperchainConfig.
    /// @param _initialRespectedGameType Initial game type to be respected.
    function initialize(
        IDisputeGameFactory _disputeGameFactory,
        ISystemConfig _systemConfig,
        ISuperchainConfig _superchainConfig,
        GameType _initialRespectedGameType
    )
        external
        override
        initializer
    {
        _initialize(_disputeGameFactory, _systemConfig, _superchainConfig, _initialRespectedGameType);

        OptimismPortalStorage storage s = _storage();
        s.sharedLockbox = address(ISuperchainConfigInterop(address(_superchainConfig)).sharedLockbox());
    }

    /// @notice Getter for the address of the shared lockbox.
    function sharedLockbox() external view returns (ISharedLockbox) {
        return ISharedLockbox(_storage().sharedLockbox);
    }

    /// @notice Getter for the migrated flag.
    function migrated() external view returns (bool) {
        return _storage().migrated;
    }

    /// @notice Unlock and receive the ETH from the SharedLockbox.
    /// @param _tx Withdrawal transaction to finalize.
    function _unlockETH(Types.WithdrawalTransaction memory _tx) internal virtual override {
        OptimismPortalStorage storage s = _storage();

        // We don't allow the SharedLockbox to be the target of a withdrawal.
        // This is to prevent the SharedLockbox from being drained.
        // This check needs to be done for every withdrawal.
        if (_tx.target == s.sharedLockbox) revert MessageTargetSharedLockbox();

        // If ETH liquidity has not been migrated to the SharedLockbox yet, maintain legacy behavior
        // where ETH accumulates in the portal contract itself rather than being managed by the lockbox
        if (!s.migrated) return;

        // Skip calling the lockbox if the withdrawal value is 0 since there is no ETH to unlock
        if (_tx.value == 0) return;

        ISharedLockbox(s.sharedLockbox).unlockETH(_tx.value);
    }

    /// @notice Locks the ETH in the SharedLockbox.
    function _lockETH() internal virtual override {
        // Skip calling the lockbox if the deposit value is 0 since there is no ETH to lock
        if (msg.value == 0) return;

        // If ETH liquidity has not been migrated to the SharedLockbox yet, maintain legacy behavior
        // where ETH accumulates in the portal contract itself rather than being managed by the lockbox
        OptimismPortalStorage storage s = _storage();
        if (!s.migrated) return;

        ISharedLockbox(s.sharedLockbox).lockETH{ value: msg.value }();
    }

    /// @notice Migrates the ETH liquidity to the SharedLockbox. This function will only be called once by the
    ///         SuperchainConfig when adding this chain to the dependency set.
    function migrateLiquidity() external {
        if (msg.sender != address(superchainConfig)) revert Unauthorized();

        OptimismPortalStorage storage s = _storage();
        s.migrated = true;

        uint256 ethBalance = address(this).balance;

        ISharedLockbox(s.sharedLockbox).lockETH{ value: ethBalance }();

        emit ETHMigrated(ethBalance);
    }
}
