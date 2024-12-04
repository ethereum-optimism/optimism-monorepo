// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

// Forge
import { Script } from "forge-std/Script.sol";

// Scripts
import { BaseDeployIO } from "scripts/deploy/BaseDeployIO.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

// Interfaces
import { IProxy } from "src/universal/interfaces/IProxy.sol";

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

    function checkOutput(DeployProxyInput _mi) public {
        DeployUtils.assertValidContractAddress(address(_proxy));
        assertValidDeploy(_mi);
    }

    function proxy() public view returns (IProxy) {
        DeployUtils.assertValidContractAddress(address(_proxy));
        return _proxy;
    }

    function assertValidDeploy(DeployProxyInput _mi) public {
        assertValidProxy(_mi);
    }

    function assertValidProxy(DeployProxyInput _mi) internal {
        IProxy prox = proxy();
        vm.prank(_mi.owner());
        address proxyOwner = prox.admin();

        require(
            proxyOwner == _mi.owner(), "DeployProxy: owner of proxy does not match the owner specified in the input"
        );
    }
}

/// @title DeployProxy
contract DeployProxy is Script {
    function run(DeployProxyInput _mi, DeployProxyOutput _mo) public {
        DeployProxySingleton(_mi, _mo);
        _mo.checkOutput(_mi);
    }

    function DeployProxySingleton(DeployProxyInput _mi, DeployProxyOutput _mo) internal {
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
}
