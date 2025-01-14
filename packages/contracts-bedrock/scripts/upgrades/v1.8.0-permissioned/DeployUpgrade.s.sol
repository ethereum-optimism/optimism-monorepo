// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Forge
import { StdAssertions } from "forge-std/StdAssertions.sol";
import { console2 as console } from "forge-std/console2.sol";

// Scripts
import { Deploy } from "scripts/deploy/Deploy.s.sol";
import { ChainAssertions } from "scripts/deploy/ChainAssertions.sol";
import { Types } from "scripts/libraries/Types.sol";

// Contracts
import { Proxy } from "src/universal/Proxy.sol";
import { IAnchorStateRegistry } from "src/dispute/interfaces/IAnchorStateRegistry.sol";
import { IDisputeGameFactory } from "src/dispute/interfaces/IDisputeGameFactory.sol";
import { PermissionedDisputeGame } from "src/dispute/PermissionedDisputeGame.sol";
import { IDelayedWETH } from "src/dispute/interfaces/IDelayedWETH.sol";
import { IPreimageOracle } from "src/cannon/interfaces/IPreimageOracle.sol";
import { MIPS } from "src/cannon/MIPS.sol";
import { IBigStepper } from "src/dispute/interfaces/IBigStepper.sol";
import { IDisputeGame } from "src/dispute/interfaces/IDisputeGame.sol";
import { ISuperchainConfig } from "src/L1/interfaces/ISuperchainConfig.sol";
import { Blueprint } from "src/libraries/Blueprint.sol";
import { GameTypes } from "src/dispute/lib/Types.sol";
import { Duration } from "src/dispute/lib/LibUDT.sol";

// Libraries
import { GameTypes, OutputRoot, Hash } from "src/dispute/lib/Types.sol";
import { Constants } from "src/libraries/Constants.sol";

/// @title DeployUpgrade
/// @notice Script for deploying contracts required to upgrade from v1.8.0 L2OO to v1.8.0 in a
///         PERMISSIONED configuration.
contract DeployUpgrade is Deploy, StdAssertions {
    /// @notice Address of the ProxyAdmin contract.
    address public proxyAdmin;

    /// @notice Address of the SystemOwnerSafe contract.
    address public systemOwnerSafe;

    /// @notice Address of the SuperchainConfigProxy contract.
    address public superchainConfigProxy;

    /// @notice Deploys the contracts required to upgrade from v1.8.0 L2OO to v1.8.0 in a
    ///         PERMISSIONED configuration.
    /// @param _proxyAdmin Address of the ProxyAdmin contract.
    /// @param _systemOwnerSafe Address of the SystemOwnerSafe contract.
    /// @param _superchainConfigProxy Address of the SuperchainConfigProxy contract.
    /// @param _disputeGameFactoryImpl Address of the DisputeGameFactory implementation contract.
    /// @param _delayedWethImpl Address of the DelayedWETH implementation contract.
    /// @param _preimageOracleImpl Address of the PreimageOracle implementation contract.
    /// @param _mipsImpl Address of the MIPS implementation contract.
    /// @param _optimismPortal2Impl Address of the OptimismPortal2 implementation contract.
    /// @param _anchorStateRegistryBlueprint Address of the AnchorStateRegistry implementation blueprint.
    function deploy(
        address _proxyAdmin,
        address _systemOwnerSafe,
        address _superchainConfigProxy,
        address _disputeGameFactoryImpl,
        address _delayedWethImpl,
        address _preimageOracleImpl,
        address _mipsImpl,
        address _optimismPortal2Impl,
        address _anchorStateRegistryBlueprint
    )
        public
    {
        console.log("Deploying contracts required to upgrade from v1.8.0 L2OO to v1.8.0 permissioned");
        console.log("Using PERMISSIONED proof system");

        // Set address variables.
        proxyAdmin = _proxyAdmin;
        systemOwnerSafe = _systemOwnerSafe;
        superchainConfigProxy = _superchainConfigProxy;

        // Prank admin contracts.
        prankDeployment("ProxyAdmin", msg.sender);
        prankDeployment("SystemOwnerSafe", msg.sender);

        // Prank shared contracts.
        prankDeployment("SuperchainConfigProxy", superchainConfigProxy);
        prankDeployment("DisputeGameFactory", _disputeGameFactoryImpl);
        prankDeployment("DelayedWETH", _delayedWethImpl);
        prankDeployment("PreimageOracle", _preimageOracleImpl);
        prankDeployment("OptimismPortal2", _optimismPortal2Impl);

        // Deploy proxy contracts.
        deployERC1967Proxy("DisputeGameFactoryProxy");
        deployERC1967Proxy("AnchorStateRegistryProxy");
        deployERC1967Proxy("PermissionedDelayedWETHProxy");

        // Deploy AnchorStateRegistry implementation contract.
        // We can't use a pre-created implementation because the ASR implementation holds an
        // immutable variable that points at the DisputeGameFactoryProxy.
        deployAnchorStateRegistry(_anchorStateRegistryBlueprint);

        // Re-use the existing MIPS implementation, but ensure its address is available
        save("Mips", _mipsImpl);

        // Initialize proxy contracts.
        initializeDisputeGameFactoryProxy();
        initializeAnchorStateRegistryProxy();
        initializePermissionedDelayedWETHProxy();

        // ONLY deploy and set up the PermissionedDisputeGame.
        // We can't use a pre-created implementation because the PermissionedDisputeGame holds an
        // immutable variable that refers to the L2 chain ID.
        setPermissionedCannonFaultGameImplementation();

        // Transfer contract ownership to ProxyAdmin.
        transferPermissionedWETHOwnershipFinal();
        transferDGFOwnershipFinal();
        transferAnchorStateOwnershipFinal();

        // Run post-deployment assertions.
        postDeployAssertions();

        // Print config summary.
        printConfigSummary();

        // Print deployment summary.
        printDeploymentSummary();
    }

    /// @notice Deploys teh AnchorStateRegistry implementation
    function deployAnchorStateRegistry(address _anchorStateRegistryBlueprint) internal broadcast {
        address impl = Blueprint.deployFrom(
            _anchorStateRegistryBlueprint, _implSalt(), abi.encode(mustGetAddress("DisputeGameFactoryProxy"))
        );
        save("AnchorStateRegistry", impl);
    }

    /// @notice Sets the implementation for the `PERMISSIONED_CANNON` game type in the `DisputeGameFactory`
    function setPermissionedCannonFaultGameImplementation() public broadcast {
        console.log("Setting Cannon PermissionedDisputeGame implementation");
        IDisputeGameFactory factory = IDisputeGameFactory(mustGetAddress("DisputeGameFactoryProxy"));
        IDelayedWETH weth = IDelayedWETH(mustGetAddress("PermissionedDelayedWETHProxy"));

        PermissionedDisputeGame impl = new PermissionedDisputeGame(
            GameTypes.PERMISSIONED_CANNON,
            loadMipsAbsolutePrestate(),
            cfg.faultGameMaxDepth(),
            cfg.faultGameSplitDepth(),
            Duration.wrap(uint64(cfg.faultGameClockExtension())),
            Duration.wrap(uint64(cfg.faultGameMaxClockDuration())),
            IBigStepper(mustGetAddress("Mips")),
            weth,
            IAnchorStateRegistry(mustGetAddress("AnchorStateRegistryProxy")),
            cfg.l2ChainID(),
            cfg.l2OutputOracleProposer(),
            cfg.l2OutputOracleChallenger()
        );
        factory.setImplementation(GameTypes.PERMISSIONED_CANNON, IDisputeGame(address(impl)));
        save("PermissionedDisputeGame", address(impl));
        console.log(
            "DisputeGameFactoryProxy: set `FaultDisputeGame` implementation (Backend: Cannon | GameType: PERMISSIONED_CANNON)"
        );
    }

    /// @notice Initializes the DisputeGameFactory proxy.
    function initializeDisputeGameFactoryProxy() internal broadcast {
        console.log("Initializing DisputeGameFactory proxy");
        Proxy(payable(mustGetAddress("DisputeGameFactoryProxy"))).upgradeToAndCall(
            mustGetAddress("DisputeGameFactory"), abi.encodeCall(IDisputeGameFactory.initialize, msg.sender)
        );

        // We don't need to set the initialization bond for PermissionedDisputeGame because the
        // initialization bond is meant to be zero anyway. We assert that this bond is zero in the
        // post-checks that we perform either way, so no need for an explicit transaction.
    }

    /// @notice Initializes the AnchorStateRegistry proxy.
    function initializeAnchorStateRegistryProxy() internal broadcast {
        // Set up the anchor state root array.
        IAnchorStateRegistry.StartingAnchorRoot[] memory roots = new IAnchorStateRegistry.StartingAnchorRoot[](2);
        roots[0] = IAnchorStateRegistry.StartingAnchorRoot({
            gameType: GameTypes.PERMISSIONED_CANNON,
            outputRoot: OutputRoot({
                root: Hash.wrap(cfg.faultGameGenesisOutputRoot()),
                l2BlockNumber: cfg.faultGameGenesisBlock()
            })
        });

        // Initialize AnchorStateRegistry proxy.
        console.log("Initializing AnchorStateRegistry proxy");
        Proxy(payable(mustGetAddress("AnchorStateRegistryProxy"))).upgradeToAndCall(
            mustGetAddress("AnchorStateRegistry"),
            abi.encodeCall(
                IAnchorStateRegistry.initialize, (roots, ISuperchainConfig(mustGetAddress("SuperchainConfigProxy")))
            )
        );
    }

    /// @notice Initializes the permissioned DelayedWETH proxy.
    function initializePermissionedDelayedWETHProxy() internal broadcast {
        // Initialize permissioned DelayedWETH proxy.
        console.log("Initializing permissioned DelayedWETH proxy");
        Proxy(payable(mustGetAddress("PermissionedDelayedWETHProxy"))).upgradeToAndCall(
            mustGetAddress("DelayedWETH"),
            abi.encodeCall(
                IDelayedWETH.initialize, (msg.sender, ISuperchainConfig(mustGetAddress("SuperchainConfigProxy")))
            )
        );
    }

    /// @notice Transfers ownership of the permissioned DelayedWETH proxy to the ProxyAdmin and
    ///         transfers ownership of the underlying DelayedWETH contract to the SystemOwnerSafe.
    function transferPermissionedWETHOwnershipFinal() internal broadcast {
        // Transfer ownership of permissioned DelayedWETH to SystemOwnerSafe.
        console.log("Transferring ownership of underlying permissioned DelayedWETH");
        IDelayedWETH weth = IDelayedWETH(mustGetAddress("PermissionedDelayedWETHProxy"));
        weth.transferOwnership(systemOwnerSafe);

        // Transfer ownership of permissioned DelayedWETH proxy to ProxyAdmin.
        console.log("Transferring ownership of permissioned DelayedWETH proxy");
        Proxy prox = Proxy(payable(address(weth)));
        prox.changeAdmin(proxyAdmin);
    }

    /// @notice Transfers ownership of the DisputeGameFactory proxy to the ProxyAdmin and transfers
    ///         ownership of the underlying DisputeGameFactory contract to the SystemOwnerSafe.
    function transferDGFOwnershipFinal() internal broadcast {
        // Transfer ownership of DisputeGameFactory to SystemOwnerSafe.
        console.log("Transferring ownership of underlying DisputeGameFactory");
        IDisputeGameFactory dgf = IDisputeGameFactory(mustGetAddress("DisputeGameFactoryProxy"));
        dgf.transferOwnership(systemOwnerSafe);

        // Transfer ownership of DisputeGameFactory proxy to ProxyAdmin.
        console.log("Transferring ownership of DisputeGameFactory proxy");
        Proxy prox = Proxy(payable(address(dgf)));
        prox.changeAdmin(proxyAdmin);
    }

    /// @notice Transfers ownership of the AnchorStateRegistry proxy to the ProxyAdmin.
    function transferAnchorStateOwnershipFinal() internal broadcast {
        // Transfer ownership of AnchorStateRegistry proxy to ProxyAdmin.
        console.log("Transferring ownership of AnchorStateRegistry proxy");
        IAnchorStateRegistry asr = IAnchorStateRegistry(mustGetAddress("AnchorStateRegistryProxy"));
        Proxy prox = Proxy(payable(address(asr)));
        prox.changeAdmin(proxyAdmin);
    }

    /// @notice Checks that the deployed system is configured correctly.
    function postDeployAssertions() internal view {
        Types.ContractSet memory contracts = _proxies();
        contracts.OptimismPortal2 = mustGetAddress("OptimismPortal2");

        // Ensure that `useFaultProofs` is set to `true`.
        assertTrue(cfg.useFaultProofs(), "DeployUpgrade: useFaultProofs is not set to true");

        // Verify that the DGF is owned by the ProxyAdmin.
        address dgfProxyAddr = mustGetAddress("DisputeGameFactoryProxy");
        assertEq(
            address(uint160(uint256(vm.load(dgfProxyAddr, Constants.PROXY_OWNER_ADDRESS)))),
            proxyAdmin,
            "DeployUpgrade: DGF is not owned by ProxyAdmin"
        );

        // Verify that permissioned DelayedWETH is owned by the ProxyAdmin.
        address soyWethProxyAddr = mustGetAddress("PermissionedDelayedWETHProxy");
        assertEq(
            address(uint160(uint256(vm.load(soyWethProxyAddr, Constants.PROXY_OWNER_ADDRESS)))),
            proxyAdmin,
            "DeployUpgrade: Permissioned DelayedWETH is not owned by ProxyAdmin"
        );

        // Run standard assertions.
        ChainAssertions.checkDisputeGameFactory(contracts, systemOwnerSafe, true);
        ChainAssertions.checkPermissionedDelayedWETH(contracts, cfg, true, systemOwnerSafe);
        ChainAssertions.checkOptimismPortal2(contracts, cfg, false);

        // Verify PreimageOracle configuration.
        IPreimageOracle oracle = IPreimageOracle(mustGetAddress("PreimageOracle"));
        assertEq(
            oracle.minProposalSize(),
            cfg.preimageOracleMinProposalSize(),
            "DeployUpgrade: PreimageOracle minProposalSize is not set correctly"
        );
        assertEq(
            oracle.challengePeriod(),
            cfg.preimageOracleChallengePeriod(),
            "DeployUpgrade: PreimageOracle challengePeriod is not set correctly"
        );

        // Verify MIPS configuration.
        MIPS mips = MIPS(mustGetAddress("Mips"));
        assertEq(address(mips.oracle()), address(oracle), "DeployUpgrade: MIPS oracle is not set correctly");

        // Verify AnchorStateRegistry configuration.
        IAnchorStateRegistry asr = IAnchorStateRegistry(mustGetAddress("AnchorStateRegistryProxy"));
        (Hash root1, uint256 l2BlockNumber1) = asr.anchors(GameTypes.PERMISSIONED_CANNON);
        assertEq(
            root1.raw(),
            cfg.faultGameGenesisOutputRoot(),
            "DeployUpgrade: AnchorStateRegistry root is not set correctly"
        );
        assertEq(
            l2BlockNumber1,
            cfg.faultGameGenesisBlock(),
            "DeployUpgrade: AnchorStateRegistry l2BlockNumber is not set correctly"
        );

        // Verify DisputeGameFactory configuration.
        IDisputeGameFactory dgf = IDisputeGameFactory(mustGetAddress("DisputeGameFactoryProxy"));
        assertEq(
            dgf.initBonds(GameTypes.PERMISSIONED_CANNON),
            0 ether,
            "DeployUpgrade: DisputeGameFactory initBonds is not set correctly"
        );
        assertEq(
            address(dgf.gameImpls(GameTypes.CANNON)),
            address(0),
            "DeployUpgrade: DisputeGameFactory gameImpls CANNON is not set correctly"
        );
        assertEq(
            address(dgf.gameImpls(GameTypes.PERMISSIONED_CANNON)),
            mustGetAddress("PermissionedDisputeGame"),
            "DeployUpgrade: DisputeGameFactory gameImpls PERMISSIONED_CANNON is not set correctly"
        );

        // Verify security override yoke configuration.
        address soyGameAddr = address(dgf.gameImpls(GameTypes.PERMISSIONED_CANNON));
        PermissionedDisputeGame soyGameImpl = PermissionedDisputeGame(payable(soyGameAddr));
        assertEq(
            soyGameImpl.proposer(),
            cfg.l2OutputOracleProposer(),
            "DeployUpgrade: PermissionedDisputeGame proposer is not set correctly"
        );
        assertEq(
            soyGameImpl.challenger(),
            cfg.l2OutputOracleChallenger(),
            "DeployUpgrade: PermissionedDisputeGame challenger is not set correctly"
        );
        assertEq(
            soyGameImpl.maxGameDepth(),
            cfg.faultGameMaxDepth(),
            "DeployUpgrade: PermissionedDisputeGame maxGameDepth is not set correctly"
        );
        assertEq(
            soyGameImpl.splitDepth(),
            cfg.faultGameSplitDepth(),
            "DeployUpgrade: PermissionedDisputeGame splitDepth is not set correctly"
        );
        assertEq(
            soyGameImpl.clockExtension().raw(),
            cfg.faultGameClockExtension(),
            "DeployUpgrade: PermissionedDisputeGame clockExtension is not set correctly"
        );
        assertEq(
            soyGameImpl.maxClockDuration().raw(),
            cfg.faultGameMaxClockDuration(),
            "DeployUpgrade: PermissionedDisputeGame maxClockDuration is not set correctly"
        );
        assertEq(
            soyGameImpl.absolutePrestate().raw(),
            bytes32(cfg.faultGameAbsolutePrestate()),
            "DeployUpgrade: PermissionedDisputeGame absolutePrestate is not set correctly"
        );
        assertEq(
            address(soyGameImpl.weth()),
            soyWethProxyAddr,
            "DeployUpgrade: PermissionedDisputeGame weth is not set correctly"
        );
        assertEq(
            address(soyGameImpl.anchorStateRegistry()),
            address(asr),
            "DeployUpgrade: PermissionedDisputeGame anchorStateRegistry is not set correctly"
        );
        assertEq(
            address(soyGameImpl.vm()), address(mips), "DeployUpgrade: PermissionedDisputeGame vm is not set correctly"
        );
    }

    /// @notice Prints a summary of the configuration used to deploy this system.
    function printConfigSummary() internal view {
        console.log("Configuration Summary (chainid: %d)", block.chainid);
        console.log("    0. Use Fault Proofs: %s", cfg.useFaultProofs() ? "true" : "false");
        console.log("    1. Absolute Prestate: %x", cfg.faultGameAbsolutePrestate());
        console.log("    2. Max Depth: %d", cfg.faultGameMaxDepth());
        console.log("    3. Output / Execution split Depth: %d", cfg.faultGameSplitDepth());
        console.log("    4. Clock Extension (seconds): %d", cfg.faultGameClockExtension());
        console.log("    5. Max Clock Duration (seconds): %d", cfg.faultGameMaxClockDuration());
        console.log("    6. L2 Genesis block number: %d", cfg.faultGameGenesisBlock());
        console.log("    7. L2 Genesis output root: %x", uint256(cfg.faultGameGenesisOutputRoot()));
        console.log("    8. Proof Maturity Delay (seconds): %d", cfg.proofMaturityDelaySeconds());
        console.log("    9. Dispute Game Finality Delay (seconds): %d", cfg.disputeGameFinalityDelaySeconds());
        console.log("   10. Respected Game Type: %d", cfg.respectedGameType());
        console.log("   11. Preimage Oracle Min Proposal Size (bytes): %d", cfg.preimageOracleMinProposalSize());
        console.log("   12. Preimage Oracle Challenge Period (seconds): %d", cfg.preimageOracleChallengePeriod());
        console.log("   13. ProxyAdmin: %s", proxyAdmin);
        console.log("   14. SystemOwnerSafe: %s", systemOwnerSafe);
        console.log("   15. SuperchainConfigProxy: %s", superchainConfigProxy);
    }

    /// @notice Prints a summary of the contracts deployed during this script.
    function printDeploymentSummary() internal view {
        console.log("Deployment Summary (chainid: %d)", block.chainid);
        console.log("    0. DisputeGameFactoryProxy: %s", mustGetAddress("DisputeGameFactoryProxy"));
        console.log("    1. AnchorStateRegistryProxy: %s", mustGetAddress("AnchorStateRegistryProxy"));
        console.log("    2. AnchorStateRegistryImpl: %s", mustGetAddress("AnchorStateRegistry"));
        console.log("    3. PermissionedDelayedWETHProxy: %s", mustGetAddress("PermissionedDelayedWETHProxy"));
        console.log(
            "    4. PermissionedDisputeGame: %s",
            address(
                IDisputeGameFactory(mustGetAddress("DisputeGameFactoryProxy")).gameImpls(GameTypes.PERMISSIONED_CANNON)
            )
        );
    }
}
