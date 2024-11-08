// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { Test, Vm, console2 as console } from "forge-std/Test.sol";

// Scripts
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

// Libraries
import { LibKeccak } from "@lib-keccak/LibKeccak.sol";
import { PreimageKeyLib } from "src/cannon/PreimageKeyLib.sol";
import { Bytes } from "src/libraries/Bytes.sol";
import { Process } from "scripts/libraries/Process.sol";
import "src/cannon/libraries/CannonErrors.sol";
import "src/cannon/libraries/CannonTypes.sol";

// Interfaces
import { IPreimageOracle } from "src/cannon/interfaces/IPreimageOracle.sol";

contract PreimageOracle_Test is Test {
    IPreimageOracle oracle;

    /// @notice Sets up the testing suite.
    function setUp() public {
        oracle = IPreimageOracle(
            DeployUtils.create1({
                _name: "PreimageOracle",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IPreimageOracle.__constructor__, (0, 0)))
            })
        );
        vm.label(address(oracle), "PreimageOracle");
    }

    /// @notice Tests that the challenge period cannot be made too large.
    /// @param _challengePeriod The challenge period to test.
    function testFuzz_constructor_challengePeriodTooLarge_reverts(uint256 _challengePeriod) public {
        _challengePeriod = bound(_challengePeriod, uint256(type(uint64).max) + 1, type(uint256).max);
        vm.expectRevert("PreimageOracle: challenge period too large");
        DeployUtils.create1({
            _name: "PreimageOracle",
            _args: DeployUtils.encodeConstructor(abi.encodeCall(IPreimageOracle.__constructor__, (0, _challengePeriod)))
        });
    }

    /// @notice Test the pre-image key computation with a known pre-image.
    function test_keccak256PreimageKey_succeeds() public pure {
        bytes memory preimage = hex"deadbeef";
        bytes32 key = PreimageKeyLib.keccak256PreimageKey(preimage);
        bytes32 known = 0x02fd4e189132273036449fc9e11198c739161b4c0116a9a2dccdfa1c492006f1;
        assertEq(key, known);
    }

    /// @notice Tests that context-specific data [0, 24] bytes in length can be loaded correctly.
    function test_loadLocalData_onePart_succeeds() public {
        uint256 ident = 1;
        bytes32 word = bytes32(uint256(0xdeadbeef) << 224);
        uint8 size = 4;
        uint8 partOffset = 0;

        // Load the local data into the preimage oracle under the test contract's context.
        bytes32 contextKey = oracle.loadLocalData(ident, 0, word, size, partOffset);

        // Validate that the pre-image part is set
        bool ok = oracle.preimagePartOk(contextKey, partOffset);
        assertTrue(ok);

        // Validate the local data part
        bytes32 expectedPart = 0x0000000000000004deadbeef0000000000000000000000000000000000000000;
        assertEq(oracle.preimageParts(contextKey, partOffset), expectedPart);

        // Validate the local data length
        uint256 length = oracle.preimageLengths(contextKey);
        assertEq(length, size);
    }

    /// @notice Tests that multiple local key contexts can be used by the same address for the
    ///         same local data identifier.
    function test_loadLocalData_multipleContexts_succeeds() public {
        uint256 ident = 1;
        uint8 size = 4;
        uint8 partOffset = 0;

        // Form the words we'll be storing
        bytes32[2] memory words = [bytes32(uint256(0xdeadbeef) << 224), bytes32(uint256(0xbeefbabe) << 224)];

        for (uint256 i; i < words.length; i++) {
            // Load the local data into the preimage oracle under the test contract's context
            // and the given local context.
            bytes32 contextKey = oracle.loadLocalData(ident, bytes32(i), words[i], size, partOffset);

            // Validate that the pre-image part is set
            bool ok = oracle.preimagePartOk(contextKey, partOffset);
            assertTrue(ok);

            // Validate the local data part
            bytes32 expectedPart = bytes32(uint256(words[i] >> 64) | uint256(size) << 192);
            assertEq(oracle.preimageParts(contextKey, partOffset), expectedPart);

            // Validate the local data length
            uint256 length = oracle.preimageLengths(contextKey);
            assertEq(length, size);
        }
    }

    /// @notice Tests that context-specific data [0, 32] bytes in length can be loaded correctly.
    function testFuzz_loadLocalData_varyingLength_succeeds(
        uint256 ident,
        bytes32 localContext,
        bytes32 word,
        uint256 size,
        uint256 partOffset
    )
        public
    {
        // Bound the size to [0, 32]
        size = bound(size, 0, 32);
        // Bound the part offset to [0, size + 8)
        partOffset = bound(partOffset, 0, size + 7);

        // Load the local data into the preimage oracle under the test contract's context.
        bytes32 contextKey = oracle.loadLocalData(ident, localContext, word, uint8(size), uint8(partOffset));

        // Validate that the first local data part is set
        bool ok = oracle.preimagePartOk(contextKey, partOffset);
        assertTrue(ok);
        // Validate the first local data part
        bytes32 expectedPart;
        assembly {
            mstore(0x20, 0x00)

            mstore(0x00, shl(192, size))
            mstore(0x08, word)

            expectedPart := mload(partOffset)
        }
        assertEq(oracle.preimageParts(contextKey, partOffset), expectedPart);

        // Validate the local data length
        uint256 length = oracle.preimageLengths(contextKey);
        assertEq(length, size);
    }

    /// @notice Tests that a pre-image is correctly set.
    function test_loadKeccak256PreimagePart_succeeds() public {
        // Set the pre-image
        bytes memory preimage = hex"deadbeef";
        bytes32 key = PreimageKeyLib.keccak256PreimageKey(preimage);
        uint256 offset = 0;
        oracle.loadKeccak256PreimagePart(offset, preimage);

        // Validate the pre-image part
        bytes32 part = oracle.preimageParts(key, offset);
        bytes32 expectedPart = 0x0000000000000004deadbeef0000000000000000000000000000000000000000;
        assertEq(part, expectedPart);

        // Validate the pre-image length
        uint256 length = oracle.preimageLengths(key);
        assertEq(length, preimage.length);

        // Validate that the pre-image part is set
        bool ok = oracle.preimagePartOk(key, offset);
        assertTrue(ok);
    }

    /// @notice Tests that adding a global keccak256 pre-image at the part boundary reverts.
    function test_loadKeccak256PreimagePart_partBoundary_reverts() public {
        bytes memory preimage = hex"deadbeef";
        uint256 offset = preimage.length + 8;

        vm.expectRevert(PartOffsetOOB.selector);
        oracle.loadKeccak256PreimagePart(offset, preimage);
    }

    /// @notice Tests that a pre-image cannot be set with an out-of-bounds offset.
    function test_loadLocalData_outOfBoundsOffset_reverts() public {
        bytes32 preimage = bytes32(uint256(0xdeadbeef));
        uint256 offset = preimage.length + 9;

        vm.expectRevert(PartOffsetOOB.selector);
        oracle.loadLocalData(1, 0, preimage, 32, offset);
    }

    /// @notice Tests that a pre-image cannot be set with an out-of-bounds offset.
    function test_loadKeccak256PreimagePart_outOfBoundsOffset_reverts() public {
        bytes memory preimage = hex"deadbeef";
        uint256 offset = preimage.length + 9;

        vm.expectRevert(PartOffsetOOB.selector);
        oracle.loadKeccak256PreimagePart(offset, preimage);
    }

    /// @notice Reading a pre-image part that has not been set should revert.
    function testFuzz_readPreimage_missingPreimage_reverts(bytes32 key, uint256 offset) public {
        vm.expectRevert("pre-image must exist");
        oracle.readPreimage(key, offset);
    }

    /// @notice Tests that a precompile pre-image result is correctly set.
    function test_loadPrecompilePreimagePart_succeeds() public {
        bytes memory input = hex"deadbeef";
        uint256 offset = 0;
        address precompile = address(bytes20(uint160(0x02))); // sha256
        uint64 gas = 72;
        bytes32 key = precompilePreimageKey(precompile, gas, input);
        oracle.loadPrecompilePreimagePart(offset, precompile, gas, input);

        bytes32 part = oracle.preimageParts(key, offset);
        // size prefix - 1-byte result + 32-byte sha return data
        assertEq(hex"0000000000000021", bytes8(part));
        // precompile result
        assertEq(bytes1(0x01), bytes1(part << 64));
        // precompile call return data
        assertEq(bytes23(sha256(input)), bytes23(part << 72));

        // Validate the local data length
        uint256 length = oracle.preimageLengths(key);
        assertEq(length, 1 + 32);

        // Validate that the first local data part is set
        bool ok = oracle.preimagePartOk(key, offset);
        assertTrue(ok);
    }

    /// @notice Tests that a precompile pre-image result is correctly set at its return data offset.
    function test_loadPrecompilePreimagePart_atReturnOffset_succeeds() public {
        bytes memory input = hex"deadbeef";
        uint256 offset = 9;
        address precompile = address(bytes20(uint160(0x02))); // sha256
        uint64 gas = 72;
        bytes32 key = precompilePreimageKey(precompile, gas, input);
        oracle.loadPrecompilePreimagePart(offset, precompile, gas, input);

        bytes32 part = oracle.preimageParts(key, offset);
        // 32-byte sha return data
        assertEq(sha256(input), part);

        // Validate the local data length
        uint256 length = oracle.preimageLengths(key);
        assertEq(length, 1 + 32);

        // Validate that the first local data part is set
        bool ok = oracle.preimagePartOk(key, offset);
        assertTrue(ok);
    }

    /// @notice Tests that a failed precompile call has a zero status byte in preimage
    function test_loadPrecompilePreimagePart_failedCall_succeeds() public {
        bytes memory input = new bytes(193); // invalid input to induce a failed precompile call
        uint256 offset = 0;
        address precompile = address(bytes20(uint160(0x08))); // bn256Pairing
        uint64 gas = 72;
        bytes32 key = precompilePreimageKey(precompile, gas, input);
        oracle.loadPrecompilePreimagePart(offset, precompile, gas, input);

        bytes32 part = oracle.preimageParts(key, offset);
        // size prefix - 1-byte result + 0-byte sha return data
        assertEq(hex"0000000000000001", bytes8(part));
        // precompile result
        assertEq(bytes1(0x00), bytes1(part << 64));
        // precompile call return data
        assertEq(bytes23(0), bytes23(part << 72));

        // Validate the local data length
        uint256 length = oracle.preimageLengths(key);
        assertEq(length, 1);

        // Validate that the first local data part is set
        bool ok = oracle.preimagePartOk(key, offset);
        assertTrue(ok);
    }

    /// @notice Tests that adding a global precompile result at the part boundary reverts.
    function test_loadPrecompilePreimagePart_partBoundary_reverts() public {
        bytes memory input = hex"deadbeef";
        uint256 offset = 41; // 8-byte prefix + 1-byte result + 32-byte sha return data
        address precompile = address(bytes20(uint160(0x02))); // sha256
        uint64 gas = 72;
        vm.expectRevert(PartOffsetOOB.selector);
        oracle.loadPrecompilePreimagePart(offset, precompile, gas, input);
    }

    /// @notice Tests that a global precompile result cannot be set with an out-of-bounds offset.
    function test_loadPrecompilePreimagePart_outOfBoundsOffset_reverts() public {
        bytes memory input = hex"deadbeef";
        uint256 offset = 42;
        address precompile = address(bytes20(uint160(0x02))); // sha256
        uint64 gas = 72;
        vm.expectRevert(PartOffsetOOB.selector);
        oracle.loadPrecompilePreimagePart(offset, precompile, gas, input);
    }

    /// @notice Tests that a global precompile load succeeds on a variety of gas inputs.
    function testFuzz_loadPrecompilePreimagePart_withVaryingGas_succeeds(uint64 _gas) public {
        uint64 requiredGas = 100_000;
        bytes memory input = hex"deadbeef";
        address precompile = address(uint160(0xdeadbeef));
        vm.mockCall(precompile, input, hex"abba");
        uint256 offset = 0;
        uint64 minGas = uint64(bound(_gas, requiredGas * 3, 20_000_000));
        vm.expectCallMinGas(precompile, 0, requiredGas, input);
        oracle.loadPrecompilePreimagePart{ gas: minGas }(offset, precompile, requiredGas, input);
    }

    /// @notice Tests that a global precompile load succeeds on insufficient gas.
    function test_loadPrecompilePreimagePart_withInsufficientGas_reverts() public {
        uint64 requiredGas = 1_000_000;
        bytes memory input = hex"deadbeef";
        uint256 offset = 0;
        address precompile = address(uint160(0xdeadbeef));
        // This gas is sufficient to reach the gas checks in `loadPrecompilePreimagePart` but not enough to pass those
        // checks
        uint64 insufficientGas = requiredGas * 63 / 64;
        vm.expectRevert(NotEnoughGas.selector);
        oracle.loadPrecompilePreimagePart{ gas: insufficientGas }(offset, precompile, requiredGas, input);
    }
}

contract PreimageOracle_LargePreimageProposals_Test is Test {
    uint256 internal constant MIN_SIZE_BYTES = 0;
    uint256 internal constant CHALLENGE_PERIOD = 1 days;
    uint256 internal constant TEST_UUID = 0xFACADE;

    IPreimageOracle internal oracle;

    /// @notice Sets up the testing suite.
    function setUp() public {
        oracle = IPreimageOracle(
            DeployUtils.create1({
                _name: "PreimageOracle",
                _args: DeployUtils.encodeConstructor(
                    abi.encodeCall(IPreimageOracle.__constructor__, (MIN_SIZE_BYTES, CHALLENGE_PERIOD))
                )
            })
        );
        vm.label(address(oracle), "PreimageOracle");

        // Set `tx.origin` and `msg.sender` to `address(this)` so that it may behave like an EOA for `addLeavesLPP`.
        vm.startPrank(address(this), address(this));

        // Give this address some ETH to work with.
        vm.deal(address(this), 100 ether);
    }

    /// @notice Tests that the `initLPP` function reverts when the part offset is out of bounds of the full preimage.
    function test_initLPP_partOffsetOOB_reverts() public {
        // Allocate the preimage data.
        bytes memory data = new bytes(136);
        for (uint256 i; i < data.length; i++) {
            data[i] = 0xFF;
        }

        // Initialize the proposal.
        uint256 bondSize = oracle.MIN_BOND_SIZE();
        vm.expectRevert(PartOffsetOOB.selector);
        oracle.initLPP{ value: bondSize }(TEST_UUID, 136 + 8, uint32(data.length));
    }

    /// @notice Tests that the `initLPP` function reverts when the part offset is out of bounds of the full preimage.
    function test_initLPP_sizeTooSmall_reverts() public {
        oracle = IPreimageOracle(
            DeployUtils.create1({
                _name: "PreimageOracle",
                _args: DeployUtils.encodeConstructor(
                    abi.encodeCall(IPreimageOracle.__constructor__, (1000, CHALLENGE_PERIOD))
                )
            })
        );

        // Allocate the preimage data.
        bytes memory data = new bytes(136);
        for (uint256 i; i < data.length; i++) {
            data[i] = 0xFF;
        }

        // Initialize the proposal.
        uint256 bondSize = oracle.MIN_BOND_SIZE();
        vm.expectRevert(InvalidInputSize.selector);
        oracle.initLPP{ value: bondSize }(TEST_UUID, 0, uint32(data.length));
    }

    /// @notice Tests that the `initLPP` function reverts if the proposal has already been initialized.
    function test_initLPP_alreadyInitialized_reverts() public {
        // Initialize the proposal.
        uint256 bondSize = oracle.MIN_BOND_SIZE();
        oracle.initLPP{ value: bondSize }(TEST_UUID, 0, uint32(500));

        // Re-initialize the proposal.
        vm.expectRevert(AlreadyInitialized.selector);
        oracle.initLPP{ value: bondSize }(TEST_UUID, 0, uint32(500));
    }

    /// @notice Gas snapshot for `addLeaves`
    function test_addLeaves_gasSnapshot_benchmark() public {
        // Allocate the preimage data.
        bytes memory data = new bytes(136 * 500);
        for (uint256 i; i < data.length; i++) {
            data[i] = 0xFF;
        }

        // Initialize the proposal.
        oracle.initLPP{ value: oracle.MIN_BOND_SIZE() }(TEST_UUID, 0, uint32(data.length));

        // Add the leaves to the tree (2 keccak blocks.)
        LibKeccak.StateMatrix memory stateMatrix;
        bytes32[] memory stateCommitments = _generateStateCommitments(stateMatrix, data);

        // Allocate the calldata so it isn't included in the gas measurement.
        bytes memory cd = abi.encodeCall(oracle.addLeavesLPP, (TEST_UUID, 0, data, stateCommitments, true));

        // Record logs from the call. `expectEmit` does not capture assembly logs.
        bytes memory expectedLog = abi.encodePacked(address(this), cd);
        vm.recordLogs();

        uint256 gas = gasleft();
        (bool success,) = address(oracle).call(cd);
        uint256 gasUsed = gas - gasleft();
        assertTrue(success);

        Vm.Log[] memory logs = vm.getRecordedLogs();
        assertEq(logs[0].data, expectedLog);
        assertEq(logs[0].emitter, address(oracle));

        console.log("Gas used: %d", gasUsed);
        console.log("Gas per byte (%d bytes streamed): %d", data.length, gasUsed / data.length);
        console.log("Gas for 4MB: %d", (gasUsed / data.length) * 4000000);
    }

    /// @notice Tests that `addLeavesLPP` sets the proposal as countered when `_finalize = true` and the number of
    ///         bytes processed is less than the claimed size.
    function test_addLeaves_mismatchedSize_succeeds() public {
        // Allocate the preimage data.
        bytes memory data = new bytes(136);
        for (uint256 i; i < data.length; i++) {
            data[i] = 0xFF;
        }

        // Initialize the proposal.
        oracle.initLPP{ value: oracle.MIN_BOND_SIZE() }(TEST_UUID, 0, uint32(data.length + 1));

        // Add the leaves to the tree (2 keccak blocks.)
        LibKeccak.StateMatrix memory stateMatrix;
        bytes32[] memory stateCommitments = _generateStateCommitments(stateMatrix, data);

        vm.expectRevert(InvalidInputSize.selector);
        oracle.addLeavesLPP(TEST_UUID, 0, data, stateCommitments, true);
    }

    /// @notice Tests that the `addLeavesLPP` function may never be called when `tx.origin != msg.sender`
    function test_addLeaves_notEOA_reverts() public {
        // Allocate the preimage data.
        bytes memory data = new bytes(136 * 500);

        // Initialize the proposal.
        oracle.initLPP{ value: oracle.MIN_BOND_SIZE() }(TEST_UUID, 0, uint32(data.length));

        // Add the leaves to the tree (2 keccak blocks.)
        LibKeccak.StateMatrix memory stateMatrix;
        bytes32[] memory stateCommitments = _generateStateCommitments(stateMatrix, data);

        // Replace the global prank, set `tx.origin` to `address(0)`, and set `msg.sender` to `address(this)`.
        vm.stopPrank();
        vm.prank(address(0), address(this));

        vm.expectRevert(NotEOA.selector);
        oracle.addLeavesLPP(TEST_UUID, 0, data, stateCommitments, true);
    }

    /// @notice Tests that the `addLeavesLPP` function reverts when the starting block index is not what is expected.
    function test_addLeaves_notContiguous_reverts() public {
        // Allocate the preimage data.
        bytes memory data = new bytes(136 * 500);

        // Initialize the proposal.
        oracle.initLPP{ value: oracle.MIN_BOND_SIZE() }(TEST_UUID, 0, uint32(data.length));

        // Add the leaves to the tree (2 keccak blocks.)
        LibKeccak.StateMatrix memory stateMatrix;
        bytes32[] memory stateCommitments = _generateStateCommitments(stateMatrix, data);

        vm.expectRevert(WrongStartingBlock.selector);
        oracle.addLeavesLPP(TEST_UUID, 1, data, stateCommitments, true);
    }

    /// @notice Tests that leaves can be added the large preimage proposal mapping and proven to be contained within
    ///         the computed merkle root.
    function test_addLeaves_multipleParts_succeeds() public {
        // Allocate the preimage data.
        bytes memory data = new bytes(136 * 3);
        for (uint256 i; i < data.length; i++) {
            data[i] = 0xFF;
        }

        // Initialize the proposal.
        oracle.initLPP{ value: oracle.MIN_BOND_SIZE() }(TEST_UUID, 0, uint32(data.length));
        // Ensure that the proposal keys are present in the array.
        (address claimant, uint256 uuid) = oracle.proposals(0);
        assertEq(oracle.proposalCount(), 1);
        assertEq(claimant, address(this));
        assertEq(uuid, TEST_UUID);

        // Add the leaves to the tree (2 keccak blocks.)
        LibKeccak.StateMatrix memory stateMatrix;
        bytes32[] memory stateCommitments = _generateStateCommitments(stateMatrix, data);

        uint256 midPoint = stateCommitments.length / 2;
        bytes32[] memory commitmentsA = new bytes32[](midPoint);
        bytes32[] memory commitmentsB = new bytes32[](midPoint);
        for (uint256 i = 0; i < midPoint; i++) {
            commitmentsA[i] = stateCommitments[i];
            commitmentsB[i] = stateCommitments[i + midPoint];
        }

        oracle.addLeavesLPP(TEST_UUID, 0, Bytes.slice(data, 0, 136 * 2), commitmentsA, false);

        // MetaData assertions
        LPPMetaData metaData = oracle.proposalMetadata(address(this), TEST_UUID);
        assertEq(metaData.timestamp(), 0);
        assertEq(metaData.partOffset(), 0);
        assertEq(metaData.claimedSize(), data.length);
        assertEq(metaData.blocksProcessed(), 2);
        assertEq(metaData.bytesProcessed(), 136 * 2);
        assertFalse(metaData.countered());

        // Move ahead one block.
        vm.roll(block.number + 1);

        oracle.addLeavesLPP(TEST_UUID, 2, Bytes.slice(data, 136 * 2, 136), commitmentsB, true);

        // MetaData assertions
        metaData = oracle.proposalMetadata(address(this), TEST_UUID);
        assertEq(metaData.timestamp(), 1);
        assertEq(metaData.partOffset(), 0);
        assertEq(metaData.claimedSize(), data.length);
        assertEq(metaData.blocksProcessed(), 4);
        assertEq(metaData.bytesProcessed(), data.length);
        assertFalse(metaData.countered());

        // Preimage part assertions
        bytes32 expectedPart = bytes32((~uint256(0) & ~(uint256(type(uint64).max) << 192)) | (data.length << 192));
        assertEq(oracle.proposalParts(address(this), TEST_UUID), expectedPart);

        assertEq(oracle.proposalBlocks(address(this), TEST_UUID, 0), block.number - 1);
        assertEq(oracle.proposalBlocks(address(this), TEST_UUID, 1), block.number);

        // Should revert if we try to add new leaves.
        vm.expectRevert(AlreadyFinalized.selector);
        oracle.addLeavesLPP(TEST_UUID, 4, data, stateCommitments, true);
    }

    /// @notice Tests that leaves cannot be added until the large preimage proposal has been initialized.
    function test_addLeaves_notInitialized_reverts() public {
        // Allocate the preimage data.
        bytes memory data = new bytes(136 * 500);

        // Add the leaves to the tree (2 keccak blocks.)
        LibKeccak.StateMatrix memory stateMatrix;
        bytes32[] memory stateCommitments = _generateStateCommitments(stateMatrix, data);

        // Allocate the calldata so it isn't included in the gas measurement.
        vm.expectRevert(NotInitialized.selector);
        oracle.addLeavesLPP(TEST_UUID, 0, data, stateCommitments, true);
    }

    /// @notice Tests that leaves can be added the large preimage proposal mapping and finalized to be added to the
    ///         authorized mappings.
    function test_squeeze_challengePeriodPassed_succeeds() public {
        // Allocate the preimage data.
        bytes memory data = new bytes(136);
        for (uint256 i; i < data.length; i++) {
            data[i] = 0xFF;
        }

        // Initialize the proposal.
        oracle.initLPP{ value: oracle.MIN_BOND_SIZE() }(TEST_UUID, 0, uint32(data.length));

        // Add the leaves to the tree (2 keccak blocks.)
        LibKeccak.StateMatrix memory stateMatrix;
        bytes32[] memory stateCommitments = _generateStateCommitments(stateMatrix, data);
        oracle.addLeavesLPP(TEST_UUID, 0, data, stateCommitments, true);

        // Construct the leaf preimage data for the blocks added.
        LibKeccak.StateMatrix memory matrix;
        IPreimageOracle.Leaf[] memory leaves = _generateLeaves(matrix, data);

        // Create a proof array with 16 elements.
        bytes32[] memory preProof = new bytes32[](16);
        preProof[0] = _hashLeaf(leaves[1]);
        bytes32[] memory postProof = new bytes32[](16);
        postProof[0] = _hashLeaf(leaves[0]);
        for (uint256 i = 1; i < preProof.length; i++) {
            bytes32 zeroHash = oracle.zeroHashes(i);
            preProof[i] = zeroHash;
            postProof[i] = zeroHash;
        }

        vm.warp(block.timestamp + oracle.challengePeriod() + 1 seconds);

        // Finalize the proposal.
        uint256 balanceBefore = address(this).balance;
        oracle.squeezeLPP({
            _claimant: address(this),
            _uuid: TEST_UUID,
            _stateMatrix: _stateMatrixAtBlockIndex(data, 1),
            _preState: leaves[0],
            _preStateProof: preProof,
            _postState: leaves[1],
            _postStateProof: postProof
        });
        assertEq(address(this).balance, balanceBefore + oracle.MIN_BOND_SIZE());
        assertEq(oracle.proposalBonds(address(this), TEST_UUID), 0);

        bytes32 finalDigest = _setStatusByte(keccak256(data), 2);
        bytes32 expectedPart = bytes32((~uint256(0) & ~(uint256(type(uint64).max) << 192)) | (data.length << 192));
        assertTrue(oracle.preimagePartOk(finalDigest, 0));
        assertEq(oracle.preimageLengths(finalDigest), data.length);
        assertEq(oracle.preimageParts(finalDigest, 0), expectedPart);
    }

    /// @notice Tests that a proposal cannot be finalized until it has passed the challenge period.
    function test_squeeze_proposalChallenged_reverts() public {
        // Allocate the preimage data.
        bytes memory data = new bytes(136);
        for (uint256 i; i < data.length; i++) {
            data[i] = 0xFF;
        }
        bytes memory phonyData = new bytes(136);

        // Initialize the proposal.
        oracle.initLPP{ value: oracle.MIN_BOND_SIZE() }(TEST_UUID, 0, uint32(data.length));

        // Add the leaves to the tree with mismatching state commitments.
        LibKeccak.StateMatrix memory stateMatrix;
        bytes32[] memory stateCommitments = _generateStateCommitments(stateMatrix, data);
        oracle.addLeavesLPP(TEST_UUID, 0, phonyData, stateCommitments, true);

        // Construct the leaf preimage data for the blocks added.
        LibKeccak.StateMatrix memory matrix;
        IPreimageOracle.Leaf[] memory leaves = _generateLeaves(matrix, phonyData);
        leaves[0].stateCommitment = stateCommitments[0];
        leaves[1].stateCommitment = stateCommitments[1];

        // Create a proof array with 16 elements.
        bytes32[] memory preProof = new bytes32[](16);
        preProof[0] = _hashLeaf(leaves[1]);
        bytes32[] memory postProof = new bytes32[](16);
        postProof[0] = _hashLeaf(leaves[0]);
        for (uint256 i = 1; i < preProof.length; i++) {
            bytes32 zeroHash = oracle.zeroHashes(i);
            preProof[i] = zeroHash;
            postProof[i] = zeroHash;
        }

        // Should succeed since the commitment was wrong.
        oracle.challengeFirstLPP({
            _claimant: address(this),
            _uuid: TEST_UUID,
            _postState: leaves[0],
            _postStateProof: preProof
        });

        LPPMetaData metaData = oracle.proposalMetadata(address(this), TEST_UUID);
        assertTrue(metaData.countered());

        vm.warp(block.timestamp + oracle.challengePeriod() + 1 seconds);

        // Finalize the proposal.
        vm.expectRevert(BadProposal.selector);
        oracle.squeezeLPP({
            _claimant: address(this),
            _uuid: TEST_UUID,
            _stateMatrix: _stateMatrixAtBlockIndex(data, 1),
            _preState: leaves[0],
            _preStateProof: preProof,
            _postState: leaves[1],
            _postStateProof: postProof
        });
    }

    /// @notice Tests that a proposal cannot be squeezed if the proposal has not been finalized.
    function test_squeeze_notFinalized_reverts() public {
        // Allocate the preimage data.
        bytes memory data = new bytes(136);
        for (uint256 i; i < data.length; i++) {
            data[i] = 0xFF;
        }

        // Initialize the proposal.
        oracle.initLPP{ value: oracle.MIN_BOND_SIZE() }(TEST_UUID, 0, uint32(data.length));

        // Generate the padded input data.
        // Since the data is 136 bytes, which is exactly one keccak block, we will add one extra
        // keccak block of empty padding to the input data. We need to do this here because the
        // addLeavesLPP function will normally perform this padding internally when _finalize is
        // set to true but we're explicitly testing the case where _finalize is not true.
        bytes memory paddedData = new bytes(136 * 2);
        for (uint256 i; i < data.length; i++) {
            paddedData[i] = data[i];
        }

        // Add the leaves to the tree (2 keccak blocks.)
        LibKeccak.StateMatrix memory stateMatrix;
        bytes32[] memory stateCommitments = _generateStateCommitments(stateMatrix, data);
        oracle.addLeavesLPP(TEST_UUID, 0, paddedData, stateCommitments, false);

        // Construct the leaf preimage data for the blocks added.
        LibKeccak.StateMatrix memory matrix;
        IPreimageOracle.Leaf[] memory leaves = _generateLeaves(matrix, data);

        // Create a proof array with 16 elements.
        bytes32[] memory preProof = new bytes32[](16);
        preProof[0] = _hashLeaf(leaves[1]);
        bytes32[] memory postProof = new bytes32[](16);
        postProof[0] = _hashLeaf(leaves[0]);
        for (uint256 i = 1; i < preProof.length; i++) {
            bytes32 zeroHash = oracle.zeroHashes(i);
            preProof[i] = zeroHash;
            postProof[i] = zeroHash;
        }

        // Warp past the challenge period.
        vm.warp(block.timestamp + oracle.challengePeriod() + 1 seconds);

        // Finalize the proposal.
        vm.expectRevert(ActiveProposal.selector);
        oracle.squeezeLPP({
            _claimant: address(this),
            _uuid: TEST_UUID,
            _stateMatrix: _stateMatrixAtBlockIndex(data, 1),
            _preState: leaves[0],
            _preStateProof: preProof,
            _postState: leaves[1],
            _postStateProof: postProof
        });
    }

    /// @notice Tests that a proposal cannot be finalized until it has passed the challenge period.
    function test_squeeze_challengePeriodActive_reverts() public {
        // Allocate the preimage data.
        bytes memory data = new bytes(136);
        for (uint256 i; i < data.length; i++) {
            data[i] = 0xFF;
        }

        // Initialize the proposal.
        oracle.initLPP{ value: oracle.MIN_BOND_SIZE() }(TEST_UUID, 0, uint32(data.length));

        // Add the leaves to the tree (2 keccak blocks.)
        LibKeccak.StateMatrix memory stateMatrix;
        bytes32[] memory stateCommitments = _generateStateCommitments(stateMatrix, data);
        oracle.addLeavesLPP(TEST_UUID, 0, data, stateCommitments, true);

        // Construct the leaf preimage data for the blocks added.
        LibKeccak.StateMatrix memory matrix;
        IPreimageOracle.Leaf[] memory leaves = _generateLeaves(matrix, data);

        // Finalize the proposal.
        vm.expectRevert(ActiveProposal.selector);
        oracle.squeezeLPP({
            _claimant: address(this),
            _uuid: TEST_UUID,
            _stateMatrix: _stateMatrixAtBlockIndex(data, 1),
            _preState: leaves[0],
            _preStateProof: new bytes32[](16),
            _postState: leaves[1],
            _postStateProof: new bytes32[](16)
        });
    }

    /// @notice Tests that a proposal cannot be finalized until it has passed the challenge period.
    function test_squeeze_incompleteAbsorbtion_reverts() public {
        // Allocate the preimage data.
        bytes memory data = new bytes(136);
        for (uint256 i; i < data.length; i++) {
            data[i] = 0xFF;
        }

        // Initialize the proposal.
        oracle.initLPP{ value: oracle.MIN_BOND_SIZE() }(TEST_UUID, 0, uint32(data.length));

        // Construct the leaf preimage data for the blocks added.
        LibKeccak.StateMatrix memory matrix;
        IPreimageOracle.Leaf[] memory leaves = _generateLeaves(matrix, data);

        // Finalize the proposal.
        vm.expectRevert(ActiveProposal.selector);
        oracle.squeezeLPP({
            _claimant: address(this),
            _uuid: TEST_UUID,
            _stateMatrix: _stateMatrixAtBlockIndex(data, 1),
            _preState: leaves[0],
            _preStateProof: new bytes32[](16),
            _postState: leaves[1],
            _postStateProof: new bytes32[](16)
        });
    }

    /// @notice Tests that the `squeeze` function reverts when the passed states are not contiguous.
    function test_squeeze_statesNotContiguous_reverts() public {
        // Allocate the preimage data.
        bytes memory data = new bytes(136);
        for (uint256 i; i < data.length; i++) {
            data[i] = 0xFF;
        }

        // Initialize the proposal.
        oracle.initLPP{ value: oracle.MIN_BOND_SIZE() }(TEST_UUID, 0, uint32(data.length));

        // Add the leaves to the tree (2 keccak blocks.)
        LibKeccak.StateMatrix memory stateMatrix;
        bytes32[] memory stateCommitments = _generateStateCommitments(stateMatrix, data);
        oracle.addLeavesLPP(TEST_UUID, 0, data, stateCommitments, true);

        // Construct the leaf preimage data for the blocks added.
        LibKeccak.StateMatrix memory matrix;
        IPreimageOracle.Leaf[] memory leaves = _generateLeaves(matrix, data);

        // Create a proof array with 16 elements.
        bytes32[] memory preProof = new bytes32[](16);
        preProof[0] = _hashLeaf(leaves[1]);
        bytes32[] memory postProof = new bytes32[](16);
        postProof[0] = _hashLeaf(leaves[0]);
        for (uint256 i = 1; i < preProof.length; i++) {
            bytes32 zeroHash = oracle.zeroHashes(i);
            preProof[i] = zeroHash;
            postProof[i] = zeroHash;
        }

        vm.warp(block.timestamp + oracle.challengePeriod() + 1 seconds);

        // Finalize the proposal.
        vm.expectRevert(StatesNotContiguous.selector);
        oracle.squeezeLPP({
            _claimant: address(this),
            _uuid: TEST_UUID,
            _stateMatrix: _stateMatrixAtBlockIndex(data, 2),
            _preState: leaves[1],
            _preStateProof: postProof,
            _postState: leaves[0],
            _postStateProof: preProof
        });
    }

    /// @notice Tests that the `squeeze` function reverts when the post state passed
    function test_squeeze_invalidPreimage_reverts() public {
        // Allocate the preimage data.
        bytes memory data = new bytes(136);
        for (uint256 i; i < data.length; i++) {
            data[i] = 0xFF;
        }

        // Initialize the proposal.
        oracle.initLPP{ value: oracle.MIN_BOND_SIZE() }(TEST_UUID, 0, uint32(data.length));

        // Add the leaves to the tree (2 keccak blocks.)
        LibKeccak.StateMatrix memory stateMatrix;
        bytes32[] memory stateCommitments = _generateStateCommitments(stateMatrix, data);
        oracle.addLeavesLPP(TEST_UUID, 0, data, stateCommitments, true);

        // Construct the leaf preimage data for the blocks added.
        LibKeccak.StateMatrix memory matrix;
        IPreimageOracle.Leaf[] memory leaves = _generateLeaves(matrix, data);

        // Create a proof array with 16 elements.
        bytes32[] memory preProof = new bytes32[](16);
        preProof[0] = _hashLeaf(leaves[1]);
        bytes32[] memory postProof = new bytes32[](16);
        postProof[0] = _hashLeaf(leaves[0]);
        for (uint256 i = 1; i < preProof.length; i++) {
            bytes32 zeroHash = oracle.zeroHashes(i);
            preProof[i] = zeroHash;
            postProof[i] = zeroHash;
        }

        vm.warp(block.timestamp + oracle.challengePeriod() + 1 seconds);

        // Finalize the proposal.
        vm.expectRevert(InvalidPreimage.selector);
        oracle.squeezeLPP({
            _claimant: address(this),
            _uuid: TEST_UUID,
            _stateMatrix: _stateMatrixAtBlockIndex(data, 2),
            _preState: leaves[0],
            _preStateProof: preProof,
            _postState: leaves[1],
            _postStateProof: postProof
        });
    }

    /// @notice Tests that squeezing a large preimage proposal after the challenge period has passed always succeeds and
    ///         persists the correct data.
    /// forge-config: ciheavy.fuzz.runs = 512
    function testFuzz_squeezeLPP_succeeds(uint256 _numBlocks, uint32 _partOffset) public {
        _numBlocks = bound(_numBlocks, 1, 2 ** 8);
        _partOffset = uint32(bound(_partOffset, 0, _numBlocks * LibKeccak.BLOCK_SIZE_BYTES + 8 - 1));

        // Allocate the preimage data.
        bytes memory data = new bytes(136 * _numBlocks);
        for (uint256 i; i < data.length; i++) {
            data[i] = bytes1(uint8(i % 256));
        }

        // Propose and squeeze a large preimage.
        {
            // Initialize the proposal.
            oracle.initLPP{ value: oracle.MIN_BOND_SIZE() }(TEST_UUID, _partOffset, uint32(data.length));

            // Add the leaves to the tree with correct state commitments.
            LibKeccak.StateMatrix memory matrixA;
            bytes32[] memory stateCommitments = _generateStateCommitments(matrixA, data);
            oracle.addLeavesLPP(TEST_UUID, 0, data, stateCommitments, true);

            // Construct the leaf preimage data for the blocks added.
            LibKeccak.StateMatrix memory matrixB;
            IPreimageOracle.Leaf[] memory leaves = _generateLeaves(matrixB, data);

            // Fetch the merkle proofs for the pre/post state leaves in the proposal tree.
            bytes32 canonicalRoot = oracle.getTreeRootLPP(address(this), TEST_UUID);
            (bytes32 rootA, bytes32[] memory preProof) = _generateProof(leaves.length - 2, leaves);
            assertEq(rootA, canonicalRoot);
            (bytes32 rootB, bytes32[] memory postProof) = _generateProof(leaves.length - 1, leaves);
            assertEq(rootB, canonicalRoot);

            // Warp past the challenge period.
            vm.warp(block.timestamp + CHALLENGE_PERIOD + 1 seconds);

            // Squeeze the LPP.
            LibKeccak.StateMatrix memory preMatrix = _stateMatrixAtBlockIndex(data, leaves.length - 1);
            oracle.squeezeLPP({
                _claimant: address(this),
                _uuid: TEST_UUID,
                _stateMatrix: preMatrix,
                _preState: leaves[leaves.length - 2],
                _preStateProof: preProof,
                _postState: leaves[leaves.length - 1],
                _postStateProof: postProof
            });
        }

        // Validate the preimage part
        {
            bytes32 finalDigest = _setStatusByte(keccak256(data), 2);
            bytes32 expectedPart;
            assembly {
                switch lt(_partOffset, 0x08)
                case true {
                    mstore(0x00, shl(192, mload(data)))
                    mstore(0x08, mload(add(data, 0x20)))
                    expectedPart := mload(_partOffset)
                }
                default {
                    // Clean the word after `data` so we don't get any dirty bits.
                    mstore(add(add(data, 0x20), mload(data)), 0x00)
                    expectedPart := mload(add(add(data, 0x20), sub(_partOffset, 0x08)))
                }
            }

            assertTrue(oracle.preimagePartOk(finalDigest, _partOffset));
            assertEq(oracle.preimageLengths(finalDigest), data.length);
            assertEq(oracle.preimageParts(finalDigest, _partOffset), expectedPart);
        }
    }

    /// @notice Tests that a valid leaf cannot be countered with the `challengeFirst` function.
    function test_challengeFirst_validCommitment_reverts() public {
        // Allocate the preimage data.
        bytes memory data = new bytes(136);
        for (uint256 i; i < data.length; i++) {
            data[i] = 0xFF;
        }

        // Initialize the proposal.
        oracle.initLPP{ value: oracle.MIN_BOND_SIZE() }(TEST_UUID, 0, uint32(data.length));

        // Add the leaves to the tree with mismatching state commitments.
        LibKeccak.StateMatrix memory stateMatrix;
        bytes32[] memory stateCommitments = _generateStateCommitments(stateMatrix, data);
        oracle.addLeavesLPP(TEST_UUID, 0, data, stateCommitments, true);

        // Construct the leaf preimage data for the blocks added.
        LibKeccak.StateMatrix memory matrix;
        IPreimageOracle.Leaf[] memory leaves = _generateLeaves(matrix, data);

        // Create a proof array with 16 elements.
        bytes32[] memory p = new bytes32[](16);
        p[0] = _hashLeaf(leaves[1]);
        for (uint256 i = 1; i < p.length; i++) {
            p[i] = oracle.zeroHashes(i);
        }

        vm.expectRevert(PostStateMatches.selector);
        oracle.challengeFirstLPP({
            _claimant: address(this),
            _uuid: TEST_UUID,
            _postState: leaves[0],
            _postStateProof: p
        });

        LPPMetaData metaData = oracle.proposalMetadata(address(this), TEST_UUID);
        assertFalse(metaData.countered());
    }

    /// @notice Tests that an invalid leaf cannot be countered with `challengeFirst` if it is not the first leaf.
    function test_challengeFirst_statesNotContiguous_reverts() public {
        // Allocate the preimage data.
        bytes memory data = new bytes(136);
        for (uint256 i; i < data.length; i++) {
            data[i] = 0xFF;
        }
        bytes memory phonyData = new bytes(136);

        // Initialize the proposal.
        oracle.initLPP{ value: oracle.MIN_BOND_SIZE() }(TEST_UUID, 0, uint32(data.length));

        // Add the leaves to the tree with mismatching state commitments.
        LibKeccak.StateMatrix memory stateMatrix;
        bytes32[] memory stateCommitments = _generateStateCommitments(stateMatrix, data);
        oracle.addLeavesLPP(TEST_UUID, 0, phonyData, stateCommitments, true);

        // Construct the leaf preimage data for the blocks added.
        LibKeccak.StateMatrix memory matrix;
        IPreimageOracle.Leaf[] memory leaves = _generateLeaves(matrix, phonyData);
        leaves[0].stateCommitment = stateCommitments[0];
        leaves[1].stateCommitment = stateCommitments[1];

        // Create a proof array with 16 elements.
        bytes32[] memory p = new bytes32[](16);
        p[0] = _hashLeaf(leaves[0]);
        for (uint256 i = 1; i < p.length; i++) {
            p[i] = oracle.zeroHashes(i);
        }

        // Should succeed since the commitment was wrong.
        vm.expectRevert(StatesNotContiguous.selector);
        oracle.challengeFirstLPP({
            _claimant: address(this),
            _uuid: TEST_UUID,
            _postState: leaves[1],
            _postStateProof: p
        });
    }

    /// @notice Tests that an invalid leaf can be countered with the `challengeFirst` function.
    function test_challengeFirst_invalidCommitment_succeeds() public {
        // Allocate the preimage data.
        bytes memory data = new bytes(136);
        for (uint256 i; i < data.length; i++) {
            data[i] = 0xFF;
        }
        bytes memory phonyData = new bytes(136);

        // Initialize the proposal.
        oracle.initLPP{ value: oracle.MIN_BOND_SIZE() }(TEST_UUID, 0, uint32(data.length));

        // Add the leaves to the tree with mismatching state commitments.
        LibKeccak.StateMatrix memory stateMatrix;
        bytes32[] memory stateCommitments = _generateStateCommitments(stateMatrix, data);
        oracle.addLeavesLPP(TEST_UUID, 0, phonyData, stateCommitments, true);

        // Construct the leaf preimage data for the blocks added.
        LibKeccak.StateMatrix memory matrix;
        IPreimageOracle.Leaf[] memory leaves = _generateLeaves(matrix, phonyData);
        leaves[0].stateCommitment = stateCommitments[0];
        leaves[1].stateCommitment = stateCommitments[1];

        // Create a proof array with 16 elements.
        bytes32[] memory p = new bytes32[](16);
        p[0] = _hashLeaf(leaves[1]);
        for (uint256 i = 1; i < p.length; i++) {
            p[i] = oracle.zeroHashes(i);
        }

        // Should succeed since the commitment was wrong.
        uint256 balanceBefore = address(this).balance;
        oracle.challengeFirstLPP({
            _claimant: address(this),
            _uuid: TEST_UUID,
            _postState: leaves[0],
            _postStateProof: p
        });
        assertEq(address(this).balance, balanceBefore + oracle.MIN_BOND_SIZE());
        assertEq(oracle.proposalBonds(address(this), TEST_UUID), 0);

        LPPMetaData metaData = oracle.proposalMetadata(address(this), TEST_UUID);
        assertTrue(metaData.countered());
    }

    /// @notice Tests that challenging the first divergence in a large preimage proposal at an arbitrary location
    ///         in the leaf values always succeeds.
    /// forge-config: ciheavy.fuzz.runs = 512
    function testFuzz_challenge_arbitraryLocation_succeeds(uint256 _lastCorrectLeafIdx, uint256 _numBlocks) public {
        _numBlocks = bound(_numBlocks, 1, 2 ** 8);
        _lastCorrectLeafIdx = bound(_lastCorrectLeafIdx, 0, _numBlocks - 1);

        // Allocate the preimage data.
        bytes memory data = new bytes(136 * _numBlocks);

        // Initialize the proposal.
        oracle.initLPP{ value: oracle.MIN_BOND_SIZE() }(TEST_UUID, 0, uint32(data.length));

        // Add the leaves to the tree with corrupted state commitments.
        LibKeccak.StateMatrix memory matrixA;
        bytes32[] memory stateCommitments = _generateStateCommitments(matrixA, data);
        for (uint256 i = _lastCorrectLeafIdx + 1; i < stateCommitments.length; i++) {
            stateCommitments[i] = 0;
        }
        oracle.addLeavesLPP(TEST_UUID, 0, data, stateCommitments, true);

        // Construct the leaf preimage data for the blocks added and corrupt the state commitments.
        LibKeccak.StateMatrix memory matrixB;
        IPreimageOracle.Leaf[] memory leaves = _generateLeaves(matrixB, data);
        for (uint256 i = _lastCorrectLeafIdx + 1; i < leaves.length; i++) {
            leaves[i].stateCommitment = 0;
        }

        // Avoid stack too deep
        uint256 agreedLeafIdx = _lastCorrectLeafIdx;
        uint256 disputedLeafIdx = agreedLeafIdx + 1;

        // Fetch the merkle proofs for the pre/post state leaves in the proposal tree.
        bytes32 canonicalRoot = oracle.getTreeRootLPP(address(this), TEST_UUID);
        (bytes32 rootA, bytes32[] memory preProof) = _generateProof(agreedLeafIdx, leaves);
        assertEq(rootA, canonicalRoot);
        (bytes32 rootB, bytes32[] memory postProof) = _generateProof(disputedLeafIdx, leaves);
        assertEq(rootB, canonicalRoot);

        LibKeccak.StateMatrix memory preMatrix = _stateMatrixAtBlockIndex(data, disputedLeafIdx);
        oracle.challengeLPP({
            _claimant: address(this),
            _uuid: TEST_UUID,
            _stateMatrix: preMatrix,
            _preState: leaves[agreedLeafIdx],
            _preStateProof: preProof,
            _postState: leaves[disputedLeafIdx],
            _postStateProof: postProof
        });

        LPPMetaData metaData = oracle.proposalMetadata(address(this), TEST_UUID);
        assertTrue(metaData.countered());
    }

    /// @notice Tests that challenging the a divergence in a large preimage proposal at the first leaf always succeeds.
    /// forge-config: ciheavy.fuzz.runs = 1024
    function testFuzz_challengeFirst_succeeds(uint256 _numBlocks) public {
        _numBlocks = bound(_numBlocks, 1, 2 ** 8);

        // Allocate the preimage data.
        bytes memory data = new bytes(136 * _numBlocks);

        // Initialize the proposal.
        oracle.initLPP{ value: oracle.MIN_BOND_SIZE() }(TEST_UUID, 0, uint32(data.length));

        // Add the leaves to the tree with corrupted state commitments.
        bytes32[] memory stateCommitments = new bytes32[](_numBlocks + 1);
        for (uint256 i = 0; i < stateCommitments.length; i++) {
            stateCommitments[i] = 0;
        }
        oracle.addLeavesLPP(TEST_UUID, 0, data, stateCommitments, true);

        // Construct the leaf preimage data for the blocks added and corrupt the state commitments.
        LibKeccak.StateMatrix memory matrixB;
        IPreimageOracle.Leaf[] memory leaves = _generateLeaves(matrixB, data);
        for (uint256 i = 0; i < leaves.length; i++) {
            leaves[i].stateCommitment = 0;
        }

        // Fetch the merkle proofs for the pre/post state leaves in the proposal tree.
        bytes32 canonicalRoot = oracle.getTreeRootLPP(address(this), TEST_UUID);
        (bytes32 rootA, bytes32[] memory postProof) = _generateProof(0, leaves);
        assertEq(rootA, canonicalRoot);

        oracle.challengeFirstLPP({
            _claimant: address(this),
            _uuid: TEST_UUID,
            _postState: leaves[0],
            _postStateProof: postProof
        });

        LPPMetaData metaData = oracle.proposalMetadata(address(this), TEST_UUID);
        assertTrue(metaData.countered());
    }

    /// @notice Tests that a valid leaf cannot be countered with the `challenge` function in the middle of the tree.
    function test_challenge_validCommitment_reverts() public {
        // Allocate the preimage data.
        bytes memory data = new bytes(136);
        for (uint256 i; i < data.length; i++) {
            data[i] = 0xFF;
        }

        // Initialize the proposal.
        oracle.initLPP{ value: oracle.MIN_BOND_SIZE() }(TEST_UUID, 0, uint32(data.length));

        // Add the leaves to the tree with mismatching state commitments.
        LibKeccak.StateMatrix memory stateMatrix;
        bytes32[] memory stateCommitments = _generateStateCommitments(stateMatrix, data);
        oracle.addLeavesLPP(TEST_UUID, 0, data, stateCommitments, true);

        // Construct the leaf preimage data for the blocks added.
        LibKeccak.StateMatrix memory matrix;
        IPreimageOracle.Leaf[] memory leaves = _generateLeaves(matrix, data);

        // Create a proof array with 16 elements.
        bytes32[] memory preProof = new bytes32[](16);
        preProof[0] = _hashLeaf(leaves[1]);
        bytes32[] memory postProof = new bytes32[](16);
        postProof[0] = _hashLeaf(leaves[0]);
        for (uint256 i = 1; i < preProof.length; i++) {
            bytes32 zeroHash = oracle.zeroHashes(i);
            preProof[i] = zeroHash;
            postProof[i] = zeroHash;
        }

        LibKeccak.StateMatrix memory preMatrix = _stateMatrixAtBlockIndex(data, 1);

        vm.expectRevert(PostStateMatches.selector);
        oracle.challengeLPP({
            _claimant: address(this),
            _uuid: TEST_UUID,
            _stateMatrix: preMatrix,
            _preState: leaves[0],
            _preStateProof: preProof,
            _postState: leaves[1],
            _postStateProof: postProof
        });

        LPPMetaData metaData = oracle.proposalMetadata(address(this), TEST_UUID);
        assertFalse(metaData.countered());
    }

    /// @notice Tests that an invalid leaf can not be countered with non-contiguous states.
    function test_challenge_statesNotContiguous_reverts() public {
        // Allocate the preimage data.
        bytes memory data = new bytes(136 * 2);
        for (uint256 i; i < data.length; i++) {
            data[i] = 0xFF;
        }
        bytes memory phonyData = new bytes(136 * 2);
        for (uint256 i = 0; i < phonyData.length / 2; i++) {
            data[i] = 0xFF;
        }

        // Initialize the proposal.
        oracle.initLPP{ value: oracle.MIN_BOND_SIZE() }(TEST_UUID, 0, uint32(data.length));

        // Add the leaves to the tree with mismatching state commitments.
        LibKeccak.StateMatrix memory stateMatrix;
        bytes32[] memory stateCommitments = _generateStateCommitments(stateMatrix, data);
        oracle.addLeavesLPP(TEST_UUID, 0, phonyData, stateCommitments, true);

        // Construct the leaf preimage data for the blocks added.
        LibKeccak.StateMatrix memory matrix;
        IPreimageOracle.Leaf[] memory leaves = _generateLeaves(matrix, phonyData);
        leaves[0].stateCommitment = stateCommitments[0];
        leaves[1].stateCommitment = stateCommitments[1];
        leaves[2].stateCommitment = stateCommitments[2];

        // Create a proof array with 16 elements.
        bytes32[] memory preProof = new bytes32[](16);
        preProof[0] = _hashLeaf(leaves[1]);
        preProof[1] = keccak256(abi.encode(_hashLeaf(leaves[2]), bytes32(0)));
        bytes32[] memory postProof = new bytes32[](16);
        postProof[0] = _hashLeaf(leaves[0]);
        postProof[1] = keccak256(abi.encode(_hashLeaf(leaves[2]), bytes32(0)));
        for (uint256 i = 2; i < preProof.length; i++) {
            bytes32 zeroHash = oracle.zeroHashes(i);
            preProof[i] = zeroHash;
            postProof[i] = zeroHash;
        }

        LibKeccak.StateMatrix memory preMatrix = _stateMatrixAtBlockIndex(data, 2);

        vm.expectRevert(StatesNotContiguous.selector);
        oracle.challengeLPP({
            _claimant: address(this),
            _uuid: TEST_UUID,
            _stateMatrix: preMatrix,
            _preState: leaves[1],
            _preStateProof: postProof,
            _postState: leaves[0],
            _postStateProof: preProof
        });
    }

    /// @notice Tests that an invalid leaf can not be countered with an incorrect prestate matrix reveal.
    function test_challenge_invalidPreimage_reverts() public {
        // Allocate the preimage data.
        bytes memory data = new bytes(136 * 2);
        for (uint256 i; i < data.length; i++) {
            data[i] = 0xFF;
        }
        bytes memory phonyData = new bytes(136 * 2);
        for (uint256 i = 0; i < phonyData.length / 2; i++) {
            data[i] = 0xFF;
        }

        // Initialize the proposal.
        oracle.initLPP{ value: oracle.MIN_BOND_SIZE() }(TEST_UUID, 0, uint32(data.length));

        // Add the leaves to the tree with mismatching state commitments.
        LibKeccak.StateMatrix memory stateMatrix;
        bytes32[] memory stateCommitments = _generateStateCommitments(stateMatrix, data);
        oracle.addLeavesLPP(TEST_UUID, 0, phonyData, stateCommitments, true);

        // Construct the leaf preimage data for the blocks added.
        LibKeccak.StateMatrix memory matrix;
        IPreimageOracle.Leaf[] memory leaves = _generateLeaves(matrix, phonyData);
        leaves[0].stateCommitment = stateCommitments[0];
        leaves[1].stateCommitment = stateCommitments[1];
        leaves[2].stateCommitment = stateCommitments[2];

        // Create a proof array with 16 elements.
        bytes32[] memory preProof = new bytes32[](16);
        preProof[0] = _hashLeaf(leaves[1]);
        preProof[1] = keccak256(abi.encode(_hashLeaf(leaves[2]), bytes32(0)));
        bytes32[] memory postProof = new bytes32[](16);
        postProof[0] = _hashLeaf(leaves[0]);
        postProof[1] = keccak256(abi.encode(_hashLeaf(leaves[2]), bytes32(0)));
        for (uint256 i = 2; i < preProof.length; i++) {
            bytes32 zeroHash = oracle.zeroHashes(i);
            preProof[i] = zeroHash;
            postProof[i] = zeroHash;
        }

        LibKeccak.StateMatrix memory preMatrix = _stateMatrixAtBlockIndex(data, 2);

        vm.expectRevert(InvalidPreimage.selector);
        oracle.challengeLPP({
            _claimant: address(this),
            _uuid: TEST_UUID,
            _stateMatrix: preMatrix,
            _preState: leaves[0],
            _preStateProof: preProof,
            _postState: leaves[1],
            _postStateProof: postProof
        });
    }

    /// @notice Tests that an invalid leaf can be countered with the `challenge` function in the middle of the tree.
    function test_challenge_invalidCommitment_succeeds() public {
        // Allocate the preimage data.
        bytes memory data = new bytes(136 * 2);
        for (uint256 i; i < data.length; i++) {
            data[i] = 0xFF;
        }
        bytes memory phonyData = new bytes(136 * 2);
        for (uint256 i = 0; i < phonyData.length / 2; i++) {
            data[i] = 0xFF;
        }

        // Initialize the proposal.
        oracle.initLPP{ value: oracle.MIN_BOND_SIZE() }(TEST_UUID, 0, uint32(data.length));

        // Add the leaves to the tree with mismatching state commitments.
        LibKeccak.StateMatrix memory stateMatrix;
        bytes32[] memory stateCommitments = _generateStateCommitments(stateMatrix, data);
        oracle.addLeavesLPP(TEST_UUID, 0, phonyData, stateCommitments, true);

        // Construct the leaf preimage data for the blocks added.
        LibKeccak.StateMatrix memory matrix;
        IPreimageOracle.Leaf[] memory leaves = _generateLeaves(matrix, phonyData);
        leaves[0].stateCommitment = stateCommitments[0];
        leaves[1].stateCommitment = stateCommitments[1];
        leaves[2].stateCommitment = stateCommitments[2];

        // Create a proof array with 16 elements.
        bytes32[] memory preProof = new bytes32[](16);
        preProof[0] = _hashLeaf(leaves[1]);
        preProof[1] = keccak256(abi.encode(_hashLeaf(leaves[2]), bytes32(0)));
        bytes32[] memory postProof = new bytes32[](16);
        postProof[0] = _hashLeaf(leaves[0]);
        postProof[1] = keccak256(abi.encode(_hashLeaf(leaves[2]), bytes32(0)));
        for (uint256 i = 2; i < preProof.length; i++) {
            bytes32 zeroHash = oracle.zeroHashes(i);
            preProof[i] = zeroHash;
            postProof[i] = zeroHash;
        }

        uint256 balanceBefore = address(this).balance;
        LibKeccak.StateMatrix memory preMatrix = _stateMatrixAtBlockIndex(data, 1);
        oracle.challengeLPP({
            _claimant: address(this),
            _uuid: TEST_UUID,
            _stateMatrix: preMatrix,
            _preState: leaves[0],
            _preStateProof: preProof,
            _postState: leaves[1],
            _postStateProof: postProof
        });
        assertEq(address(this).balance, balanceBefore + oracle.MIN_BOND_SIZE());
        assertEq(oracle.proposalBonds(address(this), TEST_UUID), 0);

        LPPMetaData metaData = oracle.proposalMetadata(address(this), TEST_UUID);
        assertTrue(metaData.countered());
    }

    /// @notice Hashes leaf data for the preimage proposals tree
    function _hashLeaf(IPreimageOracle.Leaf memory _leaf) internal pure returns (bytes32 leaf_) {
        leaf_ = keccak256(abi.encodePacked(_leaf.input, _leaf.index, _leaf.stateCommitment));
    }

    /// @notice Helper to construct the keccak merkle tree's leaves from a given input `_data`.
    function _generateLeaves(
        LibKeccak.StateMatrix memory _stateMatrix,
        bytes memory _data
    )
        internal
        pure
        returns (IPreimageOracle.Leaf[] memory leaves_)
    {
        bytes memory data = LibKeccak.padMemory(_data);
        uint256 numCommitments = data.length / LibKeccak.BLOCK_SIZE_BYTES;

        leaves_ = new IPreimageOracle.Leaf[](numCommitments);
        for (uint256 i = 0; i < numCommitments; i++) {
            bytes memory blockSlice = Bytes.slice(data, i * LibKeccak.BLOCK_SIZE_BYTES, LibKeccak.BLOCK_SIZE_BYTES);
            LibKeccak.absorb(_stateMatrix, blockSlice);
            LibKeccak.permutation(_stateMatrix);

            leaves_[i] = IPreimageOracle.Leaf({
                input: blockSlice,
                index: uint32(i),
                stateCommitment: keccak256(abi.encode(_stateMatrix))
            });
        }
    }

    /// @notice Helper to get the keccak state matrix before applying the block at `_blockIndex` within `_data`.
    function _stateMatrixAtBlockIndex(
        bytes memory _data,
        uint256 _blockIndex
    )
        internal
        pure
        returns (LibKeccak.StateMatrix memory matrix_)
    {
        bytes memory data = LibKeccak.padMemory(_data);

        for (uint256 i = 0; i < _blockIndex; i++) {
            bytes memory blockSlice = Bytes.slice(data, i * LibKeccak.BLOCK_SIZE_BYTES, LibKeccak.BLOCK_SIZE_BYTES);
            LibKeccak.absorb(matrix_, blockSlice);
            LibKeccak.permutation(matrix_);
        }
    }

    /// @notice Helper to construct the keccak state commitments for each block processed in the input `_data`.
    function _generateStateCommitments(
        LibKeccak.StateMatrix memory _stateMatrix,
        bytes memory _data
    )
        internal
        pure
        returns (bytes32[] memory stateCommitments_)
    {
        bytes memory data = LibKeccak.padMemory(_data);
        uint256 numCommitments = data.length / LibKeccak.BLOCK_SIZE_BYTES;

        stateCommitments_ = new bytes32[](numCommitments);
        for (uint256 i = 0; i < numCommitments; i++) {
            bytes memory blockSlice = Bytes.slice(data, i * LibKeccak.BLOCK_SIZE_BYTES, LibKeccak.BLOCK_SIZE_BYTES);
            LibKeccak.absorb(_stateMatrix, blockSlice);
            LibKeccak.permutation(_stateMatrix);

            stateCommitments_[i] = keccak256(abi.encode(_stateMatrix));
        }
    }

    /// @notice Calls out to the `go-ffi` tool to generate a merkle proof for the leaf at `_leafIdx` in a merkle tree
    ///         constructed with `_leaves`.
    function _generateProof(
        uint256 _leafIdx,
        IPreimageOracle.Leaf[] memory _leaves
    )
        internal
        returns (bytes32 root_, bytes32[] memory proof_)
    {
        bytes32[] memory leaves = new bytes32[](_leaves.length);
        for (uint256 i = 0; i < _leaves.length; i++) {
            leaves[i] = _hashLeaf(_leaves[i]);
        }

        string[] memory commands = new string[](5);
        commands[0] = "scripts/go-ffi/go-ffi";
        commands[1] = "merkle";
        commands[2] = "gen_proof";
        commands[3] = vm.toString(abi.encodePacked(leaves));
        commands[4] = vm.toString(_leafIdx);
        (root_, proof_) = abi.decode(Process.run(commands), (bytes32, bytes32[]));
    }

    fallback() external payable { }

    receive() external payable { }
}

/// @notice Sets the status byte of a hash.
function _setStatusByte(bytes32 _hash, uint8 _status) pure returns (bytes32 out_) {
    assembly {
        out_ := or(and(not(shl(248, 0xFF)), _hash), shl(248, _status))
    }
}

/// @notice Computes a precompile key for a given precompile address and input.
function precompilePreimageKey(address _precompile, uint64 _gas, bytes memory _input) pure returns (bytes32 key_) {
    bytes memory p = abi.encodePacked(_precompile, _gas, _input);
    uint256 sz = 20 + 8 + _input.length;
    assembly {
        let h := keccak256(add(0x20, p), sz)
        // Mask out prefix byte, replace with type 6 byte
        key_ := or(and(h, not(shl(248, 0xFF))), shl(248, 6))
    }
}
