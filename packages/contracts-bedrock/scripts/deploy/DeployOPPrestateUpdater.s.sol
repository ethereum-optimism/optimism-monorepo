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

contract DeployOPPrestateUpdater is Script {
    bytes32 internal _salt = DeployUtils.DEFAULT_SALT;

    function deployOPPrestateUpdater(
        ISuperchainConfig _superchainConfig,
        IProtocolVersions _protocolVersions
    )
        public
        returns (OPPrestateUpdater oppu)
    {
        // forgefmt: disable-start
        vm.startBroadcast(msg.sender);
        IOPContractsManager.Blueprints memory blueprints;
        blueprints.addressManager = address(0);
        blueprints.proxy = address(0);
        blueprints.proxyAdmin = address(0);
        blueprints.l1ChugSplashProxy = address(0);
        blueprints.resolvedDelegateProxy = address(0);
        // The max initcode/runtimecode size is 48KB/24KB.
        // But for Blueprint, the initcode is stored as runtime code, that's why it's necessary to split into 2 parts.
        (blueprints.permissionedDisputeGame1, blueprints.permissionedDisputeGame2) = DeployUtils.createDeterministicBlueprint(vm.getCode("PermissionedDisputeGame"), _salt);
        (blueprints.permissionlessDisputeGame1, blueprints.permissionlessDisputeGame2) = DeployUtils.createDeterministicBlueprint(vm.getCode("FaultDisputeGame"), _salt);
        // forgefmt: disable-end

        oppu = OPPrestateUpdater(
            DeployUtils.createDeterministic({
                _name: "OPPrestateUpdater",
                _args: DeployUtils.encodeConstructor(
                    abi.encodeCall(IOPPrestateUpdater.__constructor__, (_superchainConfig, _protocolVersions, blueprints))
                ),
                _salt: bytes32(_salt)
            })
        );
        vm.stopBroadcast();

        require(address(oppu.superchainConfig()) == address(_superchainConfig), "OPPUI-10");
        require(address(oppu.protocolVersions()) == address(_protocolVersions), "OPPUI-20");
        require(LibString.eq(oppu.l1ContractsRelease(), string.concat("", "-rc")), "OPPUI-30");

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
