// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// @title StaticConfig
/// @notice Library for encoding and decoding static configuration data.
library StaticConfig {
    /// @notice Encodes the static configuration data for setting a gas paying token.
    /// @param _token    Address of the gas paying token.
    /// @param _decimals Number of decimals for the gas paying token.
    /// @param _name     Name of the gas paying token.
    /// @param _symbol   Symbol of the gas paying token.
    /// @return Encoded static configuration data.
    function encodeSetGasPayingToken(
        address _token,
        uint8 _decimals,
        bytes32 _name,
        bytes32 _symbol
    )
        internal
        pure
        returns (bytes memory)
    {
        return abi.encode(_token, _decimals, _name, _symbol);
    }

    /// @notice Decodes the static configuration data for setting a gas paying token.
    /// @param _data Encoded static configuration data.
    /// @return Decoded gas paying token data (token address, decimals, name, symbol).
    function decodeSetGasPayingToken(bytes memory _data) internal pure returns (address, uint8, bytes32, bytes32) {
        return abi.decode(_data, (address, uint8, bytes32, bytes32));
    }

    /// @notice Encodes the static configuration data for adding a dependency.
    /// @param _chainId Chain ID of the dependency to add.
    /// @return Encoded static configuration data.
    function encodeAddDependency(uint256 _chainId) internal pure returns (bytes memory) {
        return abi.encode(_chainId);
    }

    /// @notice Decodes the static configuration data for adding a dependency.
    /// @param _data Encoded static configuration data.
    /// @return Decoded chain ID of the dependency to add.
    function decodeAddDependency(bytes memory _data) internal pure returns (uint256) {
        return abi.decode(_data, (uint256));
    }

    /// @notice Encodes the static configuration data for removing a dependency.
    /// @param _chainId Chain ID of the dependency to remove.
    /// @return Encoded static configuration data.
    function encodeRemoveDependency(uint256 _chainId) internal pure returns (bytes memory) {
        return abi.encode(_chainId);
    }

    /// @notice Decodes the static configuration data for removing a dependency.
    /// @param _data Encoded static configuration data.
    /// @return Decoded chain ID of the dependency to remove.
    function decodeRemoveDependency(bytes memory _data) internal pure returns (uint256) {
        return abi.decode(_data, (uint256));
    }
}
