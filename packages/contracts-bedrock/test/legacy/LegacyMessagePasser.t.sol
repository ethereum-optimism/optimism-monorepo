// SPDX-License-Identifier: MIT
pragma solidity 0.8.25;

// Testing utilities
import { CommonTest } from "test/setup/CommonTest.sol";

contract LegacyMessagePasser_Test is CommonTest {
    /// @dev Tests that `passMessageToL1` succeeds.
    function test_passMessageToL1_succeeds() external {
        vm.prank(alice);
        legacyMessagePasser.passMessageToL1(hex"ff");
        assert(legacyMessagePasser.sentMessages(keccak256(abi.encodePacked(hex"ff", alice))));
    }
}
