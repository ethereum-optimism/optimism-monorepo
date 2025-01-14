// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

// Interfaces
import { Initializable } from "src/vendor/Initializable-v5.sol";

/// @title Reinitializable
/// @notice Expands Initializable v5 vendored from OpenZeppelin to expose the version getter, and provides a
///         reinitValue() function to be passed into the reinitializer modifier.
/// @dev This contract should be inherited by any upgradeable contracts. Currently it is only used by L1 contracts, this
///      should be expanded to L2 contracts in the future.
///      On upgradeable contracts:
///       - Both the initialize() and upgrade() functions should have the reinitializer(reinitValue()) modifier.
///       - The _reinitNonce() function must be implemented on each contract. Its return value is used to determine if
///         the upgrade() function should be called during an upgrade.
///       When to call upgrade():
///       - The upgrade() function should be called if and only if it is updating storage which will create a difference
///         in the contract's storage.
/// TODO: move the following to the OPCM's upgrade() function once it is merged in:
///       - The OPCM's upgrade() function should be emptied out following an upgrade.
///       - Contracts which are unchanged should not be touched during an upgrade.
///       - when a contract's bytecode is changing, it should then be upgraded using `ProxyAdmin.upgrade()`.
///       - When a contract's storage is changing, it should be upgraded using `ProxyAdmin.upgradeTo()`.
abstract contract Reinitializable is Initializable {
    /// @notice Spacing constant. Allowing for 100 patch releases between upgrades.
    uint64 private constant SPACING = 100;

    /// @notice Getter for the nonce of the contract. This must be implemented on each contract.
    function _reinitNonce() internal view virtual returns (uint64);

    /// @notice Converts a contact's semver to a uint64 with padding for use with the reinitializer modifier.
    function reinitValue() public view returns (uint64) {
        return SPACING * _reinitNonce();
    }

    /// @notice Getter for the initialized version of the contract.
    function getInitializedVersion() external view returns (uint64) {
        return _getInitializedVersion();
    }
}
