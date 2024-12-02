// SPDX-License-Identifier: MIT
pragma solidity 0.8.25;

import { Test } from "forge-std/Test.sol";
import { AddressAliasHelper } from "src/vendor/AddressAliasHelper.sol";

contract AddressAliasHelper_applyAndUndo_Test is Test {
    /// @notice Tests that applying and then undoing an alias results in the original address.
    function testFuzz_applyAndUndo_succeeds(address _address) external pure {
        address aliased = AddressAliasHelper.applyL1ToL2Alias(_address);
        address unaliased = AddressAliasHelper.undoL1ToL2Alias(aliased);
        assertEq(_address, unaliased);
    }
}
