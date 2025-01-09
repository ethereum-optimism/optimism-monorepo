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

/// @dev Can be etched in to any address to test making a delegatecall from that address.
contract DelegateCaller {
    function dcForward(address _target, bytes calldata _data) external {
        (bool success,) = _target.delegatecall(_data);
        require(success, "DelegateCaller: Delegatecall failed");
    }
}
