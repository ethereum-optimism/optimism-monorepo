// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Test } from "forge-std/Test.sol";
import { StdUtils } from "forge-std/StdUtils.sol";
import { Vm } from "forge-std/Vm.sol";
import { SafeCall } from "src/libraries/SafeCall.sol";
import { InvariantTest } from "test/invariants/InvariantTest.sol";

contract SafeCall_Succeeds_Invariants is InvariantTest {
    SafeCaller_Actor actor;

    function setUp() public override {
        super.setUp();
        // Create a new safe caller actor.
        actor = new SafeCaller_Actor(vm, false);

        // Set the caller to this contract
        targetSender(address(this));

        // Target the safe caller actor.
        targetContract(address(actor));

        // Give the actor some ETH to work with
        vm.deal(address(actor), type(uint128).max);
    }

    /// @custom:invariant If `callWithMinGas` performs a call, then it must always
    ///                   provide at least the specified minimum gas limit to the subcontext.
    ///
    ///                   If the check for remaining gas in `SafeCall.callWithMinGas` passes, the
    ///                   subcontext of the call below it must be provided at least `minGas` gas.
    function invariant_callWithMinGas_alwaysForwardsMinGas_succeeds() public view {
        assertEq(actor.numCalls(), 0, "no failed calls allowed");
    }

    function performSafeCallMinGas(address to, uint64 minGas) external payable {
        SafeCall.callWithMinGas(to, minGas, msg.value, hex"");
    }
}

contract SafeCall_Fails_Invariants is InvariantTest {
    SafeCaller_Actor actor;

    function setUp() public override {
        super.setUp();
        // Create a new safe caller actor.
        actor = new SafeCaller_Actor(vm, true);

        // Set the caller to this contract
        targetSender(address(this));

        // Target the safe caller actor.
        targetContract(address(actor));

        // Give the actor some ETH to work with
        vm.deal(address(actor), type(uint128).max);
    }

    /// @custom:invariant `callWithMinGas` reverts if there is not enough gas to pass
    ///                   to the subcontext.
    ///
    ///                   If there is not enough gas in the callframe to ensure that
    ///                   `callWithMinGas` can provide the specified minimum gas limit
    ///                   to the subcontext of the call, then `callWithMinGas` must revert.
    function invariant_callWithMinGas_neverForwardsMinGas_reverts() public view {
        assertEq(actor.numCalls(), 0, "no successful calls allowed");
    }

    function performSafeCallMinGas(address to, uint64 minGas) external payable {
        SafeCall.callWithMinGas(to, minGas, msg.value, hex"");
    }
}

contract SafeCaller_Actor is StdUtils {
    bool internal immutable FAILS;

    Vm internal vm;
    uint256 public numCalls;

    constructor(Vm _vm, bool _fails) {
        vm = _vm;
        FAILS = _fails;
    }

    function performSafeCallMinGas(uint64 gas, uint64 minGas, address to, uint8 value) external {
        // Only send to EOAs - we exclude the console as it has no code but reverts when called
        // with a selector that doesn't exist due to the foundry hook.
        vm.assume(to.code.length == 0 && to != 0x000000000000000000636F6e736F6c652e6c6f67);

        // Bound the minimum gas amount to [2500, type(uint48).max]
        minGas = uint64(bound(minGas, 2500, type(uint48).max));
        if (FAILS) {
            // Bound the gas passed to [minGas, ((minGas * 64) / 63)]
            gas = uint64(bound(gas, minGas, (minGas * 64) / 63));
        } else {
            // Bound the gas passed to
            // [((minGas * 64) / 63) + 40_000 + 1000, type(uint64).max]
            // The extra 1000 gas is to account for the gas used by the `SafeCall.call` call
            // itself.
            gas = uint64(bound(gas, ((minGas * 64) / 63) + 40_000 + 1000, type(uint64).max));
        }

        vm.expectCallMinGas(to, value, minGas, hex"");
        bool success = SafeCall.call(
            msg.sender, gas, value, abi.encodeCall(SafeCall_Succeeds_Invariants.performSafeCallMinGas, (to, minGas))
        );

        if (success && FAILS) numCalls++;
        if (!FAILS && !success) numCalls++;
    }
}
