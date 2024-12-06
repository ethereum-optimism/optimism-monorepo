// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { LibKeccak } from "@lib-keccak/LibKeccak.sol";
import { LPPMetaData } from "src/cannon/libraries/CannonTypes.sol";

interface IPreimageOracle {
    struct Leaf {
        bytes input;
        uint256 index;
        bytes32 stateCommitment;
    }

    error ActiveProposal();
    error AlreadyFinalized();
    error AlreadyInitialized();
    error BadProposal();
    error BondTransferFailed();
    error InsufficientBond();
    error InvalidInputSize();
    error InvalidPreimage();
    error InvalidProof();
    error NotEOA();
    error NotInitialized();
    error PartOffsetOOB();
    error PostStateMatches();
    error StatesNotContiguous();
    error TreeSizeOverflow();
    error WrongStartingBlock();

    function KECCAK_TREE_DEPTH() external view returns (uint256);
    function MAX_LEAF_COUNT() external view returns (uint256);
    function MIN_BOND_SIZE() external view returns (uint256);
    function PRECOMPILE_CALL_RESERVED_GAS() external view returns (uint256);
    function addLeavesLPP(
        uint256 _uuid,
        uint256 _inputStartBlock,
        bytes memory _input,
        bytes32[] memory _stateCommitments,
        bool _finalize
    )
        external;
    function challengeFirstLPP(
        address _claimant,
        uint256 _uuid,
        Leaf memory _postState,
        bytes32[] memory _postStateProof
    )
        external;
    function challengeLPP(
        address _claimant,
        uint256 _uuid,
        LibKeccak.StateMatrix memory _stateMatrix,
        Leaf memory _preState,
        bytes32[] memory _preStateProof,
        Leaf memory _postState,
        bytes32[] memory _postStateProof
    )
        external;
    function challengePeriod() external view returns (uint256 challengePeriod_);
    function getTreeRootLPP(address _owner, uint256 _uuid) external view returns (bytes32 treeRoot_);
    function initLPP(uint256 _uuid, uint32 _partOffset, uint32 _claimedSize) external payable;
    function loadBlobPreimagePart(
        uint256 _z,
        uint256 _y,
        bytes memory _commitment,
        bytes memory _proof,
        uint256 _partOffset
    )
        external;
    function loadKeccak256PreimagePart(uint256 _partOffset, bytes memory _preimage) external;
    function loadLocalData(
        uint256 _ident,
        bytes32 _localContext,
        bytes32 _word,
        uint256 _size,
        uint256 _partOffset
    )
        external
        returns (bytes32 key_);
    function loadPrecompilePreimagePart(
        uint256 _partOffset,
        address _precompile,
        uint64 _requiredGas,
        bytes memory _input
    )
        external;
    function loadSha256PreimagePart(uint256 _partOffset, bytes memory _preimage) external;
    function minProposalSize() external view returns (uint256 minProposalSize_);
    function preimageLengths(bytes32) external view returns (uint256);
    function preimagePartOk(bytes32, uint256) external view returns (bool);
    function preimageParts(bytes32, uint256) external view returns (bytes32);
    function proposalBlocks(address, uint256, uint256) external view returns (uint64);
    function proposalBlocksLen(address _claimant, uint256 _uuid) external view returns (uint256 len_);
    function proposalBonds(address, uint256) external view returns (uint256);
    function proposalBranches(address, uint256, uint256) external view returns (bytes32);
    function proposalCount() external view returns (uint256 count_);
    function proposalMetadata(address, uint256) external view returns (LPPMetaData);
    function proposalParts(address, uint256) external view returns (bytes32);
    function proposals(uint256) external view returns (address claimant, uint256 uuid); // nosemgrep:
        // sol-style-return-arg-fmt
    function readPreimage(bytes32 _key, uint256 _offset) external view returns (bytes32 dat_, uint256 datLen_);
    function squeezeLPP(
        address _claimant,
        uint256 _uuid,
        LibKeccak.StateMatrix memory _stateMatrix,
        Leaf memory _preState,
        bytes32[] memory _preStateProof,
        Leaf memory _postState,
        bytes32[] memory _postStateProof
    )
        external;
    function version() external view returns (string memory);
    function zeroHashes(uint256) external view returns (bytes32);

    function __constructor__(uint256 _minProposalSize, uint256 _challengePeriod) external;
}
