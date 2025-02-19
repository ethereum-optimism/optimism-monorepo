// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { OPContractsManager } from "src/L1/OPContractsManager.sol";

// Interfaces
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { IProtocolVersions } from "interfaces/L1/IProtocolVersions.sol";
import { IPermissionedDisputeGame } from "interfaces/dispute/IPermissionedDisputeGame.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { IFaultDisputeGame } from "interfaces/dispute/IFaultDisputeGame.sol";
import { IDelayedWETH } from "interfaces/dispute/IDelayedWETH.sol";
import { IProxyAdmin } from "interfaces/universal/IProxyAdmin.sol";

// Libraries
import { Claim, GameTypes } from "src/dispute/lib/Types.sol";

///  @title OPPrestateUpdater
///  @notice A custom implementation of OPContractsManager that enables updating the prestate hash
///          for the permissioned and fault dispute games on a set of chains.
contract OPPrestateUpdater is OPContractsManager {
    /// @notice Thrown when a function from the parent (OPCM) is not implemented.
    error NotImplemented();

    /// @notice Thrown when the prestate of a permissioned disputed game is 0.
    error PrestateRequired();

    // @return Version string
    /// @custom:semver 1.6.0
    function version() public pure override returns (string memory) {
        return "1.6.0";
    }

    // @notice Constructs the OPPrestateUpdater contract
    // @param _superchainConfig Address of the SuperchainConfig contract
    // @param _protocolVersions Address of the ProtocolVersions contract
    // @param _blueprints Addresses of Blueprint contracts
    constructor(
        ISuperchainConfig _superchainConfig,
        IProtocolVersions _protocolVersions,
        Blueprints memory _blueprints
    )
        OPContractsManager(
            _superchainConfig,
            _protocolVersions,
            IProxyAdmin(address(0)),
            "",
            _blueprints,
            Implementations(
                address(0), // l1CrossDomainMessenger
                address(0), // l1StandardBridge
                address(0), // l2OutputOracle
                address(0), // optimismPortal
                address(0), // optimismMintableERC20Factory
                address(0), // protocolVersions
                address(0), // superchainConfig
                address(0), // systemConfig
                address(0), // l1ERC721Bridge
                address(0), // disputeGameFactory
                address(0), // permissionedDisputeGame
                address(0) // faultDisputeGame
            ),
            address(0)
        )
    { }

    /// @notice Overrides the l1ContractsRelease function to return "none", as this OPCM
    /// is not releasing new contracts.
    function l1ContractsRelease() external pure override returns (string memory) {
        return "none";
    }

    function deploy(DeployInput memory _input) external pure override returns (DeployOutput memory) {
        _input; // Silence warning
        revert NotImplemented();
    }

    function upgrade(OpChainConfig[] memory _opChainConfigs) external pure override {
        _opChainConfigs; // Silence warning
        revert NotImplemented();
    }

    function addGameType(AddGameInput[] memory _gameConfigs) public pure override returns (AddGameOutput[] memory) {
        _gameConfigs; // Silence warning
        revert NotImplemented();
    }

    /// @notice Updates the prestate hash for a new game type while keeping all other parameters the same
    /// @param _prestateUpdateInputs The new prestate hash to use
    function updatePrestate(OpChainConfig[] memory _prestateUpdateInputs) external {
        // Loop through each chain and prestate hash
        for (uint256 i = 0; i < _prestateUpdateInputs.length; i++) {
            if (Claim.unwrap(_prestateUpdateInputs[i].absolutePrestate) == bytes32(0)) {
                revert PrestateRequired();
            }

            // Get the DisputeGameFactory and existing game implementations
            IDisputeGameFactory dgf =
                IDisputeGameFactory(_prestateUpdateInputs[i].systemConfigProxy.disputeGameFactory());
            IFaultDisputeGame fdg = IFaultDisputeGame(address(getGameImplementation(dgf, GameTypes.CANNON)));
            IPermissionedDisputeGame pdg =
                IPermissionedDisputeGame(address(getGameImplementation(dgf, GameTypes.PERMISSIONED_CANNON)));

            // All chains must have a permissioned game, but not all chains must have a fault dispute game.
            // Whether a chain has a fault dispute game determines how many AddGameInput objects are needed.
            bool hasFDG = address(fdg) != address(0);

            AddGameInput[] memory inputs = new AddGameInput[](hasFDG ? 2 : 1);
            AddGameInput memory pdgInput;
            AddGameInput memory fdgInput;

            // Get the existing game parameters and init bond for the permissioned game
            IFaultDisputeGame.GameConstructorParams memory pdgParams =
                getGameConstructorParams(IFaultDisputeGame(address(pdg)));
            uint256 initBond = dgf.initBonds(GameTypes.PERMISSIONED_CANNON);

            string memory saltMixer = reusableSaltMixer(_prestateUpdateInputs[i]);
            // Create game input with updated prestate but same other params
            pdgInput = AddGameInput({
                disputeAbsolutePrestate: _prestateUpdateInputs[i].absolutePrestate,
                saltMixer: saltMixer,
                systemConfig: _prestateUpdateInputs[i].systemConfigProxy,
                proxyAdmin: _prestateUpdateInputs[i].proxyAdmin,
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

            // If a fault dispute game exists, create a new game with the same parameters but updated prestate.
            if (hasFDG) {
                // Get the existing game parameters and init bond for the fault dispute game
                IFaultDisputeGame.GameConstructorParams memory fdgParams =
                    getGameConstructorParams(IFaultDisputeGame(address(fdg)));
                initBond = dgf.initBonds(GameTypes.CANNON);

                // Create game input with updated prestate but same other params
                fdgInput = AddGameInput({
                    disputeAbsolutePrestate: _prestateUpdateInputs[i].absolutePrestate,
                    saltMixer: saltMixer,
                    systemConfig: _prestateUpdateInputs[i].systemConfigProxy,
                    proxyAdmin: _prestateUpdateInputs[i].proxyAdmin,
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
            if (hasFDG) {
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
