// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// @title IStaticERC1967Proxy
/// @notice IStaticERC1967Proxy is a static version of the ERC1967 proxy interface.
interface IStaticERC1967Proxy {
    function implementation() external view returns (address);
    function admin() external view returns (address);
}
