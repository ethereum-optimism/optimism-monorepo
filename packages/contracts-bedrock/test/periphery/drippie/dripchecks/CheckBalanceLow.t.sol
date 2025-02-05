// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Test } from "forge-std/Test.sol";
import { CheckBalanceLow } from "src/periphery/drippie/dripchecks/CheckBalanceLow.sol";

/// @title  CheckBalanceLowTest
/// @notice Tests the CheckBalanceLow contract via fuzzing both the success case
///         and the failure case.
contract CheckBalanceLowTest is Test {
    /// @notice An instance of the CheckBalanceLow contract.
    CheckBalanceLow c;

    /// @notice Deploy the `CheckBalanceLow` contract.
    function setUp() external {
        c = new CheckBalanceLow();
    }

    /// @notice Test that the `name` function returns the correct value.
    function test_name_succeeds() external view {
        assertEq(c.name(), "CheckBalanceLow");
    }

    /// @notice Fuzz the `check` function and assert that it always returns true
    ///         when the target's balance is smaller than the threshold.
    function testFuzz_check_succeeds(address _target, uint256 _threshold) external {
        CheckBalanceLow.Params memory p = CheckBalanceLow.Params({ target: _target, threshold: _threshold });

        if (_target.balance >= p.threshold) {
            if (_threshold == 0) p.threshold = 1;
            vm.deal(_target, p.threshold - 1);
        }

        assertEq(c.check(abi.encode(p)), true);
    }

    /// @notice Fuzz the `check` function and assert that it always returns false
    ///         when the target's balance is larger than the threshold.
    function testFuzz_check_highBalance_fails(address _target, uint256 _threshold) external {
        CheckBalanceLow.Params memory p = CheckBalanceLow.Params({ target: _target, threshold: _threshold });

        // prevent overflows
        _threshold = bound(_threshold, 0, type(uint256).max - 1);
        vm.deal(_target, _threshold + 1);

        assertEq(c.check(abi.encode(p)), false);
    }
}
