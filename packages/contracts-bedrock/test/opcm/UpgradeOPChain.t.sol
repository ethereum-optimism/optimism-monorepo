// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Test } from "forge-std/Test.sol";
import { Claim } from "src/dispute/lib/Types.sol";

import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";
import { IProxyAdmin } from "interfaces/universal/IProxyAdmin.sol";

import { OPContractsManager } from "src/L1/OPContractsManager.sol";
import { UpgradeOPChain, UpgradeOPChainInput } from "scripts/deploy/UpgradeOPChain.s.sol";

contract UpgradeOPChainInput_Test is Test {
    UpgradeOPChainInput input;

    function setUp() public {
        input = new UpgradeOPChainInput();
    }

    function test_getters_whenNotSet_reverts() public {
        vm.expectRevert("UpgradeOPCMInput: prank not set");
        input.prank();

        vm.expectRevert("UpgradeOPCMInput: not set");
        input.opcm();

        vm.expectRevert("UpgradeOPCMInput: not set");
        input.opChainConfigs();
    }

    function test_setAddress_succeeds() public {
        address mockPrank = makeAddr("prank");
        address mockOPCM = makeAddr("opcm");

        // Create mock contract at OPCM address
        vm.etch(mockOPCM, hex"01");

        input.set(input.prank.selector, mockPrank);
        input.set(input.opcm.selector, mockOPCM);

        assertEq(input.prank(), mockPrank);
        assertEq(address(input.opcm()), mockOPCM);
    }

    function test_setOpChainConfigs_succeeds() public {
        // Create sample OpChainConfig array
        OPContractsManager.OpChainConfig[] memory configs = new OPContractsManager.OpChainConfig[](2);

        // Setup mock addresses and contracts for first config
        address systemConfig1 = makeAddr("systemConfig1");
        address proxyAdmin1 = makeAddr("proxyAdmin1");
        vm.etch(systemConfig1, hex"01");
        vm.etch(proxyAdmin1, hex"01");

        configs[0] = OPContractsManager.OpChainConfig({
            systemConfigProxy: ISystemConfig(systemConfig1),
            proxyAdmin: IProxyAdmin(proxyAdmin1),
            absolutePrestate: Claim.wrap(bytes32(uint256(1)))
        });

        // Setup mock addresses and contracts for second config
        address systemConfig2 = makeAddr("systemConfig2");
        address proxyAdmin2 = makeAddr("proxyAdmin2");
        vm.etch(systemConfig2, hex"01");
        vm.etch(proxyAdmin2, hex"01");

        configs[1] = OPContractsManager.OpChainConfig({
            systemConfigProxy: ISystemConfig(systemConfig2),
            proxyAdmin: IProxyAdmin(proxyAdmin2),
            absolutePrestate: Claim.wrap(bytes32(uint256(2)))
        });

        input.set(input.opChainConfigs.selector, configs);

        bytes memory storedConfigs = input.opChainConfigs();
        assertEq(storedConfigs, abi.encode(configs));

        // Additional verification of stored claims if needed
        OPContractsManager.OpChainConfig[] memory decodedConfigs =
            abi.decode(storedConfigs, (OPContractsManager.OpChainConfig[]));
        assertEq(Claim.unwrap(decodedConfigs[0].absolutePrestate), bytes32(uint256(1)));
        assertEq(Claim.unwrap(decodedConfigs[1].absolutePrestate), bytes32(uint256(2)));
    }

    function test_setAddress_withZeroAddress_reverts() public {
        vm.expectRevert("UpgradeOPCMInput: cannot set zero address");
        input.set(input.prank.selector, address(0));

        vm.expectRevert("UpgradeOPCMInput: cannot set zero address");
        input.set(input.opcm.selector, address(0));
    }

    function test_setOpChainConfigs_withEmptyArray_reverts() public {
        OPContractsManager.OpChainConfig[] memory emptyConfigs = new OPContractsManager.OpChainConfig[](0);

        vm.expectRevert("UpgradeOPCMInput: cannot set empty array");
        input.set(input.opChainConfigs.selector, emptyConfigs);
    }

    function test_set_withInvalidSelector_reverts() public {
        vm.expectRevert("UpgradeOPCMInput: unknown selector");
        input.set(bytes4(0xdeadbeef), makeAddr("test"));

        // Create a single config for testing invalid selector
        OPContractsManager.OpChainConfig[] memory configs = new OPContractsManager.OpChainConfig[](1);
        address mockSystemConfig = makeAddr("systemConfig");
        address mockProxyAdmin = makeAddr("proxyAdmin");
        vm.etch(mockSystemConfig, hex"01");
        vm.etch(mockProxyAdmin, hex"01");

        configs[0] = OPContractsManager.OpChainConfig({
            systemConfigProxy: ISystemConfig(mockSystemConfig),
            proxyAdmin: IProxyAdmin(mockProxyAdmin),
            absolutePrestate: Claim.wrap(bytes32(uint256(1)))
        });

        vm.expectRevert("UpgradeOPCMInput: unknown selector");
        input.set(bytes4(0xdeadbeef), configs);
    }
}

contract MockOPCM {
    event UpgradeCalled(address indexed sysCfgProxy, address indexed proxyAdmin, bytes32 indexed absolutePrestate);

    function upgrade(OPContractsManager.OpChainConfig[] memory _opChainConfigs) public {
        emit UpgradeCalled(
            address(_opChainConfigs[0].systemConfigProxy),
            address(_opChainConfigs[0].proxyAdmin),
            Claim.unwrap(_opChainConfigs[0].absolutePrestate)
        );
    }
}

contract UpgradeOPChain_Test is Test {
    MockOPCM mockOPCM;
    UpgradeOPChainInput uoci;
    OPContractsManager.OpChainConfig config;
    UpgradeOPChain upgradeOPChain;
    address prank;

    event UpgradeCalled(address indexed sysCfgProxy, address indexed proxyAdmin, bytes32 indexed absolutePrestate);

    function setUp() public virtual {
        mockOPCM = new MockOPCM();
        uoci = new UpgradeOPChainInput();
        uoci.set(uoci.opcm.selector, address(mockOPCM));
        config = OPContractsManager.OpChainConfig({
            systemConfigProxy: ISystemConfig(makeAddr("systemConfigProxy")),
            proxyAdmin: IProxyAdmin(makeAddr("proxyAdmin")),
            absolutePrestate: Claim.wrap(keccak256("absolutePrestate"))
        });
        OPContractsManager.OpChainConfig[] memory configs = new OPContractsManager.OpChainConfig[](1);
        configs[0] = config;
        uoci.set(uoci.opChainConfigs.selector, configs);
        prank = makeAddr("prank");
        uoci.set(uoci.prank.selector, prank);
        upgradeOPChain = new UpgradeOPChain();
    }

    function test_upgrade_succeeds() public {
        // UpgradeCalled should be emitted by the prank since it's a delegate call.
        vm.expectEmit(address(prank));
        emit UpgradeCalled(
            address(config.systemConfigProxy), address(config.proxyAdmin), Claim.unwrap(config.absolutePrestate)
        );
        upgradeOPChain.run(uoci);
    }
}
