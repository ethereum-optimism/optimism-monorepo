// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { SystemConfig } from "src/L1/SystemConfig.sol";

// Libraries
import { StaticConfig } from "src/libraries/StaticConfig.sol";

// Interfaces
import { IOptimismPortalInterop as IOptimismPortal } from "interfaces/L1/IOptimismPortalInterop.sol";
import { ConfigType } from "interfaces/L2/IL1BlockInterop.sol";

/// @custom:proxied true
/// @title SystemConfigInterop
/// @notice The SystemConfig contract is used to manage configuration of an Optimism network.
///         All configuration is stored on L1 and picked up by L2 as part of the derviation of
///         the L2 chain.
contract SystemConfigInterop is SystemConfig {
    /// @custom:semver +interop-beta.10
    function version() public pure override returns (string memory) {
        return string.concat(super.version(), "+interop-beta.10");
    }
}
