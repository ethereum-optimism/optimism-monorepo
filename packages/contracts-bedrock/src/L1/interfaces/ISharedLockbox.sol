// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { ISemver } from "src/universal/interfaces/ISemver.sol";

/// @title ISharedLockbox
/// @notice Interface for the SharedLockbox contract
interface ISharedLockbox is ISemver {
    error Unauthorized();

    event ETHLocked(address indexed portal, uint256 amount);

    event ETHUnlocked(address indexed portal, uint256 amount);

    event AuthorizedPortal(address indexed portal);

    function SUPERCHAIN_CONFIG() external view returns (address);

    function authorizedPortals(address) external view returns (bool);

    function __constructor__(address _superchainConfig) external;

    function unlockETH(uint256 _value) external;

    function lockETH() external payable;

    function authorizePortal(address _portal) external;
}
