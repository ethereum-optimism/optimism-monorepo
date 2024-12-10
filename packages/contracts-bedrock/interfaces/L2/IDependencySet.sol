// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// @title IDependencySet
/// @notice Interface for L1Block with only `isInDependencySet(uint256)` method.
interface IDependencySet {
    /// @notice Returns true if the chain associated with input chain ID is in the interop dependency set.
    ///         Every chain is in the interop dependency set of itself.
    /// @param _chainId Input chain ID.
    /// @return True if the input chain ID corresponds to a chain in the interop dependency set, and false otherwise.
    function isInDependencySet(uint256 _chainId) external view returns (bool);
}
