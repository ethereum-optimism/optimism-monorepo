// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { ISemver } from "src/universal/interfaces/ISemver.sol";
import { IOptimismPortal } from "src/L1/interfaces/IOptimismPortal.sol";
import { Unauthorized } from "src/libraries/errors/CommonErrors.sol";

/// @custom:proxied true
/// @title SharedLockbox
/// @notice Manages ETH liquidity locking and unlocking for authorized OptimismPortals, enabling unified ETH liquidity
///         management across chains in the superchain cluster.
contract SharedLockbox is ISemver {
    /// @notice Emitted when ETH is locked in the lockbox by an authorized portal.
    /// @param portal The address of the portal that locked the ETH.
    /// @param amount The amount of ETH locked.
    event ETHLocked(address indexed portal, uint256 amount);

    /// @notice Emitted when ETH is unlocked from the lockbox by an authorized portal.
    /// @param portal The address of the portal that unlocked the ETH.
    /// @param amount The amount of ETH unlocked.
    event ETHUnlocked(address indexed portal, uint256 amount);

    /// @notice Emitted when a portal is set as authorized to interact with the lockbox.
    /// @param portal The address of the authorized portal.
    event PortalAuthorized(address indexed portal);

    /// @notice The address of the SuperchainConfig contract.
    address public immutable SUPERCHAIN_CONFIG;

    /// @notice OptimismPortals that are part of the dependency cluster authorized to interact with the SharedLockbox
    mapping(address => bool) public authorizedPortals;

    /// @notice Semantic version.
    /// @custom:semver 1.0.0-beta.1
    function version() public view virtual returns (string memory) {
        return "1.0.0-beta.1";
    }

    /// @notice Constructs the SharedLockbox contract.
    /// @param _superchainConfig The address of the SuperchainConfig contract.
    constructor(address _superchainConfig) {
        SUPERCHAIN_CONFIG = _superchainConfig;
    }

    /// @notice Locks ETH in the lockbox.
    ///         Called by an authorized portal when migrating its ETH liquidity or when depositing with some ETH value.
    function lockETH() external payable {
        if (!authorizedPortals[msg.sender]) revert Unauthorized();

        emit ETHLocked(msg.sender, msg.value);
    }

    /// @notice Unlocks ETH from the lockbox.
    ///         Called by an authorized portal when finalizing a withdrawal that requires ETH.
    function unlockETH(uint256 _value) external {
        if (!authorizedPortals[msg.sender]) revert Unauthorized();

        // Using `donateETH` to avoid triggering a deposit
        IOptimismPortal(payable(msg.sender)).donateETH{ value: _value }();
        emit ETHUnlocked(msg.sender, _value);
    }

    /// @notice Authorizes a portal to interact with the lockbox.
    function authorizePortal(address _portal) external {
        if (msg.sender != SUPERCHAIN_CONFIG) revert Unauthorized();

        authorizedPortals[_portal] = true;
        emit PortalAuthorized(_portal);
    }
}
