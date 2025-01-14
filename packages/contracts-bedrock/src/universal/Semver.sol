// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { LibString } from "@solady/utils/LibString.sol";

/// @title Semver
/// @notice Semver is a simple contract for ensuring that contracts are
///         versioned using semantic versioning.
abstract contract Semver {
    /// @notice Spacing constant for version components. Used to convert the MAJOR, MINOR, and PATCH components to a
    ///         uint64.
    uint64 private constant SPACING = 1_000_000;

    struct Versions {
        uint16 major;
        uint16 minor;
        uint16 patch;
    }

    /// @notice Getter for the semantic version of the contract. This must be implemented on each contract.
    function _version() internal pure virtual returns (Versions memory);

    /// @notice Getter for the semantic version of the contract. This is not
    ///         meant to be used onchain but instead meant to be used by offchain
    ///         tooling.
    /// @return Semver contract version as a string.
    function version() external view returns (string memory) {
        Versions memory v = _version();
        return string.concat(
            LibString.toString(v.major), ".", LibString.toString(v.minor), ".", LibString.toString(v.patch)
        );
    }

    /// @notice Converts a contact's semver to a uint64 with padding for use with the reinitializer modifier.
    function reinitializerValue() public pure returns (uint64) {
        Versions memory v = _version();
        return (v.major * SPACING ** 2) + (v.minor * SPACING) + v.patch;
    }
}
