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

        // If the SuperchainConfig is not already upgraded, upgrade it.
        if (superchainProxyAdmin.getProxyImplementation(address(superchainConfig)) != impls.superchainConfigImpl) {
            // Attempt to upgrade. If the ProxyAdmin is not the SuperchainConfig's admin, this will revert.
            upgradeTo(superchainProxyAdmin, address(superchainConfig), impls.superchainConfigImpl);
        }

        // If the ProtocolVersions contract is not already upgraded, upgrade it.
        if (superchainProxyAdmin.getProxyImplementation(address(protocolVersions)) != impls.protocolVersionsImpl) {
            upgradeTo(superchainProxyAdmin, address(protocolVersions), impls.protocolVersionsImpl);
        }

        for (uint256 i = 0; i < _opChainConfigs.length; i++) {
            assertValidOpChainConfig(_opChainConfigs[i]);

            // After Upgrade 13, we will be able to use systemConfigProxy.getAddresses() here.
            ISystemConfig.Addresses memory opChainAddrs = ISystemConfig.Addresses({
                l1CrossDomainMessenger: _opChainConfigs[i].systemConfigProxy.l1CrossDomainMessenger(),
                l1ERC721Bridge: _opChainConfigs[i].systemConfigProxy.l1ERC721Bridge(),
                l1StandardBridge: _opChainConfigs[i].systemConfigProxy.l1StandardBridge(),
                disputeGameFactory: address(getDisputeGameFactory(_opChainConfigs[i].systemConfigProxy)),
                optimismPortal: _opChainConfigs[i].systemConfigProxy.optimismPortal(),
                optimismMintableERC20Factory: _opChainConfigs[i].systemConfigProxy.optimismMintableERC20Factory()
            });

            // Check that all contracts have the correct superchainConfig
            if (
                getSuperchainConfig(opChainAddrs.optimismPortal) != superchainConfig
                || getSuperchainConfig(opChainAddrs.l1CrossDomainMessenger) != superchainConfig
                || getSuperchainConfig(opChainAddrs.l1ERC721Bridge) != superchainConfig
                || getSuperchainConfig(opChainAddrs.l1StandardBridge) != superchainConfig
            ) {
                revert SuperchainConfigMismatch(_opChainConfigs[i].systemConfigProxy);
            }

            // -------- Upgrade Contracts Stored in SystemConfig --------
            upgradeTo(
                _opChainConfigs[i].proxyAdmin, address(_opChainConfigs[i].systemConfigProxy), impls.systemConfigImpl
            );
            upgradeTo(
                _opChainConfigs[i].proxyAdmin, opChainAddrs.l1CrossDomainMessenger, impls.l1CrossDomainMessengerImpl
            );
            upgradeTo(_opChainConfigs[i].proxyAdmin, opChainAddrs.l1ERC721Bridge, impls.l1ERC721BridgeImpl);
            upgradeTo(_opChainConfigs[i].proxyAdmin, opChainAddrs.l1StandardBridge, impls.l1StandardBridgeImpl);
            upgradeTo(_opChainConfigs[i].proxyAdmin, opChainAddrs.disputeGameFactory, impls.disputeGameFactoryImpl);
            upgradeTo(_opChainConfigs[i].proxyAdmin, opChainAddrs.optimismPortal, impls.optimismPortalImpl);
            upgradeTo(
                _opChainConfigs[i].proxyAdmin,
                opChainAddrs.optimismMintableERC20Factory,
                impls.optimismMintableERC20FactoryImpl
            );

            // -------- Discover and Upgrade Proofs Contracts --------
            // Note that, the code below uses several independently scoped blocks to avoid stack too deep errors.

            // All chains have the Permissioned Dispute Game. We get it first so that we can use it to
            // retrieve its WETH and the Anchor State Registry when we need them.
            IPermissionedDisputeGame permissionedDisputeGame = IPermissionedDisputeGame(
                address(
                    getGameImplementation(
                        IDisputeGameFactory(opChainAddrs.disputeGameFactory), GameTypes.PERMISSIONED_CANNON
                    )
                )
            );
            // We're also going to need the l2ChainId below, so we cache it in the outer scope.
            uint256 l2ChainId = getL2ChainId(IFaultDisputeGame(address(permissionedDisputeGame)));

            // Replace the Anchor State Registry Proxy with a new Proxy and Implementation
            // For this upgrade, we are replacing the previous Anchor State Registry, thus we:
            // 1. deploy a new Anchor State Registry proxy
            // 2. get the starting anchor root corresponding to the currently respected game type.
            // 3. initialize the proxy with that anchor root
            IAnchorStateRegistry newAnchorStateRegistryProxy;
            {
                // Deploy a new proxy, because we're replacing the old one.
                // Include the system config address in the salt to ensure that the new proxy is unique,
                // even if another chains with the same L2 chain ID has been deployed by this contract.
                newAnchorStateRegistryProxy = IAnchorStateRegistry(
                    deployProxy({
                        _l2ChainId: l2ChainId,
                        _proxyAdmin: _opChainConfigs[i].proxyAdmin,
                        _saltMixer: reusableSaltMixer(_opChainConfigs[i]),
                        _contractName: "AnchorStateRegistry"
                    })
                );

                // Get the starting anchor root by:
                // 1. getting the anchor state registry from the Permissioned Dispute Game.
                // 2. getting the respected game type from the OptimismPortal.
                // 3. getting the anchor root for the respected game type from the Anchor State Registry.
                {
                    GameType gameType = IOptimismPortal2(payable(opChainAddrs.optimismPortal)).respectedGameType();
                    (Hash root, uint256 l2BlockNumber) =
                                            getAnchorStateRegistry(IFaultDisputeGame(address(permissionedDisputeGame))).anchors(gameType);
                    OutputRoot memory startingAnchorRoot = OutputRoot({ root: root, l2BlockNumber: l2BlockNumber });

                    upgradeToAndCall(
                        _opChainConfigs[i].proxyAdmin,
                        address(newAnchorStateRegistryProxy),
                        impls.anchorStateRegistryImpl,
                        abi.encodeCall(
                            IAnchorStateRegistry.initialize,
                            (
                                superchainConfig,
                                IDisputeGameFactory(opChainAddrs.disputeGameFactory),
                                IOptimismPortal2(payable(opChainAddrs.optimismPortal)),
                                startingAnchorRoot
                            )
                        )
                    );
                }

                // Deploy and set a new permissioned game to update its prestate

                deployAndSetNewGameImpl({
                    _l2ChainId: l2ChainId,
                    _disputeGame: IDisputeGame(address(permissionedDisputeGame)),
                    _newAnchorStateRegistryProxy: newAnchorStateRegistryProxy,
                    _gameType: GameTypes.PERMISSIONED_CANNON,
                    _opChainConfig: _opChainConfigs[i],
                    _implementations: impls,
                    _blueprints: bps,
                    _opChainAddrs: opChainAddrs
                });
            }

            // Now retrieve the permissionless game. If it exists, upgrade its weth and replace its implementation.
            IFaultDisputeGame permissionlessDisputeGame = IFaultDisputeGame(
                address(getGameImplementation(IDisputeGameFactory(opChainAddrs.disputeGameFactory), GameTypes.CANNON))
            );

            if (address(permissionlessDisputeGame) != address(0)) {
                // Deploy and set a new permissionless game to update its prestate
                deployAndSetNewGameImpl({
                    _l2ChainId: l2ChainId,
                    _disputeGame: IDisputeGame(address(permissionlessDisputeGame)),
                    _newAnchorStateRegistryProxy: newAnchorStateRegistryProxy,
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
    /// @param _newAnchorStateRegistryProxy The new anchor state registry proxy
    /// @param _gameType The type of game to deploy
    /// @param _opChainConfig The OP chain configuration
    /// @param _blueprints The blueprint addresses
    /// @param _implementations The implementation addresses
    /// @param _opChainAddrs The OP chain addresses
    function deployAndSetNewGameImpl(
        uint256 _l2ChainId,
        IDisputeGame _disputeGame,
        IAnchorStateRegistry _newAnchorStateRegistryProxy,
        GameType _gameType,
        OpChainConfig memory _opChainConfig,
        Blueprints memory _blueprints,
        Implementations memory _implementations,
        ISystemConfig.Addresses memory _opChainAddrs
    )
    internal
    {
        // independently scoped block to avoid stack too deep
        {
            // Get and upgrade the WETH proxy
            IDelayedWETH delayedWethProxy = getWETH(IFaultDisputeGame(address(_disputeGame)));
            upgradeTo(_opChainConfig.proxyAdmin, address(delayedWethProxy), _implementations.delayedWETHImpl);
        }

        // Get the constructor params for the game
        IFaultDisputeGame.GameConstructorParams memory params =
                        getGameConstructorParams(IFaultDisputeGame(address(_disputeGame)));

        // Modify the params with the new anchorStateRegistry and vm values.
        params.anchorStateRegistry = IAnchorStateRegistry(address(_newAnchorStateRegistryProxy));
        params.vm = IBigStepper(_implementations.mipsImpl);
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
