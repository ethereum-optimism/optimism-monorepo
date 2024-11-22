// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Test } from "forge-std/Test.sol";

import { DeploySuperchainInput, DeploySuperchain, DeploySuperchainOutput } from "scripts/deploy/DeploySuperchain.s.sol";
import {
    DeployImplementationsInput,
    DeployImplementations,
    DeployImplementationsInterop,
    DeployImplementationsOutput
} from "scripts/deploy/DeployImplementations.s.sol";
import { DeployOPChainInput, DeployOPChain, DeployOPChainOutput } from "scripts/deploy/DeployOPChain.s.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

import { IProxyAdmin } from "src/universal/interfaces/IProxyAdmin.sol";

import { IAddressManager } from "src/legacy/interfaces/IAddressManager.sol";
import { IAnchorStateRegistry } from "src/dispute/interfaces/IAnchorStateRegistry.sol";
import { IFaultDisputeGame } from "src/dispute/interfaces/IFaultDisputeGame.sol";
import { IPermissionedDisputeGame } from "src/dispute/interfaces/IPermissionedDisputeGame.sol";
import { IL1ChugSplashProxy } from "src/legacy/interfaces/IL1ChugSplashProxy.sol";
import { IResolvedDelegateProxy } from "src/legacy/interfaces/IResolvedDelegateProxy.sol";

import { ISuperchainConfig } from "src/L1/interfaces/ISuperchainConfig.sol";
import { IProtocolVersions, ProtocolVersion } from "src/L1/interfaces/IProtocolVersions.sol";
import { OPContractsManager } from "src/L1/OPContractsManager.sol";
import { IProxy } from "src/universal/interfaces/IProxy.sol";

import { Claim, Duration, GameType, GameTypes, Hash, OutputRoot } from "src/dispute/lib/Types.sol";

contract DeployOPChainInput_Test is Test {
    DeployOPChainInput doi;

    // Define defaults.
    address opChainProxyAdminOwner = makeAddr("opChainProxyAdminOwner");
    address systemConfigOwner = makeAddr("systemConfigOwner");
    address batcher = makeAddr("batcher");
    address unsafeBlockSigner = makeAddr("unsafeBlockSigner");
    address proposer = makeAddr("proposer");
    address challenger = makeAddr("challenger");
    address opcm = makeAddr("opcm");
    uint32 basefeeScalar = 100;
    uint32 blobBaseFeeScalar = 200;
    uint256 l2ChainId = 300;
    string saltMixer = "saltMixer";

    function setUp() public {
        doi = new DeployOPChainInput();
    }

    function test_set_succeeds() public {
        doi.set(doi.opChainProxyAdminOwner.selector, opChainProxyAdminOwner);
        doi.set(doi.systemConfigOwner.selector, systemConfigOwner);
        doi.set(doi.batcher.selector, batcher);
        doi.set(doi.unsafeBlockSigner.selector, unsafeBlockSigner);
        doi.set(doi.proposer.selector, proposer);
        doi.set(doi.challenger.selector, challenger);
        doi.set(doi.basefeeScalar.selector, basefeeScalar);
        doi.set(doi.blobBaseFeeScalar.selector, blobBaseFeeScalar);
        doi.set(doi.l2ChainId.selector, l2ChainId);
        doi.set(doi.allowCustomDisputeParameters.selector, true);
        doi.set(doi.opcm.selector, opcm);
        vm.etch(opcm, hex"01");

        // Compare the default inputs to the getter methods.
        assertEq(opChainProxyAdminOwner, doi.opChainProxyAdminOwner(), "200");
        assertEq(systemConfigOwner, doi.systemConfigOwner(), "300");
        assertEq(batcher, doi.batcher(), "400");
        assertEq(unsafeBlockSigner, doi.unsafeBlockSigner(), "500");
        assertEq(proposer, doi.proposer(), "600");
        assertEq(challenger, doi.challenger(), "700");
        assertEq(basefeeScalar, doi.basefeeScalar(), "800");
        assertEq(blobBaseFeeScalar, doi.blobBaseFeeScalar(), "900");
        assertEq(l2ChainId, doi.l2ChainId(), "1000");
        assertEq(opcm, address(doi.opcm()), "1100");
        assertEq(true, doi.allowCustomDisputeParameters(), "1200");
    }

    function test_getters_whenNotSet_reverts() public {
        bytes memory expectedErr = "DeployOPChainInput: not set";

        vm.expectRevert(expectedErr);
        doi.opChainProxyAdminOwner();

        vm.expectRevert(expectedErr);
        doi.systemConfigOwner();

        vm.expectRevert(expectedErr);
        doi.batcher();

        vm.expectRevert(expectedErr);
        doi.unsafeBlockSigner();

        vm.expectRevert(expectedErr);
        doi.proposer();

        vm.expectRevert(expectedErr);
        doi.challenger();

        vm.expectRevert(expectedErr);
        doi.basefeeScalar();

        vm.expectRevert(expectedErr);
        doi.blobBaseFeeScalar();

        vm.expectRevert(expectedErr);
        doi.l2ChainId();
    }
}

contract DeployOPChainOutput_Test is Test {
    DeployOPChainOutput doo;

    // We set the non proxy contracts in storage because doing it locally in 'test_set_succeeds' function results in
    // stack too deep.
    IAddressManager addressManager = DeployUtils.buildAddressManager();
    IProxyAdmin opChainProxyAdmin = IProxyAdmin(makeAddr("opChainProxyAdmin"));
    IAnchorStateRegistry anchorStateRegistryImpl = IAnchorStateRegistry(makeAddr("anchorStateRegistryImpl"));
    IFaultDisputeGame faultDisputeGame = IFaultDisputeGame(makeAddr("faultDisputeGame"));
    IPermissionedDisputeGame permissionedDisputeGame = IPermissionedDisputeGame(makeAddr("permissionedDisputeGame"));

    function setUp() public {
        doo = new DeployOPChainOutput();
    }

    function test_set_succeeds() public {
        vm.etch(address(opChainProxyAdmin), hex"01");
        (IProxy l1ERC721BridgeProxy) = DeployUtils.buildERC1967ProxyWithImpl("l1ERC721BridgeProxy");
        (IProxy systemConfigProxy) = DeployUtils.buildERC1967ProxyWithImpl("systemConfigProxy");
        (IProxy optimismMintableERC20FactoryProxy) =
            DeployUtils.buildERC1967ProxyWithImpl("optimismMintableERC20FactoryProxy");
        (IL1ChugSplashProxy l1StandardBridgeProxy) = DeployUtils.buildL1ChugSplashProxyWithImpl("l1StandardBridgeProxy");
        (IResolvedDelegateProxy l1CrossDomainMessengerProxy) =
            DeployUtils.buildResolvedDelegateProxyWithImpl(addressManager, "OVM_L1CrossDomainMessenger");
        (IProxy optimismPortalProxy) = DeployUtils.buildERC1967ProxyWithImpl("optimismPortalProxy");
        (IProxy disputeGameFactoryProxy) = DeployUtils.buildERC1967ProxyWithImpl("disputeGameFactoryProxy");
        (IProxy anchorStateRegistryProxy) = DeployUtils.buildERC1967ProxyWithImpl("anchorStateRegistryProxy");
        vm.etch(address(anchorStateRegistryImpl), hex"01");
        vm.etch(address(faultDisputeGame), hex"01");
        vm.etch(address(permissionedDisputeGame), hex"01");
        // TODO: Eventually switch from Permissioned to Permissionless.
        // (IProxy delayedWETHPermissionlessGameProxy) =
        // DeployUtils.buildERC1967ProxyWithImpl("delayedWETHPermissionlessGameProxy");
        (IProxy delayedWETHPermissionedGameProxy) =
            DeployUtils.buildERC1967ProxyWithImpl("delayedWETHPermissionedGameProxy");

        doo.set(doo.opChainProxyAdmin.selector, address(opChainProxyAdmin));
        doo.set(doo.addressManager.selector, address(addressManager));
        doo.set(doo.l1ERC721BridgeProxy.selector, address(l1ERC721BridgeProxy));
        doo.set(doo.systemConfigProxy.selector, address(systemConfigProxy));
        doo.set(doo.optimismMintableERC20FactoryProxy.selector, address(optimismMintableERC20FactoryProxy));
        doo.set(doo.l1StandardBridgeProxy.selector, address(l1StandardBridgeProxy));
        doo.set(doo.l1CrossDomainMessengerProxy.selector, address(l1CrossDomainMessengerProxy));
        doo.set(doo.optimismPortalProxy.selector, address(optimismPortalProxy));
        doo.set(doo.disputeGameFactoryProxy.selector, address(disputeGameFactoryProxy));
        doo.set(doo.anchorStateRegistryProxy.selector, address(anchorStateRegistryProxy));
        doo.set(doo.anchorStateRegistryImpl.selector, address(anchorStateRegistryImpl));
        doo.set(doo.faultDisputeGame.selector, address(faultDisputeGame));
        doo.set(doo.permissionedDisputeGame.selector, address(permissionedDisputeGame));
        doo.set(doo.delayedWETHPermissionedGameProxy.selector, address(delayedWETHPermissionedGameProxy));
        // TODO: Eventually switch from Permissioned to Permissionless.
        // doo.set(doo.delayedWETHPermissionlessGameProxy.selector, address(delayedWETHPermissionlessGameProxy));

        assertEq(address(opChainProxyAdmin), address(doo.opChainProxyAdmin()), "100");
        assertEq(address(addressManager), address(doo.addressManager()), "200");
        assertEq(address(l1ERC721BridgeProxy), address(doo.l1ERC721BridgeProxy()), "300");
        assertEq(address(systemConfigProxy), address(doo.systemConfigProxy()), "400");
        assertEq(address(optimismMintableERC20FactoryProxy), address(doo.optimismMintableERC20FactoryProxy()), "500");
        assertEq(address(l1StandardBridgeProxy), address(doo.l1StandardBridgeProxy()), "600");
        assertEq(address(l1CrossDomainMessengerProxy), address(doo.l1CrossDomainMessengerProxy()), "700");
        assertEq(address(optimismPortalProxy), address(doo.optimismPortalProxy()), "800");
        assertEq(address(disputeGameFactoryProxy), address(doo.disputeGameFactoryProxy()), "900");
        assertEq(address(anchorStateRegistryProxy), address(doo.anchorStateRegistryProxy()), "1100");
        assertEq(address(anchorStateRegistryImpl), address(doo.anchorStateRegistryImpl()), "1200");
        assertEq(address(faultDisputeGame), address(doo.faultDisputeGame()), "1300");
        assertEq(address(permissionedDisputeGame), address(doo.permissionedDisputeGame()), "1400");
        assertEq(address(delayedWETHPermissionedGameProxy), address(doo.delayedWETHPermissionedGameProxy()), "1500");
        // TODO: Eventually switch from Permissioned to Permissionless.
        // assertEq(address(delayedWETHPermissionlessGameProxy), address(doo.delayedWETHPermissionlessGameProxy()),
        // "1600");
    }

    function test_getters_whenNotSet_reverts() public {
        bytes memory expectedErr = "DeployUtils: zero address";

        vm.expectRevert(expectedErr);
        doo.opChainProxyAdmin();

        vm.expectRevert(expectedErr);
        doo.addressManager();

        vm.expectRevert(expectedErr);
        doo.l1ERC721BridgeProxy();

        vm.expectRevert(expectedErr);
        doo.systemConfigProxy();

        vm.expectRevert(expectedErr);
        doo.optimismMintableERC20FactoryProxy();

        vm.expectRevert(expectedErr);
        doo.l1StandardBridgeProxy();

        vm.expectRevert(expectedErr);
        doo.l1CrossDomainMessengerProxy();

        vm.expectRevert(expectedErr);
        doo.optimismPortalProxy();

        vm.expectRevert(expectedErr);
        doo.disputeGameFactoryProxy();

        vm.expectRevert(expectedErr);
        doo.anchorStateRegistryProxy();

        vm.expectRevert(expectedErr);
        doo.anchorStateRegistryImpl();

        vm.expectRevert(expectedErr);
        doo.faultDisputeGame();

        vm.expectRevert(expectedErr);
        doo.permissionedDisputeGame();

        vm.expectRevert(expectedErr);
        doo.delayedWETHPermissionedGameProxy();

        // TODO: Eventually switch from Permissioned to Permissionless.
        // vm.expectRevert(expectedErr);
        // doo.delayedWETHPermissionlessGameProxy();
    }

    function test_getters_whenAddrHasNoCode_reverts() public {
        address emptyAddr = makeAddr("emptyAddr");
        bytes memory expectedErr = bytes(string.concat("DeployUtils: no code at ", vm.toString(emptyAddr)));

        doo.set(doo.opChainProxyAdmin.selector, emptyAddr);
        vm.expectRevert(expectedErr);
        doo.opChainProxyAdmin();

        doo.set(doo.addressManager.selector, emptyAddr);
        vm.expectRevert(expectedErr);
        doo.addressManager();

        doo.set(doo.l1ERC721BridgeProxy.selector, emptyAddr);
        vm.expectRevert(expectedErr);
        doo.l1ERC721BridgeProxy();

        doo.set(doo.systemConfigProxy.selector, emptyAddr);
        vm.expectRevert(expectedErr);
        doo.systemConfigProxy();

        doo.set(doo.optimismMintableERC20FactoryProxy.selector, emptyAddr);
        vm.expectRevert(expectedErr);
        doo.optimismMintableERC20FactoryProxy();

        doo.set(doo.l1StandardBridgeProxy.selector, emptyAddr);
        vm.expectRevert(expectedErr);
        doo.l1StandardBridgeProxy();

        doo.set(doo.l1CrossDomainMessengerProxy.selector, emptyAddr);
        vm.expectRevert(expectedErr);
        doo.l1CrossDomainMessengerProxy();

        doo.set(doo.optimismPortalProxy.selector, emptyAddr);
        vm.expectRevert(expectedErr);
        doo.optimismPortalProxy();

        doo.set(doo.disputeGameFactoryProxy.selector, emptyAddr);
        vm.expectRevert(expectedErr);
        doo.disputeGameFactoryProxy();

        doo.set(doo.anchorStateRegistryProxy.selector, emptyAddr);
        vm.expectRevert(expectedErr);
        doo.anchorStateRegistryProxy();

        doo.set(doo.anchorStateRegistryImpl.selector, emptyAddr);
        vm.expectRevert(expectedErr);
        doo.anchorStateRegistryImpl();

        doo.set(doo.faultDisputeGame.selector, emptyAddr);
        vm.expectRevert(expectedErr);
        doo.faultDisputeGame();

        doo.set(doo.permissionedDisputeGame.selector, emptyAddr);
        vm.expectRevert(expectedErr);
        doo.permissionedDisputeGame();

        doo.set(doo.delayedWETHPermissionedGameProxy.selector, emptyAddr);
        vm.expectRevert(expectedErr);
        doo.delayedWETHPermissionedGameProxy();

        // TODO: Eventually switch from Permissioned to Permissionless.
        // doo.set(doo.delayedWETHPermissionlessGameProxy.selector, emptyAddr);
        // vm.expectRevert(expectedErr);
        // doo.delayedWETHPermissionlessGameProxy();
    }
}

// To mimic a production environment, we default to integration tests here that actually run the
// DeploySuperchain and DeployImplementations scripts.
contract DeployOPChain_TestBase is Test {
    DeployOPChain deployOPChain;
    DeployOPChainInput doi;
    DeployOPChainOutput doo;

    // Define default inputs for DeploySuperchain.
    address superchainProxyAdminOwner = makeAddr("defaultSuperchainProxyAdminOwner");
    address protocolVersionsOwner = makeAddr("defaultProtocolVersionsOwner");
    address guardian = makeAddr("defaultGuardian");
    bool paused = false;
    ProtocolVersion requiredProtocolVersion = ProtocolVersion.wrap(1);
    ProtocolVersion recommendedProtocolVersion = ProtocolVersion.wrap(2);

    // Define default inputs for DeployImplementations.
    // `superchainConfigProxy` and `protocolVersionsProxy` are set during `setUp` since they are
    // outputs of the previous step.
    uint256 withdrawalDelaySeconds = 100;
    uint256 minProposalSizeBytes = 200;
    uint256 challengePeriodSeconds = 300;
    uint256 proofMaturityDelaySeconds = 400;
    uint256 disputeGameFinalityDelaySeconds = 500;
    string release = "dev-release"; // this means implementation contracts will be deployed
    ISuperchainConfig superchainConfigProxy;
    IProtocolVersions protocolVersionsProxy;

    // Define default inputs for DeployOPChain.
    // `opcm` is set during `setUp` since it is an output of the previous step.
    address opChainProxyAdminOwner = makeAddr("defaultOPChainProxyAdminOwner");
    address systemConfigOwner = makeAddr("defaultSystemConfigOwner");
    address batcher = makeAddr("defaultBatcher");
    address unsafeBlockSigner = makeAddr("defaultUnsafeBlockSigner");
    address proposer = makeAddr("defaultProposer");
    address challenger = makeAddr("defaultChallenger");
    uint32 basefeeScalar = 100;
    uint32 blobBaseFeeScalar = 200;
    uint256 l2ChainId = 300;
    IAnchorStateRegistry.StartingAnchorRoot[] startingAnchorRoots;
    OPContractsManager opcm = OPContractsManager(address(0));
    string saltMixer = "defaultSaltMixer";
    uint64 gasLimit = 60_000_000;
    // Configurable dispute game parameters.
    uint32 disputeGameType = GameType.unwrap(GameTypes.PERMISSIONED_CANNON);
    bytes32 disputeAbsolutePrestate = hex"038512e02c4c3f7bdaec27d00edf55b7155e0905301e1a88083e4e0a6764d54c";
    uint256 disputeMaxGameDepth = 73;
    uint256 disputeSplitDepth = 30;
    uint64 disputeClockExtension = Duration.unwrap(Duration.wrap(3 hours));
    uint64 disputeMaxClockDuration = Duration.unwrap(Duration.wrap(3.5 days));

    function setUp() public virtual {
        // Set defaults for reference types
        uint256 cannonBlock = 400;
        uint256 permissionedBlock = 500;
        startingAnchorRoots.push(
            IAnchorStateRegistry.StartingAnchorRoot({
                gameType: GameTypes.CANNON,
                outputRoot: OutputRoot({ root: Hash.wrap(keccak256("defaultOutputRootCannon")), l2BlockNumber: cannonBlock })
            })
        );
        startingAnchorRoots.push(
            IAnchorStateRegistry.StartingAnchorRoot({
                gameType: GameTypes.PERMISSIONED_CANNON,
                outputRoot: OutputRoot({
                    root: Hash.wrap(keccak256("defaultOutputRootPermissioned")),
                    l2BlockNumber: permissionedBlock
                })
            })
        );

        // Configure and deploy Superchain contracts
        DeploySuperchain deploySuperchain = new DeploySuperchain();
        (DeploySuperchainInput dsi, DeploySuperchainOutput dso) = deploySuperchain.etchIOContracts();

        dsi.set(dsi.superchainProxyAdminOwner.selector, superchainProxyAdminOwner);
        dsi.set(dsi.protocolVersionsOwner.selector, protocolVersionsOwner);
        dsi.set(dsi.guardian.selector, guardian);
        dsi.set(dsi.paused.selector, paused);
        dsi.set(dsi.requiredProtocolVersion.selector, requiredProtocolVersion);
        dsi.set(dsi.recommendedProtocolVersion.selector, recommendedProtocolVersion);

        deploySuperchain.run(dsi, dso);

        // Populate the inputs for DeployImplementations based on the output of DeploySuperchain.
        superchainConfigProxy = dso.superchainConfigProxy();
        protocolVersionsProxy = dso.protocolVersionsProxy();

        // Configure and deploy Implementation contracts
        DeployImplementations deployImplementations = createDeployImplementationsContract();
        (DeployImplementationsInput dii, DeployImplementationsOutput dio) = deployImplementations.etchIOContracts();

        dii.set(dii.withdrawalDelaySeconds.selector, withdrawalDelaySeconds);
        dii.set(dii.minProposalSizeBytes.selector, minProposalSizeBytes);
        dii.set(dii.challengePeriodSeconds.selector, challengePeriodSeconds);
        dii.set(dii.proofMaturityDelaySeconds.selector, proofMaturityDelaySeconds);
        dii.set(dii.disputeGameFinalityDelaySeconds.selector, disputeGameFinalityDelaySeconds);
        dii.set(dii.mipsVersion.selector, 1);
        dii.set(dii.l1ContractsRelease.selector, release);
        dii.set(dii.superchainConfigProxy.selector, address(superchainConfigProxy));
        dii.set(dii.protocolVersionsProxy.selector, address(protocolVersionsProxy));
        // End users of the DeployImplementations contract will need to set the `standardVersionsToml`.
        string memory standardVersionsTomlPath =
            string.concat(vm.projectRoot(), "/test/fixtures/standard-versions.toml");
        string memory standardVersionsToml = vm.readFile(standardVersionsTomlPath);
        dii.set(dii.standardVersionsToml.selector, standardVersionsToml);

        deployImplementations.run(dii, dio);

        // Deploy DeployOpChain, but defer populating the input values to the test suites inheriting this contract.
        deployOPChain = new DeployOPChain();
        (doi, doo) = deployOPChain.etchIOContracts();

        // Set the OPContractsManager input for DeployOPChain.
        opcm = dio.opcm();
    }

    // See the function of the same name in the `DeployImplementations_Test` contract of
    // `DeployImplementations.t.sol` for more details on why we use this method.
    function createDeployImplementationsContract() internal virtual returns (DeployImplementations) {
        return new DeployImplementations();
    }
}

contract DeployOPChain_Test is DeployOPChain_TestBase {
    function hash(bytes32 _seed, uint256 _i) internal pure returns (bytes32) {
        return keccak256(abi.encode(_seed, _i));
    }

    function testFuzz_run_memory_succeeds(bytes32 _seed) public {
        opChainProxyAdminOwner = address(uint160(uint256(hash(_seed, 0))));
        systemConfigOwner = address(uint160(uint256(hash(_seed, 1))));
        batcher = address(uint160(uint256(hash(_seed, 2))));
        unsafeBlockSigner = address(uint160(uint256(hash(_seed, 3))));
        proposer = address(uint160(uint256(hash(_seed, 4))));
        challenger = address(uint160(uint256(hash(_seed, 5))));
        basefeeScalar = uint32(uint256(hash(_seed, 6)));
        blobBaseFeeScalar = uint32(uint256(hash(_seed, 7)));
        l2ChainId = uint256(hash(_seed, 8));

        // Set the initial anchor states. The typical usage we expect is to pass in one root per game type.
        uint256 cannonBlock = uint256(hash(_seed, 9));
        uint256 permissionedBlock = uint256(hash(_seed, 10));
        startingAnchorRoots.push(
            IAnchorStateRegistry.StartingAnchorRoot({
                gameType: GameTypes.CANNON,
                outputRoot: OutputRoot({ root: Hash.wrap(keccak256(abi.encode(_seed, 11))), l2BlockNumber: cannonBlock })
            })
        );
        startingAnchorRoots.push(
            IAnchorStateRegistry.StartingAnchorRoot({
                gameType: GameTypes.PERMISSIONED_CANNON,
                outputRoot: OutputRoot({
                    root: Hash.wrap(keccak256(abi.encode(_seed, 12))),
                    l2BlockNumber: permissionedBlock
                })
            })
        );

        doi.set(doi.opChainProxyAdminOwner.selector, opChainProxyAdminOwner);
        doi.set(doi.systemConfigOwner.selector, systemConfigOwner);
        doi.set(doi.batcher.selector, batcher);
        doi.set(doi.unsafeBlockSigner.selector, unsafeBlockSigner);
        doi.set(doi.proposer.selector, proposer);
        doi.set(doi.challenger.selector, challenger);
        doi.set(doi.basefeeScalar.selector, basefeeScalar);
        doi.set(doi.blobBaseFeeScalar.selector, blobBaseFeeScalar);
        doi.set(doi.l2ChainId.selector, l2ChainId);
        doi.set(doi.opcm.selector, address(opcm));
        doi.set(doi.saltMixer.selector, saltMixer);
        doi.set(doi.gasLimit.selector, gasLimit);
        doi.set(doi.disputeGameType.selector, disputeGameType);
        doi.set(doi.disputeAbsolutePrestate.selector, disputeAbsolutePrestate);
        doi.set(doi.disputeMaxGameDepth.selector, disputeMaxGameDepth);
        doi.set(doi.disputeSplitDepth.selector, disputeSplitDepth);
        doi.set(doi.disputeClockExtension.selector, disputeClockExtension);
        doi.set(doi.disputeMaxClockDuration.selector, disputeMaxClockDuration);

        deployOPChain.run(doi, doo);

        // TODO Add fault proof contract assertions below once OPCM fully supports them.

        // Assert that individual input fields were properly set based on the inputs.
        assertEq(opChainProxyAdminOwner, doi.opChainProxyAdminOwner(), "100");
        assertEq(systemConfigOwner, doi.systemConfigOwner(), "200");
        assertEq(batcher, doi.batcher(), "300");
        assertEq(unsafeBlockSigner, doi.unsafeBlockSigner(), "400");
        assertEq(proposer, doi.proposer(), "500");
        assertEq(challenger, doi.challenger(), "600");
        assertEq(basefeeScalar, doi.basefeeScalar(), "700");
        assertEq(blobBaseFeeScalar, doi.blobBaseFeeScalar(), "800");
        assertEq(l2ChainId, doi.l2ChainId(), "900");
        assertEq(saltMixer, doi.saltMixer(), "1000");
        assertEq(gasLimit, doi.gasLimit(), "1100");
        assertEq(disputeGameType, GameType.unwrap(doi.disputeGameType()), "1200");
        assertEq(disputeAbsolutePrestate, Claim.unwrap(doi.disputeAbsolutePrestate()), "1300");
        assertEq(disputeMaxGameDepth, doi.disputeMaxGameDepth(), "1400");
        assertEq(disputeSplitDepth, doi.disputeSplitDepth(), "1500");
        assertEq(disputeClockExtension, Duration.unwrap(doi.disputeClockExtension()), "1600");
        assertEq(disputeMaxClockDuration, Duration.unwrap(doi.disputeMaxClockDuration()), "1700");

        // Assert inputs were properly passed through to the contract initializers.
        assertEq(address(doo.opChainProxyAdmin().owner()), opChainProxyAdminOwner, "2100");
        assertEq(address(doo.systemConfigProxy().owner()), systemConfigOwner, "2200");
        address batcherActual = address(uint160(uint256(doo.systemConfigProxy().batcherHash())));
        assertEq(batcherActual, batcher, "2300");
        assertEq(address(doo.systemConfigProxy().unsafeBlockSigner()), unsafeBlockSigner, "2400");
        assertEq(address(doo.permissionedDisputeGame().proposer()), proposer, "2500");
        assertEq(address(doo.permissionedDisputeGame().challenger()), challenger, "2600");

        // TODO once we deploy the Permissionless Dispute Game
        // assertEq(address(doo.faultDisputeGame().proposer()), proposer, "2610");
        // assertEq(address(doo.faultDisputeGame().challenger()), challenger, "2620");

        // Verify that the initial bonds are zero.
        assertEq(doo.disputeGameFactoryProxy().initBonds(GameTypes.CANNON), 0, "2700");
        assertEq(doo.disputeGameFactoryProxy().initBonds(GameTypes.PERMISSIONED_CANNON), 0, "2800");

        (Hash actualRoot,) = doo.anchorStateRegistryProxy().anchors(GameTypes.PERMISSIONED_CANNON);
        assertEq(Hash.unwrap(actualRoot), 0xdead000000000000000000000000000000000000000000000000000000000000, "2900");
        assertEq(doo.permissionedDisputeGame().l2BlockNumber(), 0, "3000");
        assertEq(
            Claim.unwrap(doo.permissionedDisputeGame().absolutePrestate()),
            0x038512e02c4c3f7bdaec27d00edf55b7155e0905301e1a88083e4e0a6764d54c,
            "3100"
        );
        assertEq(Duration.unwrap(doo.permissionedDisputeGame().clockExtension()), 10800, "3200");
        assertEq(Duration.unwrap(doo.permissionedDisputeGame().maxClockDuration()), 302400, "3300");
        assertEq(doo.permissionedDisputeGame().splitDepth(), 30, "3400");
        assertEq(doo.permissionedDisputeGame().maxGameDepth(), 73, "3500");

        assertEq(address(doo.opChainProxyAdmin().addressManager().owner()), address(doo.opChainProxyAdmin()), "3600");
        assertEq(address(doo.opChainProxyAdmin().addressManager()), address(doo.addressManager()), "3700");
        assertEq(address(doo.opChainProxyAdmin().owner()), opChainProxyAdminOwner, "3800");
    }

    function test_customDisputeGame_customDisabled_reverts() public {
        setDOI();
        doi.set(doi.disputeSplitDepth.selector, disputeSplitDepth + 1);
        vm.expectRevert("DPG-90");
        deployOPChain.run(doi, doo);
    }

    function test_customDisputeGame_customEnabled_succeeds() public {
        setDOI();
        doi.set(doi.allowCustomDisputeParameters.selector, true);
        doi.set(doi.disputeSplitDepth.selector, disputeSplitDepth + 1);
        deployOPChain.run(doi, doo);
        assertEq(doo.permissionedDisputeGame().splitDepth(), disputeSplitDepth + 1);
    }

    function setDOI() internal {
        doi.set(doi.opChainProxyAdminOwner.selector, opChainProxyAdminOwner);
        doi.set(doi.systemConfigOwner.selector, systemConfigOwner);
        doi.set(doi.batcher.selector, batcher);
        doi.set(doi.unsafeBlockSigner.selector, unsafeBlockSigner);
        doi.set(doi.proposer.selector, proposer);
        doi.set(doi.challenger.selector, challenger);
        doi.set(doi.basefeeScalar.selector, basefeeScalar);
        doi.set(doi.blobBaseFeeScalar.selector, blobBaseFeeScalar);
        doi.set(doi.l2ChainId.selector, l2ChainId);
        doi.set(doi.opcm.selector, address(opcm));
        doi.set(doi.saltMixer.selector, saltMixer);
        doi.set(doi.gasLimit.selector, gasLimit);
        doi.set(doi.disputeGameType.selector, disputeGameType);
        doi.set(doi.disputeAbsolutePrestate.selector, disputeAbsolutePrestate);
        doi.set(doi.disputeMaxGameDepth.selector, disputeMaxGameDepth);
        doi.set(doi.disputeSplitDepth.selector, disputeSplitDepth);
        doi.set(doi.disputeClockExtension.selector, disputeClockExtension);
        doi.set(doi.disputeMaxClockDuration.selector, disputeMaxClockDuration);
    }
}

contract DeployOPChain_Test_Interop is DeployOPChain_Test {
    function createDeployImplementationsContract() internal override returns (DeployImplementations) {
        return new DeployImplementationsInterop();
    }
}
