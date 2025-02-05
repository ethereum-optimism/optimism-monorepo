// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { ISemver } from "interfaces/universal/ISemver.sol";

/// @title ILegacyMessagePasser
/// @notice Interface for the LegacyMessagePasser contract.
interface ILegacyMessagePasser is ISemver {
    function passMessageToL1(bytes memory _message) external;
    function sentMessages(bytes32) external view returns (bool);

    function __constructor__() external;
}
