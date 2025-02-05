// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { AddressManager } from "src/legacy/AddressManager.sol";

/// @custom:legacy true
/// @title ResolvedDelegateProxy
/// @notice ResolvedDelegateProxy is a legacy proxy contract that makes use of the AddressManager to
///         resolve the implementation address. We're maintaining this contract for backwards
///         compatibility so we can manage all legacy proxies where necessary.
contract ResolvedDelegateProxy {
    /// @notice Mapping used to store the implementation name that corresponds to this contract. A
    ///         mapping was originally used as a way to bypass the same issue normally solved by
    ///         storing the implementation address in a specific storage slot that does not conflict
    ///         with any other storage slot. Generally NOT a safe solution but works as long as the
    ///         implementation does not also keep a mapping in the first storage slot.
    mapping(address => string) private implementationName;

    /// @notice Mapping used to store the address of the AddressManager contract where the
    ///         implementation address will be resolved from. Same concept here as with the above
    ///         mapping. Also generally unsafe but fine if the implementation doesn't keep a mapping
    ///         in the second storage slot.
    mapping(address => AddressManager) private addressManager;

    /// @param _addressManager  Address of the AddressManager.
    /// @param _implementationName implementationName of the contract to proxy to.
    constructor(AddressManager _addressManager, string memory _implementationName) {
        addressManager[address(this)] = _addressManager;
        implementationName[address(this)] = _implementationName;
    }

    /// @notice Fallback, performs a delegatecall to the resolved implementation address.
    fallback() external payable {
        address target = addressManager[address(this)].getAddress((implementationName[address(this)]));

        require(target != address(0), "ResolvedDelegateProxy: target address must be initialized");

        // slither-disable-next-line controlled-delegatecall
        (bool success, bytes memory returndata) = target.delegatecall(msg.data);

        if (success == true) {
            assembly {
                return(add(returndata, 0x20), mload(returndata))
            }
        } else {
            assembly {
                revert(add(returndata, 0x20), mload(returndata))
            }
        }
    }
}
