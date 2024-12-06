// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing utilities
import { Test } from "forge-std/Test.sol";
import { VmSafe } from "forge-std/Vm.sol";
import { StdCheatsSafe } from "forge-std/StdCheats.sol";

// Target contract
import { SafeCall } from "src/libraries/SafeCall.sol";

contract SafeCall_Test is Test {
    /// @notice Helper function to deduplicate code. Makes all assumptions required for these tests.
    function assumeNot(address _addr) internal {
        vm.assume(_addr.balance == 0);
        vm.assume(_addr != address(this));
        assumeAddressIsNot(_addr, StdCheatsSafe.AddressType.ForgeAddress, StdCheatsSafe.AddressType.Precompile);
    }

    /// @notice Internal helper function for `send` tests
    function sendTest(address _from, address _to, uint64 _gas, uint256 _value) internal {
        assumeNot(_from);
        assumeNot(_to);

        assertEq(_from.balance, 0, "from balance is 0");
        vm.deal(_from, _value);
        assertEq(_from.balance, _value, "from balance not dealt");

        uint256[2] memory balancesBefore = [_from.balance, _to.balance];

        vm.expectCall(_to, _value, bytes(""));
        vm.prank(_from);
        bool success;
        if (_gas == 0) {
            success = SafeCall.send({ _target: _to, _value: _value });
        } else {
            success = SafeCall.send({ _target: _to, _gas: _gas, _value: _value });
        }

        assertTrue(success, "send not successful");
        if (_from == _to) {
            assertEq(_from.balance, balancesBefore[0], "Self-send did not change balance");
        } else {
            assertEq(_from.balance, balancesBefore[0] - _value, "from balance not drained");
            assertEq(_to.balance, balancesBefore[1] + _value, "to balance received");
        }
    }

    /// @dev Tests that the `send` function succeeds.
    function testFuzz_send_succeeds(address _from, address _to, uint256 _value) external {
        sendTest({ _from: _from, _to: _to, _gas: 0, _value: _value });
    }

    /// @dev Tests that the `send` function with value succeeds.
    function testFuzz_sendWithGas_succeeds(address _from, address _to, uint64 _gas, uint256 _value) external {
        _gas = uint64(bound(_gas, 1, type(uint64).max));
        sendTest({ _from: _from, _to: _to, _gas: _gas, _value: _value });
    }

    /// @dev Tests that `call` succeeds.
    function testFuzz_call_succeeds(address from, address to, uint256 gas, uint64 value, bytes memory data) external {
        assumeNot(from);
        assumeNot(to);

        assertEq(from.balance, 0, "from balance is 0");
        vm.deal(from, value);
        assertEq(from.balance, value, "from balance not dealt");

        uint256[2] memory balancesBefore = [from.balance, to.balance];

        vm.expectCall(to, value, data);
        vm.prank(from);
        bool success = SafeCall.call(to, gas, value, data);

        assertTrue(success, "call not successful");
        if (from == to) {
            assertEq(from.balance, balancesBefore[0], "Self-send did not change balance");
        } else {
            assertEq(from.balance, balancesBefore[0] - value, "from balance not drained");
            assertEq(to.balance, balancesBefore[1] + value, "to balance received");
        }
    }

    /// @dev Tests that `callWithMinGas` succeeds with enough gas.
    function testFuzz_callWithMinGas_hasEnough_succeeds(
        address from,
        address to,
        uint64 minGas,
        uint64 value,
        bytes memory data
    )
        external
    {
        assumeNot(from);
        assumeNot(to);

        assertEq(from.balance, 0, "from balance is 0");
        vm.deal(from, value);
        assertEq(from.balance, value, "from balance not dealt");

        // Bound minGas to [0, l1_block_gas_limit]
        minGas = uint64(bound(minGas, 0, 30_000_000));

        uint256[2] memory balancesBefore = [from.balance, to.balance];

        vm.expectCallMinGas(to, value, minGas, data);
        vm.prank(from);
        bool success = SafeCall.callWithMinGas(to, minGas, value, data);

        assertTrue(success, "call not successful");
        if (from == to) {
            assertEq(from.balance, balancesBefore[0], "Self-send did not change balance");
        } else {
            assertEq(from.balance, balancesBefore[0] - value, "from balance not drained");
            assertEq(to.balance, balancesBefore[1] + value, "to balance received");
        }
    }

    /// @dev Tests that `callWithMinGas` succeeds for the lower gas bounds.
    function test_callWithMinGas_noLeakageLow_succeeds() external {
        SimpleSafeCaller caller = new SimpleSafeCaller();

        for (uint64 i = 40_000; i < 100_000; i++) {
            uint256 snapshot = vm.snapshot();

            // The values below are best gotten by setting the value to a high number and running the test with a
            // verbosity of `-vvv` then setting the value to the value (gas arg) of the failed assertion.
            // A faster way to do this for forge coverage cases, is to comment out the optimizer and optimizer runs in
            // the foundry.toml file and then run forge test. This is faster because forge test only compiles modified
            // contracts unlike forge coverage.
            uint256 expected;

            // Because forge coverage always runs with the optimizer disabled,
            // if forge coverage is run before testing this with forge test or forge snapshot, forge clean should be
            // run first so that it recompiles the contracts using the foundry.toml optimizer settings.
            if (vm.isContext(VmSafe.ForgeContext.Coverage)) {
                // 66_290 is the exact amount of gas required to make the safe call
                // successfully with the optimizer disabled (ran via forge coverage)
                expected = 66_290;
            } else if (vm.isContext(VmSafe.ForgeContext.Test) || vm.isContext(VmSafe.ForgeContext.Snapshot)) {
                // 65_922 is the exact amount of gas required to make the safe call
                // successfully with the foundry.toml optimizer settings.
                expected = 65_922;
            } else {
                revert("SafeCall_Test: unknown context");
            }

            if (i < expected) {
                assertFalse(caller.makeSafeCall(i, 25_000));
            } else {
                vm.expectCallMinGas(address(caller), 0, 25_000, abi.encodeCall(caller.setA, (1)));
                assertTrue(caller.makeSafeCall(i, 25_000));
            }

            assertTrue(vm.revertTo(snapshot));
        }
    }

    /// @dev Tests that `callWithMinGas` succeeds on the upper gas bounds.
    function test_callWithMinGas_noLeakageHigh_succeeds() external {
        SimpleSafeCaller caller = new SimpleSafeCaller();

        for (uint64 i = 15_200_000; i < 15_300_000; i++) {
            uint256 snapshot = vm.snapshot();

            // The values below are best gotten by setting the value to a high number and running the test with a
            // verbosity of `-vvv` then setting the value to the value (gas arg) of the failed assertion.
            // A faster way to do this for forge coverage cases, is to comment out the optimizer and optimizer runs in
            // the foundry.toml file and then run forge test. This is faster because forge test only compiles modified
            // contracts unlike forge coverage.
            uint256 expected;

            // Because forge coverage always runs with the optimizer disabled,
            // if forge coverage is run before testing this with forge test or forge snapshot, forge clean should be
            // run first so that it recompiles the contracts using the foundry.toml optimizer settings.
            if (vm.isContext(VmSafe.ForgeContext.Coverage)) {
                // 15_278_989 is the exact amount of gas required to make the safe call
                // successfully with the optimizer disabled (ran via forge coverage)
                expected = 15_278_989;
            } else if (vm.isContext(VmSafe.ForgeContext.Test) || vm.isContext(VmSafe.ForgeContext.Snapshot)) {
                // 15_278_621 is the exact amount of gas required to make the safe call
                // successfully with the foundry.toml optimizer settings.
                expected = 15_278_621;
            } else {
                revert("SafeCall_Test: unknown context");
            }

            if (i < expected) {
                assertFalse(caller.makeSafeCall(i, 15_000_000));
            } else {
                vm.expectCallMinGas(address(caller), 0, 15_000_000, abi.encodeCall(caller.setA, (1)));
                assertTrue(caller.makeSafeCall(i, 15_000_000));
            }

            assertTrue(vm.revertTo(snapshot));
        }
    }
}

contract SimpleSafeCaller {
    uint256 public a;

    function makeSafeCall(uint64 gas, uint64 minGas) external returns (bool) {
        return SafeCall.call(address(this), gas, 0, abi.encodeCall(this.makeSafeCallMinGas, (minGas)));
    }

    function makeSafeCallMinGas(uint64 minGas) external returns (bool) {
        return SafeCall.callWithMinGas(address(this), minGas, 0, abi.encodeCall(this.setA, (1)));
    }

    function setA(uint256 _a) external {
        a = _a;
    }
}
