// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Test } from "forge-std/Test.sol";
import { DeployOPCM, DeployOPCMInput, DeployOPCMOutput } from "scripts/deploy/DeployOPCM.s.sol";
import { IOPContractsManager } from "interfaces/L1/IOPContractsManager.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { IProtocolVersions } from "interfaces/L1/IProtocolVersions.sol";
import { IProxyAdmin } from "interfaces/universal/IProxyAdmin.sol";

contract DeployOPCMInput_Test is Test {
    DeployOPCMInput dii;
    string release = "1.0.0";

    function setUp() public {
        dii = new DeployOPCMInput();
    }

    function test_getters_whenNotSet_reverts() public {
        vm.expectRevert("DeployOPCMInput: not set");
        dii.superchainConfig();

        vm.expectRevert("DeployOPCMInput: not set");
        dii.protocolVersions();

        vm.expectRevert("DeployOPCMInput: not set");
        dii.l1ContractsRelease();

        vm.expectRevert("DeployOPCMInput: not set");
        dii.addressManagerBlueprint();

        vm.expectRevert("DeployOPCMInput: not set");
        dii.proxyBlueprint();

        vm.expectRevert("DeployOPCMInput: not set");
        dii.proxyAdminBlueprint();

        vm.expectRevert("DeployOPCMInput: not set");
        dii.l1ChugSplashProxyBlueprint();

        vm.expectRevert("DeployOPCMInput: not set");
        dii.resolvedDelegateProxyBlueprint();

        vm.expectRevert("DeployOPCMInput: not set");
        dii.permissionedDisputeGame1Blueprint();

        vm.expectRevert("DeployOPCMInput: not set");
        dii.permissionedDisputeGame2Blueprint();

        vm.expectRevert("DeployOPCMInput: not set");
        dii.l1ERC721BridgeImpl();

        vm.expectRevert("DeployOPCMInput: not set");
        dii.optimismPortalImpl();

        vm.expectRevert("DeployOPCMInput: not set");
        dii.systemConfigImpl();

        vm.expectRevert("DeployOPCMInput: not set");
        dii.optimismMintableERC20FactoryImpl();

        vm.expectRevert("DeployOPCMInput: not set");
        dii.l1CrossDomainMessengerImpl();

        vm.expectRevert("DeployOPCMInput: not set");
        dii.l1StandardBridgeImpl();

        vm.expectRevert("DeployOPCMInput: not set");
        dii.disputeGameFactoryImpl();

        vm.expectRevert("DeployOPCMInput: not set");
        dii.anchorStateRegistryImpl();

        vm.expectRevert("DeployOPCMInput: not set");
        dii.delayedWETHImpl();

        vm.expectRevert("DeployOPCMInput: not set");
        dii.mipsImpl();
    }

    // Below setter tests are split into two parts to avoid stack too deep errors

    function test_set_part1_succeeds() public {
        ISuperchainConfig superchainConfig = ISuperchainConfig(makeAddr("superchainConfig"));
        IProtocolVersions protocolVersions = IProtocolVersions(makeAddr("protocolVersions"));
        address superchainConfigImpl = makeAddr("superchainConfigImpl");
        address protocolVersionsImpl = makeAddr("protocolVersionsImpl");
        address upgradeController = makeAddr("upgradeController");
        address addressManagerBlueprint = makeAddr("addressManagerBlueprint");
        address proxyBlueprint = makeAddr("proxyBlueprint");
        address proxyAdminBlueprint = makeAddr("proxyAdminBlueprint");
        address l1ChugSplashProxyBlueprint = makeAddr("l1ChugSplashProxyBlueprint");
        address resolvedDelegateProxyBlueprint = makeAddr("resolvedDelegateProxyBlueprint");
        address permissionedDisputeGame1Blueprint = makeAddr("permissionedDisputeGame1Blueprint");
        address permissionedDisputeGame2Blueprint = makeAddr("permissionedDisputeGame2Blueprint");

        dii.set(dii.superchainConfig.selector, address(superchainConfig));
        dii.set(dii.protocolVersions.selector, address(protocolVersions));
        dii.set(dii.superchainConfigImpl.selector, superchainConfigImpl);
        dii.set(dii.protocolVersionsImpl.selector, protocolVersionsImpl);
        dii.set(dii.l1ContractsRelease.selector, release);
        dii.set(dii.upgradeController.selector, upgradeController);
        dii.set(dii.addressManagerBlueprint.selector, addressManagerBlueprint);
        dii.set(dii.proxyBlueprint.selector, proxyBlueprint);
        dii.set(dii.proxyAdminBlueprint.selector, proxyAdminBlueprint);
        dii.set(dii.l1ChugSplashProxyBlueprint.selector, l1ChugSplashProxyBlueprint);
        dii.set(dii.resolvedDelegateProxyBlueprint.selector, resolvedDelegateProxyBlueprint);
        dii.set(dii.permissionedDisputeGame1Blueprint.selector, permissionedDisputeGame1Blueprint);
        dii.set(dii.permissionedDisputeGame2Blueprint.selector, permissionedDisputeGame2Blueprint);

        assertEq(address(dii.superchainConfig()), address(superchainConfig), "50");
        assertEq(address(dii.protocolVersions()), address(protocolVersions), "100");
        assertEq(dii.l1ContractsRelease(), release, "150");
        assertEq(dii.addressManagerBlueprint(), addressManagerBlueprint, "200");
        assertEq(dii.proxyBlueprint(), proxyBlueprint, "250");
        assertEq(dii.proxyAdminBlueprint(), proxyAdminBlueprint, "300");
        assertEq(dii.l1ChugSplashProxyBlueprint(), l1ChugSplashProxyBlueprint, "350");
        assertEq(dii.resolvedDelegateProxyBlueprint(), resolvedDelegateProxyBlueprint, "400");
        assertEq(dii.permissionedDisputeGame1Blueprint(), permissionedDisputeGame1Blueprint, "500");
        assertEq(dii.permissionedDisputeGame2Blueprint(), permissionedDisputeGame2Blueprint, "550");
        assertEq(dii.upgradeController(), upgradeController, "600");
    }

    function test_set_part2_succeeds() public {
        address l1ERC721BridgeImpl = makeAddr("l1ERC721BridgeImpl");
        address optimismPortalImpl = makeAddr("optimismPortalImpl");
        address systemConfigImpl = makeAddr("systemConfigImpl");
        address optimismMintableERC20FactoryImpl = makeAddr("optimismMintableERC20FactoryImpl");
        address l1CrossDomainMessengerImpl = makeAddr("l1CrossDomainMessengerImpl");
        address l1StandardBridgeImpl = makeAddr("l1StandardBridgeImpl");
        address disputeGameFactoryImpl = makeAddr("disputeGameFactoryImpl");
        address anchorStateRegistryImpl = makeAddr("anchorStateRegistryImpl");
        address delayedWETHImpl = makeAddr("delayedWETHImpl");
        address mipsImpl = makeAddr("mipsImpl");

        dii.set(dii.l1ERC721BridgeImpl.selector, l1ERC721BridgeImpl);
        dii.set(dii.optimismPortalImpl.selector, optimismPortalImpl);
        dii.set(dii.systemConfigImpl.selector, systemConfigImpl);
        dii.set(dii.optimismMintableERC20FactoryImpl.selector, optimismMintableERC20FactoryImpl);
        dii.set(dii.l1CrossDomainMessengerImpl.selector, l1CrossDomainMessengerImpl);
        dii.set(dii.l1StandardBridgeImpl.selector, l1StandardBridgeImpl);
        dii.set(dii.disputeGameFactoryImpl.selector, disputeGameFactoryImpl);
        dii.set(dii.anchorStateRegistryImpl.selector, anchorStateRegistryImpl);
        dii.set(dii.delayedWETHImpl.selector, delayedWETHImpl);
        dii.set(dii.mipsImpl.selector, mipsImpl);

        assertEq(dii.l1ERC721BridgeImpl(), l1ERC721BridgeImpl, "600");
        assertEq(dii.optimismPortalImpl(), optimismPortalImpl, "650");
        assertEq(dii.systemConfigImpl(), systemConfigImpl, "700");
        assertEq(dii.optimismMintableERC20FactoryImpl(), optimismMintableERC20FactoryImpl, "750");
        assertEq(dii.l1CrossDomainMessengerImpl(), l1CrossDomainMessengerImpl, "800");
        assertEq(dii.l1StandardBridgeImpl(), l1StandardBridgeImpl, "850");
        assertEq(dii.disputeGameFactoryImpl(), disputeGameFactoryImpl, "900");
        assertEq(dii.delayedWETHImpl(), delayedWETHImpl, "950");
        assertEq(dii.mipsImpl(), mipsImpl, "1000");
    }

    function test_set_withZeroAddress_reverts() public {
        vm.expectRevert("DeployOPCMInput: cannot set zero address");
        dii.set(dii.superchainConfig.selector, address(0));
    }

    function test_set_withEmptyString_reverts() public {
        vm.expectRevert("DeployOPCMInput: cannot set empty string");
        dii.set(dii.l1ContractsRelease.selector, "");
    }

    function test_set_withInvalidSelector_reverts() public {
        vm.expectRevert("DeployOPCMInput: unknown selector");
        dii.set(bytes4(0xdeadbeef), address(1));
    }

    function test_set_withInvalidStringSelector_reverts() public {
        vm.expectRevert("DeployOPCMInput: unknown selector");
        dii.set(bytes4(0xdeadbeef), "test");
    }
}

contract DeployOPCMOutput_Test is Test {
    DeployOPCMOutput doo;

    function setUp() public {
        doo = new DeployOPCMOutput();
    }

    function test_getters_whenNotSet_reverts() public {
        vm.expectRevert("DeployOPCMOutput: not set");
        doo.opcm();
    }

    function test_set_succeeds() public {
        IOPContractsManager opcm = IOPContractsManager(makeAddr("opcm"));
        vm.etch(address(opcm), hex"01");

        doo.set(doo.opcm.selector, address(opcm));

        assertEq(address(doo.opcm()), address(opcm), "50");
    }

    function test_set_withZeroAddress_reverts() public {
        vm.expectRevert("DeployOPCMOutput: cannot set zero address");
        doo.set(doo.opcm.selector, address(0));
    }

    function test_set_withInvalidSelector_reverts() public {
        vm.expectRevert("DeployOPCMOutput: unknown selector");
        doo.set(bytes4(0xdeadbeef), makeAddr("test"));
    }
}

contract DeployOPCMTest is Test {
    DeployOPCM deployOPCM;
    DeployOPCMInput doi;
    DeployOPCMOutput doo;

    ISuperchainConfig superchainConfigProxy = ISuperchainConfig(makeAddr("superchainConfigProxy"));
    IProtocolVersions protocolVersionsProxy = IProtocolVersions(makeAddr("protocolVersionsProxy"));
    IProxyAdmin superchainProxyAdmin = IProxyAdmin(makeAddr("superchainProxyAdmin"));
    address superchainConfigImpl = makeAddr("superchainConfigImpl");
    address protocolVersionsImpl = makeAddr("protocolVersionsImpl");
    address upgradeController = makeAddr("upgradeController");

    function setUp() public virtual {
        deployOPCM = new DeployOPCM();
        (doi, doo) = deployOPCM.etchIOContracts();
    }

    function test_run_succeeds() public {
        doi.set(doi.superchainConfig.selector, address(superchainConfigProxy));
        doi.set(doi.protocolVersions.selector, address(protocolVersionsProxy));
        doi.set(doi.superchainProxyAdmin.selector, address(superchainProxyAdmin));
        doi.set(doi.superchainConfigImpl.selector, address(superchainConfigImpl));
        doi.set(doi.protocolVersionsImpl.selector, address(protocolVersionsImpl));
        doi.set(doi.l1ContractsRelease.selector, "1.0.0");
        doi.set(doi.upgradeController.selector, upgradeController);

        // Set and etch blueprints
        doi.set(doi.addressManagerBlueprint.selector, makeAddr("addressManagerBlueprint"));
        doi.set(doi.proxyBlueprint.selector, makeAddr("proxyBlueprint"));
        doi.set(doi.proxyAdminBlueprint.selector, makeAddr("proxyAdminBlueprint"));
        doi.set(doi.l1ChugSplashProxyBlueprint.selector, makeAddr("l1ChugSplashProxyBlueprint"));
        doi.set(doi.resolvedDelegateProxyBlueprint.selector, makeAddr("resolvedDelegateProxyBlueprint"));
        doi.set(doi.permissionedDisputeGame1Blueprint.selector, makeAddr("permissionedDisputeGame1Blueprint"));
        doi.set(doi.permissionedDisputeGame2Blueprint.selector, makeAddr("permissionedDisputeGame2Blueprint"));

        // Set and etch implementations
        doi.set(doi.l1ERC721BridgeImpl.selector, makeAddr("l1ERC721BridgeImpl"));
        doi.set(doi.optimismPortalImpl.selector, makeAddr("optimismPortalImpl"));
        doi.set(doi.systemConfigImpl.selector, makeAddr("systemConfigImpl"));
        doi.set(doi.optimismMintableERC20FactoryImpl.selector, makeAddr("optimismMintableERC20FactoryImpl"));
        doi.set(doi.l1CrossDomainMessengerImpl.selector, makeAddr("l1CrossDomainMessengerImpl"));
        doi.set(doi.l1StandardBridgeImpl.selector, makeAddr("l1StandardBridgeImpl"));
        doi.set(doi.disputeGameFactoryImpl.selector, makeAddr("disputeGameFactoryImpl"));
        doi.set(doi.anchorStateRegistryImpl.selector, makeAddr("anchorStateRegistryImpl"));
        doi.set(doi.delayedWETHImpl.selector, makeAddr("delayedWETHImpl"));
        doi.set(doi.mipsImpl.selector, makeAddr("mipsImpl"));

        // Etch all addresses with dummy bytecode
        vm.etch(address(doi.superchainConfig()), hex"01");
        vm.etch(address(doi.protocolVersions()), hex"01");
        vm.etch(address(doi.upgradeController()), hex"01");

        vm.etch(doi.addressManagerBlueprint(), hex"01");
        vm.etch(doi.proxyBlueprint(), hex"01");
        vm.etch(doi.proxyAdminBlueprint(), hex"01");
        vm.etch(doi.l1ChugSplashProxyBlueprint(), hex"01");
        vm.etch(doi.resolvedDelegateProxyBlueprint(), hex"01");
        vm.etch(doi.permissionedDisputeGame1Blueprint(), hex"01");
        vm.etch(doi.permissionedDisputeGame2Blueprint(), hex"01");

        vm.etch(doi.l1ERC721BridgeImpl(), hex"01");
        vm.etch(doi.optimismPortalImpl(), hex"01");
        vm.etch(doi.systemConfigImpl(), hex"01");
        vm.etch(doi.optimismMintableERC20FactoryImpl(), hex"01");
        vm.etch(doi.l1CrossDomainMessengerImpl(), hex"01");
        vm.etch(doi.l1StandardBridgeImpl(), hex"01");
        vm.etch(doi.disputeGameFactoryImpl(), hex"01");
        vm.etch(doi.delayedWETHImpl(), hex"01");
        vm.etch(doi.mipsImpl(), hex"01");

        deployOPCM.run(doi, doo);

        assertNotEq(address(doo.opcm()), address(0));

        // sanity check to ensure that the OPCM is validated
        deployOPCM.assertValidOpcm(doi, doo);
    }
}
