// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { Initializable } from "@openzeppelin/contracts/proxy/utils/Initializable.sol";

// Libraries
import { Storage } from "src/libraries/Storage.sol";
import { EnumerableSet } from "@openzeppelin/contracts/utils/structs/EnumerableSet.sol";
import { Unauthorized } from "src/libraries/errors/CommonErrors.sol";

// Interfaces
import { ISemver } from "interfaces/universal/ISemver.sol";

/// @custom:proxied true
/// @custom:audit none This contracts is not yet audited.
/// @title SuperchainConfig
/// @notice The SuperchainConfig contract is used to manage configuration of global superchain values.
contract SuperchainConfig is Initializable, ISemver {

    error InvalidChainID();
    error DependencySetTooLarge();
    error InvalidDependency();

    using EnumerableSet for EnumerableSet.UintSet;

    /// @notice Event emitted when a new dependency is added to the interop dependency set.
    event DependencyAdded(uint256 indexed chainId);

    /// @notice Event emitted when a dependency is removed from the interop dependency set.
    event DependencyRemoved(uint256 indexed chainId);

    /// @notice The interop dependency set, containing the chain IDs in it.
    EnumerableSet.UintSet dependencySet;

    /// @notice Enum representing different types of updates.
    /// @custom:value GUARDIAN            Represents an update to the guardian.
    enum UpdateType {
        GUARDIAN,
        DEPENDENCY_MANAGER
    }

    /// @notice Whether or not the Superchain is paused.
    bytes32 public constant PAUSED_SLOT = bytes32(uint256(keccak256("superchainConfig.paused")) - 1);

    /// @notice The address of the guardian, which can pause withdrawals from the System.
    ///         It can only be modified by an upgrade.
    bytes32 public constant GUARDIAN_SLOT = bytes32(uint256(keccak256("superchainConfig.guardian")) - 1);

    /// @notice Storage slot where the dependency manager address is stored
    ///         It can only be modified by an upgrade.
    bytes32 internal constant DEPENDENCY_MANAGER_SLOT = bytes32(uint256(keccak256("superchainConfig.dependencymanager")) - 1);

    /// @notice Emitted when the pause is triggered.
    /// @param identifier A string helping to identify provenance of the pause transaction.
    event Paused(string identifier);

    /// @notice Emitted when the pause is lifted.
    event Unpaused();

    /// @notice Emitted when configuration is updated.
    /// @param updateType Type of update.
    /// @param data       Encoded update data.
    event ConfigUpdate(UpdateType indexed updateType, bytes data);

    /// @notice Semantic version.
    /// @custom:semver 1.1.1-beta.4
    string public constant version = "1.1.1-beta.4";

    /// @notice Constructs the SuperchainConfig contract.
    constructor() {
        _disableInitializers();
    }

    /// @notice Initializer.
    /// @param _guardian    Address of the guardian, can pause the OptimismPortal.
    /// @param _paused      Initial paused status.
    function initialize(address _guardian, address _dependencyManager, bool _paused) external initializer {
        _setGuardian(_guardian);
        _setDependencyManager(_dependencyManager);
        if (_paused) {
            _pause("Initializer paused");
        }
    }

    /// @notice Returns true if a chain ID is in the interop dependency set and false otherwise.
    ///         The chain's chain ID is always considered to be in the dependency set.
    /// @param _chainId The chain ID to check.
    /// @return True if the chain ID to check is in the interop dependency set. False otherwise.
    function isInDependencySet(uint256 _chainId) public view returns (bool) {
        return dependencySet.contains(_chainId);
    }

    /// @notice Returns the size of the interop dependency set.
    /// @return The size of the interop dependency set.
    function dependencySetSize() external view returns (uint8) {
        return uint8(dependencySet.length());
    }

    /// @notice Getter for the guardian address.
    function guardian() public view returns (address guardian_) {
        guardian_ = Storage.getAddress(GUARDIAN_SLOT);
    }

    /// @notice Getter for the current paused status.
    function paused() public view returns (bool paused_) {
        paused_ = Storage.getBool(PAUSED_SLOT);
    }

    // @notice Getter for the current dependency manager
    function dependencyManager() public view returns (address dependencyManager_) {
        dependencyManager_ = Storage.getAddress(DEPENDENCY_MANAGER_SLOT);
    }

    /// @notice Adds a chain to the interop dependency set. Can only be called by the dependency manager.
    /// @param _chainId Chain ID of chain to add.
    function addDependency(uint256 _chainId) external {
        if (msg.sender != dependencyManager()) revert Unauthorized();

        if (dependencySet.length() == type(uint8).max) revert DependencySetTooLarge();
        if (_chainId == block.chainid) revert InvalidChainID();

        bool success = dependencySet.add(_chainId);
        if (!success) revert InvalidDependency();

        emit DependencyAdded(_chainId);
    }

    /// @notice Removes a chain from the interop dependency set. Can only be called by the dependency manager
    /// @param _chainId Chain ID of the chain to remove.
    function removeDependency(uint256 _chainId) external {
        if (msg.sender != dependencyManager()) revert Unauthorized();

        bool success = dependencySet.remove(_chainId);
        if (!success) revert InvalidDependency();

        emit DependencyRemoved(_chainId);
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

    /// @notice Sets the dependency manager address. This is only callable during initialization,
    ///         so an upgrade will be required to change the guardian.
    /// @param _dependencyManager The new dependency manager address.
    function _setDependencyManager(address _dependencyManager) internal {
        Storage.setAddress(DEPENDENCY_MANAGER_SLOT, _dependencyManager);
        emit ConfigUpdate(UpdateType.DEPENDENCY_MANAGER, abi.encode(_dependencyManager));
    }
}
