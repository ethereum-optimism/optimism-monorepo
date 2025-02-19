// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { stdError } from "forge-std/Test.sol";
import { Test } from "forge-std/Test.sol";
import { RLPReader } from "src/libraries/rlp/RLPReader.sol";
import "src/libraries/rlp/RLPErrors.sol";

library RLPReader_Harness {
    function readBytes(bytes memory _in) public pure returns (bytes memory) {
        return RLPReader.readBytes(_in);
    }

    function readList(bytes memory _in) public pure returns (RLPReader.RLPItem[] memory) {
        return RLPReader.readList(_in);
    }

    function readList(RLPReader.RLPItem memory _in) public pure returns (RLPReader.RLPItem[] memory) {
        return RLPReader.readList(_in);
    }

    function readRawBytes(RLPReader.RLPItem memory _in) public pure returns (bytes memory out_) {
        return RLPReader.readRawBytes(_in);
    }
}

contract RLPReader_readBytes_Test is Test {
    function test_readBytes_bytestring00_succeeds() external pure {
        assertEq(RLPReader_Harness.readBytes(hex"00"), hex"00");
    }

    function test_readBytes_bytestring01_succeeds() external pure {
        assertEq(RLPReader_Harness.readBytes(hex"01"), hex"01");
    }

    function test_readBytes_bytestring7f_succeeds() external pure {
        assertEq(RLPReader_Harness.readBytes(hex"7f"), hex"7f");
    }

    function test_readBytes_revertListItem_reverts() external {
        vm.expectRevert(UnexpectedList.selector);
        RLPReader_Harness.readBytes(hex"c7c0c1c0c3c0c1c0");
    }

    function test_readBytes_invalidStringLength_reverts() external {
        vm.expectRevert(ContentLengthMismatch.selector);
        RLPReader_Harness.readBytes(hex"b9");
    }

    function test_readBytes_invalidListLength_reverts() external {
        vm.expectRevert(ContentLengthMismatch.selector);
        RLPReader_Harness.readBytes(hex"ff");
    }

    function test_readBytes_invalidRemainder_reverts() external {
        vm.expectRevert(InvalidDataRemainder.selector);
        RLPReader_Harness.readBytes(hex"800a");
    }

    function test_readBytes_invalidPrefix_reverts() external {
        vm.expectRevert(InvalidHeader.selector);
        RLPReader_Harness.readBytes(hex"810a");
    }
}

contract RLPReader_readList_Test is Test {
    function test_readList_empty_succeeds() external pure {
        RLPReader.RLPItem[] memory list = RLPReader_Harness.readList(hex"c0");
        assertEq(list.length, 0);
    }

    function test_readList_multiList_succeeds() external pure {
        RLPReader.RLPItem[] memory list = RLPReader_Harness.readList(hex"c6827a77c10401");
        assertEq(list.length, 3);

        assertEq(RLPReader_Harness.readRawBytes(list[0]), hex"827a77");
        assertEq(RLPReader_Harness.readRawBytes(list[1]), hex"c104");
        assertEq(RLPReader_Harness.readRawBytes(list[2]), hex"01");
    }

    function test_readList_shortListMax1_succeeds() external pure {
        RLPReader.RLPItem[] memory list = RLPReader_Harness.readList(
            hex"f784617364668471776572847a78637684617364668471776572847a78637684617364668471776572847a78637684617364668471776572"
        );

        assertEq(list.length, 11);
        assertEq(RLPReader_Harness.readRawBytes(list[0]), hex"8461736466");
        assertEq(RLPReader_Harness.readRawBytes(list[1]), hex"8471776572");
        assertEq(RLPReader_Harness.readRawBytes(list[2]), hex"847a786376");
        assertEq(RLPReader_Harness.readRawBytes(list[3]), hex"8461736466");
        assertEq(RLPReader_Harness.readRawBytes(list[4]), hex"8471776572");
        assertEq(RLPReader_Harness.readRawBytes(list[5]), hex"847a786376");
        assertEq(RLPReader_Harness.readRawBytes(list[6]), hex"8461736466");
        assertEq(RLPReader_Harness.readRawBytes(list[7]), hex"8471776572");
        assertEq(RLPReader_Harness.readRawBytes(list[8]), hex"847a786376");
        assertEq(RLPReader_Harness.readRawBytes(list[9]), hex"8461736466");
        assertEq(RLPReader_Harness.readRawBytes(list[10]), hex"8471776572");
    }

    function test_readList_longList1_succeeds() external pure {
        RLPReader.RLPItem[] memory list = RLPReader_Harness.readList(
            hex"f840cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376"
        );

        assertEq(list.length, 4);
        assertEq(RLPReader_Harness.readRawBytes(list[0]), hex"cf84617364668471776572847a786376");
        assertEq(RLPReader_Harness.readRawBytes(list[1]), hex"cf84617364668471776572847a786376");
        assertEq(RLPReader_Harness.readRawBytes(list[2]), hex"cf84617364668471776572847a786376");
        assertEq(RLPReader_Harness.readRawBytes(list[3]), hex"cf84617364668471776572847a786376");
    }

    function test_readList_longList2_succeeds() external pure {
        RLPReader.RLPItem[] memory list = RLPReader_Harness.readList(
            hex"f90200cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376"
        );
        assertEq(list.length, 32);

        for (uint256 i = 0; i < 32; i++) {
            assertEq(RLPReader_Harness.readRawBytes(list[i]), hex"cf84617364668471776572847a786376");
        }
    }

    function test_readList_listLongerThan32Elements_reverts() external {
        vm.expectRevert(stdError.indexOOBError);
        RLPReader_Harness.readList(hex"e1454545454545454545454545454545454545454545454545454545454545454545");
    }

    function test_readList_listOfLists_succeeds() external pure {
        RLPReader.RLPItem[] memory list = RLPReader_Harness.readList(hex"c4c2c0c0c0");
        assertEq(list.length, 2);
        assertEq(RLPReader_Harness.readRawBytes(list[0]), hex"c2c0c0");
        assertEq(RLPReader_Harness.readRawBytes(list[1]), hex"c0");
    }

    function test_readList_listOfLists2_succeeds() external pure {
        RLPReader.RLPItem[] memory list = RLPReader_Harness.readList(hex"c7c0c1c0c3c0c1c0");
        assertEq(list.length, 3);

        assertEq(RLPReader_Harness.readRawBytes(list[0]), hex"c0");
        assertEq(RLPReader_Harness.readRawBytes(list[1]), hex"c1c0");
        assertEq(RLPReader_Harness.readRawBytes(list[2]), hex"c3c0c1c0");
    }

    function test_readList_dictTest1_succeeds() external pure {
        RLPReader.RLPItem[] memory list = RLPReader_Harness.readList(
            hex"ecca846b6579318476616c31ca846b6579328476616c32ca846b6579338476616c33ca846b6579348476616c34"
        );
        assertEq(list.length, 4);

        assertEq(RLPReader_Harness.readRawBytes(list[0]), hex"ca846b6579318476616c31");
        assertEq(RLPReader_Harness.readRawBytes(list[1]), hex"ca846b6579328476616c32");
        assertEq(RLPReader_Harness.readRawBytes(list[2]), hex"ca846b6579338476616c33");
        assertEq(RLPReader_Harness.readRawBytes(list[3]), hex"ca846b6579348476616c34");
    }

    function test_readList_invalidShortList_reverts() external {
        vm.expectRevert(ContentLengthMismatch.selector);
        RLPReader_Harness.readList(hex"efdebd");
    }

    function test_readList_longStringLength_reverts() external {
        vm.expectRevert(ContentLengthMismatch.selector);
        RLPReader_Harness.readList(hex"efb83600");
    }

    function test_readList_notLongEnough_reverts() external {
        vm.expectRevert(ContentLengthMismatch.selector);
        RLPReader_Harness.readList(hex"efdebdaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa");
    }

    function test_readList_int32Overflow_reverts() external {
        vm.expectRevert(ContentLengthMismatch.selector);
        RLPReader_Harness.readList(hex"bf0f000000000000021111");
    }

    function test_readList_int32Overflow2_reverts() external {
        vm.expectRevert(ContentLengthMismatch.selector);
        RLPReader_Harness.readList(hex"ff0f000000000000021111");
    }

    function test_readList_incorrectLengthInArray_reverts() external {
        vm.expectRevert(InvalidHeader.selector);
        RLPReader_Harness.readList(hex"b9002100dc2b275d0f74e8a53e6f4ec61b27f24278820be3f82ea2110e582081b0565df0");
    }

    function test_readList_leadingZerosInLongLengthArray1_reverts() external {
        vm.expectRevert(InvalidHeader.selector);
        RLPReader_Harness.readList(
            hex"b90040000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f"
        );
    }

    function test_readList_leadingZerosInLongLengthArray2_reverts() external {
        vm.expectRevert(InvalidHeader.selector);
        RLPReader_Harness.readList(hex"b800");
    }

    function test_readList_leadingZerosInLongLengthList1_reverts() external {
        vm.expectRevert(InvalidHeader.selector);
        RLPReader_Harness.readList(
            hex"fb00000040000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f"
        );
    }

    function test_readList_nonOptimalLongLengthArray1_reverts() external {
        vm.expectRevert(InvalidHeader.selector);
        RLPReader_Harness.readList(hex"b81000112233445566778899aabbccddeeff");
    }

    function test_readList_nonOptimalLongLengthArray2_reverts() external {
        vm.expectRevert(InvalidHeader.selector);
        RLPReader_Harness.readList(hex"b801ff");
    }

    function test_readList_invalidValue_reverts() external {
        vm.expectRevert(ContentLengthMismatch.selector);
        RLPReader_Harness.readList(hex"91");
    }

    function test_readList_invalidRemainder_reverts() external {
        vm.expectRevert(InvalidDataRemainder.selector);
        RLPReader_Harness.readList(hex"c000");
    }

    function test_readList_notEnoughContentForString1_reverts() external {
        vm.expectRevert(ContentLengthMismatch.selector);
        RLPReader_Harness.readList(hex"ba010000aabbccddeeff");
    }

    function test_readList_notEnoughContentForString2_reverts() external {
        vm.expectRevert(ContentLengthMismatch.selector);
        RLPReader_Harness.readList(hex"b840ffeeddccbbaa99887766554433221100");
    }

    function test_readList_notEnoughContentForList1_reverts() external {
        vm.expectRevert(ContentLengthMismatch.selector);
        RLPReader_Harness.readList(hex"f90180");
    }

    function test_readList_notEnoughContentForList2_reverts() external {
        vm.expectRevert(ContentLengthMismatch.selector);
        RLPReader_Harness.readList(hex"ffffffffffffffffff0001020304050607");
    }

    function test_readList_longStringLessThan56Bytes_reverts() external {
        vm.expectRevert(InvalidHeader.selector);
        RLPReader_Harness.readList(hex"b80100");
    }

    function test_readList_longListLessThan56Bytes_reverts() external {
        vm.expectRevert(InvalidHeader.selector);
        RLPReader_Harness.readList(hex"f80100");
    }
}
