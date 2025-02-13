// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing utilities
import { CommonTest } from "test/setup/CommonTest.sol";

// Target contract
import {
    LibSuperRoot,
    SuperRoot,
    UnexpectedLength,
    InvalidVersion,
    OutputRootNotFound,
    OutputRootsNotSorted,
    SUPER_ROOT_VERSION,
    OutputRootWithChainId
} from "src/libraries/SuperRoot.sol";

/// @title LibSuperRoot_Test
/// @notice Tests for the `LibSuperRoot` library.
contract LibSuperRoot_Test is CommonTest {
    /// @dev The minimum length of a super root.
    uint256 constant MIN_SUPER_ROOT_LENGTH = 1 + 8 + 64;

    /// @dev The test harness contract for `LibSuperRoot.decode`.
    LibSuperRoot_CalldataDecodeHarness public harness;

    function setUp() public override {
        super.setUp();
        harness = new LibSuperRoot_CalldataDecodeHarness();
    }

    /// @notice Tests that `LibSuperRoot.decode` can decode a static super root case.
    function test_decode_static_succeeds() external view {
        bytes memory exampleCase =
            hex"01000000000000c0dedeaddeaddeaddeaddeaddeaddeaddeaddeaddeaddeaddeaddeaddeaddeaddeadbeefbeefbeefbeefbeefbeefbeefbeefbeefbeefbeefbeefbeefbeefbeefbeef";

        SuperRoot memory superRoot = harness.decode(exampleCase);
        assertEq(superRoot.timestamp, 0xc0de);
        assertEq(superRoot.outputRoots.length, 1);
        assertEq(superRoot.outputRoots[0].l2ChainId, 0xdeaddeaddeaddeaddeaddeaddeaddeaddeaddeaddeaddeaddeaddeaddeaddead);
        assertEq(
            superRoot.outputRoots[0].outputRoot, 0xbeefbeefbeefbeefbeefbeefbeefbeefbeefbeefbeefbeefbeefbeefbeefbeef
        );
    }

    /// @notice Tests that `LibSuperRoot.decode` is memory safe.
    function test_decode_memorySafety_succeeds() external {
        bytes memory exampleCase =
            hex"01000000000000c0dedeaddeaddeaddeaddeaddeaddeaddeaddeaddeaddeaddeaddeaddeaddeaddeadbeefbeefbeefbeefbeefbeefbeefbeefbeefbeefbeefbeefbeefbeefbeefbeef";
        harness.decodeCheckMemorySafety(exampleCase);
    }

    /// @notice Tests that `LibSuperRoot.decode` reverts when the data is less than 1 + 8 + 64 bytes.
    function test_decode_insufficientData_reverts(uint256 _seed) external {
        _seed = bound(_seed, 1, MIN_SUPER_ROOT_LENGTH - 1);
        bytes memory data = new bytes(_seed);

        // Set a valid super root version, to ensure that the test will not fail due to
        // the version check.
        data[0] = SUPER_ROOT_VERSION;

        vm.expectRevert(UnexpectedLength.selector);
        harness.decode(data);
    }

    /// @notice Tests that `LibSuperRoot.decode` reverts when the version byte is incorrect.
    function test_decode_invalidVersion_reverts(bytes1 _seed) external {
        // If the seed is the SUPER_ROOT_VERSION, increment it to avoid the success case.
        if (_seed == SUPER_ROOT_VERSION) {
            _seed = bytes1(uint8(_seed) + 1);
        }

        bytes memory data = new bytes(MIN_SUPER_ROOT_LENGTH);
        data[0] = _seed;

        vm.expectRevert(InvalidVersion.selector);
        harness.decode(data);
    }

    /// @notice Tests that `LibSuperRoot.decode` reverts when the length of the output root tuple array
    ///         is not a multiple of 64 bytes with at least one element.
    function test_decode_badOutputRootsLength_reverts(bytes calldata _seed) external {
        bytes memory baseData = new bytes(MIN_SUPER_ROOT_LENGTH);
        baseData[0] = SUPER_ROOT_VERSION;

        // If the seed a multiple of 64 bytes, append some extra data to avoid the success case.
        bytes memory outputRoots = _seed;
        if (_seed.length % 64 == 0) {
            outputRoots = abi.encodePacked(_seed, hex"0badc0de");
        }

        vm.expectRevert(UnexpectedLength.selector);
        harness.decode(abi.encodePacked(baseData, outputRoots));
    }

    /// @notice Tests that `LibSuperRoot.decode` reverts when the output roots are not sorted in
    ///         ascending order by L2 chain ID.
    function test_decode_unsortedOutputRoots_reverts() external {
        bytes memory exampleCase =
            hex"01000000000000c0dedeaddeaddeaddeaddeaddeaddeaddeaddeaddeaddeaddeaddeaddeaddeaddeadbeefbeefbeefbeefbeefbeefbeefbeefbeefbeefbeefbeefbeefbeefbeefbeef0000000000000000000000000000000000000000000000000000000000000000beefbeefbeefbeefbeefbeefbeefbeefbeefbeefbeefbeefbeefbeefbeefbeef";

        vm.expectRevert(OutputRootsNotSorted.selector);
        harness.decode(exampleCase);
    }

    /// @notice Tests that `LibSuperRoot.findOutputRoot` can find an output root in a super root by chain ID.
    function test_findOutputRoot_succeeds(bytes32[] calldata _outputRoots, uint256 _index) external pure {
        vm.assume(_outputRoots.length > 0);

        // Create a sorted array of output roots.
        OutputRootWithChainId[] memory sortedOutputRoots = new OutputRootWithChainId[](_outputRoots.length);
        for (uint256 i = 0; i < _outputRoots.length; i++) {
            sortedOutputRoots[i] = OutputRootWithChainId({ l2ChainId: i, outputRoot: _outputRoots[i] });
        }

        _index = bound(_index, 0, sortedOutputRoots.length - 1);
        bytes32 expectedOutputRoot = sortedOutputRoots[_index].outputRoot;
        uint256 expectedChainId = sortedOutputRoots[_index].l2ChainId;

        SuperRoot memory superRoot = SuperRoot({ timestamp: 0, outputRoots: sortedOutputRoots });

        bytes32 outputRoot = LibSuperRoot.findOutputRoot(superRoot, expectedChainId);
        assertEq(outputRoot, expectedOutputRoot);
    }

    /// @notice Tests that `LibSuperRoot.findOutputRoot` reverts when the chain ID being searched for is not
    ///         present in the super root.
    function test_findOutputRoot_notFound_reverts(bytes32[] calldata _outputRoots) external {
        vm.assume(_outputRoots.length > 0);

        // Create a sorted array of output roots.
        OutputRootWithChainId[] memory sortedOutputRoots = new OutputRootWithChainId[](_outputRoots.length);
        for (uint256 i = 0; i < _outputRoots.length; i++) {
            sortedOutputRoots[i] = OutputRootWithChainId({ l2ChainId: i, outputRoot: _outputRoots[i] });
        }

        SuperRoot memory superRoot = SuperRoot({ timestamp: 0, outputRoots: sortedOutputRoots });

        vm.expectRevert(OutputRootNotFound.selector);
        LibSuperRoot.findOutputRoot(superRoot, sortedOutputRoots.length);
    }
}

/// @title LibSuperRoot_CalldataDecodeHarness
/// @notice Harness for testing the `LibSuperRoot.decode` function, which accepts calldata.
contract LibSuperRoot_CalldataDecodeHarness is CommonTest {
    function decode(bytes calldata _raw) external pure returns (SuperRoot memory) {
        return LibSuperRoot.decode(_raw);
    }

    function decodeCheckMemorySafety(bytes calldata _raw) external {
        uint64 ptr;
        assembly {
            ptr := mload(0x40)
        }
        uint64 outputRootsArrSize = 0x20 + 0x20;
        uint64 outputRootsDataSize = 0x40;
        uint64 superRootSize = 0x40;

        vm.expectSafeMemory(ptr, ptr + 0x40 + outputRootsArrSize + outputRootsDataSize + superRootSize);
        LibSuperRoot.decode(_raw);
    }
}
