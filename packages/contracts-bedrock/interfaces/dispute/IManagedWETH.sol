// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";

interface IManagedWETH {
    struct WithdrawalRequest {
        uint256 amount;
        uint256 timestamp;
    }

    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);
    event Initialized(uint8 version);
    event Unwrap(address indexed src, uint256 wad);

    fallback() external payable;
    receive() external payable;

    function config() external view returns (ISuperchainConfig);
    function hold(address _guy) external;
    function hold(address _guy, uint256 _wad) external;
    function initialize(address _owner, ISuperchainConfig _config) external;
    function owner() external view returns (address);
    function recover(uint256 _wad) external;
    function transferOwnership(address newOwner) external; // nosemgrep
    function renounceOwnership() external;
    function version() external view returns (string memory);

    function withdraw(uint256 _wad) external;

    event Approval(address indexed src, address indexed guy, uint256 wad);

    event Transfer(address indexed src, address indexed dst, uint256 wad);

    event Deposit(address indexed dst, uint256 wad);

    event Withdrawal(address indexed src, uint256 wad);

    function name() external view returns (string memory);

    function symbol() external view returns (string memory);

    function decimals() external view returns (uint8);

    function balanceOf(address src) external view returns (uint256);

    function allowance(address owner, address spender) external view returns (uint256);

    function deposit() external payable;

    function totalSupply() external view returns (uint256);

    function approve(address guy, uint256 wad) external returns (bool);

    function transfer(address dst, uint256 wad) external returns (bool);

    function transferFrom(address src, address dst, uint256 wad) external returns (bool);

    function __constructor__(uint256 _delay) external;
}
