// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// @title  SafeSend
/// @notice Sends ETH to a recipient account without triggering any code.
contract SafeSend {
    /// @param _recipient Account to send ETH to.
    constructor(address payable _recipient) payable {
        selfdestruct(_recipient);
    }
}
