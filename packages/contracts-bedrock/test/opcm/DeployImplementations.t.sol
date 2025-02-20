// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { Test, stdStorage, StdStorage } from "forge-std/Test.sol";

// Libraries
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";
import { Chains } from "scripts/libraries/Chains.sol";

// Interfaces
import { IDelayedWETH } from "interfaces/dispute/IDelayedWETH.sol";
import { IPreimageOracle } from "interfaces/cannon/IPreimageOracle.sol";
import { IMIPS } from "interfaces/cannon/IMIPS.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { IAnchorStateRegistry } from "interfaces/dispute/IAnchorStateRegistry.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { IProtocolVersions } from "interfaces/L1/IProtocolVersions.sol";
import { IOPContractsManager } from "interfaces/L1/IOPContractsManager.sol";
import { IOptimismPortal2 } from "interfaces/L1/IOptimismPortal2.sol";
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";
import { IL1CrossDomainMessenger } from "interfaces/L1/IL1CrossDomainMessenger.sol";
import { IL1ERC721Bridge } from "interfaces/L1/IL1ERC721Bridge.sol";
import { IL1StandardBridge } from "interfaces/L1/IL1StandardBridge.sol";
import { IOptimismMintableERC20Factory } from "interfaces/universal/IOptimismMintableERC20Factory.sol";
import { IProxyAdmin } from "interfaces/universal/IProxyAdmin.sol";
import { IProxy } from "interfaces/universal/IProxy.sol";

import {
    DeployImplementationsInput,
    DeployImplementations,
    DeployImplementationsInterop,
    DeployImplementationsOutput
} from "scripts/deploy/DeployImplementations.s.sol";

contract DeployImplementationsInput_Test is Test {
    DeployImplementationsInput dii;

    uint256 withdrawalDelaySeconds = 100;
    uint256 minProposalSizeBytes = 200;
    uint256 challengePeriodSeconds = 300;
    uint256 proofMaturityDelaySeconds = 400;
    uint256 disputeGameFinalityDelaySeconds = 500;
    string release = "dev-release"; // this means implementation contracts will be deployed
    ISuperchainConfig superchainConfigProxy = ISuperchainConfig(makeAddr("superchainConfigProxy"));
    IProtocolVersions protocolVersionsProxy = IProtocolVersions(makeAddr("protocolVersionsProxy"));

    function setUp() public {
        dii = new DeployImplementationsInput();
    }

    function test_getters_whenNotSet_reverts() public {
        vm.expectRevert("DeployImplementationsInput: not set");
        dii.withdrawalDelaySeconds();

        vm.expectRevert("DeployImplementationsInput: not set");
        dii.minProposalSizeBytes();

        vm.expectRevert("DeployImplementationsInput: not set");
        dii.challengePeriodSeconds();

        vm.expectRevert("DeployImplementationsInput: not set");
        dii.proofMaturityDelaySeconds();

        vm.expectRevert("DeployImplementationsInput: not set");
        dii.disputeGameFinalityDelaySeconds();

        vm.expectRevert("DeployImplementationsInput: not set");
        dii.l1ContractsRelease();

        vm.expectRevert("DeployImplementationsInput: not set");
        dii.superchainConfigProxy();

        vm.expectRevert("DeployImplementationsInput: not set");
        dii.protocolVersionsProxy();

        vm.expectRevert("DeployImplementationsInput: not set");
        dii.upgradeController();
    }
}

contract DeployImplementationsOutput_Test is Test {
    DeployImplementationsOutput dio;

    function setUp() public {
        dio = new DeployImplementationsOutput();
    }

    function test_set_succeeds() public {
        IOPContractsManager opcm = IOPContractsManager(address(makeAddr("opcm")));
        IOptimismPortal2 optimismPortalImpl = IOptimismPortal2(payable(makeAddr("optimismPortalImpl")));
        IDelayedWETH delayedWETHImpl = IDelayedWETH(payable(makeAddr("delayedWETHImpl")));
        IPreimageOracle preimageOracleSingleton = IPreimageOracle(makeAddr("preimageOracleSingleton"));
        IMIPS mipsSingleton = IMIPS(makeAddr("mipsSingleton"));
        ISystemConfig systemConfigImpl = ISystemConfig(makeAddr("systemConfigImpl"));
        IL1CrossDomainMessenger l1CrossDomainMessengerImpl =
            IL1CrossDomainMessenger(makeAddr("l1CrossDomainMessengerImpl"));
        IL1ERC721Bridge l1ERC721BridgeImpl = IL1ERC721Bridge(makeAddr("l1ERC721BridgeImpl"));
        IL1StandardBridge l1StandardBridgeImpl = IL1StandardBridge(payable(makeAddr("l1StandardBridgeImpl")));
        IOptimismMintableERC20Factory optimismMintableERC20FactoryImpl =
            IOptimismMintableERC20Factory(makeAddr("optimismMintableERC20FactoryImpl"));
        IDisputeGameFactory disputeGameFactoryImpl = IDisputeGameFactory(makeAddr("disputeGameFactoryImpl"));
        IAnchorStateRegistry anchorStateRegistryImpl = IAnchorStateRegistry(makeAddr("anchorStateRegistryImpl"));

        vm.etch(address(opcm), hex"01");
        vm.etch(address(optimismPortalImpl), hex"01");
        vm.etch(address(delayedWETHImpl), hex"01");
        vm.etch(address(preimageOracleSingleton), hex"01");
        vm.etch(address(mipsSingleton), hex"01");
        vm.etch(address(systemConfigImpl), hex"01");
        vm.etch(address(l1CrossDomainMessengerImpl), hex"01");
        vm.etch(address(l1ERC721BridgeImpl), hex"01");
        vm.etch(address(l1StandardBridgeImpl), hex"01");
        vm.etch(address(optimismMintableERC20FactoryImpl), hex"01");
        vm.etch(address(disputeGameFactoryImpl), hex"01");
        vm.etch(address(anchorStateRegistryImpl), hex"01");
        dio.set(dio.opcm.selector, address(opcm));
        dio.set(dio.optimismPortalImpl.selector, address(optimismPortalImpl));
        dio.set(dio.delayedWETHImpl.selector, address(delayedWETHImpl));
        dio.set(dio.preimageOracleSingleton.selector, address(preimageOracleSingleton));
        dio.set(dio.mipsSingleton.selector, address(mipsSingleton));
        dio.set(dio.systemConfigImpl.selector, address(systemConfigImpl));
        dio.set(dio.l1CrossDomainMessengerImpl.selector, address(l1CrossDomainMessengerImpl));
        dio.set(dio.l1ERC721BridgeImpl.selector, address(l1ERC721BridgeImpl));
        dio.set(dio.l1StandardBridgeImpl.selector, address(l1StandardBridgeImpl));
        dio.set(dio.optimismMintableERC20FactoryImpl.selector, address(optimismMintableERC20FactoryImpl));
        dio.set(dio.disputeGameFactoryImpl.selector, address(disputeGameFactoryImpl));
        dio.set(dio.anchorStateRegistryImpl.selector, address(anchorStateRegistryImpl));

        assertEq(address(opcm), address(dio.opcm()), "50");
        assertEq(address(optimismPortalImpl), address(dio.optimismPortalImpl()), "100");
        assertEq(address(delayedWETHImpl), address(dio.delayedWETHImpl()), "200");
        assertEq(address(preimageOracleSingleton), address(dio.preimageOracleSingleton()), "300");
        assertEq(address(mipsSingleton), address(dio.mipsSingleton()), "400");
        assertEq(address(systemConfigImpl), address(dio.systemConfigImpl()), "500");
        assertEq(address(l1CrossDomainMessengerImpl), address(dio.l1CrossDomainMessengerImpl()), "600");
        assertEq(address(l1ERC721BridgeImpl), address(dio.l1ERC721BridgeImpl()), "700");
        assertEq(address(l1StandardBridgeImpl), address(dio.l1StandardBridgeImpl()), "800");
        assertEq(address(optimismMintableERC20FactoryImpl), address(dio.optimismMintableERC20FactoryImpl()), "900");
        assertEq(address(disputeGameFactoryImpl), address(dio.disputeGameFactoryImpl()), "950");
        assertEq(address(anchorStateRegistryImpl), address(dio.anchorStateRegistryImpl()), "960");
    }

    function test_getters_whenNotSet_reverts() public {
        bytes memory expectedErr = "DeployUtils: zero address";

        vm.expectRevert(expectedErr);
        dio.optimismPortalImpl();

        vm.expectRevert(expectedErr);
        dio.delayedWETHImpl();

        vm.expectRevert(expectedErr);
        dio.preimageOracleSingleton();

        vm.expectRevert(expectedErr);
        dio.mipsSingleton();

        vm.expectRevert(expectedErr);
        dio.systemConfigImpl();

        vm.expectRevert(expectedErr);
        dio.l1CrossDomainMessengerImpl();

        vm.expectRevert(expectedErr);
        dio.l1ERC721BridgeImpl();

        vm.expectRevert(expectedErr);
        dio.l1StandardBridgeImpl();

        vm.expectRevert(expectedErr);
        dio.optimismMintableERC20FactoryImpl();

        vm.expectRevert(expectedErr);
        dio.disputeGameFactoryImpl();

        vm.expectRevert(expectedErr);
        dio.anchorStateRegistryImpl();
    }

    function test_getters_whenAddrHasNoCode_reverts() public {
        address emptyAddr = makeAddr("emptyAddr");
        bytes memory expectedErr = bytes(string.concat("DeployUtils: no code at ", vm.toString(emptyAddr)));

        dio.set(dio.optimismPortalImpl.selector, emptyAddr);
        vm.expectRevert(expectedErr);
        dio.optimismPortalImpl();

        dio.set(dio.delayedWETHImpl.selector, emptyAddr);
        vm.expectRevert(expectedErr);
        dio.delayedWETHImpl();

        dio.set(dio.preimageOracleSingleton.selector, emptyAddr);
        vm.expectRevert(expectedErr);
        dio.preimageOracleSingleton();

        dio.set(dio.mipsSingleton.selector, emptyAddr);
        vm.expectRevert(expectedErr);
        dio.mipsSingleton();

        dio.set(dio.systemConfigImpl.selector, emptyAddr);
        vm.expectRevert(expectedErr);
        dio.systemConfigImpl();

        dio.set(dio.l1CrossDomainMessengerImpl.selector, emptyAddr);
        vm.expectRevert(expectedErr);
        dio.l1CrossDomainMessengerImpl();

        dio.set(dio.l1ERC721BridgeImpl.selector, emptyAddr);
        vm.expectRevert(expectedErr);
        dio.l1ERC721BridgeImpl();

        dio.set(dio.l1StandardBridgeImpl.selector, emptyAddr);
        vm.expectRevert(expectedErr);
        dio.l1StandardBridgeImpl();

        dio.set(dio.optimismMintableERC20FactoryImpl.selector, emptyAddr);
        vm.expectRevert(expectedErr);
        dio.optimismMintableERC20FactoryImpl();
    }
}

contract DeployImplementations_Test is Test {
    using stdStorage for StdStorage;

    DeployImplementations deployImplementations;
    DeployImplementationsInput dii;
    DeployImplementationsOutput dio;

    // Define default inputs for testing.
    uint256 withdrawalDelaySeconds = 100;
    uint256 minProposalSizeBytes = 200;
    uint256 challengePeriodSeconds = 300;
    uint256 proofMaturityDelaySeconds = 400;
    uint256 disputeGameFinalityDelaySeconds = 500;
    ISuperchainConfig superchainConfigProxy = ISuperchainConfig(makeAddr("superchainConfigProxy"));
    IProtocolVersions protocolVersionsProxy = IProtocolVersions(makeAddr("protocolVersionsProxy"));
    IProxyAdmin superchainProxyAdmin = IProxyAdmin(makeAddr("superchainProxyAdmin"));
    address upgradeController = makeAddr("upgradeController");

    function setUp() public virtual {
        vm.etch(address(superchainConfigProxy), hex"01");
        vm.etch(address(protocolVersionsProxy), hex"01");
        deployImplementations = new DeployImplementations();
        (dii, dio) = deployImplementations.etchIOContracts();
    }

    // By deploying the `DeployImplementations` contract with this virtual function, we provide a
    // hook that child contracts can override to return a different implementation of the contract.
    // This lets us test e.g. the `DeployImplementationsInterop` contract without duplicating test code.
    function createDeployImplementationsContract() internal virtual returns (DeployImplementations) {
        return new DeployImplementations();
    }

    function hash(bytes32 _seed, uint256 _i) internal pure returns (bytes32) {
        return keccak256(abi.encode(_seed, _i));
    }

    function test_deployImplementation_succeeds() public {
        deployImplementations.deploySystemConfigImpl(dio);
        assertTrue(address(0) != address(dio.systemConfigImpl()));
    }

    function test_reuseImplementation_succeeds() public {
        string memory testRelease = "op-contracts/v1.6.0";
        dii.set(dii.l1ContractsRelease.selector, testRelease);
        dii.set(dii.proofMaturityDelaySeconds.selector, 1);
        dii.set(dii.disputeGameFinalityDelaySeconds.selector, 1);
        dii.set(dii.withdrawalDelaySeconds.selector, 1);
        dii.set(dii.minProposalSizeBytes.selector, 1);
        dii.set(dii.challengePeriodSeconds.selector, 1);
        dii.set(dii.mipsVersion.selector, 1);
        dii.set(dii.superchainConfigProxy.selector, address(superchainConfigProxy));
        dii.set(dii.protocolVersionsProxy.selector, address(protocolVersionsProxy));
        dii.set(dii.superchainProxyAdmin.selector, address(superchainProxyAdmin));
        dii.set(dii.upgradeController.selector, upgradeController);

        // Perform the initial deployment.
        deployImplementations.deploySuperchainConfigImpl(dio);
        deployImplementations.deployProtocolVersionsImpl(dio);
        deployImplementations.deploySystemConfigImpl(dio);
        deployImplementations.deployL1CrossDomainMessengerImpl(dio);
        deployImplementations.deployL1ERC721BridgeImpl(dio);
        deployImplementations.deployL1StandardBridgeImpl(dio);
        deployImplementations.deployOptimismMintableERC20FactoryImpl(dio);
        deployImplementations.deployOptimismPortalImpl(dii, dio);
        deployImplementations.deployDelayedWETHImpl(dii, dio);
        deployImplementations.deployPreimageOracleSingleton(dii, dio);
        deployImplementations.deployMipsSingleton(dii, dio);
        deployImplementations.deployDisputeGameFactoryImpl(dio);
        deployImplementations.deployAnchorStateRegistryImpl(dio);
        deployImplementations.deployOPContractsManager(dii, dio);

        // Store the original addresses.
        address systemConfigImpl = address(dio.systemConfigImpl());
        address l1CrossDomainMessengerImpl = address(dio.l1CrossDomainMessengerImpl());
        address l1ERC721BridgeImpl = address(dio.l1ERC721BridgeImpl());
        address l1StandardBridgeImpl = address(dio.l1StandardBridgeImpl());
        address optimismMintableERC20FactoryImpl = address(dio.optimismMintableERC20FactoryImpl());
        address optimismPortalImpl = address(dio.optimismPortalImpl());
        address delayedWETHImpl = address(dio.delayedWETHImpl());
        address preimageOracleSingleton = address(dio.preimageOracleSingleton());
        address mipsSingleton = address(dio.mipsSingleton());
        address disputeGameFactoryImpl = address(dio.disputeGameFactoryImpl());
        address anchorStateRegistryImpl = address(dio.anchorStateRegistryImpl());
        address opcm = address(dio.opcm());

        // Do the deployments again. Thi should be a noop.
        deployImplementations.deploySystemConfigImpl(dio);
        deployImplementations.deployL1CrossDomainMessengerImpl(dio);
        deployImplementations.deployL1ERC721BridgeImpl(dio);
        deployImplementations.deployL1StandardBridgeImpl(dio);
        deployImplementations.deployOptimismMintableERC20FactoryImpl(dio);
        deployImplementations.deployOptimismPortalImpl(dii, dio);
        deployImplementations.deployDelayedWETHImpl(dii, dio);
        deployImplementations.deployPreimageOracleSingleton(dii, dio);
        deployImplementations.deployMipsSingleton(dii, dio);
        deployImplementations.deployDisputeGameFactoryImpl(dio);
        deployImplementations.deployAnchorStateRegistryImpl(dio);
        deployImplementations.deployOPContractsManager(dii, dio);

        // Assert that the addresses did not change.
        assertEq(systemConfigImpl, address(dio.systemConfigImpl()), "100");
        assertEq(l1CrossDomainMessengerImpl, address(dio.l1CrossDomainMessengerImpl()), "200");
        assertEq(l1ERC721BridgeImpl, address(dio.l1ERC721BridgeImpl()), "300");
        assertEq(l1StandardBridgeImpl, address(dio.l1StandardBridgeImpl()), "400");
        assertEq(optimismMintableERC20FactoryImpl, address(dio.optimismMintableERC20FactoryImpl()), "500");
        assertEq(optimismPortalImpl, address(dio.optimismPortalImpl()), "600");
        assertEq(delayedWETHImpl, address(dio.delayedWETHImpl()), "700");
        assertEq(preimageOracleSingleton, address(dio.preimageOracleSingleton()), "800");
        assertEq(mipsSingleton, address(dio.mipsSingleton()), "900");
        assertEq(disputeGameFactoryImpl, address(dio.disputeGameFactoryImpl()), "1000");
        assertEq(anchorStateRegistryImpl, address(dio.anchorStateRegistryImpl()), "1100");
        assertEq(opcm, address(dio.opcm()), "1200");
    }

    function testFuzz_run_memory_succeeds(bytes32 _seed) public {
        withdrawalDelaySeconds = uint256(hash(_seed, 0));
        minProposalSizeBytes = uint256(hash(_seed, 1));
        challengePeriodSeconds = bound(uint256(hash(_seed, 2)), 0, type(uint64).max);
        proofMaturityDelaySeconds = uint256(hash(_seed, 3));
        disputeGameFinalityDelaySeconds = uint256(hash(_seed, 4));
        string memory release = string(bytes.concat(hash(_seed, 5)));
        protocolVersionsProxy = IProtocolVersions(address(uint160(uint256(hash(_seed, 7)))));

        // Must configure the ProxyAdmin contract.
        superchainProxyAdmin = IProxyAdmin(
            DeployUtils.create1({
                _name: "ProxyAdmin",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IProxyAdmin.__constructor__, (msg.sender)))
            })
        );
        superchainConfigProxy = ISuperchainConfig(
            DeployUtils.create1({
                _name: "Proxy",
                _args: DeployUtils.encodeConstructor(
                    abi.encodeCall(IProxy.__constructor__, (address(superchainProxyAdmin)))
                )
            })
        );

        ISuperchainConfig superchainConfigImpl = ISuperchainConfig(address(uint160(uint256(hash(_seed, 6)))));
        vm.prank(address(superchainProxyAdmin));
        IProxy(payable(address(superchainConfigProxy))).upgradeTo(address(superchainConfigImpl));

        vm.etch(address(superchainProxyAdmin), address(superchainProxyAdmin).code);
        vm.etch(address(superchainConfigProxy), address(superchainConfigProxy).code);
        vm.etch(address(protocolVersionsProxy), hex"01");

        dii.set(dii.withdrawalDelaySeconds.selector, withdrawalDelaySeconds);
        dii.set(dii.minProposalSizeBytes.selector, minProposalSizeBytes);
        dii.set(dii.challengePeriodSeconds.selector, challengePeriodSeconds);
        dii.set(dii.proofMaturityDelaySeconds.selector, proofMaturityDelaySeconds);
        dii.set(dii.disputeGameFinalityDelaySeconds.selector, disputeGameFinalityDelaySeconds);
        dii.set(dii.mipsVersion.selector, 1);
        dii.set(dii.l1ContractsRelease.selector, release);
        dii.set(dii.superchainConfigProxy.selector, address(superchainConfigProxy));
        dii.set(dii.protocolVersionsProxy.selector, address(protocolVersionsProxy));
        dii.set(dii.superchainProxyAdmin.selector, address(superchainProxyAdmin));
        dii.set(dii.upgradeController.selector, upgradeController);

        deployImplementations.run(dii, dio);

        // Assert that individual input fields were properly set based on the inputs.
        assertEq(withdrawalDelaySeconds, dii.withdrawalDelaySeconds(), "100");
        assertEq(minProposalSizeBytes, dii.minProposalSizeBytes(), "200");
        assertEq(challengePeriodSeconds, dii.challengePeriodSeconds(), "300");
        assertEq(proofMaturityDelaySeconds, dii.proofMaturityDelaySeconds(), "400");
        assertEq(disputeGameFinalityDelaySeconds, dii.disputeGameFinalityDelaySeconds(), "500");
        assertEq(1, dii.mipsVersion(), "512");
        assertEq(release, dii.l1ContractsRelease(), "525");
        assertEq(address(superchainConfigProxy), address(dii.superchainConfigProxy()), "550");
        assertEq(address(protocolVersionsProxy), address(dii.protocolVersionsProxy()), "575");
        assertEq(address(superchainProxyAdmin), address(dii.superchainProxyAdmin()), "600");
        assertEq(upgradeController, dii.upgradeController(), "625");

        // Architecture assertions.
        assertEq(address(dio.mipsSingleton().oracle()), address(dio.preimageOracleSingleton()), "600");

        // Ensure that `checkOutput` passes. This is called by the `run` function during execution,
        // so this just acts as a sanity check. It reverts on failure.
        dio.checkOutput(dii);
    }

    function setDefaults() internal {
        // Set the defaults.
        dii.set(dii.withdrawalDelaySeconds.selector, withdrawalDelaySeconds);
        dii.set(dii.minProposalSizeBytes.selector, minProposalSizeBytes);
        dii.set(dii.challengePeriodSeconds.selector, challengePeriodSeconds);
        dii.set(dii.proofMaturityDelaySeconds.selector, proofMaturityDelaySeconds);
        dii.set(dii.disputeGameFinalityDelaySeconds.selector, disputeGameFinalityDelaySeconds);
        dii.set(dii.mipsVersion.selector, 1);
        string memory release = "dev-release";
        dii.set(dii.l1ContractsRelease.selector, release);
        dii.set(dii.superchainConfigProxy.selector, address(superchainConfigProxy));
        dii.set(dii.protocolVersionsProxy.selector, address(protocolVersionsProxy));
        dii.set(dii.superchainProxyAdmin.selector, address(superchainProxyAdmin));
    }

    function testFuzz_run_largeChallengePeriodSeconds_reverts(uint256 _challengePeriodSeconds) public {
        setDefaults();
        // Set the challenge period to a value that is too large, using vm.store because the setter
        // method won't allow it.
        challengePeriodSeconds = bound(_challengePeriodSeconds, uint256(type(uint64).max) + 1, type(uint256).max);
        uint256 slot =
            stdstore.enable_packed_slots().target(address(dii)).sig(dii.challengePeriodSeconds.selector).find();
        vm.store(address(dii), bytes32(slot), bytes32(challengePeriodSeconds));

        vm.expectRevert("DeployImplementationsInput: challengePeriodSeconds too large");
        deployImplementations.run(dii, dio);
    }

    function test_run_deployMipsV1OnMainnetOrSepolia_reverts() public {
        setDefaults();
        dii.set(dii.mipsVersion.selector, 2);

        vm.chainId(Chains.Mainnet);
        vm.expectRevert("DeployImplementations: Only Mips32 should be deployed on Mainnet or Sepolia");
        deployImplementations.run(dii, dio);

        vm.chainId(Chains.Sepolia);
        vm.expectRevert("DeployImplementations: Only Mips32 should be deployed on Mainnet or Sepolia");
        deployImplementations.run(dii, dio);
    }
}

contract DeployImplementationsInterop_Test is DeployImplementations_Test {
    function createDeployImplementationsContract() internal override returns (DeployImplementations) {
        return new DeployImplementationsInterop();
    }
}
