// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { BaseDeployIO } from "scripts/deploy/BaseDeployIO.sol";
import { IProxy } from "src/universal/interfaces/IProxy.sol";
import { Script } from "forge-std/Script.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";
import { DeployOPChainOutput } from "scripts/deploy/DeployOPChain.s.sol";
import { IMIPS } from "src/cannon/interfaces/IMIPS.sol";
import { OPContractsManager } from "src/L1/OPContractsManager.sol";
import { IAddressManager } from "src/legacy/interfaces/IAddressManager.sol";
import { IStaticL1ChugSplashProxy } from "src/legacy/interfaces/IL1ChugSplashProxy.sol";

contract ReadImplementationAddressesInput is DeployOPChainOutput {
    OPContractsManager internal _opcm;

    function set(bytes4 _sel, address _addr) public override {
        require(_addr != address(0), "ReadImplementationAddressesInput: cannot set zero address");
        if (_sel == this.opcm.selector) _opcm = OPContractsManager(_addr);
        else if (_sel == this.addressManager.selector) _addressManager = IAddressManager(_addr);
        else super.set(_sel, _addr);
    }

    function opcm() public view returns (OPContractsManager) {
        DeployUtils.assertValidContractAddress(address(_opcm));
        return _opcm;
    }
}

contract ReadImplementationAddressesOutput is BaseDeployIO {
    address internal _delayedWETH;
    address internal _optimismPortal;
    address internal _systemConfig;
    address internal _l1CrossDomainMessenger;
    address internal _l1ERC721Bridge;
    address internal _l1StandardBridge;
    address internal _optimismMintableERC20Factory;
    address internal _disputeGameFactory;
    address internal _mipsSingleton;
    address internal _preimageOracleSingleton;

    function set(bytes4 _sel, address _addr) public {
        require(_addr != address(0), "ReadImplementationAddressesOutput: cannot set zero address");
        if (_sel == this.delayedWETH.selector) _delayedWETH = _addr;
        else if (_sel == this.optimismPortal.selector) _optimismPortal = _addr;
        else if (_sel == this.systemConfig.selector) _systemConfig = _addr;
        else if (_sel == this.l1CrossDomainMessenger.selector) _l1CrossDomainMessenger = _addr;
        else if (_sel == this.l1ERC721Bridge.selector) _l1ERC721Bridge = _addr;
        else if (_sel == this.l1StandardBridge.selector) _l1StandardBridge = _addr;
        else if (_sel == this.optimismMintableERC20Factory.selector) _optimismMintableERC20Factory = _addr;
        else if (_sel == this.disputeGameFactory.selector) _disputeGameFactory = _addr;
        else if (_sel == this.mipsSingleton.selector) _mipsSingleton = _addr;
        else if (_sel == this.preimageOracleSingleton.selector) _preimageOracleSingleton = _addr;
        else revert("ReadImplementationAddressesOutput: unknown selector");
    }

    function delayedWETH() public view returns (address) {
        require(_delayedWETH != address(0), "ReadImplementationAddressesOutput: delayedWETH not set");
        return _delayedWETH;
    }

    function optimismPortal() public view returns (address) {
        require(_optimismPortal != address(0), "ReadImplementationAddressesOutput: optimismPortal not set");
        return _optimismPortal;
    }

    function systemConfig() public view returns (address) {
        require(_systemConfig != address(0), "ReadImplementationAddressesOutput: systemConfig not set");
        return _systemConfig;
    }

    function l1CrossDomainMessenger() public view returns (address) {
        require(
            _l1CrossDomainMessenger != address(0), "ReadImplementationAddressesOutput: l1CrossDomainMessenger not set"
        );
        return _l1CrossDomainMessenger;
    }

    function l1ERC721Bridge() public view returns (address) {
        require(_l1ERC721Bridge != address(0), "ReadImplementationAddressesOutput: l1ERC721Bridge not set");
        return _l1ERC721Bridge;
    }

    function l1StandardBridge() public view returns (address) {
        require(_l1StandardBridge != address(0), "ReadImplementationAddressesOutput: l1StandardBridge not set");
        return _l1StandardBridge;
    }

    function optimismMintableERC20Factory() public view returns (address) {
        require(
            _optimismMintableERC20Factory != address(0),
            "ReadImplementationAddressesOutput: optimismMintableERC20Factory not set"
        );
        return _optimismMintableERC20Factory;
    }

    function disputeGameFactory() public view returns (address) {
        require(_disputeGameFactory != address(0), "ReadImplementationAddressesOutput: disputeGameFactory not set");
        return _disputeGameFactory;
    }

    function mipsSingleton() public view returns (address) {
        require(_mipsSingleton != address(0), "ReadImplementationAddressesOutput: mipsSingleton not set");
        return _mipsSingleton;
    }

    function preimageOracleSingleton() public view returns (address) {
        require(
            _preimageOracleSingleton != address(0), "ReadImplementationAddressesOutput: preimageOracleSingleton not set"
        );
        return _preimageOracleSingleton;
    }
}

contract ReadImplementationAddresses is Script {
    function run(ReadImplementationAddressesInput _rii, ReadImplementationAddressesOutput _rio) public {
        address[6] memory eip1967Proxies = [
            address(_rii.delayedWETHPermissionedGameProxy()),
            address(_rii.optimismPortalProxy()),
            address(_rii.systemConfigProxy()),
            address(_rii.l1ERC721BridgeProxy()),
            address(_rii.optimismMintableERC20FactoryProxy()),
            address(_rii.disputeGameFactoryProxy())
        ];

        bytes4[6] memory sels = [
            _rio.delayedWETH.selector,
            _rio.optimismPortal.selector,
            _rio.systemConfig.selector,
            _rio.l1ERC721Bridge.selector,
            _rio.optimismMintableERC20Factory.selector,
            _rio.disputeGameFactory.selector
        ];

        for (uint256 i = 0; i < eip1967Proxies.length; i++) {
            IProxy proxy = IProxy(payable(eip1967Proxies[i]));
            vm.prank(address(0));
            _rio.set(sels[i], proxy.implementation());
        }

        vm.prank(address(0));
        address l1SBImpl = IStaticL1ChugSplashProxy(address(_rii.l1StandardBridgeProxy())).getImplementation();
        vm.prank(address(0));
        _rio.set(_rio.l1StandardBridge.selector, l1SBImpl);

        address mipsLogic = _rii.opcm().implementations().mipsImpl;
        _rio.set(_rio.mipsSingleton.selector, mipsLogic);

        address delayedWETH = _rii.opcm().implementations().delayedWETHImpl;
        _rio.set(_rio.delayedWETH.selector, delayedWETH);

        IAddressManager am = _rii.addressManager();
        _rio.set(_rio.l1CrossDomainMessenger.selector, am.getAddress("OVM_L1CrossDomainMessenger"));

        address preimageOracle = address(IMIPS(mipsLogic).oracle());
        _rio.set(_rio.preimageOracleSingleton.selector, preimageOracle);
    }
}
