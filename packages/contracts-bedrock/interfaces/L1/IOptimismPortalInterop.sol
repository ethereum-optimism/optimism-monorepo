// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { Types } from "src/libraries/Types.sol";
import { GameType, Timestamp } from "src/dispute/lib/LibUDT.sol";
import { IDisputeGame } from "interfaces/dispute/IDisputeGame.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { IAnchorStateRegistry } from "interfaces/dispute/IAnchorStateRegistry.sol";
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { ConfigType } from "interfaces/L2/IL1BlockInterop.sol";

interface IOptimismPortalInterop {
    error AlreadyFinalized();
    error BadTarget();
    error CallPaused();
    error ContentLengthMismatch();
    error EmptyItem();
    error GasEstimation();
    error InvalidDataRemainder();
    error InvalidHeader();
    error InvalidMerkleProof();
    error InvalidProof();
    error LargeCalldata();
    error NoValue();
    error NonReentrant();
    error OnlyCustomGasToken();
    error OutOfGas();
    error SmallGasLimit();
    error TransferFailed();
    error Unauthorized();
    error UnexpectedList();
    error UnexpectedString();
    error Unproven();
    error GameInvalid(string reason);

    event Initialized(uint8 version);
    event TransactionDeposited(address indexed from, address indexed to, uint256 indexed version, bytes opaqueData);
    event WithdrawalFinalized(bytes32 indexed withdrawalHash, bool success);
    event WithdrawalProven(bytes32 indexed withdrawalHash, address indexed from, address indexed to);
    event WithdrawalProvenExtension1(bytes32 indexed withdrawalHash, address indexed proofSubmitter);

    receive() external payable;

    function anchorStateRegistry() external view returns (IAnchorStateRegistry);
    function balance() external view returns (uint256);
    function checkWithdrawal(bytes32 _withdrawalHash, address _proofSubmitter) external view;
    function depositERC20Transaction(
        address _to,
        uint256 _mint,
        uint256 _value,
        uint64 _gasLimit,
        bool _isCreation,
        bytes memory _data
    )
        external;
    function depositTransaction(
        address _to,
        uint256 _value,
        uint64 _gasLimit,
        bool _isCreation,
        bytes memory _data
    )
        external
        payable;
    function disputeGameFactory() external view returns (IDisputeGameFactory);
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
        IAnchorStateRegistry _anchorStateRegistry
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
    function setConfig(ConfigType _type, bytes memory _value) external;
    function setGasPayingToken(address _token, uint8 _decimals, bytes32 _name, bytes32 _symbol) external;
    function superchainConfig() external view returns (ISuperchainConfig);
    function systemConfig() external view returns (ISystemConfig);
    function version() external pure returns (string memory);

    function __constructor__(uint256 _proofMaturityDelaySeconds) external;
}
