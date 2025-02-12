// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { console2 as console } from "forge-std/console2.sol";
import { Vm, VmSafe } from "forge-std/Vm.sol";

// Scripts
import { Deploy } from "scripts/deploy/Deploy.s.sol";
import { ForkLive } from "test/setup/ForkLive.s.sol";
import { Fork, LATEST_FORK } from "scripts/libraries/Config.sol";
import { L2Genesis, L1Dependencies } from "scripts/L2Genesis.s.sol";
import { OutputMode, Fork, ForkUtils } from "scripts/libraries/Config.sol";
import { Artifacts } from "scripts/Artifacts.s.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

// Libraries
import { Predeploys } from "src/libraries/Predeploys.sol";
import { Preinstalls } from "src/libraries/Preinstalls.sol";
import { AddressAliasHelper } from "src/vendor/AddressAliasHelper.sol";
import { Chains } from "scripts/libraries/Chains.sol";

// Interfaces
import { IOPContractsManager } from "interfaces/L1/IOPContractsManager.sol";
import { IOptimismPortal2 } from "interfaces/L1/IOptimismPortal2.sol";
import { IL1CrossDomainMessenger } from "interfaces/L1/IL1CrossDomainMessenger.sol";
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { IDataAvailabilityChallenge } from "interfaces/L1/IDataAvailabilityChallenge.sol";
import { IL1StandardBridge } from "interfaces/L1/IL1StandardBridge.sol";
import { IProtocolVersions } from "interfaces/L1/IProtocolVersions.sol";
import { IL1ERC721Bridge } from "interfaces/L1/IL1ERC721Bridge.sol";
import { IOptimismMintableERC721Factory } from "interfaces/L2/IOptimismMintableERC721Factory.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { IDelayedWETH } from "interfaces/dispute/IDelayedWETH.sol";
import { IAnchorStateRegistry } from "interfaces/dispute/IAnchorStateRegistry.sol";
import { IL2CrossDomainMessenger } from "interfaces/L2/IL2CrossDomainMessenger.sol";
import { IL2StandardBridgeInterop } from "interfaces/L2/IL2StandardBridgeInterop.sol";
import { IL2ToL1MessagePasser } from "interfaces/L2/IL2ToL1MessagePasser.sol";
import { IL2ERC721Bridge } from "interfaces/L2/IL2ERC721Bridge.sol";
import { IOptimismMintableERC20Factory } from "interfaces/universal/IOptimismMintableERC20Factory.sol";
import { IAddressManager } from "interfaces/legacy/IAddressManager.sol";
import { IOptimismSuperchainERC20Factory } from "interfaces/L2/IOptimismSuperchainERC20Factory.sol";
import { IBaseFeeVault } from "interfaces/L2/IBaseFeeVault.sol";
import { ISequencerFeeVault } from "interfaces/L2/ISequencerFeeVault.sol";
import { IL1FeeVault } from "interfaces/L2/IL1FeeVault.sol";
import { IGasPriceOracle } from "interfaces/L2/IGasPriceOracle.sol";
import { IL1Block } from "interfaces/L2/IL1Block.sol";
import { ISuperchainWETH } from "interfaces/L2/ISuperchainWETH.sol";
import { IETHLiquidity } from "interfaces/L2/IETHLiquidity.sol";
import { IWETH98 } from "interfaces/universal/IWETH98.sol";
import { IGovernanceToken } from "interfaces/governance/IGovernanceToken.sol";
import { ILegacyMessagePasser } from "interfaces/legacy/ILegacyMessagePasser.sol";
import { ISuperchainTokenBridge } from "interfaces/L2/ISuperchainTokenBridge.sol";
import { IPermissionedDisputeGame } from "interfaces/dispute/IPermissionedDisputeGame.sol";
import { IFaultDisputeGame } from "interfaces/dispute/IFaultDisputeGame.sol";

/// @title Setup
/// @dev This contact is responsible for setting up the contracts in state. It currently
///      sets the L2 contracts directly at the predeploy addresses instead of setting them
///      up behind proxies. In the future we will migrate to importing the genesis JSON
///      file that is created to set up the L2 contracts instead of setting them up manually.
contract Setup {
    using ForkUtils for Fork;

    /// @notice The address of the foundry Vm contract.
    Vm private constant vm = Vm(0x7109709ECfa91a80626fF3989D68f67F5b1DD12D);

    /// @notice The address of the Deploy contract. Set into state with `etch` to avoid
    ///         mutating any nonces. MUST not have constructor logic.
    Deploy internal constant deploy = Deploy(address(uint160(uint256(keccak256(abi.encode("optimism.deploy"))))));

    /// @notice The address of the ForkLive contract. Set into state with `etch` to avoid
    ///         mutating any nonces. MUST not have constructor logic.
    ForkLive internal constant forkLive =
        ForkLive(address(uint160(uint256(keccak256(abi.encode("optimism.forklive"))))));

    /// @notice The address of the Artifacts contract. Set into state by Deployer.setUp() with `etch` to avoid
    ///         mutating any nonces. MUST not have constructor logic.
    Artifacts public constant artifacts =
        Artifacts(address(uint160(uint256(keccak256(abi.encode("optimism.artifacts"))))));

    L2Genesis internal constant l2Genesis =
        L2Genesis(address(uint160(uint256(keccak256(abi.encode("optimism.l2genesis"))))));

    /// @notice Allows users of Setup to override what L2 genesis is being created.
    Fork l2Fork = LATEST_FORK;

    // L1 contracts - dispute
    IDisputeGameFactory disputeGameFactory;
    IAnchorStateRegistry anchorStateRegistry;
    IFaultDisputeGame faultDisputeGame;
    IDelayedWETH delayedWeth;
    IPermissionedDisputeGame permissionedDisputeGame;
    IDelayedWETH delayedWETHPermissionedGameProxy;

    // L1 contracts - core
    IOptimismPortal2 optimismPortal2;
    ISystemConfig systemConfig;
    IL1StandardBridge l1StandardBridge;
    IL1CrossDomainMessenger l1CrossDomainMessenger;
    IAddressManager addressManager;
    IL1ERC721Bridge l1ERC721Bridge;
    IOptimismMintableERC20Factory l1OptimismMintableERC20Factory;
    IProtocolVersions protocolVersions;
    ISuperchainConfig superchainConfig;
    IDataAvailabilityChallenge dataAvailabilityChallenge;
    IOPContractsManager opcm;

    // L2 contracts
    IL2CrossDomainMessenger l2CrossDomainMessenger =
        IL2CrossDomainMessenger(payable(Predeploys.L2_CROSS_DOMAIN_MESSENGER));
    IL2StandardBridgeInterop l2StandardBridge = IL2StandardBridgeInterop(payable(Predeploys.L2_STANDARD_BRIDGE));
    IL2ToL1MessagePasser l2ToL1MessagePasser = IL2ToL1MessagePasser(payable(Predeploys.L2_TO_L1_MESSAGE_PASSER));
    IOptimismMintableERC20Factory l2OptimismMintableERC20Factory =
        IOptimismMintableERC20Factory(Predeploys.OPTIMISM_MINTABLE_ERC20_FACTORY);
    IL2ERC721Bridge l2ERC721Bridge = IL2ERC721Bridge(Predeploys.L2_ERC721_BRIDGE);
    IOptimismMintableERC721Factory l2OptimismMintableERC721Factory =
        IOptimismMintableERC721Factory(Predeploys.OPTIMISM_MINTABLE_ERC721_FACTORY);
    IBaseFeeVault baseFeeVault = IBaseFeeVault(payable(Predeploys.BASE_FEE_VAULT));
    ISequencerFeeVault sequencerFeeVault = ISequencerFeeVault(payable(Predeploys.SEQUENCER_FEE_WALLET));
    IL1FeeVault l1FeeVault = IL1FeeVault(payable(Predeploys.L1_FEE_VAULT));
    IGasPriceOracle gasPriceOracle = IGasPriceOracle(Predeploys.GAS_PRICE_ORACLE);
    IL1Block l1Block = IL1Block(Predeploys.L1_BLOCK_ATTRIBUTES);
    IGovernanceToken governanceToken = IGovernanceToken(Predeploys.GOVERNANCE_TOKEN);
    ILegacyMessagePasser legacyMessagePasser = ILegacyMessagePasser(Predeploys.LEGACY_MESSAGE_PASSER);
    IWETH98 weth = IWETH98(payable(Predeploys.WETH));
    ISuperchainWETH superchainWeth = ISuperchainWETH(payable(Predeploys.SUPERCHAIN_WETH));
    IETHLiquidity ethLiquidity = IETHLiquidity(Predeploys.ETH_LIQUIDITY);
    ISuperchainTokenBridge superchainTokenBridge = ISuperchainTokenBridge(Predeploys.SUPERCHAIN_TOKEN_BRIDGE);
    IOptimismSuperchainERC20Factory l2OptimismSuperchainERC20Factory =
        IOptimismSuperchainERC20Factory(Predeploys.OPTIMISM_SUPERCHAIN_ERC20_FACTORY);

    /// @notice Indicates whether a test is running against a forked production network.
    function isForkTest() public view returns (bool) {
        return vm.envOr("FORK_TEST", false);
    }

    /// @dev Deploys either the Deploy.s.sol or Fork.s.sol contract, by fetching the bytecode dynamically using
    ///      `vm.getDeployedCode()` and etching it into the state.
    ///      This enables us to avoid including the bytecode of those contracts in the bytecode of this contract.
    ///      If the bytecode of those contracts was included in this contract, then it will double
    ///      the compile time and bloat all of the test contract artifacts since they
    ///      will also need to include the bytecode for the Deploy contract.
    ///      This is a hack as we are pushing solidity to the edge.
    function setUp() public virtual {
        console.log("Setup: L1 setup start!");

        if (isForkTest()) {
            vm.createSelectFork(vm.envString("FORK_RPC_URL"), vm.envUint("FORK_BLOCK_NUMBER"));
            require(
                block.chainid == Chains.Sepolia || block.chainid == Chains.Mainnet,
                "Setup: ETH_RPC_URL must be set to a production (Sepolia or Mainnet) RPC URL"
            );
        }

        // Etch the contracts used to setup the test environment
        DeployUtils.etchLabelAndAllowCheatcodes({ _etchTo: address(deploy), _cname: "Deploy" });
        DeployUtils.etchLabelAndAllowCheatcodes({ _etchTo: address(forkLive), _cname: "ForkLive" });

        deploy.setUp();
        forkLive.setUp();
        console.log("Setup: L1 setup done!");

        if (isForkTest()) {
            // Return early if this is a fork test as we don't need to setup L2
            console.log("Setup: fork test detected, skipping L2 genesis generation");
            return;
        }

        console.log("Setup: L2 setup start!");
        vm.etch(address(l2Genesis), vm.getDeployedCode("L2Genesis.s.sol:L2Genesis"));
        vm.allowCheatcodes(address(l2Genesis));
        l2Genesis.setUp();
        console.log("Setup: L2 setup done!");
    }

    /// @dev Skips tests when running in coverage mode.
    function skipIfCoverage() public {
        if (vm.isContext(VmSafe.ForgeContext.Coverage)) {
            vm.skip(true);
        }
    }

    /// @dev Skips tests when running against a forked production network.
    function skipIfForkTest(string memory message) public {
        if (isForkTest()) {
            vm.skip(true);
            console.log(string.concat("Skipping fork test: ", message));
        }
    }

    /// @dev Skips tests when running against a forked production network using the superchain ops repo.
    function skipIfOpsRepoTest(string memory message) public {
        if (forkLive.useOpsRepo()) {
            vm.skip(true);
            console.log(string.concat("Skipping ops repo test: ", message));
        }
    }

    /// @dev Returns early when running against a forked production network. Useful for allowing a portion of a test
    ///      to run.
    function returnIfForkTest(string memory message) public view {
        if (isForkTest()) {
            console.log(string.concat("Returning early from fork test: ", message));
            assembly {
                return(0, 0)
            }
        }
    }

    /// @dev Sets up the L1 contracts.
    function L1() public {
        console.log("Setup: creating L1 deployments");
        // Set the deterministic deployer in state to ensure that it is there
        vm.etch(
            0x4e59b44847b379578588920cA78FbF26c0B4956C,
            hex"7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe03601600081602082378035828234f58015156039578182fd5b8082525050506014600cf3"
        );

        if (isForkTest()) {
            forkLive.run();
        } else {
            deploy.run();
        }

        console.log("Setup: completed L1 deployment, registering addresses now");

        optimismPortal2 = IOptimismPortal2(artifacts.mustGetAddress("OptimismPortalProxy"));
        systemConfig = ISystemConfig(artifacts.mustGetAddress("SystemConfigProxy"));
        l1StandardBridge = IL1StandardBridge(artifacts.mustGetAddress("L1StandardBridgeProxy"));
        l1CrossDomainMessenger = IL1CrossDomainMessenger(artifacts.mustGetAddress("L1CrossDomainMessengerProxy"));
        vm.label(
            AddressAliasHelper.applyL1ToL2Alias(address(l1CrossDomainMessenger)), "L1CrossDomainMessengerProxy_aliased"
        );
        addressManager = IAddressManager(artifacts.mustGetAddress("AddressManager"));
        l1ERC721Bridge = IL1ERC721Bridge(artifacts.mustGetAddress("L1ERC721BridgeProxy"));
        l1OptimismMintableERC20Factory =
            IOptimismMintableERC20Factory(artifacts.mustGetAddress("OptimismMintableERC20FactoryProxy"));
        protocolVersions = IProtocolVersions(artifacts.mustGetAddress("ProtocolVersionsProxy"));
        superchainConfig = ISuperchainConfig(artifacts.mustGetAddress("SuperchainConfigProxy"));
        anchorStateRegistry = IAnchorStateRegistry(artifacts.mustGetAddress("AnchorStateRegistryProxy"));
        disputeGameFactory = IDisputeGameFactory(artifacts.mustGetAddress("DisputeGameFactoryProxy"));
        delayedWeth = IDelayedWETH(artifacts.mustGetAddress("DelayedWETHProxy"));
        opcm = IOPContractsManager(artifacts.mustGetAddress("OPContractsManager"));

        if (deploy.cfg().useAltDA()) {
            dataAvailabilityChallenge =
                IDataAvailabilityChallenge(artifacts.mustGetAddress("DataAvailabilityChallengeProxy"));
        }
        console.log("Setup: registered L1 deployments");
    }

    /// @dev Sets up the L2 contracts. Depends on `L1()` being called first.
    function L2() public {
        // Fork tests focus on L1 contracts so there is no need to do all the work of setting up L2.
        if (isForkTest()) {
            console.log("Setup: fork test detected, skipping L2 setup");
            return;
        }

        console.log("Setup: creating L2 genesis with fork %s", l2Fork.toString());
        l2Genesis.runWithOptions(
            OutputMode.NONE,
            l2Fork,
            L1Dependencies({
                l1CrossDomainMessengerProxy: payable(address(l1CrossDomainMessenger)),
                l1StandardBridgeProxy: payable(address(l1StandardBridge)),
                l1ERC721BridgeProxy: payable(address(l1ERC721Bridge))
            })
        );

        // Set the governance token's owner to be the final system owner
        address finalSystemOwner = deploy.cfg().finalSystemOwner();
        vm.startPrank(governanceToken.owner());
        governanceToken.transferOwnership(finalSystemOwner);
        vm.stopPrank();

        // L2 predeploys
        labelPredeploy(Predeploys.L2_STANDARD_BRIDGE);
        labelPredeploy(Predeploys.L2_CROSS_DOMAIN_MESSENGER);
        labelPredeploy(Predeploys.L2_TO_L1_MESSAGE_PASSER);
        labelPredeploy(Predeploys.SEQUENCER_FEE_WALLET);
        labelPredeploy(Predeploys.L2_ERC721_BRIDGE);
        labelPredeploy(Predeploys.OPTIMISM_MINTABLE_ERC721_FACTORY);
        labelPredeploy(Predeploys.BASE_FEE_VAULT);
        labelPredeploy(Predeploys.L1_FEE_VAULT);
        labelPredeploy(Predeploys.L1_BLOCK_ATTRIBUTES);
        labelPredeploy(Predeploys.GAS_PRICE_ORACLE);
        labelPredeploy(Predeploys.LEGACY_MESSAGE_PASSER);
        labelPredeploy(Predeploys.GOVERNANCE_TOKEN);
        labelPredeploy(Predeploys.EAS);
        labelPredeploy(Predeploys.SCHEMA_REGISTRY);
        labelPredeploy(Predeploys.WETH);
        labelPredeploy(Predeploys.SUPERCHAIN_WETH);
        labelPredeploy(Predeploys.ETH_LIQUIDITY);
        labelPredeploy(Predeploys.OPTIMISM_SUPERCHAIN_ERC20_FACTORY);
        labelPredeploy(Predeploys.OPTIMISM_SUPERCHAIN_ERC20_BEACON);
        labelPredeploy(Predeploys.SUPERCHAIN_TOKEN_BRIDGE);

        // L2 Preinstalls
        labelPreinstall(Preinstalls.MultiCall3);
        labelPreinstall(Preinstalls.Create2Deployer);
        labelPreinstall(Preinstalls.Safe_v130);
        labelPreinstall(Preinstalls.SafeL2_v130);
        labelPreinstall(Preinstalls.MultiSendCallOnly_v130);
        labelPreinstall(Preinstalls.SafeSingletonFactory);
        labelPreinstall(Preinstalls.DeterministicDeploymentProxy);
        labelPreinstall(Preinstalls.MultiSend_v130);
        labelPreinstall(Preinstalls.Permit2);
        labelPreinstall(Preinstalls.SenderCreator_v060);
        labelPreinstall(Preinstalls.EntryPoint_v060);
        labelPreinstall(Preinstalls.SenderCreator_v070);
        labelPreinstall(Preinstalls.EntryPoint_v070);
        labelPreinstall(Preinstalls.BeaconBlockRoots);
        labelPreinstall(Preinstalls.HistoryStorage);
        labelPreinstall(Preinstalls.CreateX);

        console.log("Setup: completed L2 genesis");
    }

    function labelPredeploy(address _addr) internal {
        vm.label(_addr, Predeploys.getName(_addr));
    }

    function labelPreinstall(address _addr) internal {
        vm.label(_addr, Preinstalls.getName(_addr));
    }
}
