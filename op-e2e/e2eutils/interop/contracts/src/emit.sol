// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

contract EmitEvent {
    // Define an event that logs the emitted data
    event DataEmitted(bytes indexed _data);

    // Function that takes calldata and emits the data as an event
    function emitData(bytes calldata _data) external {
        emit DataEmitted(_data);
    }
}
