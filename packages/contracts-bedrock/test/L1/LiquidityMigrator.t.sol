// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { CommonTest } from "test/setup/CommonTest.sol";
import { LiquidityMigrator } from "src/L1/LiquidityMigrator.sol";

contract LiquidityMigratorTest is CommonTest {
    event ETHMigrated(uint256 amount);

    LiquidityMigrator public migrator;

    function setUp() public virtual override {
        super.enableInterop();
        super.setUp();
        migrator = new LiquidityMigrator(address(sharedLockbox));
    }

    /// @notice Tests the migration of the contract's ETH balance to the SharedLockbox works properly.
    function test_migrateETH_succeeds(uint256 _ethAmount) public {
        vm.deal(address(migrator), _ethAmount);

        // Get the balance of the migrator before the migration to compare later on the assertions
        uint256 _migratorEthBalance = address(migrator).balance;
        uint256 _lockboxBalanceBefore = address(sharedLockbox).balance;

        // Look for the emit of the `ETHMigrated` event
        emit ETHMigrated(_migratorEthBalance);

        // Set the migrator as an authorized portal so it can lock the ETH while migrating
        vm.prank(address(superchainConfig));
        sharedLockbox.authorizePortal(address(migrator));

        // Call the `migrateETH` function with the amount
        migrator.migrateETH();

        // Assert the balances after the migration happened
        assert(address(migrator).balance == 0);
        assert(address(sharedLockbox).balance == _lockboxBalanceBefore + _migratorEthBalance);
    }
}
