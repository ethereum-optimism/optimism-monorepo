// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { SuperchainConfig } from "src/L1/SuperchainConfig.sol";

// Libraries
import { Storage } from "src/libraries/Storage.sol";
import { EnumerableSet } from "@openzeppelin/contracts/utils/structs/EnumerableSet.sol";
import { Unauthorized } from "src/libraries/errors/CommonErrors.sol";

// Interfaces
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";
import { ISharedLockbox } from "interfaces/L1/ISharedLockbox.sol";
import { IOptimismPortalInterop } from "interfaces/L1/IOptimismPortalInterop.sol";

/// @custom:proxied true
/// @custom:audit none This contracts is not yet audited.
/// @title SuperchainConfigInterop
/// @notice The SuperchainConfig contract is used to manage configuration of global superchain values.
///         The interop version of the contract adds the ability to add dependencies to the dependency set
///         and authorize OptimismPortals to interact with the SharedLockbox.
contract SuperchainConfigInterop is SuperchainConfig {
    using EnumerableSet for EnumerableSet.UintSet;

    /// @notice The address of the cluster manager, which can add a chain to the dependency set.
    ///         It can only be modified by an upgrade.
    bytes32 public constant CLUSTER_MANAGER_SLOT = bytes32(uint256(keccak256("superchainConfig.clusterManager")) - 1);

    /// @notice Emitted when a new dependency is added as part of the dependency set.
    /// @param chainId      The chain ID.
    /// @param systemConfig The address of the SystemConfig contract.
    /// @param portal       The address of the OptimismPortal contract.
    event DependencyAdded(uint256 indexed chainId, address indexed systemConfig, address indexed portal);

    /// @notice Thrown when the dependency set is too large to add a new dependency.
    error DependencySetTooLarge();

    /// @notice Thrown when the input dependency is already added to the set.
    error DependencyAlreadyAdded();

    /// @notice Thrown when a OptimismPortal does not have the right SuperchainConfig.
    error InvalidSuperchainConfig();

    /// @notice Thrown when trying to add an OptimismPortal that is already authorized.
    error PortalAlreadyAuthorized();

    /// @notice Thrown when the superchain is paused.
    error SuperchainPaused();

    /// @notice Semantic version.
    /// @custom:semver +interop-beta.1
    function version() public pure override returns (string memory) {
        return string.concat(super.version(), "+interop-beta.1");
    }

    /// @notice Storage slot that the SuperchainConfigDependencies struct is stored at.
    /// keccak256(abi.encode(uint256(keccak256("superchainConfig.dependencies")) - 1)) & ~bytes32(uint256(0xff));
    bytes32 internal constant SUPERCHAIN_CONFIG_DEPENDENCIES_SLOT =
        0x342033bc92db70f979584a5299db090f7892d8d8c6e2e81871d9009f08fc2400;

    /// @notice Storage struct for the SuperchainConfig dependencies data.
    /// @custom:storage-location erc7201:superchainConfig.dependencies
    struct SuperchainConfigDependencies {
        /// @notice The Shared Lockbox contract
        ISharedLockbox sharedLockbox;
        /// @notice Dependency set of chains that are part of the same cluster
        EnumerableSet.UintSet dependencySet;
        /// @notice OptimismPortals that are part of the dependency cluster
        mapping(address => bool) authorizedPortals;
    }

    /// @notice Returns the storage for the SuperchainConfigDependencies.
    function _dependenciesStorage() private pure returns (SuperchainConfigDependencies storage storage_) {
        assembly {
            storage_.slot := SUPERCHAIN_CONFIG_DEPENDENCIES_SLOT
        }
    }

    /// @notice Initializer.
    /// @param _guardian             Address of the guardian, can pause the OptimismPortal.
    /// @param _paused               Initial paused status.
    /// @param _clusterManager       Address of the clusterManager, can add a chain to the dependency set.
    /// @param _sharedLockbox        Address of the SharedLockbox contract.
    function initialize(
        address _guardian,
        bool _paused,
        address _clusterManager,
        address _sharedLockbox
    )
        external
        initializer
    {
        _initialize(_guardian, _paused);

        _setClusterManager(_clusterManager);

        SuperchainConfigDependencies storage dependenciesStorage = _dependenciesStorage();
        dependenciesStorage.sharedLockbox = ISharedLockbox(_sharedLockbox);
    }

    /// @notice Getter for the cluster manager address.
    function clusterManager() public view returns (address clusterManager_) {
        clusterManager_ = Storage.getAddress(CLUSTER_MANAGER_SLOT);
    }

    /// @notice Sets the cluster manager address. This is only callable during initialization, so an upgrade
    ///         will be required to change the cluster manager.
    /// @param _clusterManager The new cluster manager address.
    function _setClusterManager(address _clusterManager) internal {
        Storage.setAddress(CLUSTER_MANAGER_SLOT, _clusterManager);
        emit ConfigUpdate(UpdateType.CLUSTER_MANAGER, abi.encode(_clusterManager));
    }

    /// @notice Adds a new dependency to the dependency set. It also authorizes it's OptimismPortal on the
    ///         SharedLockbox and migrate it's ETH liquidity to it. Can only be called by an authorized
    ///         OptimismPortal via a withdrawal transaction initiated by the DependencyManager.
    /// @param _chainId         The chain ID to add.
    /// @param _systemConfig    The SystemConfig contract address of the chain to add.
    function addDependency(uint256 _chainId, address _systemConfig) external {
        if (paused()) revert SuperchainPaused();
        if (msg.sender != clusterManager()) revert Unauthorized();

        SuperchainConfigDependencies storage dependenciesStorage = _dependenciesStorage();

        if (dependenciesStorage.dependencySet.length() == type(uint8).max) revert DependencySetTooLarge();

        // Add to the dependency set and check it is not already added (`add()` returns false if it already exists)
        if (!dependenciesStorage.dependencySet.add(_chainId)) revert DependencyAlreadyAdded();

        address portal = ISystemConfig(_systemConfig).optimismPortal();
        _joinSharedLockbox(portal);

        emit DependencyAdded(_chainId, _systemConfig, portal);
    }

    /// @notice Authorize a portal to interact with the SharedLockbox. It also migrates the ETH liquidity
    ///         from the portal to the SharedLockbox.
    /// @param _portal The address of the portal to authorize.
    function _joinSharedLockbox(address _portal) internal {
        SuperchainConfigDependencies storage dependenciesStorage = _dependenciesStorage();

        if (address(IOptimismPortalInterop(payable(_portal)).superchainConfig()) != address(this)) {
            revert InvalidSuperchainConfig();
        }

        if (dependenciesStorage.authorizedPortals[_portal]) revert PortalAlreadyAuthorized();

        dependenciesStorage.authorizedPortals[_portal] = true;

        // Migrate the ETH liquidity from the OptimismPortal to the SharedLockbox
        IOptimismPortalInterop(payable(_portal)).migrateLiquidity();
    }

    /// @notice Getter for the SharedLockbox contract.
    function sharedLockbox() public view returns (ISharedLockbox sharedLockbox_) {
        sharedLockbox_ = _dependenciesStorage().sharedLockbox;
    }

    /// @notice Checks if a chain is part or not of the dependency set.
    /// @param _chainId The chain ID to check for.
    function isInDependencySet(uint256 _chainId) public view returns (bool) {
        return _dependenciesStorage().dependencySet.contains(_chainId);
    }

    /// @notice Getter for the chain ids list on the dependency set.
    function dependencySet() external view returns (uint256[] memory) {
        return _dependenciesStorage().dependencySet.values();
    }

    /// @notice Returns the size of the dependency set.
    /// @return The size of the dependency set.
    function dependencySetSize() external view returns (uint8) {
        return uint8(_dependenciesStorage().dependencySet.length());
    }

    /// @notice Checks if a portal is authorized to interact with the SharedLockbox.
    /// @param _portal The address of the portal to check for.
    function authorizedPortals(address _portal) public view returns (bool) {
        return _dependenciesStorage().authorizedPortals[_portal];
    }
}
