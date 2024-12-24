// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { CommonTest } from "test/setup/CommonTest.sol";
import { LiquidityMigrator } from "src/L1/LiquidityMigrator.sol";
import { IProxyAdmin } from "interfaces/universal/IProxyAdmin.sol";

contract LiquidityMigratorTest is CommonTest {
    event ETHMigrated(uint256 amount);

    function setUp() public virtual override {
        super.enableInterop();
        super.setUp();
    }

    /// @notice Tests the migration of the contract's ETH balance to the SharedLockbox works properly.
    function test_migrateETH_succeeds(uint256 _ethAmount) public {
        vm.deal(address(liquidityMigrator), _ethAmount);

        // Get the balance of the migrator before the migration to compare later on the assertions
        uint256 migratorEthBalance = address(liquidityMigrator).balance;
        uint256 lockboxBalanceBefore = address(sharedLockbox).balance;

        // Set the migrator as an authorized portal so it can lock the ETH while migrating
        vm.prank(address(superchainConfig));
        sharedLockbox.authorizePortal(address(liquidityMigrator));

        // Look for the emit of the `ETHMigrated` event
        vm.expectEmit(address(liquidityMigrator));
        emit ETHMigrated(migratorEthBalance);

        // Call the `migrateETH` function with the amount
        liquidityMigrator.migrateETH();

        // Assert the balances after the migration happened
        assert(address(liquidityMigrator).balance == 0);
        assert(address(sharedLockbox).balance == lockboxBalanceBefore + migratorEthBalance);
    }

    /// @notice Tests the migration of the portal's ETH balance to the SharedLockbox works properly.
    function test_portal_migrateETH_succeeds(uint256 _ethAmount) public {
        vm.deal(address(optimismPortal2), _ethAmount);

        // Get the balance of the portal before the migration to compare later on the assertions
        uint256 portalEthBalance = address(optimismPortal2).balance;
        uint256 lockboxBalanceBefore = address(sharedLockbox).balance;

        // Get the proxy admin address and it's owner
        IProxyAdmin proxyAdmin = IProxyAdmin(deploy.mustGetAddress("ProxyAdmin"));
        address proxyAdminOwner = proxyAdmin.owner();

        // Look for the emit of the `ETHMigrated` event
        vm.expectEmit(address(optimismPortal2));
        emit ETHMigrated(portalEthBalance);

        // Update the portal proxy implementation to the LiquidityMigrator contract
        vm.prank(proxyAdminOwner);
        proxyAdmin.upgradeAndCall({
            _proxy: payable(optimismPortal2),
            _implementation: address(liquidityMigrator),
            _data: abi.encodeCall(LiquidityMigrator.migrateETH, ())
        });

        // Assert the balances after the migration happened
        assert(address(optimismPortal2).balance == 0);
        assert(address(sharedLockbox).balance == lockboxBalanceBefore + portalEthBalance);
    }
}
