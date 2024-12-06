// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

using LibPosition for Position global;

/// @notice A `Position` represents a position of a claim within the game tree.
/// @dev This is represented as a "generalized index" where the high-order bit
/// is the level in the tree and the remaining bits is a unique bit pattern, allowing
/// a unique identifier for each node in the tree. Mathematically, it is calculated
/// as 2^{depth} + indexAtDepth.
type Position is uint128;

/// @title LibPosition
/// @notice This library contains helper functions for working with the `Position` type.
library LibPosition {
    /// @notice the `MAX_POSITION_BITLEN` is the number of bits that the `Position` type, and the implementation of
    ///         its behavior within this library, can safely support.
    uint8 internal constant MAX_POSITION_BITLEN = 126;

    /// @notice Computes a generalized index (2^{depth} + indexAtDepth).
    /// @param _depth The depth of the position.
    /// @param _indexAtDepth The index at the depth of the position.
    /// @return position_ The computed generalized index.
    function wrap(uint8 _depth, uint128 _indexAtDepth) internal pure returns (Position position_) {
        assembly {
            // gindex = 2^{_depth} + _indexAtDepth
            position_ := add(shl(_depth, 1), _indexAtDepth)
        }
    }

    /// @notice Pulls the `depth` out of a `Position` type.
    /// @param _position The generalized index to get the `depth` of.
    /// @return depth_ The `depth` of the `position` gindex.
    /// @custom:attribution Solady <https://github.com/Vectorized/Solady>
    function depth(Position _position) internal pure returns (uint8 depth_) {
        // Return the most significant bit offset, which signifies the depth of the gindex.
        assembly {
            depth_ := or(depth_, shl(6, lt(0xffffffffffffffff, shr(depth_, _position))))
            depth_ := or(depth_, shl(5, lt(0xffffffff, shr(depth_, _position))))

            // For the remaining 32 bits, use a De Bruijn lookup.
            _position := shr(depth_, _position)
            _position := or(_position, shr(1, _position))
            _position := or(_position, shr(2, _position))
            _position := or(_position, shr(4, _position))
            _position := or(_position, shr(8, _position))
            _position := or(_position, shr(16, _position))

            depth_ :=
                or(
                    depth_,
                    byte(
                        shr(251, mul(_position, shl(224, 0x07c4acdd))),
                        0x0009010a0d15021d0b0e10121619031e080c141c0f111807131b17061a05041f
                    )
                )
        }
    }

    /// @notice Pulls the `indexAtDepth` out of a `Position` type.
    ///         The `indexAtDepth` is the left/right index of a position at a specific depth within
    ///         the binary tree, starting from index 0. For example, at gindex 2, the `depth` = 1
    ///         and the `indexAtDepth` = 0.
    /// @param _position The generalized index to get the `indexAtDepth` of.
    /// @return indexAtDepth_ The `indexAtDepth` of the `position` gindex.
    function indexAtDepth(Position _position) internal pure returns (uint128 indexAtDepth_) {
        // Return bits p_{msb-1}...p_{0}. This effectively pulls the 2^{depth} out of the gindex,
        // leaving only the `indexAtDepth`.
        uint256 msb = depth(_position);
        assembly {
            indexAtDepth_ := sub(_position, shl(msb, 1))
        }
    }

    /// @notice Get the left child of `_position`.
    /// @param _position The position to get the left position of.
    /// @return left_ The position to the left of `position`.
    function left(Position _position) internal pure returns (Position left_) {
        assembly {
            left_ := shl(1, _position)
        }
    }

    /// @notice Get the right child of `_position`
    /// @param _position The position to get the right position of.
    /// @return right_ The position to the right of `position`.
    function right(Position _position) internal pure returns (Position right_) {
        assembly {
            right_ := or(1, shl(1, _position))
        }
    }

    /// @notice Get the parent position of `_position`.
    /// @param _position The position to get the parent position of.
    /// @return parent_ The parent position of `position`.
    function parent(Position _position) internal pure returns (Position parent_) {
        assembly {
            parent_ := shr(1, _position)
        }
    }

    /// @notice Get the deepest, right most gindex relative to the `position`. This is equivalent to
    ///         calling `right` on a position until the maximum depth is reached.
    /// @param _position The position to get the relative deepest, right most gindex of.
    /// @param _maxDepth The maximum depth of the game.
    /// @return rightIndex_ The deepest, right most gindex relative to the `position`.
    function rightIndex(Position _position, uint256 _maxDepth) internal pure returns (Position rightIndex_) {
        uint256 msb = depth(_position);
        assembly {
            let remaining := sub(_maxDepth, msb)
            rightIndex_ := or(shl(remaining, _position), sub(shl(remaining, 1), 1))
        }
    }

    /// @notice Get the deepest, right most trace index relative to the `position`. This is
    ///         equivalent to calling `right` on a position until the maximum depth is reached and
    ///         then finding its index at depth.
    /// @param _position The position to get the relative trace index of.
    /// @param _maxDepth The maximum depth of the game.
    /// @return traceIndex_ The trace index relative to the `position`.
    function traceIndex(Position _position, uint256 _maxDepth) internal pure returns (uint256 traceIndex_) {
        uint256 msb = depth(_position);
        assembly {
            let remaining := sub(_maxDepth, msb)
            traceIndex_ := sub(or(shl(remaining, _position), sub(shl(remaining, 1), 1)), shl(_maxDepth, 1))
        }
    }

    /// @notice Gets the position of the highest ancestor of `_position` that commits to the same
    ///         trace index.
    /// @param _position The position to get the highest ancestor of.
    /// @return ancestor_ The highest ancestor of `position` that commits to the same trace index.
    function traceAncestor(Position _position) internal pure returns (Position ancestor_) {
        // Create a field with only the lowest unset bit of `_position` set.
        Position lsb;
        assembly {
            lsb := and(not(_position), add(_position, 1))
        }
        // Find the index of the lowest unset bit within the field.
        uint256 msb = depth(lsb);
        // The highest ancestor that commits to the same trace index is the original position
        // shifted right by the index of the lowest unset bit.
        assembly {
            let a := shr(msb, _position)
            // Bound the ancestor to the minimum gindex, 1.
            ancestor_ := or(a, iszero(a))
        }
    }

    /// @notice Gets the position of the highest ancestor of `_position` that commits to the same
    ///         trace index, while still being below `_upperBoundExclusive`.
    /// @param _position The position to get the highest ancestor of.
    /// @param _upperBoundExclusive The exclusive upper depth bound, used to inform where to stop in order
    ///                             to not escape a sub-tree.
    /// @return ancestor_ The highest ancestor of `position` that commits to the same trace index.
    function traceAncestorBounded(
        Position _position,
        uint256 _upperBoundExclusive
    )
        internal
        pure
        returns (Position ancestor_)
    {
        // This function only works for positions that are below the upper bound.
        if (_position.depth() <= _upperBoundExclusive) {
            assembly {
                // Revert with `ClaimAboveSplit()`
                mstore(0x00, 0xb34b5c22)
                revert(0x1C, 0x04)
            }
        }

        // Grab the global trace ancestor.
        ancestor_ = traceAncestor(_position);

        // If the ancestor is above or at the upper bound, shift it to be below the upper bound.
        // This should be a special case that only covers positions that commit to the final leaf
        // in a sub-tree.
        if (ancestor_.depth() <= _upperBoundExclusive) {
            ancestor_ = ancestor_.rightIndex(_upperBoundExclusive + 1);
        }
    }

    /// @notice Get the move position of `_position`, which is the left child of:
    ///         1. `_position` if `_isAttack` is true.
    ///         2. `_position | 1` if `_isAttack` is false.
    /// @param _position The position to get the relative attack/defense position of.
    /// @param _isAttack Whether or not the move is an attack move.
    /// @return move_ The move position relative to `position`.
    function move(Position _position, bool _isAttack) internal pure returns (Position move_) {
        assembly {
            move_ := shl(1, or(iszero(_isAttack), _position))
        }
    }

    /// @notice Get the value of a `Position` type in the form of the underlying uint128.
    /// @param _position The position to get the value of.
    /// @return raw_ The value of the `position` as a uint128 type.
    function raw(Position _position) internal pure returns (uint128 raw_) {
        assembly {
            raw_ := _position
        }
    }
}
