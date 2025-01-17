// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { Initializable } from "@openzeppelin/contracts/proxy/utils/Initializable.sol";
import { ResourceMetering } from "src/L1/ResourceMetering.sol";

// Libraries
import { SafeERC20 } from "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import { EOA } from "src/libraries/EOA.sol";
import { SafeCall } from "src/libraries/SafeCall.sol";
import { Constants } from "src/libraries/Constants.sol";
import { Types } from "src/libraries/Types.sol";
import { Hashing } from "src/libraries/Hashing.sol";
import { SecureMerkleTrie } from "src/libraries/trie/SecureMerkleTrie.sol";
import { AddressAliasHelper } from "src/vendor/AddressAliasHelper.sol";
import { GameStatus, GameType } from "src/dispute/lib/Types.sol";

// Interfaces
import { IERC20 } from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import { ISemver } from "interfaces/universal/ISemver.sol";
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";
import { IResourceMetering } from "interfaces/L1/IResourceMetering.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { IDisputeGame } from "interfaces/dispute/IDisputeGame.sol";
import { IAnchorStateRegistry } from "interfaces/dispute/IAnchorStateRegistry.sol";

/// @custom:proxied true
/// @title OptimismPortal2
/// @notice The OptimismPortal is a low-level contract responsible for passing messages between L1
///         and L2. Messages sent directly to the OptimismPortal have no form of replayability.
///         Users are encouraged to use the L1CrossDomainMessenger for a higher-level interface.
contract OptimismPortal2 is Initializable, ResourceMetering, ISemver {
    /// @notice Allows for interactions with non standard ERC20 tokens.
    using SafeERC20 for IERC20;

    /// @notice Represents a proven withdrawal.
    /// @custom:field disputeGameProxy Game that the withdrawal was proven against.
    /// @custom:field timestamp        Timestamp at which the withdrawal was proven.
    struct ProvenWithdrawal {
        IDisputeGame disputeGameProxy;
        uint64 timestamp;
    }

    /// @notice The delay between when a withdrawal is proven and when it may be finalized.
    uint256 internal immutable PROOF_MATURITY_DELAY_SECONDS;

    /// @notice Version of the deposit event.
    uint256 internal constant DEPOSIT_VERSION = 0;

    /// @notice The L2 gas limit set when eth is deposited using the receive() function.
    uint64 internal constant RECEIVE_DEFAULT_GAS_LIMIT = 100_000;

    /// @notice The L2 gas limit for system deposit transactions that are initiated from L1.
    uint32 internal constant SYSTEM_DEPOSIT_GAS_LIMIT = 200_000;

    /// @notice Address of the L2 account which initiated a withdrawal in this transaction.
    ///         If the of this variable is the default L2 sender address, then we are NOT inside of
    ///         a call to finalizeWithdrawalTransaction.
    address public l2Sender;

    /// @notice A list of withdrawal hashes which have been successfully finalized.
    mapping(bytes32 => bool) public finalizedWithdrawals;

    /// @custom:legacy
    /// @custom:spacer provenWithdrawals
    /// @notice Spacer taking up the legacy `provenWithdrawals` mapping slot.
    bytes32 private spacer_52_0_32;

    /// @custom:legacy
    /// @custom:spacer paused
    /// @notice Spacer for backwards compatibility.
    bool private spacer_53_0_1;

    /// @notice Address of the SuperchainConfig contract.
    ISuperchainConfig public superchainConfig;

    /// @custom:legacy
    /// @custom:spacer l2Oracle
    /// @notice Spacer taking up the legacy `l2Oracle` address slot.
    address private spacer_54_0_20;

    /// @notice Address of the SystemConfig contract.
    /// @custom:network-specific
    ISystemConfig public systemConfig;

    /// @custom:network-specific
    /// @custom:legacy
    /// @custom:spacer disputeGameFactory
    /// @notice Spacer taking up the legacy `disputeGameFactory` address slot.
    address private spacer_56_0_20;

    /// @notice A mapping of withdrawal hashes to proof submitters to ProvenWithdrawal data.
    mapping(bytes32 => mapping(address => ProvenWithdrawal)) public provenWithdrawals;

    /// @custom:legacy
    /// @custom:spacer disputeGameBlacklist
    bytes32 private spacer_58_0_32;

    /// @custom:legacy
    /// @custom:spacer respectedGameType
    GameType private spacer_59_0_4;

    /// @custom:legacy
    /// @custom:spacer respectedGameTypeUpdatedAt
    uint64 private spacer_59_4_8;

    /// @notice Mapping of withdrawal hashes to addresses that have submitted a proof for the
    ///         withdrawal. Original OptimismPortal contract only allowed one proof to be submitted
    ///         for any given withdrawal hash. Fault Proofs version of this contract must allow
    ///         multiple proofs for the same withdrawal hash to prevent a malicious user from
    ///         blocking other withdrawals by proving them against invalid proposals. Submitters
    ///         are tracked in an array to simplify the off-chain process of determining which
    ///         proof submission should be used when finalizing a withdrawal.
    mapping(bytes32 => address[]) public proofSubmitters;

    /// @custom:legacy
    /// @custom:spacer _balance
    uint256 private spacer_61_0_32;

    /// @notice Address of the AnchorStateRegistry contract.
    IAnchorStateRegistry public anchorStateRegistry;

    /// @notice Emitted when a transaction is deposited from L1 to L2. The parameters of this event
    ///         are read by the rollup node and used to derive deposit transactions on L2.
    /// @param from       Address that triggered the deposit transaction.
    /// @param to         Address that the deposit transaction is directed to.
    /// @param version    Version of this deposit transaction event.
    /// @param opaqueData ABI encoded deposit data to be parsed off-chain.
    event TransactionDeposited(address indexed from, address indexed to, uint256 indexed version, bytes opaqueData);

    /// @notice Emitted when a withdrawal transaction is proven.
    /// @param withdrawalHash Hash of the withdrawal transaction.
    /// @param from           Address that triggered the withdrawal transaction.
    /// @param to             Address that the withdrawal transaction is directed to.
    event WithdrawalProven(bytes32 indexed withdrawalHash, address indexed from, address indexed to);

    /// @notice Emitted when a withdrawal transaction is proven. Exists as a separate event to
    ///         allow for backwards compatibility for tooling that observes the WithdrawalProven
    ///         event.
    /// @param withdrawalHash Hash of the withdrawal transaction.
    /// @param proofSubmitter Address of the proof submitter.
    event WithdrawalProvenExtension1(bytes32 indexed withdrawalHash, address indexed proofSubmitter);

    /// @notice Emitted when a withdrawal transaction is finalized.
    /// @param withdrawalHash Hash of the withdrawal transaction.
    /// @param success        Whether the withdrawal transaction was successful.
    event WithdrawalFinalized(bytes32 indexed withdrawalHash, bool success);

    /// @notice Thrown when a withdrawal has already been finalized.
    error OptimismPortal_AlreadyFinalized();

    /// @notice Thrown when the target of a withdrawal is unsafe.
    error OptimismPortal_BadTarget();

    /// @notice Thrown when the calldata for a deposit is too large.
    error OptimismPortal_CalldataTooLarge();

    /// @notice Thrown when the portal is paused.
    error OptimismPortal_CallPaused();

    /// @notice Thrown when a gas estimation transaction is being executed.
    error OptimismPortal_GasEstimation();

    /// @notice Thrown when the gas limit for a deposit is too low.
    error OptimismPortal_GasLimitTooLow();

    /// @notice Thrown when the target of a withdrawal is not a proper dispute game.
    error OptimismPortal_ImproperDisputeGame();

    /// @notice Thrown when a withdrawal has not been proven against a valid dispute game.
    error OptimismPortal_InvalidDisputeGame();

    /// @notice Thrown when a withdrawal has not been proven against a valid merkle proof.
    error OptimismPortal_InvalidMerkleProof();

    /// @notice Thrown when a withdrawal has not been proven against a valid output root proof.
    error OptimismPortal_InvalidOutputRootProof();

    /// @notice Thrown when a withdrawal's timestamp is not greater than the dispute game's creation timestamp.
    error OptimismPortal_InvalidProofTimestamp();

    /// @notice Thrown when the root claim of a dispute game is invalid.
    error OptimismPortal_InvalidRootClaim();

    /// @notice Thrown when a withdrawal is being finalized by a reentrant call.
    error OptimismPortal_NoReentrancy();

    /// @notice Thrown when a withdrawal has not been proven for long enough.
    error OptimismPortal_ProofNotOldEnough();

    /// @notice Thrown when a withdrawal has not been proven.
    error OptimismPortal_Unproven();

    /// @notice Reverts when paused.
    modifier whenNotPaused() {
        if (paused()) revert OptimismPortal_CallPaused();
        _;
    }

    /// @notice Semantic version.
    /// @custom:semver 4.0.0
    function version() public pure virtual returns (string memory) {
        return "4.0.0";
    }

    /// @param _proofMaturityDelaySeconds The proof maturity delay in seconds.
    constructor(uint256 _proofMaturityDelaySeconds) {
        PROOF_MATURITY_DELAY_SECONDS = _proofMaturityDelaySeconds;
        _disableInitializers();
    }

    /// @notice Initializer.
    /// @param _systemConfig Address of the SystemConfig.
    /// @param _superchainConfig Address of the SuperchainConfig.
    /// @param _anchorStateRegistry Address of the AnchorStateRegistry.
    function initialize(
        ISystemConfig _systemConfig,
        ISuperchainConfig _superchainConfig,
        IAnchorStateRegistry _anchorStateRegistry
    )
        external
        reinitializer(2)
    {
        systemConfig = _systemConfig;
        superchainConfig = _superchainConfig;
        anchorStateRegistry = _anchorStateRegistry;

        // Set the l2Sender slot, only if it is currently empty. This signals the first
        // initialization of the contract.
        if (l2Sender == address(0)) {
            l2Sender = Constants.DEFAULT_L2_SENDER;
        }

        __ResourceMetering_init();
    }

    /// @notice Upgrades the OptimismPortal contract to have a reference to the AnchorStateRegistry.
    /// @param _anchorStateRegistry AnchorStateRegistry contract.
    function upgrade(IAnchorStateRegistry _anchorStateRegistry) external reinitializer(2) {
        anchorStateRegistry = _anchorStateRegistry;
    }

    /// @notice Getter for the current paused status.
    function paused() public view returns (bool) {
        return superchainConfig.paused();
    }

    /// @notice Getter for the proof maturity delay.
    function proofMaturityDelaySeconds() public view returns (uint256) {
        return PROOF_MATURITY_DELAY_SECONDS;
    }

    /// @notice Getter for the address of the DisputeGameFactory contract.
    function disputeGameFactory() public view returns (IDisputeGameFactory) {
        return anchorStateRegistry.disputeGameFactory();
    }

    /// @custom:legacy
    /// @notice Getter function for the address of the guardian.
    function guardian() public view returns (address) {
        return superchainConfig.guardian();
    }

    /// @custom:legacy
    /// @notice Getter for the dispute game finality delay.
    function disputeGameFinalityDelaySeconds() external view returns (uint256) {
        return anchorStateRegistry.disputeGameFinalityDelaySeconds();
    }

    /// @custom:legacy
    /// @notice Getter for the respected game type.
    function respectedGameType() external view returns (GameType) {
        return anchorStateRegistry.respectedGameType();
    }

    /// @custom:legacy
    /// @notice Getter for the timestamp at which the respected game type was updated.
    function respectedGameTypeUpdatedAt() external view returns (uint64) {
        return anchorStateRegistry.retirementTimestamp();
    }

    /// @notice Computes the minimum gas limit for a deposit.
    ///         The minimum gas limit linearly increases based on the size of the calldata.
    ///         This is to prevent users from creating L2 resource usage without paying for it.
    ///         This function can be used when interacting with the portal to ensure forwards
    ///         compatibility.
    /// @param _byteCount Number of bytes in the calldata.
    /// @return The minimum gas limit for a deposit.
    function minimumGasLimit(uint64 _byteCount) public pure returns (uint64) {
        return _byteCount * 16 + 21000;
    }

    /// @notice Accepts value so that users can send ETH directly to this contract and have the
    ///         funds be deposited to their address on L2. This is intended as a convenience
    ///         function for EOAs. Contracts should call the depositTransaction() function directly
    ///         otherwise any deposited funds will be lost due to address aliasing.
    receive() external payable {
        depositTransaction(msg.sender, msg.value, RECEIVE_DEFAULT_GAS_LIMIT, false, bytes(""));
    }

    /// @notice Accepts ETH value without triggering a deposit to L2.
    ///         This function mainly exists for the sake of the migration between the legacy
    ///         Optimism system and Bedrock.
    function donateETH() external payable {
        // Intentionally empty.
    }

    /// @notice Proves a withdrawal transaction.
    /// @param _tx               Withdrawal transaction to finalize.
    /// @param _disputeGameIndex Index of the dispute game to prove the withdrawal against.
    /// @param _outputRootProof  Inclusion proof of the L2ToL1MessagePasser storage root.
    /// @param _withdrawalProof  Inclusion proof of the withdrawal within the L2ToL1MessagePasser.
    function proveWithdrawalTransaction(
        Types.WithdrawalTransaction memory _tx,
        uint256 _disputeGameIndex,
        Types.OutputRootProof calldata _outputRootProof,
        bytes[] calldata _withdrawalProof
    )
        external
        whenNotPaused
    {
        // Make sure that the target address is safe.
        if (_isUnsafeTarget(_tx.target)) {
            revert OptimismPortal_BadTarget();
        }

        // Fetch the dispute game proxy from the `DisputeGameFactory` contract.
        (,, IDisputeGame gameProxy) = disputeGameFactory().gameAtIndex(_disputeGameIndex);

        // Game must be a Proper Game.
        if (!anchorStateRegistry.isGameProper(gameProxy)) {
            revert OptimismPortal_ImproperDisputeGame();
        }

        // Game must have been respected game type when created.
        if (!anchorStateRegistry.isGameRespected(gameProxy)) {
            revert OptimismPortal_InvalidDisputeGame();
        }

        // Game must not have resolved in favor of the Challenger (invalid root claim).
        if (gameProxy.status() == GameStatus.CHALLENGER_WINS) {
            revert OptimismPortal_InvalidDisputeGame();
        }

        // Verify that the output root can be generated with the elements in the proof.
        if (gameProxy.rootClaim().raw() != Hashing.hashOutputRootProof(_outputRootProof)) {
            revert OptimismPortal_InvalidOutputRootProof();
        }

        // As a sanity check, we make sure that the current timestamp is not less than or equal to
        // the dispute game's creation timestamp. Not strictly necessary but extra layer of
        // safety against weird bugs. Note that this blocks withdrawals from being proven in the
        // same block that a dispute game is created.
        if (block.timestamp <= gameProxy.createdAt().raw()) {
            revert OptimismPortal_InvalidProofTimestamp();
        }

        // Load the ProvenWithdrawal into memory, using the withdrawal hash as a unique identifier.
        bytes32 withdrawalHash = Hashing.hashWithdrawal(_tx);

        // Compute the storage slot of the withdrawal hash in the L2ToL1MessagePasser contract.
        // Refer to the Solidity documentation for more information on how storage layouts are
        // computed for mappings.
        bytes32 storageKey = keccak256(
            abi.encode(
                withdrawalHash,
                uint256(0) // The withdrawals mapping is at the first slot in the layout.
            )
        );

        // Verify that the hash of this withdrawal was stored in the L2toL1MessagePasser contract
        // on L2. If this is true, under the assumption that the SecureMerkleTrie does not have
        // bugs, then we know that this withdrawal was actually triggered on L2 and can therefore
        // be relayed on L1.
        if (
            SecureMerkleTrie.verifyInclusionProof({
                _key: abi.encode(storageKey),
                _value: hex"01",
                _proof: _withdrawalProof,
                _root: _outputRootProof.messagePasserStorageRoot
            }) == false
        ) {
            revert OptimismPortal_InvalidMerkleProof();
        }

        // Designate the withdrawalHash as proven by storing the disputeGameProxy and timestamp in
        // the provenWithdrawals mapping. A given user may re-prove a withdrawalHash multiple
        // times, but each proof will reset the proof timer.
        provenWithdrawals[withdrawalHash][msg.sender] =
            ProvenWithdrawal({ disputeGameProxy: gameProxy, timestamp: uint64(block.timestamp) });

        // Add the proof submitter to the list of proof submitters for this withdrawal hash.
        proofSubmitters[withdrawalHash].push(msg.sender);

        // Emit a WithdrawalProven events.
        emit WithdrawalProven(withdrawalHash, _tx.sender, _tx.target);
        emit WithdrawalProvenExtension1(withdrawalHash, msg.sender);
    }

    /// @notice Finalizes a withdrawal transaction.
    /// @param _tx Withdrawal transaction to finalize.
    function finalizeWithdrawalTransaction(Types.WithdrawalTransaction memory _tx) external whenNotPaused {
        finalizeWithdrawalTransactionExternalProof(_tx, msg.sender);
    }

    /// @notice Finalizes a withdrawal transaction, using an external proof submitter.
    /// @param _tx Withdrawal transaction to finalize.
    /// @param _proofSubmitter Address of the proof submitter.
    function finalizeWithdrawalTransactionExternalProof(
        Types.WithdrawalTransaction memory _tx,
        address _proofSubmitter
    )
        public
        whenNotPaused
    {
        // Make sure that the l2Sender has not yet been set. The l2Sender is set to a value other
        // than the default value when a withdrawal transaction is being finalized. This check is
        // a defacto reentrancy guard.
        if (l2Sender != Constants.DEFAULT_L2_SENDER) {
            revert OptimismPortal_NoReentrancy();
        }

        // Make sure that the target address is safe.
        if (_isUnsafeTarget(_tx.target)) {
            revert OptimismPortal_BadTarget();
        }

        // Grab the withdrawal.
        bytes32 withdrawalHash = Hashing.hashWithdrawal(_tx);

        // Check that the withdrawal can be finalized.
        checkWithdrawal(withdrawalHash, _proofSubmitter);

        // Mark the withdrawal as finalized so it can't be replayed.
        finalizedWithdrawals[withdrawalHash] = true;

        // Set the l2Sender so contracts know who triggered this withdrawal on L2.
        l2Sender = _tx.sender;

        // Trigger the call to the target contract. We use a custom low level method
        // SafeCall.callWithMinGas to ensure two key properties
        //   1. Target contracts cannot force this call to run out of gas by returning a very large
        //      amount of data (and this is OK because we don't care about the returndata here).
        //   2. The amount of gas provided to the execution context of the target is at least the
        //      gas limit specified by the user. If there is not enough gas in the current context
        //      to accomplish this, `callWithMinGas` will revert.
        bool success = SafeCall.callWithMinGas(_tx.target, _tx.gasLimit, _tx.value, _tx.data);

        // Reset the l2Sender back to the default value.
        l2Sender = Constants.DEFAULT_L2_SENDER;

        // All withdrawals are immediately finalized. Replayability can
        // be achieved through contracts built on top of this contract
        emit WithdrawalFinalized(withdrawalHash, success);

        // Reverting here is useful for determining the exact gas cost to successfully execute the
        // sub call to the target contract if the minimum gas limit specified by the user would not
        // be sufficient to execute the sub call.
        if (!success && tx.origin == Constants.ESTIMATION_ADDRESS) {
            revert OptimismPortal_GasEstimation();
        }
    }

    /// @notice Checks that a withdrawal has been proven and is ready to be finalized.
    /// @param _withdrawalHash Hash of the withdrawal.
    /// @param _proofSubmitter Address of the proof submitter.
    function checkWithdrawal(bytes32 _withdrawalHash, address _proofSubmitter) public view {
        // Grab the withdrawal and dispute game proxy.
        ProvenWithdrawal memory provenWithdrawal = provenWithdrawals[_withdrawalHash][_proofSubmitter];
        IDisputeGame disputeGameProxy = provenWithdrawal.disputeGameProxy;

        // Check that this withdrawal has not already been finalized, this is replay protection.
        if (finalizedWithdrawals[_withdrawalHash]) {
            revert OptimismPortal_AlreadyFinalized();
        }

        // A withdrawal can only be finalized if it has been proven. We know that a withdrawal has
        // been proven at least once when its timestamp is non-zero. Unproven withdrawals will have
        // a timestamp of zero.
        if (provenWithdrawal.timestamp == 0) {
            revert OptimismPortal_Unproven();
        }

        // As a sanity check, we make sure that the proven withdrawal's timestamp is greater than
        // starting timestamp inside the Dispute Game. Not strictly necessary but extra layer of
        // safety against weird bugs in the proving step. Note that this blocks withdrawals that
        // are proven in the same block that a dispute game is created.
        if (provenWithdrawal.timestamp <= disputeGameProxy.createdAt().raw()) {
            revert OptimismPortal_InvalidProofTimestamp();
        }

        // A proven withdrawal must wait at least `PROOF_MATURITY_DELAY_SECONDS` before finalizing.
        if (block.timestamp - provenWithdrawal.timestamp <= PROOF_MATURITY_DELAY_SECONDS) {
            revert OptimismPortal_ProofNotOldEnough();
        }

        // Check that the root claim is valid.
        if (!anchorStateRegistry.isGameClaimValid(disputeGameProxy)) {
            revert OptimismPortal_InvalidRootClaim();
        }
    }

    /// @notice Accepts deposits of ETH and data, and emits a TransactionDeposited event for use in
    ///         deriving deposit transactions. Note that if a deposit is made by a contract, its
    ///         address will be aliased when retrieved using `tx.origin` or `msg.sender`. Consider
    ///         using the CrossDomainMessenger contracts for a simpler developer experience.
    /// @param _to         Target address on L2.
    /// @param _value      ETH value to send to the recipient.
    /// @param _gasLimit   Amount of L2 gas to purchase by burning gas on L1.
    /// @param _isCreation Whether or not the transaction is a contract creation.
    /// @param _data       Data to trigger the recipient with.
    function depositTransaction(
        address _to,
        uint256 _value,
        uint64 _gasLimit,
        bool _isCreation,
        bytes memory _data
    )
        public
        payable
        metered(_gasLimit)
    {
        // Just to be safe, make sure that people specify address(0) as the target when doing
        // contract creations.
        if (_isCreation && _to != address(0)) {
            revert OptimismPortal_BadTarget();
        }

        // Prevent depositing transactions that have too small of a gas limit. Users should pay
        // more for more resource usage.
        if (_gasLimit < minimumGasLimit(uint64(_data.length))) {
            revert OptimismPortal_GasLimitTooLow();
        }

        // Prevent the creation of deposit transactions that have too much calldata. This gives an
        // upper limit on the size of unsafe blocks over the p2p network. 120kb is chosen to ensure
        // that the transaction can fit into the p2p network policy of 128kb even though deposit
        // transactions are not gossipped over the p2p network.
        if (_data.length > 120_000) {
            revert OptimismPortal_CalldataTooLarge();
        }

        // Transform the from-address to its alias if the caller is a contract.
        address from = msg.sender;
        if (!EOA.isSenderEOA()) {
            from = AddressAliasHelper.applyL1ToL2Alias(msg.sender);
        }

        // Compute the opaque data that will be emitted as part of the TransactionDeposited event.
        // We use opaque data so that we can update the TransactionDeposited event in the future
        // without breaking the current interface.
        bytes memory opaqueData = abi.encodePacked(msg.value, _value, _gasLimit, _isCreation, _data);

        // Emit a TransactionDeposited event so that the rollup node can derive a deposit
        // transaction for this deposit.
        emit TransactionDeposited(from, _to, DEPOSIT_VERSION, opaqueData);
    }

    /// @notice External getter for the number of proof submitters for a withdrawal hash.
    /// @param _withdrawalHash Hash of the withdrawal.
    /// @return The number of proof submitters for the withdrawal hash.
    function numProofSubmitters(bytes32 _withdrawalHash) external view returns (uint256) {
        return proofSubmitters[_withdrawalHash].length;
    }

    /// @notice Checks if a target address is unsafe.
    function _isUnsafeTarget(address _target) internal view virtual returns (bool) {
        // Prevent users from creating a deposit transaction where this address is the message
        // sender on L2.
        return _target == address(this);
    }

    /// @notice Getter for the resource config. Used internally by the ResourceMetering contract.
    ///         The SystemConfig is the source of truth for the resource config.
    /// @return config_ ResourceMetering ResourceConfig
    function _resourceConfig() internal view override returns (ResourceMetering.ResourceConfig memory config_) {
        IResourceMetering.ResourceConfig memory config = systemConfig.resourceConfig();
        assembly ("memory-safe") {
            config_ := config
        }
    }
}
