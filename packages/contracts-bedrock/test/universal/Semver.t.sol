// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Test } from "forge-std/Test.sol";
import { Semver } from "src/universal/Semver.sol";

/// @notice A mock contract that implements Semver
contract MockSemver is Semver {
    function _version() internal pure override returns (Semver.Versions memory) {
        return Semver.Versions({ major: 1, minor: 2, patch: 3, suffix: "" });
    }
}

/// @title SemverTest
/// @notice Tests for the Semver interface
contract SemverTest is Test {
    /// @notice The mock Semver contract
    MockSemver semver;

    /// @notice Deploy the mock contract
    function setUp() public {
        semver = new MockSemver();
    }

    /// @notice Test that the version is returned correctly
    function test_version() public view {
        assertEq(semver.version(), "1.2.3");
    }

    /// @notice Test that the reinitializerValue function returns the correct uint64
    function test_reinitializerValue() public view {
        assertEq(uint256(semver.reinitializerValue()), 1000002000003);
    }
}
