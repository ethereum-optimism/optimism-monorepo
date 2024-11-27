// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { IAddressManager } from "interfaces/legacy/IAddressManager.sol";

/// @title IResolvedDelegateProxy
/// @notice Interface for the ResolvedDelegateProxy contract.
interface IResolvedDelegateProxy {
    fallback() external payable;

    receive() external payable;

    function __constructor__(IAddressManager _addressManager, string memory _implementationName) external;
}
