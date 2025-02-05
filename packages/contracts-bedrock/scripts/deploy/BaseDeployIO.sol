// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { CommonBase } from "forge-std/Base.sol";

/// @notice All contracts of the form `Deploy<X>Input` and `Deploy<X>Output` should inherit from this contract.
/// It provides a base set of functionality, such as access to cheat codes, that these scripts may need.
/// See the comments in `DeploySuperchain.s.sol` for more information on this pattern.
abstract contract BaseDeployIO is CommonBase { }
