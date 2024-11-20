// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

// Testing
import { CommonTest } from "test/setup/CommonTest.sol";
import { console2 as console } from "forge-std/console2.sol";

// Scripts
import { ForgeArtifacts, Abi, AbiEntry } from "scripts/libraries/ForgeArtifacts.sol";

// Contracts
import { OPContractsManager } from "src/L1/OPContractsManager.sol";

// Interfaces
import { IOptimismPortal } from "src/L1/interfaces/IOptimismPortal.sol";
import { IOptimismPortal2 } from "src/L1/interfaces/IOptimismPortal2.sol";
import { IOptimismPortalInterop } from "src/L1/interfaces/IOptimismPortalInterop.sol";
import { ISystemConfig } from "src/L1/interfaces/ISystemConfig.sol";
import { ISystemConfigInterop } from "src/L1/interfaces/ISystemConfigInterop.sol";
import { IDataAvailabilityChallenge } from "src/L1/interfaces/IDataAvailabilityChallenge.sol";
import { IProtocolVersions } from "src/L1/interfaces/IProtocolVersions.sol";

/// @title Specification_Test
/// @dev Specifies common security properties of entrypoints to L1 contracts, including authorization and
///      pausability.
///      When adding new functions to the L1 system, the `setUp` function must be updated to document the security
///      properties of the new function. The `Spec` struct reppresents this documentation. However, this contract does
///      not actually test to verify these properties, only that a spec is defined.
contract Specification_Test is CommonTest {
    enum Role {
        NOAUTH,
        PROPOSER,
        CHALLENGER,
        SYSTEMCONFIGOWNER,
        GUARDIAN,
        DEPUTYGUARDIAN,
        MESSENGER,
        L1PROXYADMINOWNER,
        GOVERNANCETOKENOWNER,
        MINTMANAGEROWNER,
        DATAAVAILABILITYCHALLENGEOWNER,
        DISPUTEGAMEFACTORYOWNER,
        DELAYEDWETHOWNER,
        COUNCILSAFE,
        COUNCILSAFEOWNER,
        DEPENDENCYMANAGER
    }

    /// @notice Represents the specification of a function.
    /// @custom:field name     Contract name
    /// @custom:field sel      Function selector
    /// @custom:field auth     Specifies authentication as a requirement
    /// @custom:field pausable Specifies that the function is pausable
    struct Spec {
        string name;
        bytes4 sel;
        Role auth;
        bool pausable;
    }

    mapping(string => mapping(bytes4 => Spec)) specs;
    mapping(Role => Spec[]) public specsByRole;
    mapping(string => uint256) public numEntries;
    uint256 numSpecs;

    function setUp() public override {
        super.setUp();

        // DataAvailabilityChallenge
        _addSpec({ _name: "DataAvailabilityChallenge", _sel: _getSel("owner()") });
        _addSpec({
            _name: "DataAvailabilityChallenge",
            _sel: _getSel("renounceOwnership()"),
            _auth: Role.DATAAVAILABILITYCHALLENGEOWNER
        });
        _addSpec({
            _name: "DataAvailabilityChallenge",
            _sel: _getSel("transferOwnership(address)"),
            _auth: Role.DATAAVAILABILITYCHALLENGEOWNER
        });
        _addSpec({ _name: "DataAvailabilityChallenge", _sel: _getSel("version()") });
        _addSpec({ _name: "DataAvailabilityChallenge", _sel: _getSel("fixedResolutionCost()") });
        _addSpec({ _name: "DataAvailabilityChallenge", _sel: _getSel("variableResolutionCost()") });
        _addSpec({ _name: "DataAvailabilityChallenge", _sel: _getSel("variableResolutionCostPrecision()") });
        _addSpec({ _name: "DataAvailabilityChallenge", _sel: _getSel("bondSize()") });
        _addSpec({ _name: "DataAvailabilityChallenge", _sel: _getSel("challengeWindow()") });
        _addSpec({ _name: "DataAvailabilityChallenge", _sel: _getSel("resolveWindow()") });
        _addSpec({ _name: "DataAvailabilityChallenge", _sel: _getSel("resolverRefundPercentage()") });
        _addSpec({ _name: "DataAvailabilityChallenge", _sel: _getSel("balances(address)") });
        _addSpec({ _name: "DataAvailabilityChallenge", _sel: IDataAvailabilityChallenge.initialize.selector });
        _addSpec({
            _name: "DataAvailabilityChallenge",
            _sel: IDataAvailabilityChallenge.setBondSize.selector,
            _auth: Role.DATAAVAILABILITYCHALLENGEOWNER
        });
        _addSpec({
            _name: "DataAvailabilityChallenge",
            _sel: IDataAvailabilityChallenge.setResolverRefundPercentage.selector,
            _auth: Role.DATAAVAILABILITYCHALLENGEOWNER
        });
        _addSpec({ _name: "DataAvailabilityChallenge", _sel: IDataAvailabilityChallenge.deposit.selector });
        _addSpec({ _name: "DataAvailabilityChallenge", _sel: IDataAvailabilityChallenge.withdraw.selector });
        _addSpec({ _name: "DataAvailabilityChallenge", _sel: IDataAvailabilityChallenge.getChallenge.selector });
        _addSpec({ _name: "DataAvailabilityChallenge", _sel: IDataAvailabilityChallenge.getChallengeStatus.selector });
        _addSpec({ _name: "DataAvailabilityChallenge", _sel: IDataAvailabilityChallenge.validateCommitment.selector });
        _addSpec({ _name: "DataAvailabilityChallenge", _sel: IDataAvailabilityChallenge.challenge.selector });
        _addSpec({ _name: "DataAvailabilityChallenge", _sel: IDataAvailabilityChallenge.resolve.selector });
        _addSpec({ _name: "DataAvailabilityChallenge", _sel: IDataAvailabilityChallenge.unlockBond.selector });

        // L1CrossDomainMessenger
        _addSpec({ _name: "L1CrossDomainMessenger", _sel: _getSel("MESSAGE_VERSION()") });
        _addSpec({ _name: "L1CrossDomainMessenger", _sel: _getSel("MIN_GAS_CALLDATA_OVERHEAD()") });
        _addSpec({ _name: "L1CrossDomainMessenger", _sel: _getSel("MIN_GAS_DYNAMIC_OVERHEAD_DENOMINATOR()") });
        _addSpec({ _name: "L1CrossDomainMessenger", _sel: _getSel("MIN_GAS_DYNAMIC_OVERHEAD_NUMERATOR()") });
        _addSpec({ _name: "L1CrossDomainMessenger", _sel: _getSel("OTHER_MESSENGER()") });
        _addSpec({ _name: "L1CrossDomainMessenger", _sel: _getSel("PORTAL()") });
        _addSpec({ _name: "L1CrossDomainMessenger", _sel: _getSel("RELAY_CALL_OVERHEAD()") });
        _addSpec({ _name: "L1CrossDomainMessenger", _sel: _getSel("RELAY_CONSTANT_OVERHEAD()") });
        _addSpec({ _name: "L1CrossDomainMessenger", _sel: _getSel("RELAY_GAS_CHECK_BUFFER()") });
        _addSpec({ _name: "L1CrossDomainMessenger", _sel: _getSel("RELAY_RESERVED_GAS()") });
        _addSpec({ _name: "L1CrossDomainMessenger", _sel: _getSel("baseGas(bytes,uint32)") });
        _addSpec({ _name: "L1CrossDomainMessenger", _sel: _getSel("failedMessages(bytes32)") });
        _addSpec({ _name: "L1CrossDomainMessenger", _sel: _getSel("initialize(address,address,address)") });
        _addSpec({ _name: "L1CrossDomainMessenger", _sel: _getSel("messageNonce()") });
        _addSpec({ _name: "L1CrossDomainMessenger", _sel: _getSel("paused()") });
        _addSpec({ _name: "L1CrossDomainMessenger", _sel: _getSel("otherMessenger()") });
        _addSpec({ _name: "L1CrossDomainMessenger", _sel: _getSel("portal()") });
        _addSpec({ _name: "L1CrossDomainMessenger", _sel: _getSel("systemConfig()") });
        _addSpec({
            _name: "L1CrossDomainMessenger",
            _sel: _getSel("relayMessage(uint256,address,address,uint256,uint256,bytes)"),
            _pausable: true
        });
        _addSpec({ _name: "L1CrossDomainMessenger", _sel: _getSel("sendMessage(address,bytes,uint32)") });
        _addSpec({ _name: "L1CrossDomainMessenger", _sel: _getSel("successfulMessages(bytes32)") });
        _addSpec({ _name: "L1CrossDomainMessenger", _sel: _getSel("superchainConfig()") });
        _addSpec({ _name: "L1CrossDomainMessenger", _sel: _getSel("version()") });
        _addSpec({ _name: "L1CrossDomainMessenger", _sel: _getSel("xDomainMessageSender()") });

        // L1ERC721Bridge
        _addSpec({ _name: "L1ERC721Bridge", _sel: _getSel("MESSENGER()") });
        _addSpec({ _name: "L1ERC721Bridge", _sel: _getSel("OTHER_BRIDGE()") });
        _addSpec({ _name: "L1ERC721Bridge", _sel: _getSel("bridgeERC721(address,address,uint256,uint32,bytes)") });
        _addSpec({
            _name: "L1ERC721Bridge",
            _sel: _getSel("bridgeERC721To(address,address,address,uint256,uint32,bytes)")
        });
        _addSpec({ _name: "L1ERC721Bridge", _sel: _getSel("deposits(address,address,uint256)") });
        _addSpec({
            _name: "L1ERC721Bridge",
            _sel: _getSel("finalizeBridgeERC721(address,address,address,address,uint256,bytes)"),
            _pausable: true
        });
        _addSpec({ _name: "L1ERC721Bridge", _sel: _getSel("messenger()") });
        _addSpec({ _name: "L1ERC721Bridge", _sel: _getSel("otherBridge()") });
        _addSpec({ _name: "L1ERC721Bridge", _sel: _getSel("version()") });
        _addSpec({ _name: "L1ERC721Bridge", _sel: _getSel("superchainConfig()") });
        _addSpec({ _name: "L1ERC721Bridge", _sel: _getSel("paused()") });
        _addSpec({ _name: "L1ERC721Bridge", _sel: _getSel("initialize(address,address)") });

        // L1StandardBridge
        _addSpec({ _name: "L1StandardBridge", _sel: _getSel("MESSENGER()") });
        _addSpec({ _name: "L1StandardBridge", _sel: _getSel("OTHER_BRIDGE()") });
        _addSpec({ _name: "L1StandardBridge", _sel: _getSel("bridgeERC20(address,address,uint256,uint32,bytes)") });
        _addSpec({
            _name: "L1StandardBridge",
            _sel: _getSel("bridgeERC20To(address,address,address,uint256,uint32,bytes)")
        });
        _addSpec({ _name: "L1StandardBridge", _sel: _getSel("bridgeETH(uint32,bytes)") });
        _addSpec({ _name: "L1StandardBridge", _sel: _getSel("bridgeETHTo(address,uint32,bytes)") });
        _addSpec({ _name: "L1StandardBridge", _sel: _getSel("depositERC20(address,address,uint256,uint32,bytes)") });
        _addSpec({
            _name: "L1StandardBridge",
            _sel: _getSel("depositERC20To(address,address,address,uint256,uint32,bytes)")
        });
        _addSpec({ _name: "L1StandardBridge", _sel: _getSel("depositETH(uint32,bytes)") });
        _addSpec({ _name: "L1StandardBridge", _sel: _getSel("depositETHTo(address,uint32,bytes)") });
        _addSpec({ _name: "L1StandardBridge", _sel: _getSel("deposits(address,address)") });
        _addSpec({
            _name: "L1StandardBridge",
            _sel: _getSel("finalizeBridgeERC20(address,address,address,address,uint256,bytes)"),
            _auth: Role.MESSENGER,
            _pausable: true
        });
        _addSpec({
            _name: "L1StandardBridge",
            _sel: _getSel("finalizeBridgeETH(address,address,uint256,bytes)"),
            _auth: Role.MESSENGER,
            _pausable: true
        });
        _addSpec({
            _name: "L1StandardBridge",
            _sel: _getSel("finalizeERC20Withdrawal(address,address,address,address,uint256,bytes)"),
            _auth: Role.MESSENGER,
            _pausable: true
        });
        _addSpec({
            _name: "L1StandardBridge",
            _sel: _getSel("finalizeETHWithdrawal(address,address,uint256,bytes)"),
            _auth: Role.MESSENGER,
            _pausable: true
        });
        _addSpec({ _name: "L1StandardBridge", _sel: _getSel("initialize(address,address,address)") });
        _addSpec({ _name: "L1StandardBridge", _sel: _getSel("l2TokenBridge()") });
        _addSpec({ _name: "L1StandardBridge", _sel: _getSel("messenger()") });
        _addSpec({ _name: "L1StandardBridge", _sel: _getSel("otherBridge()") });
        _addSpec({ _name: "L1StandardBridge", _sel: _getSel("paused()") });
        _addSpec({ _name: "L1StandardBridge", _sel: _getSel("superchainConfig()") });
        _addSpec({ _name: "L1StandardBridge", _sel: _getSel("version()") });
        _addSpec({ _name: "L1StandardBridge", _sel: _getSel("systemConfig()") });

        // L2OutputOracle
        _addSpec({ _name: "L2OutputOracle", _sel: _getSel("CHALLENGER()") });
        _addSpec({ _name: "L2OutputOracle", _sel: _getSel("FINALIZATION_PERIOD_SECONDS()") });
        _addSpec({ _name: "L2OutputOracle", _sel: _getSel("L2_BLOCK_TIME()") });
        _addSpec({ _name: "L2OutputOracle", _sel: _getSel("PROPOSER()") });
        _addSpec({ _name: "L2OutputOracle", _sel: _getSel("SUBMISSION_INTERVAL()") });
        _addSpec({ _name: "L2OutputOracle", _sel: _getSel("challenger()") });
        _addSpec({ _name: "L2OutputOracle", _sel: _getSel("computeL2Timestamp(uint256)") });
        _addSpec({ _name: "L2OutputOracle", _sel: _getSel("deleteL2Outputs(uint256)"), _auth: Role.CHALLENGER });
        _addSpec({ _name: "L2OutputOracle", _sel: _getSel("finalizationPeriodSeconds()") });
        _addSpec({ _name: "L2OutputOracle", _sel: _getSel("getL2Output(uint256)") });
        _addSpec({ _name: "L2OutputOracle", _sel: _getSel("getL2OutputAfter(uint256)") });
        _addSpec({ _name: "L2OutputOracle", _sel: _getSel("getL2OutputIndexAfter(uint256)") });
        _addSpec({
            _name: "L2OutputOracle",
            _sel: _getSel("initialize(uint256,uint256,uint256,uint256,address,address,uint256)")
        });
        _addSpec({ _name: "L2OutputOracle", _sel: _getSel("l2BlockTime()") });
        _addSpec({ _name: "L2OutputOracle", _sel: _getSel("latestBlockNumber()") });
        _addSpec({ _name: "L2OutputOracle", _sel: _getSel("latestOutputIndex()") });
        _addSpec({ _name: "L2OutputOracle", _sel: _getSel("nextBlockNumber()") });
        _addSpec({ _name: "L2OutputOracle", _sel: _getSel("nextOutputIndex()") });
        _addSpec({
            _name: "L2OutputOracle",
            _sel: _getSel("proposeL2Output(bytes32,uint256,bytes32,uint256)"),
            _auth: Role.PROPOSER
        });
        _addSpec({ _name: "L2OutputOracle", _sel: _getSel("proposer()") });
        _addSpec({ _name: "L2OutputOracle", _sel: _getSel("startingBlockNumber()") });
        _addSpec({ _name: "L2OutputOracle", _sel: _getSel("startingTimestamp()") });
        _addSpec({ _name: "L2OutputOracle", _sel: _getSel("submissionInterval()") });
        _addSpec({ _name: "L2OutputOracle", _sel: _getSel("version()") });

        // OptimismPortal
        _addSpec({ _name: "OptimismPortal", _sel: _getSel("depositTransaction(address,uint256,uint64,bool,bytes)") });
        _addSpec({ _name: "OptimismPortal", _sel: _getSel("donateETH()") });
        _addSpec({
            _name: "OptimismPortal",
            _sel: IOptimismPortal.finalizeWithdrawalTransaction.selector,
            _pausable: true
        });
        _addSpec({ _name: "OptimismPortal", _sel: _getSel("finalizedWithdrawals(bytes32)") });
        _addSpec({ _name: "OptimismPortal", _sel: _getSel("guardian()") });
        _addSpec({ _name: "OptimismPortal", _sel: _getSel("initialize(address,address,address)") });
        _addSpec({ _name: "OptimismPortal", _sel: _getSel("isOutputFinalized(uint256)") });
        _addSpec({ _name: "OptimismPortal", _sel: _getSel("l2Oracle()") });
        _addSpec({ _name: "OptimismPortal", _sel: _getSel("l2Sender()") });
        _addSpec({ _name: "OptimismPortal", _sel: _getSel("minimumGasLimit(uint64)") });
        _addSpec({ _name: "OptimismPortal", _sel: _getSel("params()") });
        _addSpec({ _name: "OptimismPortal", _sel: _getSel("paused()") });
        _addSpec({ _name: "OptimismPortal", _sel: IOptimismPortal.proveWithdrawalTransaction.selector, _pausable: true });
        _addSpec({ _name: "OptimismPortal", _sel: _getSel("provenWithdrawals(bytes32)") });
        _addSpec({ _name: "OptimismPortal", _sel: _getSel("superchainConfig()") });
        _addSpec({ _name: "OptimismPortal", _sel: _getSel("systemConfig()") });
        _addSpec({ _name: "OptimismPortal", _sel: _getSel("version()") });
        _addSpec({ _name: "OptimismPortal", _sel: _getSel("balance()") });
        _addSpec({
            _name: "OptimismPortal",
            _sel: _getSel("depositERC20Transaction(address,uint256,uint256,uint64,bool,bytes)")
        });
        _addSpec({ _name: "OptimismPortal", _sel: _getSel("setGasPayingToken(address,uint8,bytes32,bytes32)") });

        // OptimismPortalInterop
        _addSpec({
            _name: "OptimismPortalInterop",
            _sel: _getSel("depositTransaction(address,uint256,uint64,bool,bytes)")
        });
        _addSpec({ _name: "OptimismPortalInterop", _sel: _getSel("donateETH()") });
        _addSpec({
            _name: "OptimismPortalInterop",
            _sel: IOptimismPortal2.finalizeWithdrawalTransaction.selector,
            _pausable: true
        });
        _addSpec({
            _name: "OptimismPortalInterop",
            _sel: IOptimismPortal2.finalizeWithdrawalTransactionExternalProof.selector,
            _pausable: true
        });
        _addSpec({ _name: "OptimismPortalInterop", _sel: _getSel("finalizedWithdrawals(bytes32)") });
        _addSpec({ _name: "OptimismPortalInterop", _sel: _getSel("guardian()") });
        _addSpec({ _name: "OptimismPortalInterop", _sel: _getSel("initialize(address,address,address,uint32)") });
        _addSpec({ _name: "OptimismPortalInterop", _sel: _getSel("l2Sender()") });
        _addSpec({ _name: "OptimismPortalInterop", _sel: _getSel("minimumGasLimit(uint64)") });
        _addSpec({ _name: "OptimismPortalInterop", _sel: _getSel("params()") });
        _addSpec({ _name: "OptimismPortalInterop", _sel: _getSel("paused()") });
        _addSpec({
            _name: "OptimismPortalInterop",
            _sel: IOptimismPortal2.proveWithdrawalTransaction.selector,
            _pausable: true
        });
        _addSpec({ _name: "OptimismPortalInterop", _sel: _getSel("provenWithdrawals(bytes32,address)") });
        _addSpec({ _name: "OptimismPortalInterop", _sel: _getSel("superchainConfig()") });
        _addSpec({ _name: "OptimismPortalInterop", _sel: _getSel("systemConfig()") });
        _addSpec({ _name: "OptimismPortalInterop", _sel: _getSel("version()") });
        _addSpec({ _name: "OptimismPortalInterop", _sel: _getSel("disputeGameFactory()") });
        _addSpec({ _name: "OptimismPortalInterop", _sel: _getSel("disputeGameBlacklist(address)") });
        _addSpec({ _name: "OptimismPortalInterop", _sel: _getSel("respectedGameType()") });
        // Comment out the auth to not disturb the testDeputyGuardianAuth test. This code is not meant to run in
        // production,
        // and will be merged into the OptimismPortal2 contract itself in the future.
        _addSpec({
            _name: "OptimismPortalInterop",
            _sel: _getSel("blacklistDisputeGame(address)") /*, _auth: Role.GUARDIAN*/
        });
        _addSpec({
            _name: "OptimismPortalInterop",
            _sel: _getSel("setRespectedGameType(uint32)") /*, _auth: Role.GUARDIAN*/
        });
        _addSpec({ _name: "OptimismPortalInterop", _sel: _getSel("checkWithdrawal(bytes32,address)") });
        _addSpec({ _name: "OptimismPortalInterop", _sel: _getSel("proofMaturityDelaySeconds()") });
        _addSpec({ _name: "OptimismPortalInterop", _sel: _getSel("disputeGameFinalityDelaySeconds()") });
        _addSpec({ _name: "OptimismPortalInterop", _sel: _getSel("respectedGameTypeUpdatedAt()") });
        _addSpec({ _name: "OptimismPortalInterop", _sel: _getSel("proofSubmitters(bytes32,uint256)") });
        _addSpec({ _name: "OptimismPortalInterop", _sel: _getSel("numProofSubmitters(bytes32)") });
        _addSpec({ _name: "OptimismPortalInterop", _sel: _getSel("balance()") });
        _addSpec({
            _name: "OptimismPortalInterop",
            _sel: _getSel("depositERC20Transaction(address,uint256,uint256,uint64,bool,bytes)")
        });
        _addSpec({ _name: "OptimismPortalInterop", _sel: _getSel("setGasPayingToken(address,uint8,bytes32,bytes32)") });
        _addSpec({
            _name: "OptimismPortalInterop",
            _sel: IOptimismPortalInterop.setConfig.selector,
            _auth: Role.SYSTEMCONFIGOWNER
        });

        // OptimismPortal2
        _addSpec({ _name: "OptimismPortal2", _sel: _getSel("depositTransaction(address,uint256,uint64,bool,bytes)") });
        _addSpec({ _name: "OptimismPortal2", _sel: _getSel("donateETH()") });
        _addSpec({
            _name: "OptimismPortal2",
            _sel: IOptimismPortal2.finalizeWithdrawalTransaction.selector,
            _pausable: true
        });
        _addSpec({
            _name: "OptimismPortal2",
            _sel: IOptimismPortal2.finalizeWithdrawalTransactionExternalProof.selector,
            _pausable: true
        });
        _addSpec({ _name: "OptimismPortal2", _sel: _getSel("finalizedWithdrawals(bytes32)") });
        _addSpec({ _name: "OptimismPortal2", _sel: _getSel("guardian()") });
        _addSpec({ _name: "OptimismPortal2", _sel: _getSel("initialize(address,address,address,uint32)") });
        _addSpec({ _name: "OptimismPortal2", _sel: _getSel("l2Sender()") });
        _addSpec({ _name: "OptimismPortal2", _sel: _getSel("minimumGasLimit(uint64)") });
        _addSpec({ _name: "OptimismPortal2", _sel: _getSel("params()") });
        _addSpec({ _name: "OptimismPortal2", _sel: _getSel("paused()") });
        _addSpec({
            _name: "OptimismPortal2",
            _sel: IOptimismPortal2.proveWithdrawalTransaction.selector,
            _pausable: true
        });
        _addSpec({ _name: "OptimismPortal2", _sel: _getSel("provenWithdrawals(bytes32,address)") });
        _addSpec({ _name: "OptimismPortal2", _sel: _getSel("superchainConfig()") });
        _addSpec({ _name: "OptimismPortal2", _sel: _getSel("systemConfig()") });
        _addSpec({ _name: "OptimismPortal2", _sel: _getSel("version()") });
        _addSpec({ _name: "OptimismPortal2", _sel: _getSel("disputeGameFactory()") });
        _addSpec({ _name: "OptimismPortal2", _sel: _getSel("disputeGameBlacklist(address)") });
        _addSpec({ _name: "OptimismPortal2", _sel: _getSel("respectedGameType()") });
        _addSpec({ _name: "OptimismPortal2", _sel: _getSel("blacklistDisputeGame(address)"), _auth: Role.GUARDIAN });
        _addSpec({ _name: "OptimismPortal2", _sel: _getSel("setRespectedGameType(uint32)"), _auth: Role.GUARDIAN });
        _addSpec({ _name: "OptimismPortal2", _sel: _getSel("checkWithdrawal(bytes32,address)") });
        _addSpec({ _name: "OptimismPortal2", _sel: _getSel("proofMaturityDelaySeconds()") });
        _addSpec({ _name: "OptimismPortal2", _sel: _getSel("disputeGameFinalityDelaySeconds()") });
        _addSpec({ _name: "OptimismPortal2", _sel: _getSel("respectedGameTypeUpdatedAt()") });
        _addSpec({ _name: "OptimismPortal2", _sel: _getSel("proofSubmitters(bytes32,uint256)") });
        _addSpec({ _name: "OptimismPortal2", _sel: _getSel("numProofSubmitters(bytes32)") });
        _addSpec({ _name: "OptimismPortal2", _sel: _getSel("balance()") });
        _addSpec({
            _name: "OptimismPortal2",
            _sel: _getSel("depositERC20Transaction(address,uint256,uint256,uint64,bool,bytes)")
        });
        _addSpec({ _name: "OptimismPortal2", _sel: _getSel("setGasPayingToken(address,uint8,bytes32,bytes32)") });

        // ProtocolVersions
        _addSpec({ _name: "ProtocolVersions", _sel: _getSel("RECOMMENDED_SLOT()") });
        _addSpec({ _name: "ProtocolVersions", _sel: _getSel("REQUIRED_SLOT()") });
        _addSpec({ _name: "ProtocolVersions", _sel: _getSel("VERSION()") });
        _addSpec({ _name: "ProtocolVersions", _sel: IProtocolVersions.initialize.selector });
        _addSpec({ _name: "ProtocolVersions", _sel: _getSel("owner()") });
        _addSpec({ _name: "ProtocolVersions", _sel: IProtocolVersions.recommended.selector });
        _addSpec({ _name: "ProtocolVersions", _sel: _getSel("renounceOwnership()"), _auth: Role.SYSTEMCONFIGOWNER });
        _addSpec({ _name: "ProtocolVersions", _sel: IProtocolVersions.required.selector });
        _addSpec({
            _name: "ProtocolVersions",
            _sel: IProtocolVersions.setRequired.selector,
            _auth: Role.SYSTEMCONFIGOWNER
        });
        _addSpec({
            _name: "ProtocolVersions",
            _sel: IProtocolVersions.setRecommended.selector,
            _auth: Role.SYSTEMCONFIGOWNER
        });
        _addSpec({ _name: "ProtocolVersions", _sel: _getSel("transferOwnership(address)") });
        _addSpec({ _name: "ProtocolVersions", _sel: _getSel("version()") });

        // ResourceMetering
        _addSpec({ _name: "ResourceMetering", _sel: _getSel("params()") });

        // SuperchainConfig
        _addSpec({ _name: "SuperchainConfig", _sel: _getSel("GUARDIAN_SLOT()") });
        _addSpec({ _name: "SuperchainConfig", _sel: _getSel("PAUSED_SLOT()") });
        _addSpec({ _name: "SuperchainConfig", _sel: _getSel("guardian()") });
        _addSpec({ _name: "SuperchainConfig", _sel: _getSel("initialize(address,bool)") });
        _addSpec({ _name: "SuperchainConfig", _sel: _getSel("pause(string)"), _auth: Role.GUARDIAN });
        _addSpec({ _name: "SuperchainConfig", _sel: _getSel("paused()") });
        _addSpec({ _name: "SuperchainConfig", _sel: _getSel("unpause()"), _auth: Role.GUARDIAN });
        _addSpec({ _name: "SuperchainConfig", _sel: _getSel("version()") });

        // SystemConfig
        _addSpec({ _name: "SystemConfig", _sel: _getSel("UNSAFE_BLOCK_SIGNER_SLOT()") });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("START_BLOCK_SLOT()") });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("VERSION()") });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("batcherHash()") });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("gasLimit()") });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("eip1559Denominator()") });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("eip1559Elasticity()") });
        _addSpec({ _name: "SystemConfig", _sel: ISystemConfig.initialize.selector });
        _addSpec({ _name: "SystemConfig", _sel: ISystemConfig.minimumGasLimit.selector });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("overhead()") });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("owner()") });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("renounceOwnership()"), _auth: Role.SYSTEMCONFIGOWNER });
        _addSpec({ _name: "SystemConfig", _sel: ISystemConfig.resourceConfig.selector });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("scalar()") });
        _addSpec({ _name: "SystemConfig", _sel: ISystemConfig.setBatcherHash.selector, _auth: Role.SYSTEMCONFIGOWNER });
        _addSpec({ _name: "SystemConfig", _sel: ISystemConfig.setGasConfig.selector, _auth: Role.SYSTEMCONFIGOWNER });
        _addSpec({ _name: "SystemConfig", _sel: ISystemConfig.setGasLimit.selector, _auth: Role.SYSTEMCONFIGOWNER });
        _addSpec({ _name: "SystemConfig", _sel: ISystemConfig.setEIP1559Params.selector, _auth: Role.SYSTEMCONFIGOWNER });
        _addSpec({
            _name: "SystemConfig",
            _sel: ISystemConfig.setUnsafeBlockSigner.selector,
            _auth: Role.SYSTEMCONFIGOWNER
        });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("transferOwnership(address)"), _auth: Role.SYSTEMCONFIGOWNER });
        _addSpec({ _name: "SystemConfig", _sel: ISystemConfig.unsafeBlockSigner.selector });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("version()") });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("l1CrossDomainMessenger()") });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("l1ERC721Bridge()") });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("l1StandardBridge()") });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("optimismPortal()") });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("optimismMintableERC20Factory()") });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("batchInbox()") });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("startBlock()") });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("L1_CROSS_DOMAIN_MESSENGER_SLOT()") });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("L1_ERC_721_BRIDGE_SLOT()") });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("L1_STANDARD_BRIDGE_SLOT()") });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("OPTIMISM_PORTAL_SLOT()") });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("OPTIMISM_MINTABLE_ERC20_FACTORY_SLOT()") });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("BATCH_INBOX_SLOT()") });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("gasPayingToken()") });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("gasPayingTokenName()") });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("gasPayingTokenSymbol()") });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("isCustomGasToken()") });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("DISPUTE_GAME_FACTORY_SLOT()") });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("disputeGameFactory()") });
        _addSpec({
            _name: "SystemConfig",
            _sel: _getSel("setGasConfigEcotone(uint32,uint32)"),
            _auth: Role.SYSTEMCONFIGOWNER
        });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("basefeeScalar()") });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("blobbasefeeScalar()") });
        _addSpec({ _name: "SystemConfig", _sel: _getSel("maximumGasLimit()") });

        // SystemConfigInterop
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("UNSAFE_BLOCK_SIGNER_SLOT()") });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("START_BLOCK_SLOT()") });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("VERSION()") });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("batcherHash()") });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("gasLimit()") });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("eip1559Denominator()") });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("eip1559Elasticity()") });
        _addSpec({ _name: "SystemConfigInterop", _sel: ISystemConfigInterop.initialize.selector });
        _addSpec({ _name: "SystemConfigInterop", _sel: ISystemConfig.initialize.selector });
        _addSpec({ _name: "SystemConfigInterop", _sel: ISystemConfigInterop.minimumGasLimit.selector });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("overhead()") });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("owner()") });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("renounceOwnership()"), _auth: Role.SYSTEMCONFIGOWNER });
        _addSpec({ _name: "SystemConfigInterop", _sel: ISystemConfigInterop.resourceConfig.selector });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("scalar()") });
        _addSpec({
            _name: "SystemConfigInterop",
            _sel: ISystemConfigInterop.setBatcherHash.selector,
            _auth: Role.SYSTEMCONFIGOWNER
        });
        _addSpec({
            _name: "SystemConfigInterop",
            _sel: ISystemConfigInterop.setGasConfig.selector,
            _auth: Role.SYSTEMCONFIGOWNER
        });
        _addSpec({
            _name: "SystemConfigInterop",
            _sel: ISystemConfigInterop.setGasLimit.selector,
            _auth: Role.SYSTEMCONFIGOWNER
        });
        _addSpec({
            _name: "SystemConfigInterop",
            _sel: ISystemConfigInterop.setEIP1559Params.selector,
            _auth: Role.SYSTEMCONFIGOWNER
        });
        _addSpec({
            _name: "SystemConfigInterop",
            _sel: ISystemConfigInterop.setUnsafeBlockSigner.selector,
            _auth: Role.SYSTEMCONFIGOWNER
        });
        _addSpec({
            _name: "SystemConfigInterop",
            _sel: _getSel("transferOwnership(address)"),
            _auth: Role.SYSTEMCONFIGOWNER
        });
        _addSpec({ _name: "SystemConfigInterop", _sel: ISystemConfigInterop.unsafeBlockSigner.selector });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("version()") });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("l1CrossDomainMessenger()") });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("l1ERC721Bridge()") });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("l1StandardBridge()") });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("optimismPortal()") });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("optimismMintableERC20Factory()") });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("batchInbox()") });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("startBlock()") });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("L1_CROSS_DOMAIN_MESSENGER_SLOT()") });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("L1_ERC_721_BRIDGE_SLOT()") });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("L1_STANDARD_BRIDGE_SLOT()") });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("OPTIMISM_PORTAL_SLOT()") });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("OPTIMISM_MINTABLE_ERC20_FACTORY_SLOT()") });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("BATCH_INBOX_SLOT()") });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("gasPayingToken()") });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("gasPayingTokenName()") });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("gasPayingTokenSymbol()") });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("isCustomGasToken()") });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("DISPUTE_GAME_FACTORY_SLOT()") });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("disputeGameFactory()") });
        _addSpec({
            _name: "SystemConfigInterop",
            _sel: _getSel("setGasConfigEcotone(uint32,uint32)"),
            _auth: Role.SYSTEMCONFIGOWNER
        });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("basefeeScalar()") });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("blobbasefeeScalar()") });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("maximumGasLimit()") });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("addDependency(uint256)"), _auth: Role.DEPENDENCYMANAGER });
        _addSpec({
            _name: "SystemConfigInterop",
            _sel: _getSel("removeDependency(uint256)"),
            _auth: Role.DEPENDENCYMANAGER
        });
        _addSpec({ _name: "SystemConfigInterop", _sel: _getSel("dependencyManager()") });

        // ProxyAdmin
        _addSpec({ _name: "ProxyAdmin", _sel: _getSel("addressManager()") });
        _addSpec({
            _name: "ProxyAdmin",
            _sel: _getSel("changeProxyAdmin(address,address)"),
            _auth: Role.L1PROXYADMINOWNER
        });
        _addSpec({ _name: "ProxyAdmin", _sel: _getSel("getProxyAdmin(address)") });
        _addSpec({ _name: "ProxyAdmin", _sel: _getSel("getProxyImplementation(address)") });
        _addSpec({ _name: "ProxyAdmin", _sel: _getSel("implementationName(address)") });
        _addSpec({ _name: "ProxyAdmin", _sel: _getSel("isUpgrading()") });
        _addSpec({ _name: "ProxyAdmin", _sel: _getSel("owner()") });
        _addSpec({ _name: "ProxyAdmin", _sel: _getSel("proxyType(address)") });
        _addSpec({ _name: "ProxyAdmin", _sel: _getSel("renounceOwnership()"), _auth: Role.L1PROXYADMINOWNER });
        _addSpec({ _name: "ProxyAdmin", _sel: _getSel("setAddress(string,address)"), _auth: Role.L1PROXYADMINOWNER });
        _addSpec({ _name: "ProxyAdmin", _sel: _getSel("setAddressManager(address)"), _auth: Role.L1PROXYADMINOWNER });
        _addSpec({
            _name: "ProxyAdmin",
            _sel: _getSel("setImplementationName(address,string)"),
            _auth: Role.L1PROXYADMINOWNER
        });
        _addSpec({ _name: "ProxyAdmin", _sel: _getSel("setProxyType(address,uint8)"), _auth: Role.L1PROXYADMINOWNER });
        _addSpec({ _name: "ProxyAdmin", _sel: _getSel("setUpgrading(bool)") });
        _addSpec({ _name: "ProxyAdmin", _sel: _getSel("transferOwnership(address)"), _auth: Role.L1PROXYADMINOWNER });
        _addSpec({ _name: "ProxyAdmin", _sel: _getSel("upgrade(address,address)") });
        _addSpec({
            _name: "ProxyAdmin",
            _sel: _getSel("upgradeAndCall(address,address,bytes)"),
            _auth: Role.L1PROXYADMINOWNER
        });

        // GovernanceToken
        _addSpec({ _name: "GovernanceToken", _sel: _getSel("DOMAIN_SEPARATOR()") });
        _addSpec({ _name: "GovernanceToken", _sel: _getSel("allowance(address,address)") });
        _addSpec({ _name: "GovernanceToken", _sel: _getSel("approve(address,uint256)") });
        _addSpec({ _name: "GovernanceToken", _sel: _getSel("balanceOf(address)") });
        _addSpec({ _name: "GovernanceToken", _sel: _getSel("burn(uint256)") });
        _addSpec({ _name: "GovernanceToken", _sel: _getSel("burnFrom(address,uint256)") });
        _addSpec({ _name: "GovernanceToken", _sel: _getSel("checkpoints(address,uint32)") });
        _addSpec({ _name: "GovernanceToken", _sel: _getSel("decimals()") });
        _addSpec({ _name: "GovernanceToken", _sel: _getSel("decreaseAllowance(address,uint256)") });
        _addSpec({ _name: "GovernanceToken", _sel: _getSel("delegate(address)") });
        _addSpec({
            _name: "GovernanceToken",
            _sel: _getSel("delegateBySig(address,uint256,uint256,uint8,bytes32,bytes32)")
        });
        _addSpec({ _name: "GovernanceToken", _sel: _getSel("delegates(address)") });
        _addSpec({ _name: "GovernanceToken", _sel: _getSel("getPastTotalSupply(uint256)") });
        _addSpec({ _name: "GovernanceToken", _sel: _getSel("getPastVotes(address,uint256)") });
        _addSpec({ _name: "GovernanceToken", _sel: _getSel("getVotes(address)") });
        _addSpec({ _name: "GovernanceToken", _sel: _getSel("increaseAllowance(address,uint256)") });
        _addSpec({ _name: "GovernanceToken", _sel: _getSel("mint(address,uint256)"), _auth: Role.GOVERNANCETOKENOWNER });
        _addSpec({ _name: "GovernanceToken", _sel: _getSel("name()") });
        _addSpec({ _name: "GovernanceToken", _sel: _getSel("nonces(address)") });
        _addSpec({ _name: "GovernanceToken", _sel: _getSel("numCheckpoints(address)") });
        _addSpec({ _name: "GovernanceToken", _sel: _getSel("owner()") });
        _addSpec({
            _name: "GovernanceToken",
            _sel: _getSel("permit(address,address,uint256,uint256,uint8,bytes32,bytes32)")
        });
        _addSpec({ _name: "GovernanceToken", _sel: _getSel("renounceOwnership()") });
        _addSpec({ _name: "GovernanceToken", _sel: _getSel("symbol()") });
        _addSpec({ _name: "GovernanceToken", _sel: _getSel("totalSupply()") });
        _addSpec({ _name: "GovernanceToken", _sel: _getSel("transfer(address,uint256)") });
        _addSpec({ _name: "GovernanceToken", _sel: _getSel("transferFrom(address,address,uint256)") });
        _addSpec({ _name: "GovernanceToken", _sel: _getSel("transferOwnership(address)") });

        // MintManager
        _addSpec({ _name: "MintManager", _sel: _getSel("DENOMINATOR()") });
        _addSpec({ _name: "MintManager", _sel: _getSel("MINT_CAP()") });
        _addSpec({ _name: "MintManager", _sel: _getSel("MINT_PERIOD()") });
        _addSpec({ _name: "MintManager", _sel: _getSel("governanceToken()") });
        _addSpec({ _name: "MintManager", _sel: _getSel("mint(address,uint256)"), _auth: Role.MINTMANAGEROWNER });
        _addSpec({ _name: "MintManager", _sel: _getSel("mintPermittedAfter()") });
        _addSpec({ _name: "MintManager", _sel: _getSel("owner()") });
        _addSpec({ _name: "MintManager", _sel: _getSel("renounceOwnership()"), _auth: Role.MINTMANAGEROWNER });
        _addSpec({ _name: "MintManager", _sel: _getSel("transferOwnership(address)"), _auth: Role.MINTMANAGEROWNER });
        _addSpec({ _name: "MintManager", _sel: _getSel("upgrade(address)"), _auth: Role.MINTMANAGEROWNER });

        // AnchorStateRegistry
        _addSpec({ _name: "AnchorStateRegistry", _sel: _getSel("anchors(uint32)") });
        _addSpec({ _name: "AnchorStateRegistry", _sel: _getSel("disputeGameFactory()") });
        _addSpec({ _name: "AnchorStateRegistry", _sel: _getSel("initialize((uint32,(bytes32,uint256))[],address)") });
        _addSpec({ _name: "AnchorStateRegistry", _sel: _getSel("tryUpdateAnchorState()") });
        _addSpec({ _name: "AnchorStateRegistry", _sel: _getSel("setAnchorState(address)"), _auth: Role.GUARDIAN });
        _addSpec({ _name: "AnchorStateRegistry", _sel: _getSel("version()") });
        _addSpec({ _name: "AnchorStateRegistry", _sel: _getSel("superchainConfig()") });

        // PermissionedDisputeGame
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("absolutePrestate()") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("addLocalData(uint256,uint256,uint256)") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("anchorStateRegistry()") });
        _addSpec({
            _name: "PermissionedDisputeGame",
            _sel: _getSel("attack(bytes32,uint256,bytes32)"),
            _auth: Role.CHALLENGER
        });
        _addSpec({
            _name: "PermissionedDisputeGame",
            _sel: _getSel("challengeRootL2Block((bytes32,bytes32,bytes32,bytes32),bytes)"),
            _auth: Role.CHALLENGER
        });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("challenger()") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("claimCredit(address)") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("claimData(uint256)") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("claimDataLen()") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("claims(bytes32)") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("clockExtension()") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("createdAt()") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("credit(address)") });
        _addSpec({
            _name: "PermissionedDisputeGame",
            _sel: _getSel("defend(bytes32,uint256,bytes32)"),
            _auth: Role.CHALLENGER
        });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("extraData()") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("gameCreator()") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("gameData()") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("gameType()") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("getChallengerDuration(uint256)") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("getNumToResolve(uint256)") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("getRequiredBond(uint128)") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("initialize()") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("l1Head()") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("l2BlockNumber()") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("l2BlockNumberChallenged()") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("l2BlockNumberChallenger()") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("l2ChainId()") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("maxClockDuration()") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("maxGameDepth()") });
        _addSpec({
            _name: "PermissionedDisputeGame",
            _sel: _getSel("move(bytes32,uint256,bytes32,bool)"),
            _auth: Role.CHALLENGER
        });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("proposer()") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("resolutionCheckpoints(uint256)") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("resolve()") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("resolveClaim(uint256,uint256)") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("resolvedAt()") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("resolvedSubgames(uint256)") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("rootClaim()") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("splitDepth()") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("startingBlockNumber()") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("startingOutputRoot()") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("startingRootHash()") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("status()") });
        _addSpec({
            _name: "PermissionedDisputeGame",
            _sel: _getSel("step(uint256,bool,bytes,bytes)"),
            _auth: Role.CHALLENGER
        });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("subgames(uint256,uint256)") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("version()") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("vm()") });
        _addSpec({ _name: "PermissionedDisputeGame", _sel: _getSel("weth()") });

        // FaultDisputeGame
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("absolutePrestate()") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("addLocalData(uint256,uint256,uint256)") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("anchorStateRegistry()") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("attack(bytes32,uint256,bytes32)") });
        _addSpec({
            _name: "FaultDisputeGame",
            _sel: _getSel("challengeRootL2Block((bytes32,bytes32,bytes32,bytes32),bytes)")
        });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("claimCredit(address)") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("claimData(uint256)") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("claimDataLen()") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("claims(bytes32)") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("clockExtension()") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("createdAt()") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("credit(address)") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("defend(bytes32,uint256,bytes32)") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("extraData()") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("gameCreator()") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("gameData()") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("gameType()") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("getChallengerDuration(uint256)") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("getRequiredBond(uint128)") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("initialize()") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("l1Head()") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("l2BlockNumber()") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("l2BlockNumberChallenged()") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("l2BlockNumberChallenger()") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("l2ChainId()") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("maxClockDuration()") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("maxGameDepth()") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("move(bytes32,uint256,bytes32,bool)") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("resolutionCheckpoints(uint256)") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("resolve()") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("getNumToResolve(uint256)") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("resolveClaim(uint256,uint256)") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("resolvedAt()") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("resolvedSubgames(uint256)") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("rootClaim()") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("splitDepth()") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("startingBlockNumber()") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("startingOutputRoot()") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("startingRootHash()") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("status()") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("step(uint256,bool,bytes,bytes)") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("subgames(uint256,uint256)") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("version()") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("vm()") });
        _addSpec({ _name: "FaultDisputeGame", _sel: _getSel("weth()") });

        // DisputeGameFactory
        _addSpec({ _name: "DisputeGameFactory", _sel: _getSel("create(uint32,bytes32,bytes)") });
        _addSpec({ _name: "DisputeGameFactory", _sel: _getSel("findLatestGames(uint32,uint256,uint256)") });
        _addSpec({ _name: "DisputeGameFactory", _sel: _getSel("gameAtIndex(uint256)") });
        _addSpec({ _name: "DisputeGameFactory", _sel: _getSel("gameCount()") });
        _addSpec({ _name: "DisputeGameFactory", _sel: _getSel("gameImpls(uint32)") });
        _addSpec({ _name: "DisputeGameFactory", _sel: _getSel("games(uint32,bytes32,bytes)") });
        _addSpec({ _name: "DisputeGameFactory", _sel: _getSel("getGameUUID(uint32,bytes32,bytes)") });
        _addSpec({ _name: "DisputeGameFactory", _sel: _getSel("initBonds(uint32)") });
        _addSpec({ _name: "DisputeGameFactory", _sel: _getSel("initialize(address)") });
        _addSpec({ _name: "DisputeGameFactory", _sel: _getSel("owner()") });
        _addSpec({
            _name: "DisputeGameFactory",
            _sel: _getSel("renounceOwnership()"),
            _auth: Role.DISPUTEGAMEFACTORYOWNER
        });
        _addSpec({
            _name: "DisputeGameFactory",
            _sel: _getSel("setImplementation(uint32,address)"),
            _auth: Role.DISPUTEGAMEFACTORYOWNER
        });
        _addSpec({
            _name: "DisputeGameFactory",
            _sel: _getSel("setInitBond(uint32,uint256)"),
            _auth: Role.DISPUTEGAMEFACTORYOWNER
        });
        _addSpec({
            _name: "DisputeGameFactory",
            _sel: _getSel("transferOwnership(address)"),
            _auth: Role.DISPUTEGAMEFACTORYOWNER
        });
        _addSpec({ _name: "DisputeGameFactory", _sel: _getSel("version()") });

        // DelayedWETH
        _addSpec({ _name: "DelayedWETH", _sel: _getSel("allowance(address,address)") });
        _addSpec({ _name: "DelayedWETH", _sel: _getSel("approve(address,uint256)") });
        _addSpec({ _name: "DelayedWETH", _sel: _getSel("balanceOf(address)") });
        _addSpec({ _name: "DelayedWETH", _sel: _getSel("config()") });
        _addSpec({ _name: "DelayedWETH", _sel: _getSel("decimals()") });
        _addSpec({ _name: "DelayedWETH", _sel: _getSel("delay()") });
        _addSpec({ _name: "DelayedWETH", _sel: _getSel("deposit()") });
        _addSpec({ _name: "DelayedWETH", _sel: _getSel("hold(address,uint256)"), _auth: Role.DELAYEDWETHOWNER });
        _addSpec({ _name: "DelayedWETH", _sel: _getSel("initialize(address,address)") });
        _addSpec({ _name: "DelayedWETH", _sel: _getSel("name()") });
        _addSpec({ _name: "DelayedWETH", _sel: _getSel("owner()") });
        _addSpec({ _name: "DelayedWETH", _sel: _getSel("recover(uint256)"), _auth: Role.DELAYEDWETHOWNER });
        _addSpec({ _name: "DelayedWETH", _sel: _getSel("renounceOwnership()"), _auth: Role.DELAYEDWETHOWNER });
        _addSpec({ _name: "DelayedWETH", _sel: _getSel("symbol()") });
        _addSpec({ _name: "DelayedWETH", _sel: _getSel("totalSupply()") });
        _addSpec({ _name: "DelayedWETH", _sel: _getSel("transfer(address,uint256)") });
        _addSpec({ _name: "DelayedWETH", _sel: _getSel("transferFrom(address,address,uint256)") });
        _addSpec({ _name: "DelayedWETH", _sel: _getSel("transferOwnership(address)"), _auth: Role.DELAYEDWETHOWNER });
        _addSpec({ _name: "DelayedWETH", _sel: _getSel("unlock(address,uint256)") });
        _addSpec({ _name: "DelayedWETH", _sel: _getSel("version()") });
        _addSpec({ _name: "DelayedWETH", _sel: _getSel("withdraw(address,uint256)"), _pausable: true });
        _addSpec({ _name: "DelayedWETH", _sel: _getSel("withdraw(uint256)"), _pausable: true });
        _addSpec({ _name: "DelayedWETH", _sel: _getSel("withdrawals(address,address)") });

        // WETH98
        _addSpec({ _name: "WETH98", _sel: _getSel("allowance(address,address)") });
        _addSpec({ _name: "WETH98", _sel: _getSel("approve(address,uint256)") });
        _addSpec({ _name: "WETH98", _sel: _getSel("balanceOf(address)") });
        _addSpec({ _name: "WETH98", _sel: _getSel("decimals()") });
        _addSpec({ _name: "WETH98", _sel: _getSel("deposit()") });
        _addSpec({ _name: "WETH98", _sel: _getSel("name()") });
        _addSpec({ _name: "WETH98", _sel: _getSel("symbol()") });
        _addSpec({ _name: "WETH98", _sel: _getSel("totalSupply()") });
        _addSpec({ _name: "WETH98", _sel: _getSel("transfer(address,uint256)") });
        _addSpec({ _name: "WETH98", _sel: _getSel("transferFrom(address,address,uint256)") });
        _addSpec({ _name: "WETH98", _sel: _getSel("withdraw(uint256)") });

        // OPContractsManager
        _addSpec({ _name: "OPContractsManager", _sel: _getSel("version()") });
        _addSpec({ _name: "OPContractsManager", _sel: _getSel("superchainConfig()") });
        _addSpec({ _name: "OPContractsManager", _sel: _getSel("protocolVersions()") });
        _addSpec({ _name: "OPContractsManager", _sel: _getSel("l1ContractsRelease()") });
        _addSpec({ _name: "OPContractsManager", _sel: _getSel("systemConfigs(uint256)") });
        _addSpec({ _name: "OPContractsManager", _sel: _getSel("OUTPUT_VERSION()") });
        _addSpec({ _name: "OPContractsManager", _sel: OPContractsManager.deploy.selector });
        _addSpec({ _name: "OPContractsManager", _sel: OPContractsManager.blueprints.selector });
        _addSpec({ _name: "OPContractsManager", _sel: OPContractsManager.chainIdToBatchInboxAddress.selector });
        _addSpec({ _name: "OPContractsManager", _sel: OPContractsManager.implementations.selector });

        // OPContractsManagerInterop
        _addSpec({ _name: "OPContractsManagerInterop", _sel: _getSel("version()") });
        _addSpec({ _name: "OPContractsManagerInterop", _sel: _getSel("superchainConfig()") });
        _addSpec({ _name: "OPContractsManagerInterop", _sel: _getSel("protocolVersions()") });
        _addSpec({ _name: "OPContractsManagerInterop", _sel: _getSel("l1ContractsRelease()") });
        _addSpec({ _name: "OPContractsManagerInterop", _sel: _getSel("systemConfigs(uint256)") });
        _addSpec({ _name: "OPContractsManagerInterop", _sel: _getSel("OUTPUT_VERSION()") });
        _addSpec({ _name: "OPContractsManagerInterop", _sel: OPContractsManager.deploy.selector });
        _addSpec({ _name: "OPContractsManagerInterop", _sel: OPContractsManager.blueprints.selector });
        _addSpec({ _name: "OPContractsManagerInterop", _sel: OPContractsManager.chainIdToBatchInboxAddress.selector });
        _addSpec({ _name: "OPContractsManagerInterop", _sel: OPContractsManager.implementations.selector });

        // DeputyGuardianModule
        _addSpec({
            _name: "DeputyGuardianModule",
            _sel: _getSel("blacklistDisputeGame(address,address)"),
            _auth: Role.DEPUTYGUARDIAN
        });
        _addSpec({
            _name: "DeputyGuardianModule",
            _sel: _getSel("setRespectedGameType(address,uint32)"),
            _auth: Role.DEPUTYGUARDIAN
        });
        _addSpec({
            _name: "DeputyGuardianModule",
            _sel: _getSel("setAnchorState(address,address)"),
            _auth: Role.DEPUTYGUARDIAN
        });
        _addSpec({ _name: "DeputyGuardianModule", _sel: _getSel("pause()"), _auth: Role.DEPUTYGUARDIAN });
        _addSpec({ _name: "DeputyGuardianModule", _sel: _getSel("unpause()"), _auth: Role.DEPUTYGUARDIAN });
        _addSpec({ _name: "DeputyGuardianModule", _sel: _getSel("deputyGuardian()") });
        _addSpec({ _name: "DeputyGuardianModule", _sel: _getSel("safe()") });
        _addSpec({ _name: "DeputyGuardianModule", _sel: _getSel("superchainConfig()") });
        _addSpec({ _name: "DeputyGuardianModule", _sel: _getSel("version()") });

        // LivenessGuard
        _addSpec({ _name: "LivenessGuard", _sel: _getSel("checkAfterExecution(bytes32,bool)"), _auth: Role.COUNCILSAFE });
        _addSpec({
            _name: "LivenessGuard",
            _sel: _getSel(
                "checkTransaction(address,uint256,bytes,uint8,uint256,uint256,uint256,address,address,bytes,address)"
            ),
            _auth: Role.COUNCILSAFE
        });
        _addSpec({ _name: "LivenessGuard", _sel: _getSel("lastLive(address)") });
        _addSpec({ _name: "LivenessGuard", _sel: _getSel("safe()") });
        _addSpec({ _name: "LivenessGuard", _sel: _getSel("showLiveness()"), _auth: Role.COUNCILSAFEOWNER });
        _addSpec({ _name: "LivenessGuard", _sel: _getSel("version()") });

        // LivenessModule
        _addSpec({ _name: "LivenessModule", _sel: _getSel("canRemove(address)") });
        _addSpec({ _name: "LivenessModule", _sel: _getSel("fallbackOwner()") });
        _addSpec({ _name: "LivenessModule", _sel: _getSel("getRequiredThreshold(uint256)") });
        _addSpec({ _name: "LivenessModule", _sel: _getSel("livenessGuard()") });
        _addSpec({ _name: "LivenessModule", _sel: _getSel("livenessInterval()") });
        _addSpec({ _name: "LivenessModule", _sel: _getSel("minOwners()") });
        _addSpec({ _name: "LivenessModule", _sel: _getSel("ownershipTransferredToFallback()") });
        _addSpec({ _name: "LivenessModule", _sel: _getSel("removeOwners(address[],address[])") });
        _addSpec({ _name: "LivenessModule", _sel: _getSel("safe()") });
        _addSpec({ _name: "LivenessModule", _sel: _getSel("thresholdPercentage()") });
        _addSpec({ _name: "LivenessModule", _sel: _getSel("version()") });
    }

    /// @dev Computes the selector from a function signature.
    function _getSel(string memory _name) internal pure returns (bytes4) {
        return bytes4(keccak256(abi.encodePacked(_name)));
    }

    /// @dev Adds a spec for a function.
    function _addSpec(string memory _name, bytes4 _sel, Role _auth, bool _pausable) internal {
        Spec memory spec = Spec({ name: _name, sel: _sel, auth: _auth, pausable: _pausable });
        specs[_name][_sel] = spec;
        numEntries[_name]++;
        specsByRole[_auth].push(spec);
        numSpecs++;
    }

    /// @dev Adds a spec for a function with no auth.
    function _addSpec(string memory _name, bytes4 _sel, bool _pausable) internal {
        _addSpec({ _name: _name, _sel: _sel, _auth: Role.NOAUTH, _pausable: _pausable });
    }

    /// @dev Adds a spec for a function with no pausability.
    function _addSpec(string memory _name, bytes4 _sel, Role _auth) internal {
        _addSpec({ _name: _name, _sel: _sel, _auth: _auth, _pausable: false });
    }

    /// @dev Adds a spec for a function with no auth and no pausability.
    function _addSpec(string memory _name, bytes4 _sel) internal {
        _addSpec({ _name: _name, _sel: _sel, _auth: Role.NOAUTH, _pausable: false });
    }

    /// @notice Ensures that there's an auth spec for every L1 contract function.
    function test_contractAuth_works() public {
        string[] memory pathExcludes = new string[](6);
        pathExcludes[0] = "src/dispute/interfaces/*";
        pathExcludes[1] = "src/dispute/lib/*";
        pathExcludes[2] = "src/safe/SafeSigners.sol";
        pathExcludes[3] = "src/L1/interfaces/*";
        pathExcludes[4] = "src/governance/interfaces/*";
        pathExcludes[5] = "src/safe/interfaces/*";
        Abi[] memory abis = ForgeArtifacts.getContractFunctionAbis(
            "src/{L1,dispute,governance,safe,universal/ProxyAdmin.sol,universal/WETH98.sol}", pathExcludes
        );

        uint256 numCheckedEntries = 0;
        for (uint256 i = 0; i < abis.length; i++) {
            string memory contractName = abis[i].contractName;
            assertEq(
                abis[i].entries.length,
                numEntries[contractName],
                string.concat("Specification_Test: invalid number of ABI entries for ", contractName)
            );

            for (uint256 j = 0; j < abis[i].entries.length; j++) {
                AbiEntry memory abiEntry = abis[i].entries[j];
                console.log(
                    "Checking auth spec for %s: %s(%x)", contractName, abiEntry.fnName, uint256(uint32(abiEntry.sel))
                );
                Spec memory spec = specs[contractName][abiEntry.sel];
                assertTrue(
                    spec.sel != bytes4(0),
                    string.concat(
                        "Specification_Test: missing spec definition for ", contractName, "::", abiEntry.fnName
                    )
                );
                assertEq(
                    abiEntry.sel,
                    spec.sel,
                    string.concat("Specification_Test: invalid ABI ", contractName, "::", abiEntry.fnName)
                );
                numCheckedEntries++;
            }
        }
        assertEq(numSpecs, numCheckedEntries, "Some specs were not checked");
    }

    /// @dev Asserts that two roles are equal by comparing their uint256 representations.
    function _assertRolesEq(Role leftRole, Role rightRole) internal pure {
        assertEq(uint256(leftRole), uint256(rightRole));
    }

    /// @notice Ensures that the DeputyGuardian is authorized to take all Guardian actions.
    function test_deputyGuardianAuth_works() public view {
        assertEq(specsByRole[Role.DEPUTYGUARDIAN].length, specsByRole[Role.GUARDIAN].length);
        assertEq(specsByRole[Role.DEPUTYGUARDIAN].length, 5);

        mapping(bytes4 => Spec) storage dgmFuncSpecs = specs["DeputyGuardianModule"];
        mapping(bytes4 => Spec) storage superchainConfigFuncSpecs = specs["SuperchainConfig"];
        mapping(bytes4 => Spec) storage portal2FuncSpecs = specs["OptimismPortal2"];
        mapping(bytes4 => Spec) storage anchorRegFuncSpecs = specs["AnchorStateRegistry"];

        // Ensure that for each of the DeputyGuardianModule's methods there is a corresponding method on another
        // system contract authed to the Guardian role.
        _assertRolesEq(dgmFuncSpecs[_getSel("pause()")].auth, Role.DEPUTYGUARDIAN);
        _assertRolesEq(superchainConfigFuncSpecs[_getSel("pause(string)")].auth, Role.GUARDIAN);

        _assertRolesEq(dgmFuncSpecs[_getSel("unpause()")].auth, Role.DEPUTYGUARDIAN);
        _assertRolesEq(superchainConfigFuncSpecs[_getSel("unpause()")].auth, Role.GUARDIAN);

        _assertRolesEq(dgmFuncSpecs[_getSel("blacklistDisputeGame(address,address)")].auth, Role.DEPUTYGUARDIAN);
        _assertRolesEq(portal2FuncSpecs[_getSel("blacklistDisputeGame(address)")].auth, Role.GUARDIAN);

        _assertRolesEq(dgmFuncSpecs[_getSel("setRespectedGameType(address,uint32)")].auth, Role.DEPUTYGUARDIAN);
        _assertRolesEq(portal2FuncSpecs[_getSel("setRespectedGameType(uint32)")].auth, Role.GUARDIAN);

        _assertRolesEq(dgmFuncSpecs[_getSel("setAnchorState(address,address)")].auth, Role.DEPUTYGUARDIAN);
        _assertRolesEq(anchorRegFuncSpecs[_getSel("setAnchorState(address)")].auth, Role.GUARDIAN);
    }
}
