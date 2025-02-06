// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { ISemver } from "interfaces/universal/ISemver.sol";
import { ISuperchainConfigInterop } from "interfaces/L1/ISuperchainConfigInterop.sol";

/// @title ISharedLockbox
/// @notice Interface for the SharedLockbox contract
interface ISharedLockbox is ISemver {
    error Unauthorized();
    error Paused();
    error InvalidInitialization();
    error NotInitializing();

    event Initialized(uint64 version);
    event ETHLocked(address indexed portal, uint256 amount);
    event ETHUnlocked(address indexed portal, uint256 amount);

    function superchainConfig() external view returns (ISuperchainConfigInterop superchainConfig_);
    function initialize(address _superchainConfig) external;
    function paused() external view returns (bool);
    function unlockETH(uint256 _value) external;
    function lockETH() external payable;

    function __constructor__() external;
}
