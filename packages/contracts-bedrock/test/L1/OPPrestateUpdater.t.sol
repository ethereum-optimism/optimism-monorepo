// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { Test } from "forge-std/Test.sol";
import { DelegateCaller } from "test/mocks/Callers.sol";

// Scripts
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

// Libraries
import { Blueprint } from "src/libraries/Blueprint.sol";

// Interfaces
import { IProxyAdmin } from "interfaces/universal/IProxyAdmin.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { IProtocolVersions } from "interfaces/L1/IProtocolVersions.sol";
import { IPreimageOracle } from "interfaces/cannon/IPreimageOracle.sol";
import { IPermissionedDisputeGame } from "interfaces/dispute/IPermissionedDisputeGame.sol";
import { IDelayedWETH } from "interfaces/dispute/IDelayedWETH.sol";
import { IOPContractsManager } from "interfaces/L1/IOPContractsManager.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";

// Contracts
import { OPContractsManager } from "src/L1/OPContractsManager.sol";
import { OPPrestateUpdater } from "src/L1/OPPrestateUpdater.sol";
import { Blueprint } from "src/libraries/Blueprint.sol";
import { IBigStepper } from "interfaces/dispute/IBigStepper.sol";
import { GameType, Duration, Hash, Claim } from "src/dispute/lib/LibUDT.sol";
import { OutputRoot, GameTypes } from "src/dispute/lib/Types.sol";

contract OPPrestateUpdater_Test is Test {
    IOPContractsManager internal opcm;
    OPPrestateUpdater internal prestateUpdater;

    OPContractsManager.OpChainConfig[] internal opChainConfigs;
    OPContractsManager.AddGameInput[] internal gameInput;

    IOPContractsManager.DeployOutput internal chainDeployOutput;

    function setUp() public {
        IProxyAdmin superchainProxyAdmin = IProxyAdmin(makeAddr("superchainProxyAdmin"));
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

        IPreimageOracle oracle = IPreimageOracle(DeployUtils.create1("PreimageOracle", abi.encode(126000, 86400)));

        IOPContractsManager.Implementations memory impls = IOPContractsManager.Implementations({
            superchainConfigImpl: DeployUtils.create1("SuperchainConfig"),
            protocolVersionsImpl: DeployUtils.create1("ProtocolVersions"),
            l1ERC721BridgeImpl: DeployUtils.create1("L1ERC721Bridge"),
            optimismPortalImpl: DeployUtils.create1("OptimismPortal2", abi.encode(1, 1)),
            systemConfigImpl: DeployUtils.create1("SystemConfig"),
            optimismMintableERC20FactoryImpl: DeployUtils.create1("OptimismMintableERC20Factory"),
            l1CrossDomainMessengerImpl: DeployUtils.create1("L1CrossDomainMessenger"),
            l1StandardBridgeImpl: DeployUtils.create1("L1StandardBridge"),
            disputeGameFactoryImpl: DeployUtils.create1("DisputeGameFactory"),
            anchorStateRegistryImpl: DeployUtils.create1("AnchorStateRegistry"),
            delayedWETHImpl: DeployUtils.create1("DelayedWETH", abi.encode(3)),
            mipsImpl: DeployUtils.create1("MIPS", abi.encode(oracle))
        });

        vm.etch(address(superchainConfigProxy), hex"01");
        vm.etch(address(protocolVersionsProxy), hex"01");

        opcm = IOPContractsManager(
            DeployUtils.createDeterministic({
                _name: "OPContractsManager",
                _args: DeployUtils.encodeConstructor(
                    abi.encodeCall(
                        IOPContractsManager.__constructor__,
                        (
                            superchainConfigProxy,
                            protocolVersionsProxy,
                            superchainProxyAdmin,
                            "dev",
                            blueprints,
                            impls,
                            address(this)
                        )
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
                startingAnchorRoot: abi.encode(
                    OutputRoot({
                        root: Hash.wrap(0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef),
                        l2BlockNumber: 0
                    })
                ),
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

        // Also add a permissionless game
        IOPContractsManager.AddGameInput memory input = newGameInputFactory({ permissioned: false });
        input.disputeGameType = GameTypes.CANNON;
        addGameType(input);

        prestateUpdater = OPPrestateUpdater(
            DeployUtils.createDeterministic({
                _name: "OPPrestateUpdater",
                _args: DeployUtils.encodeConstructor(
                    abi.encodeCall(
                        IOPContractsManager.__constructor__,
                        (
                            ISuperchainConfig(address(this)),
                            IProtocolVersions(address(this)),
                            superchainProxyAdmin,
                            "dev",
                            blueprints,
                            impls,
                            address(0)
                        )
                    )
                ),
                _salt: DeployUtils.DEFAULT_SALT
            })
        );
    }

    function test_updatePrestate_withValidInput_succeeds() public {
        OPPrestateUpdater.PrestateUpdateInput[] memory inputs = new OPPrestateUpdater.PrestateUpdateInput[](1);
        inputs[0] = OPPrestateUpdater.PrestateUpdateInput({
            opChain: OPContractsManager.OpChainConfig({
                systemConfigProxy: chainDeployOutput.systemConfigProxy,
                proxyAdmin: chainDeployOutput.opChainProxyAdmin
            }),
            absolutePrestate: Claim.wrap(bytes32(hex"ABBA"))
        });
        address proxyAdminOwner = chainDeployOutput.opChainProxyAdmin.owner();

        vm.etch(address(proxyAdminOwner), vm.getDeployedCode("test/mocks/Callers.sol:DelegateCaller"));
        DelegateCaller(proxyAdminOwner).dcForward(
            address(prestateUpdater), abi.encodeCall(OPPrestateUpdater.updatePrestate, (inputs))
        );

        IPermissionedDisputeGame pdg = IPermissionedDisputeGame(
            address(
                IDisputeGameFactory(chainDeployOutput.systemConfigProxy.disputeGameFactory()).gameImpls(
                    GameTypes.PERMISSIONED_CANNON
                )
            )
        );

        assertEq(pdg.absolutePrestate().raw(), inputs[0].absolutePrestate.raw(), "pdg prestate mismatch");
    }

    function test_updatePrestate_whenPDGPrestateIsZero_reverts() public {
        OPPrestateUpdater.PrestateUpdateInput[] memory inputs = new OPPrestateUpdater.PrestateUpdateInput[](1);
        inputs[0] = OPPrestateUpdater.PrestateUpdateInput({
            opChain: OPContractsManager.OpChainConfig({
                systemConfigProxy: chainDeployOutput.systemConfigProxy,
                proxyAdmin: chainDeployOutput.opChainProxyAdmin
            }),
            absolutePrestate: Claim.wrap(bytes32(0))
        });

        address proxyAdminOwner = chainDeployOutput.opChainProxyAdmin.owner();
        vm.etch(address(proxyAdminOwner), vm.getDeployedCode("test/mocks/Callers.sol:DelegateCaller"));

        vm.expectRevert(OPPrestateUpdater.PDGPrestateRequired.selector);
        DelegateCaller(proxyAdminOwner).dcForward(
            address(prestateUpdater), abi.encodeCall(OPPrestateUpdater.updatePrestate, (inputs))
        );
    }

    function test_updatePrestate_whenFDGNotFound_reverts() public {
        OPPrestateUpdater.PrestateUpdateInput[] memory inputs = new OPPrestateUpdater.PrestateUpdateInput[](1);
        inputs[0] = OPPrestateUpdater.PrestateUpdateInput({
            opChain: OPContractsManager.OpChainConfig({
                systemConfigProxy: chainDeployOutput.systemConfigProxy,
                proxyAdmin: chainDeployOutput.opChainProxyAdmin
            }),
            absolutePrestate: Claim.wrap(bytes32(hex"ABBA"))
        });

        IDisputeGameFactory dgf = IDisputeGameFactory(chainDeployOutput.systemConfigProxy.disputeGameFactory());

        vm.mockCall(address(dgf), abi.encodeCall(dgf.gameImpls, GameTypes.CANNON), abi.encode(address(0)));

        address proxyAdminOwner = chainDeployOutput.opChainProxyAdmin.owner();
        vm.etch(address(proxyAdminOwner), vm.getDeployedCode("test/mocks/Callers.sol:DelegateCaller"));

        vm.expectRevert(OPPrestateUpdater.FDGNotFound.selector);
        DelegateCaller(proxyAdminOwner).dcForward(
            address(prestateUpdater), abi.encodeCall(OPPrestateUpdater.updatePrestate, (inputs))
        );
    }

    function test_deploy_notImplemented_reverts() public {
        OPContractsManager.DeployInput memory input = OPContractsManager.DeployInput({
            roles: OPContractsManager.Roles({
                opChainProxyAdminOwner: address(0),
                systemConfigOwner: address(0),
                batcher: address(0),
                unsafeBlockSigner: address(0),
                proposer: address(0),
                challenger: address(0)
            }),
            basefeeScalar: 0,
            blobBasefeeScalar: 0,
            l2ChainId: 0,
            startingAnchorRoot: bytes(abi.encode(0)),
            saltMixer: "",
            gasLimit: 0,
            disputeGameType: GameType.wrap(0),
            disputeAbsolutePrestate: Claim.wrap(0),
            disputeMaxGameDepth: 0,
            disputeSplitDepth: 0,
            disputeClockExtension: Duration.wrap(0),
            disputeMaxClockDuration: Duration.wrap(0)
        });

        vm.expectRevert(OPPrestateUpdater.NotImplemented.selector);
        prestateUpdater.deploy(input);
    }

    function test_upgrade_notImplemented_reverts() public {
        opChainConfigs.push(
            OPContractsManager.OpChainConfig({
                systemConfigProxy: ISystemConfig(address(0)),
                proxyAdmin: IProxyAdmin(address(0)),
                absolutePrestate: Claim.wrap(0)
            })
        );

        vm.expectRevert(OPPrestateUpdater.NotImplemented.selector);
        prestateUpdater.upgrade(opChainConfigs);
    }

    function test_addGameType_notImplemented_reverts() public {
        gameInput.push(
            OPContractsManager.AddGameInput({
                saltMixer: "hello",
                systemConfig: ISystemConfig(address(0)),
                proxyAdmin: IProxyAdmin(address(0)),
                delayedWETH: IDelayedWETH(payable(address(0))),
                disputeGameType: GameType.wrap(2000),
                disputeAbsolutePrestate: Claim.wrap(bytes32(hex"deadbeef1234")),
                disputeMaxGameDepth: 73,
                disputeSplitDepth: 30,
                disputeClockExtension: Duration.wrap(10800),
                disputeMaxClockDuration: Duration.wrap(302400),
                initialBond: 1 ether,
                vm: IBigStepper(address(opcm.implementations().mipsImpl)),
                permissioned: true
            })
        );

        vm.expectRevert(OPPrestateUpdater.NotImplemented.selector);
        prestateUpdater.addGameType(gameInput);
    }

    function test_l1ContractsRelease_works() public view {
        string memory result = "none";

        assertEq(result, prestateUpdater.l1ContractsRelease());
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
