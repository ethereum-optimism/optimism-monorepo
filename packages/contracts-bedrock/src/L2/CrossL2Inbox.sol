// SPDX-License-Identifier: MIT
pragma solidity 0.8.25;

// Libraries
import { Predeploys } from "src/libraries/Predeploys.sol";

// Interfaces
import { ISemver } from "interfaces/universal/ISemver.sol";
import { IL1BlockInterop } from "interfaces/L2/IL1BlockInterop.sol";

/// @notice Thrown when trying to execute a cross chain message on a deposit transaction.
error NoExecutingDeposits();

/// @notice The struct for a pointer to a message payload in a remote (or local) chain.
struct Identifier {
    /// @dev The origin address of the message.
    address origin;
    /// @dev The block number of the message.
    uint256 blockNumber;
    /// @dev The log index of the message.
    uint256 logIndex;
    /// @dev The timestamp of the message.
    uint256 timestamp;
    /// @dev The origin chain ID of the message.
    uint256 chainId;
}

/// @custom:proxied true
/// @custom:predeploy 0x4200000000000000000000000000000000000022
/// @title CrossL2Inbox
/// @notice The CrossL2Inbox is responsible for executing a cross chain message on the destination
///         chain. It is permissionless to execute a cross chain message on behalf of any user.
contract CrossL2Inbox is ISemver {
    /// @notice Semantic version.
    /// @custom:semver 1.0.0-beta.13
    string public constant version = "1.0.0-beta.13";

    /// @notice Emitted when a cross chain message is being executed.
    /// @param msgHash Hash of message payload being executed.
    /// @param id Encoded Identifier of the message.
    event ExecutingMessage(bytes32 indexed msgHash, Identifier id);

    /// @notice Validates a cross chain message on the destination chain
    ///         and emits an ExecutingMessage event. This function is useful
    ///         for applications that understand the schema of the _message payload and want to
    ///         process it in a custom way.
    /// @param _id      Identifier of the message.
    /// @param _msgHash Hash of the message payload to call target with.
    function validateMessage(Identifier calldata _id, bytes32 _msgHash) external {
        // We need to know if this is being called on a depositTx
        if (IL1BlockInterop(Predeploys.L1_BLOCK_ATTRIBUTES).isDeposit()) revert NoExecutingDeposits();

        emit ExecutingMessage(_msgHash, _id);
    }
}
