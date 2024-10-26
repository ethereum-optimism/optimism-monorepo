// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { IWETH98 } from "src/universal/interfaces/IWETH98.sol";
import { ISuperchainConfig } from "src/L1/interfaces/ISuperchainConfig.sol";

interface IDelayedWETH is IWETH98 {
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
    function delay() external view returns (uint256);
    function hold(address _guy, uint256 _wad) external;
    function initialize(address _owner, ISuperchainConfig _config) external;
    function owner() external view returns (address);
    function recover(uint256 _wad) external;
    function transferOwnership(address newOwner) external; // nosemgrep
    function renounceOwnership() external;
    function unlock(address _guy, uint256 _wad) external;
    function withdraw(address _guy, uint256 _wad) external;
    function withdrawals(address, address) external view returns (uint256 amount, uint256 timestamp);
    function version() external view returns (string memory);

    function withdraw(uint256 _wad) external override;

    function __constructor__(uint256 _delay) external;
}
