// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing utilities
import { Test } from "forge-std/Test.sol";
import { Unauthorized } from "src/libraries/errors/CommonErrors.sol";

// Targent contract
import { SharedLockbox } from "src/L1/SharedLockbox.sol";

// Interfaces
import { IOptimismPortal } from "src/L1/interfaces/IOptimismPortal.sol";

contract SharedLockboxTest is Test {
    event ETHLocked(address indexed portal, uint256 amount);

    event ETHUnlocked(address indexed portal, uint256 amount);

    event AuthorizedPortal(address indexed portal);

    address internal immutable SUPERCHAIN_CONFIG = makeAddr("SuperchainConfig");
    IOptimismPortal internal immutable PORTAL = IOptimismPortal(payable(makeAddr("OptimismPortal")));
    SharedLockbox public sharedLockbox;

    // TODO: Update setup to use CommonTest and simulate a real deployment environment
    function setUp() public {
        // Deploy the SharedLockbox contract
        sharedLockbox = new SharedLockbox(SUPERCHAIN_CONFIG);

        // Etch the OptimismPortal contract code into the declared address
        bytes memory code = vm.getDeployedCode("L1/OptimismPortal.sol:OptimismPortal");
        vm.etch(address(PORTAL), code);
    }

    /// @notice Tests it reverts when the caller is not an authorized portal.
    function test_lockETH_unauthorizedPortal_reverts(address _caller) public {
        vm.assume(!sharedLockbox.authorizedPortals(_caller));

        // Expect the revert with `Unauthorized` selector
        vm.expectRevert(Unauthorized.selector);

        // Call the `lockETH` function with an unauthorized caller
        vm.prank(_caller);
        sharedLockbox.lockETH();
    }

    /// @notice Tests the ETH is correctly locked when the caller is an authorized portal.
    function test_lockETH_succeeds(address _portal, uint256 _amount) public {
        // Set the caller as an authorized portal
        vm.prank(SUPERCHAIN_CONFIG);
        sharedLockbox.authorizePortal(_portal);

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

    /// @notice Tests it reverts when the caller is not an authorized portal.
    function test_unlockETH_unauthorizedPortal_reverts(address _caller, uint256 _value) public {
        vm.assume(!sharedLockbox.authorizedPortals(_caller));

        // Expect the revert with `Unauthorized` selector
        vm.expectRevert(Unauthorized.selector);

        // Call the `unlockETH` function with an unauthorized caller
        vm.prank(_caller);
        sharedLockbox.unlockETH(_value);
    }

    /// @notice Tests the ETH is correctly unlocked when the caller is an authorized portal.
    function test_unlockETH_succeeds(uint256 _value) public {
        // Set the caller as an authorized portal
        vm.prank(SUPERCHAIN_CONFIG);
        sharedLockbox.authorizePortal(address(PORTAL));

        // Deal the ETH amount to the lockbox
        vm.deal(address(sharedLockbox), _value);

        // Get the balance of the portal and lockbox before the unlock to compare later on the assertions
        uint256 _portalBalanceBefore = address(PORTAL).balance;
        uint256 _lockboxBalanceBefore = address(sharedLockbox).balance;

        // Expect `donateETH` function to be called on Portal
        vm.expectCall(address(PORTAL), abi.encodeWithSelector(IOptimismPortal.donateETH.selector));

        // Look for the emit of the `ETHUnlocked` event
        vm.expectEmit(address(sharedLockbox));
        emit ETHUnlocked(address(PORTAL), _value);

        // Call the `unlockETH` function with the portal
        vm.prank(address(PORTAL));
        sharedLockbox.unlockETH(_value);

        // Assert the portal's balance increased and the lockbox's balance decreased by the amount unlocked
        assertEq(address(PORTAL).balance, _portalBalanceBefore + _value);
        assertEq(address(sharedLockbox).balance, _lockboxBalanceBefore - _value);
    }

    /// @notice Tests it reverts when the caller is not the SuperchainConfig.
    function test_authorizePortal_notSuperchainConfig_reverts(address _caller) public {
        vm.assume(_caller != SUPERCHAIN_CONFIG);

        // Expect the revert with `Unauthorized` selector
        vm.expectRevert(Unauthorized.selector);

        // Call the `authorizePortal` function with a non-SuperchainConfig caller
        vm.prank(_caller);
        sharedLockbox.authorizePortal(_caller);
    }

    /// @notice Tests the portal is correctly authorized when the caller is the SuperchainConfig.
    function test_authorizePortal_succeeds(address _portal) public {
        // Check the portal's authorized status before the authorization to compare later on the assertions.
        // Adding this check to make it more future proof in case something changes on the setup.
        vm.assume(sharedLockbox.authorizedPortals(_portal) == false);

        // Look for the emit of the `AuthorizedPortal` event
        vm.expectEmit(address(sharedLockbox));
        emit AuthorizedPortal(_portal);

        // Call the `authorizePortal` function with the SuperchainConfig
        vm.prank(SUPERCHAIN_CONFIG);
        sharedLockbox.authorizePortal(_portal);

        // Assert the portal's authorized status was updated correctly
        assertEq(sharedLockbox.authorizedPortals(_portal), true);
    }
}
