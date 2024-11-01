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
