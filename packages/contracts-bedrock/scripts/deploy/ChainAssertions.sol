// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

// Testing
import { Vm } from "forge-std/Vm.sol";
import { console2 as console } from "forge-std/console2.sol";

// Scripts
import { DeployConfig } from "scripts/deploy/DeployConfig.s.sol";
import { ISystemConfigInterop } from "src/L1/interfaces/ISystemConfigInterop.sol";

// Libraries
import { Constants } from "src/libraries/Constants.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";
import { Types } from "scripts/libraries/Types.sol";

// Interfaces
import { IResourceMetering } from "src/L1/interfaces/IResourceMetering.sol";
import { ISystemConfig } from "src/L1/interfaces/ISystemConfig.sol";
import { IL2OutputOracle } from "src/L1/interfaces/IL2OutputOracle.sol";
import { ISuperchainConfig } from "src/L1/interfaces/ISuperchainConfig.sol";
import { IL1CrossDomainMessenger } from "src/L1/interfaces/IL1CrossDomainMessenger.sol";
import { IOptimismPortal } from "src/L1/interfaces/IOptimismPortal.sol";
import { IOptimismPortal2 } from "src/L1/interfaces/IOptimismPortal2.sol";
import { IL1ERC721Bridge } from "src/L1/interfaces/IL1ERC721Bridge.sol";
import { IL1StandardBridge } from "src/L1/interfaces/IL1StandardBridge.sol";
import { ProtocolVersion, IProtocolVersions } from "src/L1/interfaces/IProtocolVersions.sol";
import { IDisputeGameFactory } from "src/dispute/interfaces/IDisputeGameFactory.sol";
import { IDelayedWETH } from "src/dispute/interfaces/IDelayedWETH.sol";
import { IOptimismMintableERC20Factory } from "src/universal/interfaces/IOptimismMintableERC20Factory.sol";
import { IPreimageOracle } from "src/cannon/interfaces/IPreimageOracle.sol";
import { IMIPS } from "src/cannon/interfaces/IMIPS.sol";
import { OPContractsManager } from "src/L1/OPContractsManager.sol";

library ChainAssertions {
    Vm internal constant vm = Vm(0x7109709ECfa91a80626fF3989D68f67F5b1DD12D);

    /// @notice Asserts the correctness of an L1 deployment. This function expects that all contracts
    ///         within the `prox` ContractSet are proxies that have been setup and initialized.
    function postDeployAssertions(
        Types.ContractSet memory _prox,
        DeployConfig _cfg,
        uint256 _l2OutputOracleStartingTimestamp,
        Vm _vm
    )
        internal
        view
    {
        console.log("Running post-deploy assertions");
        IResourceMetering.ResourceConfig memory rcfg = ISystemConfig(_prox.SystemConfig).resourceConfig();
        IResourceMetering.ResourceConfig memory dflt = Constants.DEFAULT_RESOURCE_CONFIG();
        require(keccak256(abi.encode(rcfg)) == keccak256(abi.encode(dflt)));

        checkSystemConfig({ _contracts: _prox, _cfg: _cfg, _isProxy: true });
        checkL1CrossDomainMessenger({ _contracts: _prox, _vm: _vm, _isProxy: true });
        checkL1StandardBridge({ _contracts: _prox, _isProxy: true });
        checkL2OutputOracle({
            _contracts: _prox,
            _cfg: _cfg,
            _l2OutputOracleStartingTimestamp: _l2OutputOracleStartingTimestamp,
            _isProxy: true
        });
        checkOptimismMintableERC20Factory({ _contracts: _prox, _isProxy: true });
        checkL1ERC721Bridge({ _contracts: _prox, _isProxy: true });
        checkOptimismPortal({ _contracts: _prox, _cfg: _cfg, _isProxy: true });
        checkOptimismPortal2({ _contracts: _prox, _cfg: _cfg, _isProxy: true });
        checkProtocolVersions({ _contracts: _prox, _cfg: _cfg, _isProxy: true });
    }

    /// @notice Asserts that the SystemConfig is setup correctly
    function checkSystemConfig(Types.ContractSet memory _contracts, DeployConfig _cfg, bool _isProxy) internal view {
        ISystemConfig config = ISystemConfig(_contracts.SystemConfig);
        console.log(
            "Running chain assertions on the SystemConfig %s at %s",
            _isProxy ? "proxy" : "implementation",
            address(config)
        );

        // Check that the contract is initialized
        assertInitializedSlotIsSet({ _contractAddress: address(config), _slot: 0, _offset: 0 });

        IResourceMetering.ResourceConfig memory resourceConfig = config.resourceConfig();

        if (_isProxy) {
            require(config.owner() == _cfg.finalSystemOwner(), "CHECK-SCFG-10");
            require(config.basefeeScalar() == _cfg.basefeeScalar(), "CHECK-SCFG-20");
            require(config.blobbasefeeScalar() == _cfg.blobbasefeeScalar(), "CHECK-SCFG-30");
            require(config.batcherHash() == bytes32(uint256(uint160(_cfg.batchSenderAddress()))), "CHECK-SCFG-40");
            require(config.gasLimit() == uint64(_cfg.l2GenesisBlockGasLimit()), "CHECK-SCFG-50");
            require(config.unsafeBlockSigner() == _cfg.p2pSequencerAddress(), "CHECK-SCFG-60");
            require(config.scalar() >> 248 == 1, "CHECK-SCFG-70");
            // Check _config
            IResourceMetering.ResourceConfig memory rconfig = Constants.DEFAULT_RESOURCE_CONFIG();
            require(resourceConfig.maxResourceLimit == rconfig.maxResourceLimit, "CHECK-SCFG-80");
            require(resourceConfig.elasticityMultiplier == rconfig.elasticityMultiplier, "CHECK-SCFG-90");
            require(resourceConfig.baseFeeMaxChangeDenominator == rconfig.baseFeeMaxChangeDenominator, "CHECK-SCFG-100");
            require(resourceConfig.systemTxMaxGas == rconfig.systemTxMaxGas, "CHECK-SCFG-110");
            require(resourceConfig.minimumBaseFee == rconfig.minimumBaseFee, "CHECK-SCFG-120");
            require(resourceConfig.maximumBaseFee == rconfig.maximumBaseFee, "CHECK-SCFG-130");
            // Depends on start block being set to 0 in `initialize`
            uint256 cfgStartBlock = _cfg.systemConfigStartBlock();
            require(config.startBlock() == (cfgStartBlock == 0 ? block.number : cfgStartBlock), "CHECK-SCFG-140");
            require(config.batchInbox() == _cfg.batchInboxAddress(), "CHECK-SCFG-150");
            // Check _addresses
            require(config.l1CrossDomainMessenger() == _contracts.L1CrossDomainMessenger, "CHECK-SCFG-160");
            require(config.l1ERC721Bridge() == _contracts.L1ERC721Bridge, "CHECK-SCFG-170");
            require(config.l1StandardBridge() == _contracts.L1StandardBridge, "CHECK-SCFG-180");
            require(config.disputeGameFactory() == _contracts.DisputeGameFactory, "CHECK-SCFG-190");
            require(config.optimismPortal() == _contracts.OptimismPortal, "CHECK-SCFG-200");
            require(config.optimismMintableERC20Factory() == _contracts.OptimismMintableERC20Factory, "CHECK-SCFG-210");
        } else {
            require(config.owner() == address(0xdead), "CHECK-SCFG-220");
            require(config.overhead() == 0, "CHECK-SCFG-230");
            require(config.scalar() == uint256(0x01) << 248, "CHECK-SCFG-240"); // version 1
            require(config.basefeeScalar() == 0, "CHECK-SCFG-250");
            require(config.blobbasefeeScalar() == 0, "CHECK-SCFG-260");
            require(config.batcherHash() == bytes32(0), "CHECK-SCFG-270");
            require(config.gasLimit() == 1, "CHECK-SCFG-280");
            require(config.unsafeBlockSigner() == address(0), "CHECK-SCFG-290");
            // Check _config
            require(resourceConfig.maxResourceLimit == 1, "CHECK-SCFG-300");
            require(resourceConfig.elasticityMultiplier == 1, "CHECK-SCFG-310");
            require(resourceConfig.baseFeeMaxChangeDenominator == 2, "CHECK-SCFG-320");
            require(resourceConfig.systemTxMaxGas == 0, "CHECK-SCFG-330");
            require(resourceConfig.minimumBaseFee == 0, "CHECK-SCFG-340");
            require(resourceConfig.maximumBaseFee == 0, "CHECK-SCFG-350");
            // Check _addresses
            require(config.startBlock() == type(uint256).max, "CHECK-SCFG-360");
            require(config.batchInbox() == address(0), "CHECK-SCFG-370");
            require(config.l1CrossDomainMessenger() == address(0), "CHECK-SCFG-380");
            require(config.l1ERC721Bridge() == address(0), "CHECK-SCFG-390");
            require(config.l1StandardBridge() == address(0), "CHECK-SCFG-400");
            require(config.disputeGameFactory() == address(0), "CHECK-SCFG-410");
            require(config.optimismPortal() == address(0), "CHECK-SCFG-420");
            require(config.optimismMintableERC20Factory() == address(0), "CHECK-SCFG-430");
        }
    }

    /// @notice Asserts that the SystemConfigInterop is setup correctly
    function checkSystemConfigInterop(
        Types.ContractSet memory _contracts,
        DeployConfig _cfg,
        bool _isProxy
    )
        internal
        view
    {
        ISystemConfigInterop config = ISystemConfigInterop(_contracts.SystemConfig);
        console.log(
            "Running chain assertions on the SystemConfigInterop %s at %s",
            _isProxy ? "proxy" : "implementation",
            address(config)
        );

        checkSystemConfig(_contracts, _cfg, _isProxy);
        if (_isProxy) {
            // TODO: this is not being set in the deployment, nor is a config value.
            // Update this when it has an entry in hardhat.json
            require(config.dependencyManager() == address(0), "CHECK-SCFGI-10");
        } else {
            require(config.dependencyManager() == address(0), "CHECK-SCFGI-20");
        }
    }

    /// @notice Asserts that the L1CrossDomainMessenger is setup correctly
    function checkL1CrossDomainMessenger(Types.ContractSet memory _contracts, Vm _vm, bool _isProxy) internal view {
        IL1CrossDomainMessenger messenger = IL1CrossDomainMessenger(_contracts.L1CrossDomainMessenger);
        console.log(
            "Running chain assertions on the L1CrossDomainMessenger %s at %s",
            _isProxy ? "proxy" : "implementation",
            address(messenger)
        );
        require(address(messenger) != address(0), "CHECK-L1XDM-10");

        // Check that the contract is initialized
        assertInitializedSlotIsSet({ _contractAddress: address(messenger), _slot: 0, _offset: 20 });

        require(address(messenger.OTHER_MESSENGER()) == Predeploys.L2_CROSS_DOMAIN_MESSENGER, "CHECK-L1XDM-20");
        require(address(messenger.otherMessenger()) == Predeploys.L2_CROSS_DOMAIN_MESSENGER, "CHECK-L1XDM-30");

        if (_isProxy) {
            require(address(messenger.PORTAL()) == _contracts.OptimismPortal, "CHECK-L1XDM-40");
            require(address(messenger.portal()) == _contracts.OptimismPortal, "CHECK-L1XDM-50");
            require(address(messenger.superchainConfig()) == _contracts.SuperchainConfig, "CHECK-L1XDM-60");
            bytes32 xdmSenderSlot = _vm.load(address(messenger), bytes32(uint256(204)));
            require(address(uint160(uint256(xdmSenderSlot))) == Constants.DEFAULT_L2_SENDER, "CHECK-L1XDM-70");
        } else {
            require(address(messenger.PORTAL()) == address(0), "CHECK-L1XDM-80");
            require(address(messenger.portal()) == address(0), "CHECK-L1XDM-90");
            require(address(messenger.superchainConfig()) == address(0), "CHECK-L1XDM-100");
        }
    }

    /// @notice Asserts that the L1StandardBridge is setup correctly
    function checkL1StandardBridge(Types.ContractSet memory _contracts, bool _isProxy) internal view {
        IL1StandardBridge bridge = IL1StandardBridge(payable(_contracts.L1StandardBridge));
        console.log(
            "Running chain assertions on the L1StandardBridge %s at %s",
            _isProxy ? "proxy" : "implementation",
            address(bridge)
        );
        require(address(bridge) != address(0), "CHECK-L1SB-10");

        // Check that the contract is initialized
        assertInitializedSlotIsSet({ _contractAddress: address(bridge), _slot: 0, _offset: 0 });

        if (_isProxy) {
            require(address(bridge.MESSENGER()) == _contracts.L1CrossDomainMessenger, "CHECK-L1SB-20");
            require(address(bridge.messenger()) == _contracts.L1CrossDomainMessenger, "CHECK-L1SB-30");
            require(address(bridge.OTHER_BRIDGE()) == Predeploys.L2_STANDARD_BRIDGE, "CHECK-L1SB-40");
            require(address(bridge.otherBridge()) == Predeploys.L2_STANDARD_BRIDGE, "CHECK-L1SB-50");
            require(address(bridge.superchainConfig()) == _contracts.SuperchainConfig, "CHECK-L1SB-60");
        } else {
            require(address(bridge.MESSENGER()) == address(0), "CHECK-L1SB-70");
            require(address(bridge.messenger()) == address(0), "CHECK-L1SB-80");
            require(address(bridge.OTHER_BRIDGE()) == Predeploys.L2_STANDARD_BRIDGE, "CHECK-L1SB-90");
            require(address(bridge.otherBridge()) == Predeploys.L2_STANDARD_BRIDGE, "CHECK-L1SB-100");
            require(address(bridge.superchainConfig()) == address(0), "CHECK-L1SB-110");
        }
    }

    /// @notice Asserts that the DisputeGameFactory is setup correctly
    function checkDisputeGameFactory(
        Types.ContractSet memory _contracts,
        address _expectedOwner,
        bool _isProxy
    )
        internal
        view
    {
        IDisputeGameFactory factory = IDisputeGameFactory(_contracts.DisputeGameFactory);
        console.log(
            "Running chain assertions on the DisputeGameFactory %s at %s",
            _isProxy ? "proxy" : "implementation",
            address(factory)
        );
        require(address(factory) != address(0), "CHECK-DG-10");

        // Check that the contract is initialized
        assertInitializedSlotIsSet({ _contractAddress: address(factory), _slot: 0, _offset: 0 });

        // The same check is made for both proxy and implementation
        require(factory.owner() == _expectedOwner, "CHECK-DG-20");
    }

    /// @notice Asserts that the PreimageOracle is setup correctly
    function checkPreimageOracle(IPreimageOracle _oracle, DeployConfig _cfg) internal view {
        console.log("Running chain assertions on the PreimageOracle %s at %s", address(_oracle));
        require(address(_oracle) != address(0), "CHECK-PIO-10");

        require(_oracle.minProposalSize() == _cfg.preimageOracleMinProposalSize(), "CHECK-PIO-30");
        require(_oracle.challengePeriod() == _cfg.preimageOracleChallengePeriod(), "CHECK-PIO-40");
    }

    /// @notice Asserts that the MIPs contract is setup correctly
    function checkMIPS(IMIPS _mips, IPreimageOracle _oracle) internal view {
        console.log("Running chain assertions on the MIPS %s at %s", address(_mips));
        require(address(_mips) != address(0), "CHECK-MIPS-10");

        require(_mips.oracle() == _oracle, "CHECK-MIPS-20");
    }

    /// @notice Asserts that the DelayedWETH is setup correctly
    function checkDelayedWETH(
        Types.ContractSet memory _contracts,
        DeployConfig _cfg,
        bool _isProxy,
        address _expectedOwner
    )
        internal
        view
    {
        IDelayedWETH weth = IDelayedWETH(payable(_contracts.DelayedWETH));
        console.log(
            "Running chain assertions on the DelayedWETH %s at %s", _isProxy ? "proxy" : "implementation", address(weth)
        );
        require(address(weth) != address(0), "CHECK-DWETH-10");

        // Check that the contract is initialized
        assertInitializedSlotIsSet({ _contractAddress: address(weth), _slot: 0, _offset: 0 });

        if (_isProxy) {
            require(weth.owner() == _expectedOwner, "CHECK-DWETH-20");
            require(weth.delay() == _cfg.faultGameWithdrawalDelay(), "CHECK-DWETH-30");
            require(weth.config() == ISuperchainConfig(_contracts.SuperchainConfig), "CHECK-DWETH-40");
        } else {
            require(weth.owner() == _expectedOwner, "CHECK-DWETH-50");
            require(weth.delay() == _cfg.faultGameWithdrawalDelay(), "CHECK-DWETH-60");
        }
    }

    /// @notice Asserts that the permissioned DelayedWETH is setup correctly
    function checkPermissionedDelayedWETH(
        Types.ContractSet memory _contracts,
        DeployConfig _cfg,
        bool _isProxy,
        address _expectedOwner
    )
        internal
        view
    {
        IDelayedWETH weth = IDelayedWETH(payable(_contracts.PermissionedDelayedWETH));
        console.log(
            "Running chain assertions on the PermissionedDelayedWETH %s at %s",
            _isProxy ? "proxy" : "implementation",
            address(weth)
        );
        require(address(weth) != address(0), "CHECK-PDWETH-10");

        // Check that the contract is initialized
        assertInitializedSlotIsSet({ _contractAddress: address(weth), _slot: 0, _offset: 0 });

        if (_isProxy) {
            require(weth.owner() == _expectedOwner, "CHECK-PDWETH-20");
            require(weth.delay() == _cfg.faultGameWithdrawalDelay(), "CHECK-PDWETH-30");
            require(weth.config() == ISuperchainConfig(_contracts.SuperchainConfig), "CHECK-PDWETH-40");
        } else {
            require(weth.owner() == _expectedOwner, "CHECK-PDWETH-50");
            require(weth.delay() == _cfg.faultGameWithdrawalDelay(), "CHECK-PDWETH-60");
        }
    }

    /// @notice Asserts that the L2OutputOracle is setup correctly
    function checkL2OutputOracle(
        Types.ContractSet memory _contracts,
        DeployConfig _cfg,
        uint256 _l2OutputOracleStartingTimestamp,
        bool _isProxy
    )
        internal
        view
    {
        IL2OutputOracle oracle = IL2OutputOracle(_contracts.L2OutputOracle);
        console.log(
            "Running chain assertions on the L2OutputOracle %s at %s",
            _isProxy ? "proxy" : "implementation",
            address(oracle)
        );
        require(address(oracle) != address(0), "CHECK-L2OO-10");

        // Check that the contract is initialized
        assertInitializedSlotIsSet({ _contractAddress: address(oracle), _slot: 0, _offset: 0 });

        if (_isProxy) {
            require(oracle.SUBMISSION_INTERVAL() == _cfg.l2OutputOracleSubmissionInterval(), "CHECK-L2OO-20");
            require(oracle.submissionInterval() == _cfg.l2OutputOracleSubmissionInterval(), "CHECK-L2OO-30");
            require(oracle.L2_BLOCK_TIME() == _cfg.l2BlockTime(), "CHECK-L2OO-40");
            require(oracle.l2BlockTime() == _cfg.l2BlockTime(), "CHECK-L2OO-50");
            require(oracle.PROPOSER() == _cfg.l2OutputOracleProposer(), "CHECK-L2OO-60");
            require(oracle.proposer() == _cfg.l2OutputOracleProposer(), "CHECK-L2OO-70");
            require(oracle.CHALLENGER() == _cfg.l2OutputOracleChallenger(), "CHECK-L2OO-80");
            require(oracle.challenger() == _cfg.l2OutputOracleChallenger(), "CHECK-L2OO-90");
            require(oracle.FINALIZATION_PERIOD_SECONDS() == _cfg.finalizationPeriodSeconds(), "CHECK-L2OO-100");
            require(oracle.finalizationPeriodSeconds() == _cfg.finalizationPeriodSeconds(), "CHECK-L2OO-110");
            require(oracle.startingBlockNumber() == _cfg.l2OutputOracleStartingBlockNumber(), "CHECK-L2OO-120");
            require(oracle.startingTimestamp() == _l2OutputOracleStartingTimestamp, "CHECK-L2OO-130");
        } else {
            require(oracle.SUBMISSION_INTERVAL() == 1, "CHECK-L2OO-140");
            require(oracle.submissionInterval() == 1, "CHECK-L2OO-150");
            require(oracle.L2_BLOCK_TIME() == 1, "CHECK-L2OO-160");
            require(oracle.l2BlockTime() == 1, "CHECK-L2OO-170");
            require(oracle.PROPOSER() == address(0), "CHECK-L2OO-180");
            require(oracle.proposer() == address(0), "CHECK-L2OO-190");
            require(oracle.CHALLENGER() == address(0), "CHECK-L2OO-200");
            require(oracle.challenger() == address(0), "CHECK-L2OO-210");
            require(oracle.FINALIZATION_PERIOD_SECONDS() == 0, "CHECK-L2OO-220");
            require(oracle.finalizationPeriodSeconds() == 0, "CHECK-L2OO-230");
            require(oracle.startingBlockNumber() == 0, "CHECK-L2OO-240");
            require(oracle.startingTimestamp() == 0, "CHECK-L2OO-250");
        }
    }

    /// @notice Asserts that the OptimismMintableERC20Factory is setup correctly
    function checkOptimismMintableERC20Factory(Types.ContractSet memory _contracts, bool _isProxy) internal view {
        IOptimismMintableERC20Factory factory = IOptimismMintableERC20Factory(_contracts.OptimismMintableERC20Factory);
        console.log(
            "Running chain assertions on the OptimismMintableERC20Factory %s at %s",
            _isProxy ? "proxy" : "implementation",
            address(factory)
        );
        require(address(factory) != address(0), "CHECK-MERC20F-10");

        // Check that the contract is initialized
        assertInitializedSlotIsSet({ _contractAddress: address(factory), _slot: 0, _offset: 0 });

        if (_isProxy) {
            require(factory.BRIDGE() == _contracts.L1StandardBridge, "CHECK-MERC20F-10");
            require(factory.bridge() == _contracts.L1StandardBridge, "CHECK-MERC20F-20");
        } else {
            require(factory.BRIDGE() == address(0), "CHECK-MERC20F-30");
            require(factory.bridge() == address(0), "CHECK-MERC20F-40");
        }
    }

    /// @notice Asserts that the L1ERC721Bridge is setup correctly
    function checkL1ERC721Bridge(Types.ContractSet memory _contracts, bool _isProxy) internal view {
        console.log("Running chain assertions on the L1ERC721Bridge");
        IL1ERC721Bridge bridge = IL1ERC721Bridge(_contracts.L1ERC721Bridge);
        console.log(
            "Running chain assertions on the L1ERC721Bridge %s at %s",
            _isProxy ? "proxy" : "implementation",
            address(bridge)
        );
        require(address(bridge) != address(0), "CHECK-L1ERC721B-10");

        // Check that the contract is initialized
        assertInitializedSlotIsSet({ _contractAddress: address(bridge), _slot: 0, _offset: 0 });

        require(address(bridge.OTHER_BRIDGE()) == Predeploys.L2_ERC721_BRIDGE, "CHECK-L1ERC721B-10");
        require(address(bridge.otherBridge()) == Predeploys.L2_ERC721_BRIDGE, "CHECK-L1ERC721B-20");

        if (_isProxy) {
            require(address(bridge.MESSENGER()) == _contracts.L1CrossDomainMessenger, "CHECK-L1ERC721B-30");
            require(address(bridge.messenger()) == _contracts.L1CrossDomainMessenger, "CHECK-L1ERC721B-40");
            require(address(bridge.superchainConfig()) == _contracts.SuperchainConfig, "CHECK-L1ERC721B-50");
        } else {
            require(address(bridge.MESSENGER()) == address(0), "CHECK-L1ERC721B-60");
            require(address(bridge.messenger()) == address(0), "CHECK-L1ERC721B-70");
            require(address(bridge.superchainConfig()) == address(0), "CHECK-L1ERC721B-80");
        }
    }

    /// @notice Asserts the OptimismPortal is setup correctly
    function checkOptimismPortal(Types.ContractSet memory _contracts, DeployConfig _cfg, bool _isProxy) internal view {
        IOptimismPortal portal = IOptimismPortal(payable(_contracts.OptimismPortal));
        console.log(
            "Running chain assertions on the OptimismPortal %s at %s",
            _isProxy ? "proxy" : "implementation",
            address(portal)
        );
        require(address(portal) != address(0), "CHECK-OP-10");

        // Check that the contract is initialized
        assertInitializedSlotIsSet({ _contractAddress: address(portal), _slot: 0, _offset: 0 });

        address guardian = _cfg.superchainConfigGuardian();
        if (guardian.code.length == 0) {
            console.log("Guardian has no code: %s", guardian);
        }

        if (_isProxy) {
            require(address(portal.l2Oracle()) == _contracts.L2OutputOracle, "CHECK-OP-20");
            require(address(portal.systemConfig()) == _contracts.SystemConfig, "CHECK-OP-30");
            require(portal.guardian() == guardian, "CHECK-OP-40");
            require(address(portal.superchainConfig()) == address(_contracts.SuperchainConfig), "CHECK-OP-50");
            require(portal.paused() == ISuperchainConfig(_contracts.SuperchainConfig).paused(), "CHECK-OP-60");
            require(portal.l2Sender() == Constants.DEFAULT_L2_SENDER, "CHECK-OP-70");
        } else {
            require(address(portal.l2Oracle()) == address(0), "CHECK-OP-80");
            require(address(portal.systemConfig()) == address(0), "CHECK-OP-90");
            require(address(portal.superchainConfig()) == address(0), "CHECK-OP-100");
            require(portal.l2Sender() == Constants.DEFAULT_L2_SENDER, "CHECK-OP-110");
        }
    }

    /// @notice Asserts the OptimismPortal2 is setup correctly
    function checkOptimismPortal2(
        Types.ContractSet memory _contracts,
        DeployConfig _cfg,
        bool _isProxy
    )
        internal
        view
    {
        IOptimismPortal2 portal = IOptimismPortal2(payable(_contracts.OptimismPortal2));
        console.log(
            "Running chain assertions on the OptimismPortal2 %s at %s",
            _isProxy ? "proxy" : "implementation",
            address(portal)
        );
        require(address(portal) != address(0), "CHECK-OP2-10");

        // Check that the contract is initialized
        assertInitializedSlotIsSet({ _contractAddress: address(portal), _slot: 0, _offset: 0 });

        address guardian = _cfg.superchainConfigGuardian();
        if (guardian.code.length == 0) {
            console.log("Guardian has no code: %s", guardian);
        }

        if (_isProxy) {
            require(address(portal.disputeGameFactory()) == _contracts.DisputeGameFactory, "CHECK-OP2-20");
            require(address(portal.systemConfig()) == _contracts.SystemConfig, "CHECK-OP2-30");
            require(portal.guardian() == guardian, "CHECK-OP2-40");
            require(address(portal.superchainConfig()) == address(_contracts.SuperchainConfig), "CHECK-OP2-50");
            require(portal.paused() == ISuperchainConfig(_contracts.SuperchainConfig).paused(), "CHECK-OP2-60");
            require(portal.l2Sender() == Constants.DEFAULT_L2_SENDER, "CHECK-OP2-70");
        } else {
            require(address(portal.disputeGameFactory()) == address(0), "CHECK-OP2-80");
            require(address(portal.systemConfig()) == address(0), "CHECK-OP2-90");
            require(address(portal.superchainConfig()) == address(0), "CHECK-OP2-100");
            require(portal.l2Sender() == Constants.DEFAULT_L2_SENDER, "CHECK-OP2-110");
        }
        // This slot is the custom gas token _balance and this check ensures
        // that it stays unset for forwards compatibility with custom gas token.
        require(vm.load(address(portal), bytes32(uint256(61))) == bytes32(0), "CHECK-OP2-120");
    }

    /// @notice Asserts that the ProtocolVersions is setup correctly
    function checkProtocolVersions(
        Types.ContractSet memory _contracts,
        DeployConfig _cfg,
        bool _isProxy
    )
        internal
        view
    {
        IProtocolVersions versions = IProtocolVersions(_contracts.ProtocolVersions);
        console.log(
            "Running chain assertions on the ProtocolVersions %s at %s",
            _isProxy ? "proxy" : "implementation",
            address(versions)
        );
        require(address(versions) != address(0), "CHECK-PV-10");

        // Check that the contract is initialized
        assertInitializedSlotIsSet({ _contractAddress: address(versions), _slot: 0, _offset: 0 });

        if (_isProxy) {
            require(versions.owner() == _cfg.finalSystemOwner(), "CHECK-PV-20");
            require(ProtocolVersion.unwrap(versions.required()) == _cfg.requiredProtocolVersion(), "CHECK-PV-30");
            require(ProtocolVersion.unwrap(versions.recommended()) == _cfg.recommendedProtocolVersion(), "CHECK-PV-40");
        } else {
            require(versions.owner() == address(0xdead), "CHECK-PV-50");
            require(ProtocolVersion.unwrap(versions.required()) == 0, "CHECK-PV-60");
            require(ProtocolVersion.unwrap(versions.recommended()) == 0, "CHECK-PV-70");
        }
    }

    /// @notice Asserts that the SuperchainConfig is setup correctly
    function checkSuperchainConfig(
        Types.ContractSet memory _contracts,
        DeployConfig _cfg,
        bool _isPaused,
        bool _isProxy
    )
        internal
        view
    {
        ISuperchainConfig superchainConfig = ISuperchainConfig(_contracts.SuperchainConfig);
        console.log(
            "Running chain assertions on the SuperchainConfig %s at %s",
            _isProxy ? "proxy" : "implementation",
            address(superchainConfig)
        );
        require(address(superchainConfig) != address(0), "CHECK-SC-10");

        // Check that the contract is initialized
        assertInitializedSlotIsSet({ _contractAddress: address(superchainConfig), _slot: 0, _offset: 0 });

        if (_isProxy) {
            require(superchainConfig.guardian() == _cfg.superchainConfigGuardian(), "CHECK-SC-20");
            require(superchainConfig.paused() == _isPaused, "CHECK-SC-30");
        } else {
            require(superchainConfig.guardian() == address(0), "CHECK-SC-40");
            require(superchainConfig.paused() == false, "CHECK-SC-50");
        }
    }

    /// @notice Asserts that the SuperchainConfig is setup correctly
    function checkOPContractsManager(Types.ContractSet memory _contracts, bool _isProxy) internal view {
        OPContractsManager opcm = OPContractsManager(_contracts.OPContractsManager);
        console.log(
            "Running chain assertions on the OPContractsManager %s at %s",
            _isProxy ? "proxy" : "implementation",
            address(opcm)
        );
        require(address(opcm) != address(0), "CHECK-OPCM-10");

        // Check that the contract is initialized
        assertInitializedSlotIsSet({ _contractAddress: address(opcm), _slot: 0, _offset: 0 });

        // These values are immutable so are shared by the proxy and implementation
        require(address(opcm.superchainConfig()) == address(_contracts.SuperchainConfig), "CHECK-OPCM-30");
        require(address(opcm.protocolVersions()) == address(_contracts.ProtocolVersions), "CHECK-OPCM-40");

        // TODO: Add assertions for blueprints and setters?
    }

    /// @dev Asserts that for a given contract the value of a storage slot at an offset is 1 or 0xff.
    ///      A call to `initialize` will set it to 1 and a call to _disableInitializers will set it to 0xff.
    function assertInitializedSlotIsSet(address _contractAddress, uint256 _slot, uint256 _offset) internal view {
        bytes32 slotVal = vm.load(_contractAddress, bytes32(_slot));
        uint8 val = uint8((uint256(slotVal) >> (_offset * 8)) & 0xFF);
        require(
            val == uint8(1) || val == uint8(0xff),
            "ChainAssertions: storage value is not 1 or 0xff at the given slot and offset"
        );
    }
}
