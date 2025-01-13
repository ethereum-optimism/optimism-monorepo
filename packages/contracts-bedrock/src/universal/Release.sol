// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// @title Release
/// @notice Library for handling release versioning with padded spacing
abstract contract Release {
    /// @notice Spacing constants for version components.
    /// @dev These values are used to convert the MAJOR, MINOR, and PATCH components to a uint64.
    uint64 private constant SPACING = 1_000_000;

    /// @notice Version identifier, used for upgrades.
    uint32 private constant MAJOR = 2;
    uint16 private constant MINOR = 0;
    uint16 private constant PATCH = 0;

    /// @notice Converts release components to a uint64 with padding
    function toUint64() internal pure returns (uint64) {
        return (MAJOR * SPACING ** 2) + (MINOR * SPACING) + PATCH;
    }

    /// @notice Getter for the version components.
    function getVersion() internal pure returns (uint32, uint16, uint16) {
        return (MAJOR, MINOR, PATCH);
    }
}
