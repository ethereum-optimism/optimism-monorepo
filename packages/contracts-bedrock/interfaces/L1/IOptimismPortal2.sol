// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { Types } from "src/libraries/Types.sol";
import { GameType, Timestamp } from "src/dispute/lib/LibUDT.sol";
import { IDisputeGame } from "interfaces/dispute/IDisputeGame.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";

interface IOptimismPortal2 {
    error AlreadyFinalized();
    error BadTarget();
    error Blacklisted();
    error CallPaused();
    error ContentLengthMismatch();
    error EmptyItem();
    error GasEstimation();
    error InvalidDataRemainder();
    error InvalidDisputeGame();
    error InvalidGameType();
    error InvalidHeader();
    error InvalidMerkleProof();
    error InvalidProof();
    error LargeCalldata();
    error NonReentrant();
    error OutOfGas();
    error ProposalNotValidated();
    error SmallGasLimit();
    error Unauthorized();
    error UnexpectedList();
    error UnexpectedString();
    error Unproven();
    error LegacyGame();

    event DisputeGameBlacklisted(IDisputeGame indexed disputeGame);
    event Initialized(uint8 version);
    event RespectedGameTypeSet(GameType indexed newGameType, Timestamp indexed updatedAt);
    event TransactionDeposited(address indexed from, address indexed to, uint256 indexed version, bytes opaqueData);
    event WithdrawalFinalized(bytes32 indexed withdrawalHash, bool success);
    event WithdrawalProven(bytes32 indexed withdrawalHash, address indexed from, address indexed to);
    event WithdrawalProvenExtension1(bytes32 indexed withdrawalHash, address indexed proofSubmitter);

    receive() external payable;

    function blacklistDisputeGame(IDisputeGame _disputeGame) external;
    function checkWithdrawal(bytes32 _withdrawalHash, address _proofSubmitter) external view;
    function depositTransaction(
        address _to,
        uint256 _value,
        uint64 _gasLimit,
        bool _isCreation,
        bytes memory _data
    )
        external
        payable;
    function disputeGameBlacklist(IDisputeGame) external view returns (bool);
    function disputeGameFactory() external view returns (IDisputeGameFactory);
    function disputeGameFinalityDelaySeconds() external view returns (uint256);
    function donateETH() external payable;
    function finalizeWithdrawalTransaction(Types.WithdrawalTransaction memory _tx) external;
    function finalizeWithdrawalTransactionExternalProof(
        Types.WithdrawalTransaction memory _tx,
        address _proofSubmitter
    )
        external;
    function finalizedWithdrawals(bytes32) external view returns (bool);
    function guardian() external view returns (address);
    function initialize(
        IDisputeGameFactory _disputeGameFactory,
        ISystemConfig _systemConfig,
        ISuperchainConfig _superchainConfig,
        GameType _initialRespectedGameType
    )
        external;
    function l2Sender() external view returns (address);
    function minimumGasLimit(uint64 _byteCount) external pure returns (uint64);
    function numProofSubmitters(bytes32 _withdrawalHash) external view returns (uint256);
    function params() external view returns (uint128 prevBaseFee, uint64 prevBoughtGas, uint64 prevBlockNum); // nosemgrep
    function paused() external view returns (bool);
    function proofMaturityDelaySeconds() external view returns (uint256);
    function proofSubmitters(bytes32, uint256) external view returns (address);
    function proveWithdrawalTransaction(
        Types.WithdrawalTransaction memory _tx,
        uint256 _disputeGameIndex,
        Types.OutputRootProof memory _outputRootProof,
        bytes[] memory _withdrawalProof
    )
        external;
    function provenWithdrawals(
        bytes32,
        address
    )
        external
        view
        returns (IDisputeGame disputeGameProxy, uint64 timestamp); // nosemgrep
    function respectedGameType() external view returns (GameType);
    function respectedGameTypeUpdatedAt() external view returns (uint64);
    function setRespectedGameType(GameType _gameType) external;
    function superchainConfig() external view returns (ISuperchainConfig);
    function systemConfig() external view returns (ISystemConfig);
    function version() external pure returns (string memory);

    function __constructor__(uint256 _proofMaturityDelaySeconds, uint256 _disputeGameFinalityDelaySeconds) external;
}
