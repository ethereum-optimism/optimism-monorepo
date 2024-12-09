// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// @title IDeployerWhitelist
/// @notice Interface for the DeployerWhitelist contract.
interface IDeployerWhitelist {
    event OwnerChanged(address oldOwner, address newOwner);
    event WhitelistDisabled(address oldOwner);
    event WhitelistStatusChanged(address deployer, bool whitelisted);

    function enableArbitraryContractDeployment() external;
    function isDeployerAllowed(address _deployer) external view returns (bool);
    function owner() external view returns (address);
    function setOwner(address _owner) external;
    function setWhitelistedDeployer(address _deployer, bool _isWhitelisted) external;
    function version() external view returns (string memory);
    function whitelist(address) external view returns (bool);

    function __constructor__() external;
}
