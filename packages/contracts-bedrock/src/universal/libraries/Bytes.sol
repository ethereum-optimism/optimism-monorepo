// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// @title Bytes
/// @notice Bytes is a library for manipulating byte arrays.
library Bytes {
    /// @custom:attribution https://github.com/GNSPS/solidity-bytes-utils
    /// @notice Slices a byte array with a given starting index and length. Returns a new byte array
    ///         as opposed to a pointer to the original array. Will throw if trying to slice more
    ///         bytes than exist in the array.
    /// @param _bytes Byte array to slice.
    /// @param _start Starting index of the slice.
    /// @param _length Length of the slice.
    /// @return Slice of the input byte array.
    function slice(bytes memory _bytes, uint256 _start, uint256 _length) internal pure returns (bytes memory) {
        unchecked {
            require(_length + 31 >= _length, "slice_overflow");
            require(_start + _length >= _start, "slice_overflow");
            require(_bytes.length >= _start + _length, "slice_outOfBounds");
        }

        bytes memory tempBytes;

        assembly {
            switch iszero(_length)
            case 0 {
                // Get a location of some free memory and store it in tempBytes as
                // Solidity does for memory variables.
                tempBytes := mload(0x40)

                // The first word of the slice result is potentially a partial
                // word read from the original array. To read it, we calculate
                // the length of that partial word and start copying that many
                // bytes into the array. The first word we copy will start with
                // data we don't care about, but the last `lengthmod` bytes will
                // land at the beginning of the contents of the new array. When
                // we're done copying, we overwrite the full first word with
                // the actual length of the slice.
                let lengthmod := and(_length, 31)

                // The multiplication in the next line is necessary
                // because when slicing multiples of 32 bytes (lengthmod == 0)
                // the following copy loop was copying the origin's length
                // and then ending prematurely not copying everything it should.
                let mc := add(add(tempBytes, lengthmod), mul(0x20, iszero(lengthmod)))
                let end := add(mc, _length)

                for {
                    // The multiplication in the next line has the same exact purpose
                    // as the one above.
                    let cc := add(add(add(_bytes, lengthmod), mul(0x20, iszero(lengthmod))), _start)
                } lt(mc, end) {
                    mc := add(mc, 0x20)
                    cc := add(cc, 0x20)
                } { mstore(mc, mload(cc)) }

                mstore(tempBytes, _length)

                //update free-memory pointer
                //allocating the array padded to 32 bytes like the compiler does now
                mstore(0x40, and(add(mc, 31), not(31)))
            }
            //if we want a zero-length slice let's just return a zero-length array
            default {
                tempBytes := mload(0x40)

                //zero out the 32 bytes slice we are about to return
                //we need to do it because Solidity does not garbage collect
                mstore(tempBytes, 0)

                mstore(0x40, add(tempBytes, 0x20))
            }
        }

        return tempBytes;
    }

    /// @notice Slices a byte array with a given starting index up to the end of the original byte
    ///         array. Returns a new array rathern than a pointer to the original.
    /// @param _bytes Byte array to slice.
    /// @param _start Starting index of the slice.
    /// @return Slice of the input byte array.
    function slice(bytes memory _bytes, uint256 _start) internal pure returns (bytes memory) {
        if (_start >= _bytes.length) {
            return bytes("");
        }
        return slice(_bytes, _start, _bytes.length - _start);
    }

    /// @notice Converts a byte array into a nibble array by splitting each byte into two nibbles.
    ///         Resulting nibble array will be exactly twice as long as the input byte array.
    /// @param _bytes Input byte array to convert.
    /// @return Resulting nibble array.
    function toNibbles(bytes memory _bytes) internal pure returns (bytes memory) {
        bytes memory _nibbles;
        assembly {
            // Grab a free memory offset for the new array
            _nibbles := mload(0x40)

            // Load the length of the passed bytes array from memory
            let bytesLength := mload(_bytes)

            // Calculate the length of the new nibble array
            // This is the length of the input array times 2
            let nibblesLength := shl(0x01, bytesLength)

            // Update the free memory pointer to allocate memory for the new array.
            // To do this, we add the length of the new array + 32 bytes for the array length
            // rounded up to the nearest 32 byte boundary to the current free memory pointer.
            mstore(0x40, add(_nibbles, and(not(0x1F), add(nibblesLength, 0x3F))))

            // Store the length of the new array in memory
            mstore(_nibbles, nibblesLength)

            // Store the memory offset of the _bytes array's contents on the stack
            let bytesStart := add(_bytes, 0x20)

            // Store the memory offset of the nibbles array's contents on the stack
            let nibblesStart := add(_nibbles, 0x20)

            // Loop through each byte in the input array
            for { let i := 0x00 } lt(i, bytesLength) { i := add(i, 0x01) } {
                // Get the starting offset of the next 2 bytes in the nibbles array
                let offset := add(nibblesStart, shl(0x01, i))
                // Load the byte at the current index within the `_bytes` array
                let b := byte(0x00, mload(add(bytesStart, i)))

                // Pull out the first nibble and store it in the new array
                mstore8(offset, shr(0x04, b))
                // Pull out the second nibble and store it in the new array
                mstore8(add(offset, 0x01), and(b, 0x0F))
            }
        }
        return _nibbles;
    }

    /// @notice Compares two byte arrays by comparing their keccak256 hashes.
    /// @param _bytes First byte array to compare.
    /// @param _other Second byte array to compare.
    /// @return True if the two byte arrays are equal, false otherwise.
    function equal(bytes memory _bytes, bytes memory _other) internal pure returns (bool) {
        return keccak256(_bytes) == keccak256(_other);
    }
}
