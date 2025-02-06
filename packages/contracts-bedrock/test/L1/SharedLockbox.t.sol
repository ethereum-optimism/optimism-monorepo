// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing utilities
import { CommonTest } from "test/setup/CommonTest.sol";
import { Unauthorized, Paused as PausedError } from "src/libraries/errors/CommonErrors.sol";

// Interfaces
import { IOptimismPortal2 as IOptimismPortal } from "interfaces/L1/IOptimismPortal2.sol";
import { ISuperchainConfigInterop } from "interfaces/L1/ISuperchainConfigInterop.sol";

contract SharedLockboxTest is CommonTest {
    event ETHLocked(address indexed portal, uint256 amount);

    event ETHUnlocked(address indexed portal, uint256 amount);

    event PortalAuthorized(address indexed portal);

    function setUp() public virtual override {
        super.enableInterop();
        super.setUp();
    }

    function _superchainConfig() internal view returns (ISuperchainConfigInterop) {
        return ISuperchainConfigInterop(address(superchainConfig));
    }

    /// @notice Tests it reverts when the caller is not an authorized portal.
    function test_lockETH_unauthorizedPortal_reverts(address _caller) public {
        vm.assume(!_superchainConfig().authorizedPortals(_caller));

        // Expect the revert with `Unauthorized` selector
        vm.expectRevert(Unauthorized.selector);

        // Call the `lockETH` function with an unauthorized caller
        vm.prank(_caller);
        sharedLockbox.lockETH();
    }

    /// @notice Tests the ETH is correctly locked when the caller is an authorized portal.
    function test_lockETH_succeeds(uint256 _amount) public {
        // Deal the ETH amount to the portal
        vm.deal(address(optimismPortal2), _amount);

        // Get the balance of the portal and lockbox before the lock to compare later on the assertions
        uint256 _portalBalanceBefore = address(optimismPortal2).balance;
        uint256 _lockboxBalanceBefore = address(sharedLockbox).balance;

        // Look for the emit of the `ETHLocked` event
        vm.expectEmit(address(sharedLockbox));
        emit ETHLocked(address(optimismPortal2), _amount);

        // Call the `lockETH` function with the portal
        vm.prank(address(optimismPortal2));
        sharedLockbox.lockETH{ value: _amount }();

        // Assert the portal's balance decreased and the lockbox's balance increased by the amount locked
        assertEq(address(optimismPortal2).balance, _portalBalanceBefore - _amount);
        assertEq(address(sharedLockbox).balance, _lockboxBalanceBefore + _amount);
    }

    /// @notice Tests the ETH is correctly locked when the caller is an authorized portal with different portals.
    function test_lockETHWithDifferentPortal_succeeds(address _portal, uint256 _amount) public {
        vm.assume(_portal != address(sharedLockbox));

        // Mock the portal as an authorized portal
        vm.mockCall(
            address(superchainConfig),
            abi.encodeCall(ISuperchainConfigInterop.authorizedPortals, (_portal)),
            abi.encode(true)
        );

        // Deal the ETH amount to the portal
        vm.deal(_portal, _amount);

        // Get the balance of the portal and lockbox before the lock to compare later on the assertions
        uint256 _portalBalanceBefore = address(_portal).balance;
        uint256 _lockboxBalanceBefore = address(sharedLockbox).balance;

        // Look for the emit of the `ETHLocked` event
        vm.expectEmit(address(sharedLockbox));
        emit ETHLocked(_portal, _amount);

        // Call the `lockETH` function with the portal
        vm.prank(_portal);
        sharedLockbox.lockETH{ value: _amount }();

        // Assert the portal's balance decreased and the lockbox's balance increased by the amount locked
        assertEq(address(_portal).balance, _portalBalanceBefore - _amount);
        assertEq(address(sharedLockbox).balance, _lockboxBalanceBefore + _amount);
    }

    /// @notice Tests `unlockETH` reverts when the contract is paused.
    function test_unlockETH_paused_reverts(address _caller, uint256 _value) public {
        // Set the paused status to true
        vm.prank(superchainConfig.guardian());
        superchainConfig.pause("test");

        // Expect the revert with `Paused` selector
        vm.expectRevert(PausedError.selector);

        // Call the `unlockETH` function with the caller
        vm.prank(_caller);
        sharedLockbox.unlockETH(_value);
    }

    /// @notice Tests it reverts when the caller is not an authorized portal.
    function test_unlockETH_unauthorizedPortal_reverts(address _caller, uint256 _value) public {
        vm.assume(!_superchainConfig().authorizedPortals(_caller));

        // Expect the revert with `Unauthorized` selector
        vm.expectRevert(Unauthorized.selector);

        // Call the `unlockETH` function with an unauthorized caller
        vm.prank(_caller);
        sharedLockbox.unlockETH(_value);
    }

    /// @notice Tests the ETH is correctly unlocked when the caller is an authorized portal.
    function test_unlockETH_succeeds(uint256 _value) public {
        // Deal the ETH amount to the lockbox
        vm.deal(address(sharedLockbox), _value);

        // Get the balance of the portal and lockbox before the unlock to compare later on the assertions
        uint256 _portalBalanceBefore = address(optimismPortal2).balance;
        uint256 _lockboxBalanceBefore = address(sharedLockbox).balance;

        // Expect `donateETH` function to be called on Portal
        vm.expectCall(address(optimismPortal2), abi.encodeCall(IOptimismPortal.donateETH, ()));

        // Look for the emit of the `ETHUnlocked` event
        vm.expectEmit(address(sharedLockbox));
        emit ETHUnlocked(address(optimismPortal2), _value);

        // Call the `unlockETH` function with the portal
        vm.prank(address(optimismPortal2));
        sharedLockbox.unlockETH(_value);

        // Assert the portal's balance increased and the lockbox's balance decreased by the amount unlocked
        assertEq(address(optimismPortal2).balance, _portalBalanceBefore + _value);
        assertEq(address(sharedLockbox).balance, _lockboxBalanceBefore - _value);
    }

    /// @notice Tests the paused status is correctly returned.
    function test_paused_succeeds() public {
        // Assert the paused status is false
        assertEq(sharedLockbox.paused(), false);

        // Set the paused status to true
        vm.prank(superchainConfig.guardian());
        superchainConfig.pause("test");

        // Assert the paused status is true
        assertEq(sharedLockbox.paused(), true);
    }
}
