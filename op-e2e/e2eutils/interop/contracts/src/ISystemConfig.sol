// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

interface ISystemConfig {
    /// @notice Adds a chain to the interop dependency set. Can only be called by the dependency manager.
    /// @param _chainId Chain ID of chain to add.
    function addDependency(uint256 _chainId) external;
}
