// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { CommonTest } from "test/setup/CommonTest.sol";

/// @dev These tests are somewhat redundant with tests in the SuperchainConfig and other pausable contracts, however
///      it is worthwhile to pull them into one location to ensure that the behavior is consistent.
contract ExtendedPause_Test is CommonTest {
    /// @dev Tests that other contracts are paused when the superchain config is paused
    function test_pause_fullSystem_succeeds() public {
        assertFalse(superchainConfig.paused());

        vm.prank(superchainConfig.guardian());
        superchainConfig.pause("identifier");

        // validate the paused state
        assertTrue(superchainConfig.paused());
        assertTrue(optimismPortal2.paused());
        assertTrue(l1CrossDomainMessenger.paused());
        assertTrue(l1StandardBridge.paused());
        assertTrue(l1ERC721Bridge.paused());
    }

    /// @dev Tests that other contracts are unpaused when the superchain config is paused and then unpaused.
    function test_unpause_fullSystem_succeeds() external {
        // first use the test above to pause the system
        test_pause_fullSystem_succeeds();

        vm.prank(superchainConfig.guardian());
        superchainConfig.unpause();

        // validate the unpaused state
        assertFalse(superchainConfig.paused());
        assertFalse(optimismPortal2.paused());
        assertFalse(l1CrossDomainMessenger.paused());
        assertFalse(l1StandardBridge.paused());
        assertFalse(l1ERC721Bridge.paused());
    }
}
