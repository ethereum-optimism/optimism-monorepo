// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { StdInvariant } from "forge-std/StdInvariant.sol";
import { AddressAliasHelper } from "src/vendor/AddressAliasHelper.sol";
import { InvariantTest } from "test/invariants/InvariantTest.sol";

contract AddressAliasHelper_Converter {
    bool public failedRoundtrip;

    /// @dev Allows the actor to convert L1 to L2 addresses and vice versa.
    function convertRoundTrip(address addr) external {
        // Alias our address
        address aliasedAddr = AddressAliasHelper.applyL1ToL2Alias(addr);

        // Unalias our address
        address undoneAliasAddr = AddressAliasHelper.undoL1ToL2Alias(aliasedAddr);

        // If our round trip aliasing did not return the original result, set our state.
        if (addr != undoneAliasAddr) {
            failedRoundtrip = true;
        }
    }
}

contract AddressAliasHelper_AddressAliasing_Invariant is StdInvariant, InvariantTest {
    AddressAliasHelper_Converter internal actor;

    function setUp() public override {
        super.setUp();
        // Create a converter actor.
        actor = new AddressAliasHelper_Converter();

        targetContract(address(actor));

        bytes4[] memory selectors = new bytes4[](1);
        selectors[0] = actor.convertRoundTrip.selector;
        FuzzSelector memory selector = FuzzSelector({ addr: address(actor), selectors: selectors });
        targetSelector(selector);
    }

    /// @custom:invariant Address aliases are always able to be undone.
    ///
    ///                   Asserts that an address that has been aliased with
    ///                   `applyL1ToL2Alias` can always be unaliased with
    ///                   `undoL1ToL2Alias`.
    function invariant_round_trip_aliasing() external view {
        // ASSERTION: The round trip aliasing done in testRoundTrip(...) should never fail.
        assertEq(actor.failedRoundtrip(), false);
    }
}
