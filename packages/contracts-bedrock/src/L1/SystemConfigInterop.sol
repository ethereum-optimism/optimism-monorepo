// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { SystemConfig } from "src/L1/SystemConfig.sol";

/// @custom:proxied true
/// @title SystemConfigInterop
/// @notice The SystemConfig contract is used to manage configuration of an Optimism network.
///         All configuration is stored on L1 and picked up by L2 as part of the derviation of
///         the L2 chain.
contract SystemConfigInterop is SystemConfig {
    /// @custom:semver +interop-beta.11
    function version() public pure override returns (string memory) {
        return string.concat(super.version(), "+interop-beta.11");
    }
}
