// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { Test } from "forge-std/Test.sol";

import {
    DeployPreimageOracle,
    DeployPreimageOracleInput,
    DeployPreimageOracleOutput
} from "scripts/deploy/DeployPreimageOracle.s.sol";

contract DeployPreimageOracleInput_Test is Test {
    DeployPreimageOracleInput input;

    function setUp() public {
        input = new DeployPreimageOracleInput();
    }

    function test_getters_whenNotSet_reverts() public {
        vm.expectRevert("DeployPreimageOracleInput: not set");
        input.minProposalSize();

        vm.expectRevert("DeployPreimageOracleInput: not set");
        input.challengePeriod();
    }

    function test_set_succeeds() public {
        uint256 minProposalSize = 1000;
        uint256 challengePeriod = 7 days;

        input.set(input.minProposalSize.selector, minProposalSize);
        input.set(input.challengePeriod.selector, challengePeriod);

        assertEq(input.minProposalSize(), minProposalSize);
        assertEq(input.challengePeriod(), challengePeriod);
    }

    function test_set_withInvalidSelector_reverts() public {
        vm.expectRevert("DeployPreimageOracleInput: unknown selector");
        input.set(bytes4(0xdeadbeef), 100);
    }
}

contract DeployPreimageOracleOutput_Test is Test {
    DeployPreimageOracleOutput output;
    address mockOracle;

    function setUp() public {
        output = new DeployPreimageOracleOutput();
        mockOracle = makeAddr("oracle");
        vm.etch(mockOracle, hex"01");
    }

    function test_getters_whenNotSet_reverts() public {
        vm.expectRevert("DeployPreimageOracleOutput: not set");
        output.preimageOracle();
    }

    function test_set_succeeds() public {
        output.set(output.preimageOracle.selector, mockOracle);
        assertEq(address(output.preimageOracle()), mockOracle);
    }

    function test_set_withZeroAddress_reverts() public {
        vm.expectRevert("DeployPreimageOracleOutput: cannot set zero address");
        output.set(output.preimageOracle.selector, address(0));
    }

    function test_set_withInvalidSelector_reverts() public {
        vm.expectRevert("DeployPreimageOracleOutput: unknown selector");
        output.set(bytes4(0xdeadbeef), mockOracle);
    }
}

contract DeployPreimageOracle_Test is Test {
    DeployPreimageOracle script;
    DeployPreimageOracleInput input;
    DeployPreimageOracleOutput output;

    uint256 minProposalSize;
    uint256 challengePeriod;

    function setUp() public {
        script = new DeployPreimageOracle();
        input = new DeployPreimageOracleInput();
        output = new DeployPreimageOracleOutput();

        minProposalSize = 1000;
        challengePeriod = 7 days;
    }

    function test_run_succeeds() public {
        input.set(input.minProposalSize.selector, minProposalSize);
        input.set(input.challengePeriod.selector, challengePeriod);

        script.run(input, output);

        assertTrue(address(output.preimageOracle()) != address(0));
    }

    function test_assertValid_whenInvalid_reverts() public {
        vm.expectRevert("DeployPreimageOracleOutput: not set");
        script.assertValid(input, output);
    }
}
