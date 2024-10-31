// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

// Forge
import { Script } from "forge-std/Script.sol";

// Scripts
import { BaseDeployIO } from "scripts/deploy/BaseDeployIO.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

// Libraries
import { LibString } from "@solady/utils/LibString.sol";

// Interfaces
import { IProxy } from "src/universal/interfaces/IProxy.sol";
import { IDelayedWETH } from "src/dispute/interfaces/IDelayedWETH.sol";
import { ISuperchainConfig } from "src/L1/interfaces/ISuperchainConfig.sol";

/// @title DeployDelayedWETH
contract DeployDelayedWETHInput is BaseDeployIO {
    /// Required inputs.
    string internal _release;
    string internal _standardVersionsToml;
    address public _proxyAdmin;
    ISuperchainConfig public _superchainConfigProxy;
    address public _delayedWethOwner;
    uint256 public _delayedWethDelay;

    function set(bytes4 _sel, uint256 _value) public {
        if (_sel == this.delayedWethDelay.selector) {
            require(_value != 0, "DeployDelayedWETH: delayedWethDelay cannot be zero");
            _delayedWethDelay = _value;
        } else {
            revert("DeployDelayedWETH: unknown selector");
        }
    }

    function set(bytes4 _sel, address _value) public {
        if (_sel == this.proxyAdmin.selector) {
            require(_value != address(0), "DeployDelayedWETH: proxyAdmin cannot be zero address");
            _proxyAdmin = _value;
        } else if (_sel == this.superchainConfigProxy.selector) {
            require(_value != address(0), "DeployDelayedWETH: superchainConfigProxy cannot be zero address");
            _superchainConfigProxy = ISuperchainConfig(_value);
        } else if (_sel == this.delayedWethOwner.selector) {
            require(_value != address(0), "DeployDelayedWETH: delayedWethOwner cannot be zero address");
            _delayedWethOwner = _value;
        } else {
            revert("DeployDelayedWETH: unknown selector");
        }
    }

    function set(bytes4 _sel, string memory _value) public {
        if (_sel == this.release.selector) {
            require(!LibString.eq(_value, ""), "DeployDelayedWETH: release cannot be empty");
            _release = _value;
        } else if (_sel == this.standardVersionsToml.selector) {
            require(!LibString.eq(_value, ""), "DeployDelayedWETH: standardVersionsToml cannot be empty");
            _standardVersionsToml = _value;
        } else {
            revert("DeployDelayedWETH: unknown selector");
        }
    }

    function release() public view returns (string memory) {
        require(!LibString.eq(_release, ""), "DeployDelayedWETH: release not set");
        return _release;
    }

    function standardVersionsToml() public view returns (string memory) {
        require(!LibString.eq(_standardVersionsToml, ""), "DeployDelayedWETH: standardVersionsToml not set");
        return _standardVersionsToml;
    }

    function proxyAdmin() public view returns (address) {
        require(_proxyAdmin != address(0), "DeployDelayedWETH: proxyAdmin not set");
        return _proxyAdmin;
    }

    function superchainConfigProxy() public view returns (ISuperchainConfig) {
        require(address(_superchainConfigProxy) != address(0), "DeployDisputeGame: superchainConfigProxy not set");
        return _superchainConfigProxy;
    }

    function delayedWethOwner() public view returns (address) {
        require(_delayedWethOwner != address(0), "DeployDelayedWETH: delayedWethOwner not set");
        return _delayedWethOwner;
    }

    function delayedWethDelay() public view returns (uint256) {
        require(_delayedWethDelay != 0, "DeployDelayedWETH: delayedWethDelay not set");
        return _delayedWethDelay;
    }
}

/// @title DeployDelayedWETHOutput
contract DeployDelayedWETHOutput is BaseDeployIO {
    IDelayedWETH internal _delayedWethImpl;
    IDelayedWETH internal _delayedWethProxy;

    function set(bytes4 _sel, address _value) public {
        if (_sel == this.delayedWethImpl.selector) {
            require(_value != address(0), "DeployDelayedWETHOutput: delayedWethImpl cannot be zero address");
            _delayedWethImpl = IDelayedWETH(payable(_value));
        } else if (_sel == this.delayedWethProxy.selector) {
            require(_value != address(0), "DeployDelayedWETHOutput: delayedWethProxy cannot be zero address");
            _delayedWethProxy = IDelayedWETH(payable(_value));
        } else {
            revert("DeployDelayedWETHOutput: unknown selector");
        }
    }

    function checkOutput(DeployDelayedWETHInput _dwi) public {
        DeployUtils.assertValidContractAddress(address(_delayedWethImpl));
        DeployUtils.assertValidContractAddress(address(_delayedWethProxy));
        assertValidDeploy(_dwi);
    }

    function delayedWethImpl() public view returns (IDelayedWETH) {
        DeployUtils.assertValidContractAddress(address(_delayedWethImpl));
        return _delayedWethImpl;
    }

    function delayedWethProxy() public view returns (IDelayedWETH) {
        DeployUtils.assertValidContractAddress(address(_delayedWethProxy));
        return _delayedWethProxy;
    }

    function assertValidDeploy(DeployDelayedWETHInput _dwi) public {
        assertValidDelayedWethImpl(_dwi);
        assertValidDelayedWethProxy(_dwi);
    }

    function assertValidDelayedWethImpl(DeployDelayedWETHInput _dwi) internal {
        IProxy proxy = IProxy(payable(address(delayedWethProxy())));
        vm.prank(address(0));
        address impl = proxy.implementation();
        require(impl == address(delayedWethImpl()), "DWI-10");
        DeployUtils.assertInitialized({ _contractAddress: address(delayedWethImpl()), _slot: 0, _offset: 0 });
        require(delayedWethImpl().owner() == address(0), "DWI-20");
        require(delayedWethImpl().delay() == _dwi.delayedWethDelay(), "DWI-30");
        require(address(delayedWethImpl().config()) == address(0), "DWI-30");
    }

    function assertValidDelayedWethProxy(DeployDelayedWETHInput _dwi) internal {
        // Check as proxy.
        IProxy proxy = IProxy(payable(address(delayedWethProxy())));
        vm.prank(address(0));
        address admin = proxy.admin();
        require(admin == _dwi.proxyAdmin(), "DWP-10");

        // Check as implementation.
        DeployUtils.assertInitialized({ _contractAddress: address(delayedWethProxy()), _slot: 0, _offset: 0 });
        require(delayedWethProxy().owner() == _dwi.delayedWethOwner(), "DWP-20");
        require(delayedWethProxy().delay() == _dwi.delayedWethDelay(), "DWP-30");
        require(delayedWethProxy().config() == _dwi.superchainConfigProxy(), "DWP-40");
    }
}

/// @title DeployDelayedWETH
contract DeployDelayedWETH is Script {
    function run(DeployDelayedWETHInput _dwi, DeployDelayedWETHOutput _dwo) public {
        deployDelayedWethProxy(_dwi, _dwo);
        _dwo.checkOutput(_dwi);
    }

    function deployDelayedWethImpl(DeployDelayedWETHInput _dwi, DeployDelayedWETHOutput _dwo) internal {
        string memory release = _dwi.release();
        string memory stdVerToml = _dwi.standardVersionsToml();
        string memory contractName = "delayed_weth";
        IDelayedWETH impl;

        address existingImplementation = getReleaseAddress(release, contractName, stdVerToml);
        if (existingImplementation != address(0)) {
            impl = IDelayedWETH(payable(existingImplementation));
        } else if (isDevelopRelease(release)) {
            vm.broadcast(msg.sender);
            impl = IDelayedWETH(
                DeployUtils.create1({
                    _name: "DelayedWETH",
                    _args: DeployUtils.encodeConstructor(
                        abi.encodeCall(IDelayedWETH.__constructor__, (_dwi.delayedWethDelay()))
                    )
                })
            );
        } else {
            revert(string.concat("DeployDelayedWETH: failed to deploy release ", release));
        }

        vm.label(address(impl), "DelayedWETHImpl");
        _dwo.set(_dwo.delayedWethImpl.selector, address(impl));
    }

    function deployDelayedWethProxy(DeployDelayedWETHInput _dwi, DeployDelayedWETHOutput _dwo) internal {
        vm.broadcast(msg.sender);
        IProxy proxy = IProxy(
            DeployUtils.create1({
                _name: "Proxy",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IProxy.__constructor__, (msg.sender)))
            })
        );

        deployDelayedWethImpl(_dwi, _dwo);
        IDelayedWETH impl = _dwo.delayedWethImpl();

        vm.startBroadcast(msg.sender);
        proxy.upgradeToAndCall(
            address(impl), abi.encodeCall(impl.initialize, (_dwi.delayedWethOwner(), _dwi.superchainConfigProxy()))
        );
        proxy.changeAdmin(_dwi.proxyAdmin());
        vm.stopBroadcast();

        vm.label(address(proxy), "DelayedWETHProxy");
        _dwo.set(_dwo.delayedWethProxy.selector, address(proxy));
    }

    // Zero address is returned if the address is not found in '_standardVersionsToml'.
    function getReleaseAddress(
        string memory _version,
        string memory _contractName,
        string memory _standardVersionsToml
    )
        internal
        pure
        returns (address addr_)
    {
        string memory baseKey = string.concat('.releases["', _version, '"].', _contractName);
        string memory implAddressKey = string.concat(baseKey, ".implementation_address");
        string memory addressKey = string.concat(baseKey, ".address");
        try vm.parseTomlAddress(_standardVersionsToml, implAddressKey) returns (address parsedAddr_) {
            addr_ = parsedAddr_;
        } catch {
            try vm.parseTomlAddress(_standardVersionsToml, addressKey) returns (address parsedAddr_) {
                addr_ = parsedAddr_;
            } catch {
                addr_ = address(0);
            }
        }
    }

    // A release is considered a 'develop' release if it does not start with 'op-contracts'.
    function isDevelopRelease(string memory _release) internal pure returns (bool) {
        return !LibString.startsWith(_release, "op-contracts");
    }
}
