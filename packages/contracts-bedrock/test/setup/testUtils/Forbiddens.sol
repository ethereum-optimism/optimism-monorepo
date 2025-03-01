// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

// Ephemeral contract that stores a list of forbidden addresses.
contract ForbiddenAddresses {
    mapping(address => bool) public forbiddenAddresses;

    // chainable
    function forbid(address _addr) public returns (ForbiddenAddresses) {
        forbiddenAddresses[_addr] = true;
        return this;
    }
}

// Ephemeral contract that stores a list of forbidden uint256 values.
contract ForbiddenUints {
    mapping(uint256 => bool) public forbiddenUints;

    // chainable
    function forbid(uint256 _value) public returns (ForbiddenUints) {
        forbiddenUints[_value] = true;
        return this;
    }
}

// Ephemeral contract that stores a list of forbidden int256 values.
contract ForbiddenInts {
    mapping(int256 => bool) public forbiddenInts;

    // chainable
    function forbid(int256 _value) public returns (ForbiddenInts) {
        forbiddenInts[_value] = true;
        return this;
    }
}
