// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { Ownable } from "@openzeppelin/contracts/access/Ownable.sol";

/// @custom:legacy true
/// @title AddressManager
/// @notice AddressManager is a legacy contract that was used in the old version of the Optimism
///         system to manage a registry of string names to addresses. We now use a more standard
///         proxy system instead, but this contract is still necessary for backwards compatibility
///         with several older contracts.
contract AddressManager is Ownable {
    /// @notice Mapping of the hashes of string names to addresses.
    mapping(bytes32 => address) private addresses;

    /// @notice Emitted when an address is modified in the registry.
    /// @param name       String name being set in the registry.
    /// @param newAddress Address set for the given name.
    /// @param oldAddress Address that was previously set for the given name.
    event AddressSet(string indexed name, address newAddress, address oldAddress);

    /// @notice Changes the address associated with a particular name.
    /// @param _name    String name to associate an address with.
    /// @param _address Address to associate with the name.
    function setAddress(string memory _name, address _address) external onlyOwner {
        bytes32 nameHash = _getNameHash(_name);
        address oldAddress = addresses[nameHash];
        addresses[nameHash] = _address;

        emit AddressSet(_name, _address, oldAddress);
    }

    /// @notice Retrieves the address associated with a given name.
    /// @param _name Name to retrieve an address for.
    /// @return Address associated with the given name.
    function getAddress(string memory _name) external view returns (address) {
        return addresses[_getNameHash(_name)];
    }

    /// @notice Computes the hash of a name.
    /// @param _name Name to compute a hash for.
    /// @return Hash of the given name.
    function _getNameHash(string memory _name) internal pure returns (bytes32) {
        return keccak256(abi.encodePacked(_name));
    }
}
