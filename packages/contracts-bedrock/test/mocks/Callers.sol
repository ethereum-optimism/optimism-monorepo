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

/// @dev Useful for testing reentrancy guards
contract CallerCaller {
    event WhatHappened(bool success, bytes returndata);

    fallback() external {
        (bool success, bytes memory returndata) = msg.sender.call(msg.data);
        emit WhatHappened(success, returndata);
        assembly {
            switch success
            case 0 { revert(add(returndata, 0x20), mload(returndata)) }
            default { return(add(returndata, 0x20), mload(returndata)) }
        }
    }
}

/// @dev Used for testing the `CrossDomainMessenger`'s per-message reentrancy guard.
contract ConfigurableCaller {
    bool doRevert = true;
    address target;
    bytes payload;

    event WhatHappened(bool success, bytes returndata);

    /// @notice Call the configured target with the configured payload OR revert.
    function call() external {
        if (doRevert) {
            revert("ConfigurableCaller: revert");
        } else {
            (bool success, bytes memory returndata) = address(target).call(payload);
            emit WhatHappened(success, returndata);
            assembly {
                switch success
                case 0 { revert(add(returndata, 0x20), mload(returndata)) }
                default { return(add(returndata, 0x20), mload(returndata)) }
            }
        }
    }

    /// @notice Set whether or not to have `call` revert.
    function setDoRevert(bool _doRevert) external {
        doRevert = _doRevert;
    }

    /// @notice Set the target for the call made in `call`.
    function setTarget(address _target) external {
        target = _target;
    }

    /// @notice Set the payload for the call made in `call`.
    function setPayload(bytes calldata _payload) external {
        payload = _payload;
    }

    /// @notice Fallback function that reverts if `doRevert` is true.
    ///        Otherwise, it does nothing.
    fallback() external {
        if (doRevert) {
            revert("ConfigurableCaller: revert");
        }
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
