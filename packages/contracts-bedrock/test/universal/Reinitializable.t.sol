// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { Test } from "forge-std/Test.sol";
import { Reinitializable } from "src/universal/Reinitializable.sol";

/// @notice A concrete implementation of Reinitializable for testing
contract MockReinitializable is Reinitializable {
    uint64 internal immutable NONCE;

    /// @notice Constructor for MockReinitNonce
    constructor(uint64 _nonce) {
        NONCE = _nonce;
    }

    /// @notice Returns a fixed nonce for testing
    function reinitNonce() public view override returns (uint64) {
        return NONCE;
    }

    function initialize() reinitializer(reinitValue()) { }

    function upgrade() reinitializer(reinitValue()) { }
}

/// @title ReinitializableTest
/// @notice Test contract for Reinitializable
contract ReinitializableTest is Test {
    /// @notice The test contract instance
    MockReinitializable internal mockReinitializable;

    /// @notice Sets up the test contract
    function setUp() public {
        mockReinitializable = new MockReinitializable(5);
    }

    /// @notice Tests that reinitializerValue returns the correct value
    function test_reinitializerValue_succeeds() external {
        // Should return SPACING * reinitNonce() = 10 * 5 = 50
        assertEq(mockReinitializable.reinitializerValue(), 50);
    }

    /// @notice Tests that reinitializerValue returns the correct values for different nonces
    function testFuzz_reinitializerValue_differentNonces(uint64 _nonce) external {
        _nonce = uint64(bound(uint256(_nonce), 1, uint256(type(uint64).max / 10 - 1)));

        // Create a new test contract that returns the fuzzed nonce
        MockReinitializable implementation = new MockReinitializable(_nonce);
        uint64 value = implementation.reinitializerValue();

        // Value should be _nonce * 10
        assertEq(value, uint64(10) * _nonce);
    }
}
