// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

// Forge
import { Script } from "forge-std/Script.sol";

// Scripts
import { BaseDeployIO } from "scripts/deploy/BaseDeployIO.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

// Interfaces
import { IProxy } from "interfaces/universal/IProxy.sol";

/// @title DeployProxyInput
contract DeployProxyInput is BaseDeployIO {
    // Specify the owner of the proxy that is being deployed
    address internal _owner;

    function set(bytes4 _sel, address _value) public {
        if (_sel == this.owner.selector) {
            require(_value != address(0), "DeployProxy: owner cannot be empty");
            _owner = _value;
        } else {
            revert("DeployProxy: unknown selector");
        }
    }

    function owner() public view returns (address) {
        require(_owner != address(0), "DeployProxy: owner not set");
        return _owner;
    }
}

/// @title DeployProxyOutput
contract DeployProxyOutput is BaseDeployIO {
    IProxy internal _proxy;

    function set(bytes4 _sel, address _value) public {
        if (_sel == this.proxy.selector) {
            require(_value != address(0), "DeployProxy: proxy cannot be zero address");
            _proxy = IProxy(payable(_value));
        } else {
            revert("DeployProxy: unknown selector");
        }
    }

    function proxy() public view returns (IProxy) {
        DeployUtils.assertValidContractAddress(address(_proxy));
        return _proxy;
    }
}

/// @title DeployProxy
contract DeployProxy is Script {
    function run(DeployProxyInput _mi, DeployProxyOutput _mo) public {
        deployProxySingleton(_mi, _mo);
        checkOutput(_mi, _mo);
    }

    function deployProxySingleton(DeployProxyInput _mi, DeployProxyOutput _mo) internal {
        address owner = _mi.owner();
        vm.broadcast(msg.sender);
        IProxy proxy = IProxy(
            DeployUtils.create1({
                _name: "Proxy",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IProxy.__constructor__, (owner)))
            })
        );

        vm.label(address(proxy), "Proxy");
        _mo.set(_mo.proxy.selector, address(proxy));
    }

    function checkOutput(DeployProxyInput _mi, DeployProxyOutput _mo) public {
        DeployUtils.assertValidContractAddress(address(_mo.proxy()));
        IProxy prox = _mo.proxy();
        vm.prank(_mi.owner());
        address proxyOwner = prox.admin();

        require(
            proxyOwner == _mi.owner(), "DeployProxy: owner of proxy does not match the owner specified in the input"
        );
    }
}
