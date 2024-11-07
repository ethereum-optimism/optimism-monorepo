// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Test } from "forge-std/Test.sol";
import { Blueprint } from "src/libraries/Blueprint.sol";

// Used to test that constructor args are appended properly when deploying from a blueprint.
contract ConstructorArgMock {
    uint256 public x;
    bytes public y;

    constructor(uint256 _x, bytes memory _y) {
        x = _x;
        y = _y;
    }
}

// Foundry cheatcodes operate on the next call, and since all library methods are internal we would
// just JUMP to them if called directly in the test. Therefore we wrap the library in a contract.
contract BlueprintHarness {
    function blueprintDeployerBytecode(bytes memory _initcode) public pure returns (bytes memory) {
        return Blueprint.blueprintDeployerBytecode(_initcode);
    }

    function parseBlueprintPreamble(bytes memory _bytecode) public view returns (Blueprint.Preamble memory) {
        return Blueprint.parseBlueprintPreamble(_bytecode);
    }

    function deployFrom(address _blueprint, bytes32 _salt) public returns (address) {
        return Blueprint.deployFrom(_blueprint, _salt);
    }

    function deployFrom(address _blueprint, bytes32 _salt, bytes memory _args) public returns (address) {
        return Blueprint.deployFrom(_blueprint, _salt, _args);
    }

    function bytesToUint(bytes memory _bytes) public pure returns (uint256) {
        return Blueprint.bytesToUint(_bytes);
    }
}

contract Blueprint_Test is Test {
    BlueprintHarness blueprint;

    function setUp() public {
        blueprint = new BlueprintHarness();
    }

    function deployWithCreate2(bytes memory _initcode, bytes32 _salt) public returns (address addr_) {
        assembly ("memory-safe") {
            addr_ := create2(0, add(_initcode, 0x20), mload(_initcode), _salt)
        }
        require(addr_ != address(0), "deployWithCreate2: deployment failed");
    }

    // --- We start with the test cases from ERC-5202 ---

    // An example (and trivial!) blueprint contract with no data section, whose initcode is just the STOP instruction.
    function test_trivialBlueprint_erc5202_succeeds() public view {
        bytes memory bytecode = hex"FE710000";
        Blueprint.Preamble memory preamble = blueprint.parseBlueprintPreamble(bytecode);

        assertEq(preamble.ercVersion, 0, "100");
        assertEq(preamble.preambleData, hex"", "200");
        assertEq(preamble.initcode, hex"00", "300");
    }

    // An example blueprint contract whose initcode is the trivial STOP instruction and whose data
    // section contains the byte 0xFF repeated seven times.
    function test_blueprintWithDataSection_erc5202_succeeds() public view {
        // Here, 0xFE71 is the magic header, 0x01 means version 0 + 1 length bit, 0x07 encodes the
        // length in bytes of the data section. These are followed by the data section, and then the
        // initcode. For illustration, this code with delimiters would be:
        //   0xFE71|01|07|FFFFFFFFFFFFFF|00
        bytes memory bytecode = hex"FE710107FFFFFFFFFFFFFF00";
        Blueprint.Preamble memory preamble = blueprint.parseBlueprintPreamble(bytecode);

        assertEq(preamble.ercVersion, 0, "100");
        assertEq(preamble.preambleData, hex"FFFFFFFFFFFFFF", "200");
        assertEq(preamble.initcode, hex"00", "300");
    }

    // An example blueprint whose initcode is the trivial STOP instruction and whose data section
    // contains the byte 0xFF repeated 256 times.
    function test_blueprintWithLargeDataSection_erc5205_succeeds() public view {
        // Delimited, this would be 0xFE71|02|0100|FF...FF|00
        bytes memory bytecode =
            hex"FE71020100FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00";
        Blueprint.Preamble memory preamble = blueprint.parseBlueprintPreamble(bytecode);

        assertEq(preamble.ercVersion, 0, "100");
        assertEq(preamble.preambleData.length, 256, "200");
        for (uint256 i = 0; i < 256; i++) {
            assertEq(preamble.preambleData[i], bytes1(0xFF), string.concat("300-", vm.toString(i)));
        }
        assertEq(preamble.initcode, hex"00", "400");
    }

    // --- Now we add a generic roundtrip test ---

    // Test that a roundtrip from initcode to blueprint to initcode succeeds, i.e. the invariant
    // here is that `parseBlueprintPreamble(blueprintDeployerBytecode(x)) = x`.
    function testFuzz_roundtrip_succeeds(bytes memory _initcode) public {
        vm.assume(_initcode.length > 0);

        // Convert the initcode to match the ERC-5202 blueprint format.
        bytes memory blueprintInitcode = blueprint.blueprintDeployerBytecode(_initcode);

        // Deploy the blueprint.
        address blueprintAddress = deployWithCreate2(blueprintInitcode, bytes32(0));

        // Read the blueprint code from the deployed code.
        bytes memory blueprintCode = address(blueprintAddress).code;

        // Parse the blueprint preamble and ensure it matches the expected values.
        Blueprint.Preamble memory preamble = blueprint.parseBlueprintPreamble(blueprintCode);
        assertEq(preamble.ercVersion, 0, "100");
        assertEq(preamble.preambleData, hex"", "200");
        assertEq(preamble.initcode, _initcode, "300");
    }

    // --- Lastly, function-specific unit tests ---

    function test_blueprintDeployerBytecode_emptyInitcode_reverts() public {
        bytes memory initcode = "";
        vm.expectRevert(Blueprint.EmptyInitcode.selector);
        blueprint.blueprintDeployerBytecode(initcode);
    }

    function test_parseBlueprintPreamble_notABlueprint_reverts() public {
        // Length too short.
        bytes memory invalidBytecode = hex"01";
        vm.expectRevert(Blueprint.NotABlueprint.selector);
        blueprint.parseBlueprintPreamble(invalidBytecode);

        // First byte is not 0xFE.
        invalidBytecode = hex"0071";
        vm.expectRevert(Blueprint.NotABlueprint.selector);
        blueprint.parseBlueprintPreamble(invalidBytecode);

        // Second byte is not 0x71.
        invalidBytecode = hex"FE00";
        vm.expectRevert(Blueprint.NotABlueprint.selector);
        blueprint.parseBlueprintPreamble(invalidBytecode);
    }

    function test_parseBlueprintPreamble_reservedBitsSet_reverts() public {
        bytes memory invalidBytecode = hex"FE7103";
        vm.expectRevert(Blueprint.ReservedBitsSet.selector);
        blueprint.parseBlueprintPreamble(invalidBytecode);
    }

    function test_parseBlueprintPreamble_emptyInitcode_reverts() public {
        bytes memory invalidBytecode = hex"FE7100";
        vm.expectRevert(Blueprint.EmptyInitcode.selector);
        blueprint.parseBlueprintPreamble(invalidBytecode);
    }

    function testFuzz_deployFrom_succeeds(bytes memory _initcode, bytes32 _salt) public {
        vm.assume(_initcode.length > 0);
        vm.assume(_initcode[0] != 0xef); // https://eips.ethereum.org/EIPS/eip-3541

        // This deployBytecode prefix is the same bytecode used in `blueprintDeployerBytecode`, and
        // it ensures that whatever initcode the fuzzer generates is actually deployable.
        bytes memory deployBytecode = bytes.concat(hex"61", bytes2(uint16(_initcode.length)), hex"3d81600a3d39f3");
        bytes memory initcode = bytes.concat(deployBytecode, _initcode);
        bytes memory blueprintInitcode = blueprint.blueprintDeployerBytecode(initcode);

        // Deploy the blueprint.
        address blueprintAddress = deployWithCreate2(blueprintInitcode, _salt);

        // Deploy from the blueprint.
        address deployedContract = Blueprint.deployFrom(blueprintAddress, _salt);

        // Verify the deployment worked.
        assertTrue(deployedContract != address(0), "100");
        assertTrue(deployedContract.code.length > 0, "200");
        assertEq(keccak256(deployedContract.code), keccak256(_initcode), "300");
    }

    // Here we deploy a simple mock contract to test that constructor args are appended properly.
    function testFuzz_deployFrom_withConstructorArgs_succeeds(uint256 _x, bytes memory _y, bytes32 _salt) public {
        bytes memory blueprintInitcode = blueprint.blueprintDeployerBytecode(type(ConstructorArgMock).creationCode);

        // Deploy the blueprint.
        address blueprintAddress = deployWithCreate2(blueprintInitcode, _salt);

        // Deploy from the blueprint.
        bytes memory args = abi.encode(_x, _y);
        address deployedContract = blueprint.deployFrom(blueprintAddress, _salt, args);

        // Verify the deployment worked.
        assertTrue(deployedContract != address(0), "100");
        assertTrue(deployedContract.code.length > 0, "200");
        assertEq(keccak256(deployedContract.code), keccak256(type(ConstructorArgMock).runtimeCode), "300");
        assertEq(ConstructorArgMock(deployedContract).x(), _x, "400");
        assertEq(ConstructorArgMock(deployedContract).y(), _y, "500");
    }

    function test_deployFrom_unsupportedERCVersion_reverts() public {
        bytes32 salt = bytes32(0);
        address blueprintAddress = makeAddr("blueprint");

        bytes memory invalidBlueprintCode = hex"FE710400"; // ercVersion = uint8(0x04 & 0xfc) >> 2 = 1
        vm.etch(blueprintAddress, invalidBlueprintCode);
        vm.expectRevert(abi.encodeWithSelector(Blueprint.UnsupportedERCVersion.selector, 1));
        blueprint.deployFrom(blueprintAddress, salt);

        invalidBlueprintCode = hex"FE71B000"; // ercVersion = uint8(0xB0 & 0xfc) >> 2 = 44
        vm.etch(blueprintAddress, invalidBlueprintCode);
        vm.expectRevert(abi.encodeWithSelector(Blueprint.UnsupportedERCVersion.selector, 44));
        blueprint.deployFrom(blueprintAddress, salt);
    }

    function test_deployFrom_unexpectedPreambleData_reverts() public {
        bytes32 salt = bytes32(0);
        address blueprintAddress = makeAddr("blueprint");

        // Create invalid blueprint code with non-empty preamble data
        bytes memory invalidBlueprintCode = hex"FE7101030102030001020304";
        vm.etch(blueprintAddress, invalidBlueprintCode);

        // Expect revert with UnexpectedPreambleData error
        vm.expectRevert(abi.encodeWithSelector(Blueprint.UnexpectedPreambleData.selector, hex"010203"));
        blueprint.deployFrom(blueprintAddress, salt);
    }

    function test_bytesToUint_succeeds() public view {
        // These test cases (and the logic for bytesToUint) are taken from forge-std.
        assertEq(3, blueprint.bytesToUint(hex"03"));
        assertEq(2, blueprint.bytesToUint(hex"02"));
        assertEq(255, blueprint.bytesToUint(hex"ff"));
        assertEq(29625, blueprint.bytesToUint(hex"73b9"));

        // Additional test cases.
        assertEq(0, blueprint.bytesToUint(hex""));
        assertEq(0, blueprint.bytesToUint(hex"00"));
        assertEq(3, blueprint.bytesToUint(hex"0003"));
        assertEq(3145731, blueprint.bytesToUint(hex"300003"));
        assertEq(14545064521499334880, blueprint.bytesToUint(hex"c9da731e871ad8e0"));
        assertEq(14545064521499334880, blueprint.bytesToUint(hex"00c9da731e871ad8e0"));
        assertEq(type(uint256).max, blueprint.bytesToUint(bytes.concat(bytes32(type(uint256).max))));
    }
}
