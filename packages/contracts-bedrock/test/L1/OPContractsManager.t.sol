// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Test, stdStorage, StdStorage } from "forge-std/Test.sol";

import { DeployOPChainInput } from "scripts/deploy/DeployOPChain.s.sol";
import { DeployOPChain_TestBase } from "test/opcm/DeployOPChain.t.sol";

import { OPContractsManager } from "src/L1/OPContractsManager.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { IProtocolVersions } from "interfaces/L1/IProtocolVersions.sol";
import { IPreimageOracle } from "interfaces/cannon/IPreimageOracle.sol";
import { IPermissionedDisputeGame } from "interfaces/dispute/IPermissionedDisputeGame.sol";
import { IDelayedWETH } from "interfaces/dispute/IDelayedWETH.sol";

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
        Implementations memory _implementations
    )
        OPContractsManager(_superchainConfig, _protocolVersions, _l1ContractsRelease, _blueprints, _implementations)
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

    event Deployed(
        uint256 indexed outputVersion, uint256 indexed l2ChainId, address indexed deployer, bytes deployOutput
    );

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
    function toOPCMDeployInput(DeployOPChainInput _doi) internal view returns (OPContractsManager.DeployInput memory) {
        return OPContractsManager.DeployInput({
            roles: OPContractsManager.Roles({
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
        OPContractsManager.DeployInput memory deployInput = toOPCMDeployInput(doi);
        deployInput.l2ChainId = 0;
        vm.expectRevert(OPContractsManager.InvalidChainId.selector);
        opcm.deploy(deployInput);
    }

    function test_deploy_l2ChainIdEqualsCurrentChainId_reverts() public {
        OPContractsManager.DeployInput memory deployInput = toOPCMDeployInput(doi);
        deployInput.l2ChainId = block.chainid;

        vm.expectRevert(OPContractsManager.InvalidChainId.selector);
        opcm.deploy(deployInput);
    }

    function test_deploy_succeeds() public {
        vm.expectEmit(true, true, true, false); // TODO precompute the expected `deployOutput`.
        emit Deployed(0, doi.l2ChainId(), address(this), bytes(""));
        opcm.deploy(toOPCMDeployInput(doi));
    }
}

// These tests use the harness which exposes internal functions for testing.
contract OPContractsManager_InternalMethods_Test is Test {
    OPContractsManager_Harness opcmHarness;

    function setUp() public {
        ISuperchainConfig superchainConfigProxy = ISuperchainConfig(makeAddr("superchainConfig"));
        IProtocolVersions protocolVersionsProxy = IProtocolVersions(makeAddr("protocolVersions"));
        OPContractsManager.Blueprints memory emptyBlueprints;
        OPContractsManager.Implementations memory emptyImpls;
        vm.etch(address(superchainConfigProxy), hex"01");
        vm.etch(address(protocolVersionsProxy), hex"01");

        opcmHarness = new OPContractsManager_Harness({
            _superchainConfig: superchainConfigProxy,
            _protocolVersions: protocolVersionsProxy,
            _l1ContractsRelease: "dev",
            _blueprints: emptyBlueprints,
            _implementations: emptyImpls
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

contract OPContractsManager_AddGameType_Test is Test {
    OPContractsManager internal opcm;

    OPContractsManager.DeployOutput internal chainDeployOutput;

    function setUp() public {
        ISuperchainConfig superchainConfigProxy = ISuperchainConfig(makeAddr("superchainConfig"));
        IProtocolVersions protocolVersionsProxy = IProtocolVersions(makeAddr("protocolVersions"));
        bytes32 salt = hex"01";
        OPContractsManager.Blueprints memory blueprints;
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

        OPContractsManager.Implementations memory impls = OPContractsManager.Implementations({
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

        opcm = new OPContractsManager(superchainConfigProxy, protocolVersionsProxy, "dev", blueprints, impls);

        chainDeployOutput = opcm.deploy(
            OPContractsManager.DeployInput({
                roles: OPContractsManager.Roles({
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
        OPContractsManager.AddGameInput memory input = newGameInputFactory(true);
        OPContractsManager.AddGameOutput memory output = addGameType(input);
        assertValidGameType(input, output);
        IPermissionedDisputeGame newPDG = IPermissionedDisputeGame(address(output.faultDisputeGame));
        IPermissionedDisputeGame oldPDG = chainDeployOutput.permissionedDisputeGame;
        assertEq(newPDG.proposer(), oldPDG.proposer(), "proposer mismatch");
        assertEq(newPDG.challenger(), oldPDG.challenger(), "challenger mismatch");
    }

    function test_addGameType_permissionless_succeeds() public {
        OPContractsManager.AddGameInput memory input = newGameInputFactory(false);
        OPContractsManager.AddGameOutput memory output = addGameType(input);
        assertValidGameType(input, output);
        IPermissionedDisputeGame notPDG = IPermissionedDisputeGame(address(output.faultDisputeGame));
        vm.expectRevert(); // nosemgrep: sol-safety-expectrevert-no-args
        notPDG.proposer();
    }

    function test_addGameType_reusedDelayedWETH_succeeds() public {
        IDelayedWETH delayedWETH = IDelayedWETH(payable(address(new DelayedWETH(1))));
        vm.etch(address(delayedWETH), hex"01");
        OPContractsManager.AddGameInput memory input = newGameInputFactory(false);
        input.delayedWETH = delayedWETH;
        OPContractsManager.AddGameOutput memory output = addGameType(input);
        assertValidGameType(input, output);
        assertEq(address(output.delayedWETH), address(delayedWETH), "delayedWETH address mismatch");
    }

    function test_addGameType_outOfOrderInputs_reverts() public {
        OPContractsManager.AddGameInput memory input1 = newGameInputFactory(false);
        input1.disputeGameType = GameType.wrap(2);
        OPContractsManager.AddGameInput memory input2 = newGameInputFactory(false);
        input2.disputeGameType = GameType.wrap(1);
        OPContractsManager.AddGameInput[] memory inputs = new OPContractsManager.AddGameInput[](2);
        inputs[0] = input1;
        inputs[1] = input2;

        // For the sake of completeness, we run the call again to validate the success behavior.
        (bool success,) = address(opcm).delegatecall(abi.encodeCall(OPContractsManager.addGameType, (inputs)));
        assertFalse(success, "addGameType should have failed");
    }

    function test_addGameType_duplicateGameType_reverts() public {
        OPContractsManager.AddGameInput memory input = newGameInputFactory(false);
        OPContractsManager.AddGameInput[] memory inputs = new OPContractsManager.AddGameInput[](2);
        inputs[0] = input;
        inputs[1] = input;

        // See test above for why we run the call twice.
        (bool success, bytes memory revertData) =
            address(opcm).delegatecall(abi.encodeCall(OPContractsManager.addGameType, (inputs)));
        assertFalse(success, "addGameType should have failed");
        assertEq(bytes4(revertData), OPContractsManager.InvalidGameConfigs.selector, "revertData mismatch");
    }

    function test_addGameType_zeroLengthInput_reverts() public {
        OPContractsManager.AddGameInput[] memory inputs = new OPContractsManager.AddGameInput[](0);

        (bool success, bytes memory revertData) =
            address(opcm).delegatecall(abi.encodeCall(OPContractsManager.addGameType, (inputs)));
        assertFalse(success, "addGameType should have failed");
        assertEq(bytes4(revertData), OPContractsManager.InvalidGameConfigs.selector, "revertData mismatch");
    }

    function test_addGameType_notDelegateCall_reverts() public {
        OPContractsManager.AddGameInput memory input = newGameInputFactory(true);
        OPContractsManager.AddGameInput[] memory inputs = new OPContractsManager.AddGameInput[](1);
        inputs[0] = input;

        vm.expectRevert(OPContractsManager.OnlyDelegatecall.selector);
        opcm.addGameType(inputs);
    }

    function addGameType(OPContractsManager.AddGameInput memory input)
        internal
        returns (OPContractsManager.AddGameOutput memory)
    {
        OPContractsManager.AddGameInput[] memory inputs = new OPContractsManager.AddGameInput[](1);
        inputs[0] = input;

        (bool success, bytes memory rawGameOut) =
            address(opcm).delegatecall(abi.encodeCall(OPContractsManager.addGameType, (inputs)));
        assertTrue(success, "addGameType failed");

        OPContractsManager.AddGameOutput[] memory addGameOutAll =
            abi.decode(rawGameOut, (OPContractsManager.AddGameOutput[]));
        return addGameOutAll[0];
    }

    function newGameInputFactory(bool permissioned) internal view returns (OPContractsManager.AddGameInput memory) {
        return OPContractsManager.AddGameInput({
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
        OPContractsManager.AddGameInput memory agi,
        OPContractsManager.AddGameOutput memory ago
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
