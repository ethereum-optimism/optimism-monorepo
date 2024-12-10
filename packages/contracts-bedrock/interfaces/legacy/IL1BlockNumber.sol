// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { ISemver } from "interfaces/universal/ISemver.sol";

/// @title IL1BlockNumber
/// @notice Interface for the L1BlockNumber contract.
interface IL1BlockNumber is ISemver {
    fallback() external payable;

    receive() external payable;

    function getL1BlockNumber() external view returns (uint256);

    function __constructor__() external;
}
