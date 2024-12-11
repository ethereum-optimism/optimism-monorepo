// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

type ProtocolVersion is uint256;

interface IProtocolVersions {
    enum UpdateType {
        REQUIRED_PROTOCOL_VERSION,
        RECOMMENDED_PROTOCOL_VERSION
    }

    event ConfigUpdate(uint256 indexed version, UpdateType indexed updateType, bytes data);
    event Initialized(uint8 version);
    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);

    function RECOMMENDED_SLOT() external view returns (bytes32);
    function REQUIRED_SLOT() external view returns (bytes32);
    function VERSION() external view returns (uint256);
    function initialize(address _owner, ProtocolVersion _required, ProtocolVersion _recommended) external;
    function owner() external view returns (address);
    function recommended() external view returns (ProtocolVersion out_);
    function renounceOwnership() external;
    function required() external view returns (ProtocolVersion out_);
    function setRecommended(ProtocolVersion _recommended) external;
    function setRequired(ProtocolVersion _required) external;
    function transferOwnership(address newOwner) external; // nosemgrep
    function version() external view returns (string memory);

    function __constructor__() external;
}
