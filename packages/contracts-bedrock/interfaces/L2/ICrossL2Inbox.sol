// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

struct Identifier {
    address origin;
    uint256 blockNumber;
    uint256 logIndex;
    uint256 timestamp;
    uint256 chainId;
}

/// @title ICrossL2Inbox
/// @notice Interface for the CrossL2Inbox contract.
interface ICrossL2Inbox {
    error ReentrantCall();

    /// @notice Thrown when the caller is not DEPOSITOR_ACCOUNT when calling `setInteropStart()`
    error NotDepositor();

    /// @notice Thrown when attempting to set interop start when it's already set.
    error InteropStartAlreadySet();

    /// @notice Thrown when a non-written transient storage slot is attempted to be read from.
    error NotEntered();

    /// @notice Thrown when trying to execute a cross chain message with an invalid Identifier timestamp.
    error InvalidTimestamp();

    /// @notice Thrown when trying to execute a cross chain message with an invalid Identifier chain ID.
    error InvalidChainId();

    /// @notice Thrown when trying to execute a cross chain message and the target call fails.
    error TargetCallFailed();

    /// @notice Thrown when trying to execute a cross chain message on a deposit transaction.
    error NoExecutingDeposits();

    event ExecutingMessage(bytes32 indexed msgHash, Identifier id);

    function version() external view returns (string memory);

    /// @notice Returns the interop start timestamp.
    /// @return interopStart_ interop start timestamp.
    function interopStart() external view returns (uint256 interopStart_);

    /// @notice Returns the origin address of the Identifier.
    function origin() external view returns (address);

    /// @notice Returns the block number of the Identifier.
    function blockNumber() external view returns (uint256);

    /// @notice Returns the log index of the Identifier.
    function logIndex() external view returns (uint256);

    /// @notice Returns the timestamp of the Identifier.
    function timestamp() external view returns (uint256);

    /// @notice Returns the chain ID of the Identifier.
    function chainId() external view returns (uint256);

    function setInteropStart() external;

    /// @notice Executes a cross chain message on the destination chain.
    /// @param _id An Identifier pointing to the initiating message.
    /// @param _target Account that is called with _msg.
    /// @param _message The message payload, matching the initiating message.
    function executeMessage(Identifier calldata _id, address _target, bytes calldata _message) external payable;

    /// @notice Validates a cross chain message on the destination chain
    ///         and emits an ExecutingMessage event. This function is useful
    ///         for applications that understand the schema of the _message payload and want to
    ///         process it in a custom way.
    /// @param _id      Identifier of the message.
    /// @param _msgHash Hash of the message payload to call target with.
    function validateMessage(Identifier calldata _id, bytes32 _msgHash) external;
}
