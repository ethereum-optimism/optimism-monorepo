// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { ISharedLockbox } from "./interfaces/ISharedLockbox.sol";

/// @custom:proxied true
/// @title LiquidityMigrator
/// @notice A contract to migrate the OptimisPortal's ETH balance to the SharedLockbox. One-time use logic, executed in
/// a batch of transactions to enable the SharedLockbox interaction within the OptimismPortal.
contract LiquidityMigrator {
    /// @notice Emitted when the contract's ETH balance is migrated to the SharedLockbox.
    /// @param amount The amount corresponding to the contract's ETH balance migrated.
    event ETHMigrated(uint256 amount);

    /// @notice The SharedLockbox contract.
    ISharedLockbox internal immutable SHARED_LOCKBOX;

    /// @notice Constructs the LiquidityMigrator contract.
    /// @param _sharedLockbox The address of the SharedLockbox contract.
    constructor(address _sharedLockbox) {
        SHARED_LOCKBOX = ISharedLockbox(_sharedLockbox);
    }

    /// @notice Migrates the contract's whole ETH balance to the SharedLockbox.
    ///         One-time use logic upgraded over OptimismPortalProxy address and then deprecated by another approval.
    function migrateETH() external {
        uint256 balance = address(this).balance;
        SHARED_LOCKBOX.lockETH{ value: balance }();
        emit ETHMigrated(balance);
    }
}
