// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Libraries
import { GameType, Claim, GameTypes, Hash } from "src/dispute/lib/Types.sol";
import { Duration } from "src/dispute/lib/LibUDT.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";
import { Constants } from "src/libraries/Constants.sol";

// Interfaces
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";
import { IAnchorStateRegistry } from "interfaces/dispute/IAnchorStateRegistry.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { IL1CrossDomainMessenger } from "interfaces/L1/IL1CrossDomainMessenger.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { IOptimismMintableERC20Factory } from "interfaces/universal/IOptimismMintableERC20Factory.sol";
import { IL1StandardBridge } from "interfaces/L1/IL1StandardBridge.sol";
import { IL1ERC721Bridge } from "interfaces/L1/IL1ERC721Bridge.sol";
import { IPermissionedDisputeGame } from "interfaces/dispute/IPermissionedDisputeGame.sol";
import { IProxyAdmin } from "interfaces/universal/IProxyAdmin.sol";
import { IDelayedWETH } from "interfaces/dispute/IDelayedWETH.sol";
import { ISemver } from "interfaces/universal/ISemver.sol";
import { IResourceMetering } from "interfaces/L1/IResourceMetering.sol";
import { IOptimismPortal2 } from "interfaces/L1/IOptimismPortal2.sol";
import { IPreimageOracle } from "interfaces/cannon/IPreimageOracle.sol";
import { IProtocolVersions } from "interfaces/L1/IProtocolVersions.sol";

contract StandardValidatorBase {
    ISuperchainConfig public superchainConfig;
    IProtocolVersions public protocolVersions;
    address public l1PAOMultisig;
    address public mips;
    address public challenger;

    // Implementation addresses as state variables
    address public superchainConfigImpl;
    address public protocolVersionsImpl;
    address public l1ERC721BridgeImpl;
    address public optimismPortalImpl;
    address public systemConfigImpl;
    address public optimismMintableERC20FactoryImpl;
    address public l1CrossDomainMessengerImpl;
    address public l1StandardBridgeImpl;
    address public disputeGameFactoryImpl;
    address public anchorStateRegistryImpl;
    address public delayedWETHImpl;
    address public mipsImpl;

    struct ImplementationsBase {
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

    constructor(
        ImplementationsBase memory _implementations,
        ISuperchainConfig _superchainConfig,
        IProtocolVersions _protocolVersions,
        address _l1PAOMultisig,
        address _mips,
        address _challenger
    ) {
        superchainConfig = _superchainConfig;
        protocolVersions = _protocolVersions;
        l1PAOMultisig = _l1PAOMultisig;
        mips = _mips;
        challenger = _challenger;

        // Set implementation addresses from struct
        superchainConfigImpl = _implementations.superchainConfigImpl;
        protocolVersionsImpl = _implementations.protocolVersionsImpl;
        l1ERC721BridgeImpl = _implementations.l1ERC721BridgeImpl;
        optimismPortalImpl = _implementations.optimismPortalImpl;
        systemConfigImpl = _implementations.systemConfigImpl;
        optimismMintableERC20FactoryImpl = _implementations.optimismMintableERC20FactoryImpl;
        l1CrossDomainMessengerImpl = _implementations.l1CrossDomainMessengerImpl;
        l1StandardBridgeImpl = _implementations.l1StandardBridgeImpl;
        disputeGameFactoryImpl = _implementations.disputeGameFactoryImpl;
        anchorStateRegistryImpl = _implementations.anchorStateRegistryImpl;
        delayedWETHImpl = _implementations.delayedWETHImpl;
        mipsImpl = _implementations.mipsImpl;
    }

    function validate(
        string memory _errors,
        ISystemConfig _sysCfg,
        IProxyAdmin _admin,
        bytes32 _absolutePrestate,
        uint256 _l2ChainID
    )
        internal
        view
        returns (string memory)
    {
        _errors = assertValidSuperchainConfig(_errors, _admin);
        _errors = assertValidProtocolVersions(_errors, _admin);
        _errors = assertValidProxyAdmin(_errors, _admin);
        _errors = assertValidSystemConfig(_errors, _sysCfg, _admin);
        _errors = assertValidL1CrossDomainMessenger(_errors, _sysCfg, _admin);
        _errors = assertValidL1StandardBridge(_errors, _sysCfg, _admin);
        _errors = assertValidOptimismMintableERC20Factory(_errors, _sysCfg, _admin);
        _errors = assertValidL1ERC721Bridge(_errors, _sysCfg, _admin);
        _errors = assertValidOptimismPortal(_errors, _sysCfg, _admin);
        _errors = assertValidDisputeGameFactory(_errors, _sysCfg, _admin);
        _errors = assertValidPermissionedDisputeGame(_errors, _sysCfg, _absolutePrestate, _l2ChainID, _admin);
        _errors = assertValidPermissionlessDisputeGame(_errors, _sysCfg, _absolutePrestate, _l2ChainID, _admin);
        return _errors;
    }

    function superchainConfigVersion() public pure virtual returns (string memory) {
        return "1.1.0";
    }

    function protocolVersionsVersion() public pure virtual returns (string memory) {
        return "1.0.0";
    }

    function l1ERC721BridgeVersion() public pure virtual returns (string memory) {
        return "2.1.0";
    }

    function optimismPortalVersion() public pure virtual returns (string memory) {
        return "3.10.0";
    }

    function systemConfigVersion() public pure virtual returns (string memory) {
        return "2.3.0";
    }

    function optimismMintableERC20FactoryVersion() public pure virtual returns (string memory) {
        return "1.9.0";
    }

    function l1CrossDomainMessengerVersion() public pure virtual returns (string memory) {
        return "2.3.0";
    }

    function l1StandardBridgeVersion() public pure virtual returns (string memory) {
        return "2.1.0";
    }

    function disputeGameFactoryVersion() public pure virtual returns (string memory) {
        return "1.0.0";
    }

    function anchorStateRegistryVersion() public pure virtual returns (string memory) {
        return "2.0.0";
    }

    function delayedWETHVersion() public pure virtual returns (string memory) {
        return "1.1.0";
    }

    function mipsVersion() public pure virtual returns (string memory) {
        return "1.2.1";
    }

    function permissionedDisputeGameVersion() public pure virtual returns (string memory) {
        return "1.3.1";
    }

    function preimageOracleVersion() public pure virtual returns (string memory) {
        return "1.1.2";
    }

    function assertValidSuperchainConfig(
        string memory _errors,
        IProxyAdmin _admin
    )
        internal
        view
        returns (string memory)
    {
        _errors = internalRequire(stringEq(superchainConfig.version(), superchainConfigVersion()), "SPRCFG-10", _errors);
        _errors = internalRequire(
            _admin.getProxyImplementation(address(superchainConfig)) == superchainConfigImpl, "SPRCFG-20", _errors
        );
        _errors = internalRequire(!superchainConfig.paused(), "SPRCFG-30", _errors);
        return _errors;
    }

    function assertValidProtocolVersions(
        string memory _errors,
        IProxyAdmin _admin
    )
        internal
        view
        returns (string memory)
    {
        _errors = internalRequire(stringEq(protocolVersions.version(), protocolVersionsVersion()), "PVER-10", _errors);
        _errors = internalRequire(
            _admin.getProxyImplementation(address(protocolVersions)) == protocolVersionsImpl, "PVER-20", _errors
        );

        return _errors;
    }

    function assertValidProxyAdmin(string memory _errors, IProxyAdmin _admin) internal view returns (string memory) {
        _errors = internalRequire(_admin.owner() == l1PAOMultisig, "PROXYA-10", _errors);
        return _errors;
    }

    function assertValidSystemConfig(
        string memory _errors,
        ISystemConfig _sysCfg,
        IProxyAdmin _admin
    )
        internal
        view
        returns (string memory)
    {
        ISemver _semver = ISemver(address(_sysCfg));
        _errors = internalRequire(stringEq(_semver.version(), systemConfigVersion()), "SYSCON-10", _errors);
        _errors = internalRequire(_sysCfg.gasLimit() == uint64(60_000_000), "SYSCON-20", _errors);
        _errors = internalRequire(_sysCfg.scalar() >> 248 == 1, "SYSCON-30", _errors);
        _errors =
            internalRequire(_admin.getProxyImplementation(address(_sysCfg)) == systemConfigImpl, "SYSCON-40", _errors);

        IResourceMetering.ResourceConfig memory outputConfig = _sysCfg.resourceConfig();
        _errors = internalRequire(outputConfig.maxResourceLimit == 20_000_000, "SYSCON-50", _errors);
        _errors = internalRequire(outputConfig.elasticityMultiplier == 10, "SYSCON-60", _errors);
        _errors = internalRequire(outputConfig.baseFeeMaxChangeDenominator == 8, "SYSCON-70", _errors);
        _errors = internalRequire(outputConfig.systemTxMaxGas == 1_000_000, "SYSCON-80", _errors);
        _errors = internalRequire(outputConfig.minimumBaseFee == 1 gwei, "SYSCON-90", _errors);
        _errors = internalRequire(outputConfig.maximumBaseFee == type(uint128).max, "SYSCON-100", _errors);
        return _errors;
    }

    function assertValidL1CrossDomainMessenger(
        string memory _errors,
        ISystemConfig _sysCfg,
        IProxyAdmin _admin
    )
        internal
        view
        returns (string memory)
    {
        IL1CrossDomainMessenger _messenger = IL1CrossDomainMessenger(_sysCfg.l1CrossDomainMessenger());
        _errors = internalRequire(stringEq(_messenger.version(), l1CrossDomainMessengerVersion()), "L1xDM-10", _errors);
        _errors = internalRequire(
            _admin.getProxyImplementation(address(_messenger)) == l1CrossDomainMessengerImpl, "L1xDM-20", _errors
        );

        IOptimismPortal2 _portal = IOptimismPortal2(payable(_sysCfg.optimismPortal()));

        _errors = internalRequire(
            address(_messenger.OTHER_MESSENGER()) == Predeploys.L2_CROSS_DOMAIN_MESSENGER, "L1xDM-30", _errors
        );
        _errors = internalRequire(
            address(_messenger.otherMessenger()) == Predeploys.L2_CROSS_DOMAIN_MESSENGER, "L1xDM-40", _errors
        );
        _errors = internalRequire(address(_messenger.PORTAL()) == address(_portal), "L1xDM-50", _errors);
        _errors = internalRequire(address(_messenger.portal()) == address(_portal), "L1xDM-60", _errors);
        _errors =
            internalRequire(address(_messenger.superchainConfig()) == address(superchainConfig), "L1xDM-70", _errors);
        return _errors;
    }

    function assertValidL1StandardBridge(
        string memory _errors,
        ISystemConfig _sysCfg,
        IProxyAdmin _admin
    )
        internal
        view
        returns (string memory)
    {
        IL1StandardBridge _bridge = IL1StandardBridge(payable(_sysCfg.l1StandardBridge()));
        _errors = internalRequire(stringEq(_bridge.version(), l1StandardBridgeVersion()), "L1SB-10", _errors);
        _errors =
            internalRequire(_admin.getProxyImplementation(address(_bridge)) == l1StandardBridgeImpl, "L1SB-20", _errors);

        IL1CrossDomainMessenger _messenger = IL1CrossDomainMessenger(_sysCfg.l1CrossDomainMessenger());

        _errors = internalRequire(address(_bridge.MESSENGER()) == address(_messenger), "L1SB-30", _errors);
        _errors = internalRequire(address(_bridge.messenger()) == address(_messenger), "L1SB-40", _errors);
        _errors = internalRequire(address(_bridge.OTHER_BRIDGE()) == Predeploys.L2_STANDARD_BRIDGE, "L1SB-50", _errors);
        _errors = internalRequire(address(_bridge.otherBridge()) == Predeploys.L2_STANDARD_BRIDGE, "L1SB-60", _errors);
        _errors = internalRequire(address(_bridge.superchainConfig()) == address(superchainConfig), "L1SB-70", _errors);
        return _errors;
    }

    function assertValidOptimismMintableERC20Factory(
        string memory _errors,
        ISystemConfig _sysCfg,
        IProxyAdmin _admin
    )
        internal
        view
        returns (string memory)
    {
        IOptimismMintableERC20Factory _factory = IOptimismMintableERC20Factory(_sysCfg.optimismMintableERC20Factory());
        _errors =
            internalRequire(stringEq(_factory.version(), optimismMintableERC20FactoryVersion()), "MERC20F-10", _errors);
        _errors = internalRequire(
            _admin.getProxyImplementation(address(_factory)) == optimismMintableERC20FactoryImpl, "MERC20F-20", _errors
        );

        IL1StandardBridge _bridge = IL1StandardBridge(payable(_sysCfg.l1StandardBridge()));
        _errors = internalRequire(_factory.BRIDGE() == address(_bridge), "MERC20F-30", _errors);
        _errors = internalRequire(_factory.bridge() == address(_bridge), "MERC20F-40", _errors);
        return _errors;
    }

    function assertValidL1ERC721Bridge(
        string memory _errors,
        ISystemConfig _sysCfg,
        IProxyAdmin _admin
    )
        internal
        view
        returns (string memory)
    {
        IL1ERC721Bridge _bridge = IL1ERC721Bridge(_sysCfg.l1ERC721Bridge());
        _errors = internalRequire(stringEq(_bridge.version(), l1ERC721BridgeVersion()), "L721B-10", _errors);
        _errors =
            internalRequire(_admin.getProxyImplementation(address(_bridge)) == l1ERC721BridgeImpl, "L721B-20", _errors);

        IL1CrossDomainMessenger _l1XDM = IL1CrossDomainMessenger(_sysCfg.l1CrossDomainMessenger());
        _errors = internalRequire(address(_bridge.OTHER_BRIDGE()) == Predeploys.L2_ERC721_BRIDGE, "L721B-30", _errors);
        _errors = internalRequire(address(_bridge.otherBridge()) == Predeploys.L2_ERC721_BRIDGE, "L721B-40", _errors);
        _errors = internalRequire(address(_bridge.MESSENGER()) == address(_l1XDM), "L721B-50", _errors);
        _errors = internalRequire(address(_bridge.messenger()) == address(_l1XDM), "L721B-60", _errors);
        _errors = internalRequire(address(_bridge.superchainConfig()) == address(superchainConfig), "L721B-70", _errors);
        return _errors;
    }

    function assertValidOptimismPortal(
        string memory _errors,
        ISystemConfig _sysCfg,
        IProxyAdmin _admin
    )
        internal
        view
        returns (string memory)
    {
        IOptimismPortal2 _portal = IOptimismPortal2(payable(_sysCfg.optimismPortal()));
        _errors = internalRequire(stringEq(_portal.version(), optimismPortalVersion()), "PORTAL-10", _errors);
        _errors =
            internalRequire(_admin.getProxyImplementation(address(_portal)) == optimismPortalImpl, "PORTAL-20", _errors);

        IDisputeGameFactory _dgf = IDisputeGameFactory(_sysCfg.disputeGameFactory());
        _errors = internalRequire(address(_portal.disputeGameFactory()) == address(_dgf), "PORTAL-30", _errors);
        _errors = internalRequire(address(_portal.systemConfig()) == address(_sysCfg), "PORTAL-40", _errors);
        _errors =
            internalRequire(address(_portal.superchainConfig()) == address(superchainConfig), "PORTAL-50", _errors);
        _errors = internalRequire(_portal.guardian() == superchainConfig.guardian(), "PORTAL-60", _errors);
        _errors = internalRequire(_portal.paused() == superchainConfig.paused(), "PORTAL-70", _errors);
        _errors = internalRequire(_portal.l2Sender() == Constants.DEFAULT_L2_SENDER, "PORTAL-80", _errors);
        return _errors;
    }

    function assertValidDisputeGameFactory(
        string memory _errors,
        ISystemConfig _sysCfg,
        IProxyAdmin _admin
    )
        internal
        view
        returns (string memory)
    {
        IDisputeGameFactory _factory = IDisputeGameFactory(_sysCfg.disputeGameFactory());
        _errors = internalRequire(stringEq(_factory.version(), disputeGameFactoryVersion()), "DF-10", _errors);
        _errors = internalRequire(
            _admin.getProxyImplementation(address(_factory)) == disputeGameFactoryImpl, "DF-20", _errors
        );
        _errors = internalRequire(_factory.owner() == l1PAOMultisig, "DF-30", _errors);
        return _errors;
    }

    function assertValidPermissionedDisputeGame(
        string memory _errors,
        ISystemConfig _sysCfg,
        bytes32 _absolutePrestate,
        uint256 _l2ChainID,
        IProxyAdmin _admin
    )
        internal
        view
        returns (string memory)
    {
        IDisputeGameFactory _factory = IDisputeGameFactory(_sysCfg.disputeGameFactory());
        IPermissionedDisputeGame _game =
            IPermissionedDisputeGame(address(_factory.gameImpls(GameTypes.PERMISSIONED_CANNON)));

        if (address(_game) == address(0)) {
            _errors = internalRequire(false, "PDDG-10", _errors);
            // Return early to avoid reverting, since this means that there is no valid game impl
            // for this game type.
            return _errors;
        }

        _errors = assertValidDisputeGame(
            _errors, _game, _factory, _absolutePrestate, _l2ChainID, _admin, GameTypes.PERMISSIONED_CANNON, "PDDG"
        );
        _errors = internalRequire(_game.challenger() == challenger, "PDDG-120", _errors);

        return _errors;
    }

    function assertValidPermissionlessDisputeGame(
        string memory _errors,
        ISystemConfig _sysCfg,
        bytes32 _absolutePrestate,
        uint256 _l2ChainID,
        IProxyAdmin _admin
    )
        internal
        view
        returns (string memory)
    {
        IDisputeGameFactory _factory = IDisputeGameFactory(_sysCfg.disputeGameFactory());
        IPermissionedDisputeGame _game = IPermissionedDisputeGame(address(_factory.gameImpls(GameTypes.CANNON)));

        if (address(_game) == address(0)) {
            _errors = internalRequire(false, "PLDG-10", _errors);
            // Return early to avoid reverting, since this means that there is no valid game impl
            // for this game type.
            return _errors;
        }

        _errors = assertValidDisputeGame(
            _errors, _game, _factory, _absolutePrestate, _l2ChainID, _admin, GameTypes.CANNON, "PLDG"
        );

        return _errors;
    }

    function assertValidDisputeGame(
        string memory _errors,
        IPermissionedDisputeGame _game,
        IDisputeGameFactory _factory,
        bytes32 _absolutePrestate,
        uint256 _l2ChainID,
        IProxyAdmin _admin,
        GameType _gameType,
        string memory _errorPrefix
    )
        internal
        view
        returns (string memory)
    {
        bool validGameVM = address(_game.vm()) == address(mips);

        _errors = internalRequire(
            stringEq(_game.version(), permissionedDisputeGameVersion()), string.concat(_errorPrefix, "-20"), _errors
        );
        IAnchorStateRegistry _asr = _game.anchorStateRegistry();
        _errors = internalRequire(
            GameType.unwrap(_game.gameType()) == GameType.unwrap(_gameType), string.concat(_errorPrefix, "-30"), _errors
        );
        _errors = internalRequire(
            Claim.unwrap(_game.absolutePrestate()) == _absolutePrestate, string.concat(_errorPrefix, "-40"), _errors
        );
        _errors = internalRequire(validGameVM, string.concat(_errorPrefix, "-50"), _errors);
        _errors = internalRequire(_game.l2ChainId() == _l2ChainID, string.concat(_errorPrefix, "-60"), _errors);
        _errors = internalRequire(_game.l2BlockNumber() == 0, string.concat(_errorPrefix, "-70"), _errors);
        _errors = internalRequire(
            Duration.unwrap(_game.clockExtension()) == 10800, string.concat(_errorPrefix, "-80"), _errors
        );
        _errors = internalRequire(_game.splitDepth() == 30, string.concat(_errorPrefix, "-90"), _errors);
        _errors = internalRequire(_game.maxGameDepth() == 73, string.concat(_errorPrefix, "-100"), _errors);
        _errors = internalRequire(
            Duration.unwrap(_game.maxClockDuration()) == 302400, string.concat(_errorPrefix, "-110"), _errors
        );

        _errors = assertValidDelayedWETH(_errors, _game.weth(), _admin, _errorPrefix);
        _errors = assertValidAnchorStateRegistry(_errors, _factory, _asr, _admin, _gameType, _errorPrefix);

        // Only assert valid preimage oracle if the game VM is valid, since otherwise
        // the contract is likely to revert.
        if (validGameVM) {
            _errors = assertValidPreimageOracle(_errors, _game.vm().oracle(), _errorPrefix);
        }

        return _errors;
    }

    function assertValidDelayedWETH(
        string memory _errors,
        IDelayedWETH _weth,
        IProxyAdmin _admin,
        string memory _errorPrefix
    )
        internal
        view
        returns (string memory)
    {
        _errorPrefix = string.concat(_errorPrefix, "-DWETH");
        _errors = internalRequire(
            stringEq(_weth.version(), delayedWETHVersion()), string.concat(_errorPrefix, "-10"), _errors
        );
        _errors = internalRequire(
            _admin.getProxyImplementation(address(_weth)) == delayedWETHImpl,
            string.concat(_errorPrefix, "-20"),
            _errors
        );
        _errors = internalRequire(_weth.owner() == l1PAOMultisig, string.concat(_errorPrefix, "-30"), _errors);
        _errors = internalRequire(_weth.delay() == 1 weeks, string.concat(_errorPrefix, "-40"), _errors);
        return _errors;
    }

    function assertValidAnchorStateRegistry(
        string memory _errors,
        IDisputeGameFactory _dgf,
        IAnchorStateRegistry _asr,
        IProxyAdmin,
        GameType _gameType,
        string memory _errorPrefix
    )
        internal
        view
        virtual
        returns (string memory)
    {
        _errorPrefix = string.concat(_errorPrefix, "-ANCHORP");
        _errors = internalRequire(
            stringEq(_asr.version(), anchorStateRegistryVersion()), string.concat(_errorPrefix, "-10"), _errors
        );
        _errors = internalRequire(
            address(_asr.disputeGameFactory()) == address(_dgf), string.concat(_errorPrefix, "-30"), _errors
        );

        (Hash actualRoot,) = _asr.anchors(_gameType);
        bytes32 expectedRoot = 0xdead000000000000000000000000000000000000000000000000000000000000;
        _errors = internalRequire(Hash.unwrap(actualRoot) == expectedRoot, string.concat(_errorPrefix, "-40"), _errors);
        _errors = internalRequire(
            address(_asr.superchainConfig()) == address(superchainConfig), string.concat(_errorPrefix, "-50"), _errors
        );
        return _errors;
    }

    function assertValidPreimageOracle(
        string memory _errors,
        IPreimageOracle _oracle,
        string memory _errorPrefix
    )
        internal
        view
        returns (string memory)
    {
        _errorPrefix = string.concat(_errorPrefix, "-PIMGO");
        // The preimage oracle's address is correct if the MIPS address is correct.
        _errors = internalRequire(
            stringEq(_oracle.version(), preimageOracleVersion()), string.concat(_errorPrefix, "-10"), _errors
        );
        _errors = internalRequire(_oracle.challengePeriod() == 86400, string.concat(_errorPrefix, "-20"), _errors);
        _errors = internalRequire(_oracle.minProposalSize() == 126000, string.concat(_errorPrefix, "-30"), _errors);
        return _errors;
    }

    function internalRequire(
        bool _condition,
        string memory _message,
        string memory _errors
    )
        internal
        pure
        returns (string memory)
    {
        if (_condition) {
            return _errors;
        }
        if (bytes(_errors).length == 0) {
            _errors = _message;
        } else {
            _errors = string.concat(_errors, ",", _message);
        }
        return _errors;
    }

    function stringEq(string memory _a, string memory _b) internal pure returns (bool) {
        return keccak256(bytes(_a)) == keccak256(bytes(_b));
    }
}

contract StandardValidatorV180 is StandardValidatorBase {
    struct InputV180 {
        IProxyAdmin proxyAdmin;
        ISystemConfig sysCfg;
        bytes32 absolutePrestate;
        uint256 l2ChainID;
    }

    constructor(
        ImplementationsBase memory _implementations,
        ISuperchainConfig _superchainConfig,
        IProtocolVersions _protocolVersions,
        address _l1PAOMultisig,
        address _mips,
        address _challenger
    )
        StandardValidatorBase(_implementations, _superchainConfig, _protocolVersions, _l1PAOMultisig, _mips, _challenger)
    { }

    function validate(InputV180 memory _input, bool _allowFailure) public view returns (string memory) {
        string memory _errors = "";

        _errors = super.validate(_errors, _input.sysCfg, _input.proxyAdmin, _input.absolutePrestate, _input.l2ChainID);

        if (bytes(_errors).length > 0 && !_allowFailure) {
            revert(string.concat("StandardValidatorV180: ", _errors));
        }

        return _errors;
    }
}

contract StandardValidatorV200 is StandardValidatorBase {
    struct InputV200 {
        IProxyAdmin proxyAdmin;
        ISystemConfig sysCfg;
        bytes32 absolutePrestate;
        uint256 l2ChainID;
    }

    constructor(
        ImplementationsBase memory _implementations,
        ISuperchainConfig _superchainConfig,
        IProtocolVersions _protocolVersions,
        address _l1PAOMultisig,
        address _mips,
        address _challenger
    )
        StandardValidatorBase(_implementations, _superchainConfig, _protocolVersions, _l1PAOMultisig, _mips, _challenger)
    { }

    function validate(InputV200 memory _input, bool _allowFailure) public view returns (string memory) {
        string memory _errors = "";

        _errors = super.validate(_errors, _input.sysCfg, _input.proxyAdmin, _input.absolutePrestate, _input.l2ChainID);

        if (bytes(_errors).length > 0 && !_allowFailure) {
            revert(string.concat("StandardValidatorV200: ", _errors));
        }

        return _errors;
    }

    function assertValidAnchorStateRegistry(
        string memory _errors,
        IDisputeGameFactory _dgf,
        IAnchorStateRegistry _asr,
        IProxyAdmin _admin,
        GameType _gameType,
        string memory _errorPrefix
    )
        internal
        view
        override
        returns (string memory)
    {
        _errors = super.assertValidAnchorStateRegistry(_errors, _dgf, _asr, _admin, _gameType, _errorPrefix);
        _errors = internalRequire(
            _admin.getProxyImplementation(address(_asr)) == anchorStateRegistryImpl,
            string.concat(_errorPrefix, "-ANCHORP-20"),
            _errors
        );
        return _errors;
    }

    function superchainConfigVersion() public pure override returns (string memory) {
        return "1.2.0";
    }

    function protocolVersionsVersion() public pure override returns (string memory) {
        return "1.1.0";
    }

    function l1ERC721BridgeVersion() public pure override returns (string memory) {
        return "2.3.0";
    }

    function optimismPortalVersion() public pure override returns (string memory) {
        return "3.12.0";
    }

    function systemConfigVersion() public pure override returns (string memory) {
        return "2.4.0";
    }

    function optimismMintableERC20FactoryVersion() public pure override returns (string memory) {
        return "1.10.1";
    }

    function l1CrossDomainMessengerVersion() public pure override returns (string memory) {
        return "2.5.0";
    }

    function l1StandardBridgeVersion() public pure override returns (string memory) {
        return "2.2.1";
    }

    function disputeGameFactoryVersion() public pure override returns (string memory) {
        return "1.0.1";
    }
}
