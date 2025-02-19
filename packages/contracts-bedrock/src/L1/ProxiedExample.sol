// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { Initializable } from "@openzeppelin/contracts/proxy/utils/Initializable.sol";

// Libraries
import { Storage } from "src/libraries/Storage.sol";

// Interfaces
import { ISemver } from "interfaces/universal/ISemver.sol";

/// @custom:proxied true
/// @custom:audit none This contracts is not yet audited.
/// Note the two tags above mentioning the audit status and whether the contract is proxied.
/// @title SingletonExample
/// @notice The SingletonExample contract is a test of what is required for an L1 contract.
contract ProxiedExample is Initializable, ISemver {
    /// We always implement ISemver for all contracts. We use OZ Initializable for upgradable contracts.
    /// @notice Enum representing different types of updates.
    /// @custom:value GUARDIAN            Represents an update to the guardian.
    enum UpdateType {
        GUARDIAN
    }
    /// For each setter function, add an enum value.

    /// @notice The address of the guardian, which can pause withdrawals from the System.
    ///         It can only be modified by an upgrade.
    bytes32 public constant GUARDIAN_SLOT = bytes32(uint256(keccak256("superchainConfig.guardian")) - 1);
    /// We allocate storage slots ourselves to avoid collisions during upgrades.

    /// @notice A mapping of keys to values. We use @notice to document code elements.
    mapping(string => string) public _stored;
    /// For mappings, we use the storage slot directly, since it is never used directly for the mapping itself.

    /// @notice Emitted when a value is stored.
    /// @param key The key of the value stored.
    /// @param value The value stored.
    event Stored(string key, string value);

    /// @notice Emitted when configuration is updated.
    /// @param updateType Type of update.
    /// @param data       Encoded update data.
    event ConfigUpdate(UpdateType indexed updateType, bytes data);
    /// We use a single event for all setters, indexed by the update type.

    /// @notice Semantic version.
    /// @custom:semver 1.2.0
    string public constant version = "1.2.0";
    /// All individual contracts are versioned. Note the @custom:semver tag.

    /// @notice Constructs the ProxiedExample contract.
    constructor() {
        _disableInitializers();
    }

    /// @notice Initializer.
    /// @param _guardian    Address of the guardian, can store values.
    function initialize(address _guardian) external initializer {
        _setGuardian(_guardian);
    }

    /// @notice Getter for the guardian address.
    function guardian() public view returns (address guardian_) {
        guardian_ = Storage.getAddress(GUARDIAN_SLOT);
    }

    /// @notice Store a value in the registry.
    /// @param _key The key of the value to store.
    /// @param _value The value to store.
    function store(string memory _key, string memory _value) public {
        require(msg.sender == guardian(), "SingletonExample: only guardian can store");
        _stored[_key] = _value;
        emit Stored(_key, _value);
    }

    /// @notice Read a value from the registry.
    /// @param _key The key of the value to get.
    /// @return value_ The value stored.
    function read(string memory _key) public view returns (string memory value_) {
        value_ = _stored[_key];
    }
    /// Note that input arguments are preceded by underscores, and output arguments followed.

    /// @notice Sets the guardian address. This is only callable during initialization, so an upgrade
    ///         will be required to change the guardian.
    /// @param guardian_ The new guardian address.
    function _setGuardian(address guardian_) internal {
        Storage.setAddress(GUARDIAN_SLOT, guardian_);
        emit ConfigUpdate(UpdateType.GUARDIAN, abi.encode(guardian_));
    }
}
