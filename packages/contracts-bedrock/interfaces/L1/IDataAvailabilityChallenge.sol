// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

enum ChallengeStatus {
    Uninitialized,
    Active,
    Resolved,
    Expired
}

enum CommitmentType {
    Keccak256
}

struct Challenge {
    address challenger;
    uint256 lockedBond;
    uint256 startBlock;
    uint256 resolvedBlock;
}

interface IDataAvailabilityChallenge {
    error BondTooLow(uint256 balance, uint256 required);
    error ChallengeExists();
    error ChallengeNotActive();
    error ChallengeNotExpired();
    error ChallengeWindowNotOpen();
    error InvalidCommitmentLength(uint8 commitmentType, uint256 expectedLength, uint256 actualLength);
    error InvalidInputData(bytes providedDataCommitment, bytes expectedCommitment);
    error InvalidResolverRefundPercentage(uint256 invalidResolverRefundPercentage);
    error UnknownCommitmentType(uint8 commitmentType);
    error WithdrawalFailed();

    event BalanceChanged(address account, uint256 balance);
    event ChallengeStatusChanged(
        uint256 indexed challengedBlockNumber, bytes challengedCommitment, ChallengeStatus status
    );
    event Initialized(uint8 version);
    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);
    event RequiredBondSizeChanged(uint256 challengeWindow);
    event ResolverRefundPercentageChanged(uint256 resolverRefundPercentage);

    receive() external payable;

    function balances(address) external view returns (uint256);
    function bondSize() external view returns (uint256);
    function challenge(uint256 _challengedBlockNumber, bytes memory _challengedCommitment) external payable;
    function challengeWindow() external view returns (uint256);
    function deposit() external payable;
    function fixedResolutionCost() external view returns (uint256);
    function getChallenge(
        uint256 _challengedBlockNumber,
        bytes memory _challengedCommitment
    )
        external
        view
        returns (Challenge memory);
    function getChallengeStatus(
        uint256 _challengedBlockNumber,
        bytes memory _challengedCommitment
    )
        external
        view
        returns (ChallengeStatus);
    function initialize(
        address _owner,
        uint256 _challengeWindow,
        uint256 _resolveWindow,
        uint256 _bondSize,
        uint256 _resolverRefundPercentage
    )
        external;
    function owner() external view returns (address);
    function renounceOwnership() external;
    function resolve(
        uint256 _challengedBlockNumber,
        bytes memory _challengedCommitment,
        bytes memory _resolveData
    )
        external;
    function resolveWindow() external view returns (uint256);
    function resolverRefundPercentage() external view returns (uint256);
    function setBondSize(uint256 _bondSize) external;
    function setResolverRefundPercentage(uint256 _resolverRefundPercentage) external;
    function transferOwnership(address newOwner) external; // nosemgrep
    function unlockBond(uint256 _challengedBlockNumber, bytes memory _challengedCommitment) external;
    function validateCommitment(bytes memory _commitment) external pure;
    function variableResolutionCost() external view returns (uint256);
    function variableResolutionCostPrecision() external view returns (uint256);
    function version() external view returns (string memory);
    function withdraw() external;

    function __constructor__() external;
}
