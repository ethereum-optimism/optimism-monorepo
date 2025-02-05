// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { ISemver } from "interfaces/universal/ISemver.sol";

/// @title IOptimismSuperchainERC20Beacon
/// @notice Interface for the OptimismSuperchainERC20Beacon contract
interface IOptimismSuperchainERC20Beacon is ISemver {
    function implementation() external pure returns (address);

    function __constructor__() external;
}
