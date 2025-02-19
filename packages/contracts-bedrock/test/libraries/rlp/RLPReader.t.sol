// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { stdError } from "forge-std/Test.sol";
import { Test } from "forge-std/Test.sol";
import { RLPReader } from "src/libraries/rlp/RLPReader.sol";
import "src/libraries/rlp/RLPErrors.sol";

/// @notice Here we allow internal reverts as readRawBytes uses memory allocations and can only be tested internally
contract RLPReader_readBytes_Test is Test {
    function test_readBytes_bytestring00_succeeds() external pure {
        assertEq(RLPReader.readBytes(hex"00"), hex"00");
    }

    function test_readBytes_bytestring01_succeeds() external pure {
        assertEq(RLPReader.readBytes(hex"01"), hex"01");
    }

    function test_readBytes_bytestring7f_succeeds() external pure {
        assertEq(RLPReader.readBytes(hex"7f"), hex"7f");
    }

    /// forge-config: default.allow_internal_expect_revert = true
    function test_readBytes_revertListItem_reverts() external {
        vm.expectRevert(UnexpectedList.selector);
        RLPReader.readBytes(hex"c7c0c1c0c3c0c1c0");
    }

    /// forge-config: default.allow_internal_expect_revert = true
    function test_readBytes_invalidStringLength_reverts() external {
        vm.expectRevert(ContentLengthMismatch.selector);
        RLPReader.readBytes(hex"b9");
    }

    /// forge-config: default.allow_internal_expect_revert = true
    function test_readBytes_invalidListLength_reverts() external {
        vm.expectRevert(ContentLengthMismatch.selector);
        RLPReader.readBytes(hex"ff");
    }

    /// forge-config: default.allow_internal_expect_revert = true
    function test_readBytes_invalidRemainder_reverts() external {
        vm.expectRevert(InvalidDataRemainder.selector);
        RLPReader.readBytes(hex"800a");
    }

    /// forge-config: default.allow_internal_expect_revert = true
    function test_readBytes_invalidPrefix_reverts() external {
        vm.expectRevert(InvalidHeader.selector);
        RLPReader.readBytes(hex"810a");
    }
}

contract RLPReader_readList_Test is Test {
    function test_readList_empty_succeeds() external pure {
        RLPReader.RLPItem[] memory list = RLPReader.readList(hex"c0");
        assertEq(list.length, 0);
    }

    function test_readList_multiList_succeeds() external pure {
        RLPReader.RLPItem[] memory list = RLPReader.readList(hex"c6827a77c10401");
        assertEq(list.length, 3);

        assertEq(RLPReader.readRawBytes(list[0]), hex"827a77");
        assertEq(RLPReader.readRawBytes(list[1]), hex"c104");
        assertEq(RLPReader.readRawBytes(list[2]), hex"01");
    }

    function test_readList_shortListMax1_succeeds() external pure {
        RLPReader.RLPItem[] memory list = RLPReader.readList(
            hex"f784617364668471776572847a78637684617364668471776572847a78637684617364668471776572847a78637684617364668471776572"
        );

        assertEq(list.length, 11);
        assertEq(RLPReader.readRawBytes(list[0]), hex"8461736466");
        assertEq(RLPReader.readRawBytes(list[1]), hex"8471776572");
        assertEq(RLPReader.readRawBytes(list[2]), hex"847a786376");
        assertEq(RLPReader.readRawBytes(list[3]), hex"8461736466");
        assertEq(RLPReader.readRawBytes(list[4]), hex"8471776572");
        assertEq(RLPReader.readRawBytes(list[5]), hex"847a786376");
        assertEq(RLPReader.readRawBytes(list[6]), hex"8461736466");
        assertEq(RLPReader.readRawBytes(list[7]), hex"8471776572");
        assertEq(RLPReader.readRawBytes(list[8]), hex"847a786376");
        assertEq(RLPReader.readRawBytes(list[9]), hex"8461736466");
        assertEq(RLPReader.readRawBytes(list[10]), hex"8471776572");
    }

    function test_readList_longList1_succeeds() external pure {
        RLPReader.RLPItem[] memory list = RLPReader.readList(
            hex"f840cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376"
        );

        assertEq(list.length, 4);
        assertEq(RLPReader.readRawBytes(list[0]), hex"cf84617364668471776572847a786376");
        assertEq(RLPReader.readRawBytes(list[1]), hex"cf84617364668471776572847a786376");
        assertEq(RLPReader.readRawBytes(list[2]), hex"cf84617364668471776572847a786376");
        assertEq(RLPReader.readRawBytes(list[3]), hex"cf84617364668471776572847a786376");
    }

    function test_readList_longList2_succeeds() external pure {
        RLPReader.RLPItem[] memory list = RLPReader.readList(
            hex"f90200cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376"
        );
        assertEq(list.length, 32);

        for (uint256 i = 0; i < 32; i++) {
            assertEq(RLPReader.readRawBytes(list[i]), hex"cf84617364668471776572847a786376");
        }
    }

    /// forge-config: default.allow_internal_expect_revert = true
    function test_readList_listLongerThan32Elements_reverts() external {
        vm.expectRevert(stdError.indexOOBError);
        RLPReader.readList(hex"e1454545454545454545454545454545454545454545454545454545454545454545");
    }

    function test_readList_listOfLists_succeeds() external pure {
        RLPReader.RLPItem[] memory list = RLPReader.readList(hex"c4c2c0c0c0");
        assertEq(list.length, 2);
        assertEq(RLPReader.readRawBytes(list[0]), hex"c2c0c0");
        assertEq(RLPReader.readRawBytes(list[1]), hex"c0");
    }

    function test_readList_listOfLists2_succeeds() external pure {
        RLPReader.RLPItem[] memory list = RLPReader.readList(hex"c7c0c1c0c3c0c1c0");
        assertEq(list.length, 3);

        assertEq(RLPReader.readRawBytes(list[0]), hex"c0");
        assertEq(RLPReader.readRawBytes(list[1]), hex"c1c0");
        assertEq(RLPReader.readRawBytes(list[2]), hex"c3c0c1c0");
    }

    function test_readList_dictTest1_succeeds() external pure {
        RLPReader.RLPItem[] memory list = RLPReader.readList(
            hex"ecca846b6579318476616c31ca846b6579328476616c32ca846b6579338476616c33ca846b6579348476616c34"
        );
        assertEq(list.length, 4);

        assertEq(RLPReader.readRawBytes(list[0]), hex"ca846b6579318476616c31");
        assertEq(RLPReader.readRawBytes(list[1]), hex"ca846b6579328476616c32");
        assertEq(RLPReader.readRawBytes(list[2]), hex"ca846b6579338476616c33");
        assertEq(RLPReader.readRawBytes(list[3]), hex"ca846b6579348476616c34");
    }

    /// forge-config: default.allow_internal_expect_revert = true
    function test_readList_invalidShortList_reverts() external {
        vm.expectRevert(ContentLengthMismatch.selector);
        RLPReader.readList(hex"efdebd");
    }

    /// forge-config: default.allow_internal_expect_revert = true
    function test_readList_longStringLength_reverts() external {
        vm.expectRevert(ContentLengthMismatch.selector);
        RLPReader.readList(hex"efb83600");
    }

    /// forge-config: default.allow_internal_expect_revert = true
    function test_readList_notLongEnough_reverts() external {
        vm.expectRevert(ContentLengthMismatch.selector);
        RLPReader.readList(hex"efdebdaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa");
    }

    /// forge-config: default.allow_internal_expect_revert = true
    function test_readList_int32Overflow_reverts() external {
        vm.expectRevert(ContentLengthMismatch.selector);
        RLPReader.readList(hex"bf0f000000000000021111");
    }

    /// forge-config: default.allow_internal_expect_revert = true
    function test_readList_int32Overflow2_reverts() external {
        vm.expectRevert(ContentLengthMismatch.selector);
        RLPReader.readList(hex"ff0f000000000000021111");
    }

    /// forge-config: default.allow_internal_expect_revert = true
    function test_readList_incorrectLengthInArray_reverts() external {
        vm.expectRevert(InvalidHeader.selector);
        RLPReader.readList(hex"b9002100dc2b275d0f74e8a53e6f4ec61b27f24278820be3f82ea2110e582081b0565df0");
    }

    /// forge-config: default.allow_internal_expect_revert = true
    function test_readList_leadingZerosInLongLengthArray1_reverts() external {
        vm.expectRevert(InvalidHeader.selector);
        RLPReader.readList(
            hex"b90040000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f"
        );
    }

    /// forge-config: default.allow_internal_expect_revert = true
    function test_readList_leadingZerosInLongLengthArray2_reverts() external {
        vm.expectRevert(InvalidHeader.selector);
        RLPReader.readList(hex"b800");
    }

    /// forge-config: default.allow_internal_expect_revert = true
    function test_readList_leadingZerosInLongLengthList1_reverts() external {
        vm.expectRevert(InvalidHeader.selector);
        RLPReader.readList(
            hex"fb00000040000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f"
        );
    }

    /// forge-config: default.allow_internal_expect_revert = true
    function test_readList_nonOptimalLongLengthArray1_reverts() external {
        vm.expectRevert(InvalidHeader.selector);
        RLPReader.readList(hex"b81000112233445566778899aabbccddeeff");
    }

    /// forge-config: default.allow_internal_expect_revert = true
    function test_readList_nonOptimalLongLengthArray2_reverts() external {
        vm.expectRevert(InvalidHeader.selector);
        RLPReader.readList(hex"b801ff");
    }

    /// forge-config: default.allow_internal_expect_revert = true
    function test_readList_invalidValue_reverts() external {
        vm.expectRevert(ContentLengthMismatch.selector);
        RLPReader.readList(hex"91");
    }

    /// forge-config: default.allow_internal_expect_revert = true
    function test_readList_invalidRemainder_reverts() external {
        vm.expectRevert(InvalidDataRemainder.selector);
        RLPReader.readList(hex"c000");
    }

    /// forge-config: default.allow_internal_expect_revert = true
    function test_readList_notEnoughContentForString1_reverts() external {
        vm.expectRevert(ContentLengthMismatch.selector);
        RLPReader.readList(hex"ba010000aabbccddeeff");
    }

    /// forge-config: default.allow_internal_expect_revert = true
    function test_readList_notEnoughContentForString2_reverts() external {
        vm.expectRevert(ContentLengthMismatch.selector);
        RLPReader.readList(hex"b840ffeeddccbbaa99887766554433221100");
    }

    /// forge-config: default.allow_internal_expect_revert = true
    function test_readList_notEnoughContentForList1_reverts() external {
        vm.expectRevert(ContentLengthMismatch.selector);
        RLPReader.readList(hex"f90180");
    }

    /// forge-config: default.allow_internal_expect_revert = true
    function test_readList_notEnoughContentForList2_reverts() external {
        vm.expectRevert(ContentLengthMismatch.selector);
        RLPReader.readList(hex"ffffffffffffffffff0001020304050607");
    }

    /// forge-config: default.allow_internal_expect_revert = true
    function test_readList_longStringLessThan56Bytes_reverts() external {
        vm.expectRevert(InvalidHeader.selector);
        RLPReader.readList(hex"b80100");
    }

    /// forge-config: default.allow_internal_expect_revert = true
    function test_readList_longListLessThan56Bytes_reverts() external {
        vm.expectRevert(InvalidHeader.selector);
        RLPReader.readList(hex"f80100");
    }
}
