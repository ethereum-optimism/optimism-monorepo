// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Interfaces
import { ISemver } from "interfaces/universal/ISemver.sol";

/// @custom:legacy true
/// @custom:proxied true
/// @custom:predeployed 0x4200000000000000000000000000000000000002
/// @title DeployerWhitelist
/// @notice DeployerWhitelist is a legacy contract that was originally used to act as a whitelist of
///         addresses allowed to the Optimism network. The DeployerWhitelist has since been
///         disabled, but the code is kept in state for the sake of full backwards compatibility.
///         As of the Bedrock upgrade, the DeployerWhitelist is completely unused by the Optimism
///         system and could, in theory, be removed entirely.
contract DeployerWhitelist is ISemver {
    /// @notice Address of the owner of this contract. Note that when this address is set to
    ///         address(0), the whitelist is disabled.
    address public owner;

    /// @notice Mapping of deployer addresses to boolean whitelist status.
    mapping(address => bool) public whitelist;

    /// @notice Emitted when the owner of this contract changes.
    /// @param oldOwner Address of the previous owner.
    /// @param newOwner Address of the new owner.
    event OwnerChanged(address oldOwner, address newOwner);

    /// @notice Emitted when the whitelist status of a deployer changes.
    /// @param deployer    Address of the deployer.
    /// @param whitelisted Boolean indicating whether the deployer is whitelisted.
    event WhitelistStatusChanged(address deployer, bool whitelisted);

    /// @notice Emitted when the whitelist is disabled.
    /// @param oldOwner Address of the final owner of the whitelist.
    event WhitelistDisabled(address oldOwner);

    /// @notice Blocks functions to anyone except the contract owner.
    modifier onlyOwner() {
        require(msg.sender == owner, "DeployerWhitelist: function can only be called by the owner of this contract");
        _;
    }

    /// @notice Semantic version.
    /// @custom:semver 1.1.1-beta.3
    string public constant version = "1.1.1-beta.3";

    /// @notice Adds or removes an address from the deployment whitelist.
    /// @param _deployer      Address to update permissions for.
    /// @param _isWhitelisted Whether or not the address is whitelisted.
    function setWhitelistedDeployer(address _deployer, bool _isWhitelisted) external onlyOwner {
        whitelist[_deployer] = _isWhitelisted;
        emit WhitelistStatusChanged(_deployer, _isWhitelisted);
    }

    /// @notice Updates the owner of this contract.
    /// @param _owner Address of the new owner.
    function setOwner(address _owner) external onlyOwner {
        // Prevent users from setting the whitelist owner to address(0) except via
        // enableArbitraryContractDeployment. If you want to burn the whitelist owner, send it to
        // any other address that doesn't have a corresponding knowable private key.
        require(_owner != address(0), "DeployerWhitelist: can only be disabled via enableArbitraryContractDeployment");

        emit OwnerChanged(owner, _owner);
        owner = _owner;
    }

    /// @notice Permanently enables arbitrary contract deployment and deletes the owner.
    function enableArbitraryContractDeployment() external onlyOwner {
        emit WhitelistDisabled(owner);
        owner = address(0);
    }

    /// @notice Checks whether an address is allowed to deploy contracts.
    /// @param _deployer Address to check.
    /// @return Whether or not the address can deploy contracts.
    function isDeployerAllowed(address _deployer) external view returns (bool) {
        return (owner == address(0) || whitelist[_deployer]);
    }
}
