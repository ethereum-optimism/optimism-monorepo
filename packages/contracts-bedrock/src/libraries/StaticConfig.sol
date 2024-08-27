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

    /// @notice Encodes the static configuration data for setting batcher hash.
    /// @param _batcherHash New batcher hash.
    /// @return Encoded static configuration data.
    function encodeSetBatcherHash(bytes32 _batcherHash) internal pure returns (bytes memory) {
        return abi.encode(_batcherHash);
    }

    /// @notice Decodes the static configuration data for setting batcher hash.
    /// @param _data Encoded static configuration data.
    /// @return Decoded batcher hash to set.
    function decodeSetBatcherHash(bytes memory _data) internal pure returns (bytes32) {
        return abi.decode(_data, (bytes32));
    }

    /// @notice Encodes the static configuration data for setting the fee scalars.
    /// @param _scalar   New scalar value.
    /// @return Encoded static configuration data.
    function encodeSetFeeScalars(uint256 _scalar) internal pure returns (bytes memory) {
        return abi.encode(_scalar);
    }

    /// @notice Decodes the static configuration data for setting the fee scalars.
    /// @param _data Encoded static configuration data.
    /// @return Decoded fee scalars to set.
    function decodeSetFeeScalars(bytes memory _data) internal pure returns (uint256) {
        return abi.decode(_data, (uint256));
    }

    /// @notice Encodes the static configuration data for setting the gas limit.
    /// @param _gasLimit New gas limit.
    function encodeSetGasLimit(uint64 _gasLimit) internal pure returns (bytes memory) {
        return abi.encode(_gasLimit);
    }

    /// @notice Decodes the static configuration data for setting the gas limit.
    /// @param _data Encoded static configuration data.
    /// @return Decoded gas limit to set.
    function decodeSetGasLimit(bytes memory _data) internal pure returns (uint64) {
        return abi.decode(_data, (uint64));
    }
}
