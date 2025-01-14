// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Libraries
import { Predeploys } from "src/libraries/Predeploys.sol";
import { Semver } from "src/universal/Semver.sol";

// Interfaces
import { IBeacon } from "@openzeppelin/contracts/proxy/beacon/IBeacon.sol";

/// @custom:proxied true
/// @custom:predeployed 0x4200000000000000000000000000000000000027
/// @title OptimismSuperchainERC20Beacon
/// @notice OptimismSuperchainERC20Beacon is the beacon proxy for the OptimismSuperchainERC20 implementation.
contract OptimismSuperchainERC20Beacon is IBeacon, Semver {
    /// @notice Semantic version.
    /// @custom:semver 1.0.1
    function _version() internal pure override returns (Versions memory) {
        return Versions({ major: 1, minor: 0, patch: 1, suffix: "" });
    }

    /// @inheritdoc IBeacon
    function implementation() external pure override returns (address) {
        return Predeploys.OPTIMISM_SUPERCHAIN_ERC20;
    }
}
