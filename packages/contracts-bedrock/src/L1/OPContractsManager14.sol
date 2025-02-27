// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { OPContractsManager } from "./OPContractsManager.sol";
// Libraries
import { Blueprint } from "src/libraries/Blueprint.sol";
import { Claim, GameType, GameTypes } from "src/dispute/lib/Types.sol";
// Interfaces
import { IBigStepper } from "interfaces/dispute/IBigStepper.sol";
import { IDisputeGame } from "interfaces/dispute/IDisputeGame.sol";
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { IFaultDisputeGame } from "interfaces/dispute/IFaultDisputeGame.sol";
import { IPermissionedDisputeGame } from "interfaces/dispute/IPermissionedDisputeGame.sol";
import { IProtocolVersions } from "interfaces/L1/IProtocolVersions.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { IProxyAdmin } from "interfaces/universal/IProxyAdmin.sol";

///  @title OPContractsManager14
///  @notice Represents the new OPContractsManager for Upgrade 14
contract OPContractsManager14 is OPContractsManager {
    function version() public pure override returns (string memory) {
        return string.concat(super.version(), "+upgrade14.1");
    }

    // TODO: Review required arguments
    constructor(
        ISuperchainConfig _superchainConfig,
        IProtocolVersions _protocolVersions,
        IProxyAdmin _superchainProxyAdmin,
        string memory _l1ContractsRelease,
        Blueprints memory _blueprints,
        Implementations memory _implementations,
        address _upgradeController
    )
    OPContractsManager(
    _superchainConfig,
    _protocolVersions,
    _superchainProxyAdmin,
    _l1ContractsRelease,
    _blueprints,
    _implementations,
    _upgradeController
    )
    { }

    /// @notice Upgrades a set of chains to the latest implementation contracts
    /// @param _opChainConfigs Array of OpChain structs, one per chain to upgrade
    /// @dev This function is intended to be called via DELEGATECALL from the Upgrade Controller Safe
    function upgrade(OpChainConfig[] memory _opChainConfigs) external override {
        if (address(this) == address(thisOPCM)) revert OnlyDelegatecall();

        // If this is delegatecalled by the upgrade controller, set isRC to false first, else, continue execution.
        if (address(this) == upgradeController) {
            // Set isRC to false.
            // This function asserts that the caller is the upgrade controller.
            thisOPCM.setRC(false);
        }

        Implementations memory impls = getImplementations();
        Blueprints memory bps = getBlueprints();

        for (uint256 i = 0; i < _opChainConfigs.length; i++) {
            assertValidOpChainConfig(_opChainConfigs[i]);
            ISystemConfig.Addresses memory opChainAddrs = _opChainConfigs[i].systemConfigProxy.getAddresses();

            // Check that all contracts have the correct superchainConfig
            if (
                getSuperchainConfig(opChainAddrs.optimismPortal) != superchainConfig
                    || getSuperchainConfig(opChainAddrs.l1CrossDomainMessenger) != superchainConfig
                    || getSuperchainConfig(opChainAddrs.l1ERC721Bridge) != superchainConfig
                    || getSuperchainConfig(opChainAddrs.l1StandardBridge) != superchainConfig
            ) {
                revert SuperchainConfigMismatch(_opChainConfigs[i].systemConfigProxy);
            }

            // -------- Discover and Upgrade Proofs Contracts --------

            // All chains have the Permissioned Dispute Game.
            IPermissionedDisputeGame permissionedDisputeGame = IPermissionedDisputeGame(
                address(
                    getGameImplementation(
                        IDisputeGameFactory(opChainAddrs.disputeGameFactory), GameTypes.PERMISSIONED_CANNON
                    )
                )
            );
            // We're also going to need the l2ChainId below, so we cache it in the outer scope.
            uint256 l2ChainId = getL2ChainId(IFaultDisputeGame(address(permissionedDisputeGame)));

            // Now retrieve the permissionless game. If it exists, replace its implementation.
            IFaultDisputeGame permissionlessDisputeGame = IFaultDisputeGame(
                address(getGameImplementation(IDisputeGameFactory(opChainAddrs.disputeGameFactory), GameTypes.CANNON))
            );

            if (address(permissionlessDisputeGame) != address(0)) {
                // Deploy and set a new permissionless game to update its prestate
                deployAndSetNewGameImpl({
                    _l2ChainId: l2ChainId,
                    _disputeGame: IDisputeGame(address(permissionlessDisputeGame)),
                    _gameType: GameTypes.CANNON,
                    _opChainConfig: _opChainConfigs[i],
                    _implementations: impls,
                    _blueprints: bps,
                    _opChainAddrs: opChainAddrs
                });
            }

            // Emit the upgraded event with the address of the caller. Since this will be a delegatecall,
            // the caller will be the value of the ADDRESS opcode.
            emit Upgraded(l2ChainId, _opChainConfigs[i].systemConfigProxy, address(this));
        }
    }

    /// @notice Deploys and sets a new dispute game implementation
    /// @param _l2ChainId The L2 chain ID
    /// @param _disputeGame The current dispute game implementation
    /// @param _gameType The type of game to deploy
    /// @param _opChainConfig The OP chain configuration
    /// @param _blueprints The blueprint addresses
    /// @param _implementations The implementation addresses
    /// @param _opChainAddrs The OP chain addresses
    function deployAndSetNewGameImpl(
        uint256 _l2ChainId,
        IDisputeGame _disputeGame,
        GameType _gameType,
        OpChainConfig memory _opChainConfig,
        Blueprints memory _blueprints,
        Implementations memory _implementations,
        ISystemConfig.Addresses memory _opChainAddrs
    )
        internal
    {
        // Get the constructor params for the game
        IFaultDisputeGame.GameConstructorParams memory params =
            getGameConstructorParams(IFaultDisputeGame(address(_disputeGame)));

        // Set the new vm value.
        params.vm = IBigStepper(_implementations.mipsImpl);
        // Set the new absolute prestate
        if (Claim.unwrap(_opChainConfig.absolutePrestate) == bytes32(0)) {
            revert PrestateNotSet();
        }
        params.absolutePrestate = _opChainConfig.absolutePrestate;

        IDisputeGame newGame;
        if (GameType.unwrap(_gameType) == GameType.unwrap(GameTypes.PERMISSIONED_CANNON)) {
            address proposer = getProposer(IPermissionedDisputeGame(address(_disputeGame)));
            address challenger = getChallenger(IPermissionedDisputeGame(address(_disputeGame)));
            newGame = IDisputeGame(
                Blueprint.deployFrom(
                    _blueprints.permissionedDisputeGame1,
                    _blueprints.permissionedDisputeGame2,
                    computeSalt(_l2ChainId, reusableSaltMixer(_opChainConfig), "PermissionedDisputeGame"),
                    encodePermissionedFDGConstructor(params, proposer, challenger)
                )
            );
        } else {
            newGame = IDisputeGame(
                Blueprint.deployFrom(
                    _blueprints.permissionlessDisputeGame1,
                    _blueprints.permissionlessDisputeGame2,
                    computeSalt(_l2ChainId, reusableSaltMixer(_opChainConfig), "PermissionlessDisputeGame"),
                    encodePermissionlessFDGConstructor(params)
                )
            );
        }
        setDGFImplementation(IDisputeGameFactory(_opChainAddrs.disputeGameFactory), _gameType, IDisputeGame(newGame));
    }
}
