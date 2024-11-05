// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { BaseDeployIO } from "scripts/deploy/BaseDeployIO.sol";
import { IDataAvailabilityChallenge } from "src/L1/interfaces/IDataAvailabilityChallenge.sol";
import { IProxy } from "src/universal/interfaces/IProxy.sol";
import { Script } from "forge-std/Script.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";
import { IProxyAdmin } from "src/universal/interfaces/IProxyAdmin.sol";
import { Solarray } from "scripts/libraries/Solarray.sol";

contract DeployAltDAInput is BaseDeployIO {
    bytes32 internal _salt;
    IProxyAdmin internal _proxyAdmin;
    address internal _challengeContractOwner;
    uint256 internal _challengeWindow;
    uint256 internal _resolveWindow;
    uint256 internal _bondSize;
    uint256 internal _resolverRefundPercentage;

    function set(bytes4 _sel, bytes32 _val) public {
        if (_sel == this.salt.selector) _salt = _val;
        else revert("DeployAltDAInput: unknown selector");
    }

    function set(bytes4 _sel, address _addr) public {
        require(_addr != address(0), "DeployAltDAInput: cannot set zero address");
        if (_sel == this.proxyAdmin.selector) _proxyAdmin = IProxyAdmin(_addr);
        else if (_sel == this.challengeContractOwner.selector) _challengeContractOwner = _addr;
        else revert("DeployAltDAInput: unknown selector");
    }

    function set(bytes4 _sel, uint256 _val) public {
        if (_sel == this.challengeWindow.selector) _challengeWindow = _val;
        else if (_sel == this.resolveWindow.selector) _resolveWindow = _val;
        else if (_sel == this.bondSize.selector) _bondSize = _val;
        else if (_sel == this.resolverRefundPercentage.selector) _resolverRefundPercentage = _val;
        else revert("DeployAltDAInput: unknown selector");
    }

    function salt() public view returns (bytes32) {
        require(_salt != 0, "DeployAltDAInput: salt not set");
        return _salt;
    }

    function proxyAdmin() public view returns (IProxyAdmin) {
        require(address(_proxyAdmin) != address(0), "DeployAltDAInput: proxyAdmin not set");
        return _proxyAdmin;
    }

    function challengeContractOwner() public view returns (address) {
        require(_challengeContractOwner != address(0), "DeployAltDAInput: challengeContractOwner not set");
        return _challengeContractOwner;
    }

    function challengeWindow() public view returns (uint256) {
        require(_challengeWindow != 0, "DeployAltDAInput: challengeWindow not set");
        return _challengeWindow;
    }

    function resolveWindow() public view returns (uint256) {
        require(_resolveWindow != 0, "DeployAltDAInput: resolveWindow not set");
        return _resolveWindow;
    }

    function bondSize() public view returns (uint256) {
        require(_bondSize != 0, "DeployAltDAInput: bondSize not set");
        return _bondSize;
    }

    function resolverRefundPercentage() public view returns (uint256) {
        require(_resolverRefundPercentage != 0, "DeployAltDAInput: resolverRefundPercentage not set");
        return _resolverRefundPercentage;
    }
}

contract DeployAltDAOutput is BaseDeployIO {
    IDataAvailabilityChallenge internal _dataAvailabilityChallengeProxy;
    IDataAvailabilityChallenge internal _dataAvailabilityChallengeImpl;

    function set(bytes4 _sel, address _addr) public {
        require(_addr != address(0), "DeployAltDAOutput: cannot set zero address");
        if (_sel == this.dataAvailabilityChallengeProxy.selector) {
            _dataAvailabilityChallengeProxy = IDataAvailabilityChallenge(payable(_addr));
        } else if (_sel == this.dataAvailabilityChallengeImpl.selector) {
            _dataAvailabilityChallengeImpl = IDataAvailabilityChallenge(payable(_addr));
        } else {
            revert("DeployAltDAOutput: unknown selector");
        }
    }

    function dataAvailabilityChallengeProxy() public view returns (IDataAvailabilityChallenge) {
        DeployUtils.assertValidContractAddress(address(_dataAvailabilityChallengeProxy));
        return _dataAvailabilityChallengeProxy;
    }

    function dataAvailabilityChallengeImpl() public view returns (IDataAvailabilityChallenge) {
        DeployUtils.assertValidContractAddress(address(_dataAvailabilityChallengeImpl));
        return _dataAvailabilityChallengeImpl;
    }
}

contract DeployAltDA is Script {
    function run(DeployAltDAInput _dai, DeployAltDAOutput _dao) public {
        deployDataAvailabilityChallengeProxy(_dai, _dao);
        deployDataAvailabilityChallengeImpl(_dai, _dao);
        initializeDataAvailabilityChallengeProxy(_dai, _dao);

        checkOutput(_dai, _dao);
    }

    function deployDataAvailabilityChallengeProxy(DeployAltDAInput _dai, DeployAltDAOutput _dao) public {
        bytes32 salt = _dai.salt();
        vm.broadcast(msg.sender);
        IProxy proxy = IProxy(
            DeployUtils.create2({
                _name: "Proxy",
                _salt: salt,
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IProxy.__constructor__, (msg.sender)))
            })
        );
        vm.label(address(proxy), "DataAvailabilityChallengeProxy");
        _dao.set(_dao.dataAvailabilityChallengeProxy.selector, address(proxy));
    }

    function deployDataAvailabilityChallengeImpl(DeployAltDAInput _dai, DeployAltDAOutput _dao) public {
        bytes32 salt = _dai.salt();
        vm.broadcast(msg.sender);
        IDataAvailabilityChallenge impl = IDataAvailabilityChallenge(
            DeployUtils.create2({
                _name: "DataAvailabilityChallenge",
                _salt: salt,
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IDataAvailabilityChallenge.__constructor__, ()))
            })
        );
        vm.label(address(impl), "DataAvailabilityChallengeImpl");
        _dao.set(_dao.dataAvailabilityChallengeImpl.selector, address(impl));
    }

    function initializeDataAvailabilityChallengeProxy(DeployAltDAInput _dai, DeployAltDAOutput _dao) public {
        IProxy proxy = IProxy(payable(address(_dao.dataAvailabilityChallengeProxy())));
        IDataAvailabilityChallenge impl = _dao.dataAvailabilityChallengeImpl();
        IProxyAdmin proxyAdmin = IProxyAdmin(payable(address(_dai.proxyAdmin())));

        address contractOwner = _dai.challengeContractOwner();
        uint256 challengeWindow = _dai.challengeWindow();
        uint256 resolveWindow = _dai.resolveWindow();
        uint256 bondSize = _dai.bondSize();
        uint256 resolverRefundPercentage = _dai.resolverRefundPercentage();

        vm.startBroadcast(msg.sender);
        proxy.upgradeToAndCall(
            address(impl),
            abi.encodeCall(
                IDataAvailabilityChallenge.initialize,
                (contractOwner, challengeWindow, resolveWindow, bondSize, resolverRefundPercentage)
            )
        );
        proxy.changeAdmin(address(proxyAdmin));
        vm.stopBroadcast();
    }

    function checkOutput(DeployAltDAInput _dai, DeployAltDAOutput _dao) public {
        address[] memory addresses = Solarray.addresses(
            address(_dao.dataAvailabilityChallengeProxy()), address(_dao.dataAvailabilityChallengeImpl())
        );
        DeployUtils.assertValidContractAddresses(addresses);

        assertValidDataAvailabilityChallengeProxy(_dai, _dao);
        assertValidDataAvailabilityChallengeImpl(_dao);
    }

    function assertValidDataAvailabilityChallengeProxy(DeployAltDAInput _dai, DeployAltDAOutput _dao) public {
        DeployUtils.assertERC1967ImplementationSet(address(_dao.dataAvailabilityChallengeProxy()));

        IProxy proxy = IProxy(payable(address(_dao.dataAvailabilityChallengeProxy())));
        vm.prank(address(0));
        address admin = proxy.admin();
        require(admin == address(_dai.proxyAdmin()), "DACP-10");

        DeployUtils.assertInitialized({ _contractAddress: address(proxy), _slot: 0, _offset: 0 });

        vm.prank(address(0));
        address impl = proxy.implementation();
        require(impl == address(_dao.dataAvailabilityChallengeImpl()), "DACP-20");

        IDataAvailabilityChallenge dac = _dao.dataAvailabilityChallengeProxy();
        require(dac.owner() == _dai.challengeContractOwner(), "DACP-30");
        require(dac.challengeWindow() == _dai.challengeWindow(), "DACP-40");
        require(dac.resolveWindow() == _dai.resolveWindow(), "DACP-50");
        require(dac.bondSize() == _dai.bondSize(), "DACP-60");
        require(dac.resolverRefundPercentage() == _dai.resolverRefundPercentage(), "DACP-70");
    }

    function assertValidDataAvailabilityChallengeImpl(DeployAltDAOutput _dao) public view {
        IDataAvailabilityChallenge dac = _dao.dataAvailabilityChallengeImpl();
        DeployUtils.assertInitialized({ _contractAddress: address(dac), _slot: 0, _offset: 0 });
    }
}
