// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

bytes1 constant SUPER_ROOT_VERSION = 0x01;

/// @notice Error thrown when the version of the encoded SuperRoot is invalid.
error InvalidVersion();

/// @notice Error thrown when the length of the encoded SuperRoot is unexpected.
error UnexpectedLength();

/// @notice Error thrown when the output roots are not sorted in ascending order.
error OutputRootsNotSorted();

/// @notice The SuperRoot type represents the state of the Superchain at a given timestamp.
struct SuperRoot {
    /// @notice The timestamp of the SuperRoot.
    uint64 timestamp;
    /// @notice The output roots that the SuperRoot commits to. MUST be sorted in ascending order by L2 chain ID.
    OutputRootWithChainId[] outputRoots;
}

/// @notice Represents an output root commitment and the L2 chain ID that it commits to.
struct OutputRootWithChainId {
    /// @notice The chain ID of the L2 chain that the output root commits to.
    uint256 l2ChainId;
    /// @notice The output root.
    bytes32 outputRoot;
}

/// @notice The SuperRootProof type defines the metadata required to prove the inclusion of an output root
///         within a SuperRoot.
struct SuperRootProof {
    /// @notice The pre-image of the super root commitment.
    bytes rawSuperRoot;
    /// @notice The L2 chain ID that the output root commits to.
    uint256 l2ChainId;
    /// @notice The index of the `OutputRootWithChainId` tuple in the `outputRoots` array of the super root
    ///         that commits to the output root @ `l2ChainId`.
    uint256 index;
}

/// @title LibSuperRoot
/// @notice SuperRoot is a library that supports the `SuperRoot` type.
library LibSuperRoot {
    /// @notice Decodes an encoded SuperRoot into a SuperRoot struct.
    /// @param _raw The encoded SuperRoot.
    /// @return superRoot_ The decoded SuperRoot.
    /// @dev The encoded layout of the encoded super root is as follows:
    /// ┌───────────────┬─────────────────────────────┬───────────────────────────────┐
    /// │      0        │           [1, 9)            │         [9, 9 + n*64)         │
    /// ├───────────────┼─────────────────────────────┼───────────────────────────────┤
    /// │ Version Byte  │ Timestamp (big-endian, u64) │ Output Root & Chain ID tuples │
    /// └───────────────┴─────────────────────────────┴───────────────────────────────┘
    function decode(bytes calldata _raw) internal pure returns (SuperRoot memory superRoot_) {
        // Ensure the length is at least 1 + 8 + 64 bytes. (version + timestamp + a single
        // output root with chain ID).
        if (_raw.length < 1 + 8 + 64) revert UnexpectedLength();

        // Ensure the version is correct.
        if (_raw[0] != SUPER_ROOT_VERSION) revert InvalidVersion();

        // Extract the timestamp as a big-endian uint64.
        uint64 rootTimestamp;
        assembly {
            rootTimestamp := shr(0xC0, calldataload(add(_raw.offset, 0x01)))
        }

        // Decode the output roots iteratively, starting at the 9th byte.
        OutputRootWithChainId[] memory outputRoots;
        assembly {
            // Place the `outputRoots` array at the free memory pointer.
            outputRoots := mload(0x40)

            // Compute the offset of the output root tuples within the encoded super root.
            let outputRootsCalldataOffset := add(_raw.offset, 0x09)

            // Calculate the remaining length of the encoded super root.
            let remaining := sub(add(_raw.offset, _raw.length), outputRootsCalldataOffset)

            // Check if the calldata is well-formed. At this point, the remaining length must be
            // a multiple of 64.
            if mod(remaining, 0x40) {
                // Signature of `UnexpectedLength()` error
                mstore(0x00, 0x10345c7c)
                revert(0x1c, 0x04)
            }

            // Store the data offsets on the stack. Arrays of dynamically sized objects in solidity
            // are comprised of pointers, so we must allocate two blocks of memory - one for the array
            // of pointers, and one for the data itself.
            let ptrArrayMemoryOffset := add(outputRoots, 0x20)
            let outputRootsMemoryOffset := add(ptrArrayMemoryOffset, shr(0x01, remaining))

            // Iteratively decode the output roots, checking that they are sorted in ascending order.
            let lastChainId := 0x00
            for { let i := 0x00 } lt(i, remaining) { i := add(i, 0x40) } {
                let chainIdLocalOffset := i
                let outputRootLocalOffset := add(i, 0x20)

                // Extract the chain ID and output root commitment from the calldata.
                let chainId := calldataload(add(outputRootsCalldataOffset, chainIdLocalOffset))
                let outputRoot := calldataload(add(outputRootsCalldataOffset, outputRootLocalOffset))

                // Check that the chain IDs are sorted in ascending order.
                if lt(chainId, lastChainId) {
                    // Signature of `OutputRootsNotSorted()` error
                    mstore(0x00, 0x0277c843)
                    revert(0x1c, 0x04)
                }
                lastChainId := chainId

                // Store the tuple data.
                let dataPtr := add(outputRootsMemoryOffset, i)
                mstore(dataPtr, chainId)
                mstore(add(dataPtr, 0x20), outputRoot)

                // Store the pointer to the tuple data in the array of pointers.
                mstore(add(ptrArrayMemoryOffset, shr(0x01, i)), dataPtr)
            }

            // Store the length of the output root tuple array in memory.
            mstore(outputRoots, shr(0x06, remaining))

            // Compute the new free memory pointer, adding the length of the output roots array
            // (32 + num tuples * 32) and the length of the tuple data (num tuples * 64).
            let endPtr := add(outputRoots, add(add(0x20, shr(0x01, remaining)), remaining))
            let fmp := and(not(0x1F), add(endPtr, 0x1F))

            // Finally, update the free memory pointer to the nearest aligned word to account for
            // our allocations.
            mstore(0x40, fmp)
        }

        superRoot_ = SuperRoot({ timestamp: rootTimestamp, outputRoots: outputRoots });
    }
}
