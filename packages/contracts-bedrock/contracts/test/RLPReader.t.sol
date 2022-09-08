// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { RLPReader } from "../libraries/rlp/RLPReader.sol";
import { CommonTest } from "./CommonTest.t.sol";
import { stdError } from "forge-std/Test.sol";

contract RLPReader_Test is CommonTest {
    function test_readBytes_bytestring00() external {
        assertEq(
            RLPReader.readBytes(hex"00"),
            hex"00"
        );
    }

    function test_readBytes_bytestring01() external {
        assertEq(
            RLPReader.readBytes(hex"01"),
            hex"01"
        );
    }

    function test_readBytes_bytestring7f() external {
        assertEq(
            RLPReader.readBytes(hex"7f"),
            hex"7f"
        );
    }

    function test_readBytes_revertListItem() external {
        vm.expectRevert("RLPReader: decoded item type for bytes is not a data item");
        RLPReader.readBytes(hex"c7c0c1c0c3c0c1c0");
    }

    function test_readBytes_invalidStringLength() external {
        vm.expectRevert("RLPReader: length of content must be greater than length of string length (long strong)");
        RLPReader.readBytes(hex"b9");
    }

    function test_readBytes_invalidListLength() external {
        vm.expectRevert("RLPReader: length of content must be greater than length of list length (long list)");
        RLPReader.readBytes(hex"ff");
    }

    function test_readBytes_invalidRemainder() external {
        vm.expectRevert("RLPReader: bytes value contains an invalid remainder");
        RLPReader.readBytes(hex"800a");
    }

    function test_readBytes_invalidPrefix() external {
        vm.expectRevert("RLPReader: invalid prefix, single byte < 0x80 are not prefixed (short string)");
        RLPReader.readBytes(hex"810a");
    }

    function test_readList_empty() external {
        RLPReader.RLPItem[] memory list = RLPReader.readList(hex"c0");
        assertEq(list.length, 0);
    }

    function test_readList_multiList() external {
        RLPReader.RLPItem[] memory list = RLPReader.readList(hex"c6827a77c10401");
        assertEq(list.length, 3);

        assertEq(RLPReader.readRawBytes(list[0]), hex"827a77");
        assertEq(RLPReader.readRawBytes(list[1]), hex"c104");
        assertEq(RLPReader.readRawBytes(list[2]), hex"01");
    }

    function test_readList_shortListMax1() external {
        RLPReader.RLPItem[] memory list = RLPReader.readList(hex"f784617364668471776572847a78637684617364668471776572847a78637684617364668471776572847a78637684617364668471776572");

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

    function test_readList_longList1() external {
        RLPReader.RLPItem[] memory list = RLPReader.readList(hex"f840cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376");

        assertEq(list.length, 4);
        assertEq(RLPReader.readRawBytes(list[0]), hex"cf84617364668471776572847a786376");
        assertEq(RLPReader.readRawBytes(list[1]), hex"cf84617364668471776572847a786376");
        assertEq(RLPReader.readRawBytes(list[2]), hex"cf84617364668471776572847a786376");
        assertEq(RLPReader.readRawBytes(list[3]), hex"cf84617364668471776572847a786376");
    }

    function test_readList_longList2() external {
        RLPReader.RLPItem[] memory list = RLPReader.readList(hex"f90200cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376cf84617364668471776572847a786376");
        assertEq(list.length, 32);

        for (uint256 i = 0; i < 32; i++) {
            assertEq(RLPReader.readRawBytes(list[i]), hex"cf84617364668471776572847a786376");
        }
    }

    function test_readList_listOf33ElementsSucceeds() external {
        RLPReader.RLPItem[] memory list = RLPReader.readList(hex"e1454545454545454545454545454545454545454545454545454545454545454545");
        assertEq(list.length, 33);

        for (uint256 i = 0; i < 33; i++) {
            assertEq(RLPReader.readRawBytes(list[i]), hex"45");
        }
    }

    function test_readList_listOf66ElementsSucceeds() external {
        RLPReader.RLPItem[] memory list = RLPReader.readList(hex"f842454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545");
        assertEq(list.length, 66);

        for (uint256 i = 0; i < 66; i++) {
            assertEq(RLPReader.readRawBytes(list[i]), hex"45");
        }
    }

    function test_readList_listOf132ElementsSucceeds() external {
        RLPReader.RLPItem[] memory list = RLPReader.readList(hex"f884454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545");
        assertEq(list.length, 132);

        for (uint256 i = 0; i < 132; i++) {
            assertEq(RLPReader.readRawBytes(list[i]), hex"45");
        }
    }

    function test_readList_gas_listOf33Elements() external {
        RLPReader.readList(hex"f842454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545");
    }

    function test_readList_gas_listOf66Elements() external {
        RLPReader.readList(hex"f842454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545");
    }

    function test_readList_gas_listOf132Elements() external {
        RLPReader.readList(hex"f884454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545454545");
    }

    function test_readList_listOfLists() external {
        RLPReader.RLPItem[] memory list = RLPReader.readList(hex"c4c2c0c0c0");
        assertEq(list.length, 2);
        assertEq(RLPReader.readRawBytes(list[0]), hex"c2c0c0");
        assertEq(RLPReader.readRawBytes(list[1]), hex"c0");
    }

    function test_readList_listOfLists2() external {
        RLPReader.RLPItem[] memory list = RLPReader.readList(hex"c7c0c1c0c3c0c1c0");
        assertEq(list.length, 3);

        assertEq(RLPReader.readRawBytes(list[0]), hex"c0");
        assertEq(RLPReader.readRawBytes(list[1]), hex"c1c0");
        assertEq(RLPReader.readRawBytes(list[2]), hex"c3c0c1c0");
    }

    function test_readList_dictTest1() external {
        RLPReader.RLPItem[] memory list = RLPReader.readList(hex"ecca846b6579318476616c31ca846b6579328476616c32ca846b6579338476616c33ca846b6579348476616c34");
        assertEq(list.length, 4);

        assertEq(RLPReader.readRawBytes(list[0]), hex"ca846b6579318476616c31");
        assertEq(RLPReader.readRawBytes(list[1]), hex"ca846b6579328476616c32");
        assertEq(RLPReader.readRawBytes(list[2]), hex"ca846b6579338476616c33");
        assertEq(RLPReader.readRawBytes(list[3]), hex"ca846b6579348476616c34");
    }

    function test_readList_invalidShortList() external {
        vm.expectRevert("RLPReader: length of content must be greater than list length (short list)");
        RLPReader.readList(hex"efdebd");
    }

    function test_readList_longStringLength() external {
        vm.expectRevert("RLPReader: length of content must be greater than list length (short list)");
        RLPReader.readList(hex"efb83600");
    }

    function test_readList_notLongEnough() external {
        vm.expectRevert("RLPReader: length of content must be greater than list length (short list)");
        RLPReader.readList(hex"efdebdaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa");
    }

    function test_readList_invalidRemainder() external {
        vm.expectRevert("RLPReader: list item has an invalid data remainder");
        RLPReader.readList(hex"c000");
    }

    function test_readList_notEnoughContentForString1() external {
        vm.expectRevert("RLPReader: length of content must be greater than total length (long string)");
        RLPReader.readList(hex"ba010000aabbccddeeff");
    }

    function test_readList_notEnoughContentForString2() external {
        vm.expectRevert("RLPReader: length of content must be greater than total length (long string)");
        RLPReader.readList(hex"b840ffeeddccbbaa99887766554433221100");
    }

    function test_readList_notEnoughContentForList1() external {
        vm.expectRevert("RLPReader: length of content must be greater than total length (long list)");
        RLPReader.readList(hex"f90180");
    }

    function test_readList_notEnoughContentForList2() external {
        vm.expectRevert("RLPReader: length of content must be greater than total length (long list)");
        RLPReader.readList(hex"ffffffffffffffffff0001020304050607");
    }

    function test_readList_longStringLessThan56Bytes() external {
        vm.expectRevert("RLPReader: length of content must be greater than 55 bytes (long string)");
        RLPReader.readList(hex"b80100");
    }

    function test_readList_longListLessThan56Bytes() external {
        vm.expectRevert("RLPReader: length of content must be greater than 55 bytes (long list)");
        RLPReader.readList(hex"f80100");
    }

    function test_readList_int32Overflow() external {
        vm.expectRevert("RLPReader: length of content must be greater than total length (long string)");
        RLPReader.readList(hex"bf0f000000000000021111");
    }

    function test_readList_int32Overflow2() external {
        vm.expectRevert("RLPReader: length of content must be greater than total length (long list)");
        RLPReader.readList(hex"ff0f000000000000021111");
    }

    function test_readList_incorrectLengthInArray() external {
        vm.expectRevert("RLPReader: length of content must not have any leading zeros (long string)");
        RLPReader.readList(hex"b9002100dc2b275d0f74e8a53e6f4ec61b27f24278820be3f82ea2110e582081b0565df0");
    }

    function test_readList_leadingZerosInLongLengthArray1() external {
        vm.expectRevert("RLPReader: length of content must not have any leading zeros (long string)");
        RLPReader.readList(hex"b90040000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f");
    }

    function test_readList_leadingZerosInLongLengthArray2() external {
        vm.expectRevert("RLPReader: length of content must not have any leading zeros (long string)");
        RLPReader.readList(hex"b800");
    }

    function test_readList_leadingZerosInLongLengthList1() external {
        vm.expectRevert("RLPReader: length of content must not have any leading zeros (long list)");
        RLPReader.readList(hex"fb00000040000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f");
    }

    function test_readList_nonOptimalLongLengthArray1() external {
        vm.expectRevert("RLPReader: length of content must be greater than 55 bytes (long string)");
        RLPReader.readList(hex"b81000112233445566778899aabbccddeeff");
    }

    function test_readList_nonOptimalLongLengthArray2() external {
        vm.expectRevert("RLPReader: length of content must be greater than 55 bytes (long string)");
        RLPReader.readList(hex"b801ff");
    }

    function test_readList_invalidValue() external {
        vm.expectRevert("RLPReader: length of content must be greater than string length (short string)");
        RLPReader.readList(hex"91");
    }
}
