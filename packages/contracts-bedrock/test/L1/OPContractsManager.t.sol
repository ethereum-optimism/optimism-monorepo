// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { Test, stdStorage, StdStorage } from "forge-std/Test.sol";
import { CommonTest } from "test/setup/CommonTest.sol";
import { DeployOPChain_TestBase } from "test/opcm/DeployOPChain.t.sol";
import { DelegateCaller } from "test/mocks/Callers.sol";

// Scripts
import { DeployOPChainInput } from "scripts/deploy/DeployOPChain.s.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

// Libraries
import { EIP1967Helper } from "test/mocks/EIP1967Helper.sol";
import { Blueprint } from "src/libraries/Blueprint.sol";
import { ForgeArtifacts } from "scripts/libraries/ForgeArtifacts.sol";
import { Bytes } from "src/libraries/Bytes.sol";

// Interfaces
import { IProxyAdmin } from "interfaces/universal/IProxyAdmin.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { IProtocolVersions } from "interfaces/L1/IProtocolVersions.sol";
import { IPreimageOracle } from "interfaces/cannon/IPreimageOracle.sol";
import { IPermissionedDisputeGame } from "interfaces/dispute/IPermissionedDisputeGame.sol";
import { IDelayedWETH } from "interfaces/dispute/IDelayedWETH.sol";
import { IFaultDisputeGame } from "interfaces/dispute/IFaultDisputeGame.sol";
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";
import { IOPContractsManager } from "interfaces/L1/IOPContractsManager.sol";

// Contracts
import { OPContractsManager } from "src/L1/OPContractsManager.sol";
import { Blueprint } from "src/libraries/Blueprint.sol";
import { DisputeGameFactory } from "src/dispute/DisputeGameFactory.sol";
import { L1ERC721Bridge } from "src/L1/L1ERC721Bridge.sol";
import { OptimismPortal2 } from "src/L1/OptimismPortal2.sol";
import { SystemConfig } from "src/L1/SystemConfig.sol";
import { OptimismMintableERC20Factory } from "src/universal/OptimismMintableERC20Factory.sol";
import { L1CrossDomainMessenger } from "src/L1/L1CrossDomainMessenger.sol";
import { L1StandardBridge } from "src/L1/L1StandardBridge.sol";
import { DisputeGameFactory } from "src/dispute/DisputeGameFactory.sol";
import { IBigStepper } from "interfaces/dispute/IBigStepper.sol";
import { DelayedWETH } from "src/dispute/DelayedWETH.sol";
import { MIPS } from "src/cannon/MIPS.sol";
import { GameType, Duration, Hash, Claim } from "src/dispute/lib/LibUDT.sol";
import { OutputRoot } from "src/dispute/lib/Types.sol";
import { AnchorStateRegistry } from "src/dispute/AnchorStateRegistry.sol";
import { PreimageOracle } from "src/cannon/PreimageOracle.sol";

// Exposes internal functions for testing.
contract OPContractsManager_Harness is OPContractsManager {
    constructor(
        ISuperchainConfig _superchainConfig,
        IProtocolVersions _protocolVersions,
        string memory _l1ContractsRelease,
        Blueprints memory _blueprints,
        Implementations memory _implementations,
        address _upgradeController
    )
        OPContractsManager(
            _superchainConfig,
            _protocolVersions,
            _l1ContractsRelease,
            _blueprints,
            _implementations,
            _upgradeController
        )
    { }

    function chainIdToBatchInboxAddress_exposed(uint256 l2ChainId) public pure returns (address) {
        return super.chainIdToBatchInboxAddress(l2ChainId);
    }
}

// Unlike other test suites, we intentionally do not inherit from CommonTest or Setup. This is
// because OPContractsManager acts as a deploy script, so we start from a clean slate here and
// work OPContractsManager's deployment into the existing test setup, instead of using the existing
// test setup to deploy OPContractsManager. We do however inherit from DeployOPChain_TestBase so
// we can use its setup to deploy the implementations similarly to how a real deployment would
// happen.
contract OPContractsManager_Deploy_Test is DeployOPChain_TestBase {
    using stdStorage for StdStorage;

    event Deployed(uint256 indexed l2ChainId, address indexed deployer, bytes deployOutput);

    function setUp() public override {
        DeployOPChain_TestBase.setUp();

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
        doi.set(doi.gasLimit.selector, gasLimit);

        doi.set(doi.disputeGameType.selector, disputeGameType);
        doi.set(doi.disputeAbsolutePrestate.selector, disputeAbsolutePrestate);
        doi.set(doi.disputeMaxGameDepth.selector, disputeMaxGameDepth);
        doi.set(doi.disputeSplitDepth.selector, disputeSplitDepth);
        doi.set(doi.disputeClockExtension.selector, disputeClockExtension);
        doi.set(doi.disputeMaxClockDuration.selector, disputeMaxClockDuration);
    }

    // This helper function is used to convert the input struct type defined in DeployOPChain.s.sol
    // to the input struct type defined in OPContractsManager.sol.
    function toOPCMDeployInput(DeployOPChainInput _doi)
        internal
        view
        returns (IOPContractsManager.DeployInput memory)
    {
        return IOPContractsManager.DeployInput({
            roles: IOPContractsManager.Roles({
                opChainProxyAdminOwner: _doi.opChainProxyAdminOwner(),
                systemConfigOwner: _doi.systemConfigOwner(),
                batcher: _doi.batcher(),
                unsafeBlockSigner: _doi.unsafeBlockSigner(),
                proposer: _doi.proposer(),
                challenger: _doi.challenger()
            }),
            basefeeScalar: _doi.basefeeScalar(),
            blobBasefeeScalar: _doi.blobBaseFeeScalar(),
            l2ChainId: _doi.l2ChainId(),
            startingAnchorRoot: _doi.startingAnchorRoot(),
            saltMixer: _doi.saltMixer(),
            gasLimit: _doi.gasLimit(),
            disputeGameType: _doi.disputeGameType(),
            disputeAbsolutePrestate: _doi.disputeAbsolutePrestate(),
            disputeMaxGameDepth: _doi.disputeMaxGameDepth(),
            disputeSplitDepth: _doi.disputeSplitDepth(),
            disputeClockExtension: _doi.disputeClockExtension(),
            disputeMaxClockDuration: _doi.disputeMaxClockDuration()
        });
    }

    function test_deploy_l2ChainIdEqualsZero_reverts() public {
        IOPContractsManager.DeployInput memory deployInput = toOPCMDeployInput(doi);
        deployInput.l2ChainId = 0;
        vm.expectRevert(IOPContractsManager.InvalidChainId.selector);
        opcm.deploy(deployInput);
    }

    function test_deploy_l2ChainIdEqualsCurrentChainId_reverts() public {
        IOPContractsManager.DeployInput memory deployInput = toOPCMDeployInput(doi);
        deployInput.l2ChainId = block.chainid;

        vm.expectRevert(IOPContractsManager.InvalidChainId.selector);
        opcm.deploy(deployInput);
    }

    function test_deploy_succeeds() public {
        vm.expectEmit(true, true, true, false); // TODO precompute the expected `deployOutput`.
        emit Deployed(doi.l2ChainId(), address(this), bytes(""));
        opcm.deploy(toOPCMDeployInput(doi));
    }
}

// These tests use the harness which exposes internal functions for testing.
contract OPContractsManager_InternalMethods_Test is Test {
    OPContractsManager_Harness opcmHarness;

    function setUp() public {
        ISuperchainConfig superchainConfigProxy = ISuperchainConfig(makeAddr("superchainConfig"));
        IProtocolVersions protocolVersionsProxy = IProtocolVersions(makeAddr("protocolVersions"));
        address upgradeController = makeAddr("upgradeController");
        OPContractsManager.Blueprints memory emptyBlueprints;
        OPContractsManager.Implementations memory emptyImpls;
        vm.etch(address(superchainConfigProxy), hex"01");
        vm.etch(address(protocolVersionsProxy), hex"01");

        opcmHarness = new OPContractsManager_Harness({
            _superchainConfig: superchainConfigProxy,
            _protocolVersions: protocolVersionsProxy,
            _l1ContractsRelease: "dev",
            _blueprints: emptyBlueprints,
            _implementations: emptyImpls,
            _upgradeController: upgradeController
        });
    }

    function test_calculatesBatchInboxAddress_succeeds() public view {
        // These test vectors were calculated manually:
        //   1. Compute the bytes32 encoding of the chainId: bytes32(uint256(chainId));
        //   2. Hash it and manually take the first 19 bytes, and prefixed it with 0x00.
        uint256 chainId = 1234;
        address expected = 0x0017FA14b0d73Aa6A26D6b8720c1c84b50984f5C;
        address actual = opcmHarness.chainIdToBatchInboxAddress_exposed(chainId);
        vm.assertEq(expected, actual);

        chainId = type(uint256).max;
        expected = 0x00a9C584056064687E149968cBaB758a3376D22A;
        actual = opcmHarness.chainIdToBatchInboxAddress_exposed(chainId);
        vm.assertEq(expected, actual);
    }
}

contract OPContractsManager_Upgrade_Harness is CommonTest {
    // The Upgraded event emitted by the Proxy contract.
    event Upgraded(address indexed implementation);

    // The Upgraded event emitted by the OPContractsManager contract.
    event Upgraded(uint256 indexed l2ChainId, ISystemConfig indexed systemConfig, address indexed upgrader);

    // The AddressSet event emitted by the AddressManager contract.
    event AddressSet(string indexed name, address newAddress, address oldAddress);

    uint256 l2ChainId;
    IProxyAdmin proxyAdmin;
    address upgrader;
    IOPContractsManager.OpChain[] opChains;

    function setUp() public virtual override {
        super.disableUpgradedFork();
        super.setUp();
        if (!isForkTest()) {
            // This test is only supported in forked tests, as we are testing the upgrade.
            vm.skip(true);
        }

        proxyAdmin = IProxyAdmin(EIP1967Helper.getAdmin(address(systemConfig)));
        upgrader = proxyAdmin.owner();
        vm.label(upgrader, "ProxyAdmin Owner");

        opChains.push(IOPContractsManager.OpChain({ systemConfigProxy: systemConfig, proxyAdmin: proxyAdmin }));

        // Retrieve the l2ChainId, which was read from the superchain-registry, and saved in Artifacts
        // encoded as an address.
        l2ChainId = uint256(bytes32(bytes20(address(artifacts.mustGetAddress("L2ChainId")))) >> 96);

        delayedWETHPermissionedGameProxy =
            IDelayedWETH(payable(artifacts.mustGetAddress("PermissionedDelayedWETHProxy")));
        delayedWeth = IDelayedWETH(payable(artifacts.mustGetAddress("PermissionlessDelayedWETHProxy")));
        permissionedDisputeGame = IPermissionedDisputeGame(address(artifacts.mustGetAddress("PermissionedDisputeGame")));
        faultDisputeGame = IFaultDisputeGame(address(artifacts.mustGetAddress("FaultDisputeGame")));
    }
}

contract OPContractsManager_Upgrade_Test is OPContractsManager_Upgrade_Harness {
    function runUpgradeTestAndChecks(address delegateCaller) public {
        assertTrue(opcm.isRC(), "isRC should be true");
        bytes memory releaseBytes = bytes(opcm.l1ContractsRelease());
        assertEq(Bytes.slice(releaseBytes, releaseBytes.length - 3, 3), "-rc", "release should end with '-rc'");

        vm.etch(delegateCaller, vm.getDeployedCode("test/mocks/Callers.sol:DelegateCaller"));
        IOPContractsManager.Implementations memory impls = opcm.implementations();
        address oldL1CrossDomainMessenger = addressManager.getAddress("OVM_L1CrossDomainMessenger");

        expectEmitUpgraded(impls.systemConfigImpl, address(systemConfig));
        vm.expectEmit(address(addressManager));
        emit AddressSet("OVM_L1CrossDomainMessenger", impls.l1CrossDomainMessengerImpl, oldL1CrossDomainMessenger);
        // This is where we would emit an event for the L1StandardBridge however
        // the Chugsplash proxy does not emit such an event.
        expectEmitUpgraded(impls.l1ERC721BridgeImpl, address(l1ERC721Bridge));
        expectEmitUpgraded(impls.disputeGameFactoryImpl, address(disputeGameFactory));
        expectEmitUpgraded(impls.optimismPortalImpl, address(optimismPortal2));
        expectEmitUpgraded(impls.optimismMintableERC20FactoryImpl, address(l1OptimismMintableERC20Factory));
        expectEmitUpgraded(impls.anchorStateRegistryImpl, address(anchorStateRegistry));
        expectEmitUpgraded(impls.delayedWETHImpl, address(delayedWETHPermissionedGameProxy));
        if (address(delayedWeth) != address(0)) {
            expectEmitUpgraded(impls.delayedWETHImpl, address(delayedWeth));
        }
        vm.expectEmit(true, true, true, true, address(delegateCaller));
        emit Upgraded(l2ChainId, opChains[0].systemConfigProxy, address(delegateCaller));
        DelegateCaller(delegateCaller).dcForward(address(opcm), abi.encodeCall(IOPContractsManager.upgrade, (opChains)));
        vm.stopPrank();

        if (delegateCaller == upgrader) {
            assertFalse(opcm.isRC(), "isRC should be false");
            releaseBytes = bytes(opcm.l1ContractsRelease());
            assertNotEq(
                Bytes.slice(releaseBytes, releaseBytes.length - 3, 3), "-rc", "release should not end with '-rc'"
            );
        }
        assertEq(impls.systemConfigImpl, EIP1967Helper.getImplementation(address(systemConfig)));
        assertEq(impls.l1ERC721BridgeImpl, EIP1967Helper.getImplementation(address(l1ERC721Bridge)));
        assertEq(impls.disputeGameFactoryImpl, EIP1967Helper.getImplementation(address(disputeGameFactory)));
        assertEq(impls.optimismPortalImpl, EIP1967Helper.getImplementation(address(optimismPortal2)));
        assertEq(
            impls.optimismMintableERC20FactoryImpl,
            EIP1967Helper.getImplementation(address(l1OptimismMintableERC20Factory))
        );
        assertEq(impls.l1StandardBridgeImpl, EIP1967Helper.getImplementation(address(l1StandardBridge)));
        assertEq(impls.l1CrossDomainMessengerImpl, addressManager.getAddress("OVM_L1CrossDomainMessenger"));

        assertEq(impls.anchorStateRegistryImpl, EIP1967Helper.getImplementation(address(anchorStateRegistry)));
        assertEq(impls.delayedWETHImpl, EIP1967Helper.getImplementation(address(delayedWeth)));
        if (address(delayedWeth) != address(0)) {
            assertEq(impls.delayedWETHImpl, EIP1967Helper.getImplementation(address(delayedWeth)));
        }

        // TODO: ensure dispute games are updated (upcoming PR)
    }

    function test_upgrade_succeeds() public {
        // Run the upgrade test and checks
        runUpgradeTestAndChecks(upgrader);
    }

    function test_upgrade_nonUpgradeControllerDelegatecallerShouldNotSetIsRCToFalse_works(address _nonUpgradeController)
        public
    {
        if (
            _nonUpgradeController == upgrader || _nonUpgradeController == address(0)
                || _nonUpgradeController < address(0x4200000000000000000000000000000000000000)
                || _nonUpgradeController > address(0x4200000000000000000000000000000000000800)
                || _nonUpgradeController == address(vm)
                || _nonUpgradeController == 0x000000000000000000636F6e736F6c652e6c6f67
                || _nonUpgradeController == 0x4e59b44847b379578588920cA78FbF26c0B4956C
        ) {
            _nonUpgradeController = makeAddr("nonUpgradeController");
        }

        // Set the proxy admin owner to be the non-upgrade controller
        vm.store(address(proxyAdmin), bytes32(0), bytes32(uint256(uint160(_nonUpgradeController))));

        // Run the upgrade test and checks
        runUpgradeTestAndChecks(_nonUpgradeController);
    }

    function expectEmitUpgraded(address impl, address proxy) public {
        vm.expectEmit(proxy);
        emit Upgraded(impl);
    }
}

contract OPContractsManager_Upgrade_TestFails is OPContractsManager_Upgrade_Harness {
    function test_upgrade_notDelegateCalled_reverts() public {
        vm.prank(upgrader);
        vm.expectRevert(IOPContractsManager.OnlyDelegatecall.selector);
        opcm.upgrade(opChains);
    }

    function test_upgrade_superchainConfigMismatch_reverts() public {
        upgrader = proxyAdmin.owner();
        vm.etch(upgrader, vm.getDeployedCode("test/mocks/Callers.sol:DelegateCaller"));
        // Set the superchainConfig to a different address in the OptimismPortal2 contract.
        vm.store(
            address(optimismPortal2),
            bytes32(ForgeArtifacts.getSlot("OptimismPortal2", "superchainConfig").slot),
            bytes32(abi.encode(bob))
        );

        vm.expectRevert(
            abi.encodeWithSelector(IOPContractsManager.SuperchainConfigMismatch.selector, address(systemConfig))
        );
        DelegateCaller(upgrader).dcForward(address(opcm), abi.encodeCall(IOPContractsManager.upgrade, (opChains)));
    }
}

contract OPContractsManager_SetRC_Test is OPContractsManager_Upgrade_Harness {
    /// @notice Tests the setRC function can be set by the upgrade controller.
    function test_setRC_succeeds(bool _isRC) public {
        vm.prank(upgrader);

        opcm.setRC(_isRC);
        assertTrue(opcm.isRC() == _isRC, "isRC should be true");
        bytes memory releaseBytes = bytes(opcm.l1ContractsRelease());
        if (_isRC) {
            assertEq(Bytes.slice(releaseBytes, releaseBytes.length - 3, 3), "-rc", "release should end with '-rc'");
        } else {
            assertNotEq(
                Bytes.slice(releaseBytes, releaseBytes.length - 3, 3), "-rc", "release should not end with '-rc'"
            );
        }
    }

    /// @notice Tests the setRC function can not be set by non-upgrade controller.
    function test_setRC_nonUpgradeController_reverts(address _nonUpgradeController) public {
        if (
            _nonUpgradeController == upgrader || _nonUpgradeController == address(0)
                || _nonUpgradeController < address(0x4200000000000000000000000000000000000000)
                || _nonUpgradeController > address(0x4200000000000000000000000000000000000800)
                || _nonUpgradeController == address(vm)
                || _nonUpgradeController == 0x000000000000000000636F6e736F6c652e6c6f67
                || _nonUpgradeController == 0x4e59b44847b379578588920cA78FbF26c0B4956C
        ) {
            _nonUpgradeController = makeAddr("nonUpgradeController");
        }

        vm.prank(_nonUpgradeController);

        vm.expectRevert(IOPContractsManager.OnlyUpgradeController.selector);
        opcm.setRC(true);
    }
}

contract OPContractsManager_AddGameType_Test is Test {
    IOPContractsManager internal opcm;

    IOPContractsManager.DeployOutput internal chainDeployOutput;

    function setUp() public {
        ISuperchainConfig superchainConfigProxy = ISuperchainConfig(makeAddr("superchainConfig"));
        IProtocolVersions protocolVersionsProxy = IProtocolVersions(makeAddr("protocolVersions"));
        bytes32 salt = hex"01";
        IOPContractsManager.Blueprints memory blueprints;
        (blueprints.addressManager,) = Blueprint.create(vm.getCode("AddressManager"), salt);
        (blueprints.proxy,) = Blueprint.create(vm.getCode("Proxy"), salt);
        (blueprints.proxyAdmin,) = Blueprint.create(vm.getCode("ProxyAdmin"), salt);
        (blueprints.l1ChugSplashProxy,) = Blueprint.create(vm.getCode("L1ChugSplashProxy"), salt);
        (blueprints.resolvedDelegateProxy,) = Blueprint.create(vm.getCode("ResolvedDelegateProxy"), salt);
        (blueprints.permissionedDisputeGame1, blueprints.permissionedDisputeGame2) =
            Blueprint.create(vm.getCode("PermissionedDisputeGame"), salt);
        (blueprints.permissionlessDisputeGame1, blueprints.permissionlessDisputeGame2) =
            Blueprint.create(vm.getCode("FaultDisputeGame"), salt);

        IPreimageOracle oracle = IPreimageOracle(address(new PreimageOracle(126000, 86400)));

        IOPContractsManager.Implementations memory impls = IOPContractsManager.Implementations({
            l1ERC721BridgeImpl: address(new L1ERC721Bridge()),
            optimismPortalImpl: address(new OptimismPortal2(1, 1)),
            systemConfigImpl: address(new SystemConfig()),
            optimismMintableERC20FactoryImpl: address(new OptimismMintableERC20Factory()),
            l1CrossDomainMessengerImpl: address(new L1CrossDomainMessenger()),
            l1StandardBridgeImpl: address(new L1StandardBridge()),
            disputeGameFactoryImpl: address(new DisputeGameFactory()),
            anchorStateRegistryImpl: address(new AnchorStateRegistry()),
            delayedWETHImpl: address(new DelayedWETH(3)),
            mipsImpl: address(new MIPS(oracle))
        });

        vm.etch(address(superchainConfigProxy), hex"01");
        vm.etch(address(protocolVersionsProxy), hex"01");

        opcm = IOPContractsManager(
            DeployUtils.createDeterministic({
                _name: "OPContractsManager",
                _args: DeployUtils.encodeConstructor(
                    abi.encodeCall(
                        IOPContractsManager.__constructor__,
                        (superchainConfigProxy, protocolVersionsProxy, "dev", blueprints, impls, address(this))
                    )
                ),
                _salt: DeployUtils.DEFAULT_SALT
            })
        );

        chainDeployOutput = opcm.deploy(
            IOPContractsManager.DeployInput({
                roles: IOPContractsManager.Roles({
                    opChainProxyAdminOwner: address(this),
                    systemConfigOwner: address(this),
                    batcher: address(this),
                    unsafeBlockSigner: address(this),
                    proposer: address(this),
                    challenger: address(this)
                }),
                basefeeScalar: 1,
                blobBasefeeScalar: 1,
                startingAnchorRoot: abi.encode(OutputRoot({ root: Hash.wrap(hex"dead"), l2BlockNumber: 0 })),
                l2ChainId: 100,
                saltMixer: "hello",
                gasLimit: 30_000_000,
                disputeGameType: GameType.wrap(1),
                disputeAbsolutePrestate: Claim.wrap(
                    bytes32(hex"038512e02c4c3f7bdaec27d00edf55b7155e0905301e1a88083e4e0a6764d54c")
                ),
                disputeMaxGameDepth: 73,
                disputeSplitDepth: 30,
                disputeClockExtension: Duration.wrap(10800),
                disputeMaxClockDuration: Duration.wrap(302400)
            })
        );
    }

    function test_addGameType_permissioned_succeeds() public {
        IOPContractsManager.AddGameInput memory input = newGameInputFactory(true);
        IOPContractsManager.AddGameOutput memory output = addGameType(input);
        assertValidGameType(input, output);
        IPermissionedDisputeGame newPDG = IPermissionedDisputeGame(address(output.faultDisputeGame));
        IPermissionedDisputeGame oldPDG = chainDeployOutput.permissionedDisputeGame;
        assertEq(newPDG.proposer(), oldPDG.proposer(), "proposer mismatch");
        assertEq(newPDG.challenger(), oldPDG.challenger(), "challenger mismatch");
    }

    function test_addGameType_permissionless_succeeds() public {
        IOPContractsManager.AddGameInput memory input = newGameInputFactory(false);
        IOPContractsManager.AddGameOutput memory output = addGameType(input);
        assertValidGameType(input, output);
        IPermissionedDisputeGame notPDG = IPermissionedDisputeGame(address(output.faultDisputeGame));
        vm.expectRevert(); // nosemgrep: sol-safety-expectrevert-no-args
        notPDG.proposer();
    }

    function test_addGameType_reusedDelayedWETH_succeeds() public {
        IDelayedWETH delayedWETH = IDelayedWETH(payable(address(new DelayedWETH(1))));
        vm.etch(address(delayedWETH), hex"01");
        IOPContractsManager.AddGameInput memory input = newGameInputFactory(false);
        input.delayedWETH = delayedWETH;
        IOPContractsManager.AddGameOutput memory output = addGameType(input);
        assertValidGameType(input, output);
        assertEq(address(output.delayedWETH), address(delayedWETH), "delayedWETH address mismatch");
    }

    function test_addGameType_outOfOrderInputs_reverts() public {
        IOPContractsManager.AddGameInput memory input1 = newGameInputFactory(false);
        input1.disputeGameType = GameType.wrap(2);
        IOPContractsManager.AddGameInput memory input2 = newGameInputFactory(false);
        input2.disputeGameType = GameType.wrap(1);
        IOPContractsManager.AddGameInput[] memory inputs = new IOPContractsManager.AddGameInput[](2);
        inputs[0] = input1;
        inputs[1] = input2;

        // For the sake of completeness, we run the call again to validate the success behavior.
        (bool success,) = address(opcm).delegatecall(abi.encodeCall(IOPContractsManager.addGameType, (inputs)));
        assertFalse(success, "addGameType should have failed");
    }

    function test_addGameType_duplicateGameType_reverts() public {
        IOPContractsManager.AddGameInput memory input = newGameInputFactory(false);
        IOPContractsManager.AddGameInput[] memory inputs = new IOPContractsManager.AddGameInput[](2);
        inputs[0] = input;
        inputs[1] = input;

        // See test above for why we run the call twice.
        (bool success, bytes memory revertData) =
            address(opcm).delegatecall(abi.encodeCall(IOPContractsManager.addGameType, (inputs)));
        assertFalse(success, "addGameType should have failed");
        assertEq(bytes4(revertData), IOPContractsManager.InvalidGameConfigs.selector, "revertData mismatch");
    }

    function test_addGameType_zeroLengthInput_reverts() public {
        IOPContractsManager.AddGameInput[] memory inputs = new IOPContractsManager.AddGameInput[](0);

        (bool success, bytes memory revertData) =
            address(opcm).delegatecall(abi.encodeCall(IOPContractsManager.addGameType, (inputs)));
        assertFalse(success, "addGameType should have failed");
        assertEq(bytes4(revertData), IOPContractsManager.InvalidGameConfigs.selector, "revertData mismatch");
    }

    function test_addGameType_notDelegateCall_reverts() public {
        IOPContractsManager.AddGameInput memory input = newGameInputFactory(true);
        IOPContractsManager.AddGameInput[] memory inputs = new IOPContractsManager.AddGameInput[](1);
        inputs[0] = input;

        vm.expectRevert(IOPContractsManager.OnlyDelegatecall.selector);
        opcm.addGameType(inputs);
    }

    function addGameType(IOPContractsManager.AddGameInput memory input)
        internal
        returns (IOPContractsManager.AddGameOutput memory)
    {
        IOPContractsManager.AddGameInput[] memory inputs = new IOPContractsManager.AddGameInput[](1);
        inputs[0] = input;

        (bool success, bytes memory rawGameOut) =
            address(opcm).delegatecall(abi.encodeCall(IOPContractsManager.addGameType, (inputs)));
        assertTrue(success, "addGameType failed");

        IOPContractsManager.AddGameOutput[] memory addGameOutAll =
            abi.decode(rawGameOut, (IOPContractsManager.AddGameOutput[]));
        return addGameOutAll[0];
    }

    function newGameInputFactory(bool permissioned) internal view returns (IOPContractsManager.AddGameInput memory) {
        return IOPContractsManager.AddGameInput({
            saltMixer: "hello",
            systemConfig: chainDeployOutput.systemConfigProxy,
            proxyAdmin: chainDeployOutput.opChainProxyAdmin,
            delayedWETH: IDelayedWETH(payable(address(0))),
            disputeGameType: GameType.wrap(2000),
            disputeAbsolutePrestate: Claim.wrap(bytes32(hex"deadbeef1234")),
            disputeMaxGameDepth: 73,
            disputeSplitDepth: 30,
            disputeClockExtension: Duration.wrap(10800),
            disputeMaxClockDuration: Duration.wrap(302400),
            initialBond: 1 ether,
            vm: IBigStepper(address(opcm.implementations().mipsImpl)),
            permissioned: permissioned
        });
    }

    function assertValidGameType(
        IOPContractsManager.AddGameInput memory agi,
        IOPContractsManager.AddGameOutput memory ago
    )
        internal
        view
    {
        // Check the config for the game itself
        assertEq(ago.faultDisputeGame.gameType().raw(), agi.disputeGameType.raw(), "gameType mismatch");
        assertEq(
            ago.faultDisputeGame.absolutePrestate().raw(),
            agi.disputeAbsolutePrestate.raw(),
            "absolutePrestate mismatch"
        );
        assertEq(ago.faultDisputeGame.maxGameDepth(), agi.disputeMaxGameDepth, "maxGameDepth mismatch");
        assertEq(ago.faultDisputeGame.splitDepth(), agi.disputeSplitDepth, "splitDepth mismatch");
        assertEq(
            ago.faultDisputeGame.clockExtension().raw(), agi.disputeClockExtension.raw(), "clockExtension mismatch"
        );
        assertEq(
            ago.faultDisputeGame.maxClockDuration().raw(),
            agi.disputeMaxClockDuration.raw(),
            "maxClockDuration mismatch"
        );
        assertEq(address(ago.faultDisputeGame.vm()), address(agi.vm), "vm address mismatch");
        assertEq(address(ago.faultDisputeGame.weth()), address(ago.delayedWETH), "delayedWETH address mismatch");
        assertEq(
            address(ago.faultDisputeGame.anchorStateRegistry()),
            address(chainDeployOutput.anchorStateRegistryProxy),
            "ASR address mismatch"
        );

        // Check the DGF
        assertEq(
            chainDeployOutput.disputeGameFactoryProxy.gameImpls(agi.disputeGameType).gameType().raw(),
            agi.disputeGameType.raw(),
            "gameType mismatch"
        );
        assertEq(
            address(chainDeployOutput.disputeGameFactoryProxy.gameImpls(agi.disputeGameType)),
            address(ago.faultDisputeGame),
            "gameImpl address mismatch"
        );
        assertEq(address(ago.faultDisputeGame.weth()), address(ago.delayedWETH), "weth address mismatch");
        assertEq(
            chainDeployOutput.disputeGameFactoryProxy.initBonds(agi.disputeGameType), agi.initialBond, "bond mismatch"
        );
    }
}
