// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { Test } from "forge-std/Test.sol";
import { DeployAlphabetVM, DeployAlphabetVMInput, DeployAlphabetVMOutput } from "scripts/deploy/DeployAlphabetVM.s.sol";

contract DeployAlphabetVMInput_Test is Test {
    DeployAlphabetVMInput input;

    function setUp() public {
        input = new DeployAlphabetVMInput();
    }

    function test_getters_whenNotSet_reverts() public {
        vm.expectRevert("DeployAlphabetVMInput: not set");
        input.preimageOracle();

        vm.expectRevert("DeployAlphabetVMInput: not set");
        input.absolutePrestate();
    }

    function test_set_succeeds() public {
        address oracle = makeAddr("oracle");
        bytes32 prestate = bytes32(uint256(1));

        vm.etch(oracle, hex"01");

        input.set(input.preimageOracle.selector, oracle);
        input.set(input.absolutePrestate.selector, prestate);

        assertEq(address(input.preimageOracle()), oracle);
        assertEq(input.absolutePrestate(), prestate);
    }

    function test_set_withZeroAddress_reverts() public {
        vm.expectRevert("DeployAlphabetVMInput: cannot set zero address");
        input.set(input.preimageOracle.selector, address(0));
    }

    function test_set_withInvalidSelector_reverts() public {
        vm.expectRevert("DeployAlphabetVMInput: unknown selector");
        input.set(bytes4(0xdeadbeef), makeAddr("test"));

        vm.expectRevert("DeployAlphabetVMInput: unknown selector");
        input.set(bytes4(0xdeadbeef), bytes32(0));
    }
}

contract DeployAlphabetVMOutput_Test is Test {
    DeployAlphabetVMOutput output;
    address mockVM;

    function setUp() public {
        output = new DeployAlphabetVMOutput();
        mockVM = makeAddr("vm");
        vm.etch(mockVM, hex"01");
    }

    function test_getters_whenNotSet_reverts() public {
        vm.expectRevert("DeployAlphabetVMOutput: not set");
        output.alphabetVM();
    }

    function test_set_succeeds() public {
        output.set(output.alphabetVM.selector, mockVM);
        assertEq(address(output.alphabetVM()), mockVM);
    }

    function test_set_withZeroAddress_reverts() public {
        vm.expectRevert("DeployAlphabetVMOutput: cannot set zero address");
        output.set(output.alphabetVM.selector, address(0));
    }

    function test_set_withInvalidSelector_reverts() public {
        vm.expectRevert("DeployAlphabetVMOutput: unknown selector");
        output.set(bytes4(0xdeadbeef), mockVM);
    }
}

contract DeployAlphabetVM_Test is Test {
    DeployAlphabetVM script;
    DeployAlphabetVMInput input;
    DeployAlphabetVMOutput output;
    address mockOracle;
    bytes32 mockPrestate;

    function setUp() public {
        script = new DeployAlphabetVM();
        input = new DeployAlphabetVMInput();
        output = new DeployAlphabetVMOutput();
        mockOracle = makeAddr("oracle");
        mockPrestate = bytes32(uint256(1));
    }

    function test_run_succeeds() public {
        input.set(input.preimageOracle.selector, mockOracle);
        input.set(input.absolutePrestate.selector, mockPrestate);
        script.run(input, output);
        require(address(output.alphabetVM()) != address(0), "DeployAlphabetVM_Test: alphabetVM not set");
    }
}
