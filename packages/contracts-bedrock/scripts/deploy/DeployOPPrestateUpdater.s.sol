// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Scripting
import { Script } from "forge-std/Script.sol";

// Libraries
import { LibString } from "@solady/utils/LibString.sol";

// Scripts
import { BaseDeployIO } from "scripts/deploy/BaseDeployIO.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

// Interfaces
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { IProtocolVersions } from "interfaces/L1/IProtocolVersions.sol";
import { IOPContractsManager } from "interfaces/L1/IOPContractsManager.sol";
import { IOPPrestateUpdater } from "interfaces/L1/IOPPrestateUpdater.sol";
import { IProxyAdmin } from "interfaces/universal/IProxyAdmin.sol";

// Contracts
import { OPPrestateUpdater } from "src/L1/OPPrestateUpdater.sol";

contract DeployOPPrestateUpdaterInput is BaseDeployIO {
    ISuperchainConfig internal _superchainConfig;
    IProtocolVersions internal _protocolVersions;
    IProxyAdmin internal _superchainProxyAdmin;

    address internal _upgradeController;

    address internal _permissionedDisputeGame1Blueprint;
    address internal _permissionedDisputeGame2Blueprint;
    address internal _permissionlessDisputeGame1Blueprint;
    address internal _permissionlessDisputeGame2Blueprint;

    address internal _delayedWETHImpl;

    // Setter for address type
    function set(bytes4 _sel, address _addr) public {
        require(_addr != address(0), "DeployOPPrestateUpdaterInput: cannot set zero address");

        // forgefmt: disable-start
        if (_sel == this.superchainConfig.selector) _superchainConfig = ISuperchainConfig(_addr);
        else if (_sel == this.protocolVersions.selector) _protocolVersions = IProtocolVersions(_addr);
        else if (_sel == this.upgradeController.selector) _upgradeController = _addr;
        else if (_sel == this.permissionedDisputeGame1Blueprint.selector) _permissionedDisputeGame1Blueprint = _addr;
        else if (_sel == this.permissionedDisputeGame2Blueprint.selector) _permissionedDisputeGame2Blueprint = _addr;
        else if (_sel == this.permissionlessDisputeGame1Blueprint.selector) _permissionlessDisputeGame1Blueprint = _addr;
        else if (_sel == this.permissionlessDisputeGame2Blueprint.selector) _permissionlessDisputeGame2Blueprint = _addr;
        else if (_sel == this.delayedWETHImpl.selector) _delayedWETHImpl = _addr;
        else revert("DeployOPPrestateUpdaterInput: unknown selector");
        // forgefmt: disable-end
    }

    // Getters
    function superchainConfig() public view returns (ISuperchainConfig) {
        require(address(_superchainConfig) != address(0), "DeployOPPrestateUpdaterInput: not set");
        return _superchainConfig;
    }

    function protocolVersions() public view returns (IProtocolVersions) {
        require(address(_protocolVersions) != address(0), "DeployOPPrestateUpdaterInput: not set");
        return _protocolVersions;
    }

    function superchainProxyAdmin() public view returns (IProxyAdmin) {
        require(address(_superchainProxyAdmin) != address(0), "DeployOPCMInput: not set");
        return _superchainProxyAdmin;
    }

    function upgradeController() public view returns (address) {
        require(_upgradeController != address(0), "DeployOPPrestateUpdaterInput: not set");
        return _upgradeController;
    }

    function permissionedDisputeGame1Blueprint() public view returns (address) {
        require(_permissionedDisputeGame1Blueprint != address(0), "DeployOPPrestateUpdaterInput: not set");
        return _permissionedDisputeGame1Blueprint;
    }

    function permissionedDisputeGame2Blueprint() public view returns (address) {
        require(_permissionedDisputeGame2Blueprint != address(0), "DeployOPPrestateUpdaterInput: not set");
        return _permissionedDisputeGame2Blueprint;
    }

    function permissionlessDisputeGame1Blueprint() public view returns (address) {
        require(_permissionlessDisputeGame1Blueprint != address(0), "DeployOPPrestateUpdaterInput: not set");
        return _permissionlessDisputeGame1Blueprint;
    }

    function permissionlessDisputeGame2Blueprint() public view returns (address) {
        require(_permissionlessDisputeGame2Blueprint != address(0), "DeployOPPrestateUpdaterInput: not set");
        return _permissionlessDisputeGame2Blueprint;
    }

    function delayedWETHImpl() public view returns (address) {
        require(_delayedWETHImpl != address(0), "DeployOPPrestateUpdaterInput: not set");
        return _delayedWETHImpl;
    }
}

contract DeployOPPrestateUpdaterOutput is BaseDeployIO {
    IOPContractsManager internal _oppu;

    // Setter for address type
    function set(bytes4 _sel, address _addr) public {
        require(_addr != address(0), "DeployOPPrestateUpdaterOutput: cannot set zero address");
        if (_sel == this.oppu.selector) _oppu = IOPContractsManager(_addr);
        else revert("DeployOPPrestateUpdaterOutput: unknown selector");
    }

    // Getter
    function oppu() public view returns (IOPContractsManager) {
        require(address(_oppu) != address(0), "DeployOPPrestateUpdaterOutput: not set");
        return _oppu;
    }
}

contract DeployOPPrestateUpdater is Script {
    bytes32 internal _salt = DeployUtils.DEFAULT_SALT;

    function run(DeployOPPrestateUpdaterInput _doi, DeployOPPrestateUpdaterOutput _doo) public {
        IOPContractsManager.Blueprints memory blueprints = IOPContractsManager.Blueprints({
            addressManager: address(0),
            proxy: address(0),
            proxyAdmin: address(0),
            l1ChugSplashProxy: address(0),
            resolvedDelegateProxy: address(0),
            permissionedDisputeGame1: _doi.permissionedDisputeGame1Blueprint(),
            permissionedDisputeGame2: _doi.permissionedDisputeGame2Blueprint(),
            permissionlessDisputeGame1: _doi.permissionlessDisputeGame1Blueprint(),
            permissionlessDisputeGame2: _doi.permissionlessDisputeGame2Blueprint()
        });

        OPPrestateUpdater oppu_ = deployOPPrestateUpdater(_doi.superchainConfig(), _doi.protocolVersions(), blueprints);
        _doo.set(_doo.oppu.selector, address(oppu_));

        assertValidPrestateUpdater(_doi, _doo);
    }

    function deployOPPrestateUpdater(
        ISuperchainConfig _superchainConfig,
        IProtocolVersions _protocolVersions,
        IOPContractsManager.Blueprints memory _blueprints
    )
        public
        returns (OPPrestateUpdater oppu_)
    {
        vm.startBroadcast(msg.sender);
        oppu_ = OPPrestateUpdater(
            DeployUtils.createDeterministic({
                _name: "OPPrestateUpdater",
                _args: DeployUtils.encodeConstructor(
                    abi.encodeCall(IOPPrestateUpdater.__constructor__, (_superchainConfig, _protocolVersions, _blueprints))
                ),
                _salt: bytes32(_salt)
            })
        );
        vm.label(address(oppu_), "OPPrestateUpdater");
    }

    function assertValidPrestateUpdater(
        DeployOPPrestateUpdaterInput _doi,
        DeployOPPrestateUpdaterOutput _doo
    )
        public
        view
    {
        IOPContractsManager impl = IOPContractsManager(address(_doo.oppu()));
        require(address(impl.superchainConfig()) == address(_doi.superchainConfig()), "OPPUI-10");
        require(address(impl.protocolVersions()) == address(_doi.protocolVersions()), "OPPUI-20");
        require(LibString.eq(impl.l1ContractsRelease(), string.concat("", "-rc")), "OPPUI-30");

        require(impl.upgradeController() == _doi.upgradeController(), "OPPUI-40");

        IOPContractsManager.Blueprints memory blueprints = impl.blueprints();
        require(blueprints.addressManager == address(0), "OPPUI-40");
        require(blueprints.proxy == address(0), "OPPUI-50");
        require(blueprints.proxyAdmin == address(0), "OPPUI-60");
        require(blueprints.l1ChugSplashProxy == address(0), "OPPUI-70");
        require(blueprints.resolvedDelegateProxy == address(0), "OPPUI-80");
        require(blueprints.permissionedDisputeGame1 == _doi.permissionedDisputeGame1Blueprint(), "OPPUI-100");
        require(blueprints.permissionedDisputeGame2 == _doi.permissionedDisputeGame2Blueprint(), "OPPUI-110");

        IOPContractsManager.Implementations memory implementations = impl.implementations();
        require(implementations.l1CrossDomainMessengerImpl == address(0), "OPPUI-120");
        require(implementations.l1StandardBridgeImpl == address(0), "OPPUI-130");
        require(implementations.disputeGameFactoryImpl == address(0), "OPPUI-140");
        require(implementations.optimismMintableERC20FactoryImpl == address(0), "OPPUI-150");
        require(implementations.l1CrossDomainMessengerImpl == address(0), "OPPUI-160");
        require(implementations.l1StandardBridgeImpl == address(0), "OPPUI-170");
        require(implementations.disputeGameFactoryImpl == address(0), "OPPUI-180");
        require(implementations.anchorStateRegistryImpl == address(0), "OPPUI-190");
        require(implementations.delayedWETHImpl == _doi.delayedWETHImpl(), "OPPUI-200");
        require(implementations.mipsImpl == address(0), "OPPUI-210");
    }

    function etchIOContracts() public returns (DeployOPPrestateUpdaterInput doi_, DeployOPPrestateUpdaterOutput doo_) {
        (doi_, doo_) = getIOContracts();
        vm.etch(address(doi_), type(DeployOPPrestateUpdaterInput).runtimeCode);
        vm.etch(address(doo_), type(DeployOPPrestateUpdaterOutput).runtimeCode);
    }

    function getIOContracts()
        public
        view
        returns (DeployOPPrestateUpdaterInput doi_, DeployOPPrestateUpdaterOutput doo_)
    {
        doi_ =
            DeployOPPrestateUpdaterInput(DeployUtils.toIOAddress(msg.sender, "optimism.DeployOPPrestateUpdaterInput"));
        doo_ =
            DeployOPPrestateUpdaterOutput(DeployUtils.toIOAddress(msg.sender, "optimism.DeployOPPrestateUpdaterOutput"));
    }
}
