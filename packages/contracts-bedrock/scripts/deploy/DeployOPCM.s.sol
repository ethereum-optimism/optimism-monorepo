// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Script } from "forge-std/Script.sol";

import { LibString } from "@solady/utils/LibString.sol";

import { BaseDeployIO } from "scripts/deploy/BaseDeployIO.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { IProtocolVersions } from "interfaces/L1/IProtocolVersions.sol";
import { IOPContractsManager } from "interfaces/L1/IOPContractsManager.sol";
import { IProxyAdmin } from "interfaces/universal/IProxyAdmin.sol";

contract DeployOPCMInput is BaseDeployIO {
    ISuperchainConfig internal _superchainConfig;
    IProtocolVersions internal _protocolVersions;
    IProxyAdmin internal _superchainProxyAdmin;
    string internal _l1ContractsRelease;
    address internal _upgradeController;

    address internal _addressManagerBlueprint;
    address internal _proxyBlueprint;
    address internal _proxyAdminBlueprint;
    address internal _l1ChugSplashProxyBlueprint;
    address internal _resolvedDelegateProxyBlueprint;
    address internal _permissionedDisputeGame1Blueprint;
    address internal _permissionedDisputeGame2Blueprint;

    address internal _superchainConfigImpl;
    address internal _protocolVersionsImpl;
    address internal _l1ERC721BridgeImpl;
    address internal _optimismPortalImpl;
    address internal _systemConfigImpl;
    address internal _optimismMintableERC20FactoryImpl;
    address internal _l1CrossDomainMessengerImpl;
    address internal _l1StandardBridgeImpl;
    address internal _disputeGameFactoryImpl;
    address internal _anchorStateRegistryImpl;
    address internal _delayedWETHImpl;
    address internal _mipsImpl;

    // Setter for address type
    function set(bytes4 _sel, address _addr) public {
        require(_addr != address(0), "DeployOPCMInput: cannot set zero address");

        // forgefmt: disable-start
        if (_sel == this.superchainConfig.selector) _superchainConfig = ISuperchainConfig(_addr);
        else if (_sel == this.protocolVersions.selector) _protocolVersions = IProtocolVersions(_addr);
        else if (_sel == this.superchainProxyAdmin.selector) _superchainProxyAdmin = IProxyAdmin(_addr);
        else if (_sel == this.superchainConfigImpl.selector) _superchainConfigImpl = _addr;
        else if (_sel == this.protocolVersionsImpl.selector) _protocolVersionsImpl = _addr;
        else if (_sel == this.upgradeController.selector) _upgradeController = _addr;
        else if (_sel == this.addressManagerBlueprint.selector) _addressManagerBlueprint = _addr;
        else if (_sel == this.proxyBlueprint.selector) _proxyBlueprint = _addr;
        else if (_sel == this.proxyAdminBlueprint.selector) _proxyAdminBlueprint = _addr;
        else if (_sel == this.l1ChugSplashProxyBlueprint.selector) _l1ChugSplashProxyBlueprint = _addr;
        else if (_sel == this.resolvedDelegateProxyBlueprint.selector) _resolvedDelegateProxyBlueprint = _addr;
        else if (_sel == this.permissionedDisputeGame1Blueprint.selector) _permissionedDisputeGame1Blueprint = _addr;
        else if (_sel == this.permissionedDisputeGame2Blueprint.selector) _permissionedDisputeGame2Blueprint = _addr;
        else if (_sel == this.l1ERC721BridgeImpl.selector) _l1ERC721BridgeImpl = _addr;
        else if (_sel == this.optimismPortalImpl.selector) _optimismPortalImpl = _addr;
        else if (_sel == this.systemConfigImpl.selector) _systemConfigImpl = _addr;
        else if (_sel == this.optimismMintableERC20FactoryImpl.selector) _optimismMintableERC20FactoryImpl = _addr;
        else if (_sel == this.l1CrossDomainMessengerImpl.selector) _l1CrossDomainMessengerImpl = _addr;
        else if (_sel == this.l1StandardBridgeImpl.selector) _l1StandardBridgeImpl = _addr;
        else if (_sel == this.disputeGameFactoryImpl.selector) _disputeGameFactoryImpl = _addr;
        else if (_sel == this.anchorStateRegistryImpl.selector) _anchorStateRegistryImpl = _addr;
        else if (_sel == this.delayedWETHImpl.selector) _delayedWETHImpl = _addr;
        else if (_sel == this.mipsImpl.selector) _mipsImpl = _addr;
        else revert("DeployOPCMInput: unknown selector");
        // forgefmt: disable-end
    }

    // Setter for string type
    function set(bytes4 _sel, string memory _value) public {
        require(!LibString.eq(_value, ""), "DeployOPCMInput: cannot set empty string");
        if (_sel == this.l1ContractsRelease.selector) _l1ContractsRelease = _value;
        else revert("DeployOPCMInput: unknown selector");
    }

    // Getters
    function superchainConfig() public view returns (ISuperchainConfig) {
        require(address(_superchainConfig) != address(0), "DeployOPCMInput: not set");
        return _superchainConfig;
    }

    function protocolVersions() public view returns (IProtocolVersions) {
        require(address(_protocolVersions) != address(0), "DeployOPCMInput: not set");
        return _protocolVersions;
    }

    function superchainProxyAdmin() public view returns (IProxyAdmin) {
        require(address(_superchainProxyAdmin) != address(0), "DeployOPCMInput: not set");
        return _superchainProxyAdmin;
    }

    function l1ContractsRelease() public view returns (string memory) {
        require(!LibString.eq(_l1ContractsRelease, ""), "DeployOPCMInput: not set");
        return _l1ContractsRelease;
    }

    function upgradeController() public view returns (address) {
        require(_upgradeController != address(0), "DeployOPCMInput: not set");
        return _upgradeController;
    }

    function addressManagerBlueprint() public view returns (address) {
        require(_addressManagerBlueprint != address(0), "DeployOPCMInput: not set");
        return _addressManagerBlueprint;
    }

    function proxyBlueprint() public view returns (address) {
        require(_proxyBlueprint != address(0), "DeployOPCMInput: not set");
        return _proxyBlueprint;
    }

    function proxyAdminBlueprint() public view returns (address) {
        require(_proxyAdminBlueprint != address(0), "DeployOPCMInput: not set");
        return _proxyAdminBlueprint;
    }

    function l1ChugSplashProxyBlueprint() public view returns (address) {
        require(_l1ChugSplashProxyBlueprint != address(0), "DeployOPCMInput: not set");
        return _l1ChugSplashProxyBlueprint;
    }

    function resolvedDelegateProxyBlueprint() public view returns (address) {
        require(_resolvedDelegateProxyBlueprint != address(0), "DeployOPCMInput: not set");
        return _resolvedDelegateProxyBlueprint;
    }

    function permissionedDisputeGame1Blueprint() public view returns (address) {
        require(_permissionedDisputeGame1Blueprint != address(0), "DeployOPCMInput: not set");
        return _permissionedDisputeGame1Blueprint;
    }

    function permissionedDisputeGame2Blueprint() public view returns (address) {
        require(_permissionedDisputeGame2Blueprint != address(0), "DeployOPCMInput: not set");
        return _permissionedDisputeGame2Blueprint;
    }

    function l1ERC721BridgeImpl() public view returns (address) {
        require(_l1ERC721BridgeImpl != address(0), "DeployOPCMInput: not set");
        return _l1ERC721BridgeImpl;
    }

    function optimismPortalImpl() public view returns (address) {
        require(_optimismPortalImpl != address(0), "DeployOPCMInput: not set");
        return _optimismPortalImpl;
    }

    function systemConfigImpl() public view returns (address) {
        require(_systemConfigImpl != address(0), "DeployOPCMInput: not set");
        return _systemConfigImpl;
    }

    function optimismMintableERC20FactoryImpl() public view returns (address) {
        require(_optimismMintableERC20FactoryImpl != address(0), "DeployOPCMInput: not set");
        return _optimismMintableERC20FactoryImpl;
    }

    function l1CrossDomainMessengerImpl() public view returns (address) {
        require(_l1CrossDomainMessengerImpl != address(0), "DeployOPCMInput: not set");
        return _l1CrossDomainMessengerImpl;
    }

    function l1StandardBridgeImpl() public view returns (address) {
        require(_l1StandardBridgeImpl != address(0), "DeployOPCMInput: not set");
        return _l1StandardBridgeImpl;
    }

    function disputeGameFactoryImpl() public view returns (address) {
        require(_disputeGameFactoryImpl != address(0), "DeployOPCMInput: not set");
        return _disputeGameFactoryImpl;
    }

    function anchorStateRegistryImpl() public view returns (address) {
        require(_anchorStateRegistryImpl != address(0), "DeployOPCMInput: not set");
        return _anchorStateRegistryImpl;
    }

    function superchainConfigImpl() public view returns (address) {
        require(_superchainConfigImpl != address(0), "DeployOPCMInput: not set");
        return _superchainConfigImpl;
    }

    function protocolVersionsImpl() public view returns (address) {
        require(_protocolVersionsImpl != address(0), "DeployOPCMInput: not set");
        return _protocolVersionsImpl;
    }

    function delayedWETHImpl() public view returns (address) {
        require(_delayedWETHImpl != address(0), "DeployOPCMInput: not set");
        return _delayedWETHImpl;
    }

    function mipsImpl() public view returns (address) {
        require(_mipsImpl != address(0), "DeployOPCMInput: not set");
        return _mipsImpl;
    }
}

contract DeployOPCMOutput is BaseDeployIO {
    IOPContractsManager internal _opcm;

    // Setter for address type
    function set(bytes4 _sel, address _addr) public {
        require(_addr != address(0), "DeployOPCMOutput: cannot set zero address");
        if (_sel == this.opcm.selector) _opcm = IOPContractsManager(_addr);
        else revert("DeployOPCMOutput: unknown selector");
    }

    // Getter
    function opcm() public view returns (IOPContractsManager) {
        require(address(_opcm) != address(0), "DeployOPCMOutput: not set");
        return _opcm;
    }
}

contract DeployOPCM is Script {
    function run(DeployOPCMInput _doi, DeployOPCMOutput _doo) public {
        IOPContractsManager.Blueprints memory blueprints = IOPContractsManager.Blueprints({
            addressManager: _doi.addressManagerBlueprint(),
            proxy: _doi.proxyBlueprint(),
            proxyAdmin: _doi.proxyAdminBlueprint(),
            l1ChugSplashProxy: _doi.l1ChugSplashProxyBlueprint(),
            resolvedDelegateProxy: _doi.resolvedDelegateProxyBlueprint(),
            permissionedDisputeGame1: _doi.permissionedDisputeGame1Blueprint(),
            permissionedDisputeGame2: _doi.permissionedDisputeGame2Blueprint(),
            permissionlessDisputeGame1: address(0),
            permissionlessDisputeGame2: address(0)
        });
        IOPContractsManager.Implementations memory implementations = IOPContractsManager.Implementations({
            superchainConfigImpl: address(_doi.superchainConfigImpl()),
            protocolVersionsImpl: address(_doi.protocolVersionsImpl()),
            l1ERC721BridgeImpl: address(_doi.l1ERC721BridgeImpl()),
            optimismPortalImpl: address(_doi.optimismPortalImpl()),
            systemConfigImpl: address(_doi.systemConfigImpl()),
            optimismMintableERC20FactoryImpl: address(_doi.optimismMintableERC20FactoryImpl()),
            l1CrossDomainMessengerImpl: address(_doi.l1CrossDomainMessengerImpl()),
            l1StandardBridgeImpl: address(_doi.l1StandardBridgeImpl()),
            disputeGameFactoryImpl: address(_doi.disputeGameFactoryImpl()),
            anchorStateRegistryImpl: address(_doi.anchorStateRegistryImpl()),
            delayedWETHImpl: address(_doi.delayedWETHImpl()),
            mipsImpl: address(_doi.mipsImpl())
        });

        IOPContractsManager opcm_ = deployOPCM(
            _doi.superchainConfig(),
            _doi.protocolVersions(),
            _doi.superchainProxyAdmin(),
            blueprints,
            implementations,
            _doi.l1ContractsRelease(),
            _doi.upgradeController()
        );
        _doo.set(_doo.opcm.selector, address(opcm_));

        assertValidOpcm(_doi, _doo);
    }

    function deployOPCM(
        ISuperchainConfig _superchainConfig,
        IProtocolVersions _protocolVersions,
        IProxyAdmin _superchainProxyAdmin,
        IOPContractsManager.Blueprints memory _blueprints,
        IOPContractsManager.Implementations memory _implementations,
        string memory _l1ContractsRelease,
        address _upgradeController
    )
        public
        returns (IOPContractsManager opcm_)
    {
        opcm_ = IOPContractsManager(
            DeployUtils.createDeterministic({
                _name: "OPContractsManager",
                _args: DeployUtils.encodeConstructor(
                    abi.encodeCall(
                        IOPContractsManager.__constructor__,
                        (
                            _superchainConfig,
                            _protocolVersions,
                            _superchainProxyAdmin,
                            _l1ContractsRelease,
                            _blueprints,
                            _implementations,
                            _upgradeController
                        )
                    )
                ),
                _salt: DeployUtils.DEFAULT_SALT
            })
        );
        vm.label(address(opcm_), "OPContractsManager");
    }

    function assertValidOpcm(DeployOPCMInput _doi, DeployOPCMOutput _doo) public view {
        IOPContractsManager impl = IOPContractsManager(address(_doo.opcm()));
        require(address(impl.superchainConfig()) == address(_doi.superchainConfig()), "OPCMI-10");
        require(address(impl.protocolVersions()) == address(_doi.protocolVersions()), "OPCMI-20");
        require(LibString.eq(impl.l1ContractsRelease(), string.concat(_doi.l1ContractsRelease(), "-rc")), "OPCMI-30");

        require(impl.upgradeController() == _doi.upgradeController(), "OPCMI-40");

        IOPContractsManager.Blueprints memory blueprints = impl.blueprints();
        require(blueprints.addressManager == _doi.addressManagerBlueprint(), "OPCMI-40");
        require(blueprints.proxy == _doi.proxyBlueprint(), "OPCMI-50");
        require(blueprints.proxyAdmin == _doi.proxyAdminBlueprint(), "OPCMI-60");
        require(blueprints.l1ChugSplashProxy == _doi.l1ChugSplashProxyBlueprint(), "OPCMI-70");
        require(blueprints.resolvedDelegateProxy == _doi.resolvedDelegateProxyBlueprint(), "OPCMI-80");
        require(blueprints.permissionedDisputeGame1 == _doi.permissionedDisputeGame1Blueprint(), "OPCMI-100");
        require(blueprints.permissionedDisputeGame2 == _doi.permissionedDisputeGame2Blueprint(), "OPCMI-110");

        IOPContractsManager.Implementations memory implementations = impl.implementations();
        require(implementations.l1ERC721BridgeImpl == _doi.l1ERC721BridgeImpl(), "OPCMI-120");
        require(implementations.optimismPortalImpl == _doi.optimismPortalImpl(), "OPCMI-130");
        require(implementations.systemConfigImpl == _doi.systemConfigImpl(), "OPCMI-140");
        require(
            implementations.optimismMintableERC20FactoryImpl == _doi.optimismMintableERC20FactoryImpl(), "OPCMI-150"
        );
        require(implementations.l1CrossDomainMessengerImpl == _doi.l1CrossDomainMessengerImpl(), "OPCMI-160");
        require(implementations.l1StandardBridgeImpl == _doi.l1StandardBridgeImpl(), "OPCMI-170");
        require(implementations.disputeGameFactoryImpl == _doi.disputeGameFactoryImpl(), "OPCMI-180");
        require(implementations.anchorStateRegistryImpl == _doi.anchorStateRegistryImpl(), "OPCMI-190");
        require(implementations.delayedWETHImpl == _doi.delayedWETHImpl(), "OPCMI-200");
        require(implementations.mipsImpl == _doi.mipsImpl(), "OPCMI-210");
    }

    function etchIOContracts() public returns (DeployOPCMInput doi_, DeployOPCMOutput doo_) {
        (doi_, doo_) = getIOContracts();
        vm.etch(address(doi_), type(DeployOPCMInput).runtimeCode);
        vm.etch(address(doo_), type(DeployOPCMOutput).runtimeCode);
    }

    function getIOContracts() public view returns (DeployOPCMInput doi_, DeployOPCMOutput doo_) {
        doi_ = DeployOPCMInput(DeployUtils.toIOAddress(msg.sender, "optimism.DeployOPCMInput"));
        doo_ = DeployOPCMOutput(DeployUtils.toIOAddress(msg.sender, "optimism.DeployOPCMOutput"));
    }
}
