// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { OwnableUpgradeable } from "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";

// Libraries
import { Storage } from "src/libraries/Storage.sol";

// Interfaces
import { ISemver } from "interfaces/universal/ISemver.sol";

/// @notice ProtocolVersion is a numeric identifier of the protocol version.
type ProtocolVersion is uint256;

/// @custom:proxied true
/// @title ProtocolVersions
/// @notice The ProtocolVersions contract is used to manage superchain protocol version information.
contract ProtocolVersions is OwnableUpgradeable, ISemver {
    /// @notice Enum representing different types of updates.
    /// @custom:value REQUIRED_PROTOCOL_VERSION              Represents an update to the required protocol version.
    /// @custom:value RECOMMENDED_PROTOCOL_VERSION           Represents an update to the recommended protocol version.
    enum UpdateType {
        REQUIRED_PROTOCOL_VERSION,
        RECOMMENDED_PROTOCOL_VERSION
    }

    /// @notice Version identifier, used for upgrades.
    uint256 public constant VERSION = 0;

    /// @notice Storage slot that the required protocol version is stored at.
    bytes32 public constant REQUIRED_SLOT = bytes32(uint256(keccak256("protocolversion.required")) - 1);

    /// @notice Storage slot that the recommended protocol version is stored at.
    bytes32 public constant RECOMMENDED_SLOT = bytes32(uint256(keccak256("protocolversion.recommended")) - 1);

    /// @notice Emitted when configuration is updated.
    /// @param version    ProtocolVersion version.
    /// @param updateType Type of update.
    /// @param data       Encoded update data.
    event ConfigUpdate(uint256 indexed version, UpdateType indexed updateType, bytes data);

    /// @notice Semantic version.
    /// @custom:semver 1.1.0
    string public constant version = "1.1.0";

    /// @notice Constructs the ProtocolVersion contract.
    constructor() {
        _disableInitializers();
    }

    /// @notice Initializer.
    /// @param _owner             Initial owner of the contract.
    /// @param _required          Required protocol version to operate on this chain.
    /// @param _recommended       Recommended protocol version to operate on thi chain.
    function initialize(address _owner, ProtocolVersion _required, ProtocolVersion _recommended) external initializer {
        __Ownable_init();
        transferOwnership(_owner);
        _setRequired(_required);
        _setRecommended(_recommended);
    }

    /// @notice High level getter for the required protocol version.
    /// @return out_ Required protocol version to sync to the head of the chain.
    function required() external view returns (ProtocolVersion out_) {
        out_ = ProtocolVersion.wrap(Storage.getUint(REQUIRED_SLOT));
    }

    /// @notice Updates the required protocol version. Can only be called by the owner.
    /// @param _required New required protocol version.
    function setRequired(ProtocolVersion _required) external onlyOwner {
        _setRequired(_required);
    }

    /// @notice Internal function for updating the required protocol version.
    /// @param _required New required protocol version.
    function _setRequired(ProtocolVersion _required) internal {
        Storage.setUint(REQUIRED_SLOT, ProtocolVersion.unwrap(_required));

        bytes memory data = abi.encode(_required);
        emit ConfigUpdate(VERSION, UpdateType.REQUIRED_PROTOCOL_VERSION, data);
    }

    /// @notice High level getter for the recommended protocol version.
    /// @return out_ Recommended protocol version to sync to the head of the chain.
    function recommended() external view returns (ProtocolVersion out_) {
        out_ = ProtocolVersion.wrap(Storage.getUint(RECOMMENDED_SLOT));
    }

    /// @notice Updates the recommended protocol version. Can only be called by the owner.
    /// @param _recommended New recommended protocol version.
    function setRecommended(ProtocolVersion _recommended) external onlyOwner {
        _setRecommended(_recommended);
    }

    /// @notice Internal function for updating the recommended protocol version.
    /// @param _recommended New recommended protocol version.
    function _setRecommended(ProtocolVersion _recommended) internal {
        Storage.setUint(RECOMMENDED_SLOT, ProtocolVersion.unwrap(_recommended));

        bytes memory data = abi.encode(_recommended);
        emit ConfigUpdate(VERSION, UpdateType.RECOMMENDED_PROTOCOL_VERSION, data);
    }
}
