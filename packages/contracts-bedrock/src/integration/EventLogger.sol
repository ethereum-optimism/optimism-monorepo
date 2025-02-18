// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { ICrossL2Inbox, Identifier } from "interfaces/L2/ICrossL2Inbox.sol";

import { Predeploys } from "src/libraries/Predeploys.sol";

/// @title EventLogger
/// @notice EventLogger is a util contract to emit log events, primarily for integration testing.
contract EventLogger {
    /// @notice Emits an event log with the given number of topics and the given data.
    /// @param _topics List of topics to emit. Can be 0 to 4 (incl.) entries. Also known as indexed event data.
    /// @param _data   Data to emit. As much as gas allows to emit. Also known as unindexed event data.
    function emitLog(bytes32[] calldata _topics, bytes calldata _data) external {
        assembly {
            let dataSize := _data.length
            let memDataOffset := mload(0x40) // load free memory pointer
            calldatacopy(memDataOffset, _data.offset, dataSize) // args: to, from, size
            // after the event-logging is done, the memory is not used, so no mem pointer to update/restore.

            let topicsCount := _topics.length
            let t0 := calldataload(add(_topics.offset, mul(32, 0)))
            let t1 := calldataload(add(_topics.offset, mul(32, 1)))
            let t2 := calldataload(add(_topics.offset, mul(32, 2)))
            let t3 := calldataload(add(_topics.offset, mul(32, 3)))

            // Each topic-count has its own opcode for emitting an event
            switch topicsCount
            case 0 { log0(memDataOffset, dataSize) }
            case 1 { log1(memDataOffset, dataSize, t0) }
            case 2 { log2(memDataOffset, dataSize, t0, t1) }
            case 3 { log3(memDataOffset, dataSize, t0, t1, t2) }
            case 4 { log4(memDataOffset, dataSize, t0, t1, t2, t3) }
            default { revert(0, 0) }
        }
    }

    /// @notice Validates a cross chain message using the CrossL2Inbox predeploy. This emits an executing message.
    /// @param _id      Identifier of the message.
    /// @param _msgHash Hash of the message payload to call target with.
    function validateMessage(Identifier calldata _id, bytes32 _msgHash) external {
        ICrossL2Inbox(Predeploys.CROSS_L2_INBOX).validateMessage(_id, _msgHash);
    }
}
