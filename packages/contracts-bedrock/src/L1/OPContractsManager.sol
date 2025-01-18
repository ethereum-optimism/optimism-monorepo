// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Libraries
import { Blueprint } from "src/libraries/Blueprint.sol";
import { Constants } from "src/libraries/Constants.sol";
import { Bytes } from "src/libraries/Bytes.sol";
import { Claim, Duration, GameType, GameTypes, OutputRoot } from "src/dispute/lib/Types.sol";

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

    /// @notice The input required to identify a chain for upgrading.
    struct OpChain {
        ISystemConfig systemConfigProxy;
        IProxyAdmin proxyAdmin;
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

    /// @custom:semver 1.0.0-beta.33
    function version() public pure virtual returns (string memory) {
        return "1.0.0-beta.33";
    }

    /// @notice Address of the SuperchainConfig contract shared by all chains.
    ISuperchainConfig public immutable superchainConfig;

    /// @notice Address of the ProtocolVersions contract shared by all chains.
    IProtocolVersions public immutable protocolVersions;

    /// @notice L1 smart contracts release deployed by this version of OPCM. This is used in opcm to signal which
    /// version of the L1 smart contracts is deployed. It takes the format of `op-contracts/vX.Y.Z`.
    string public l1ContractsRelease;

    /// @notice Addresses of the Blueprint contracts.
    /// This is internal because if public the autogenerated getter method would return a tuple of
    /// addresses, but we want it to return a struct.
    Blueprints internal blueprint;

    /// @notice Addresses of the latest implementation contracts.
    Implementations internal implementation;

    /// @notice The OPContractsManager contract that is currently being used. This is needed in the upgrade function
    /// which is intended to be DELEGATECALLed.
    OPContractsManager internal immutable thisOPCM;

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

    // -------- Errors --------

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

    // -------- Methods --------

    constructor(
        ISuperchainConfig _superchainConfig,
        IProtocolVersions _protocolVersions,
        string memory _l1ContractsRelease,
        Blueprints memory _blueprints,
        Implementations memory _implementations
    ) {
        assertValidContractAddress(address(_superchainConfig));
        assertValidContractAddress(address(_protocolVersions));
        superchainConfig = _superchainConfig;
        protocolVersions = _protocolVersions;
        l1ContractsRelease = _l1ContractsRelease;

        blueprint = _blueprints;
        implementation = _implementations;
        thisOPCM = this;
    }

    function deploy(DeployInput calldata _input) external returns (DeployOutput memory) {
        assertValidInputs(_input);
        uint256 l2ChainId = _input.l2ChainId;
        string memory saltMixer = _input.saltMixer;
        DeployOutput memory output;

        // -------- Deploy Chain Singletons --------

        // The ProxyAdmin is the owner of all proxies for the chain. We temporarily set the owner to
        // this contract, and then transfer ownership to the specified owner at the end of deployment.
        // The AddressManager is used to store the implementation for the L1CrossDomainMessenger
        // due to it's usage of the legacy ResolvedDelegateProxy.
        output.addressManager = IAddressManager(
            Blueprint.deployFrom(
                blueprint.addressManager, computeSalt(l2ChainId, saltMixer, "AddressManager"), abi.encode()
            )
        );
        output.opChainProxyAdmin = IProxyAdmin(
            Blueprint.deployFrom(
                blueprint.proxyAdmin, computeSalt(l2ChainId, saltMixer, "ProxyAdmin"), abi.encode(address(this))
            )
        );
        output.opChainProxyAdmin.setAddressManager(output.addressManager);

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
        // Now that all proxies are deployed, we can transfer ownership of the AddressManager to the ProxyAdmin.
        output.addressManager.transferOwnership(address(output.opChainProxyAdmin));

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

        // First we upgrade the implementation so it's version can be retrieved, then we initialize
        // it afterwards. See the comments in encodeSystemConfigInitializer to learn more.
        upgradeTo(output.opChainProxyAdmin, payable(address(output.systemConfigProxy)), implementation.systemConfigImpl);
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
        output.disputeGameFactoryProxy.setImplementation(
            GameTypes.PERMISSIONED_CANNON, IDisputeGame(address(output.permissionedDisputeGame))
        );
        output.disputeGameFactoryProxy.transferOwnership(address(_input.roles.opChainProxyAdminOwner));

        data = encodeAnchorStateRegistryInitializer(_input, output);
        upgradeToAndCall(
            output.opChainProxyAdmin,
            address(output.anchorStateRegistryProxy),
            implementation.anchorStateRegistryImpl,
            data
        );

        // -------- Finalize Deployment --------
        // Transfer ownership of the ProxyAdmin from this contract to the specified owner.
        output.opChainProxyAdmin.transferOwnership(_input.roles.opChainProxyAdminOwner);

        emit Deployed(l2ChainId, msg.sender, abi.encode(output));
        return output;
    }

    /// @notice Upgrades a set of chains to the latest implementation contracts
    /// @param _opChains Array of OpChain structs, one per chain to upgrade
    /// @dev This function is intended to be called via DELEGATECALL from the Upgrade Controller Safe
    function upgrade(OpChain[] memory _opChains) external {
        if (address(this) == address(thisOPCM)) revert OnlyDelegatecall();

        Implementations memory impls = thisOPCM.implementations();

        // TODO: upgrading the SuperchainConfig and ProtocolVersions (in a new function)

        for (uint256 i = 0; i < _opChains.length; i++) {
            ISystemConfig systemConfig = _opChains[i].systemConfigProxy;
            // After Upgrade 12, we will be able to use systemConfigProxy.getAddresses() here.
            ISystemConfig.Addresses memory opChainAddrs = ISystemConfig.Addresses({
                l1CrossDomainMessenger: systemConfig.l1CrossDomainMessenger(),
                l1ERC721Bridge: systemConfig.l1ERC721Bridge(),
                l1StandardBridge: systemConfig.l1StandardBridge(),
                disputeGameFactory: systemConfig.disputeGameFactory(),
                optimismPortal: systemConfig.optimismPortal(),
                optimismMintableERC20Factory: systemConfig.optimismMintableERC20Factory()
            });

            if (IOptimismPortal2(payable(opChainAddrs.optimismPortal)).superchainConfig() != superchainConfig) {
                revert SuperchainConfigMismatch(systemConfig);
            }

            IProxyAdmin proxyAdmin = _opChains[i].proxyAdmin;

            // -------- Upgrade Contracts Stored in SystemConfig --------
            upgradeTo(proxyAdmin, address(systemConfig), impls.systemConfigImpl);
            upgradeTo(proxyAdmin, opChainAddrs.l1CrossDomainMessenger, impls.l1CrossDomainMessengerImpl);
            upgradeTo(proxyAdmin, opChainAddrs.l1ERC721Bridge, impls.l1ERC721BridgeImpl);
            upgradeTo(proxyAdmin, opChainAddrs.l1StandardBridge, impls.l1StandardBridgeImpl);
            upgradeTo(proxyAdmin, opChainAddrs.disputeGameFactory, impls.disputeGameFactoryImpl);
            upgradeTo(proxyAdmin, opChainAddrs.optimismPortal, impls.optimismPortalImpl);
            upgradeTo(proxyAdmin, opChainAddrs.optimismMintableERC20Factory, impls.optimismMintableERC20FactoryImpl);

            // -------- Discover and Upgrade Proofs Contracts --------
            // Starting with the permissioned game, permissioned weth, and anchor state registry, which all chains have.
            IPermissionedDisputeGame permissionedDisputeGame = IPermissionedDisputeGame(
                address(
                    getGameImplementation(
                        IDisputeGameFactory(opChainAddrs.disputeGameFactory), GameTypes.PERMISSIONED_CANNON
                    )
                )
            );
            IDelayedWETH delayedWETHPermissionedGameProxy = permissionedDisputeGame.weth();
            IAnchorStateRegistry anchorStateRegistryProxy = permissionedDisputeGame.anchorStateRegistry();
            upgradeTo(proxyAdmin, address(anchorStateRegistryProxy), impls.anchorStateRegistryImpl);
            upgradeTo(proxyAdmin, address(delayedWETHPermissionedGameProxy), impls.delayedWETHImpl);
            // TODO: redeploy and replace permissioned game implementation

            // Now retrieve the permissionless game. If it exists, upgrade its weth and replace its implementation.
            IFaultDisputeGame faultDisputeGame = IFaultDisputeGame(
                address(getGameImplementation(IDisputeGameFactory(opChainAddrs.disputeGameFactory), GameTypes.CANNON))
            );
            if (address(faultDisputeGame) != address(0)) {
                IDelayedWETH delayedWETHPermissionlessGameProxy = faultDisputeGame.weth();
                upgradeTo(proxyAdmin, address(delayedWETHPermissionlessGameProxy), impls.delayedWETHImpl);
                // TODO: redeploy and replace permissionless game implementation
            }

            // Emit the upgraded event with the address of the caller. Since this will be a delegatecall,
            // the caller will be the value of the ADDRESS opcode.
            uint256 l2ChainId = permissionedDisputeGame.l2ChainId();
            emit Upgraded(l2ChainId, systemConfig, address(this));
        }
    }

    /// @notice addGameType deploys a new dispute game and links it to the DisputeGameFactory. The inputted _gameConfigs
    /// must be added in ascending GameType order.
    function addGameType(AddGameInput[] memory _gameConfigs) external returns (AddGameOutput[] memory) {
        if (address(this) == address(thisOPCM)) revert OnlyDelegatecall();
        if (_gameConfigs.length == 0) revert InvalidGameConfigs();

        AddGameOutput[] memory outputs = new AddGameOutput[](_gameConfigs.length);
        Blueprints memory bps = thisOPCM.blueprints();

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

            // Grab the FDG from the SystemConfig.
            IFaultDisputeGame fdg = IFaultDisputeGame(
                address(
                    getGameImplementation(
                        IDisputeGameFactory(gameConfig.systemConfig.disputeGameFactory()), GameTypes.PERMISSIONED_CANNON
                    )
                )
            );
            // Pull out the chain ID.
            uint256 l2ChainId = fdg.l2ChainId();

            // Deploy a new DelayedWETH proxy for this game if one hasn't already been specified. Leaving
            /// gameConfig.delayedWETH as the zero address will cause a new DelayedWETH to be deployed for this game.
            if (address(gameConfig.delayedWETH) == address(0)) {
                outputs[i].delayedWETH = IDelayedWETH(
                    payable(deployProxy(l2ChainId, gameConfig.proxyAdmin, gameConfig.saltMixer, "DelayedWETH"))
                );

                // Initialize the proxy.
                upgradeToAndCall(
                    gameConfig.proxyAdmin,
                    address(outputs[i].delayedWETH),
                    thisOPCM.implementations().delayedWETHImpl,
                    abi.encodeCall(IDelayedWETH.initialize, (gameConfig.proxyAdmin.owner(), superchainConfig))
                );
            } else {
                outputs[i].delayedWETH = gameConfig.delayedWETH;
            }

            // The below sections are functionally the same. Both deploy a new dispute game. The dispute game type is
            // either permissioned or permissionless depending on game config.
            if (gameConfig.permissioned) {
                IPermissionedDisputeGame pdg = IPermissionedDisputeGame(address(fdg));
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
                                pdg.anchorStateRegistry(),
                                l2ChainId
                            ),
                            pdg.proposer(),
                            pdg.challenger()
                        )
                    )
                );
            } else {
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
                                fdg.anchorStateRegistry(),
                                l2ChainId
                            )
                        )
                    )
                );
            }

            // As a last step, register the new game type with the DisputeGameFactory. If the game type already exists,
            // then its implementation will be overwritten.
            IDisputeGameFactory dgf = IDisputeGameFactory(gameConfig.systemConfig.disputeGameFactory());
            dgf.setImplementation(gameConfig.disputeGameType, IDisputeGame(address(outputs[i].faultDisputeGame)));
            dgf.setInitBond(gameConfig.disputeGameType, gameConfig.initialBond);
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
        return Blueprint.deployFrom(thisOPCM.blueprints().proxy, salt, abi.encode(_proxyAdmin));
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
        // We use assembly to easily convert from IResourceMetering.ResourceConfig to ResourceMetering.ResourceConfig.
        // This is required because we have not yet fully migrated the codebase to be interface-based.
        IResourceMetering.ResourceConfig memory resourceConfig = Constants.DEFAULT_RESOURCE_CONFIG();
        assembly ("memory-safe") {
            resourceConfig_ := resourceConfig
        }

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
}
