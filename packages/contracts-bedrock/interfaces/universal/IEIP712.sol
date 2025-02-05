// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// @title IEIP712
interface IEIP712 {
    function DOMAIN_SEPARATOR() external view returns (bytes32);
}
