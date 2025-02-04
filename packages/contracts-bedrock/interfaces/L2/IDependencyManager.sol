// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { ISemver } from "interfaces/universal/ISemver.sol";

/// @title IDependencyManager
/// @notice Interface for the DependencyManager contract.
interface IDependencyManager is ISemver {
    error DependencySetSizeTooLarge();
    error AlreadyDependency();
    error Unauthorized();

    event DependencyAdded(uint256 indexed chainId, address indexed systemConfig, address indexed superchainConfig);

    function addDependency(address _superchainConfig, uint256 _chainId, address _systemConfig) external;
    function isInDependencySet(uint256 _chainId) external view returns (bool);
    function dependencySetSize() external view returns (uint256);
    function dependencySet() external view returns (uint256[] memory);

    function __constructor__() external;
}
