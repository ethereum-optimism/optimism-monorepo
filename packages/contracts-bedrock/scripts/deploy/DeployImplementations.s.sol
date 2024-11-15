// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Script } from "forge-std/Script.sol";

import { LibString } from "@solady/utils/LibString.sol";

import { IResourceMetering } from "src/L1/interfaces/IResourceMetering.sol";
import { ISuperchainConfig } from "src/L1/interfaces/ISuperchainConfig.sol";
import { IProtocolVersions } from "src/L1/interfaces/IProtocolVersions.sol";

import { Constants } from "src/libraries/Constants.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";
import { Bytes } from "src/libraries/Bytes.sol";

import { IDelayedWETH } from "src/dispute/interfaces/IDelayedWETH.sol";
import { IPreimageOracle } from "src/cannon/interfaces/IPreimageOracle.sol";
import { IMIPS } from "src/cannon/interfaces/IMIPS.sol";
import { IDisputeGameFactory } from "src/dispute/interfaces/IDisputeGameFactory.sol";

import { OPContractsManager } from "src/L1/OPContractsManager.sol";
import { IOptimismPortal2 } from "src/L1/interfaces/IOptimismPortal2.sol";
import { ISystemConfig } from "src/L1/interfaces/ISystemConfig.sol";
import { IL1CrossDomainMessenger } from "src/L1/interfaces/IL1CrossDomainMessenger.sol";
import { IL1ERC721Bridge } from "src/L1/interfaces/IL1ERC721Bridge.sol";
import { IL1StandardBridge } from "src/L1/interfaces/IL1StandardBridge.sol";
import { IOptimismMintableERC20Factory } from "src/universal/interfaces/IOptimismMintableERC20Factory.sol";

import { OPContractsManagerInterop } from "src/L1/OPContractsManagerInterop.sol";
import { IOptimismPortalInterop } from "src/L1/interfaces/IOptimismPortalInterop.sol";
import { ISystemConfigInterop } from "src/L1/interfaces/ISystemConfigInterop.sol";

import { Blueprint } from "src/libraries/Blueprint.sol";

import { DeployUtils } from "scripts/libraries/DeployUtils.sol";
import { Solarray } from "scripts/libraries/Solarray.sol";
import { BaseDeployIO } from "scripts/deploy/BaseDeployIO.sol";

// See DeploySuperchain.s.sol for detailed comments on the script architecture used here.
contract DeployImplementationsInput is BaseDeployIO {
    bytes32 internal _salt;
    uint256 internal _withdrawalDelaySeconds;
    uint256 internal _minProposalSizeBytes;
    uint256 internal _challengePeriodSeconds;
    uint256 internal _proofMaturityDelaySeconds;
    uint256 internal _disputeGameFinalityDelaySeconds;
    uint256 internal _mipsVersion;

    // This is used in opcm to signal which version of the L1 smart contracts is deployed.
    // It takes the format of `op-contracts/v*.*.*`.
    string internal _l1ContractsRelease;

    // Outputs from DeploySuperchain.s.sol.
    ISuperchainConfig internal _superchainConfigProxy;
    IProtocolVersions internal _protocolVersionsProxy;

    string internal _standardVersionsToml;

    function set(bytes4 _sel, uint256 _value) public {
        require(_value != 0, "DeployImplementationsInput: cannot set zero value");

        if (_sel == this.withdrawalDelaySeconds.selector) {
            _withdrawalDelaySeconds = _value;
        } else if (_sel == this.minProposalSizeBytes.selector) {
            _minProposalSizeBytes = _value;
        } else if (_sel == this.challengePeriodSeconds.selector) {
            require(_value <= type(uint64).max, "DeployImplementationsInput: challengePeriodSeconds too large");
            _challengePeriodSeconds = _value;
        } else if (_sel == this.proofMaturityDelaySeconds.selector) {
            _proofMaturityDelaySeconds = _value;
        } else if (_sel == this.disputeGameFinalityDelaySeconds.selector) {
            _disputeGameFinalityDelaySeconds = _value;
        } else if (_sel == this.mipsVersion.selector) {
            _mipsVersion = _value;
        } else {
            revert("DeployImplementationsInput: unknown selector");
        }
    }

    function set(bytes4 _sel, string memory _value) public {
        require(!LibString.eq(_value, ""), "DeployImplementationsInput: cannot set empty string");
        if (_sel == this.l1ContractsRelease.selector) _l1ContractsRelease = _value;
        else if (_sel == this.standardVersionsToml.selector) _standardVersionsToml = _value;
        else revert("DeployImplementationsInput: unknown selector");
    }

    function set(bytes4 _sel, address _addr) public {
        require(_addr != address(0), "DeployImplementationsInput: cannot set zero address");
        if (_sel == this.superchainConfigProxy.selector) _superchainConfigProxy = ISuperchainConfig(_addr);
        else if (_sel == this.protocolVersionsProxy.selector) _protocolVersionsProxy = IProtocolVersions(_addr);
        else revert("DeployImplementationsInput: unknown selector");
    }

    function set(bytes4 _sel, bytes32 _value) public {
        if (_sel == this.salt.selector) _salt = _value;
        else revert("DeployImplementationsInput: unknown selector");
    }

    function salt() public view returns (bytes32) {
        // TODO check if implementations are deployed based on code+salt and skip deploy if so.
        return _salt;
    }

    function withdrawalDelaySeconds() public view returns (uint256) {
        require(_withdrawalDelaySeconds != 0, "DeployImplementationsInput: not set");
        return _withdrawalDelaySeconds;
    }

    function minProposalSizeBytes() public view returns (uint256) {
        require(_minProposalSizeBytes != 0, "DeployImplementationsInput: not set");
        return _minProposalSizeBytes;
    }

    function challengePeriodSeconds() public view returns (uint256) {
        require(_challengePeriodSeconds != 0, "DeployImplementationsInput: not set");
        require(
            _challengePeriodSeconds <= type(uint64).max, "DeployImplementationsInput: challengePeriodSeconds too large"
        );
        return _challengePeriodSeconds;
    }

    function proofMaturityDelaySeconds() public view returns (uint256) {
        require(_proofMaturityDelaySeconds != 0, "DeployImplementationsInput: not set");
        return _proofMaturityDelaySeconds;
    }

    function disputeGameFinalityDelaySeconds() public view returns (uint256) {
        require(_disputeGameFinalityDelaySeconds != 0, "DeployImplementationsInput: not set");
        return _disputeGameFinalityDelaySeconds;
    }

    function mipsVersion() public view returns (uint256) {
        require(_mipsVersion != 0, "DeployImplementationsInput: not set");
        return _mipsVersion;
    }

    function l1ContractsRelease() public view returns (string memory) {
        require(!LibString.eq(_l1ContractsRelease, ""), "DeployImplementationsInput: not set");
        return _l1ContractsRelease;
    }

    function standardVersionsToml() public view returns (string memory) {
        require(!LibString.eq(_standardVersionsToml, ""), "DeployImplementationsInput: not set");
        return _standardVersionsToml;
    }

    function superchainConfigProxy() public view returns (ISuperchainConfig) {
        require(address(_superchainConfigProxy) != address(0), "DeployImplementationsInput: not set");
        return _superchainConfigProxy;
    }

    function protocolVersionsProxy() public view returns (IProtocolVersions) {
        require(address(_protocolVersionsProxy) != address(0), "DeployImplementationsInput: not set");
        return _protocolVersionsProxy;
    }
}

contract DeployImplementationsOutput is BaseDeployIO {
    OPContractsManager internal _opcm;
    IDelayedWETH internal _delayedWETHImpl;
    IOptimismPortal2 internal _optimismPortalImpl;
    IPreimageOracle internal _preimageOracleSingleton;
    IMIPS internal _mipsSingleton;
    ISystemConfig internal _systemConfigImpl;
    IL1CrossDomainMessenger internal _l1CrossDomainMessengerImpl;
    IL1ERC721Bridge internal _l1ERC721BridgeImpl;
    IL1StandardBridge internal _l1StandardBridgeImpl;
    IOptimismMintableERC20Factory internal _optimismMintableERC20FactoryImpl;
    IDisputeGameFactory internal _disputeGameFactoryImpl;

    function set(bytes4 _sel, address _addr) public {
        require(_addr != address(0), "DeployImplementationsOutput: cannot set zero address");

        // forgefmt: disable-start
        if (_sel == this.opcm.selector) _opcm = OPContractsManager(_addr);
        else if (_sel == this.optimismPortalImpl.selector) _optimismPortalImpl = IOptimismPortal2(payable(_addr));
        else if (_sel == this.delayedWETHImpl.selector) _delayedWETHImpl = IDelayedWETH(payable(_addr));
        else if (_sel == this.preimageOracleSingleton.selector) _preimageOracleSingleton = IPreimageOracle(_addr);
        else if (_sel == this.mipsSingleton.selector) _mipsSingleton = IMIPS(_addr);
        else if (_sel == this.systemConfigImpl.selector) _systemConfigImpl = ISystemConfig(_addr);
        else if (_sel == this.l1CrossDomainMessengerImpl.selector) _l1CrossDomainMessengerImpl = IL1CrossDomainMessenger(_addr);
        else if (_sel == this.l1ERC721BridgeImpl.selector) _l1ERC721BridgeImpl = IL1ERC721Bridge(_addr);
        else if (_sel == this.l1StandardBridgeImpl.selector) _l1StandardBridgeImpl = IL1StandardBridge(payable(_addr));
        else if (_sel == this.optimismMintableERC20FactoryImpl.selector) _optimismMintableERC20FactoryImpl = IOptimismMintableERC20Factory(_addr);
        else if (_sel == this.disputeGameFactoryImpl.selector) _disputeGameFactoryImpl = IDisputeGameFactory(_addr);
        else revert("DeployImplementationsOutput: unknown selector");
        // forgefmt: disable-end
    }

    function checkOutput(DeployImplementationsInput _dii) public view {
        // With 12 addresses, we'd get a stack too deep error if we tried to do this inline as a
        // single call to `Solarray.addresses`. So we split it into two calls.
        address[] memory addrs1 = Solarray.addresses(
            address(this.opcm()),
            address(this.optimismPortalImpl()),
            address(this.delayedWETHImpl()),
            address(this.preimageOracleSingleton()),
            address(this.mipsSingleton())
        );

        address[] memory addrs2 = Solarray.addresses(
            address(this.systemConfigImpl()),
            address(this.l1CrossDomainMessengerImpl()),
            address(this.l1ERC721BridgeImpl()),
            address(this.l1StandardBridgeImpl()),
            address(this.optimismMintableERC20FactoryImpl()),
            address(this.disputeGameFactoryImpl())
        );

        DeployUtils.assertValidContractAddresses(Solarray.extend(addrs1, addrs2));

        assertValidDeploy(_dii);
    }

    function opcm() public view returns (OPContractsManager) {
        DeployUtils.assertValidContractAddress(address(_opcm));
        return _opcm;
    }

    function optimismPortalImpl() public view returns (IOptimismPortal2) {
        DeployUtils.assertValidContractAddress(address(_optimismPortalImpl));
        return _optimismPortalImpl;
    }

    function delayedWETHImpl() public view returns (IDelayedWETH) {
        DeployUtils.assertValidContractAddress(address(_delayedWETHImpl));
        return _delayedWETHImpl;
    }

    function preimageOracleSingleton() public view returns (IPreimageOracle) {
        DeployUtils.assertValidContractAddress(address(_preimageOracleSingleton));
        return _preimageOracleSingleton;
    }

    function mipsSingleton() public view returns (IMIPS) {
        DeployUtils.assertValidContractAddress(address(_mipsSingleton));
        return _mipsSingleton;
    }

    function systemConfigImpl() public view returns (ISystemConfig) {
        DeployUtils.assertValidContractAddress(address(_systemConfigImpl));
        return _systemConfigImpl;
    }

    function l1CrossDomainMessengerImpl() public view returns (IL1CrossDomainMessenger) {
        DeployUtils.assertValidContractAddress(address(_l1CrossDomainMessengerImpl));
        return _l1CrossDomainMessengerImpl;
    }

    function l1ERC721BridgeImpl() public view returns (IL1ERC721Bridge) {
        DeployUtils.assertValidContractAddress(address(_l1ERC721BridgeImpl));
        return _l1ERC721BridgeImpl;
    }

    function l1StandardBridgeImpl() public view returns (IL1StandardBridge) {
        DeployUtils.assertValidContractAddress(address(_l1StandardBridgeImpl));
        return _l1StandardBridgeImpl;
    }

    function optimismMintableERC20FactoryImpl() public view returns (IOptimismMintableERC20Factory) {
        DeployUtils.assertValidContractAddress(address(_optimismMintableERC20FactoryImpl));
        return _optimismMintableERC20FactoryImpl;
    }

    function disputeGameFactoryImpl() public view returns (IDisputeGameFactory) {
        DeployUtils.assertValidContractAddress(address(_disputeGameFactoryImpl));
        return _disputeGameFactoryImpl;
    }

    // -------- Deployment Assertions --------
    function assertValidDeploy(DeployImplementationsInput _dii) public view {
        assertValidDelayedWETHImpl(_dii);
        assertValidDisputeGameFactoryImpl(_dii);
        assertValidL1CrossDomainMessengerImpl(_dii);
        assertValidL1ERC721BridgeImpl(_dii);
        assertValidL1StandardBridgeImpl(_dii);
        assertValidMipsSingleton(_dii);
        assertValidOpcm(_dii);
        assertValidOptimismMintableERC20FactoryImpl(_dii);
        assertValidOptimismPortalImpl(_dii);
        assertValidPreimageOracleSingleton(_dii);
        assertValidSystemConfigImpl(_dii);
    }

    function assertValidOpcm(DeployImplementationsInput _dii) internal view {
        OPContractsManager impl = OPContractsManager(address(opcm()));
        require(address(impl.superchainConfig()) == address(_dii.superchainConfigProxy()), "OPCMI-10");
        require(address(impl.protocolVersions()) == address(_dii.protocolVersionsProxy()), "OPCMI-20");
    }

    function assertValidOptimismPortalImpl(DeployImplementationsInput) internal view {
        IOptimismPortal2 portal = optimismPortalImpl();

        DeployUtils.assertInitialized({ _contractAddress: address(portal), _slot: 0, _offset: 0 });

        require(address(portal.disputeGameFactory()) == address(0), "PORTAL-10");
        require(address(portal.systemConfig()) == address(0), "PORTAL-20");
        require(address(portal.superchainConfig()) == address(0), "PORTAL-30");
        require(portal.l2Sender() == Constants.DEFAULT_L2_SENDER, "PORTAL-40");

        // This slot is the custom gas token _balance and this check ensures
        // that it stays unset for forwards compatibility with custom gas token.
        require(vm.load(address(portal), bytes32(uint256(61))) == bytes32(0), "PORTAL-50");
    }

    function assertValidDelayedWETHImpl(DeployImplementationsInput _dii) internal view {
        IDelayedWETH delayedWETH = delayedWETHImpl();

        DeployUtils.assertInitialized({ _contractAddress: address(delayedWETH), _slot: 0, _offset: 0 });

        require(delayedWETH.owner() == address(0), "DW-10");
        require(delayedWETH.delay() == _dii.withdrawalDelaySeconds(), "DW-20");
        require(delayedWETH.config() == ISuperchainConfig(address(0)), "DW-30");
    }

    function assertValidPreimageOracleSingleton(DeployImplementationsInput _dii) internal view {
        IPreimageOracle oracle = preimageOracleSingleton();

        require(oracle.minProposalSize() == _dii.minProposalSizeBytes(), "PO-10");
        require(oracle.challengePeriod() == _dii.challengePeriodSeconds(), "PO-20");
    }

    function assertValidMipsSingleton(DeployImplementationsInput) internal view {
        IMIPS mips = mipsSingleton();
        require(address(mips.oracle()) == address(preimageOracleSingleton()), "MIPS-10");
    }

    function assertValidSystemConfigImpl(DeployImplementationsInput) internal view {
        ISystemConfig systemConfig = systemConfigImpl();

        DeployUtils.assertInitialized({ _contractAddress: address(systemConfig), _slot: 0, _offset: 0 });

        require(systemConfig.owner() == address(0xdead), "SYSCON-10");
        require(systemConfig.overhead() == 0, "SYSCON-20");
        require(systemConfig.scalar() == uint256(0x01) << 248, "SYSCON-30");
        require(systemConfig.basefeeScalar() == 0, "SYSCON-40");
        require(systemConfig.blobbasefeeScalar() == 0, "SYSCON-50");
        require(systemConfig.batcherHash() == bytes32(0), "SYSCON-60");
        require(systemConfig.gasLimit() == 1, "SYSCON-70");
        require(systemConfig.unsafeBlockSigner() == address(0), "SYSCON-80");

        IResourceMetering.ResourceConfig memory resourceConfig = systemConfig.resourceConfig();
        require(resourceConfig.maxResourceLimit == 1, "SYSCON-90");
        require(resourceConfig.elasticityMultiplier == 1, "SYSCON-100");
        require(resourceConfig.baseFeeMaxChangeDenominator == 2, "SYSCON-110");
        require(resourceConfig.systemTxMaxGas == 0, "SYSCON-120");
        require(resourceConfig.minimumBaseFee == 0, "SYSCON-130");
        require(resourceConfig.maximumBaseFee == 0, "SYSCON-140");

        require(systemConfig.startBlock() == type(uint256).max, "SYSCON-150");
        require(systemConfig.batchInbox() == address(0), "SYSCON-160");
        require(systemConfig.l1CrossDomainMessenger() == address(0), "SYSCON-170");
        require(systemConfig.l1ERC721Bridge() == address(0), "SYSCON-180");
        require(systemConfig.l1StandardBridge() == address(0), "SYSCON-190");
        require(systemConfig.disputeGameFactory() == address(0), "SYSCON-200");
        require(systemConfig.optimismPortal() == address(0), "SYSCON-210");
        require(systemConfig.optimismMintableERC20Factory() == address(0), "SYSCON-220");
    }

    function assertValidL1CrossDomainMessengerImpl(DeployImplementationsInput) internal view {
        IL1CrossDomainMessenger messenger = l1CrossDomainMessengerImpl();

        DeployUtils.assertInitialized({ _contractAddress: address(messenger), _slot: 0, _offset: 20 });

        require(address(messenger.OTHER_MESSENGER()) == Predeploys.L2_CROSS_DOMAIN_MESSENGER, "L1xDM-10");
        require(address(messenger.otherMessenger()) == Predeploys.L2_CROSS_DOMAIN_MESSENGER, "L1xDM-20");
        require(address(messenger.PORTAL()) == address(0), "L1xDM-30");
        require(address(messenger.portal()) == address(0), "L1xDM-40");
        require(address(messenger.superchainConfig()) == address(0), "L1xDM-50");

        bytes32 xdmSenderSlot = vm.load(address(messenger), bytes32(uint256(204)));
        require(address(uint160(uint256(xdmSenderSlot))) == Constants.DEFAULT_L2_SENDER, "L1xDM-60");
    }

    function assertValidL1ERC721BridgeImpl(DeployImplementationsInput) internal view {
        IL1ERC721Bridge bridge = l1ERC721BridgeImpl();

        DeployUtils.assertInitialized({ _contractAddress: address(bridge), _slot: 0, _offset: 0 });

        require(address(bridge.OTHER_BRIDGE()) == Predeploys.L2_ERC721_BRIDGE, "L721B-10");
        require(address(bridge.otherBridge()) == Predeploys.L2_ERC721_BRIDGE, "L721B-20");
        require(address(bridge.MESSENGER()) == address(0), "L721B-30");
        require(address(bridge.messenger()) == address(0), "L721B-40");
        require(address(bridge.superchainConfig()) == address(0), "L721B-50");
    }

    function assertValidL1StandardBridgeImpl(DeployImplementationsInput) internal view {
        IL1StandardBridge bridge = l1StandardBridgeImpl();

        DeployUtils.assertInitialized({ _contractAddress: address(bridge), _slot: 0, _offset: 0 });

        require(address(bridge.MESSENGER()) == address(0), "L1SB-10");
        require(address(bridge.messenger()) == address(0), "L1SB-20");
        require(address(bridge.OTHER_BRIDGE()) == Predeploys.L2_STANDARD_BRIDGE, "L1SB-30");
        require(address(bridge.otherBridge()) == Predeploys.L2_STANDARD_BRIDGE, "L1SB-40");
        require(address(bridge.superchainConfig()) == address(0), "L1SB-50");
    }

    function assertValidOptimismMintableERC20FactoryImpl(DeployImplementationsInput) internal view {
        IOptimismMintableERC20Factory factory = optimismMintableERC20FactoryImpl();

        DeployUtils.assertInitialized({ _contractAddress: address(factory), _slot: 0, _offset: 0 });

        require(address(factory.BRIDGE()) == address(0), "MERC20F-10");
        require(address(factory.bridge()) == address(0), "MERC20F-20");
    }

    function assertValidDisputeGameFactoryImpl(DeployImplementationsInput) internal view {
        IDisputeGameFactory factory = disputeGameFactoryImpl();

        DeployUtils.assertInitialized({ _contractAddress: address(factory), _slot: 0, _offset: 0 });

        require(address(factory.owner()) == address(0), "DG-10");
    }
}

contract DeployImplementations is Script {
    // -------- Core Deployment Methods --------

    function run(DeployImplementationsInput _dii, DeployImplementationsOutput _dio) public {
        // Deploy the implementations.
        deploySystemConfigImpl(_dii, _dio);
        deployL1CrossDomainMessengerImpl(_dii, _dio);
        deployL1ERC721BridgeImpl(_dii, _dio);
        deployL1StandardBridgeImpl(_dii, _dio);
        deployOptimismMintableERC20FactoryImpl(_dii, _dio);
        deployOptimismPortalImpl(_dii, _dio);
        deployDelayedWETHImpl(_dii, _dio);
        deployPreimageOracleSingleton(_dii, _dio);
        deployMipsSingleton(_dii, _dio);
        deployDisputeGameFactoryImpl(_dii, _dio);

        // Deploy the OP Contracts Manager with the new implementations set.
        deployOPContractsManager(_dii, _dio);

        _dio.checkOutput(_dii);
    }

    // -------- Deployment Steps --------

    // --- OP Contracts Manager ---

    function createOPCMContract(
        DeployImplementationsInput _dii,
        DeployImplementationsOutput _dio,
        OPContractsManager.Blueprints memory _blueprints,
        string memory _l1ContractsRelease
    )
        internal
        virtual
        returns (OPContractsManager opcm_)
    {
        ISuperchainConfig superchainConfigProxy = _dii.superchainConfigProxy();
        IProtocolVersions protocolVersionsProxy = _dii.protocolVersionsProxy();

        OPContractsManager.Implementations memory implementations = OPContractsManager.Implementations({
            l1ERC721BridgeImpl: address(_dio.l1ERC721BridgeImpl()),
            optimismPortalImpl: address(_dio.optimismPortalImpl()),
            systemConfigImpl: address(_dio.systemConfigImpl()),
            optimismMintableERC20FactoryImpl: address(_dio.optimismMintableERC20FactoryImpl()),
            l1CrossDomainMessengerImpl: address(_dio.l1CrossDomainMessengerImpl()),
            l1StandardBridgeImpl: address(_dio.l1StandardBridgeImpl()),
            disputeGameFactoryImpl: address(_dio.disputeGameFactoryImpl()),
            delayedWETHImpl: address(_dio.delayedWETHImpl()),
            mipsImpl: address(_dio.mipsSingleton())
        });

        vm.broadcast(msg.sender);
        opcm_ = new OPContractsManager(
            superchainConfigProxy, protocolVersionsProxy, _l1ContractsRelease, _blueprints, implementations
        );

        vm.label(address(opcm_), "OPContractsManager");
        _dio.set(_dio.opcm.selector, address(opcm_));
    }

    function deployOPContractsManager(
        DeployImplementationsInput _dii,
        DeployImplementationsOutput _dio
    )
        public
        virtual
    {
        string memory l1ContractsRelease = _dii.l1ContractsRelease();
        string memory stdVerToml = _dii.standardVersionsToml();
        string memory contractName = "op_contracts_manager";
        OPContractsManager opcm;

        address existingImplementation = getReleaseAddress(l1ContractsRelease, contractName, stdVerToml);
        if (existingImplementation != address(0)) {
            opcm = OPContractsManager(existingImplementation);
        } else {
            // First we deploy the blueprints for the singletons deployed by OPCM.
            // forgefmt: disable-start
            bytes32 salt = _dii.salt();
            OPContractsManager.Blueprints memory blueprints;

            vm.startBroadcast(msg.sender);
            blueprints.addressManager = deployBytecode(Blueprint.blueprintDeployerBytecode(vm.getCode("AddressManager")), salt);
            blueprints.proxy = deployBytecode(Blueprint.blueprintDeployerBytecode(vm.getCode("Proxy")), salt);
            blueprints.proxyAdmin = deployBytecode(Blueprint.blueprintDeployerBytecode(vm.getCode("ProxyAdmin")), salt);
            blueprints.l1ChugSplashProxy = deployBytecode(Blueprint.blueprintDeployerBytecode(vm.getCode("L1ChugSplashProxy")), salt);
            blueprints.resolvedDelegateProxy = deployBytecode(Blueprint.blueprintDeployerBytecode(vm.getCode("ResolvedDelegateProxy")), salt);
            blueprints.anchorStateRegistry = deployBytecode(Blueprint.blueprintDeployerBytecode(vm.getCode("AnchorStateRegistry")), salt);
            (blueprints.permissionedDisputeGame1, blueprints.permissionedDisputeGame2)  = deployBigBytecode(vm.getCode("PermissionedDisputeGame"), salt);
            vm.stopBroadcast();
            // forgefmt: disable-end

            opcm = createOPCMContract(_dii, _dio, blueprints, l1ContractsRelease);
        }

        vm.label(address(opcm), "OPContractsManager");
        _dio.set(_dio.opcm.selector, address(opcm));
    }

    // --- Core Contracts ---

    function deploySystemConfigImpl(DeployImplementationsInput _dii, DeployImplementationsOutput _dio) public virtual {
        string memory release = _dii.l1ContractsRelease();
        string memory stdVerToml = _dii.standardVersionsToml();
        // Using snake case for contract name to match the TOML file in superchain-registry.
        string memory contractName = "system_config";
        ISystemConfig impl;

        address existingImplementation = getReleaseAddress(release, contractName, stdVerToml);
        if (existingImplementation != address(0)) {
            impl = ISystemConfig(existingImplementation);
        } else {
            // Deploy a new implementation for development builds.
            vm.broadcast(msg.sender);
            impl = ISystemConfig(
                DeployUtils.create1({
                    _name: "SystemConfig",
                    _args: DeployUtils.encodeConstructor(abi.encodeCall(ISystemConfig.__constructor__, ()))
                })
            );
        }

        vm.label(address(impl), "SystemConfigImpl");
        _dio.set(_dio.systemConfigImpl.selector, address(impl));
    }

    function deployL1CrossDomainMessengerImpl(
        DeployImplementationsInput _dii,
        DeployImplementationsOutput _dio
    )
        public
        virtual
    {
        string memory release = _dii.l1ContractsRelease();
        string memory stdVerToml = _dii.standardVersionsToml();
        string memory contractName = "l1_cross_domain_messenger";
        IL1CrossDomainMessenger impl;

        address existingImplementation = getReleaseAddress(release, contractName, stdVerToml);
        if (existingImplementation != address(0)) {
            impl = IL1CrossDomainMessenger(existingImplementation);
        } else {
            vm.broadcast(msg.sender);
            impl = IL1CrossDomainMessenger(
                DeployUtils.create1({
                    _name: "L1CrossDomainMessenger",
                    _args: DeployUtils.encodeConstructor(abi.encodeCall(IL1CrossDomainMessenger.__constructor__, ()))
                })
            );
        }

        vm.label(address(impl), "L1CrossDomainMessengerImpl");
        _dio.set(_dio.l1CrossDomainMessengerImpl.selector, address(impl));
    }

    function deployL1ERC721BridgeImpl(
        DeployImplementationsInput _dii,
        DeployImplementationsOutput _dio
    )
        public
        virtual
    {
        string memory release = _dii.l1ContractsRelease();
        string memory stdVerToml = _dii.standardVersionsToml();
        string memory contractName = "l1_erc721_bridge";
        IL1ERC721Bridge impl;

        address existingImplementation = getReleaseAddress(release, contractName, stdVerToml);
        if (existingImplementation != address(0)) {
            impl = IL1ERC721Bridge(existingImplementation);
        } else {
            vm.broadcast(msg.sender);
            impl = IL1ERC721Bridge(
                DeployUtils.create1({
                    _name: "L1ERC721Bridge",
                    _args: DeployUtils.encodeConstructor(abi.encodeCall(IL1ERC721Bridge.__constructor__, ()))
                })
            );
        }

        vm.label(address(impl), "L1ERC721BridgeImpl");
        _dio.set(_dio.l1ERC721BridgeImpl.selector, address(impl));
    }

    function deployL1StandardBridgeImpl(
        DeployImplementationsInput _dii,
        DeployImplementationsOutput _dio
    )
        public
        virtual
    {
        string memory release = _dii.l1ContractsRelease();
        string memory stdVerToml = _dii.standardVersionsToml();
        string memory contractName = "l1_standard_bridge";
        IL1StandardBridge impl;

        address existingImplementation = getReleaseAddress(release, contractName, stdVerToml);
        if (existingImplementation != address(0)) {
            impl = IL1StandardBridge(payable(existingImplementation));
        } else {
            vm.broadcast(msg.sender);
            impl = IL1StandardBridge(
                DeployUtils.create1({
                    _name: "L1StandardBridge",
                    _args: DeployUtils.encodeConstructor(abi.encodeCall(IL1StandardBridge.__constructor__, ()))
                })
            );
        }

        vm.label(address(impl), "L1StandardBridgeImpl");
        _dio.set(_dio.l1StandardBridgeImpl.selector, address(impl));
    }

    function deployOptimismMintableERC20FactoryImpl(
        DeployImplementationsInput _dii,
        DeployImplementationsOutput _dio
    )
        public
        virtual
    {
        string memory release = _dii.l1ContractsRelease();
        string memory stdVerToml = _dii.standardVersionsToml();
        string memory contractName = "optimism_mintable_erc20_factory";
        IOptimismMintableERC20Factory impl;

        address existingImplementation = getReleaseAddress(release, contractName, stdVerToml);
        if (existingImplementation != address(0)) {
            impl = IOptimismMintableERC20Factory(existingImplementation);
        } else {
            vm.broadcast(msg.sender);
            impl = IOptimismMintableERC20Factory(
                DeployUtils.create1({
                    _name: "OptimismMintableERC20Factory",
                    _args: DeployUtils.encodeConstructor(abi.encodeCall(IOptimismMintableERC20Factory.__constructor__, ()))
                })
            );
        }

        vm.label(address(impl), "OptimismMintableERC20FactoryImpl");
        _dio.set(_dio.optimismMintableERC20FactoryImpl.selector, address(impl));
    }

    // --- Fault Proofs Contracts ---

    // The fault proofs contracts are configured as follows:
    // | Contract                | Proxied | Deployment                        | MCP Ready  |
    // |-------------------------|---------|-----------------------------------|------------|
    // | DisputeGameFactory      | Yes     | Bespoke                           | Yes        |
    // | AnchorStateRegistry     | Yes     | Bespoke                           | No         |
    // | FaultDisputeGame        | No      | Bespoke                           | No         | Not yet supported by OPCM
    // | PermissionedDisputeGame | No      | Bespoke                           | No         |
    // | DelayedWETH             | Yes     | Two bespoke (one per DisputeGame) | Yes *️⃣     |
    // | PreimageOracle          | No      | Shared                            | N/A        |
    // | MIPS                    | No      | Shared                            | N/A        |
    // | OptimismPortal2         | Yes     | Shared                            | Yes *️⃣     |
    //
    // - *️⃣ These contracts have immutable values which are intended to be constant for all contracts within a
    //   Superchain, and are therefore MCP ready for any chain using the Standard Configuration.
    //
    // This script only deploys the shared contracts. The bespoke contracts are deployed by
    // `DeployOPChain.s.sol`. When the shared contracts are proxied, the contracts deployed here are
    // "implementations", and when shared contracts are not proxied, they are "singletons". So
    // here we deploy:
    //
    //   - DisputeGameFactory (implementation)
    //   - OptimismPortal2 (implementation)
    //   - DelayedWETH (implementation)
    //   - PreimageOracle (singleton)
    //   - MIPS (singleton)
    //
    // For contracts which are not MCP ready neither the Proxy nor the implementation can be shared, therefore they
    // are deployed by `DeployOpChain.s.sol`.
    // These are:
    // - AnchorStateRegistry (proxy and implementation)
    // - FaultDisputeGame (not proxied)
    // - PermissionedDisputeGame (not proxied)
    // - DelayedWeth (proxies only)
    // - OptimismPortal2 (proxies only)

    function deployOptimismPortalImpl(
        DeployImplementationsInput _dii,
        DeployImplementationsOutput _dio
    )
        public
        virtual
    {
        string memory release = _dii.l1ContractsRelease();
        string memory stdVerToml = _dii.standardVersionsToml();
        string memory contractName = "optimism_portal";
        IOptimismPortal2 impl;

        address existingImplementation = getReleaseAddress(release, contractName, stdVerToml);
        if (existingImplementation != address(0)) {
            impl = IOptimismPortal2(payable(existingImplementation));
        } else {
            uint256 proofMaturityDelaySeconds = _dii.proofMaturityDelaySeconds();
            uint256 disputeGameFinalityDelaySeconds = _dii.disputeGameFinalityDelaySeconds();
            vm.broadcast(msg.sender);
            impl = IOptimismPortal2(
                DeployUtils.create1({
                    _name: "OptimismPortal2",
                    _args: DeployUtils.encodeConstructor(
                        abi.encodeCall(
                            IOptimismPortal2.__constructor__, (proofMaturityDelaySeconds, disputeGameFinalityDelaySeconds)
                        )
                    )
                })
            );
        }

        vm.label(address(impl), "OptimismPortalImpl");
        _dio.set(_dio.optimismPortalImpl.selector, address(impl));
    }

    function deployDelayedWETHImpl(DeployImplementationsInput _dii, DeployImplementationsOutput _dio) public virtual {
        string memory release = _dii.l1ContractsRelease();
        string memory stdVerToml = _dii.standardVersionsToml();
        string memory contractName = "delayed_weth";
        IDelayedWETH impl;

        address existingImplementation = getReleaseAddress(release, contractName, stdVerToml);
        if (existingImplementation != address(0)) {
            impl = IDelayedWETH(payable(existingImplementation));
        } else {
            uint256 withdrawalDelaySeconds = _dii.withdrawalDelaySeconds();
            vm.broadcast(msg.sender);
            impl = IDelayedWETH(
                DeployUtils.create1({
                    _name: "DelayedWETH",
                    _args: DeployUtils.encodeConstructor(
                        abi.encodeCall(IDelayedWETH.__constructor__, (withdrawalDelaySeconds))
                    )
                })
            );
        }

        vm.label(address(impl), "DelayedWETHImpl");
        _dio.set(_dio.delayedWETHImpl.selector, address(impl));
    }

    function deployPreimageOracleSingleton(
        DeployImplementationsInput _dii,
        DeployImplementationsOutput _dio
    )
        public
        virtual
    {
        string memory release = _dii.l1ContractsRelease();
        string memory stdVerToml = _dii.standardVersionsToml();
        string memory contractName = "preimage_oracle";
        IPreimageOracle singleton;

        address existingImplementation = getReleaseAddress(release, contractName, stdVerToml);
        if (existingImplementation != address(0)) {
            singleton = IPreimageOracle(payable(existingImplementation));
        } else {
            uint256 minProposalSizeBytes = _dii.minProposalSizeBytes();
            uint256 challengePeriodSeconds = _dii.challengePeriodSeconds();
            vm.broadcast(msg.sender);
            singleton = IPreimageOracle(
                DeployUtils.create1({
                    _name: "PreimageOracle",
                    _args: DeployUtils.encodeConstructor(
                        abi.encodeCall(IPreimageOracle.__constructor__, (minProposalSizeBytes, challengePeriodSeconds))
                    )
                })
            );
        }

        vm.label(address(singleton), "PreimageOracleSingleton");
        _dio.set(_dio.preimageOracleSingleton.selector, address(singleton));
    }

    function deployMipsSingleton(DeployImplementationsInput _dii, DeployImplementationsOutput _dio) public virtual {
        string memory release = _dii.l1ContractsRelease();
        string memory stdVerToml = _dii.standardVersionsToml();
        string memory contractName = "mips";
        IMIPS singleton;

        address existingImplementation = getReleaseAddress(release, contractName, stdVerToml);
        if (existingImplementation != address(0)) {
            singleton = IMIPS(payable(existingImplementation));
        } else {
            uint256 mipsVersion = _dii.mipsVersion();
            IPreimageOracle preimageOracle = IPreimageOracle(address(_dio.preimageOracleSingleton()));
            vm.broadcast(msg.sender);
            singleton = IMIPS(
                DeployUtils.create1({
                    _name: mipsVersion == 1 ? "MIPS" : "MIPS2",
                    _args: DeployUtils.encodeConstructor(abi.encodeCall(IMIPS.__constructor__, (preimageOracle)))
                })
            );
        }

        vm.label(address(singleton), "MIPSSingleton");
        _dio.set(_dio.mipsSingleton.selector, address(singleton));
    }

    function deployDisputeGameFactoryImpl(
        DeployImplementationsInput _dii,
        DeployImplementationsOutput _dio
    )
        public
        virtual
    {
        string memory release = _dii.l1ContractsRelease();
        string memory stdVerToml = _dii.standardVersionsToml();
        string memory contractName = "dispute_game_factory";
        IDisputeGameFactory impl;

        address existingImplementation = getReleaseAddress(release, contractName, stdVerToml);
        if (existingImplementation != address(0)) {
            impl = IDisputeGameFactory(payable(existingImplementation));
        } else {
            vm.broadcast(msg.sender);
            impl = IDisputeGameFactory(
                DeployUtils.create1({
                    _name: "DisputeGameFactory",
                    _args: DeployUtils.encodeConstructor(abi.encodeCall(IDisputeGameFactory.__constructor__, ()))
                })
            );
        }

        vm.label(address(impl), "DisputeGameFactoryImpl");
        _dio.set(_dio.disputeGameFactoryImpl.selector, address(impl));
    }

    // -------- Utilities --------

    function etchIOContracts() public returns (DeployImplementationsInput dii_, DeployImplementationsOutput dio_) {
        (dii_, dio_) = getIOContracts();
        vm.etch(address(dii_), type(DeployImplementationsInput).runtimeCode);
        vm.etch(address(dio_), type(DeployImplementationsOutput).runtimeCode);
    }

    function getIOContracts() public view returns (DeployImplementationsInput dii_, DeployImplementationsOutput dio_) {
        dii_ = DeployImplementationsInput(DeployUtils.toIOAddress(msg.sender, "optimism.DeployImplementationsInput"));
        dio_ = DeployImplementationsOutput(DeployUtils.toIOAddress(msg.sender, "optimism.DeployImplementationsOutput"));
    }

    function deployBytecode(bytes memory _bytecode, bytes32 _salt) public returns (address newContract_) {
        assembly ("memory-safe") {
            newContract_ := create2(0, add(_bytecode, 0x20), mload(_bytecode), _salt)
        }
        require(newContract_ != address(0), "DeployImplementations: create2 failed");
    }

    function deployBigBytecode(
        bytes memory _bytecode,
        bytes32 _salt
    )
        public
        returns (address newContract1_, address newContract2_)
    {
        // Preamble needs 3 bytes.
        uint256 maxInitCodeSize = 24576 - 3;
        require(_bytecode.length > maxInitCodeSize, "DeployImplementations: Use deployBytecode instead");

        bytes memory part1Slice = Bytes.slice(_bytecode, 0, maxInitCodeSize);
        bytes memory part1 = Blueprint.blueprintDeployerBytecode(part1Slice);
        bytes memory part2Slice = Bytes.slice(_bytecode, maxInitCodeSize, _bytecode.length - maxInitCodeSize);
        bytes memory part2 = Blueprint.blueprintDeployerBytecode(part2Slice);

        newContract1_ = deployBytecode(part1, _salt);
        newContract2_ = deployBytecode(part2, _salt);
    }

    // Zero address is returned if the address is not found in '_standardVersionsToml'.
    function getReleaseAddress(
        string memory _version,
        string memory _contractName,
        string memory _standardVersionsToml
    )
        internal
        pure
        returns (address addr_)
    {
        string memory baseKey = string.concat('.releases["', _version, '"].', _contractName);
        string memory implAddressKey = string.concat(baseKey, ".implementation_address");
        string memory addressKey = string.concat(baseKey, ".address");
        try vm.parseTomlAddress(_standardVersionsToml, implAddressKey) returns (address parsedAddr_) {
            addr_ = parsedAddr_;
        } catch {
            try vm.parseTomlAddress(_standardVersionsToml, addressKey) returns (address parsedAddr_) {
                addr_ = parsedAddr_;
            } catch {
                addr_ = address(0);
            }
        }
    }
}

// Similar to how DeploySuperchain.s.sol contains a lot of comments to thoroughly document the script
// architecture, this comment block documents how to update the deploy scripts to support new features.
//
// Using the base scripts and contracts (DeploySuperchain, DeployImplementations, DeployOPChain, and
// the corresponding OPContractsManager) deploys a standard chain. For nonstandard and in-development
// features we need to modify some or all of those contracts, and we do that via inheritance. Using
// interop as an example, they've made the following changes to L1 contracts:
//   - `OptimismPortalInterop is OptimismPortal`: A different portal implementation is used, and
//     it's ABI is the same.
//   - `SystemConfigInterop is SystemConfig`: A different system config implementation is used, and
//     it's initializer has a different signature. This signature is different because there is a
//     new input parameter, the `dependencyManager`.
//   - Because of the different system config initializer, there is a new input parameter (dependencyManager).
//
// Similar to how inheritance was used to develop the new portal and system config contracts, we use
// inheritance to modify up to all of the deployer contracts. For this interop example, what this
// means is we need:
//   - An `OPContractsManagerInterop is OPContractsManager` that knows how to encode the calldata for the
//     new system config initializer.
//   - A `DeployImplementationsInterop is DeployImplementations` that:
//     - Deploys OptimismPortalInterop instead of OptimismPortal.
//     - Deploys SystemConfigInterop instead of SystemConfig.
//     - Deploys OPContractsManagerInterop instead of OPContractsManager, which contains the updated logic
//       for encoding the SystemConfig initializer.
//     - Updates the OPCM release setter logic to use the updated initializer.
//  - A `DeployOPChainInterop is DeployOPChain` that allows the updated input parameter to be passed.
//
// Most of the complexity in the above flow comes from the the new input for the updated SystemConfig
// initializer. If all function signatures were the same, all we'd have to change is the contract
// implementations that are deployed then set in the OPCM. For now, to simplify things until we
// resolve https://github.com/ethereum-optimism/optimism/issues/11783, we just assume this new role
// is the same as the proxy admin owner.
contract DeployImplementationsInterop is DeployImplementations {
    function createOPCMContract(
        DeployImplementationsInput _dii,
        DeployImplementationsOutput _dio,
        OPContractsManager.Blueprints memory _blueprints,
        string memory _l1ContractsRelease
    )
        internal
        virtual
        override
        returns (OPContractsManager opcm_)
    {
        ISuperchainConfig superchainConfigProxy = _dii.superchainConfigProxy();
        IProtocolVersions protocolVersionsProxy = _dii.protocolVersionsProxy();

        OPContractsManager.Implementations memory implementations = OPContractsManager.Implementations({
            l1ERC721BridgeImpl: address(_dio.l1ERC721BridgeImpl()),
            optimismPortalImpl: address(_dio.optimismPortalImpl()),
            systemConfigImpl: address(_dio.systemConfigImpl()),
            optimismMintableERC20FactoryImpl: address(_dio.optimismMintableERC20FactoryImpl()),
            l1CrossDomainMessengerImpl: address(_dio.l1CrossDomainMessengerImpl()),
            l1StandardBridgeImpl: address(_dio.l1StandardBridgeImpl()),
            disputeGameFactoryImpl: address(_dio.disputeGameFactoryImpl()),
            delayedWETHImpl: address(_dio.delayedWETHImpl()),
            mipsImpl: address(_dio.mipsSingleton())
        });

        vm.broadcast(msg.sender);
        opcm_ = new OPContractsManagerInterop(
            superchainConfigProxy, protocolVersionsProxy, _l1ContractsRelease, _blueprints, implementations
        );

        vm.label(address(opcm_), "OPContractsManager");
        _dio.set(_dio.opcm.selector, address(opcm_));
    }

    function deployOptimismPortalImpl(
        DeployImplementationsInput _dii,
        DeployImplementationsOutput _dio
    )
        public
        override
    {
        string memory release = _dii.l1ContractsRelease();
        string memory stdVerToml = _dii.standardVersionsToml();
        string memory contractName = "optimism_portal";
        IOptimismPortalInterop impl;

        address existingImplementation = getReleaseAddress(release, contractName, stdVerToml);
        if (existingImplementation != address(0)) {
            impl = IOptimismPortalInterop(payable(existingImplementation));
        } else {
            uint256 proofMaturityDelaySeconds = _dii.proofMaturityDelaySeconds();
            uint256 disputeGameFinalityDelaySeconds = _dii.disputeGameFinalityDelaySeconds();
            vm.broadcast(msg.sender);
            impl = IOptimismPortalInterop(
                DeployUtils.create1({
                    _name: "OptimismPortalInterop",
                    _args: DeployUtils.encodeConstructor(
                        abi.encodeCall(
                            IOptimismPortalInterop.__constructor__,
                            (proofMaturityDelaySeconds, disputeGameFinalityDelaySeconds)
                        )
                    )
                })
            );
        }

        vm.label(address(impl), "OptimismPortalImpl");
        _dio.set(_dio.optimismPortalImpl.selector, address(impl));
    }

    function deploySystemConfigImpl(
        DeployImplementationsInput _dii,
        DeployImplementationsOutput _dio
    )
        public
        override
    {
        string memory release = _dii.l1ContractsRelease();
        string memory stdVerToml = _dii.standardVersionsToml();

        string memory contractName = "system_config";
        ISystemConfigInterop impl;

        address existingImplementation = getReleaseAddress(release, contractName, stdVerToml);
        if (existingImplementation != address(0)) {
            impl = ISystemConfigInterop(existingImplementation);
        } else {
            vm.broadcast(msg.sender);
            impl = ISystemConfigInterop(
                DeployUtils.create1({
                    _name: "SystemConfigInterop",
                    _args: DeployUtils.encodeConstructor(abi.encodeCall(ISystemConfigInterop.__constructor__, ()))
                })
            );
        }

        vm.label(address(impl), "SystemConfigImpl");
        _dio.set(_dio.systemConfigImpl.selector, address(impl));
    }
}
