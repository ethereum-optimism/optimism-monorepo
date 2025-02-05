// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { Test } from "forge-std/Test.sol";
import { ISystemConfigInterop } from "interfaces/L1/ISystemConfigInterop.sol";
import { ManageDependencies, ManageDependenciesInput } from "scripts/deploy/ManageDependencies.s.sol";

contract ManageDependencies_Test is Test {
    ManageDependencies script;
    ManageDependenciesInput input;
    address mockSystemConfig;
    uint256 testChainId;

    event DependencyAdded(uint256 indexed chainId);
    event DependencyRemoved(uint256 indexed chainId);

    function setUp() public {
        script = new ManageDependencies();
        input = new ManageDependenciesInput();
        mockSystemConfig = makeAddr("systemConfig");
        testChainId = 123;

        vm.etch(mockSystemConfig, hex"01");
    }

    function test_run_add_succeeds() public {
        input.set(input.systemConfig.selector, mockSystemConfig);
        input.set(input.chainId.selector, testChainId);
        input.set(input.remove.selector, false);

        // Expect the addDependency call
        vm.mockCall(mockSystemConfig, abi.encodeCall(ISystemConfigInterop.addDependency, testChainId), bytes(""));
        script.run(input);
    }

    function test_run_remove_succeeds() public {
        input.set(input.systemConfig.selector, mockSystemConfig);
        input.set(input.chainId.selector, testChainId);
        input.set(input.remove.selector, true);

        vm.mockCall(mockSystemConfig, abi.encodeCall(ISystemConfigInterop.removeDependency, testChainId), bytes(""));
        script.run(input);
    }
}

contract ManageDependenciesInput_Test is Test {
    ManageDependenciesInput input;

    function setUp() public {
        input = new ManageDependenciesInput();
    }

    function test_getters_whenNotSet_reverts() public {
        vm.expectRevert("ManageDependenciesInput: not set");
        input.chainId();

        vm.expectRevert("ManageDependenciesInput: not set");
        input.systemConfig();

        // remove() doesn't revert when not set, returns false
        assertFalse(input.remove());
    }

    function test_set_succeeds() public {
        address systemConfig = makeAddr("systemConfig");
        uint256 chainId = 123;
        bool remove = true;

        vm.etch(systemConfig, hex"01");

        input.set(input.systemConfig.selector, systemConfig);
        input.set(input.chainId.selector, chainId);
        input.set(input.remove.selector, remove);

        assertEq(address(input.systemConfig()), systemConfig);
        assertEq(input.chainId(), chainId);
        assertTrue(input.remove());
    }

    function test_set_withZeroAddress_reverts() public {
        vm.expectRevert("ManageDependenciesInput: cannot set zero address");
        input.set(input.systemConfig.selector, address(0));
    }

    function test_set_withInvalidSelector_reverts() public {
        vm.expectRevert("ManageDependenciesInput: unknown selector");
        input.set(bytes4(0xdeadbeef), makeAddr("test"));

        vm.expectRevert("ManageDependenciesInput: unknown selector");
        input.set(bytes4(0xdeadbeef), uint256(1));

        vm.expectRevert("ManageDependenciesInput: unknown selector");
        input.set(bytes4(0xdeadbeef), true);
    }
}
