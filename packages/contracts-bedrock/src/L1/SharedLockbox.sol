// SPDX-License-Identifier: MIT
pragma solidity 0.8.25;

// Contracts
import { Initializable } from "@openzeppelin/contracts-v5/proxy/utils/Initializable.sol";

// Libraries
import { Unauthorized, Paused } from "src/libraries/errors/CommonErrors.sol";
import { Storage } from "src/libraries/Storage.sol";

// Interfaces
import { ISemver } from "interfaces/universal/ISemver.sol";
import { IOptimismPortal2 as IOptimismPortal } from "interfaces/L1/IOptimismPortal2.sol";
import { ISuperchainConfigInterop } from "interfaces/L1/ISuperchainConfigInterop.sol";

/// @custom:proxied true
/// @title SharedLockbox
/// @notice Manages ETH liquidity locking and unlocking for authorized OptimismPortals, enabling unified ETH liquidity
///         management across chains in the superchain cluster.
contract SharedLockbox is Initializable, ISemver {
    /// @notice Emitted when ETH is locked in the lockbox by an authorized portal.
    /// @param portal The address of the portal that locked the ETH.
    /// @param amount The amount of ETH locked.
    event ETHLocked(address indexed portal, uint256 amount);

    /// @notice Emitted when ETH is unlocked from the lockbox by an authorized portal.
    /// @param portal The address of the portal that unlocked the ETH.
    /// @param amount The amount of ETH unlocked.
    event ETHUnlocked(address indexed portal, uint256 amount);

    /// @notice The address of the SuperchainConfig contract.
    bytes32 internal constant SUPERCHAIN_CONFIG_SLOT = bytes32(uint256(keccak256("sharedLockbox.superchainConfig")) - 1);

    /// @notice Semantic version.
    /// @custom:semver 1.0.0-beta.1
    function version() public view virtual returns (string memory) {
        return "1.0.0-beta.1";
    }

    /// @notice Constructs the SharedLockbox contract.
    constructor() {
        _disableInitializers();
    }

    /// @notice Initializer.
    /// @param _superchainConfig The address of the SuperchainConfig contract.
    function initialize(address _superchainConfig) external initializer {
        Storage.setAddress(SUPERCHAIN_CONFIG_SLOT, _superchainConfig);
    }

    /// @notice Getter for the SuperchainConfig contract.
    function superchainConfig() public view returns (ISuperchainConfigInterop superchainConfig_) {
        superchainConfig_ = ISuperchainConfigInterop(Storage.getAddress(SUPERCHAIN_CONFIG_SLOT));
    }

    /// @notice Getter for the current paused status.
    function paused() public view returns (bool) {
        return superchainConfig().paused();
    }

    /// @notice Locks ETH in the lockbox.
    ///         Called by an authorized portal when migrating its ETH liquidity or when depositing with some ETH value.
    function lockETH() external payable {
        if (!superchainConfig().authorizedPortals(msg.sender)) revert Unauthorized();

        emit ETHLocked(msg.sender, msg.value);
    }

    /// @notice Unlocks ETH from the lockbox.
    ///         Called by an authorized portal when finalizing a withdrawal that requires ETH.
    ///         Cannot be called if the lockbox is paused.
    /// @param _value The amount of ETH to unlock.
    function unlockETH(uint256 _value) external {
        if (paused()) revert Paused();
        if (!superchainConfig().authorizedPortals(msg.sender)) revert Unauthorized();

        // Using `donateETH` to avoid triggering a deposit
        IOptimismPortal(payable(msg.sender)).donateETH{ value: _value }();
        emit ETHUnlocked(msg.sender, _value);
    }
}
