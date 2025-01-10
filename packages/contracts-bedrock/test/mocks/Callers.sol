// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract CallRecorder {
    struct CallInfo {
        address sender;
        bytes data;
        uint256 gas;
        uint256 value;
    }

    CallInfo public lastCall;

    function record() public payable {
        lastCall.sender = msg.sender;
        lastCall.data = msg.data;
        lastCall.gas = gasleft();
        lastCall.value = msg.value;
    }
}

/// @dev Any call will revert
contract Reverter {
    function doRevert() public pure {
        revert("Reverter: Reverter reverted");
    }

    fallback() external {
        revert();
    }
}
