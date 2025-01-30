// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { Script } from "forge-std/Script.sol";
import { BaseDeployIO } from "scripts/deploy/BaseDeployIO.sol";
import { ISystemConfigInterop } from "interfaces/L1/ISystemConfigInterop.sol";

contract ManageDependenciesInput is BaseDeployIO {
    uint256 internal _chainId;
    ISystemConfigInterop _systemConfig;
    bool internal _remove;

    // Setter for uint256 type
    function set(bytes4 _sel, uint256 _value) public {
        if (_sel == this.chainId.selector) _chainId = _value;
        else revert("ManageDependenciesInput: unknown selector");
    }

    // Setter for address type
    function set(bytes4 _sel, address _addr) public {
        require(_addr != address(0), "ManageDependenciesInput: cannot set zero address");

        if (_sel == this.systemConfig.selector) _systemConfig = ISystemConfigInterop(_addr);
        else revert("ManageDependenciesInput: unknown selector");
    }

    // Setter for bool type
    function set(bytes4 _sel, bool _value) public {
        if (_sel == this.remove.selector) _remove = _value;
        else revert("ManageDependenciesInput: unknown selector");
    }

    // Getters
    function chainId() public view returns (uint256) {
        require(_chainId > 0, "ManageDependenciesInput: not set");
        return _chainId;
    }

    function systemConfig() public view returns (ISystemConfigInterop) {
        require(address(_systemConfig) != address(0), "ManageDependenciesInput: not set");
        return _systemConfig;
    }

    function remove() public view returns (bool) {
        return _remove;
    }
}

contract ManageDependencies is Script {
    function run(ManageDependenciesInput _input) public {
        bool remove = _input.remove();
        uint256 chainId = _input.chainId();
        ISystemConfigInterop systemConfig = _input.systemConfig();

        // Call the appropriate function based on the remove flag
        vm.broadcast(msg.sender);
        if (remove) {
            systemConfig.removeDependency(chainId);
        } else {
            systemConfig.addDependency(chainId);
        }
    }
}
