// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { ISharedLockbox } from "interfaces/L1/ISharedLockbox.sol";

/// @title ILiquidityMigrator
/// @notice Interface for the LiquidityMigrator contract
interface ILiquidityMigrator {
    event ETHMigrated(uint256 amount);

    function __constructor__(address _sharedLockbox) external;

    function SHARED_LOCKBOX() external view returns (ISharedLockbox);

    function migrateETH() external;

    function version() external view returns (string memory);
}
