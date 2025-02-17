// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// Note the interface is not inherited by the relevant contract. We just make sure it matches.
interface IDummyRegistry {
    /// Note we repeat enums and events.
    enum UpdateType {
        GUARDIAN
    }

    event Stored(string key, string value);
    event ConfigUpdate(UpdateType indexed updateType, bytes data);
    event Initialized(uint8 version);

    function GUARDIAN_SLOT() external view returns (bytes32);
    function guardian() external view returns (address guardian_);
    function initialize(address _guardian) external;
    function version() external view returns (string memory);
    function store(string memory _key, string memory _value) external;
    function read(string memory _key) external view returns (string memory value_);

    function __constructor__() external;
}
