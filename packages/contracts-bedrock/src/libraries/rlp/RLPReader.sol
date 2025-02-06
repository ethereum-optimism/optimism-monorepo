// SPDX-License-Identifier: MIT
pragma solidity ^0.8.8;

// Libraries
import {
    EmptyItem,
    UnexpectedString,
    InvalidDataRemainder,
    ContentLengthMismatch,
    InvalidHeader,
    UnexpectedList
} from "src/libraries/rlp/RLPErrors.sol";

/// @custom:attribution https://github.com/hamdiallam/Solidity-RLP
/// @title RLPReader
/// @notice RLPReader is a library for parsing RLP-encoded byte arrays into Solidity types. Adapted
///         from Solidity-RLP (https://github.com/hamdiallam/Solidity-RLP) by Hamdi Allam with
///         various tweaks to improve readability.
library RLPReader {
    /// @notice Custom pointer type to avoid confusion between pointers and uint256s.
    type MemoryPointer is uint256;

    /// @notice RLP item types.
    /// @custom:value DATA_ITEM Represents an RLP data item (NOT a list).
    /// @custom:value LIST_ITEM Represents an RLP list item.
    enum RLPItemType {
        DATA_ITEM,
        LIST_ITEM
    }

    /// @notice Struct representing an RLP item.
    /// @custom:field length Length of the RLP item.
    /// @custom:field ptr    Pointer to the RLP item in memory.
    struct RLPItem {
        uint256 length;
        MemoryPointer ptr;
    }

    /// @notice Max list length that this library will accept.
    uint256 internal constant MAX_LIST_LENGTH = 32;

    /// @notice Converts bytes to a reference to memory position and length.
    /// @param _in Input bytes to convert.
    /// @return out_ Output memory reference.
    function toRLPItem(bytes memory _in) internal pure returns (RLPItem memory out_) {
    }

    /// @notice Reads an RLP list value into a list of RLP items.
    /// @param _in RLP list value.
    /// @return out_ Decoded RLP list items.
    function readList(RLPItem memory _in) internal pure returns (RLPItem[] memory out_) {
    }

    /// @notice Reads an RLP list value into a list of RLP items.
    /// @param _in RLP list value.
    /// @return out_ Decoded RLP list items.
    function readList(bytes memory _in) internal pure returns (RLPItem[] memory out_) {
    }

    /// @notice Reads an RLP bytes value into bytes.
    /// @param _in RLP bytes value.
    /// @return out_ Decoded bytes.
    function readBytes(RLPItem memory _in) internal pure returns (bytes memory out_) {
    }

    /// @notice Reads an RLP bytes value into bytes.
    /// @param _in RLP bytes value.
    /// @return out_ Decoded bytes.
    function readBytes(bytes memory _in) internal pure returns (bytes memory out_) {
    }

    /// @notice Reads the raw bytes of an RLP item.
    /// @param _in RLP item to read.
    /// @return out_ Raw RLP bytes.
    function readRawBytes(RLPItem memory _in) internal pure returns (bytes memory out_) {
    }
}
