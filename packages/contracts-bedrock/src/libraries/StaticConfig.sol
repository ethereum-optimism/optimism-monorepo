// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { Types } from "src/libraries/Types.sol";

// The main benefit of this library is to give the reader the ability to observe that
// the encode decode logic is correct by being able to see it right next to each other.
// Otherwise the encode/decode will exist in many different locations, between mocks and
// runtime code. It is overly verbose and I don't like it.

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

    /// @notice Encodes the static configuration data for setting a fee vault config.
    /// @param _recipient Address of the recipient of the fee vault.
    /// @param _min Minimum withdrawal amount allowed to be processed.
    /// @param _network The network in which the fees should be withdrawn to.
    /// @return Encoded static configuration data.
    function encodeSetFeeVaultConfig(
        address _recipient,
        uint256 _min,
        Types.WithdrawalNetwork _network
    )
        internal
        pure
        returns (bytes memory)
    {
        return abi.encode(_recipient, _min, _network);
    }

    /// @notice Decodes the static configuration data for setting a fee vault config.
    /// @param _data Encoded static configuration data.
    /// @return Decoded fee vault config data (recipient, min, network).
    function decodeSetFeeVaultConfig(bytes memory _data)
        internal
        pure
        returns (address, uint256, Types.WithdrawalNetwork)
    {
        return abi.decode(_data, (address, uint256, Types.WithdrawalNetwork));
    }

    /// @notice Encodes the static configuration data for setting an address.
    /// @param _address Address to set.
    /// @return Encoded static configuration data.
    function encodeSetAddress(address _address) internal pure returns (bytes memory) {
        return abi.encode(_address);
    }

    /// @notice Decodes the static configuration data for setting an address.
    /// @param _data Encoded static configuration data.
    /// @return Decoded address.
    function decodeSetAddress(bytes memory _data) internal pure returns (address) {
        return abi.decode(_data, (address));
    }

    /// @notice Encodes the static configuration data for setting a remote chain ID.
    /// @param _chainId Chain ID of the remote chain.
    /// @return Encoded static configuration data.
    function encodeSetRemoteChainId(uint256 _chainId) internal pure returns (bytes memory) {
        return abi.encode(_chainId);
    }

    /// @notice Decodes the static configuration data for setting a remote chain ID.
    /// @param _data Encoded static configuration data.
    /// @return Decoded chain ID of the remote chain.
    function decodeSetRemoteChainId(bytes memory _data) internal pure returns (uint256) {
        return abi.decode(_data, (uint256));
    }
}
