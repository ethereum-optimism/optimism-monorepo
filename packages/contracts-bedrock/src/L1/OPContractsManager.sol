// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Libraries
import { Blueprint } from "src/libraries/Blueprint.sol";
import { Constants } from "src/libraries/Constants.sol";
import { Bytes } from "src/libraries/Bytes.sol";
import { Claim, Hash, Duration, GameType, GameTypes, OutputRoot } from "src/dispute/lib/Types.sol";
import { Strings } from "@openzeppelin/contracts/utils/Strings.sol";

// Interfaces
import { ISemver } from "interfaces/universal/ISemver.sol";
import { IResourceMetering } from "interfaces/L1/IResourceMetering.sol";
import { IBigStepper } from "interfaces/dispute/IBigStepper.sol";
import { IDelayedWETH } from "interfaces/dispute/IDelayedWETH.sol";
import { IAnchorStateRegistry } from "interfaces/dispute/IAnchorStateRegistry.sol";
import { IDisputeGame } from "interfaces/dispute/IDisputeGame.sol";
import { IAddressManager } from "interfaces/legacy/IAddressManager.sol";
import { IProxyAdmin } from "interfaces/universal/IProxyAdmin.sol";
import { IDelayedWETH } from "interfaces/dispute/IDelayedWETH.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { IFaultDisputeGame } from "interfaces/dispute/IFaultDisputeGame.sol";
import { IPermissionedDisputeGame } from "interfaces/dispute/IPermissionedDisputeGame.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { IProtocolVersions } from "interfaces/L1/IProtocolVersions.sol";
import { IOptimismPortal2 } from "interfaces/L1/IOptimismPortal2.sol";
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";
import { IL1CrossDomainMessenger } from "interfaces/L1/IL1CrossDomainMessenger.sol";
import { IL1ERC721Bridge } from "interfaces/L1/IL1ERC721Bridge.sol";
import { IL1StandardBridge } from "interfaces/L1/IL1StandardBridge.sol";
import { IOptimismMintableERC20Factory } from "interfaces/universal/IOptimismMintableERC20Factory.sol";
import { IHasSuperchainConfig } from "interfaces/L1/IHasSuperchainConfig.sol";

contract OPContractsManager is ISemver {
    // -------- Structs --------

    /// @notice Represents the roles that can be set when deploying a standard OP Stack chain.
    struct Roles {
        address opChainProxyAdminOwner;
        address systemConfigOwner;
        address batcher;
        address unsafeBlockSigner;
        address proposer;
        address challenger;
    }

    /// @notice The full set of inputs to deploy a new OP Stack chain.
    struct DeployInput {
        Roles roles;
        uint32 basefeeScalar;
        uint32 blobBasefeeScalar;
        uint256 l2ChainId;
        // The correct type is OutputRoot memory but OP Deployer does not yet support structs.
        bytes startingAnchorRoot;
        // The salt mixer is used as part of making the resulting salt unique.
        string saltMixer;
        uint64 gasLimit;
        // Configurable dispute game parameters.
        GameType disputeGameType;
        Claim disputeAbsolutePrestate;
        uint256 disputeMaxGameDepth;
        uint256 disputeSplitDepth;
        Duration disputeClockExtension;
        Duration disputeMaxClockDuration;
    }

    /// @notice The full set of outputs from deploying a new OP Stack chain.
    struct DeployOutput {
        IProxyAdmin opChainProxyAdmin;
        IAddressManager addressManager;
        IL1ERC721Bridge l1ERC721BridgeProxy;
        ISystemConfig systemConfigProxy;
        IOptimismMintableERC20Factory optimismMintableERC20FactoryProxy;
        IL1StandardBridge l1StandardBridgeProxy;
        IL1CrossDomainMessenger l1CrossDomainMessengerProxy;
        // Fault proof contracts below.
        IOptimismPortal2 optimismPortalProxy;
        IDisputeGameFactory disputeGameFactoryProxy;
        IAnchorStateRegistry anchorStateRegistryProxy;
        IFaultDisputeGame faultDisputeGame;
        IPermissionedDisputeGame permissionedDisputeGame;
        IDelayedWETH delayedWETHPermissionedGameProxy;
        IDelayedWETH delayedWETHPermissionlessGameProxy;
    }

    /// @notice Addresses of ERC-5202 Blueprint contracts. There are used for deploying full size
    /// contracts, to reduce the code size of this factory contract. If it deployed full contracts
    /// using the `new Proxy()` syntax, the code size would get large fast, since this contract would
    /// contain the bytecode of every contract it deploys. Therefore we instead use Blueprints to
    /// reduce the code size of this contract.
    struct Blueprints {
        address addressManager;
        address proxy;
        address proxyAdmin;
        address l1ChugSplashProxy;
        address resolvedDelegateProxy;
        address permissionedDisputeGame1;
        address permissionedDisputeGame2;
        address permissionlessDisputeGame1;
        address permissionlessDisputeGame2;
    }

    /// @notice The latest implementation contracts for the OP Stack.
    struct Implementations {
        address superchainConfigImpl;
        address protocolVersionsImpl;
        address l1ERC721BridgeImpl;
        address optimismPortalImpl;
        address systemConfigImpl;
        address optimismMintableERC20FactoryImpl;
        address l1CrossDomainMessengerImpl;
        address l1StandardBridgeImpl;
        address disputeGameFactoryImpl;
        address anchorStateRegistryImpl;
        address delayedWETHImpl;
        address mipsImpl;
    }

    /// @notice The input required to identify a chain for upgrading, along with new prestate hashes
    struct OpChainConfig {
        ISystemConfig systemConfigProxy;
        IProxyAdmin proxyAdmin;
        Claim absolutePrestate;
    }

    struct AddGameInput {
        string saltMixer;
        ISystemConfig systemConfig;
        IProxyAdmin proxyAdmin;
        IDelayedWETH delayedWETH;
        GameType disputeGameType;
        Claim disputeAbsolutePrestate;
        uint256 disputeMaxGameDepth;
        uint256 disputeSplitDepth;
        Duration disputeClockExtension;
        Duration disputeMaxClockDuration;
        uint256 initialBond;
        IBigStepper vm;
        bool permissioned;
    }

    struct AddGameOutput {
        IDelayedWETH delayedWETH;
        IFaultDisputeGame faultDisputeGame;
    }

    // -------- Constants and Variables --------

    /// @custom:semver 1.6.0
    function version() public pure virtual returns (string memory) {
        return "1.6.0";
    }

    /// @notice Address of the SuperchainConfig contract shared by all chains.
    ISuperchainConfig public immutable superchainConfig;

    /// @notice Address of the ProtocolVersions contract shared by all chains.
    IProtocolVersions public immutable protocolVersions;

    /// @notice Address of the SuperchainProxyAdmin contract shared by all chains.
    IProxyAdmin public immutable superchainProxyAdmin;

    /// @notice L1 smart contracts release deployed by this version of OPCM. This is used in opcm to signal which
    /// version of the L1 smart contracts is deployed. It takes the format of `op-contracts/vX.Y.Z`.
    string internal L1_CONTRACTS_RELEASE;

    /// @notice Addresses of the Blueprint contracts.
    /// This is internal because if public the autogenerated getter method would return a tuple of
    /// addresses, but we want it to return a struct.
    Blueprints internal blueprint;

    /// @notice Addresses of the latest implementation contracts.
    Implementations internal implementation;

    /// @notice The OPContractsManager contract that is currently being used. This is needed in the upgrade function
    /// which is intended to be DELEGATECALLed.
    OPContractsManager internal immutable thisOPCM;

    /// @notice The address of the upgrade controller.
    address public immutable upgradeController;

    /// @notice Whether this is a release candidate.
    bool public isRC = true;

    /// @notice Returns the release string. Appends "-rc" if this is a release candidate.
    function l1ContractsRelease() external view virtual returns (string memory) {
        return isRC ? string.concat(L1_CONTRACTS_RELEASE, "-rc") : L1_CONTRACTS_RELEASE;
    }

    // -------- Events --------

    /// @notice Emitted when a new OP Stack chain is deployed.
    /// @param l2ChainId Chain ID of the new chain.
    /// @param deployer Address that deployed the chain.
    /// @param deployOutput ABI-encoded output of the deployment.
    event Deployed(uint256 indexed l2ChainId, address indexed deployer, bytes deployOutput);

    /// @notice Emitted when a chain is upgraded
    /// @param systemConfig Address of the chain's SystemConfig contract
    /// @param upgrader Address that initiated the upgrade
    event Upgraded(uint256 indexed l2ChainId, ISystemConfig indexed systemConfig, address indexed upgrader);

    /// @notice Emitted when a new game type is added to a chain
    /// @param l2ChainId Chain ID of the chain
    /// @param gameType Type of the game being
    /// @param newDisputeGame Address of the deployed dispute game
    /// @param oldDisputeGame Address of the old dispute game
    event GameTypeAdded(
        uint256 indexed l2ChainId, GameType indexed gameType, IDisputeGame newDisputeGame, IDisputeGame oldDisputeGame
    );

    // -------- Errors --------

    /// @notice Thrown when an address other than the upgrade controller calls the setRC function.
    error OnlyUpgradeController();

    /// @notice Thrown when an address is the zero address.
    error AddressNotFound(address who);

    /// @notice Throw when a contract address has no code.
    error AddressHasNoCode(address who);

    /// @notice Thrown when a release version is already set.
    error AlreadyReleased();

    /// @notice Thrown when an invalid `l2ChainId` is provided to `deploy`.
    error InvalidChainId();

    /// @notice Thrown when a role's address is not valid.
    error InvalidRoleAddress(string role);

    /// @notice Thrown when the latest release is not set upon initialization.
    error LatestReleaseNotSet();

    /// @notice Thrown when the starting anchor root is not provided.
    error InvalidStartingAnchorRoot();

    /// @notice Thrown when certain methods are called outside of a DELEGATECALL.
    error OnlyDelegatecall();

    /// @notice Thrown when game configs passed to addGameType are invalid.
    error InvalidGameConfigs();

    /// @notice Thrown when the SuperchainConfig of the chain does not match the SuperchainConfig of this OPCM.
    error SuperchainConfigMismatch(ISystemConfig systemConfig);

    /// @notice Thrown when the SuperchainProxyAdmin does not match the SuperchainConfig's admin.
    error SuperchainProxyAdminMismatch();

    /// @notice Thrown when a prestate is not set for a game.
    error PrestateNotSet();

    // -------- Methods --------

    constructor(
        ISuperchainConfig _superchainConfig,
        IProtocolVersions _protocolVersions,
        IProxyAdmin _superchainProxyAdmin,
        string memory _l1ContractsRelease,
        Blueprints memory _blueprints,
        Implementations memory _implementations,
        address _upgradeController
    ) {
        assertValidContractAddress(address(_superchainConfig));
        assertValidContractAddress(address(_protocolVersions));
        superchainConfig = _superchainConfig;
        protocolVersions = _protocolVersions;
        superchainProxyAdmin = _superchainProxyAdmin;
        L1_CONTRACTS_RELEASE = _l1ContractsRelease;
        blueprint = _blueprints;
        implementation = _implementations;
        thisOPCM = this;
        upgradeController = _upgradeController;
    }

    function deploy(DeployInput calldata _input) external virtual returns (DeployOutput memory) {
        assertValidInputs(_input);
        uint256 l2ChainId = _input.l2ChainId;
        string memory saltMixer = _input.saltMixer;
        DeployOutput memory output;

        // -------- Deploy Chain Singletons --------

        // The AddressManager is used to store the implementation for the L1CrossDomainMessenger
        // due to it's usage of the legacy ResolvedDelegateProxy.
        output.addressManager = IAddressManager(
            Blueprint.deployFrom(
                blueprint.addressManager, computeSalt(l2ChainId, saltMixer, "AddressManager"), abi.encode()
            )
        );
        // The ProxyAdmin is the owner of all proxies for the chain. We temporarily set the owner to
        // this contract, and then transfer ownership to the specified owner at the end of deployment.
        output.opChainProxyAdmin = IProxyAdmin(
            Blueprint.deployFrom(
                blueprint.proxyAdmin, computeSalt(l2ChainId, saltMixer, "ProxyAdmin"), abi.encode(address(this))
            )
        );
        // Set the AddressManager on the ProxyAdmin.
        output.opChainProxyAdmin.setAddressManager(output.addressManager);
        // Transfer ownership of the AddressManager to the ProxyAdmin.
        transferOwnership(address(output.addressManager), address(output.opChainProxyAdmin));

        // -------- Deploy Proxy Contracts --------

        // Deploy ERC-1967 proxied contracts.
        output.l1ERC721BridgeProxy =
            IL1ERC721Bridge(deployProxy(l2ChainId, output.opChainProxyAdmin, saltMixer, "L1ERC721Bridge"));
        output.optimismPortalProxy =
            IOptimismPortal2(payable(deployProxy(l2ChainId, output.opChainProxyAdmin, saltMixer, "OptimismPortal")));
        output.systemConfigProxy =
            ISystemConfig(deployProxy(l2ChainId, output.opChainProxyAdmin, saltMixer, "SystemConfig"));
        output.optimismMintableERC20FactoryProxy = IOptimismMintableERC20Factory(
            deployProxy(l2ChainId, output.opChainProxyAdmin, saltMixer, "OptimismMintableERC20Factory")
        );
        output.disputeGameFactoryProxy =
            IDisputeGameFactory(deployProxy(l2ChainId, output.opChainProxyAdmin, saltMixer, "DisputeGameFactory"));
        output.anchorStateRegistryProxy =
            IAnchorStateRegistry(deployProxy(l2ChainId, output.opChainProxyAdmin, saltMixer, "AnchorStateRegistry"));

        // Deploy legacy proxied contracts.
        output.l1StandardBridgeProxy = IL1StandardBridge(
            payable(
                Blueprint.deployFrom(
                    blueprint.l1ChugSplashProxy,
                    computeSalt(l2ChainId, saltMixer, "L1StandardBridge"),
                    abi.encode(output.opChainProxyAdmin)
                )
            )
        );
        output.opChainProxyAdmin.setProxyType(address(output.l1StandardBridgeProxy), IProxyAdmin.ProxyType.CHUGSPLASH);
        string memory contractName = "OVM_L1CrossDomainMessenger";
        output.l1CrossDomainMessengerProxy = IL1CrossDomainMessenger(
            Blueprint.deployFrom(
                blueprint.resolvedDelegateProxy,
                computeSalt(l2ChainId, saltMixer, "L1CrossDomainMessenger"),
                abi.encode(output.addressManager, contractName)
            )
        );
        output.opChainProxyAdmin.setProxyType(
            address(output.l1CrossDomainMessengerProxy), IProxyAdmin.ProxyType.RESOLVED
        );
        output.opChainProxyAdmin.setImplementationName(address(output.l1CrossDomainMessengerProxy), contractName);

        // Eventually we will switch from DelayedWETHPermissionedGameProxy to DelayedWETHPermissionlessGameProxy.
        output.delayedWETHPermissionedGameProxy = IDelayedWETH(
            payable(deployProxy(l2ChainId, output.opChainProxyAdmin, saltMixer, "DelayedWETHPermissionedGame"))
        );

        // While not a proxy, we deploy the PermissionedDisputeGame here as well because it's bespoke per chain.
        output.permissionedDisputeGame = IPermissionedDisputeGame(
            Blueprint.deployFrom(
                blueprint.permissionedDisputeGame1,
                blueprint.permissionedDisputeGame2,
                computeSalt(l2ChainId, saltMixer, "PermissionedDisputeGame"),
                encodePermissionedFDGConstructor(
                    IFaultDisputeGame.GameConstructorParams({
                        gameType: _input.disputeGameType,
                        absolutePrestate: _input.disputeAbsolutePrestate,
                        maxGameDepth: _input.disputeMaxGameDepth,
                        splitDepth: _input.disputeSplitDepth,
                        clockExtension: _input.disputeClockExtension,
                        maxClockDuration: _input.disputeMaxClockDuration,
                        vm: IBigStepper(implementation.mipsImpl),
                        weth: IDelayedWETH(payable(address(output.delayedWETHPermissionedGameProxy))),
                        anchorStateRegistry: IAnchorStateRegistry(address(output.anchorStateRegistryProxy)),
                        l2ChainId: _input.l2ChainId
                    }),
                    _input.roles.proposer,
                    _input.roles.challenger
                )
            )
        );

        // -------- Set and Initialize Proxy Implementations --------
        bytes memory data;

        data = encodeL1ERC721BridgeInitializer(output);
        upgradeToAndCall(
            output.opChainProxyAdmin, address(output.l1ERC721BridgeProxy), implementation.l1ERC721BridgeImpl, data
        );

        data = encodeOptimismPortalInitializer(output);
        upgradeToAndCall(
            output.opChainProxyAdmin, address(output.optimismPortalProxy), implementation.optimismPortalImpl, data
        );

        data = encodeSystemConfigInitializer(_input, output);
        upgradeToAndCall(
            output.opChainProxyAdmin, address(output.systemConfigProxy), implementation.systemConfigImpl, data
        );

        data = encodeOptimismMintableERC20FactoryInitializer(output);
        upgradeToAndCall(
            output.opChainProxyAdmin,
            address(output.optimismMintableERC20FactoryProxy),
            implementation.optimismMintableERC20FactoryImpl,
            data
        );

        data = encodeL1CrossDomainMessengerInitializer(output);
        upgradeToAndCall(
            output.opChainProxyAdmin,
            address(output.l1CrossDomainMessengerProxy),
            implementation.l1CrossDomainMessengerImpl,
            data
        );

        data = encodeL1StandardBridgeInitializer(output);
        upgradeToAndCall(
            output.opChainProxyAdmin, address(output.l1StandardBridgeProxy), implementation.l1StandardBridgeImpl, data
        );

        data = encodeDelayedWETHInitializer(_input);
        // Eventually we will switch from DelayedWETHPermissionedGameProxy to DelayedWETHPermissionlessGameProxy.
        upgradeToAndCall(
            output.opChainProxyAdmin,
            address(output.delayedWETHPermissionedGameProxy),
            implementation.delayedWETHImpl,
            data
        );

        // We set the initial owner to this contract, set game implementations, then transfer ownership.
        data = encodeDisputeGameFactoryInitializer();
        upgradeToAndCall(
            output.opChainProxyAdmin,
            address(output.disputeGameFactoryProxy),
            implementation.disputeGameFactoryImpl,
            data
        );
        setDGFImplementation(
            output.disputeGameFactoryProxy,
            GameTypes.PERMISSIONED_CANNON,
            IDisputeGame(address(output.permissionedDisputeGame))
        );

        transferOwnership(address(output.disputeGameFactoryProxy), address(_input.roles.opChainProxyAdminOwner));

        data = encodeAnchorStateRegistryInitializer(_input, output);
        upgradeToAndCall(
            output.opChainProxyAdmin,
            address(output.anchorStateRegistryProxy),
            implementation.anchorStateRegistryImpl,
            data
        );

        // -------- Finalize Deployment --------
        // Transfer ownership of the ProxyAdmin from this contract to the specified owner.
        transferOwnership(address(output.opChainProxyAdmin), _input.roles.opChainProxyAdminOwner);

        emit Deployed(l2ChainId, msg.sender, abi.encode(output));
        return output;
    }

    /// @notice Verifies that all OpChainConfig inputs are valid and reverts if any are invalid.
    function assertValidOpChainConfig(OpChainConfig memory _config) internal view {
        assertValidContractAddress(address(_config.systemConfigProxy));
        assertValidContractAddress(address(_config.proxyAdmin));
    }

    /// @notice Upgrades a set of chains to the latest implementation contracts
    /// @param _opChainConfigs Array of OpChain structs, one per chain to upgrade
    /// @dev This function is intended to be called via DELEGATECALL from the Upgrade Controller Safe
    function upgrade(OpChainConfig[] memory _opChainConfigs) external virtual {
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

    /// @notice addGameType deploys a new dispute game and links it to the DisputeGameFactory. The inputted _gameConfigs
    /// must be added in ascending GameType order.
    function addGameType(AddGameInput[] memory _gameConfigs) public virtual returns (AddGameOutput[] memory) {
        if (address(this) == address(thisOPCM)) revert OnlyDelegatecall();
        if (_gameConfigs.length == 0) revert InvalidGameConfigs();

        AddGameOutput[] memory outputs = new AddGameOutput[](_gameConfigs.length);
        Blueprints memory bps = getBlueprints();

        // Store last game config as an int256 so that we can ensure that the same game config is not added twice.
        // Using int256 generates cheaper, simpler bytecode.
        int256 lastGameConfig = -1;

        for (uint256 i = 0; i < _gameConfigs.length; i++) {
            AddGameInput memory gameConfig = _gameConfigs[i];

            // This conversion is safe because the GameType is a uint32, which will always fit in an int256.
            int256 gameTypeInt = int256(uint256(gameConfig.disputeGameType.raw()));
            // Ensure that the game configs are added in ascending order, and not duplicated.
            if (lastGameConfig >= gameTypeInt) revert InvalidGameConfigs();
            lastGameConfig = gameTypeInt;

            // Grab the permissioned and fault dispute games from the SystemConfig.
            // We keep the FDG type as it reduces casting below.
            IFaultDisputeGame pdg = IFaultDisputeGame(
                address(
                    getGameImplementation(getDisputeGameFactory(gameConfig.systemConfig), GameTypes.PERMISSIONED_CANNON)
                )
            );
            // Pull out the chain ID.
            uint256 l2ChainId = getL2ChainId(pdg);

            // Deploy a new DelayedWETH proxy for this game if one hasn't already been specified. Leaving
            /// gameConfig.delayedWETH as the zero address will cause a new DelayedWETH to be deployed for this game.
            if (address(gameConfig.delayedWETH) == address(0)) {
                string memory contractName = string.concat(
                    "DelayedWETH-",
                    // This is a safe cast because GameType is a uint256 under the hood and no operation has been done
                    // on it at this point
                    Strings.toString(uint256(gameTypeInt))
                );
                outputs[i].delayedWETH = IDelayedWETH(
                    payable(deployProxy(l2ChainId, gameConfig.proxyAdmin, gameConfig.saltMixer, contractName))
                );

                // Initialize the proxy.
                upgradeToAndCall(
                    gameConfig.proxyAdmin,
                    address(outputs[i].delayedWETH),
                    getImplementations().delayedWETHImpl,
                    abi.encodeCall(IDelayedWETH.initialize, (gameConfig.proxyAdmin.owner(), superchainConfig))
                );
            } else {
                outputs[i].delayedWETH = gameConfig.delayedWETH;
            }

            // The FDG is only used for the event below, and only if it is being replaced,
            // so we declare it here, but only assign it below if needed.
            IFaultDisputeGame fdg;

            // The below sections are functionally the same. Both deploy a new dispute game. The dispute game type is
            // either permissioned or permissionless depending on game config.
            if (gameConfig.permissioned) {
                outputs[i].faultDisputeGame = IFaultDisputeGame(
                    Blueprint.deployFrom(
                        bps.permissionedDisputeGame1,
                        bps.permissionedDisputeGame2,
                        computeSalt(l2ChainId, gameConfig.saltMixer, "PermissionedDisputeGame"),
                        encodePermissionedFDGConstructor(
                            IFaultDisputeGame.GameConstructorParams(
                                gameConfig.disputeGameType,
                                gameConfig.disputeAbsolutePrestate,
                                gameConfig.disputeMaxGameDepth,
                                gameConfig.disputeSplitDepth,
                                gameConfig.disputeClockExtension,
                                gameConfig.disputeMaxClockDuration,
                                gameConfig.vm,
                                outputs[i].delayedWETH,
                                getAnchorStateRegistry(pdg),
                                l2ChainId
                            ),
                            getProposer(IPermissionedDisputeGame(address(pdg))),
                            getChallenger(IPermissionedDisputeGame(address(pdg)))
                        )
                    )
                );
            } else {
                fdg = IFaultDisputeGame(
                    address(getGameImplementation(getDisputeGameFactory(gameConfig.systemConfig), GameTypes.CANNON))
                );
                outputs[i].faultDisputeGame = IFaultDisputeGame(
                    Blueprint.deployFrom(
                        bps.permissionlessDisputeGame1,
                        bps.permissionlessDisputeGame2,
                        computeSalt(l2ChainId, gameConfig.saltMixer, "PermissionlessDisputeGame"),
                        encodePermissionlessFDGConstructor(
                            IFaultDisputeGame.GameConstructorParams(
                                gameConfig.disputeGameType,
                                gameConfig.disputeAbsolutePrestate,
                                gameConfig.disputeMaxGameDepth,
                                gameConfig.disputeSplitDepth,
                                gameConfig.disputeClockExtension,
                                gameConfig.disputeMaxClockDuration,
                                gameConfig.vm,
                                outputs[i].delayedWETH,
                                // We can't assume that there is an existing fault dispute game,
                                // so get the Anchor State Registry from the permissioned game.
                                getAnchorStateRegistry(pdg),
                                l2ChainId
                            )
                        )
                    )
                );
            }

            // As a last step, register the new game type with the DisputeGameFactory. If the game type already exists,
            // then its implementation will be overwritten.
            IDisputeGameFactory dgf = getDisputeGameFactory(gameConfig.systemConfig);
            setDGFImplementation(dgf, gameConfig.disputeGameType, IDisputeGame(address(outputs[i].faultDisputeGame)));
            dgf.setInitBond(gameConfig.disputeGameType, gameConfig.initialBond);

            if (gameConfig.permissioned) {
                // Emit event for the newly added game type with the old permissioned dispute game
                emit GameTypeAdded(
                    l2ChainId, gameConfig.disputeGameType, outputs[i].faultDisputeGame, IDisputeGame(address(pdg))
                );
            } else {
                // Emit event for the newly added game type with the old fault dispute game
                emit GameTypeAdded(
                    l2ChainId, gameConfig.disputeGameType, outputs[i].faultDisputeGame, IDisputeGame(address(fdg))
                );
            }
        }

        return outputs;
    }

    // -------- Utilities --------

    /// @notice Verifies that all inputs are valid and reverts if any are invalid.
    /// Typically the proxy admin owner is expected to have code, but this is not enforced here.
    function assertValidInputs(DeployInput calldata _input) internal view {
        if (_input.l2ChainId == 0 || _input.l2ChainId == block.chainid) revert InvalidChainId();

        if (_input.roles.opChainProxyAdminOwner == address(0)) revert InvalidRoleAddress("opChainProxyAdminOwner");
        if (_input.roles.systemConfigOwner == address(0)) revert InvalidRoleAddress("systemConfigOwner");
        if (_input.roles.batcher == address(0)) revert InvalidRoleAddress("batcher");
        if (_input.roles.unsafeBlockSigner == address(0)) revert InvalidRoleAddress("unsafeBlockSigner");
        if (_input.roles.proposer == address(0)) revert InvalidRoleAddress("proposer");
        if (_input.roles.challenger == address(0)) revert InvalidRoleAddress("challenger");

        if (_input.startingAnchorRoot.length == 0) revert InvalidStartingAnchorRoot();
        if (bytes32(_input.startingAnchorRoot) == bytes32(0)) revert InvalidStartingAnchorRoot();
    }

    /// @notice Maps an L2 chain ID to an L1 batch inbox address as defined by the standard
    /// configuration's convention. This convention is `versionByte || keccak256(bytes32(chainId))[:19]`,
    /// where || denotes concatenation`, versionByte is 0x00, and chainId is a uint256.
    /// https://specs.optimism.io/protocol/configurability.html#consensus-parameters
    function chainIdToBatchInboxAddress(uint256 _l2ChainId) public pure returns (address) {
        bytes1 versionByte = 0x00;
        bytes32 hashedChainId = keccak256(bytes.concat(bytes32(_l2ChainId)));
        bytes19 first19Bytes = bytes19(hashedChainId);
        return address(uint160(bytes20(bytes.concat(versionByte, first19Bytes))));
    }

    /// @notice Helper method for computing a salt that's used in CREATE2 deployments.
    /// Including the contract name ensures that the resultant address from CREATE2 is unique
    /// across our smart contract system. For example, we deploy multiple proxy contracts
    /// with the same bytecode from this contract, so they each require a unique salt for determinism.
    function computeSalt(
        uint256 _l2ChainId,
        string memory _saltMixer,
        string memory _contractName
    )
        internal
        pure
        returns (bytes32)
    {
        return keccak256(abi.encode(_l2ChainId, _saltMixer, _contractName));
    }

    /// @notice Helper method for computing a reusable salt mixer
    /// This method should be used as the salt mixer when deploying contracts when there is no user
    /// provided salt mixer. This protects against a situation where multiple chains with the same
    /// L2 chain ID exist, which would otherwise result in address collisions.
    function reusableSaltMixer(OpChainConfig memory _opChainConfig) internal pure returns (string memory) {
        return string(bytes.concat(bytes32(uint256(uint160(address(_opChainConfig.systemConfigProxy))))));
    }

    /// @notice Deterministically deploys a new proxy contract owned by the provided ProxyAdmin.
    /// The salt is computed as a function of the L2 chain ID, the salt mixer and the contract name.
    /// This is required because we deploy many identical proxies, so they each require a unique salt for determinism.
    function deployProxy(
        uint256 _l2ChainId,
        IProxyAdmin _proxyAdmin,
        string memory _saltMixer,
        string memory _contractName
    )
        internal
        returns (address)
    {
        bytes32 salt = computeSalt(_l2ChainId, _saltMixer, _contractName);
        return Blueprint.deployFrom(getBlueprints().proxy, salt, abi.encode(_proxyAdmin));
    }

    // -------- Initializer Encoding --------

    /// @notice Helper method for encoding the L1ERC721Bridge initializer data.
    function encodeL1ERC721BridgeInitializer(DeployOutput memory _output)
        internal
        view
        virtual
        returns (bytes memory)
    {
        return abi.encodeCall(IL1ERC721Bridge.initialize, (_output.l1CrossDomainMessengerProxy, superchainConfig));
    }

    /// @notice Helper method for encoding the OptimismPortal initializer data.
    function encodeOptimismPortalInitializer(DeployOutput memory _output)
        internal
        view
        virtual
        returns (bytes memory)
    {
        return abi.encodeCall(
            IOptimismPortal2.initialize,
            (
                _output.disputeGameFactoryProxy,
                _output.systemConfigProxy,
                superchainConfig,
                GameTypes.PERMISSIONED_CANNON
            )
        );
    }

    /// @notice Helper method for encoding the SystemConfig initializer data.
    function encodeSystemConfigInitializer(
        DeployInput memory _input,
        DeployOutput memory _output
    )
        internal
        view
        virtual
        returns (bytes memory)
    {
        (IResourceMetering.ResourceConfig memory referenceResourceConfig, ISystemConfig.Addresses memory opChainAddrs) =
            defaultSystemConfigParams(_input, _output);

        return abi.encodeCall(
            ISystemConfig.initialize,
            (
                _input.roles.systemConfigOwner,
                _input.basefeeScalar,
                _input.blobBasefeeScalar,
                bytes32(uint256(uint160(_input.roles.batcher))), // batcherHash
                _input.gasLimit,
                _input.roles.unsafeBlockSigner,
                referenceResourceConfig,
                chainIdToBatchInboxAddress(_input.l2ChainId),
                opChainAddrs
            )
        );
    }

    /// @notice Helper method for encoding the OptimismMintableERC20Factory initializer data.
    function encodeOptimismMintableERC20FactoryInitializer(DeployOutput memory _output)
        internal
        pure
        virtual
        returns (bytes memory)
    {
        return abi.encodeCall(IOptimismMintableERC20Factory.initialize, (address(_output.l1StandardBridgeProxy)));
    }

    /// @notice Helper method for encoding the L1CrossDomainMessenger initializer data.
    function encodeL1CrossDomainMessengerInitializer(DeployOutput memory _output)
        internal
        view
        virtual
        returns (bytes memory)
    {
        return abi.encodeCall(IL1CrossDomainMessenger.initialize, (superchainConfig, _output.optimismPortalProxy));
    }

    /// @notice Helper method for encoding the L1StandardBridge initializer data.
    function encodeL1StandardBridgeInitializer(DeployOutput memory _output)
        internal
        view
        virtual
        returns (bytes memory)
    {
        return abi.encodeCall(IL1StandardBridge.initialize, (_output.l1CrossDomainMessengerProxy, superchainConfig));
    }

    function encodeDisputeGameFactoryInitializer() internal view virtual returns (bytes memory) {
        // This contract must be the initial owner so we can set game implementations, then
        // ownership is transferred after.
        return abi.encodeCall(IDisputeGameFactory.initialize, (address(this)));
    }

    function encodeAnchorStateRegistryInitializer(
        DeployInput memory _input,
        DeployOutput memory _output
    )
        internal
        view
        virtual
        returns (bytes memory)
    {
        OutputRoot memory startingAnchorRoot = abi.decode(_input.startingAnchorRoot, (OutputRoot));
        return abi.encodeCall(
            IAnchorStateRegistry.initialize,
            (superchainConfig, _output.disputeGameFactoryProxy, _output.optimismPortalProxy, startingAnchorRoot)
        );
    }

    function encodeDelayedWETHInitializer(DeployInput memory _input) internal view virtual returns (bytes memory) {
        return abi.encodeCall(IDelayedWETH.initialize, (_input.roles.opChainProxyAdminOwner, superchainConfig));
    }

    function encodePermissionlessFDGConstructor(IFaultDisputeGame.GameConstructorParams memory _params)
        internal
        view
        virtual
        returns (bytes memory)
    {
        bytes memory dataWithSelector = abi.encodeCall(IFaultDisputeGame.__constructor__, (_params));
        return Bytes.slice(dataWithSelector, 4);
    }

    function encodePermissionedFDGConstructor(
        IFaultDisputeGame.GameConstructorParams memory _params,
        address _proposer,
        address _challenger
    )
        internal
        view
        virtual
        returns (bytes memory)
    {
        bytes memory dataWithSelector =
            abi.encodeCall(IPermissionedDisputeGame.__constructor__, (_params, _proposer, _challenger));
        return Bytes.slice(dataWithSelector, 4);
    }

    /// @notice Returns default, standard config arguments for the SystemConfig initializer.
    /// This is used by subclasses to reduce code duplication.
    function defaultSystemConfigParams(
        DeployInput memory, /* _input */
        DeployOutput memory _output
    )
        internal
        view
        virtual
        returns (IResourceMetering.ResourceConfig memory resourceConfig_, ISystemConfig.Addresses memory opChainAddrs_)
    {
        resourceConfig_ = Constants.DEFAULT_RESOURCE_CONFIG();

        opChainAddrs_ = ISystemConfig.Addresses({
            l1CrossDomainMessenger: address(_output.l1CrossDomainMessengerProxy),
            l1ERC721Bridge: address(_output.l1ERC721BridgeProxy),
            l1StandardBridge: address(_output.l1StandardBridgeProxy),
            disputeGameFactory: address(_output.disputeGameFactoryProxy),
            optimismPortal: address(_output.optimismPortalProxy),
            optimismMintableERC20Factory: address(_output.optimismMintableERC20FactoryProxy)
        });

        assertValidContractAddress(opChainAddrs_.l1CrossDomainMessenger);
        assertValidContractAddress(opChainAddrs_.l1ERC721Bridge);
        assertValidContractAddress(opChainAddrs_.l1StandardBridge);
        assertValidContractAddress(opChainAddrs_.disputeGameFactory);
        assertValidContractAddress(opChainAddrs_.optimismPortal);
        assertValidContractAddress(opChainAddrs_.optimismMintableERC20Factory);
    }

    /// @notice Makes an external call to the target to initialize the proxy with the specified data.
    /// First performs safety checks to ensure the target, implementation, and proxy admin are valid.
    function upgradeToAndCall(
        IProxyAdmin _proxyAdmin,
        address _target,
        address _implementation,
        bytes memory _data
    )
        internal
    {
        assertValidContractAddress(_implementation);

        _proxyAdmin.upgradeAndCall(payable(address(_target)), _implementation, _data);
    }

    /// @notice Updates the implementation of a proxy without calling the initializer.
    /// First performs safety checks to ensure the target, implementation, and proxy admin are valid.
    function upgradeTo(IProxyAdmin _proxyAdmin, address _target, address _implementation) internal {
        assertValidContractAddress(_implementation);

        _proxyAdmin.upgrade(payable(address(_target)), _implementation);
    }

    function assertValidContractAddress(address _who) internal view {
        if (_who.code.length == 0) revert AddressHasNoCode(_who);
    }

    /// @notice Returns the blueprint contract addresses.
    function blueprints() public view returns (Blueprints memory) {
        return blueprint;
    }

    /// @notice Returns the implementation contract addresses.
    function implementations() public view returns (Implementations memory) {
        return implementation;
    }

    /// @notice Returns the implementation contract address for a given game type.
    function getGameImplementation(
        IDisputeGameFactory _disputeGameFactory,
        GameType _gameType
    )
        internal
        view
        returns (IDisputeGame)
    {
        return _disputeGameFactory.gameImpls(_gameType);
    }

    /// @notice Sets the RC flag.
    function setRC(bool _isRC) external {
        if (msg.sender != upgradeController) revert OnlyUpgradeController();
        isRC = _isRC;
    }

    /// @notice Sets a game implementation on the dispute game factory
    function setDGFImplementation(IDisputeGameFactory _dgf, GameType _gameType, IDisputeGame _newGame) internal {
        _dgf.setImplementation(_gameType, _newGame);
    }

    /// @notice Transfers ownership
    function transferOwnership(address _target, address _newOwner) internal {
        // All transferOwnership targets have the same selector, so we just use IAddressManager
        IAddressManager(_target).transferOwnership(_newOwner);
    }

    /// @notice Retrieves the constructor params for a given game.
    function getGameConstructorParams(IFaultDisputeGame _disputeGame)
        internal
        view
        returns (IFaultDisputeGame.GameConstructorParams memory)
    {
        IFaultDisputeGame.GameConstructorParams memory params = IFaultDisputeGame.GameConstructorParams({
            gameType: _disputeGame.gameType(),
            absolutePrestate: _disputeGame.absolutePrestate(),
            maxGameDepth: _disputeGame.maxGameDepth(),
            splitDepth: _disputeGame.splitDepth(),
            clockExtension: _disputeGame.clockExtension(),
            maxClockDuration: _disputeGame.maxClockDuration(),
            vm: _disputeGame.vm(),
            weth: getWETH(_disputeGame),
            anchorStateRegistry: getAnchorStateRegistry(_disputeGame),
            l2ChainId: getL2ChainId(_disputeGame)
        });
        return params;
    }

    /// @notice Retrieves the Superchain Config for a bridge contract
    function getSuperchainConfig(address _hasSuperchainConfig) internal view returns (ISuperchainConfig) {
        return IHasSuperchainConfig(_hasSuperchainConfig).superchainConfig();
    }

    /// @notice Retrieves the Anchor State Registry for a given game
    function getAnchorStateRegistry(IFaultDisputeGame _disputeGame) internal view returns (IAnchorStateRegistry) {
        return _disputeGame.anchorStateRegistry();
    }

    /// @notice Retrieves the DelayedWETH address for a given game
    function getWETH(IFaultDisputeGame _disputeGame) internal view returns (IDelayedWETH) {
        return _disputeGame.weth();
    }

    /// @notice Retrieves the L2 chain ID for a given game
    function getL2ChainId(IFaultDisputeGame _disputeGame) internal view returns (uint256) {
        return _disputeGame.l2ChainId();
    }

    /// @notice Retrieves the proposer address for a given game
    function getProposer(IPermissionedDisputeGame _disputeGame) internal view returns (address) {
        return _disputeGame.proposer();
    }

    /// @notice Retrieves the challenger address for a given game
    function getChallenger(IPermissionedDisputeGame _disputeGame) internal view returns (address) {
        return _disputeGame.challenger();
    }

    /// @notice Retrieves the DisputeGameFactory address for a given SystemConfig
    function getDisputeGameFactory(ISystemConfig _systemConfig) internal view returns (IDisputeGameFactory) {
        return IDisputeGameFactory(_systemConfig.disputeGameFactory());
    }

    /// @notice Retrieves the implementation addresses stored in this OPCM contract
    function getImplementations() internal view returns (Implementations memory) {
        return thisOPCM.implementations();
    }

    /// @notice Retrieves the blueprint addresses stored in this OPCM contract
    function getBlueprints() internal view returns (Blueprints memory) {
        return thisOPCM.blueprints();
    }

    function getProxyImplementation(IProxyAdmin _proxyAdmin, address _proxy) internal view returns (address) {
        return _proxyAdmin.getProxyImplementation(_proxy);
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
