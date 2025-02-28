// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing utilities
import { Constants } from "src/libraries/Constants.sol";

// Interfaces
import { IOptimismPortal2 as IOptimismPortal } from "interfaces/L1/IOptimismPortal2.sol";

import { IETHLockbox } from "interfaces/L1/IETHLockbox.sol";

import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { IPAOBase } from "interfaces/L1/IPAOBase.sol";

// Test
import { CommonTest } from "test/setup/CommonTest.sol";

import { Predeploys } from "src/libraries/Predeploys.sol";
import { ProxyAdmin } from "src/universal/ProxyAdmin.sol";

contract ETHLockboxTest is CommonTest {
    error InvalidInitialization();

    event ETHLocked(address indexed portal, uint256 amount);
    event ETHUnlocked(address indexed portal, uint256 amount);
    event PortalAuthorized(address indexed portal);
    event LockboxAuthorized(address indexed lockbox);
    event LiquidityMigrated(address indexed lockbox);
    event LiquidityReceived(address indexed lockbox);

    ProxyAdmin public proxyAdmin = ProxyAdmin(Predeploys.PROXY_ADMIN);
    address public PAO;

    function setUp() public virtual override {
        super.setUp();
        PAO = proxyAdmin.owner();
    }

    /// @notice Tests the superchain config was correctly set during initialization.
    function test_initialization_succeeds() public view {
        assertEq(address(ethLockbox.superchainConfig()), address(superchainConfig));
        assertEq(ethLockbox.authorizedPortals(address(optimismPortal2)), true);
    }

    /// @notice Tests it reverts when the contract is already initialized.
    function test_initialize_alreadyInitialized_reverts() public {
        vm.expectRevert("Initializable: contract is already initialized");
        address[] memory _portals = new address[](1);
        ethLockbox.initialize(address(superchainConfig), _portals);
    }

    /// @notice Tests the proxy admin owner is correctly returned.
    function test_proxyPAO_succeeds() public view {
        assertEq(ethLockbox.PAO(), PAO);
    }

    /// @notice Tests the paused status is correctly returned.
    function test_paused_succeeds() public {
        // Assert the paused status is false
        assertEq(ethLockbox.paused(), false);

        // Mock the superchain config to return true for the paused status
        vm.mockCall(address(superchainConfig), abi.encodeCall(ISuperchainConfig.paused, ()), abi.encode(true));

        // Assert the paused status is true
        assertEq(ethLockbox.paused(), true);
    }

    /// @notice Tests the liquidity is correctly received.
    function testFuzz_receiveLiquidity_succeeds(address _lockbox, uint256 _value) public {
        vm.assume(!ethLockbox.authorizedLockboxes(_lockbox));

        // Deal the value to the lockbox
        deal(address(_lockbox), _value);

        // Mock the admin owner of the lockbox to be the same as the current lockbox proxy admin owner
        vm.mockCall(address(_lockbox), abi.encodeCall(IPAOBase.PAO, ()), abi.encode(proxyAdmin.owner()));

        // Authorize the lockbox
        vm.prank(PAO);
        ethLockbox.authorizeLockbox(_lockbox);

        // Get the balance of the lockbox before the receive
        uint256 _lockboxBalanceBefore = address(ethLockbox).balance;

        // Expect the `LiquidityReceived` event to be emitted
        vm.expectEmit(address(ethLockbox));
        emit LiquidityReceived(_lockbox);

        // Call the `receiveLiquidity` function
        vm.prank(address(_lockbox));
        ethLockbox.receiveLiquidity{ value: _value }();

        // Assert the lockbox's balance increased by the amount received
        assertEq(address(ethLockbox).balance, _lockboxBalanceBefore + _value);
    }

    /// @notice Tests it reverts when the caller is not an authorized portal.
    function testFuzz_lockETH_unauthorizedPortal_reverts(address _caller) public {
        vm.assume(!ethLockbox.authorizedPortals(_caller));

        // Expect the revert with `Unauthorized` selector
        vm.expectRevert(IETHLockbox.ETHLockbox_Unauthorized.selector);

        // Call the `lockETH` function with an unauthorized caller
        vm.prank(_caller);
        ethLockbox.lockETH();
    }

    /// @notice Tests the ETH is correctly locked when the caller is an authorized portal.
    function testFuzz_lockETH_succeeds(uint256 _amount) public {
        // Deal the ETH amount to the portal
        vm.deal(address(optimismPortal2), _amount);

        // Get the balance of the portal and lockbox before the lock to compare later on the assertions
        uint256 _portalBalanceBefore = address(optimismPortal2).balance;
        uint256 _lockboxBalanceBefore = address(ethLockbox).balance;

        // Look for the emit of the `ETHLocked` event
        vm.expectEmit(address(ethLockbox));
        emit ETHLocked(address(optimismPortal2), _amount);

        // Call the `lockETH` function with the portal
        vm.prank(address(optimismPortal2));
        ethLockbox.lockETH{ value: _amount }();

        // Assert the portal's balance decreased and the lockbox's balance increased by the amount locked
        assertEq(address(optimismPortal2).balance, _portalBalanceBefore - _amount);
        assertEq(address(ethLockbox).balance, _lockboxBalanceBefore + _amount);
    }

    /// @notice Tests the ETH is correctly locked when the caller is an authorized portal with different portals.
    function testFuzz_lockETH_multiplePortals_succeeds(address _portal, uint256 _amount) public {
        vm.assume(_portal != address(ethLockbox));

        // Mock the admin owner of the portal to be the same as the current lockbox proxy admin owner
        vm.mockCall(address(_portal), abi.encodeCall(IPAOBase.PAO, ()), abi.encode(proxyAdmin.owner()));

        // Set the portal as an authorized portal
        vm.prank(PAO);
        ethLockbox.authorizePortal(_portal);

        // Deal the ETH amount to the portal
        vm.deal(_portal, _amount);

        // Get the balance of the portal and lockbox before the lock to compare later on the assertions
        uint256 _portalBalanceBefore = address(_portal).balance;
        uint256 _lockboxBalanceBefore = address(ethLockbox).balance;

        // Look for the emit of the `ETHLocked` event
        vm.expectEmit(address(ethLockbox));
        emit ETHLocked(_portal, _amount);

        // Call the `lockETH` function with the portal
        vm.prank(_portal);
        ethLockbox.lockETH{ value: _amount }();

        // Assert the portal's balance decreased and the lockbox's balance increased by the amount locked
        assertEq(address(_portal).balance, _portalBalanceBefore - _amount);
        assertEq(address(ethLockbox).balance, _lockboxBalanceBefore + _amount);
    }

    /// @notice Tests `unlockETH` reverts when the contract is paused.
    function testFuzz_unlockETH_paused_reverts(address _caller, uint256 _value) public {
        // Mock the superchain config to return true for the paused status
        vm.mockCall(address(superchainConfig), abi.encodeCall(ISuperchainConfig.paused, ()), abi.encode(true));

        // Expect the revert with `Paused` selector
        vm.expectRevert(IETHLockbox.ETHLockbox_Paused.selector);

        // Call the `unlockETH` function with the caller
        vm.prank(_caller);
        ethLockbox.unlockETH(_value);
    }

    /// @notice Tests it reverts when the caller is not an authorized portal.
    function testFuzz_unlockETH_unauthorizedPortal_reverts(address _caller, uint256 _value) public {
        vm.assume(!ethLockbox.authorizedPortals(_caller));

        // Expect the revert with `Unauthorized` selector
        vm.expectRevert(IETHLockbox.ETHLockbox_Unauthorized.selector);

        // Call the `unlockETH` function with an unauthorized caller
        vm.prank(_caller);
        ethLockbox.unlockETH(_value);
    }

    /// @notice Tests `unlockETH` reverts when the portal is not the L2 sender to prevent unlocking ETH from the lockbox
    ///         through a withdrawal transaction.
    function testFuzz_unlockETH_withdrawalTransaction_reverts(uint256 _value, address _l2Sender) public {
        vm.assume(_l2Sender != Constants.DEFAULT_L2_SENDER);

        // Mock the L2 sender
        vm.mockCall(address(optimismPortal2), abi.encodeCall(IOptimismPortal.l2Sender, ()), abi.encode(_l2Sender));

        // Expect the revert with `NoWithdrawalTransactions` selector
        vm.expectRevert(IETHLockbox.ETHLockbox_NoWithdrawalTransactions.selector);

        // Call the `unlockETH` function with the portal
        vm.prank(address(optimismPortal2));
        ethLockbox.unlockETH(_value);
    }

    /// @notice Tests the ETH is correctly unlocked when the caller is an authorized portal.
    function testFuzz_unlockETH_succeeds(uint256 _value) public {
        // Deal the ETH amount to the lockbox
        vm.deal(address(ethLockbox), _value);

        // Get the balance of the portal and lockbox before the unlock to compare later on the assertions
        uint256 portalBalanceBefore = address(optimismPortal2).balance;
        uint256 lockboxBalanceBefore = address(ethLockbox).balance;

        // Expect `donateETH` function to be called on Portal
        vm.expectCall(address(optimismPortal2), abi.encodeCall(IOptimismPortal.donateETH, ()));

        // Look for the emit of the `ETHUnlocked` event
        vm.expectEmit(address(ethLockbox));
        emit ETHUnlocked(address(optimismPortal2), _value);

        // Call the `unlockETH` function with the portal
        vm.prank(address(optimismPortal2));
        ethLockbox.unlockETH(_value);

        // Assert the portal's balance increased and the lockbox's balance decreased by the amount unlocked
        assertEq(address(optimismPortal2).balance, portalBalanceBefore + _value);
        assertEq(address(ethLockbox).balance, lockboxBalanceBefore - _value);
    }

    /// @notice Tests the ETH is correctly unlocked when the caller is an authorized portal.
    function testFuzz_unlockETH_multiplePortals_succeeds(address _portal, uint256 _value) public {
        vm.assume(_portal != address(ethLockbox));

        // Mock the admin owner of the portal to be the same as the current lockbox proxy admin owner
        vm.mockCall(address(_portal), abi.encodeCall(IPAOBase.PAO, ()), abi.encode(proxyAdmin.owner()));

        // Set the portal as an authorized portal
        vm.prank(PAO);
        ethLockbox.authorizePortal(_portal);

        // Deal the ETH amount to the lockbox
        vm.deal(address(ethLockbox), _value);

        // Get the balance of the portal and lockbox before the unlock to compare later on the assertions
        uint256 portalBalanceBefore = address(optimismPortal2).balance;
        uint256 lockboxBalanceBefore = address(ethLockbox).balance;

        // Expect `donateETH` function to be called on Portal
        vm.expectCall(address(optimismPortal2), abi.encodeCall(IOptimismPortal.donateETH, ()));

        // Look for the emit of the `ETHUnlocked` event
        vm.expectEmit(address(ethLockbox));
        emit ETHUnlocked(address(optimismPortal2), _value);

        // Call the `unlockETH` function with the portal
        vm.prank(address(optimismPortal2));
        ethLockbox.unlockETH(_value);

        // Assert the portal's balance increased and the lockbox's balance decreased by the amount unlocked
        assertEq(address(optimismPortal2).balance, portalBalanceBefore + _value);
        assertEq(address(ethLockbox).balance, lockboxBalanceBefore - _value);
    }

    /// @notice Tests the `authorizePortal` function reverts when the caller is not the proxy admin.
    function testFuzz_authorizePortal_unauthorized_reverts(address _caller) public {
        vm.assume(_caller != proxyAdmin.owner());

        // Expect the revert with `Unauthorized` selector
        vm.expectRevert(IETHLockbox.ETHLockbox_Unauthorized.selector);

        // Call the `authorizePortal` function with an unauthorized caller
        vm.prank(_caller);
        ethLockbox.authorizePortal(address(optimismPortal2));
    }

    /// @notice Tests the `authorizePortal` function reverts when the portal is already authorized.
    function testFuzz_authorizePortal_alreadyAuthorized_reverts(address _portal) public {
        // Authorize the portal
        if (!ethLockbox.authorizedPortals(_portal)) {
            // Mock the admin owner of the portal to be the same as the current lockbox proxy admin owner
            vm.mockCall(address(_portal), abi.encodeCall(IPAOBase.PAO, ()), abi.encode(proxyAdmin.owner()));

            // Authorize the portal
            vm.prank(PAO);
            ethLockbox.authorizePortal(_portal);
        }

        // Expect the revert with `AlreadyAuthorized` selector
        vm.expectRevert(IETHLockbox.ETHLockbox_AlreadyAuthorized.selector);

        // Call the `authorizePortal` function with the portal
        vm.prank(PAO);
        ethLockbox.authorizePortal(_portal);
    }

    /// @notice Tests the `authorizePortal` function reverts when the PAO of the portal is not the same as the PAO of
    ///         the lockbox.
    function testFuzz_authorizePortal_differentPAO_reverts(address _portal) public {
        vm.mockCall(address(_portal), abi.encodeCall(IPAOBase.PAO, ()), abi.encode(address(0)));

        // Expect the revert with `DifferentOwner` selector
        vm.expectRevert(IETHLockbox.ETHLockbox_DifferentPAO.selector);

        // Call the `authorizePortal` function
        vm.prank(PAO);
        ethLockbox.authorizePortal(_portal);
    }

    /// @notice Tests the `authorizeLockbox` function succeeds using the `optimismPortal2` address as the portal.
    function test_authorizePortal_succeeds() public {
        // Calculate the correct storage slot for the mapping value
        bytes32 mappingSlot = bytes32(uint256(1)); // position on the layout
        address key = address(optimismPortal2);
        bytes32 slot = keccak256(abi.encode(key, mappingSlot));

        // Reset the authorization status to false
        vm.store(address(ethLockbox), slot, bytes32(0));

        // Expect the `PortalAuthorized` event to be emitted
        vm.expectEmit(address(ethLockbox));
        emit PortalAuthorized(address(optimismPortal2));

        // Call the `authorizePortal` function with the portal
        vm.prank(PAO);
        ethLockbox.authorizePortal(address(optimismPortal2));

        // Assert the portal is authorized
        assertTrue(ethLockbox.authorizedPortals(address(optimismPortal2)));
    }

    /// @notice Tests the `authorizeLockbox` function succeeds
    function testFuzz_authorizePortal_succeeds(address _portal) public {
        vm.assume(!ethLockbox.authorizedPortals(_portal));

        // Mock the admin owner of the portal to be the same as the current lockbox proxy admin owner
        vm.mockCall(address(_portal), abi.encodeCall(IPAOBase.PAO, ()), abi.encode(proxyAdmin.owner()));

        // Expect the `PortalAuthorized` event to be emitted
        vm.expectEmit(address(ethLockbox));
        emit PortalAuthorized(_portal);

        // Call the `authorizePortal` function with the portal
        vm.prank(PAO);
        ethLockbox.authorizePortal(_portal);

        // Assert the portal is authorized
        assertTrue(ethLockbox.authorizedPortals(_portal));
    }

    /// @notice Tests the `authorizeLockbox` function reverts when the caller is not the proxy admin.
    function testFuzz_authorizeLockbox_unauthorized_reverts(address _caller) public {
        vm.assume(_caller != proxyAdmin.owner());

        // Expect the revert with `Unauthorized` selector
        vm.expectRevert(IETHLockbox.ETHLockbox_Unauthorized.selector);

        // Call the `authorizeLockbox` function with an unauthorized caller
        vm.prank(_caller);
        ethLockbox.authorizeLockbox(address(optimismPortal2));
    }

    /// @notice Tests the `authorizeLockbox` function reverts when the lockbox is already authorized.
    function testFuzz_authorizeLockbox_alreadyAuthorized_reverts(address _lockbox) public {
        // Authorize the lockbox
        if (!ethLockbox.authorizedLockboxes(_lockbox)) {
            vm.mockCall(address(_lockbox), abi.encodeCall(IPAOBase.PAO, ()), abi.encode(proxyAdmin.owner()));

            vm.prank(PAO);
            ethLockbox.authorizeLockbox(_lockbox);
        }

        // Expect the revert with `AlreadyAuthorized` selector
        vm.expectRevert(IETHLockbox.ETHLockbox_AlreadyAuthorized.selector);

        // Call the `authorizeLockbox` function with the lockbox
        vm.prank(PAO);
        ethLockbox.authorizeLockbox(_lockbox);
    }

    /// @notice Tests the `authorizeLockbox` function reverts when the PAO of the lockbox is not the same as the PAO of
    ///         the proxy admin.
    function testFuzz_authorizeLockbox_differentPAO_reverts(address _lockbox) public {
        vm.mockCall(address(_lockbox), abi.encodeCall(IPAOBase.PAO, ()), abi.encode(address(0)));

        // Expect the revert with `ETHLockbox_DifferentPAO` selector
        vm.expectRevert(IETHLockbox.ETHLockbox_DifferentPAO.selector);

        // Call the `authorizeLockbox` function with the lockbox
        vm.prank(PAO);
        ethLockbox.authorizeLockbox(_lockbox);
    }

    /// @notice Tests the `authorizeLockbox` function succeeds
    function testFuzz_authorizeLockbox_succeeds(address _lockbox) public {
        vm.assume(!ethLockbox.authorizedLockboxes(_lockbox));

        // Mock the admin owner of the lockbox to be the same as the current lockbox proxy admin owner
        vm.mockCall(address(_lockbox), abi.encodeCall(IPAOBase.PAO, ()), abi.encode(proxyAdmin.owner()));

        // Expect the `LockboxAuthorized` event to be emitted
        vm.expectEmit(address(ethLockbox));
        emit LockboxAuthorized(_lockbox);

        // Authorize the lockbox
        vm.prank(PAO);
        ethLockbox.authorizeLockbox(_lockbox);

        // Assert the lockbox is authorized
        assertTrue(ethLockbox.authorizedLockboxes(_lockbox));
    }

    /// @notice Tests the `migrateLiquidity` function reverts when the caller is not the proxy admin.
    function testFuzz_migrateLiquidity_unauthorized_reverts(address _caller) public {
        vm.assume(_caller != proxyAdmin.owner());

        // Expect the revert with `Unauthorized` selector
        vm.expectRevert(IETHLockbox.ETHLockbox_Unauthorized.selector);

        // Call the `migrateLiquidity` function with an unauthorized caller
        vm.prank(_caller);
        ethLockbox.migrateLiquidity(address(optimismPortal2));
    }

    /// @notice Tests the `migrateLiquidity` function reverts when the PAO of the lockbox is not the same as the PAO of
    ///         the proxy admin.
    function testFuzz_migrateLiquidity_differentPAO_reverts(address _lockbox) public {
        vm.mockCall(address(_lockbox), abi.encodeCall(IPAOBase.PAO, ()), abi.encode(address(0)));

        // Expect the revert with `ETHLockbox_DifferentPAO` selector
        vm.expectRevert(IETHLockbox.ETHLockbox_DifferentPAO.selector);

        // Call the `migrateLiquidity` function with the lockbox
        vm.prank(PAO);
        ethLockbox.migrateLiquidity(_lockbox);
    }

    /// @notice Tests the `migrateLiquidity` function succeeds
    function testFuzz_migrateLiquidity_succeeds(uint256 _balance, address _lockbox) public {
        // Mock on the lockbox that will receive the migration for it to succeed
        vm.mockCall(address(_lockbox), abi.encodeCall(IPAOBase.PAO, ()), abi.encode(proxyAdmin.owner()));
        vm.mockCall(
            address(_lockbox), abi.encodeCall(IETHLockbox.authorizedLockboxes, (address(ethLockbox))), abi.encode(true)
        );
        vm.mockCall(address(_lockbox), abi.encodeCall(IETHLockbox.receiveLiquidity, ()), abi.encode(true));

        // Deal the balance to the lockbox
        deal(address(_lockbox), _balance);

        // Expect the `LiquidityMigrated` event to be emitted
        vm.expectEmit(address(ethLockbox));
        emit LiquidityMigrated(_lockbox);

        // Get balances before the migration
        uint256 ethLockboxBalanceBefore = address(ethLockbox).balance;
        uint256 newLockboxBalanceBefore = address(_lockbox).balance;

        // Call the `migrateLiquidity` function with the lockbox
        vm.prank(PAO);
        ethLockbox.migrateLiquidity(_lockbox);

        // Assert the liquidity was migrated
        assertEq(address(_lockbox).balance, newLockboxBalanceBefore + ethLockboxBalanceBefore);
        assertEq(address(ethLockbox).balance, 0);
    }
}
