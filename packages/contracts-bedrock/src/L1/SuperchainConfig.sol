// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { Initializable } from "@openzeppelin/contracts/proxy/utils/Initializable.sol";

// Libraries
import { Storage } from "src/libraries/Storage.sol";
import { EnumerableSet } from "@openzeppelin/contracts/utils/structs/EnumerableSet.sol";
import { Unauthorized } from "src/libraries/errors/CommonErrors.sol";
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";
import { ISharedLockbox } from "interfaces/L1/ISharedLockbox.sol";

// Interfaces
import { ISemver } from "interfaces/universal/ISemver.sol";

/// @custom:proxied true
/// @custom:audit none This contracts is not yet audited.
/// @title SuperchainConfig
/// @notice The SuperchainConfig contract is used to manage configuration of global superchain values.
contract SuperchainConfig is Initializable, ISemver {
    using EnumerableSet for EnumerableSet.UintSet;

    /// @notice Enum representing different types of updates.
    /// @custom:value GUARDIAN            Represents an update to the guardian.
    /// @custom:value DEPENDENCY_MANAGER  Represents an update to the dependency manager.
    enum UpdateType {
        GUARDIAN,
        DEPENDENCY_MANAGER
    }

    /// @notice Whether or not the Superchain is paused.
    bytes32 public constant PAUSED_SLOT = bytes32(uint256(keccak256("superchainConfig.paused")) - 1);

    /// @notice The address of the guardian, which can pause withdrawals from the System.
    ///         It can only be modified by an upgrade.
    bytes32 public constant GUARDIAN_SLOT = bytes32(uint256(keccak256("superchainConfig.guardian")) - 1);

    /// @notice The address of the dependency manager, which can add a chain to the dependency set.
    ///         It can only be modified by an upgrade.
    bytes32 public constant DEPENDENCY_MANAGER_SLOT =
        bytes32(uint256(keccak256("superchainConfig.dependencyManager")) - 1);

    // The Shared Lockbox contract
    ISharedLockbox public immutable SHARED_LOCKBOX;

    /// @notice Emitted when the pause is triggered.
    /// @param identifier A string helping to identify provenance of the pause transaction.
    event Paused(string identifier);

    /// @notice Emitted when the pause is lifted.
    event Unpaused();

    /// @notice Emitted when configuration is updated.
    /// @param updateType Type of update.
    /// @param data       Encoded update data.
    event ConfigUpdate(UpdateType indexed updateType, bytes data);

    /// @notice Emitted when a new dependency is added as part of the dependency set.
    /// @param chainId      The chain ID.
    /// @param systemConfig The address of the SystemConfig contract.
    /// @param portal       The address of the OptimismPortal contract.
    event DependencyAdded(uint256 indexed chainId, address indexed systemConfig, address indexed portal);

    /// @notice Thrown when the dependency set is too large to add a new dependency.
    error DependencySetTooLarge();

    /// @notice Thrown when the input chain ID is the same as the current chain ID.
    error InvalidChainID();

    /// @notice Thrown when the input dependency is already added to the set.
    error DependencyAlreadyAdded();

    /// @notice Semantic version.
    /// @custom:semver 1.1.1-beta.5
    string public constant version = "1.1.1-beta.5";

    // Dependency set of chains that are part of the same cluster
    EnumerableSet.UintSet internal _dependencySet;

    /// @notice Constructs the SuperchainConfig contract.
    constructor(address _sharedLockbox) {
        SHARED_LOCKBOX = ISharedLockbox(_sharedLockbox);
        _disableInitializers();
    }

    /// @notice Initializer.
    /// @param _guardian             Address of the guardian, can pause the OptimismPortal.
    /// @param _dependencyManager    Address of the dependencyManager, can add a chain to the dependency set.
    /// @param _paused               Initial paused status.
    function initialize(address _guardian, address _dependencyManager, bool _paused) external initializer {
        _setGuardian(_guardian);
        _setDependencyManager(_dependencyManager);
        if (_paused) {
            _pause("Initializer paused");
        }
    }

    /// @notice Getter for the guardian address.
    function guardian() public view returns (address guardian_) {
        guardian_ = Storage.getAddress(GUARDIAN_SLOT);
    }

    /// @notice Getter for the dependency manager address.
    function dependencyManager() public view returns (address dependencyManager_) {
        dependencyManager_ = Storage.getAddress(DEPENDENCY_MANAGER_SLOT);
    }

    /// @notice Getter for the current paused status.
    function paused() public view returns (bool paused_) {
        paused_ = Storage.getBool(PAUSED_SLOT);
    }

    /// @notice Pauses withdrawals.
    /// @param _identifier (Optional) A string to identify provenance of the pause transaction.
    function pause(string memory _identifier) external {
        require(msg.sender == guardian(), "SuperchainConfig: only guardian can pause");
        _pause(_identifier);
    }

    /// @notice Pauses withdrawals.
    /// @param _identifier (Optional) A string to identify provenance of the pause transaction.
    function _pause(string memory _identifier) internal {
        Storage.setBool(PAUSED_SLOT, true);
        emit Paused(_identifier);
    }

    /// @notice Unpauses withdrawals.
    function unpause() external {
        require(msg.sender == guardian(), "SuperchainConfig: only guardian can unpause");
        Storage.setBool(PAUSED_SLOT, false);
        emit Unpaused();
    }

    /// @notice Sets the guardian address. This is only callable during initialization, so an upgrade
    ///         will be required to change the guardian.
    /// @param _guardian The new guardian address.
    function _setGuardian(address _guardian) internal {
        Storage.setAddress(GUARDIAN_SLOT, _guardian);
        emit ConfigUpdate(UpdateType.GUARDIAN, abi.encode(_guardian));
    }

    /// @notice Sets the dependency manager address. This is only callable during initialization, so an upgrade
    ///         will be required to change the dependency manager.
    /// @param _dependencyManager The new dependency manager address.
    function _setDependencyManager(address _dependencyManager) internal {
        Storage.setAddress(DEPENDENCY_MANAGER_SLOT, _dependencyManager);
        emit ConfigUpdate(UpdateType.DEPENDENCY_MANAGER, abi.encode(_dependencyManager));
    }

    /// @notice Adds a new dependency to the dependency set. It also authorizes it's OptimismPortal on the
    ///         SharedLockbox. Can only be called by the dependency manager.
    /// @param _chainId         The chain ID.
    /// @param _systemConfig    The SystemConfig contract address of the chain to add.
    function addDependency(uint256 _chainId, address _systemConfig) external {
        if (msg.sender != dependencyManager()) revert Unauthorized();

        if (_dependencySet.length() == type(uint8).max) revert DependencySetTooLarge();
        if (_chainId == block.chainid) revert InvalidChainID();

        // Add to the dependency set and check it is not already added (`add()` returns false if it already exists)
        if (!_dependencySet.add(_chainId)) revert DependencyAlreadyAdded();

        // Authorize the portal on the shared lockbox
        address portal = ISystemConfig(_systemConfig).optimismPortal();
        SHARED_LOCKBOX.authorizePortal(portal);

        emit DependencyAdded(_chainId, _systemConfig, portal);
    }

    /// @notice Checks if a chain is part or not of the dependency set.
    /// @param _chainId The chain ID to check for.
    function isInDependencySet(uint256 _chainId) public view returns (bool) {
        return _dependencySet.contains(_chainId);
    }

    /// @notice Getter for the chain ids list on the dependency set.
    function dependencySet() external view returns (uint256[] memory) {
        return _dependencySet.values();
    }

    /// @notice Returns the size of the dependency set.
    /// @return The size of the dependency set.
    function dependencySetSize() external view returns (uint8) {
        return uint8(_dependencySet.length());
    }
}
