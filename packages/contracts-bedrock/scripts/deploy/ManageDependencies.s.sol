// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { Script } from "forge-std/Script.sol";
import { BaseDeployIO } from "scripts/deploy/BaseDeployIO.sol";
import { ISuperchainConfigInterop } from "interfaces/L1/ISuperchainConfigInterop.sol";
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";

contract ManageDependenciesInput is BaseDeployIO {
    uint256 internal _chainId;
    ISystemConfig _systemConfig;
    ISuperchainConfigInterop _superchainConfig;

    // Setter for uint256 type
    function set(bytes4 _sel, uint256 _value) public {
        if (_sel == this.chainId.selector) _chainId = _value;
        else revert("ManageDependenciesInput: unknown selector");
    }

    // Setter for address type
    function set(bytes4 _sel, address _addr) public {
        require(_addr != address(0), "ManageDependenciesInput: cannot set zero address");

        if (_sel == this.superchainConfig.selector) _superchainConfig = ISuperchainConfigInterop(_addr);
        else if (_sel == this.systemConfig.selector) _systemConfig = ISystemConfig(_addr);
        else revert("ManageDependenciesInput: unknown selector");
    }

    // Getters
    function chainId() public view returns (uint256) {
        require(_chainId > 0, "ManageDependenciesInput: not set");
        return _chainId;
    }

    function superchainConfig() public view returns (ISuperchainConfigInterop) {
        require(address(_superchainConfig) != address(0), "ManageDependenciesInput: not set");
        return _superchainConfig;
    }

    function systemConfig() public view returns (ISystemConfig) {
        require(address(_systemConfig) != address(0), "ManageDependenciesInput: not set");
        return _systemConfig;
    }
}

contract ManageDependencies is Script {
    function run(ManageDependenciesInput _input) public {
        uint256 chainId = _input.chainId();
        ISuperchainConfigInterop superchainConfig = _input.superchainConfig();
        ISystemConfig systemConfig = _input.systemConfig();

        vm.broadcast(msg.sender);
        superchainConfig.addDependency(chainId, address(systemConfig));
    }
}
