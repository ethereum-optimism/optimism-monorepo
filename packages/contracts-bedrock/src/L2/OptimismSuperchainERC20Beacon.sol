// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { IBeacon } from "@openzeppelin/contracts/proxy/beacon/IBeacon.sol";
import { ISemver } from "src/universal/interfaces/ISemver.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";

/// @custom:proxied true
/// @custom:predeployed 0x4200000000000000000000000000000000000027
/// @title OptimismSuperchainERC20Beacon
/// @notice OptimismSuperchainERC20Beacon is the beacon proxy for the OptimismSuperchainERC20 implementation.
contract OptimismSuperchainERC20Beacon is IBeacon, ISemver {
    /// @notice Semantic version.
    /// @custom:semver 1.0.0-beta.2
    string public constant version = "1.0.0-beta.2";

    /// @inheritdoc IBeacon
    function implementation() external pure override returns (address) {
        return Predeploys.OPTIMISM_SUPERCHAIN_ERC20;
    }
}
