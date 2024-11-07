// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { console2 as console } from "forge-std/console2.sol";
import { Vm } from "forge-std/Vm.sol";

// Scripts
import { Deploy } from "scripts/deploy/Deploy.s.sol";
import { Fork, LATEST_FORK } from "scripts/libraries/Config.sol";
import { L2Genesis, L1Dependencies } from "scripts/L2Genesis.s.sol";
import { OutputMode, Fork, ForkUtils } from "scripts/libraries/Config.sol";

// Libraries
import { Predeploys } from "src/libraries/Predeploys.sol";
import { Preinstalls } from "src/libraries/Preinstalls.sol";
import { AddressAliasHelper } from "src/vendor/AddressAliasHelper.sol";

// Interfaces
import { IOptimismPortal } from "src/L1/interfaces/IOptimismPortal.sol";
import { IOptimismPortal2 } from "src/L1/interfaces/IOptimismPortal2.sol";
import { IL1CrossDomainMessenger } from "src/L1/interfaces/IL1CrossDomainMessenger.sol";
import { IL2OutputOracle } from "src/L1/interfaces/IL2OutputOracle.sol";
import { ISystemConfig } from "src/L1/interfaces/ISystemConfig.sol";
import { ISuperchainConfig } from "src/L1/interfaces/ISuperchainConfig.sol";
import { IDataAvailabilityChallenge } from "src/L1/interfaces/IDataAvailabilityChallenge.sol";
import { IL1StandardBridge } from "src/L1/interfaces/IL1StandardBridge.sol";
import { IProtocolVersions } from "src/L1/interfaces/IProtocolVersions.sol";
import { IL1ERC721Bridge } from "src/L1/interfaces/IL1ERC721Bridge.sol";
import { IOptimismMintableERC721Factory } from "src/universal/interfaces/IOptimismMintableERC721Factory.sol";
import { IDisputeGameFactory } from "src/dispute/interfaces/IDisputeGameFactory.sol";
import { IDelayedWETH } from "src/dispute/interfaces/IDelayedWETH.sol";
import { IAnchorStateRegistry } from "src/dispute/interfaces/IAnchorStateRegistry.sol";
import { IL2CrossDomainMessenger } from "src/L2/interfaces/IL2CrossDomainMessenger.sol";
import { IL2StandardBridgeInterop } from "src/L2/interfaces/IL2StandardBridgeInterop.sol";
import { IL2ToL1MessagePasser } from "src/L2/interfaces/IL2ToL1MessagePasser.sol";
import { IL2ERC721Bridge } from "src/L2/interfaces/IL2ERC721Bridge.sol";
import { IOptimismMintableERC20Factory } from "src/universal/interfaces/IOptimismMintableERC20Factory.sol";
import { IAddressManager } from "src/legacy/interfaces/IAddressManager.sol";
import { IOptimismSuperchainERC20Factory } from "src/L2/interfaces/IOptimismSuperchainERC20Factory.sol";
import { IBaseFeeVault } from "src/L2/interfaces/IBaseFeeVault.sol";
import { ISequencerFeeVault } from "src/L2/interfaces/ISequencerFeeVault.sol";
import { IL1FeeVault } from "src/L2/interfaces/IL1FeeVault.sol";
import { IGasPriceOracle } from "src/L2/interfaces/IGasPriceOracle.sol";
import { IL1Block } from "src/L2/interfaces/IL1Block.sol";
import { ISuperchainWETH } from "src/L2/interfaces/ISuperchainWETH.sol";
import { IETHLiquidity } from "src/L2/interfaces/IETHLiquidity.sol";
import { IWETH98 } from "src/universal/interfaces/IWETH98.sol";
import { IGovernanceToken } from "src/governance/interfaces/IGovernanceToken.sol";
import { ILegacyMessagePasser } from "src/legacy/interfaces/ILegacyMessagePasser.sol";
import { ISuperchainTokenBridge } from "src/L2/interfaces/ISuperchainTokenBridge.sol";

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

    L2Genesis internal constant l2Genesis =
        L2Genesis(address(uint160(uint256(keccak256(abi.encode("optimism.l2genesis"))))));

    // @notice Allows users of Setup to override what L2 genesis is being created.
    Fork l2Fork = LATEST_FORK;

    // L1 contracts
    IDisputeGameFactory disputeGameFactory;
    IAnchorStateRegistry anchorStateRegistry;
    IDelayedWETH delayedWeth;
    IOptimismPortal optimismPortal;
    IOptimismPortal2 optimismPortal2;
    IL2OutputOracle l2OutputOracle;
    ISystemConfig systemConfig;
    IL1StandardBridge l1StandardBridge;
    IL1CrossDomainMessenger l1CrossDomainMessenger;
    IAddressManager addressManager;
    IL1ERC721Bridge l1ERC721Bridge;
    IOptimismMintableERC20Factory l1OptimismMintableERC20Factory;
    IProtocolVersions protocolVersions;
    ISuperchainConfig superchainConfig;
    IDataAvailabilityChallenge dataAvailabilityChallenge;

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

    /// @dev Deploys the Deploy contract without including its bytecode in the bytecode
    ///      of this contract by fetching the bytecode dynamically using `vm.getCode()`.
    ///      If the Deploy bytecode is included in this contract, then it will double
    ///      the compile time and bloat all of the test contract artifacts since they
    ///      will also need to include the bytecode for the Deploy contract.
    ///      This is a hack as we are pushing solidity to the edge.
    function setUp() public virtual {
        console.log("L1 setup start!");
        vm.etch(address(deploy), vm.getDeployedCode("Deploy.s.sol:Deploy"));
        vm.allowCheatcodes(address(deploy));
        deploy.setUp();
        console.log("L1 setup done!");

        console.log("L2 setup start!");
        vm.etch(address(l2Genesis), vm.getDeployedCode("L2Genesis.s.sol:L2Genesis"));
        vm.allowCheatcodes(address(l2Genesis));
        l2Genesis.setUp();
        console.log("L2 setup done!");
    }

    /// @dev Sets up the L1 contracts.
    function L1() public {
        console.log("Setup: creating L1 deployments");
        // Set the deterministic deployer in state to ensure that it is there
        vm.etch(
            0x4e59b44847b379578588920cA78FbF26c0B4956C,
            hex"7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe03601600081602082378035828234f58015156039578182fd5b8082525050506014600cf3"
        );

        deploy.run();
        console.log("Setup: completed L1 deployment, registering addresses now");

        optimismPortal = IOptimismPortal(deploy.mustGetAddress("OptimismPortalProxy"));
        optimismPortal2 = IOptimismPortal2(deploy.mustGetAddress("OptimismPortalProxy"));
        disputeGameFactory = IDisputeGameFactory(deploy.mustGetAddress("DisputeGameFactoryProxy"));
        delayedWeth = IDelayedWETH(deploy.mustGetAddress("DelayedWETHProxy"));
        systemConfig = ISystemConfig(deploy.mustGetAddress("SystemConfigProxy"));
        l1StandardBridge = IL1StandardBridge(deploy.mustGetAddress("L1StandardBridgeProxy"));
        l1CrossDomainMessenger = IL1CrossDomainMessenger(deploy.mustGetAddress("L1CrossDomainMessengerProxy"));
        addressManager = IAddressManager(deploy.mustGetAddress("AddressManager"));
        l1ERC721Bridge = IL1ERC721Bridge(deploy.mustGetAddress("L1ERC721BridgeProxy"));
        l1OptimismMintableERC20Factory =
            IOptimismMintableERC20Factory(deploy.mustGetAddress("OptimismMintableERC20FactoryProxy"));
        protocolVersions = IProtocolVersions(deploy.mustGetAddress("ProtocolVersionsProxy"));
        superchainConfig = ISuperchainConfig(deploy.mustGetAddress("SuperchainConfigProxy"));
        anchorStateRegistry = IAnchorStateRegistry(deploy.mustGetAddress("AnchorStateRegistryProxy"));

        vm.label(address(optimismPortal), "OptimismPortal");
        vm.label(deploy.mustGetAddress("OptimismPortalProxy"), "OptimismPortalProxy");
        vm.label(address(disputeGameFactory), "DisputeGameFactory");
        vm.label(deploy.mustGetAddress("DisputeGameFactoryProxy"), "DisputeGameFactoryProxy");
        vm.label(address(delayedWeth), "DelayedWETH");
        vm.label(deploy.mustGetAddress("DelayedWETHProxy"), "DelayedWETHProxy");
        vm.label(address(systemConfig), "SystemConfig");
        vm.label(deploy.mustGetAddress("SystemConfigProxy"), "SystemConfigProxy");
        vm.label(address(l1StandardBridge), "L1StandardBridge");
        vm.label(deploy.mustGetAddress("L1StandardBridgeProxy"), "L1StandardBridgeProxy");
        vm.label(address(l1CrossDomainMessenger), "L1CrossDomainMessenger");
        vm.label(deploy.mustGetAddress("L1CrossDomainMessengerProxy"), "L1CrossDomainMessengerProxy");
        vm.label(address(addressManager), "AddressManager");
        vm.label(address(l1ERC721Bridge), "L1ERC721Bridge");
        vm.label(deploy.mustGetAddress("L1ERC721BridgeProxy"), "L1ERC721BridgeProxy");
        vm.label(address(l1OptimismMintableERC20Factory), "OptimismMintableERC20Factory");
        vm.label(deploy.mustGetAddress("OptimismMintableERC20FactoryProxy"), "OptimismMintableERC20FactoryProxy");
        vm.label(address(protocolVersions), "ProtocolVersions");
        vm.label(deploy.mustGetAddress("ProtocolVersionsProxy"), "ProtocolVersionsProxy");
        vm.label(address(superchainConfig), "SuperchainConfig");
        vm.label(deploy.mustGetAddress("SuperchainConfigProxy"), "SuperchainConfigProxy");
        vm.label(AddressAliasHelper.applyL1ToL2Alias(address(l1CrossDomainMessenger)), "L1CrossDomainMessenger_aliased");

        if (!deploy.cfg().useFaultProofs()) {
            l2OutputOracle = IL2OutputOracle(deploy.mustGetAddress("L2OutputOracleProxy"));
            vm.label(address(l2OutputOracle), "L2OutputOracle");
            vm.label(deploy.mustGetAddress("L2OutputOracleProxy"), "L2OutputOracleProxy");
        }

        if (deploy.cfg().useAltDA()) {
            dataAvailabilityChallenge =
                IDataAvailabilityChallenge(deploy.mustGetAddress("DataAvailabilityChallengeProxy"));
            vm.label(address(dataAvailabilityChallenge), "DataAvailabilityChallengeProxy");
            vm.label(deploy.mustGetAddress("DataAvailabilityChallenge"), "DataAvailabilityChallenge");
        }
        console.log("Setup: registered L1 deployments");
    }

    /// @dev Sets up the L2 contracts. Depends on `L1()` being called first.
    function L2() public {
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
