// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { CommonTest } from "test/setup/CommonTest.sol";

// Target contract dependencies
import { IProxy } from "interfaces/universal/IProxy.sol";

// Target contract
import { IDummyRegistry } from "interfaces/L1/IDummyRegistry.sol";

import { DeployUtils } from "scripts/libraries/DeployUtils.sol"; /// We use DeployUtils to deploy contracts.

contract DummyRegistry_Init_Test is CommonTest { /// We inherit from CommonTest. One contract per function and outcome.
    /// @notice Emitted when a value is stored.
    /// @param key The key of the value stored.
    /// @param value The value stored.
    event Stored(string key, string value);

    function setUp() public virtual override {
        super.setUp();
        skipIfForkTest("DummyRegistry_Init_Test: cannot test initialization on forked network");
    }

    /// @dev Tests that it can be initialized as stored.
    /// Note do we use @dev for tests, and @notice for code?
    function test_initialize_succeeds() external {

        assertEq(IDummyRegistry(address(dummyRegistry)).guardian(), opcm.upgradeController());
    }
}

contract DummyRegistry_Store_TestFail is DummyRegistry_Init_Test {
    /// @dev Tests that `store` reverts when called by a non-guardian.
    function test_store_notGuardian_reverts() external {
        assertTrue(dummyRegistry.guardian() != alice);
        vm.expectRevert("DummyRegistry: only guardian can store");
        vm.prank(alice);
        dummyRegistry.store("key", "value");

        assertEq(keccak256(abi.encodePacked(dummyRegistry.read("key"))), keccak256(abi.encodePacked("value"))); /// We probably have a library to compare strings.
    }
}

contract DummyRegistry_Store_Test is DummyRegistry_Init_Test {
    /// @dev Tests that `store` successfully stores values
    ///      when called by the guardian.
    function test_store_succeeds() external {
        assertEq(keccak256(abi.encodePacked(dummyRegistry.read("key"))), keccak256(abi.encodePacked("")));

        vm.expectEmit(address(dummyRegistry));
        emit Stored("key", "value");

        vm.prank(dummyRegistry.guardian());
        dummyRegistry.store("key", "value");

        assertEq(keccak256(abi.encodePacked(dummyRegistry.read("key"))), keccak256(abi.encodePacked("value")));
    }
}
