// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// @title IOptimismERC20Factory
/// @notice Generic interface for IOptimismMintableERC20Factory and ISuperchainERC20Factory. Used to
///         determine if a ERC20 contract is deployed by a factory.
interface IOptimismERC20Factory {
    /// @notice Checks if a ERC20 token is deployed by the factory.
    /// @param _localToken The address of the ERC20 token to check the deployment.
    /// @return remoteToken_ The address of the remote token if it is deployed or `address(0)` if not.
    function deployments(address _localToken) external view returns (address remoteToken_);
}
