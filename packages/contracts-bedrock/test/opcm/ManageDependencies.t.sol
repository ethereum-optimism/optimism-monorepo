// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { Test } from "forge-std/Test.sol";
import { ManageDependencies, ManageDependenciesInput } from "scripts/deploy/ManageDependencies.s.sol";

contract ManageDependencies_Test is Test {
    ManageDependencies script;
    ManageDependenciesInput input;
    address mockSystemConfig;
    address mockSuperchainConfig;
    uint256 testChainId;

    function setUp() public {
        script = new ManageDependencies();
        input = new ManageDependenciesInput();
        mockSystemConfig = makeAddr("systemConfig");
        mockSuperchainConfig = makeAddr("superchainConfig");
        testChainId = 123;

        vm.etch(mockSystemConfig, hex"01");
        vm.etch(mockSuperchainConfig, hex"01");
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

        vm.expectRevert("ManageDependenciesInput: not set");
        input.superchainConfig();
    }

    function test_set_succeeds() public {
        address systemConfig = makeAddr("systemConfig");
        address superchainConfig = makeAddr("superchainConfig");
        uint256 chainId = 123;

        vm.etch(systemConfig, hex"01");
        vm.etch(superchainConfig, hex"01");

        input.set(input.systemConfig.selector, systemConfig);
        input.set(input.superchainConfig.selector, superchainConfig);
        input.set(input.chainId.selector, chainId);

        assertEq(address(input.systemConfig()), systemConfig);
        assertEq(address(input.superchainConfig()), superchainConfig);
        assertEq(input.chainId(), chainId);
    }

    function test_set_withZeroAddress_reverts() public {
        vm.expectRevert("ManageDependenciesInput: cannot set zero address");
        input.set(input.systemConfig.selector, address(0));

        vm.expectRevert("ManageDependenciesInput: cannot set zero address");
        input.set(input.superchainConfig.selector, address(0));
    }

    function test_set_withInvalidSelector_reverts() public {
        vm.expectRevert("ManageDependenciesInput: unknown selector");
        input.set(bytes4(0xdeadbeef), makeAddr("test"));

        vm.expectRevert("ManageDependenciesInput: unknown selector");
        input.set(bytes4(0xdeadbeef), uint256(1));
    }
}
