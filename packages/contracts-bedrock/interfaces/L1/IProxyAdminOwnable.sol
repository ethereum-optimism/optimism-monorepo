// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

interface IProxyAdminOwnable {
    function adminOwner() external view returns (address);
}
