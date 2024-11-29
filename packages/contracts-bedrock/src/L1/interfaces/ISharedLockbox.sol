// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { ISemver } from "src/universal/interfaces/ISemver.sol";
import { ISuperchainConfig } from "src/L1/interfaces/ISuperchainConfig.sol";

/// @title ISharedLockbox
/// @notice Interface for the SharedLockbox contract
interface ISharedLockbox is ISemver {
    error Unauthorized();

    error Paused();

    event ETHLocked(address indexed portal, uint256 amount);

    event ETHUnlocked(address indexed portal, uint256 amount);

    event PortalAuthorized(address indexed portal);

    function SUPERCHAIN_CONFIG() external view returns (ISuperchainConfig);

    function authorizedPortals(address) external view returns (bool);

    function __constructor__(address _superchainConfig) external;

    function paused() external view returns (bool);

    function unlockETH(uint256 _value) external;

    function lockETH() external payable;

    function authorizePortal(address _portal) external;
}
