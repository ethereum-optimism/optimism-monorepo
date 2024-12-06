// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Libraries
import { Storage } from "src/libraries/Storage.sol";

// Interfaces
import { ISemver } from "interfaces/universal/ISemver.sol";

/// @title StorageSetter
/// @notice A simple contract that allows setting arbitrary storage slots.
///         WARNING: this contract is not safe to be called by untrusted parties.
///         It is only meant as an intermediate step during upgrades.
contract StorageSetter is ISemver {
    /// @notice Represents a storage slot key value pair.
    struct Slot {
        bytes32 key;
        bytes32 value;
    }

    /// @notice Semantic version.
    /// @custom:semver 1.2.1-beta.4
    string public constant version = "1.2.1-beta.4";

    /// @notice Stores a bytes32 `_value` at `_slot`. Any storage slots that
    ///         are packed should be set through this interface.
    function setBytes32(bytes32 _slot, bytes32 _value) public {
        Storage.setBytes32(_slot, _value);
    }

    /// @notice Stores a bytes32 value at each key in `_slots`.
    function setBytes32(Slot[] calldata _slots) public {
        uint256 length = _slots.length;
        for (uint256 i; i < length; i++) {
            Storage.setBytes32(_slots[i].key, _slots[i].value);
        }
    }

    /// @notice Retrieves a bytes32 value from `_slot`.
    function getBytes32(bytes32 _slot) external view returns (bytes32 value_) {
        value_ = Storage.getBytes32(_slot);
    }

    /// @notice Stores a uint256 `_value` at `_slot`.
    function setUint(bytes32 _slot, uint256 _value) public {
        Storage.setUint(_slot, _value);
    }

    /// @notice Retrieves a uint256 value from `_slot`.
    function getUint(bytes32 _slot) external view returns (uint256 value_) {
        value_ = Storage.getUint(_slot);
    }

    /// @notice Stores an address `_value` at `_slot`.
    function setAddress(bytes32 _slot, address _address) public {
        Storage.setAddress(_slot, _address);
    }

    /// @notice Retrieves an address value from `_slot`.
    function getAddress(bytes32 _slot) external view returns (address addr_) {
        addr_ = Storage.getAddress(_slot);
    }

    /// @notice Stores a bool `_value` at `_slot`.
    function setBool(bytes32 _slot, bool _value) public {
        Storage.setBool(_slot, _value);
    }

    /// @notice Retrieves a bool value from `_slot`.
    function getBool(bytes32 _slot) external view returns (bool value_) {
        value_ = Storage.getBool(_slot);
    }
}
