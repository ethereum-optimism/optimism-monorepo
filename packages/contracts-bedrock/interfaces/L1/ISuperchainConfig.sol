// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { IDependencySet } from "interfaces/L2/IDependencySet.sol";
import { ISharedLockbox } from "interfaces/L1/ISharedLockbox.sol";

interface ISuperchainConfig is IDependencySet {
    enum UpdateType {
        GUARDIAN
    }

    event ConfigUpdate(UpdateType indexed updateType, bytes data);
    event Initialized(uint8 version);
    event Paused(string identifier);
    event Unpaused();
    event DependencyAdded(uint256 indexed chainId, address indexed systemConfig, address indexed portal);

    error Unauthorized();
    error DependencySetTooLarge();
    error InvalidChainID();
    error DependencyAlreadyAdded();

    function GUARDIAN_SLOT() external view returns (bytes32);
    function PAUSED_SLOT() external view returns (bytes32);
    function DEPENDENCY_MANAGER_SLOT() external view returns (bytes32);
    function SHARED_LOCKBOX() external view returns (ISharedLockbox);
    function guardian() external view returns (address guardian_);
    function dependencyManager() external view returns (address dependencyManager_);
    function initialize(address _guardian, address _dependencyManager, bool _paused) external;
    function pause(string memory _identifier) external;
    function paused() external view returns (bool paused_);
    function unpause() external;
    function version() external view returns (string memory);
    function addDependency(uint256 _chainId, address _systemConfig) external;
    function dependencySet() external view returns (uint256[] memory);
    function dependencySetSize() external view returns (uint8);

    function __constructor__(address _sharedLockbox) external;
}
