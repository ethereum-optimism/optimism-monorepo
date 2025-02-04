// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { IDependencySet } from "interfaces/L2/IDependencySet.sol";
import { ISharedLockbox } from "interfaces/L1/ISharedLockbox.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";

interface ISuperchainConfigInterop is IDependencySet, ISuperchainConfig {
    event DependencyAdded(uint256 indexed chainId, address indexed systemConfig, address indexed portal);

    error Unauthorized();
    error DependencySetTooLarge();
    error DependencyAlreadyAdded();
    error InvalidSuperchainConfig();
    error PortalAlreadyAuthorized();
    error SuperchainPaused();

    function CLUSTER_MANAGER_SLOT() external view returns (bytes32);
    function sharedLockbox() external view returns (ISharedLockbox sharedLockbox_);
    function clusterManager() external view returns (address clusterManager_);
    function initialize(address _guardian, bool _paused, address _clusterManager, address _sharedLockbox) external;
    function version() external pure returns (string memory);
    function addDependency(uint256 _chainId, address _systemConfig) external;
    function dependencySet() external view returns (uint256[] memory);
    function dependencySetSize() external view returns (uint256);
    function authorizedPortals(address _portal) external view returns (bool);

    function __constructor__() external;
}
