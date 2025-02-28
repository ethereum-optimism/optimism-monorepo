// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { ISemver } from "interfaces/universal/ISemver.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { IPAOBase } from "interfaces/L1/IPAOBase.sol";

interface IETHLockbox is IPAOBase, ISemver {
    error ETHLockbox_Unauthorized();
    error ETHLockbox_Paused();
    error ETHLockbox_NoWithdrawalTransactions();
    error ETHLockbox_AlreadyAuthorized();
    error ETHLockbox_DifferentPAO();

    event Initialized(uint8 version);
    event ETHLocked(address indexed portal, uint256 amount);
    event ETHUnlocked(address indexed portal, uint256 amount);
    event PortalAuthorized(address indexed portal);
    event LockboxAuthorized(address indexed lockbox);
    event LiquidityMigrated(address indexed lockbox);
    event LiquidityReceived(address indexed lockbox);

    function initialize(address _superchainConfig, address[] calldata _portals) external;
    function superchainConfig() external view returns (ISuperchainConfig);
    function paused() external view returns (bool);
    function authorizedPortals(address) external view returns (bool);
    function authorizedLockboxes(address) external view returns (bool);
    function receiveLiquidity() external payable;
    function lockETH() external payable;
    function unlockETH(uint256 _value) external;
    function authorizePortal(address _portal) external;
    function authorizeLockbox(address _lockbox) external;
    function migrateLiquidity(address _lockbox) external;

    function __constructor__() external;
}
