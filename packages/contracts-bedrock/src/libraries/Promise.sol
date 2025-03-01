// SPDX-License-Identifier: MIT
pragma solidity 0.8.25;

import {IL2ToL2CrossDomainMessenger} from "interfaces/L2/IL2ToL2CrossDomainMessenger.sol";
import {ICrossL2Inbox, Identifier} from "interfaces/L2/ICrossL2Inbox.sol";
import {Predeploys} from "src/libraries/Predeploys.sol";

contract PromiseCallack {
    bytes32 messageHash;

    bytes4 selector;
    address target;

    bool completed;

    constructor(bytes32 _messageHash, address _target, bytes4 _selector) {
        messageHash = _messageHash;
        selector = _selector;
        target = _target;
    }

    function continue(Identifier calldata _id, bytes calldata _message) external {
        require(!completed);

        // Validation
        require(_id.origin == Predeploys.L2_TO_L2_CROSS_DOMAIN_MESSENGER);
        ICrossL2Inbox(Predeploys.CROSS_L2_INBOX).validateMessage(_id, keccak256(_message));

        // Relayed Message
        bytes32 selector = abi.decode(_message[:32], (bytes32));
        require(selector == IL2ToL2CrossDomainMessenger.RelayedMessage.selector);

        (uint256, uint256, bytes32 _messageHash, bytes memory returnData) =
            abi.decode(_message[32:], (uint256, uint256, bytes32, bytes));

        // Same Message
        require(_messageHash == messageHash);

        // Invoke the callback with the return data
        (bool success, bytes memory ) = target.call(abi.encode(selector, returnData));
        completed = success;
    }
}

library Promise {
    event OptimismCallback(address callback)

    function then(bytes32 _messageHash, bytes4 _selector) internal returns (PromiseCallack) {
        PromiseCallack callback = new PromiseCallack(_messageHash, msg.sender, _selector);
        emit OptimismCallback(address(callback));

        return callback;
    }
}