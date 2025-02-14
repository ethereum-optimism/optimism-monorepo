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
import { IDelayedWETH } from "interfaces/dispute/IDelayedWETH.sol";
// Contracts
import { OPPrestateUpdater } from "src/L1/OPPrestateUpdater.sol";

interface IOPContractsManager180 {
struct Blueprints {
    address addressManager;
        address proxy;
        address proxyAdmin;
        address l1ChugSplashProxy;
        address resolvedDelegateProxy;
        address anchorStateRegistry;
        address permissionedDisputeGame1;
        address permissionedDisputeGame2;
    }
    function blueprints() external view returns (Blueprints memory);
}

contract DeployOPPrestateUpdater is Script {
    bytes32 internal _salt = DeployUtils.DEFAULT_SALT;

    function deployOPPrestateUpdater(
        string memory _baseChain
    )
        public
        returns (OPPrestateUpdater oppu)
    {
        string memory superchainBasePath = "./lib/superchain-registry/superchain/configs/";
        string memory superchainToml = vm.readFile(string.concat(superchainBasePath, _baseChain, "/superchain.toml"));

        // Superchain shared contracts
        ISuperchainConfig superchainConfig = ISuperchainConfig(vm.parseTomlAddress(superchainToml, ".superchain_config_addr"));
        IProtocolVersions protocolVersions = IProtocolVersions(vm.parseTomlAddress(superchainToml, ".protocol_versions_addr"));
        IOPContractsManager180 opContractsManager180 = IOPContractsManager180(vm.parseTomlAddress(superchainToml, ".op_contracts_manager_proxy_addr"));


        // forgefmt: disable-start
        IOPContractsManager.Blueprints memory blueprints;
        blueprints.addressManager = address(0);
        blueprints.proxy = address(0);
        blueprints.proxyAdmin = address(0);
        blueprints.l1ChugSplashProxy = address(0);
        blueprints.resolvedDelegateProxy = address(0);

        IOPContractsManager180.Blueprints memory blueprints180 = opContractsManager180.blueprints();
        blueprints.permissionedDisputeGame1 = blueprints180.permissionedDisputeGame1;
        blueprints.permissionedDisputeGame2 = blueprints180.permissionedDisputeGame2;

        // (blueprints.permissionlessDisputeGame1, blueprints.permissionlessDisputeGame2) = DeployUtils.createDeterministicBlueprint(vm.getCode("FaultDisputeGame"), _salt);
        // forgefmt: disable-end

        oppu = OPPrestateUpdater(
            DeployUtils.createDeterministic({
                _name: "OPPrestateUpdater",
                _args: DeployUtils.encodeConstructor(
                    abi.encodeCall(IOPPrestateUpdater.__constructor__, (superchainConfig, protocolVersions, blueprints))
                ),
                _salt: bytes32(_salt)
            })
        );

        require(address(oppu.superchainConfig()) == address(superchainConfig), "OPPUI-10");
        require(address(oppu.protocolVersions()) == address(protocolVersions), "OPPUI-20");
        require(LibString.eq(oppu.l1ContractsRelease(), "none"), "OPPUI-30");

        require(oppu.upgradeController() == address(0), "OPPUI-40");

        // encode decode because oppu.implementations returns IOPPrestateUpdater.Implmentations
        IOPContractsManager.Implementations memory implementations =
            abi.decode(abi.encode(oppu.implementations()), (IOPContractsManager.Implementations));
        require(implementations.l1CrossDomainMessengerImpl == address(0), "OPPUI-120");
        require(implementations.l1StandardBridgeImpl == address(0), "OPPUI-130");
        require(implementations.disputeGameFactoryImpl == address(0), "OPPUI-140");
        require(implementations.optimismMintableERC20FactoryImpl == address(0), "OPPUI-150");
        require(implementations.l1CrossDomainMessengerImpl == address(0), "OPPUI-160");
        require(implementations.l1StandardBridgeImpl == address(0), "OPPUI-170");
        require(implementations.disputeGameFactoryImpl == address(0), "OPPUI-180");
        require(implementations.anchorStateRegistryImpl == address(0), "OPPUI-190");
        require(implementations.delayedWETHImpl == address(0), "OPPUI-200");
        require(implementations.mipsImpl == address(0), "OPPUI-210");
    }
}
