// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { CommonTest } from "test/setup/CommonTest.sol";

// Target contract dependencies
import { IProxy } from "interfaces/universal/IProxy.sol";

// Target contract
import { ISingletonExample } from "interfaces/L1/ISingletonExample.sol";

import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

/// We use DeployUtils to deploy contracts.

import { console2 as console2 } from "forge-std/console2.sol";

contract SingletonExample_Init_Test is CommonTest {
    /// We inherit from CommonTest. One contract per function and outcome.
    /// @notice Emitted when a value is stored.
    /// @param key The key of the value stored.
    /// @param value The value stored.
    event Stored(string key, string value);

    function setUp() public virtual override {
        super.setUp();
        skipIfForkTest("SingletonExample_Init_Test: cannot test initialization on forked network");
    }

    /// @dev Tests that it can be initialized as stored.
    /// Note do we use @dev for tests, and @notice for code?
    function test_initialize_succeeds() external view {
        assertEq(ISingletonExample(address(singletonExample)).guardian(), opcm.upgradeController());
    }
}

contract SingletonExample_Store_TestFail is SingletonExample_Init_Test {
    /// @dev Tests that `store` reverts when called by a non-guardian.
    function test_store_notGuardian_reverts() external {
        assertTrue(singletonExample.guardian() != alice);
        vm.expectRevert("SingletonExample: only guardian can store");
        vm.prank(alice);
        singletonExample.store("key", "value");
    }
}

contract SingletonExample_Store_Test is SingletonExample_Init_Test {
    /// @dev Tests that `store` successfully stores values
    ///      when called by the guardian.
    function test_store_succeeds() external {
        assertEq(keccak256(abi.encodePacked(singletonExample.read("key"))), keccak256(abi.encodePacked("")));

        vm.expectEmit(address(singletonExample));
        emit Stored("key", "value");

        vm.prank(singletonExample.guardian());
        singletonExample.store("key", "value");

        assertEq(keccak256(abi.encodePacked(singletonExample.read("key"))), keccak256(abi.encodePacked("value")));
    }
}
