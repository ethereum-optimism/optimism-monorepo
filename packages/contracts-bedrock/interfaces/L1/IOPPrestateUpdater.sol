// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

// Libraries
import { GameType } from "src/dispute/lib/Types.sol";

// Interfaces
import { IProxyAdmin } from "interfaces/universal/IProxyAdmin.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { IProtocolVersions } from "interfaces/L1/IProtocolVersions.sol";
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";
import { IOPContractsManager } from "interfaces/L1/IOPContractsManager.sol";
import { IDisputeGame } from "interfaces/dispute/IDisputeGame.sol";

interface IOPPrestateUpdater {
    // -------- Constants and Variables --------

    function version() external pure returns (string memory);

    /// @notice Address of the SuperchainConfig contract shared by all chains.
    function superchainConfig() external view returns (ISuperchainConfig);

    /// @notice Address of the ProtocolVersions contract shared by all chains.
    function protocolVersions() external view returns (IProtocolVersions);

    /// @notice Address of the ProxyAdmin contract shared by all chains.
    function superchainProxyAdmin() external view returns (IProxyAdmin);

    /// @notice L1 smart contracts release deployed by this version of OPCM. This is used in opcm to signal which
    /// version of the L1 smart contracts is deployed. It takes the format of `op-contracts/vX.Y.Z`.
    function l1ContractsRelease() external view returns (string memory);

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
    /// @param gameType Type of the game being added
    /// @param newDisputeGame Address of the deployed dispute game
    /// @param oldDisputeGame Address of the old dispute game
    event GameTypeAdded(uint256 indexed l2ChainId, GameType indexed gameType, IDisputeGame newDisputeGame, IDisputeGame oldDisputeGame);

    // -------- Errors --------

    error BytesArrayTooLong();
    error DeploymentFailed();
    error EmptyInitcode();
    error IdentityPrecompileCallFailed();
    error NotABlueprint();
    error ReservedBitsSet();
    error UnexpectedPreambleData(bytes data);
    error UnsupportedERCVersion(uint8 version);
    error OnlyUpgradeController();
    error PrestateNotSet();

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

    error SuperchainProxyAdminMismatch();

        /// @notice Thrown when a function from the parent (OPCM) is not implemented.
    error NotImplemented();

    /// @notice Thrown when the prestate of a permissioned disputed game is 0.
    error PrestateRequired();

    // -------- Methods --------

    function __constructor__(
        ISuperchainConfig _superchainConfig,
        IProtocolVersions _protocolVersions,
        IOPContractsManager.Blueprints memory _blueprints
    )
    external;

    function deploy(IOPContractsManager.DeployInput calldata _input) external returns (IOPContractsManager.DeployOutput memory);

    /// @notice Upgrades the implementation of all proxies in the specified chains
    /// @param _opChainConfigs The chains to upgrade
    function upgrade(IOPContractsManager.OpChainConfig[] memory _opChainConfigs) external;

    /// @notice addGameType deploys a new dispute game and links it to the DisputeGameFactory. The inputted _gameConfigs
    /// must be added in ascending GameType order.
    function addGameType(IOPContractsManager.AddGameInput[] memory _gameConfigs) external returns (IOPContractsManager.AddGameOutput[] memory);

    /// @notice Maps an L2 chain ID to an L1 batch inbox address as defined by the standard
    /// configuration's convention. This convention is `versionByte || keccak256(bytes32(chainId))[:19]`,
    /// where || denotes concatenation`, versionByte is 0x00, and chainId is a uint256.
    /// https://specs.optimism.io/protocol/configurability.html#consensus-parameters
    function chainIdToBatchInboxAddress(uint256 _l2ChainId) external pure returns (address);

    /// @notice Returns the blueprint contract addresses.
    function blueprints() external view returns (IOPContractsManager.Blueprints memory);

    /// @notice Returns the implementation contract addresses.
    function implementations() external view returns (IOPContractsManager.Implementations memory);

    function upgradeController() external view returns (address);

    function isRC() external view returns (bool);

    function setRC(bool _isRC) external;

    function updatePrestate(IOPContractsManager.OpChainConfig[] memory _prestateUpdateInputs) external;
}
