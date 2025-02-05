// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { IOwnable } from "interfaces/universal/IOwnable.sol";

/// @title IAddressManager
/// @notice Interface for the AddressManager contract.
interface IAddressManager is IOwnable {
    event AddressSet(string indexed name, address newAddress, address oldAddress);

    function getAddress(string memory _name) external view returns (address);
    function setAddress(string memory _name, address _address) external;

    function __constructor__() external;
}
