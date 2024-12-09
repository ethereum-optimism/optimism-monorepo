// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing utilities
import { Test } from "forge-std/Test.sol";

// Target contract
import { Bytes } from "src/libraries/Bytes.sol";

contract Bytes_slice_Test is Test {
    /// @notice Tests that the `slice` function works as expected when starting from index 0.
    function test_slice_fromZeroIdx_works() public pure {
        bytes memory input = hex"11223344556677889900";

        // Exhaustively check if all possible slices starting from index 0 are correct.
        assertEq(Bytes.slice(input, 0, 0), hex"");
        assertEq(Bytes.slice(input, 0, 1), hex"11");
        assertEq(Bytes.slice(input, 0, 2), hex"1122");
        assertEq(Bytes.slice(input, 0, 3), hex"112233");
        assertEq(Bytes.slice(input, 0, 4), hex"11223344");
        assertEq(Bytes.slice(input, 0, 5), hex"1122334455");
        assertEq(Bytes.slice(input, 0, 6), hex"112233445566");
        assertEq(Bytes.slice(input, 0, 7), hex"11223344556677");
        assertEq(Bytes.slice(input, 0, 8), hex"1122334455667788");
        assertEq(Bytes.slice(input, 0, 9), hex"112233445566778899");
        assertEq(Bytes.slice(input, 0, 10), hex"11223344556677889900");
    }

    /// @notice Tests that the `slice` function works as expected when starting from indices [1, 9]
    ///         with lengths [1, 9], in reverse order.
    function test_slice_fromNonZeroIdx_works() public pure {
        bytes memory input = hex"11223344556677889900";

        // Exhaustively check correctness of slices starting from indexes [1, 9]
        // and spanning [1, 9] bytes, in reverse order
        assertEq(Bytes.slice(input, 9, 1), hex"00");
        assertEq(Bytes.slice(input, 8, 2), hex"9900");
        assertEq(Bytes.slice(input, 7, 3), hex"889900");
        assertEq(Bytes.slice(input, 6, 4), hex"77889900");
        assertEq(Bytes.slice(input, 5, 5), hex"6677889900");
        assertEq(Bytes.slice(input, 4, 6), hex"556677889900");
        assertEq(Bytes.slice(input, 3, 7), hex"44556677889900");
        assertEq(Bytes.slice(input, 2, 8), hex"3344556677889900");
        assertEq(Bytes.slice(input, 1, 9), hex"223344556677889900");
    }

    /// @notice Tests that the `slice` function works as expected when slicing between multiple words
    ///         in memory. In this case, we test that a 2 byte slice between the 32nd byte of the
    ///         first word and the 1st byte of the second word is correct.
    function test_slice_acrossWords_works() public pure {
        bytes memory input =
            hex"00000000000000000000000000000000000000000000000000000000000000112200000000000000000000000000000000000000000000000000000000000000";

        assertEq(Bytes.slice(input, 31, 2), hex"1122");
    }

    /// @notice Tests that the `slice` function works as expected when slicing between multiple
    ///         words in memory. In this case, we test that a 34 byte slice between 3 separate words
    ///        returns the correct result.
    function test_slice_acrossMultipleWords_works() public pure {
        bytes memory input =
            hex"000000000000000000000000000000000000000000000000000000000000001122FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF1100000000000000000000000000000000000000000000000000000000000000";
        bytes memory expected = hex"1122FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF11";

        assertEq(Bytes.slice(input, 31, 34), expected);
    }

    /// @notice Tests that the `slice` function correctly updates the free memory pointer depending
    ///         on the length of the slice.
    ///         The calls to `bound` are to reduce the number of times that `assume` is triggered.
    function testFuzz_slice_memorySafety_succeeds(bytes memory _input, uint256 _start, uint256 _length) public {
        vm.assume(_input.length > 0);

        // The start should never be more than the length of the input bytes array - 1
        _start = bound(_start, 0, _input.length - 1);

        // The length should never be more than the length of the input bytes array - the starting
        // slice index.
        _length = bound(_length, 0, _input.length - _start);

        // Grab the free memory pointer before the slice operation
        uint64 initPtr;
        assembly {
            initPtr := mload(0x40)
        }
        uint64 expectedPtr = uint64((initPtr + 0x20 + _length + 0x1f) & ~uint256(0x1f));

        // Ensure that all memory outside of the expected range is safe.
        vm.expectSafeMemory(initPtr, expectedPtr);

        // Slice the input bytes array from `_start` to `_start + _length`
        bytes memory slice = Bytes.slice(_input, _start, _length);

        vm.stopExpectSafeMemory();

        // Grab the free memory pointer after the slice operation
        uint64 finalPtr;
        assembly {
            finalPtr := mload(0x40)
        }

        // The free memory pointer should have been updated properly
        if (_length == 0) {
            // If the slice length is zero, only 32 bytes of memory should have been allocated.
            assertEq(finalPtr, initPtr + 0x20);
        } else {
            // If the slice length is greater than zero, the memory allocated should be the
            // length of the slice rounded up to the next 32 byte word + 32 bytes for the
            // length of the byte array.
            //
            // Note that we use a slightly less efficient, but equivalent method of rounding
            // up `_length` to the next multiple of 32 than is used in the `slice` function.
            // This is to diff test the method used in `slice`.
            uint64 _expectedPtr = uint64(((initPtr + 0x20 + _length + 0x1F) >> 5) << 5);
            assertEq(finalPtr, _expectedPtr);

            // Sanity check for equivalence of the rounding methods.
            assertEq(_expectedPtr, expectedPtr);
        }

        // The slice length should be equal to `_length`
        assertEq(slice.length, _length);
    }
}

contract Bytes_slice_TestFail is Test {
    /// @notice Tests that, when given an input bytes array of length `n`, the `slice` function will
    ///         always revert if `_start + _length > n`.
    function testFuzz_slice_outOfBounds_reverts(bytes memory _input, uint256 _start, uint256 _length) public {
        // We want a valid start index that will not overflow.
        if (_input.length == 0) {
            _start = 0;
        } else {
            _start = bound(_start, 0, _input.length - 1);
        }
        // And a length that will not overflow.
        // But, we want an invalid slice length.
        if (_start > 31) {
            _length = bound(_length, (_input.length - _start) + 1, type(uint256).max - _start);
        } else {
            _length = bound(_length, (_input.length - _start) + 1, type(uint256).max - 31);
        }

        vm.expectRevert("slice_outOfBounds");
        Bytes.slice(_input, _start, _length);
    }

    /// @notice Tests that, when given a length `n` that is greater than `type(uint256).max - 31`,
    ///         the `slice` function reverts.
    function testFuzz_slice_lengthOverflows_reverts(bytes memory _input, uint256 _start, uint256 _length) public {
        // Ensure that the `_length` will overflow if a number >= 31 is added to it.
        _length = uint256(bound(_length, type(uint256).max - 30, type(uint256).max));

        vm.expectRevert("slice_overflow");
        Bytes.slice(_input, _start, _length);
    }

    /// @notice Tests that, when given a start index `n` that is greater than
    ///         `type(uint256).max - n`, the `slice` function reverts.
    ///         The calls to `bound` are to reduce the number of times that `assume` is triggered.
    function testFuzz_slice_rangeOverflows_reverts(bytes memory _input, uint256 _start, uint256 _length) public {
        // Ensure that `_length` is a realistic length of a slice. This is to make sure
        // we revert on the correct require statement.
        _length = bound(_length, 0, _input.length == 0 ? 0 : _input.length - 1);
        vm.assume(_length < _input.length);

        // Ensure that `_start` will overflow if `_length` is added to it.
        _start = bound(_start, type(uint256).max - _length, type(uint256).max);
        vm.assume(_start > type(uint256).max - _length);

        vm.expectRevert("slice_overflow");
        Bytes.slice(_input, _start, _length);
    }
}

contract Bytes_toNibbles_Test is Test {
    /// @notice Tests that, given an input of 5 bytes, the `toNibbles` function returns an array of
    ///         10 nibbles corresponding to the input data.
    function test_toNibbles_expectedResult5Bytes_works() public pure {
        bytes memory input = hex"1234567890";
        bytes memory expected = hex"01020304050607080900";
        bytes memory actual = Bytes.toNibbles(input);

        assertEq(input.length * 2, actual.length);
        assertEq(expected.length, actual.length);
        assertEq(actual, expected);
    }

    /// @notice Tests that, given an input of 128 bytes, the `toNibbles` function returns an array
    ///         of 256 nibbles corresponding to the input data. This test exists to ensure that,
    ///         given a large input, the `toNibbles` function works as expected.
    function test_toNibbles_expectedResult128Bytes_works() public pure {
        bytes memory input =
            hex"000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f404142434445464748494a4b4c4d4e4f505152535455565758595a5b5c5d5e5f606162636465666768696a6b6c6d6e6f707172737475767778797a7b7c7d7e7f";
        bytes memory expected =
            hex"0000000100020003000400050006000700080009000a000b000c000d000e000f0100010101020103010401050106010701080109010a010b010c010d010e010f0200020102020203020402050206020702080209020a020b020c020d020e020f0300030103020303030403050306030703080309030a030b030c030d030e030f0400040104020403040404050406040704080409040a040b040c040d040e040f0500050105020503050405050506050705080509050a050b050c050d050e050f0600060106020603060406050606060706080609060a060b060c060d060e060f0700070107020703070407050706070707080709070a070b070c070d070e070f";
        bytes memory actual = Bytes.toNibbles(input);

        assertEq(input.length * 2, actual.length);
        assertEq(expected.length, actual.length);
        assertEq(actual, expected);
    }

    /// @notice Tests that, given an input of 0 bytes, the `toNibbles` function returns a zero
    ///         length array.
    function test_toNibbles_zeroLengthInput_works() public pure {
        bytes memory input = hex"";
        bytes memory expected = hex"";
        bytes memory actual = Bytes.toNibbles(input);

        assertEq(input.length, 0);
        assertEq(expected.length, 0);
        assertEq(actual.length, 0);
        assertEq(actual, expected);
    }

    /// @notice Tests that the `toNibbles` function correctly updates the free memory pointer depending
    ///         on the length of the resulting array.
    function testFuzz_toNibbles_memorySafety_succeeds(bytes memory _input) public {
        // Grab the free memory pointer before the `toNibbles` operation
        uint64 initPtr;
        assembly {
            initPtr := mload(0x40)
        }
        uint64 expectedPtr = uint64(initPtr + 0x20 + ((_input.length * 2 + 0x1F) & ~uint256(0x1F)));

        // Ensure that all memory outside of the expected range is safe.
        vm.expectSafeMemory(initPtr, expectedPtr);

        // Pull out each individual nibble from the input bytes array
        bytes memory nibbles = Bytes.toNibbles(_input);

        vm.stopExpectSafeMemory();

        // Grab the free memory pointer after the `toNibbles` operation
        uint64 finalPtr;
        assembly {
            finalPtr := mload(0x40)
        }

        // The free memory pointer should have been updated properly
        if (_input.length == 0) {
            // If the input length is zero, only 32 bytes of memory should have been allocated.
            assertEq(finalPtr, initPtr + 0x20);
        } else {
            // If the input length is greater than zero, the memory allocated should be the
            // length of the input * 2 + 32 bytes for the length field.
            //
            // Note that we use a slightly less efficient, but equivalent method of rounding
            // up `_length` to the next multiple of 32 than is used in the `toNibbles` function.
            // This is to diff test the method used in `toNibbles`.
            uint64 _expectedPtr = uint64(initPtr + 0x20 + (((_input.length * 2 + 0x1F) >> 5) << 5));
            assertEq(finalPtr, _expectedPtr);

            // Sanity check for equivalence of the rounding methods.
            assertEq(_expectedPtr, expectedPtr);
        }

        // The nibbles length should be equal to `_length * 2`
        assertEq(nibbles.length, _input.length << 1);
    }
}

contract Bytes_equal_Test is Test {
    /// @notice Manually checks equality of two dynamic `bytes` arrays in memory.
    /// @param _a The first `bytes` array to compare.
    /// @param _b The second `bytes` array to compare.
    /// @return True if the two `bytes` arrays are equal in memory.
    function manualEq(bytes memory _a, bytes memory _b) internal pure returns (bool) {
        bool _eq;
        assembly {
            _eq :=
                and(
                    // Check if the contents of the two bytes arrays are equal in memory.
                    eq(keccak256(add(0x20, _a), mload(_a)), keccak256(add(0x20, _b), mload(_b))),
                    // Check if the length of the two bytes arrays are equal in memory.
                    // This is redundant given the above check, but included for completeness.
                    eq(mload(_a), mload(_b))
                )
        }
        return _eq;
    }

    /// @notice Tests that the `equal` function in the `Bytes` library returns `false` if given two
    ///         non-equal byte arrays.
    function testFuzz_equal_notEqual_works(bytes memory _a, bytes memory _b) public pure {
        vm.assume(!manualEq(_a, _b));
        assertFalse(Bytes.equal(_a, _b));
    }

    /// @notice Test whether or not the `equal` function in the `Bytes` library is equivalent to
    ///         manually checking equality of the two dynamic `bytes` arrays in memory.
    function testDiff_equal_works(bytes memory _a, bytes memory _b) public pure {
        assertEq(Bytes.equal(_a, _b), manualEq(_a, _b));
    }
}
