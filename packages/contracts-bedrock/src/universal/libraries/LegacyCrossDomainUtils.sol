// SPDX-License-Identifier: MIT
pragma solidity ^0.8.9;

// Importing from the legacy contracts package causes issues with the build of the contract bindings
// so we just copy the library here from
// /packages/contracts/contracts/libraries/bridge/Lib_CrossDomainUtils.sol at commit
// 7866168c

/// @title LegacyCrossDomainUtils
library LegacyCrossDomainUtils {
    /// @notice Generates the correct cross domain calldata for a message.
    /// @param _target Target contract address.
    /// @param _sender Message sender address.
    /// @param _message Message to send to the target.
    /// @param _messageNonce Nonce for the provided message.
    /// @return ABI encoded cross domain calldata.
    function encodeXDomainCalldata(
        address _target,
        address _sender,
        bytes memory _message,
        uint256 _messageNonce
    )
        internal
        pure
        returns (bytes memory)
    {
        // nosemgrep: sol-style-use-abi-encodecall
        return abi.encodeWithSignature(
            "relayMessage(address,address,bytes,uint256)", _target, _sender, _message, _messageNonce
        );
    }
}
