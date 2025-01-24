// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { OPContractsManager } from "src/L1/OPContractsManager.sol";

// Interfaces
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { IProtocolVersions } from "interfaces/L1/IProtocolVersions.sol";
import { IPermissionedDisputeGame } from "interfaces/dispute/IPermissionedDisputeGame.sol";
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { IFaultDisputeGame } from "interfaces/dispute/IFaultDisputeGame.sol";
import { IDelayedWETH } from "interfaces/dispute/IDelayedWETH.sol";

// Libraries
import { Constants } from "src/libraries/Constants.sol";
import { Claim, GameTypes } from "src/dispute/lib/Types.sol";

/**
 * @title CustomOPContractsManager
 * @notice A custom implementation of OPContractsManager that allows for modified deployment parameters
 */
contract OPPrestateUpdater is OPContractsManager {

    struct PrestateUpdateInput {
        OpChain opChain;
        Claim faultDisputePrestate;
        Claim permissionedDisputePrestate;
    }

    /**
     * @notice Returns the version string for this contract
     * @return Version string
     */
    function version() public pure override returns (string memory) {
        return "1.0.0";
    }

    /**
     * @notice Constructs the CustomOPContractsManager contract
     * @param _superchainConfig Address of the SuperchainConfig contract
     * @param _protocolVersions Address of the ProtocolVersions contract
     * @param _l1ContractsRelease Version string for L1 contracts release
     * @param _blueprints Addresses of Blueprint contracts
     * @param _implementations Addresses of implementation contracts
     * @param _upgradeController Address of the upgrade controller
     */
    constructor(
        ISuperchainConfig _superchainConfig,
        IProtocolVersions _protocolVersions,
        string memory _l1ContractsRelease,
        Blueprints memory _blueprints,
        Implementations memory _implementations,
        address _upgradeController
    ) OPContractsManager(
        _superchainConfig,
        _protocolVersions,
        _l1ContractsRelease,
        _blueprints,
        _implementations,
        _upgradeController
    ) {}

    /// @notice Overrides the l1ContractsRelease function to return "none", as this OPCM
    /// is not releasing new contracts.
    function l1ContractsRelease() external pure override returns (string memory) {
        return "none";
    }

    function deploy(DeployInput memory) external pure override returns (DeployOutput memory) {
        revert("Not implemented");
    }

    function upgrade(OpChain[] memory) external pure override {
        revert("Not implemented");
    }

    function addGameType(AddGameInput[] memory) public pure override returns (AddGameOutput[] memory) {
        revert("Not implemented");
    }

    /// @notice Updates the prestate hash for a new game type while keeping all other parameters the same
    /// @param _prestateUpdateInputs The new prestate hash to use
    function updatePrestate(PrestateUpdateInput[] memory _prestateUpdateInputs) external {
        // Loop through each chain and prestate hash
        for (uint256 i = 0; i < _prestateUpdateInputs.length; i++) {
            bool hasFDG = Claim.unwrap(_prestateUpdateInputs[i].faultDisputePrestate) != bytes32(0);
            AddGameInput[] memory inputs = new AddGameInput[](hasFDG ? 2 : 1);
            AddGameInput memory pdgInput;
            AddGameInput memory fdgInput;

            if(Claim.unwrap(_prestateUpdateInputs[i].permissionedDisputePrestate) == bytes32(0)) {
                revert("Permissioned dispute prestate is required");
            }
            // Get the current game implementation to copy parameters from
            IDisputeGameFactory dgf = IDisputeGameFactory(_prestateUpdateInputs[i].opChain.systemConfigProxy.disputeGameFactory());
            IPermissionedDisputeGame pdg = IPermissionedDisputeGame(
                address(
                    getGameImplementation(
                        dgf,
                        GameTypes.PERMISSIONED_CANNON
                    )
                )
            );
            uint256 initBond = dgf.initBonds(GameTypes.PERMISSIONED_CANNON);

            // Get the existing game parameters
            IFaultDisputeGame.GameConstructorParams memory pdgParams = getGameConstructorParams(IFaultDisputeGame(address(pdg)));

            // Create game input with updated prestate but same other params
            pdgInput = AddGameInput({
                disputeAbsolutePrestate: _prestateUpdateInputs[i].permissionedDisputePrestate,
                saltMixer: "prestate_update",
                systemConfig: _prestateUpdateInputs[i].opChain.systemConfigProxy,
                proxyAdmin: _prestateUpdateInputs[i].opChain.proxyAdmin,
                delayedWETH: IDelayedWETH(payable(address(pdgParams.weth))),
                disputeGameType: pdgParams.gameType,
                disputeMaxGameDepth: pdgParams.maxGameDepth,
                disputeSplitDepth: pdgParams.splitDepth,
                disputeClockExtension: pdgParams.clockExtension,
                disputeMaxClockDuration: pdgParams.maxClockDuration,
                initialBond: initBond,
                vm: pdgParams.vm,
                permissioned: true
            });

            // If fault dispute prestate is provided, create a new game with the same parameters but updated prestate
            if(hasFDG) {
                // Get the current game implementation to copy parameters from
                IFaultDisputeGame fdg = IFaultDisputeGame(
                    address(
                        getGameImplementation(
                            dgf,
                            GameTypes.CANNON
                        )
                    )
                );
                if (address(fdg) == address(0)) revert("Fault dispute game not found");
                initBond = dgf.initBonds(GameTypes.CANNON);

                // Get the existing game parameters
                IFaultDisputeGame.GameConstructorParams memory fdgParams = getGameConstructorParams(IFaultDisputeGame(address(fdg)));

                // Create game input with updated prestate but same other params
                fdgInput = AddGameInput({
                    disputeAbsolutePrestate: _prestateUpdateInputs[i].faultDisputePrestate,
                    saltMixer: "prestate_update",
                    systemConfig: _prestateUpdateInputs[i].opChain.systemConfigProxy,
                    proxyAdmin: _prestateUpdateInputs[i].opChain.proxyAdmin,
                    delayedWETH: IDelayedWETH(payable(address(fdgParams.weth))),
                    disputeGameType: fdgParams.gameType,
                    disputeMaxGameDepth: fdgParams.maxGameDepth,
                    disputeSplitDepth: fdgParams.splitDepth,
                    disputeClockExtension: fdgParams.clockExtension,
                    disputeMaxClockDuration: fdgParams.maxClockDuration,
                    initialBond: initBond,
                    vm: fdgParams.vm,
                    permissioned: false
                });
            }

            // Game inputs must be ordered with increasing game type values. So FDG is first if it exists.
            if(hasFDG) {
                inputs[0] = fdgInput;
                inputs[1] = pdgInput;
            } else {
                inputs[0] = pdgInput;
            }
            // Add the new game type with updated prestate
            super.addGameType(inputs);
        }

    }
}
