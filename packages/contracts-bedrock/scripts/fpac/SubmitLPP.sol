// SPDX-License-Identifier: MIT
 pragma solidity 0.8.15;

 import { PreimageOracle } from "src/cannon/PreimageOracle.sol";
 import { Script } from "forge-std/Script.sol";
 import { StdAssertions } from "forge-std/StdAssertions.sol";
 import "src/cannon/libraries/CannonTypes.sol";

 contract SubmitLPP is Script, StdAssertions {
     /// @notice Test UUID
     uint256 private constant TEST_UUID = uint256(keccak256("TEST_UUID2"));
     /// @notice Number of bytes to submit to the preimage oracle.
     uint256 private constant BYTES_TO_SUBMIT = 4_012_000;
     /// @notice Chunk size to submit to the preimage oracle.
     uint256 private constant CHUNK_SIZE = 500 * 136;

     PreimageOracle private oracle;

     function post(address _po) external {
         // Bootstrap
         oracle = PreimageOracle(_po);

         // Allocate chunk - worst case w/ all bits set.
         bytes memory chunk = new bytes(CHUNK_SIZE);
         for (uint256 i; i < chunk.length; i++) {
             chunk[i] = 0xFF;
         }

         // Mock state commitments. Worst case w/ all bits set.
         bytes32[] memory mockStateCommitments = new bytes32[](CHUNK_SIZE / 136);
         bytes32[] memory mockStateCommitmentsLast = new bytes32[](CHUNK_SIZE / 136 + 1);
         for (uint256 i; i < mockStateCommitments.length; i++) {
             mockStateCommitments[i] = bytes32(type(uint256).max);
             mockStateCommitmentsLast[i] = bytes32(type(uint256).max);
         }
         // Assign last mock state commitment to all bits set.
         mockStateCommitmentsLast[mockStateCommitmentsLast.length - 1] = bytes32(type(uint256).max);

         uint256 bondSize = oracle.MIN_BOND_SIZE();
         vm.broadcast();
         oracle.initLPP{ value: bondSize }({ _uuid: TEST_UUID, _partOffset: 0, _claimedSize: uint32(BYTES_TO_SUBMIT) });

         // Submit LPP in 500 * 136 byte chunks.
         for (uint256 i = 0; i < BYTES_TO_SUBMIT; i += CHUNK_SIZE) {
             bool finalize = i + CHUNK_SIZE >= BYTES_TO_SUBMIT;

             vm.broadcast();
             oracle.addLeavesLPP({
                 _uuid: TEST_UUID,
                 _inputStartBlock: i / 136,
                 _input: chunk,
                 _stateCommitments: finalize ? mockStateCommitmentsLast : mockStateCommitments,
                 _finalize: finalize
             });
         }
         // Assert that all bytes were submitted.
         LPPMetaData metaData = oracle.proposalMetadata(msg.sender, TEST_UUID);
         assertEq(metaData.bytesProcessed(), BYTES_TO_SUBMIT);
         assertEq(metaData.blocksProcessed(), (BYTES_TO_SUBMIT / 136) + 1);
     }
 }
