// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { Test } from "forge-std/Test.sol";
import { LibPosition } from "src/dispute/lib/LibPosition.sol";
import "src/dispute/lib/Types.sol";

/// @notice Tests for `LibPosition`
contract LibPosition_Test is Test {
    /// @dev Assumes a MAX depth of 126 for the Position type. Any greater depth can cause overflows.
    /// @dev At the lowest level of the tree, this allows for 2 ** 126 leaves. In reality, the max game depth
    ///      will likely be much lower.
    uint8 internal constant MAX_DEPTH = 126;
    /// @dev Arbitrary split depth around half way down the tree.
    uint8 internal constant SPLIT_DEPTH = 64;

    function boundIndexAtDepth(uint8 _depth, uint128 _indexAtDepth) internal pure returns (uint128) {
        // Index at depth bound: [0, 2 ** _depth-1]
        if (_depth > 0) {
            return uint128(bound(_indexAtDepth, 0, 2 ** (_depth - 1)));
        } else {
            return 0;
        }
    }

    /// @notice Tests that the `depth` function correctly shifts out the `depth` from a packed `Position` type.
    function testFuzz_depth_correctness_succeeds(uint8 _depth, uint128 _indexAtDepth) public pure {
        _depth = uint8(bound(_depth, 0, MAX_DEPTH));
        _indexAtDepth = boundIndexAtDepth(_depth, _indexAtDepth);
        Position position = LibPosition.wrap(_depth, _indexAtDepth);
        assertEq(position.depth(), _depth);
    }

    /// @notice Tests that the `indexAtDepth` function correctly shifts out the `indexAtDepth` from a packed `Position`
    /// type.
    function testFuzz_indexAtDepth_correctness_succeeds(uint8 _depth, uint128 _indexAtDepth) public pure {
        _depth = uint8(bound(_depth, 0, MAX_DEPTH));
        _indexAtDepth = boundIndexAtDepth(_depth, _indexAtDepth);
        Position position = LibPosition.wrap(_depth, _indexAtDepth);
        assertEq(position.indexAtDepth(), _indexAtDepth);
    }

    /// @notice Tests that the `left` function correctly computes the position of the left child.
    function testFuzz_left_correctness_succeeds(uint8 _depth, uint128 _indexAtDepth) public pure {
        _depth = uint8(bound(_depth, 0, MAX_DEPTH));
        _indexAtDepth = boundIndexAtDepth(_depth, _indexAtDepth);

        Position position = LibPosition.wrap(_depth, _indexAtDepth);
        Position left = position.left();

        assertEq(left.depth(), _depth + 1);
        assertEq(left.indexAtDepth(), _indexAtDepth * 2);
    }

    /// @notice Tests that the `right` function correctly computes the position of the right child.
    function testFuzz_right_correctness_succeeds(uint8 _depth, uint128 _indexAtDepth) public pure {
        // Depth bound: [0, 63]
        _depth = uint8(bound(_depth, 0, MAX_DEPTH));
        _indexAtDepth = boundIndexAtDepth(_depth, _indexAtDepth);

        Position position = LibPosition.wrap(_depth, _indexAtDepth);
        Position right = position.right();

        assertEq(right.depth(), _depth + 1);
        assertEq(right.indexAtDepth(), _indexAtDepth * 2 + 1);
    }

    /// @notice Tests that the `parent` function correctly computes the position of the parent.
    function testFuzz_parent_correctness_succeeds(uint8 _depth, uint128 _indexAtDepth) public pure {
        _depth = uint8(bound(_depth, 1, MAX_DEPTH));
        _indexAtDepth = boundIndexAtDepth(_depth, _indexAtDepth);

        Position position = LibPosition.wrap(_depth, _indexAtDepth);
        Position parent = position.parent();

        assertEq(parent.depth(), _depth - 1);
        assertEq(parent.indexAtDepth(), _indexAtDepth / 2);
    }

    /// @notice Tests that the `traceAncestor` function correctly computes the position of the
    ///         highest ancestor that commits to the same trace index.
    function testFuzz_traceAncestor_correctness_succeeds(uint8 _depth, uint128 _indexAtDepth) public pure {
        _depth = uint8(bound(_depth, 1, MAX_DEPTH));
        _indexAtDepth = boundIndexAtDepth(_depth, _indexAtDepth);

        Position position = LibPosition.wrap(_depth, _indexAtDepth);
        Position ancestor = position.traceAncestor();
        Position loopAncestor = position;
        while (loopAncestor.parent().traceIndex(MAX_DEPTH) == position.traceIndex(MAX_DEPTH)) {
            loopAncestor = loopAncestor.parent();
        }

        assertEq(Position.unwrap(ancestor), Position.unwrap(loopAncestor));
    }

    /// @notice Tests that the `traceAncestorBounded` function correctly computes the position of the
    ///         highest ancestor (below `SPLIT_DEPTH`) that commits to the same trace index.
    function testFuzz_traceAncestorBounded_correctness_succeeds(uint8 _depth, uint128 _indexAtDepth) public pure {
        _depth = uint8(bound(_depth, SPLIT_DEPTH + 1, MAX_DEPTH));
        _indexAtDepth = boundIndexAtDepth(_depth, _indexAtDepth);

        Position position = LibPosition.wrap(_depth, _indexAtDepth);
        Position ancestor = position.traceAncestorBounded(SPLIT_DEPTH);
        Position loopAncestor = position;

        // Stop at 1 below the split depth.
        while (
            loopAncestor.parent().traceIndex(MAX_DEPTH) == position.traceIndex(MAX_DEPTH)
                && loopAncestor.depth() != SPLIT_DEPTH + 1
        ) {
            loopAncestor = loopAncestor.parent();
        }

        assertEq(Position.unwrap(ancestor), Position.unwrap(loopAncestor));
    }

    /// @notice Tests that the `rightIndex` function correctly computes the deepest, right most index relative
    ///         to a given position.
    function testFuzz_rightIndex_correctness_succeeds(
        uint8 _maxDepth,
        uint8 _depth,
        uint128 _indexAtDepth
    )
        public
        pure
    {
        // Max depth bound: [1, 63]
        // The max game depth MUST be at least 1.
        _maxDepth = uint8(bound(_maxDepth, 1, MAX_DEPTH));
        // Depth bound: [0, _maxDepth]
        _depth = uint8(bound(_depth, 0, _maxDepth));
        _indexAtDepth = boundIndexAtDepth(_depth, _indexAtDepth);

        Position position = LibPosition.wrap(_depth, _indexAtDepth);
        Position rightIndex = position.rightIndex(_maxDepth);

        // Find the deepest, rightmost index in Solidity rather than Yul
        for (uint256 i = _depth; i < _maxDepth; ++i) {
            position = position.right();
        }

        assertEq(Position.unwrap(rightIndex), Position.unwrap(position));
    }

    /// @notice Tests that the `attack` function correctly computes the position of the attack relative to
    ///         a given position.
    /// @dev `attack` is an alias for `left`, but we test it separately for completeness.
    function testFuzz_attack_correctness_succeeds(uint8 _depth, uint128 _indexAtDepth) public pure {
        // Depth bound: [0, 63]
        _depth = uint8(bound(_depth, 0, MAX_DEPTH));
        _indexAtDepth = boundIndexAtDepth(_depth, _indexAtDepth);

        Position position = LibPosition.wrap(_depth, _indexAtDepth);
        Position attack = position.move(true);

        assertEq(attack.depth(), _depth + 1);
        assertEq(attack.indexAtDepth(), _indexAtDepth * 2);
    }

    /// @notice Tests that the `defend` function correctly computes the position of the defense relative to
    ///         a given position.
    /// @dev A defense can only be given if the position does not belong to the root claim, hence the bound of [1, 127]
    ///      on the depth.
    function testFuzz_defend_correctness_succeeds(uint8 _depth, uint128 _indexAtDepth) public pure {
        // Depth bound: [1, 63]
        _depth = uint8(bound(_depth, 1, MAX_DEPTH));
        _indexAtDepth = boundIndexAtDepth(_depth, _indexAtDepth);

        Position position = LibPosition.wrap(_depth, _indexAtDepth);
        Position defend = position.move(false);

        assertEq(defend.depth(), _depth + 1);
        assertEq(defend.indexAtDepth(), ((_indexAtDepth / 2) * 2 + 1) * 2);
    }

    /// @notice A static unit test for the correctness of all gindicies, (depth, index) combos,
    ///         and the trace index in a tree of max depth = 4.
    function test_pos_correctness_succeeds() public pure {
        uint256 maxDepth = 4;

        Position p = LibPosition.wrap(0, 0);
        assertEq(Position.unwrap(p), 1); // gindex = 1
        assertEq(p.depth(), 0); // depth = 0
        assertEq(p.indexAtDepth(), 0); // indexAtDepth = 0
        Position r = p.rightIndex(maxDepth);
        assertEq(Position.unwrap(r), 31); // right gindex = 31
        assertEq(r.indexAtDepth(), 15); // trace index = 15

        p = LibPosition.wrap(1, 0);
        assertEq(Position.unwrap(p), 2); // gindex = 2
        assertEq(p.depth(), 1); // depth = 1
        assertEq(p.indexAtDepth(), 0); // indexAtDepth = 0
        r = p.rightIndex(maxDepth);
        assertEq(Position.unwrap(r), 23); // right gindex = 23
        assertEq(r.indexAtDepth(), 7); // trace index = 7

        p = LibPosition.wrap(1, 1);
        assertEq(Position.unwrap(p), 3); // gindex = 3
        assertEq(p.depth(), 1); // depth = 1
        assertEq(p.indexAtDepth(), 1); // indexAtDepth = 1
        r = p.rightIndex(maxDepth);
        assertEq(Position.unwrap(r), 31); // right gindex = 31
        assertEq(r.indexAtDepth(), 15); // trace index = 15

        p = LibPosition.wrap(2, 0);
        assertEq(Position.unwrap(p), 4); // gindex = 4
        assertEq(p.depth(), 2); // depth = 2
        assertEq(p.indexAtDepth(), 0); // indexAtDepth = 0
        r = p.rightIndex(maxDepth);
        assertEq(Position.unwrap(r), 19); // right gindex = 19
        assertEq(r.indexAtDepth(), 3); // trace index = 3

        p = LibPosition.wrap(2, 1);
        assertEq(Position.unwrap(p), 5); // gindex = 5
        assertEq(p.depth(), 2); // depth = 2
        assertEq(p.indexAtDepth(), 1); // indexAtDepth = 1
        r = p.rightIndex(maxDepth);
        assertEq(Position.unwrap(r), 23); // right gindex = 23
        assertEq(r.indexAtDepth(), 7); // trace index = 7

        p = LibPosition.wrap(2, 2);
        assertEq(Position.unwrap(p), 6); // gindex = 6
        assertEq(p.depth(), 2); // depth = 2
        assertEq(p.indexAtDepth(), 2); // indexAtDepth = 2
        r = p.rightIndex(maxDepth);
        assertEq(Position.unwrap(r), 27); // right gindex = 27
        assertEq(r.indexAtDepth(), 11); // trace index = 11

        p = LibPosition.wrap(2, 3);
        assertEq(Position.unwrap(p), 7); // gindex = 7
        assertEq(p.depth(), 2); // depth = 2
        assertEq(p.indexAtDepth(), 3); // indexAtDepth = 3
        r = p.rightIndex(maxDepth);
        assertEq(Position.unwrap(r), 31); // right gindex = 31
        assertEq(r.indexAtDepth(), 15); // trace index = 15

        p = LibPosition.wrap(3, 0);
        assertEq(Position.unwrap(p), 8); // gindex = 8
        assertEq(p.depth(), 3); // depth = 3
        assertEq(p.indexAtDepth(), 0); // indexAtDepth = 0
        r = p.rightIndex(maxDepth);
        assertEq(Position.unwrap(r), 17); // right gindex = 17
        assertEq(r.indexAtDepth(), 1); // trace index = 1

        p = LibPosition.wrap(3, 1);
        assertEq(Position.unwrap(p), 9); // gindex = 9
        assertEq(p.depth(), 3); // depth = 3
        assertEq(p.indexAtDepth(), 1); // indexAtDepth = 1
        r = p.rightIndex(maxDepth);
        assertEq(Position.unwrap(r), 19); // right gindex = 19
        assertEq(r.indexAtDepth(), 3); // trace index = 3

        p = LibPosition.wrap(3, 2);
        assertEq(Position.unwrap(p), 10); // gindex = 10
        assertEq(p.depth(), 3); // depth = 3
        assertEq(p.indexAtDepth(), 2); // indexAtDepth = 2
        r = p.rightIndex(maxDepth);
        assertEq(Position.unwrap(r), 21); // right gindex = 21
        assertEq(r.indexAtDepth(), 5); // trace index = 5

        p = LibPosition.wrap(3, 3);
        assertEq(Position.unwrap(p), 11); // gindex = 11
        assertEq(p.depth(), 3); // depth = 3
        assertEq(p.indexAtDepth(), 3); // indexAtDepth = 3
        r = p.rightIndex(maxDepth);
        assertEq(Position.unwrap(r), 23); // right gindex = 23
        assertEq(r.indexAtDepth(), 7); // trace index = 7

        p = LibPosition.wrap(3, 4);
        assertEq(Position.unwrap(p), 12); // gindex = 12
        assertEq(p.depth(), 3); // depth = 3
        assertEq(p.indexAtDepth(), 4); // indexAtDepth = 4
        r = p.rightIndex(maxDepth);
        assertEq(Position.unwrap(r), 25); // right gindex = 25
        assertEq(r.indexAtDepth(), 9); // trace index = 9

        p = LibPosition.wrap(3, 5);
        assertEq(Position.unwrap(p), 13); // gindex = 13
        assertEq(p.depth(), 3); // depth = 3
        assertEq(p.indexAtDepth(), 5); // indexAtDepth = 5
        r = p.rightIndex(maxDepth);
        assertEq(Position.unwrap(r), 27); // right gindex = 27
        assertEq(r.indexAtDepth(), 11); // trace index = 11

        p = LibPosition.wrap(3, 6);
        assertEq(Position.unwrap(p), 14); // gindex = 14
        assertEq(p.depth(), 3); // depth = 3
        assertEq(p.indexAtDepth(), 6); // indexAtDepth = 6
        r = p.rightIndex(maxDepth);
        assertEq(Position.unwrap(r), 29); // right gindex = 29
        assertEq(r.indexAtDepth(), 13); // trace index = 13

        p = LibPosition.wrap(3, 7);
        assertEq(Position.unwrap(p), 15); // gindex = 15
        assertEq(p.depth(), 3); // depth = 3
        assertEq(p.indexAtDepth(), 7); // indexAtDepth = 7
        r = p.rightIndex(maxDepth);
        assertEq(Position.unwrap(r), 31); // right gindex = 31
        assertEq(r.indexAtDepth(), 15); // trace index = 15

        p = LibPosition.wrap(4, 0);
        assertEq(Position.unwrap(p), 16); // gindex = 16
        assertEq(p.depth(), 4); // depth = 4
        assertEq(p.indexAtDepth(), 0); // indexAtDepth = 0
        r = p.rightIndex(maxDepth);
        assertEq(Position.unwrap(r), 16); // right gindex = 16
        assertEq(r.indexAtDepth(), 0); // trace index = 0

        p = LibPosition.wrap(4, 1);
        assertEq(Position.unwrap(p), 17); // gindex = 17
        assertEq(p.depth(), 4); // depth = 4
        assertEq(p.indexAtDepth(), 1); // indexAtDepth = 1
        r = p.rightIndex(maxDepth);
        assertEq(Position.unwrap(r), 17); // right gindex = 17
        assertEq(r.indexAtDepth(), 1); // trace index = 1

        p = LibPosition.wrap(4, 2);
        assertEq(Position.unwrap(p), 18); // gindex = 18
        assertEq(p.depth(), 4); // depth = 4
        assertEq(p.indexAtDepth(), 2); // indexAtDepth = 2
        r = p.rightIndex(maxDepth);
        assertEq(Position.unwrap(r), 18); // right gindex = 18
        assertEq(r.indexAtDepth(), 2); // trace index = 2

        p = LibPosition.wrap(4, 3);
        assertEq(Position.unwrap(p), 19); // gindex = 19
        assertEq(p.depth(), 4); // depth = 4
        assertEq(p.indexAtDepth(), 3); // indexAtDepth = 3
        r = p.rightIndex(maxDepth);
        assertEq(Position.unwrap(r), 19); // right gindex = 19
        assertEq(r.indexAtDepth(), 3); // trace index = 3

        p = LibPosition.wrap(4, 4);
        assertEq(Position.unwrap(p), 20); // gindex = 20
        assertEq(p.depth(), 4); // depth = 4
        assertEq(p.indexAtDepth(), 4); // indexAtDepth = 4
        r = p.rightIndex(maxDepth);
        assertEq(Position.unwrap(r), 20); // right gindex = 20
        assertEq(r.indexAtDepth(), 4); // trace index = 4

        p = LibPosition.wrap(4, 5);
        assertEq(Position.unwrap(p), 21); // gindex = 21
        assertEq(p.depth(), 4); // depth = 4
        assertEq(p.indexAtDepth(), 5); // indexAtDepth = 5
        r = p.rightIndex(maxDepth);
        assertEq(Position.unwrap(r), 21); // right gindex = 21
        assertEq(r.indexAtDepth(), 5); // trace index = 5

        p = LibPosition.wrap(4, 6);
        assertEq(Position.unwrap(p), 22); // gindex = 22
        assertEq(p.depth(), 4); // depth = 4
        assertEq(p.indexAtDepth(), 6); // indexAtDepth = 6
        r = p.rightIndex(maxDepth);
        assertEq(Position.unwrap(r), 22); // right gindex = 22
        assertEq(r.indexAtDepth(), 6); // trace index = 6

        p = LibPosition.wrap(4, 7);
        assertEq(Position.unwrap(p), 23); // gindex = 23
        assertEq(p.depth(), 4); // depth = 4
        assertEq(p.indexAtDepth(), 7); // indexAtDepth = 7
        r = p.rightIndex(maxDepth);
        assertEq(Position.unwrap(r), 23); // right gindex = 23
        assertEq(r.indexAtDepth(), 7); // trace index = 7

        p = LibPosition.wrap(4, 8);
        assertEq(Position.unwrap(p), 24); // gindex = 24
        assertEq(p.depth(), 4); // depth = 4
        assertEq(p.indexAtDepth(), 8); // indexAtDepth = 8
        r = p.rightIndex(maxDepth);
        assertEq(Position.unwrap(r), 24); // right gindex = 24
        assertEq(r.indexAtDepth(), 8); // trace index = 8

        p = LibPosition.wrap(4, 9);
        assertEq(Position.unwrap(p), 25); // gindex = 25
        assertEq(p.depth(), 4); // depth = 4
        assertEq(p.indexAtDepth(), 9); // indexAtDepth = 9
        r = p.rightIndex(maxDepth);
        assertEq(Position.unwrap(r), 25); // right gindex = 25
        assertEq(r.indexAtDepth(), 9); // trace index = 9

        p = LibPosition.wrap(4, 10);
        assertEq(Position.unwrap(p), 26); // gindex = 26
        assertEq(p.depth(), 4); // depth = 4
        assertEq(p.indexAtDepth(), 10); // indexAtDepth = 10
        r = p.rightIndex(maxDepth);
        assertEq(Position.unwrap(r), 26); // right gindex = 26
        assertEq(r.indexAtDepth(), 10); // trace index = 10

        p = LibPosition.wrap(4, 11);
        assertEq(Position.unwrap(p), 27); // gindex = 27
        assertEq(p.depth(), 4); // depth = 4
        assertEq(p.indexAtDepth(), 11); // indexAtDepth = 11
        r = p.rightIndex(maxDepth);
        assertEq(Position.unwrap(r), 27); // right gindex = 27
        assertEq(r.indexAtDepth(), 11); // trace index = 11

        p = LibPosition.wrap(4, 12);
        assertEq(Position.unwrap(p), 28); // gindex = 28
        assertEq(p.depth(), 4); // depth = 4
        assertEq(p.indexAtDepth(), 12); // indexAtDepth = 12
        r = p.rightIndex(maxDepth);
        assertEq(Position.unwrap(r), 28); // right gindex = 28
        assertEq(r.indexAtDepth(), 12); // trace index = 12

        p = LibPosition.wrap(4, 13);
        assertEq(Position.unwrap(p), 29); // gindex = 29
        assertEq(p.depth(), 4); // depth = 4
        assertEq(p.indexAtDepth(), 13); // indexAtDepth = 13
        r = p.rightIndex(maxDepth);
        assertEq(Position.unwrap(r), 29); // right gindex = 29
        assertEq(r.indexAtDepth(), 13); // trace index = 13

        p = LibPosition.wrap(4, 14);
        assertEq(Position.unwrap(p), 30); // gindex = 30
        assertEq(p.depth(), 4); // depth = 4
        assertEq(p.indexAtDepth(), 14); // indexAtDepth = 14
        r = p.rightIndex(maxDepth);
        assertEq(Position.unwrap(r), 30); // right gindex = 30
        assertEq(r.indexAtDepth(), 14); // trace index = 14

        p = LibPosition.wrap(4, 15);
        assertEq(Position.unwrap(p), 31); // gindex = 31
        assertEq(p.depth(), 4); // depth = 4
        assertEq(p.indexAtDepth(), 15); // indexAtDepth = 15
        r = p.rightIndex(maxDepth);
        assertEq(Position.unwrap(r), 31); // right gindex = 31
        assertEq(r.indexAtDepth(), 15); // trace index = 15
    }
}
