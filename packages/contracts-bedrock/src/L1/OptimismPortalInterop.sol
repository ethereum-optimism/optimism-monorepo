// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { OptimismPortal2 } from "src/L1/OptimismPortal2.sol";

// Libraries
import { Predeploys } from "src/libraries/Predeploys.sol";
import { Constants } from "src/libraries/Constants.sol";
import { Unauthorized } from "src/libraries/PortalErrors.sol";
import { Types } from "src/libraries/Types.sol";

// Interfaces
import { IL1BlockInterop, ConfigType } from "interfaces/L2/IL1BlockInterop.sol";
import { ISharedLockbox } from "interfaces/L1/ISharedLockbox.sol";
import { ISuperchainConfigInterop } from "interfaces/L1/ISuperchainConfigInterop.sol";

/// @notice Error thrown when attempting to use custom gas token specific actions.
error CustomGasTokenNotSupported();

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

    /// @custom:semver +interop-beta.10
    function version() public pure override returns (string memory) {
        return string.concat(super.version(), "+interop-beta.10");
    }

    /// @notice Sets static configuration options for the L2 system.
    /// @param _type  Type of configuration to set.
    /// @param _value Encoded value of the configuration.
    function setConfig(ConfigType _type, bytes memory _value) external {
        if (msg.sender != address(systemConfig)) revert Unauthorized();
        if (_type == ConfigType.SET_GAS_PAYING_TOKEN) revert CustomGasTokenNotSupported();

        // Set L2 deposit gas as used without paying burning gas. Ensures that deposits cannot use too much L2 gas.
        // This value must be large enough to cover the cost of calling `L1Block.setConfig`.
        useGas(SYSTEM_DEPOSIT_GAS_LIMIT);

        // Emit the special deposit transaction directly that sets the config in the L1Block predeploy contract.
        emit TransactionDeposited(
            Constants.DEPOSITOR_ACCOUNT,
            Predeploys.L1_BLOCK_ATTRIBUTES,
            DEPOSIT_VERSION,
            abi.encodePacked(
                uint256(0), // mint
                uint256(0), // value
                uint64(SYSTEM_DEPOSIT_GAS_LIMIT), // gasLimit
                false, // isCreation,
                abi.encodeCall(IL1BlockInterop.setConfig, (_type, _value))
            )
        );
    }

    /// @notice Getter for the address of the shared lockbox.
    function sharedLockbox() public view returns (ISharedLockbox) {
        return ISuperchainConfigInterop(address(superchainConfig)).sharedLockbox();
    }

    /// @notice Getter for the migrated flag.
    function migrated() external view returns (bool) {
        return _storage().migrated;
    }

    /// @notice Unlock and receive the ETH from the SharedLockbox.
    /// @param _tx Withdrawal transaction to finalize.
    function _unlockETH(Types.WithdrawalTransaction memory _tx) internal virtual override {
        // We don't allow the SharedLockbox to be the target of a withdrawal.
        // This is to prevent the SharedLockbox from being drained.
        // This check needs to be done for every withdrawal.
        if (_tx.target == address(sharedLockbox())) revert MessageTargetSharedLockbox();

        OptimismPortalStorage storage s = _storage();

        if (!s.migrated) return;
        if (_tx.value == 0) return;

        sharedLockbox().unlockETH(_tx.value);
    }

    /// @notice Locks the ETH in the SharedLockbox.
    function _lockETH() internal virtual override {
        if (msg.value == 0) return;

        OptimismPortalStorage storage s = _storage();
        if (s.migrated) sharedLockbox().lockETH{ value: msg.value }();
    }

    /// @notice Migrates the ETH liquidity to the SharedLockbox. This function will only be called once by the
    ///         SuperchainConfig when adding this chain to the dependency set.
    function migrateLiquidity() external {
        if (msg.sender != address(superchainConfig)) revert Unauthorized();

        OptimismPortalStorage storage s = _storage();
        s.migrated = true;

        uint256 ethBalance = address(this).balance;

        sharedLockbox().lockETH{ value: ethBalance }();

        emit ETHMigrated(ethBalance);
    }
}
