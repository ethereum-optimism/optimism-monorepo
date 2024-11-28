// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { Initializable } from "@openzeppelin/contracts/proxy/utils/Initializable.sol";

// Libraries
import { Storage } from "src/libraries/Storage.sol";
import { EnumerableSet } from "@openzeppelin/contracts/utils/structs/EnumerableSet.sol";
import { Unauthorized } from "src/libraries/errors/CommonErrors.sol";
import { ISystemConfigInterop } from "src/L1/interfaces/ISystemConfigInterop.sol";
import { ISharedLockbox } from "src/L1/interfaces/ISharedLockbox.sol";

// Interfaces
import { ISemver } from "src/universal/interfaces/ISemver.sol";

/// @custom:proxied true
/// @custom:audit none This contracts is not yet audited.
/// @title SuperchainConfig
/// @notice The SuperchainConfig contract is used to manage configuration of global superchain values.
contract SuperchainConfig is Initializable, ISemver {
    using EnumerableSet for EnumerableSet.UintSet;

    /// @notice Enum representing different types of updates.
    /// @custom:value GUARDIAN            Represents an update to the guardian.
    enum UpdateType {
        GUARDIAN
    }

    /// @notice Whether or not the Superchain is paused.
    bytes32 public constant PAUSED_SLOT = bytes32(uint256(keccak256("superchainConfig.paused")) - 1);

    /// @notice The address of the guardian, which can pause withdrawals from the System.
    ///         It can only be modified by an upgrade.
    bytes32 public constant GUARDIAN_SLOT = bytes32(uint256(keccak256("superchainConfig.guardian")) - 1);

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

    /// @notice Emitted when a new chain is added as part of the dependency set.
    /// @param chainId      The chain ID.
    /// @param systemConfig The address of the SystemConfig contract.
    /// @param portal       The address of the OptimismPortal contract.
    event ChainAdded(uint256 indexed chainId, address indexed systemConfig, address indexed portal);

    /// @notice Thrown when the input chain's system config already contains dependencies on its set.
    error ChainAlreadyHasDependencies();

    /// @notice Thrown when the input chain is already added to the dependency set.
    error ChainAlreadyAdded();

    /// @notice Semantic version.
    /// @custom:semver 1.1.1-beta.2
    string public constant version = "1.1.1-beta.2";

    // Mapping from chainId to SystemConfig address
    mapping(uint256 => address) public systemConfigs;

    // Dependency set of chains that are part of the same cluster
    EnumerableSet.UintSet internal _dependencySet;

    /// @notice Constructs the SuperchainConfig contract.
    constructor(address _sharedLockbox) {
        SHARED_LOCKBOX = ISharedLockbox(_sharedLockbox);
        initialize({ _guardian: address(0), _paused: false });
    }

    /// @notice Initializer.
    /// @param _guardian    Address of the guardian, can pause the OptimismPortal.
    /// @param _paused      Initial paused status.
    function initialize(address _guardian, bool _paused) public initializer {
        _setGuardian(_guardian);
        if (_paused) {
            _pause("Initializer paused");
        }
    }

    /// @notice Getter for the guardian address.
    function guardian() public view returns (address guardian_) {
        guardian_ = Storage.getAddress(GUARDIAN_SLOT);
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

    /// @notice Adds a new chain to the dependency set.
    ///         Adds the new chain as a dependency for all existing chains in the dependency set, and vice versa. It
    ///         also stores the SystemConfig address of it, and authorizes the OptimismPortal on the SharedLockbox.
    /// @param _chainId     The chain ID.
    /// @param _systemConfig The SystemConfig contract address of the chain to add.
    function addChain(uint256 _chainId, address _systemConfig) external {
        // TODO: Updater role TBD, using guardian for now.
        if (msg.sender != guardian()) revert Unauthorized();
        if (ISystemConfigInterop(_systemConfig).dependencyCounter() != 0) revert ChainAlreadyHasDependencies();
        // Add to the dependency set and check it is not already added (`add()` returns false if it already exists)
        if (!_dependencySet.add(_chainId)) revert ChainAlreadyAdded();

        systemConfigs[_chainId] = _systemConfig;

        // Loop through the dependency set and update the dependency for each chain. Using length - 1 to exclude the
        // current chain from the loop.
        for (uint256 i; i < _dependencySet.length() - 1; i++) {
            uint256 currentId = _dependencySet.at(i);

            // Add the new chain as dependency for the current chain on the loop
            ISystemConfigInterop(systemConfigs[currentId]).addDependency(_chainId);
            // Add the current chain on the loop as dependency for the new chain
            ISystemConfigInterop(_systemConfig).addDependency(currentId);
        }

        // Authorize the portal on the shared lockbox
        address portal = ISystemConfigInterop(_systemConfig).optimismPortal();
        SHARED_LOCKBOX.authorizePortal(portal);

        emit ChainAdded(_chainId, _systemConfig, portal);
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
}
