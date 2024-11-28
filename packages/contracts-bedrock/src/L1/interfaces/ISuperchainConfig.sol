// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { IDependencySet } from "src/L2/interfaces/IDependencySet.sol";
import { ISharedLockbox } from "src/L1/interfaces/ISharedLockbox.sol";

interface ISuperchainConfig is IDependencySet {
    enum UpdateType {
        GUARDIAN
    }

    event ConfigUpdate(UpdateType indexed updateType, bytes data);
    event Initialized(uint8 version);
    event Paused(string identifier);
    event Unpaused();
    event ChainAdded(uint256 indexed chainId, address indexed systemConfig, address indexed portal);

    error ChainAlreadyAdded();
    error Unauthorized();

    function GUARDIAN_SLOT() external view returns (bytes32);
    function PAUSED_SLOT() external view returns (bytes32);
    function SHARED_LOCKBOX() external view returns (ISharedLockbox);
    function guardian() external view returns (address guardian_);
    function systemConfigs(uint256) external view returns (address);
    function initialize(address _guardian, bool _paused) external;
    function pause(string memory _identifier) external;
    function paused() external view returns (bool paused_);
    function unpause() external;
    function version() external view returns (string memory);
    function addChain(uint256 _chainId, address _systemConfig) external;
    function dependencySet() external view returns (uint256[] memory);

    function __constructor__(address _sharedLockbox) external;
}
