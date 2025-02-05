// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Libraries
import { Constants } from "src/libraries/Constants.sol";

/// @title Proxy
/// @notice Proxy is a transparent proxy that passes through the call if the caller is the owner or
///         if the caller is address(0), meaning that the call originated from an off-chain
///         simulation.
contract Proxy {
    /// @notice An event that is emitted each time the implementation is changed. This event is part
    ///         of the EIP-1967 specification.
    /// @param implementation The address of the implementation contract
    event Upgraded(address indexed implementation);

    /// @notice An event that is emitted each time the owner is upgraded. This event is part of the
    ///         EIP-1967 specification.
    /// @param previousAdmin The previous owner of the contract
    /// @param newAdmin      The new owner of the contract
    event AdminChanged(address previousAdmin, address newAdmin);

    /// @notice A modifier that reverts if not called by the owner or by address(0) to allow
    ///         eth_call to interact with this proxy without needing to use low-level storage
    ///         inspection. We assume that nobody is able to trigger calls from address(0) during
    ///         normal EVM execution.
    modifier proxyCallIfNotAdmin() {
        if (msg.sender == _getAdmin() || msg.sender == address(0)) {
            _;
        } else {
            // This WILL halt the call frame on completion.
            _doProxyCall();
        }
    }

    /// @notice Sets the initial admin during contract deployment. Admin address is stored at the
    ///         EIP-1967 admin storage slot so that accidental storage collision with the
    ///         implementation is not possible.
    /// @param _admin Address of the initial contract admin. Admin has the ability to access the
    ///               transparent proxy interface.
    constructor(address _admin) {
        _changeAdmin(_admin);
    }

    // slither-disable-next-line locked-ether
    receive() external payable {
        // Proxy call by default.
        _doProxyCall();
    }

    // slither-disable-next-line locked-ether
    fallback() external payable {
        // Proxy call by default.
        _doProxyCall();
    }

    /// @notice Set the implementation contract address. The code at the given address will execute
    ///         when this contract is called.
    /// @param _implementation Address of the implementation contract.
    function upgradeTo(address _implementation) public virtual proxyCallIfNotAdmin {
        _setImplementation(_implementation);
    }

    /// @notice Set the implementation and call a function in a single transaction. Useful to ensure
    ///         atomic execution of initialization-based upgrades.
    /// @param _implementation Address of the implementation contract.
    /// @param _data           Calldata to delegatecall the new implementation with.
    function upgradeToAndCall(
        address _implementation,
        bytes calldata _data
    )
        public
        payable
        virtual
        proxyCallIfNotAdmin
        returns (bytes memory)
    {
        _setImplementation(_implementation);
        (bool success, bytes memory returndata) = _implementation.delegatecall(_data);
        require(success, "Proxy: delegatecall to new implementation contract failed");
        return returndata;
    }

    /// @notice Changes the owner of the proxy contract. Only callable by the owner.
    /// @param _admin New owner of the proxy contract.
    function changeAdmin(address _admin) public virtual proxyCallIfNotAdmin {
        _changeAdmin(_admin);
    }

    /// @notice Gets the owner of the proxy contract.
    /// @return Owner address.
    function admin() public virtual proxyCallIfNotAdmin returns (address) {
        return _getAdmin();
    }

    //// @notice Queries the implementation address.
    /// @return Implementation address.
    function implementation() public virtual proxyCallIfNotAdmin returns (address) {
        return _getImplementation();
    }

    /// @notice Sets the implementation address.
    /// @param _implementation New implementation address.
    function _setImplementation(address _implementation) internal {
        bytes32 proxyImplementation = Constants.PROXY_IMPLEMENTATION_ADDRESS;
        assembly {
            sstore(proxyImplementation, _implementation)
        }
        emit Upgraded(_implementation);
    }

    /// @notice Changes the owner of the proxy contract.
    /// @param _admin New owner of the proxy contract.
    function _changeAdmin(address _admin) internal {
        address previous = _getAdmin();
        bytes32 proxyOwner = Constants.PROXY_OWNER_ADDRESS;
        assembly {
            sstore(proxyOwner, _admin)
        }
        emit AdminChanged(previous, _admin);
    }

    /// @notice Performs the proxy call via a delegatecall.
    function _doProxyCall() internal {
        address impl = _getImplementation();
        require(impl != address(0), "Proxy: implementation not initialized");

        assembly {
            // Copy calldata into memory at 0x0....calldatasize.
            calldatacopy(0x0, 0x0, calldatasize())

            // Perform the delegatecall, make sure to pass all available gas.
            let success := delegatecall(gas(), impl, 0x0, calldatasize(), 0x0, 0x0)

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

    /// @notice Queries the implementation address.
    /// @return Implementation address.
    function _getImplementation() internal view returns (address) {
        address impl;
        bytes32 proxyImplementation = Constants.PROXY_IMPLEMENTATION_ADDRESS;
        assembly {
            impl := sload(proxyImplementation)
        }
        return impl;
    }

    /// @notice Queries the owner of the proxy contract.
    /// @return Owner address.
    function _getAdmin() internal view returns (address) {
        address owner;
        bytes32 proxyOwner = Constants.PROXY_OWNER_ADDRESS;
        assembly {
            owner := sload(proxyOwner)
        }
        return owner;
    }
}
