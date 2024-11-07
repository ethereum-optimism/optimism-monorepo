// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { Types } from "src/libraries/Types.sol";
import { ISystemConfig } from "src/L1/interfaces/ISystemConfig.sol";
import { ISuperchainConfig } from "src/L1/interfaces/ISuperchainConfig.sol";
import { IL2OutputOracle } from "src/L1/interfaces/IL2OutputOracle.sol";

interface IOptimismPortal {
    error BadTarget();
    error CallPaused();
    error ContentLengthMismatch();
    error EmptyItem();
    error GasEstimation();
    error InvalidDataRemainder();
    error InvalidHeader();
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

    event Initialized(uint8 version);
    event TransactionDeposited(address indexed from, address indexed to, uint256 indexed version, bytes opaqueData);
    event WithdrawalFinalized(bytes32 indexed withdrawalHash, bool success);
    event WithdrawalProven(bytes32 indexed withdrawalHash, address indexed from, address indexed to);

    receive() external payable;

    function balance() external view returns (uint256);
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
    function donateETH() external payable;
    function finalizeWithdrawalTransaction(Types.WithdrawalTransaction memory _tx) external;
    function finalizedWithdrawals(bytes32) external view returns (bool);
    function guardian() external view returns (address);
    function initialize(
        IL2OutputOracle _l2Oracle,
        ISystemConfig _systemConfig,
        ISuperchainConfig _superchainConfig
    )
        external;
    function isOutputFinalized(uint256 _l2OutputIndex) external view returns (bool);
    function l2Oracle() external view returns (IL2OutputOracle);
    function l2Sender() external view returns (address);
    function minimumGasLimit(uint64 _byteCount) external pure returns (uint64);
    function params() external view returns (uint128 prevBaseFee, uint64 prevBoughtGas, uint64 prevBlockNum); // nosemgrep
    function paused() external view returns (bool paused_);
    function proveWithdrawalTransaction(
        Types.WithdrawalTransaction memory _tx,
        uint256 _l2OutputIndex,
        Types.OutputRootProof memory _outputRootProof,
        bytes[] memory _withdrawalProof
    )
        external;
    function provenWithdrawals(bytes32)
        external
        view
        returns (bytes32 outputRoot, uint128 timestamp, uint128 l2OutputIndex); // nosemgrep
    function setGasPayingToken(address _token, uint8 _decimals, bytes32 _name, bytes32 _symbol) external;
    function superchainConfig() external view returns (ISuperchainConfig);
    function systemConfig() external view returns (ISystemConfig);
    function version() external pure returns (string memory);

    function __constructor__() external;
}
