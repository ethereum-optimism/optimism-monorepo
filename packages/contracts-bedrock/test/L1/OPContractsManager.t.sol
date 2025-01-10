// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { Test, stdStorage, StdStorage } from "forge-std/Test.sol";
import { CommonTest } from "test/setup/CommonTest.sol";
import { DeployOPChain_TestBase } from "test/opcm/DeployOPChain.t.sol";
import { DelegateCaller } from "test/mocks/Callers.sol";

// Scripts
import { DeployOPChainInput } from "scripts/deploy/DeployOPChain.s.sol";

// Libraries
import { EIP1967Helper } from "test/mocks/EIP1967Helper.sol";

// Contracts
import { OPContractsManager } from "src/L1/OPContractsManager.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { IProtocolVersions } from "interfaces/L1/IProtocolVersions.sol";
import { IProxyAdmin } from "interfaces/universal/IProxyAdmin.sol";
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";

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
            startingAnchorRoots: _doi.startingAnchorRoots(),
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

contract OPContractsManager_Upgrade_Harness is CommonTest {
    event Upgraded(ISystemConfig indexed systemConfig, address indexed upgrader);

    IProxyAdmin proxyAdmin;
    address upgrader;

    function setUp() public override {
        super.disableUpgradedFork();
        super.setUp();
        if (!isForkTest()) {
            // This test is only supported in forked tests, as we are testing the upgrade.
            vm.skip(true);
        }

        proxyAdmin = IProxyAdmin(EIP1967Helper.getAdmin(address(systemConfig)));
        upgrader = proxyAdmin.owner();
        vm.label(upgrader, "ProxyAdmin Owner");
    }

    function _getOpChains() internal view returns (OPContractsManager.OpChain[] memory) {
        OPContractsManager.OpChain[] memory opChains = new OPContractsManager.OpChain[](1);
        opChains[0] = OPContractsManager.OpChain({ systemConfig: systemConfig, proxyAdmin: proxyAdmin });
        return opChains;
    }
}

contract OPContractsManager_Upgrade_Test is OPContractsManager_Upgrade_Harness {
    function test_upgrade_succeeds() public {
        OPContractsManager.OpChain[] memory opChains = _getOpChains();
        vm.etch(upgrader, vm.getDeployedCode("test/mocks/Callers.sol:DelegateCaller"));

        vm.expectEmit(true, true, true, false);
        emit Upgraded(opChains[0].systemConfig, address(upgrader));
        DelegateCaller(upgrader).dcForward(address(opcm), abi.encodeCall(OPContractsManager.upgrade, (opChains)));

        OPContractsManager.Implementations memory impls = opcm.implementations();
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
    }
}

contract OPContractsManager_Upgrade_TestFails is OPContractsManager_Upgrade_Harness {
    function test_upgrade_notDelegateCalled_reverts() public {
        vm.prank(upgrader);
        vm.expectRevert(OPContractsManager.OnlyDelegatecall.selector);
        opcm.upgrade(_getOpChains());
    }
}
