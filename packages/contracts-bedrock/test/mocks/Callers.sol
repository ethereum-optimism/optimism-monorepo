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
    function dcForward(address _target, bytes memory _data) external {
        assembly {
            // Perform the delegatecall, make sure to pass all available gas.
            let success := delegatecall(gas(), _target, add(_data, 0x20), mload(_data), 0x0, 0x0)

            // Copy returndata into memory at 0x0....returndatasize. Note that this *will*
            // overwrite the calldata that we just copied into memory but that doesn't really
            // matter because we'll be returning in a second anyway.
            returndatacopy(0x0, 0x0, returndatasize())

            // Success == 0 means a revert. We'll revert too and pass the data up.
            if iszero(success) { revert(0x0, returndatasize()) }

            // Otherwise we'll just return and pass the data up.
            return(0x0, returndatasize())
        }
    }
}
