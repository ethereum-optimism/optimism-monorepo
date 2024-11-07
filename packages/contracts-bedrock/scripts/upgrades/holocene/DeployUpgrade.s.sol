// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

// Forge
import { Script } from "forge-std/Script.sol";
import { console2 as console } from "forge-std/console2.sol";

// Scripts
import { Deployer } from "scripts/deploy/Deployer.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

// Utils
import "src/dispute/lib/Types.sol";

// Interfaces
import { ISystemConfig } from "src/L1/interfaces/ISystemConfig.sol";
import {
    IFaultDisputeGame,
    IBigStepper,
    IAnchorStateRegistry,
    IDelayedWETH
} from "src/dispute/interfaces/IFaultDisputeGame.sol";
import { IPermissionedDisputeGame } from "src/dispute/interfaces/IPermissionedDisputeGame.sol";
import { IMIPS, IPreimageOracle } from "src/cannon/interfaces/IMIPS.sol";

/// @title DeployUpgrade
/// @notice A deployment script for smart contract upgrades surrounding the Holocene hardfork.
contract DeployUpgrade is Deployer {
    /// @dev The entrypoint to the deployment script.
    function deploy(address _preimageOracle, address _anchorStateRegistry, address _delayedWETH) public {
        // Shim the existing contracts that this upgrade is dependent on.
        shim(_preimageOracle, _anchorStateRegistry, _delayedWETH);

        // Deploy new implementations.
        deploySystemConfigImplementation();
        deployMIPSImplementation();
        deployFaultDisputeGameImplementation();
        deployPermissionedDisputeGameImplementation();

        // Run deployment checks.
        checkMIPS();
        checkFaultDisputeGame();
        checkPermissionedDisputeGame();

        // Print the deployment summary.
        printSummary();
    }

    /// @dev Shims the existing contracts that this upgrade is dependent on.
    function shim(address _preimageOracle, address _anchorStateRegistry, address _delayedWETH) public {
        prankDeployment("PreimageOracle", _preimageOracle);
        prankDeployment("AnchorStateRegistry", _anchorStateRegistry);
        prankDeployment("DelayedWETH", _delayedWETH);
    }

    /// @dev Deploys the Holocene `SystemConfig` implementation contract.
    function deploySystemConfigImplementation() public {
        vm.broadcast(msg.sender);
        address systemConfig = DeployUtils.create1(
            "SystemConfig", DeployUtils.encodeConstructor(abi.encodeCall(ISystemConfig.__constructor__, ()))
        );
        save("SystemConfig", systemConfig);
    }

    /// @dev Deploys the new `MIPS` implementation contract.
    function deployMIPSImplementation() public {
        vm.broadcast(msg.sender);
        address mips = DeployUtils.create1({
            _name: "MIPS",
            _args: DeployUtils.encodeConstructor(
                abi.encodeCall(IMIPS.__constructor__, (IPreimageOracle(mustGetAddress("PreimageOracle"))))
            )
        });
        save("MIPS", mips);
    }

    /// @dev Checks if the `MIPS` contract is correctly configured.
    function checkMIPS() public view {
        IMIPS mips = IMIPS(mustGetAddress("MIPS"));
        require(
            address(mips.oracle()) == mustGetAddress("PreimageOracle"), "DeployHoloceneUpgrade: invalid MIPS oracle"
        );
    }

    /// @dev Deploys the Holocene `FaultDisputeGame` implementation contract.
    function deployFaultDisputeGameImplementation() public {
        bytes memory constructorInput = abi.encodeCall(
            IFaultDisputeGame.__constructor__,
            (
                GameTypes.CANNON,
                Claim.wrap(bytes32(cfg.faultGameAbsolutePrestate())),
                cfg.faultGameMaxDepth(),
                cfg.faultGameSplitDepth(),
                Duration.wrap(uint64(cfg.faultGameClockExtension())),
                Duration.wrap(uint64(cfg.faultGameMaxClockDuration())),
                IBigStepper(mustGetAddress("MIPS")),
                IDelayedWETH(payable(mustGetAddress("DelayedWETH"))),
                IAnchorStateRegistry(mustGetAddress("AnchorStateRegistry")),
                cfg.l2ChainID()
            )
        );

        vm.broadcast(msg.sender);
        address fdg = DeployUtils.create1("FaultDisputeGame", DeployUtils.encodeConstructor(constructorInput));
        save("FaultDisputeGame", fdg);
    }

    /// @dev Checks if the `FaultDisputeGame` contract is correctly configured.
    function checkFaultDisputeGame() public view {
        IFaultDisputeGame fdg = IFaultDisputeGame(mustGetAddress("FaultDisputeGame"));
        require(
            fdg.gameType().raw() == GameTypes.CANNON.raw(), "DeployHoloceneUpgrade: invalid FaultDisputeGame gameType"
        );
        require(
            fdg.absolutePrestate().raw() == bytes32(cfg.faultGameAbsolutePrestate()),
            "DeployHoloceneUpgrade: invalid FaultDisputeGame absolutePrestate"
        );
        require(
            fdg.maxGameDepth() == cfg.faultGameMaxDepth(), "DeployHoloceneUpgrade: invalid FaultDisputeGame maxDepth"
        );
        require(
            fdg.splitDepth() == cfg.faultGameSplitDepth(), "DeployHoloceneUpgrade: invalid FaultDisputeGame splitDepth"
        );
        require(
            fdg.clockExtension().raw() == cfg.faultGameClockExtension(),
            "DeployHoloceneUpgrade: invalid FaultDisputeGame clockExtension"
        );
        require(
            fdg.maxClockDuration().raw() == cfg.faultGameMaxClockDuration(),
            "DeployHoloceneUpgrade: invalid FaultDisputeGame maxClockDuration"
        );
        require(address(fdg.vm()) == mustGetAddress("MIPS"), "DeployHoloceneUpgrade: invalid FaultDisputeGame MIPS");
        require(
            address(fdg.weth()) == mustGetAddress("DelayedWETH"),
            "DeployHoloceneUpgrade: invalid FaultDisputeGame DelayedWETH"
        );
        require(
            address(fdg.anchorStateRegistry()) == mustGetAddress("AnchorStateRegistry"),
            "DeployHoloceneUpgrade: invalid FaultDisputeGame AnchorStateRegistry"
        );
        require(fdg.l2ChainId() == cfg.l2ChainID(), "DeployHoloceneUpgrade: invalid FaultDisputeGame l2ChainID");
    }

    /// @dev Deploys the Holocene `PermissionedDisputeGame` implementation contract.
    function deployPermissionedDisputeGameImplementation() public {
        bytes memory constructorInput = abi.encodeCall(
            IPermissionedDisputeGame.__constructor__,
            (
                GameTypes.PERMISSIONED_CANNON,
                Claim.wrap(bytes32(cfg.faultGameAbsolutePrestate())),
                cfg.faultGameMaxDepth(),
                cfg.faultGameSplitDepth(),
                Duration.wrap(uint64(cfg.faultGameClockExtension())),
                Duration.wrap(uint64(cfg.faultGameMaxClockDuration())),
                IBigStepper(mustGetAddress("MIPS")),
                IDelayedWETH(payable(mustGetAddress("DelayedWETH"))),
                IAnchorStateRegistry(mustGetAddress("AnchorStateRegistry")),
                cfg.l2ChainID(),
                cfg.l2OutputOracleProposer(),
                cfg.l2OutputOracleChallenger()
            )
        );

        vm.broadcast(msg.sender);
        address fdg = DeployUtils.create1("PermissionedDisputeGame", DeployUtils.encodeConstructor(constructorInput));
        save("PermissionedDisputeGame", fdg);
    }

    /// @dev Checks if the `PermissionedDisputeGame` contract is correctly configured.
    function checkPermissionedDisputeGame() public view {
        IPermissionedDisputeGame pdg = IPermissionedDisputeGame(mustGetAddress("PermissionedDisputeGame"));
        require(
            pdg.gameType().raw() == GameTypes.PERMISSIONED_CANNON.raw(),
            "DeployHoloceneUpgrade: invalid PermissionedDisputeGame gameType"
        );
        require(
            pdg.absolutePrestate().raw() == bytes32(cfg.faultGameAbsolutePrestate()),
            "DeployHoloceneUpgrade: invalid PermissionedDisputeGame absolutePrestate"
        );
        require(
            pdg.maxGameDepth() == cfg.faultGameMaxDepth(),
            "DeployHoloceneUpgrade: invalid PermissionedDisputeGame maxDepth"
        );
        require(
            pdg.splitDepth() == cfg.faultGameSplitDepth(),
            "DeployHoloceneUpgrade: invalid PermissionedDisputeGame splitDepth"
        );
        require(
            pdg.clockExtension().raw() == cfg.faultGameClockExtension(),
            "DeployHoloceneUpgrade: invalid PermissionedDisputeGame clockExtension"
        );
        require(
            pdg.maxClockDuration().raw() == cfg.faultGameMaxClockDuration(),
            "DeployHoloceneUpgrade: invalid PermissionedDisputeGame maxClockDuration"
        );
        require(
            address(pdg.vm()) == mustGetAddress("MIPS"), "DeployHoloceneUpgrade: invalid PermissionedDisputeGame MIPS"
        );
        require(
            address(pdg.weth()) == mustGetAddress("DelayedWETH"),
            "DeployHoloceneUpgrade: invalid PermissionedDisputeGame DelayedWETH"
        );
        require(
            address(pdg.anchorStateRegistry()) == mustGetAddress("AnchorStateRegistry"),
            "DeployHoloceneUpgrade: invalid PermissionedDisputeGame AnchorStateRegistry"
        );
        require(pdg.l2ChainId() == cfg.l2ChainID(), "DeployHoloceneUpgrade: invalid PermissionedDisputeGame l2ChainID");
        require(
            pdg.proposer() == cfg.l2OutputOracleProposer(),
            "DeployHoloceneUpgrade: invalid PermissionedDisputeGame proposer"
        );
        require(
            pdg.challenger() == cfg.l2OutputOracleChallenger(),
            "DeployHoloceneUpgrade: invalid PermissionedDisputeGame challenger"
        );
    }

    /// @dev Prints a summary of the deployment.
    function printSummary() internal view {
        console.log("1. SystemConfig: %s", mustGetAddress("SystemConfig"));
        console.log("2. MIPS: %s", mustGetAddress("MIPS"));
        console.log("3. FaultDisputeGame: %s", mustGetAddress("FaultDisputeGame"));
        console.log("4. PermissionedDisputeGame: %s", mustGetAddress("PermissionedDisputeGame"));
    }
}
