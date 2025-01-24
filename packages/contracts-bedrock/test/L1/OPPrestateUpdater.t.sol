// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;
import { console2 as console } from "forge-std/console2.sol";
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
import { ISemver } from "interfaces/universal/ISemver.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
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

        IPreimageOracle oracle = IPreimageOracle(DeployUtils.create1("PreimageOracle", abi.encode(126000, 86400)));

        IOPContractsManager.Implementations memory impls = IOPContractsManager.Implementations({
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
        IOPContractsManager.AddGameInput memory input = newGameInputFactory({permissioned: false});
        input.disputeGameType = GameTypes.CANNON;
        addGameType(input);

        prestateUpdater = OPPrestateUpdater(
            DeployUtils.createDeterministic({
                _name: "OPPrestateUpdater",
                _args: DeployUtils.encodeConstructor(
                    abi.encodeCall(
                        IOPContractsManager.__constructor__,
                        (ISuperchainConfig(address(this)), IProtocolVersions(address(this)), "dev", blueprints, impls, address(0))
                    )
                ),
                _salt: DeployUtils.DEFAULT_SALT
            })
        );
    }

    function test_updatePrestate_succeeds() public {
        OPPrestateUpdater.PrestateUpdateInput[] memory inputs = new OPPrestateUpdater.PrestateUpdateInput[](1);
        inputs[0] = OPPrestateUpdater.PrestateUpdateInput({
            opChain: OPContractsManager.OpChain({
                systemConfigProxy: chainDeployOutput.systemConfigProxy,
                proxyAdmin: chainDeployOutput.opChainProxyAdmin
            }),
            permissionedDisputePrestate: Claim.wrap(bytes32(hex"ABBA")),
            faultDisputePrestate: Claim.wrap(bytes32(hex"ACDC"))
        });
        address proxyAdminOwner = chainDeployOutput.opChainProxyAdmin.owner();

        vm.etch(address(proxyAdminOwner), vm.getDeployedCode("test/mocks/Callers.sol:DelegateCaller"));
        DelegateCaller(proxyAdminOwner).dcForward(address(prestateUpdater), abi.encodeCall(OPPrestateUpdater.updatePrestate, (inputs)));

        IFaultDisputeGame fdg = IFaultDisputeGame(
            address(
                IDisputeGameFactory(chainDeployOutput.systemConfigProxy.disputeGameFactory()).gameImpls(GameTypes.CANNON)
            )
        );
        IPermissionedDisputeGame pdg = IPermissionedDisputeGame(
            address(
                IDisputeGameFactory(chainDeployOutput.systemConfigProxy.disputeGameFactory()).gameImpls(GameTypes.PERMISSIONED_CANNON)
            )
        );

        assertEq(pdg.absolutePrestate().raw(), inputs[0].permissionedDisputePrestate.raw(), "pdg prestate mismatch");
        assertEq(fdg.absolutePrestate().raw(), inputs[0].faultDisputePrestate.raw(), "fdg prestate mismatch");
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
