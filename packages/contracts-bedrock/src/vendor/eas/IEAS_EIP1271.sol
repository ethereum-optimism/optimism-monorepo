// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

interface IEAS_EIP1271 {
    function getNonce(address account) external view returns (uint256);
}