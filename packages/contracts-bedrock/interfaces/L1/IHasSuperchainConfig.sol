// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";

/// @notice Generic interface for contracts that have a superchain config.
interface IHasSuperchainConfig {
    /// @notice Retrieves the superchain config for a given contract.
    function superchainConfig() external view returns (ISuperchainConfig);
}
