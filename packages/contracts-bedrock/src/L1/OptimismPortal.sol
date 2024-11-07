// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { Initializable } from "@openzeppelin/contracts/proxy/utils/Initializable.sol";
import { ResourceMetering } from "src/L1/ResourceMetering.sol";

// Libraries
import { SafeERC20 } from "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import { SafeCall } from "src/libraries/SafeCall.sol";
import { Constants } from "src/libraries/Constants.sol";
import { Types } from "src/libraries/Types.sol";
import { Hashing } from "src/libraries/Hashing.sol";
import { SecureMerkleTrie } from "src/libraries/trie/SecureMerkleTrie.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";
import { AddressAliasHelper } from "src/vendor/AddressAliasHelper.sol";
import {
    BadTarget,
    LargeCalldata,
    SmallGasLimit,
    TransferFailed,
    OnlyCustomGasToken,
    NoValue,
    Unauthorized,
    CallPaused,
    GasEstimation,
    NonReentrant,
    Unproven
} from "src/libraries/PortalErrors.sol";

// Interfaces
import { IERC20 } from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import { ISemver } from "src/universal/interfaces/ISemver.sol";
import { IL2OutputOracle } from "src/L1/interfaces/IL2OutputOracle.sol";
import { ISystemConfig } from "src/L1/interfaces/ISystemConfig.sol";
import { IResourceMetering } from "src/L1/interfaces/IResourceMetering.sol";
import { ISuperchainConfig } from "src/L1/interfaces/ISuperchainConfig.sol";
import { IL1Block } from "src/L2/interfaces/IL1Block.sol";

/// @custom:proxied true
/// @title OptimismPortal
/// @notice The OptimismPortal is a low-level contract responsible for passing messages between L1
///         and L2. Messages sent directly to the OptimismPortal have no form of replayability.
///         Users are encouraged to use the L1CrossDomainMessenger for a higher-level interface.
contract OptimismPortal is Initializable, ResourceMetering, ISemver {
    /// @notice Allows for interactions with non standard ERC20 tokens.
    using SafeERC20 for IERC20;

    /// @notice Represents a proven withdrawal.
    /// @custom:field outputRoot    Root of the L2 output this was proven against.
    /// @custom:field timestamp     Timestamp at whcih the withdrawal was proven.
    /// @custom:field l2OutputIndex Index of the output this was proven against.
    struct ProvenWithdrawal {
        bytes32 outputRoot;
        uint128 timestamp;
        uint128 l2OutputIndex;
    }

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

    /// @notice A mapping of withdrawal hashes to `ProvenWithdrawal` data.
    mapping(bytes32 => ProvenWithdrawal) public provenWithdrawals;

    /// @custom:legacy
    /// @custom:spacer paused
    /// @notice Spacer for backwards compatibility.
    bool private spacer_53_0_1;

    /// @notice Contract of the Superchain Config.
    ISuperchainConfig public superchainConfig;

    /// @notice Contract of the L2OutputOracle.
    /// @custom:network-specific
    IL2OutputOracle public l2Oracle;

    /// @notice Contract of the SystemConfig.
    /// @custom:network-specific
    ISystemConfig public systemConfig;

    /// @custom:spacer disputeGameFactory
    /// @notice Spacer for backwards compatibility.
    address private spacer_56_0_20;

    /// @custom:spacer provenWithdrawals
    /// @notice Spacer for backwards compatibility.
    bytes32 private spacer_57_0_32;

    /// @custom:spacer disputeGameBlacklist
    /// @notice Spacer for backwards compatibility.
    bytes32 private spacer_58_0_32;

    /// @custom:spacer respectedGameType + respectedGameTypeUpdatedAt
    /// @notice Spacer for backwards compatibility.
    bytes32 private spacer_59_0_32;

    /// @custom:spacer proofSubmitters
    /// @notice Spacer for backwards compatibility.
    bytes32 private spacer_60_0_32;

    /// @notice Represents the amount of native asset minted in L2. This may not
    ///         be 100% accurate due to the ability to send ether to the contract
    ///         without triggering a deposit transaction. It also is used to prevent
    ///         overflows for L2 account balances when custom gas tokens are used.
    ///         It is not safe to trust `ERC20.balanceOf` as it may lie.
    uint256 internal _balance;

    /// @notice Emitted when a transaction is deposited from L1 to L2.
    ///         The parameters of this event are read by the rollup node and used to derive deposit
    ///         transactions on L2.
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

    /// @notice Emitted when a withdrawal transaction is finalized.
    /// @param withdrawalHash Hash of the withdrawal transaction.
    /// @param success        Whether the withdrawal transaction was successful.
    event WithdrawalFinalized(bytes32 indexed withdrawalHash, bool success);

    /// @notice Reverts when paused.
    modifier whenNotPaused() {
        if (paused()) revert CallPaused();
        _;
    }

    /// @notice Semantic version.
    /// @custom:semver 2.8.1-beta.4
    function version() public pure virtual returns (string memory) {
        return "2.8.1-beta.4";
    }

    /// @notice Constructs the OptimismPortal contract.
    constructor() {
        initialize({
            _l2Oracle: IL2OutputOracle(address(0)),
            _systemConfig: ISystemConfig(address(0)),
            _superchainConfig: ISuperchainConfig(address(0))
        });
    }

    /// @notice Initializer.
    /// @param _l2Oracle Contract of the L2OutputOracle.
    /// @param _systemConfig Contract of the SystemConfig.
    /// @param _superchainConfig Contract of the SuperchainConfig.
    function initialize(
        IL2OutputOracle _l2Oracle,
        ISystemConfig _systemConfig,
        ISuperchainConfig _superchainConfig
    )
        public
        initializer
    {
        l2Oracle = _l2Oracle;
        systemConfig = _systemConfig;
        superchainConfig = _superchainConfig;
        if (l2Sender == address(0)) {
            l2Sender = Constants.DEFAULT_L2_SENDER;
        }
        __ResourceMetering_init();
    }

    /// @notice Getter for the balance of the contract.
    function balance() public view returns (uint256) {
        (address token,) = gasPayingToken();
        if (token == Constants.ETHER) {
            return address(this).balance;
        } else {
            return _balance;
        }
    }

    /// @notice Getter function for the address of the guardian.
    ///         Public getter is legacy and will be removed in the future. Use `SuperchainConfig.guardian()` instead.
    /// @return Address of the guardian.
    /// @custom:legacy
    function guardian() public view returns (address) {
        return superchainConfig.guardian();
    }

    /// @notice Getter for the current paused status.
    /// @return paused_ Whether or not the contract is paused.
    function paused() public view returns (bool paused_) {
        paused_ = superchainConfig.paused();
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

    /// @notice Returns the gas paying token and its decimals.
    function gasPayingToken() internal view returns (address addr_, uint8 decimals_) {
        (addr_, decimals_) = systemConfig.gasPayingToken();
    }

    /// @notice Getter for the resource config.
    ///         Used internally by the ResourceMetering contract.
    ///         The SystemConfig is the source of truth for the resource config.
    /// @return ResourceMetering ResourceConfig
    function _resourceConfig() internal view override returns (ResourceConfig memory) {
        IResourceMetering.ResourceConfig memory config = systemConfig.resourceConfig();
        return ResourceConfig({
            maxResourceLimit: config.maxResourceLimit,
            elasticityMultiplier: config.elasticityMultiplier,
            baseFeeMaxChangeDenominator: config.baseFeeMaxChangeDenominator,
            minimumBaseFee: config.minimumBaseFee,
            systemTxMaxGas: config.systemTxMaxGas,
            maximumBaseFee: config.maximumBaseFee
        });
    }

    /// @notice Proves a withdrawal transaction.
    /// @param _tx              Withdrawal transaction to finalize.
    /// @param _l2OutputIndex   L2 output index to prove against.
    /// @param _outputRootProof Inclusion proof of the L2ToL1MessagePasser contract's storage root.
    /// @param _withdrawalProof Inclusion proof of the withdrawal in L2ToL1MessagePasser contract.
    function proveWithdrawalTransaction(
        Types.WithdrawalTransaction memory _tx,
        uint256 _l2OutputIndex,
        Types.OutputRootProof calldata _outputRootProof,
        bytes[] calldata _withdrawalProof
    )
        external
        whenNotPaused
    {
        // Prevent users from creating a deposit transaction where this address is the message
        // sender on L2. Because this is checked here, we do not need to check again in
        // `finalizeWithdrawalTransaction`.
        if (_tx.target == address(this)) revert BadTarget();

        // Get the output root and load onto the stack to prevent multiple mloads. This will
        // revert if there is no output root for the given block number.
        bytes32 outputRoot = l2Oracle.getL2Output(_l2OutputIndex).outputRoot;

        // Verify that the output root can be generated with the elements in the proof.
        require(
            outputRoot == Hashing.hashOutputRootProof(_outputRootProof), "OptimismPortal: invalid output root proof"
        );

        // Load the ProvenWithdrawal into memory, using the withdrawal hash as a unique identifier.
        bytes32 withdrawalHash = Hashing.hashWithdrawal(_tx);
        ProvenWithdrawal memory provenWithdrawal = provenWithdrawals[withdrawalHash];

        // We generally want to prevent users from proving the same withdrawal multiple times
        // because each successive proof will update the timestamp. A malicious user can take
        // advantage of this to prevent other users from finalizing their withdrawal. However,
        // since withdrawals are proven before an output root is finalized, we need to allow users
        // to re-prove their withdrawal only in the case that the output root for their specified
        // output index has been updated.
        require(
            provenWithdrawal.timestamp == 0
                || l2Oracle.getL2Output(provenWithdrawal.l2OutputIndex).outputRoot != provenWithdrawal.outputRoot,
            "OptimismPortal: withdrawal hash has already been proven"
        );

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
        require(
            SecureMerkleTrie.verifyInclusionProof({
                _key: abi.encode(storageKey),
                _value: hex"01",
                _proof: _withdrawalProof,
                _root: _outputRootProof.messagePasserStorageRoot
            }),
            "OptimismPortal: invalid withdrawal inclusion proof"
        );

        // Designate the withdrawalHash as proven by storing the `outputRoot`, `timestamp`, and
        // `l2BlockNumber` in the `provenWithdrawals` mapping. A `withdrawalHash` can only be
        // proven once unless it is submitted again with a different outputRoot.
        provenWithdrawals[withdrawalHash] = ProvenWithdrawal({
            outputRoot: outputRoot,
            timestamp: uint128(block.timestamp),
            l2OutputIndex: uint128(_l2OutputIndex)
        });

        // Emit a `WithdrawalProven` event.
        emit WithdrawalProven(withdrawalHash, _tx.sender, _tx.target);
    }

    /// @notice Finalizes a withdrawal transaction.
    /// @param _tx Withdrawal transaction to finalize.
    function finalizeWithdrawalTransaction(Types.WithdrawalTransaction memory _tx) external whenNotPaused {
        // Make sure that the l2Sender has not yet been set. The l2Sender is set to a value other
        // than the default value when a withdrawal transaction is being finalized. This check is
        // a defacto reentrancy guard.
        if (l2Sender != Constants.DEFAULT_L2_SENDER) revert NonReentrant();

        // Grab the proven withdrawal from the `provenWithdrawals` map.
        bytes32 withdrawalHash = Hashing.hashWithdrawal(_tx);
        ProvenWithdrawal memory provenWithdrawal = provenWithdrawals[withdrawalHash];

        // A withdrawal can only be finalized if it has been proven. We know that a withdrawal has
        // been proven at least once when its timestamp is non-zero. Unproven withdrawals will have
        // a timestamp of zero.
        require(provenWithdrawal.timestamp != 0, "OptimismPortal: withdrawal has not been proven yet");

        // As a sanity check, we make sure that the proven withdrawal's timestamp is greater than
        // starting timestamp inside the L2OutputOracle. Not strictly necessary but extra layer of
        // safety against weird bugs in the proving step.
        require(
            provenWithdrawal.timestamp >= l2Oracle.startingTimestamp(),
            "OptimismPortal: withdrawal timestamp less than L2 Oracle starting timestamp"
        );

        // A proven withdrawal must wait at least the finalization period before it can be
        // finalized. This waiting period can elapse in parallel with the waiting period for the
        // output the withdrawal was proven against. In effect, this means that the minimum
        // withdrawal time is proposal submission time + finalization period.
        require(
            _isFinalizationPeriodElapsed(provenWithdrawal.timestamp),
            "OptimismPortal: proven withdrawal finalization period has not elapsed"
        );

        // Grab the OutputProposal from the L2OutputOracle, will revert if the output that
        // corresponds to the given index has not been proposed yet.
        Types.OutputProposal memory proposal = l2Oracle.getL2Output(provenWithdrawal.l2OutputIndex);

        // Check that the output root that was used to prove the withdrawal is the same as the
        // current output root for the given output index. An output root may change if it is
        // deleted by the challenger address and then re-proposed.
        require(
            proposal.outputRoot == provenWithdrawal.outputRoot,
            "OptimismPortal: output root proven is not the same as current output root"
        );

        // Check that the output proposal has also been finalized.
        require(
            _isFinalizationPeriodElapsed(proposal.timestamp),
            "OptimismPortal: output proposal finalization period has not elapsed"
        );

        // Check that this withdrawal has not already been finalized, this is replay protection.
        require(finalizedWithdrawals[withdrawalHash] == false, "OptimismPortal: withdrawal has already been finalized");

        // Mark the withdrawal as finalized so it can't be replayed.
        finalizedWithdrawals[withdrawalHash] = true;

        // Set the l2Sender so contracts know who triggered this withdrawal on L2.
        // This acts as a reentrancy guard.
        l2Sender = _tx.sender;

        bool success;
        (address token,) = gasPayingToken();
        if (token == Constants.ETHER) {
            // Trigger the call to the target contract. We use a custom low level method
            // SafeCall.callWithMinGas to ensure two key properties
            //   1. Target contracts cannot force this call to run out of gas by returning a very large
            //      amount of data (and this is OK because we don't care about the returndata here).
            //   2. The amount of gas provided to the execution context of the target is at least the
            //      gas limit specified by the user. If there is not enough gas in the current context
            //      to accomplish this, `callWithMinGas` will revert.
            success = SafeCall.callWithMinGas(_tx.target, _tx.gasLimit, _tx.value, _tx.data);
        } else {
            // Cannot call the token contract directly from the portal. This would allow an attacker
            // to call approve from a withdrawal and drain the balance of the portal.
            if (_tx.target == token) revert BadTarget();

            // Only transfer value when a non zero value is specified. This saves gas in the case of
            // using the standard bridge or arbitrary message passing.
            if (_tx.value != 0) {
                // Update the contracts internal accounting of the amount of native asset in L2.
                _balance -= _tx.value;

                // Read the balance of the target contract before the transfer so the consistency
                // of the transfer can be checked afterwards.
                uint256 startBalance = IERC20(token).balanceOf(address(this));

                // Transfer the ERC20 balance to the target, accounting for non standard ERC20
                // implementations that may not return a boolean. This reverts if the low level
                // call is not successful.
                IERC20(token).safeTransfer({ to: _tx.target, value: _tx.value });

                // The balance must be transferred exactly.
                if (IERC20(token).balanceOf(address(this)) != startBalance - _tx.value) {
                    revert TransferFailed();
                }
            }

            // Make a call to the target contract only if there is calldata.
            if (_tx.data.length != 0) {
                success = SafeCall.callWithMinGas(_tx.target, _tx.gasLimit, 0, _tx.data);
            } else {
                success = true;
            }
        }

        // Reset the l2Sender back to the default value.
        l2Sender = Constants.DEFAULT_L2_SENDER;

        // All withdrawals are immediately finalized. Replayability can
        // be achieved through contracts built on top of this contract
        emit WithdrawalFinalized(withdrawalHash, success);

        // Reverting here is useful for determining the exact gas cost to successfully execute the
        // sub call to the target contract if the minimum gas limit specified by the user would not
        // be sufficient to execute the sub call.
        if (success == false && tx.origin == Constants.ESTIMATION_ADDRESS) {
            revert GasEstimation();
        }
    }

    /// @notice Entrypoint to depositing an ERC20 token as a custom gas token.
    ///         This function depends on a well formed ERC20 token. There are only
    ///         so many checks that can be done on chain for this so it is assumed
    ///         that chain operators will deploy chains with well formed ERC20 tokens.
    /// @param _to         Target address on L2.
    /// @param _mint       Units of ERC20 token to deposit into L2.
    /// @param _value      Units of ERC20 token to send on L2 to the recipient.
    /// @param _gasLimit   Amount of L2 gas to purchase by burning gas on L1.
    /// @param _isCreation Whether or not the transaction is a contract creation.
    /// @param _data       Data to trigger the recipient with.
    function depositERC20Transaction(
        address _to,
        uint256 _mint,
        uint256 _value,
        uint64 _gasLimit,
        bool _isCreation,
        bytes memory _data
    )
        public
        metered(_gasLimit)
    {
        // Can only be called if an ERC20 token is used for gas paying on L2
        (address token,) = gasPayingToken();
        if (token == Constants.ETHER) revert OnlyCustomGasToken();

        // Gives overflow protection for L2 account balances.
        _balance += _mint;

        // Get the balance of the portal before the transfer.
        uint256 startBalance = IERC20(token).balanceOf(address(this));

        // Take ownership of the token. It is assumed that the user has given the portal an approval.
        IERC20(token).safeTransferFrom({ from: msg.sender, to: address(this), value: _mint });

        // Double check that the portal now has the exact amount of token.
        if (IERC20(token).balanceOf(address(this)) != startBalance + _mint) {
            revert TransferFailed();
        }

        _depositTransaction({
            _to: _to,
            _mint: _mint,
            _value: _value,
            _gasLimit: _gasLimit,
            _isCreation: _isCreation,
            _data: _data
        });
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
        (address token,) = gasPayingToken();
        if (token != Constants.ETHER && msg.value != 0) revert NoValue();

        _depositTransaction({
            _to: _to,
            _mint: msg.value,
            _value: _value,
            _gasLimit: _gasLimit,
            _isCreation: _isCreation,
            _data: _data
        });
    }

    /// @notice Common logic for creating deposit transactions.
    /// @param _to         Target address on L2.
    /// @param _mint       Units of asset to deposit into L2.
    /// @param _value      Units of asset to send on L2 to the recipient.
    /// @param _gasLimit   Amount of L2 gas to purchase by burning gas on L1.
    /// @param _isCreation Whether or not the transaction is a contract creation.
    /// @param _data       Data to trigger the recipient with.
    function _depositTransaction(
        address _to,
        uint256 _mint,
        uint256 _value,
        uint64 _gasLimit,
        bool _isCreation,
        bytes memory _data
    )
        internal
    {
        // Just to be safe, make sure that people specify address(0) as the target when doing
        // contract creations.
        if (_isCreation && _to != address(0)) revert BadTarget();

        // Prevent depositing transactions that have too small of a gas limit. Users should pay
        // more for more resource usage.
        if (_gasLimit < minimumGasLimit(uint64(_data.length))) revert SmallGasLimit();

        // Prevent the creation of deposit transactions that have too much calldata. This gives an
        // upper limit on the size of unsafe blocks over the p2p network. 120kb is chosen to ensure
        // that the transaction can fit into the p2p network policy of 128kb even though deposit
        // transactions are not gossipped over the p2p network.
        if (_data.length > 120_000) revert LargeCalldata();

        // Transform the from-address to its alias if the caller is a contract.
        address from = msg.sender;
        if (msg.sender != tx.origin) {
            from = AddressAliasHelper.applyL1ToL2Alias(msg.sender);
        }

        // Compute the opaque data that will be emitted as part of the TransactionDeposited event.
        // We use opaque data so that we can update the TransactionDeposited event in the future
        // without breaking the current interface.
        bytes memory opaqueData = abi.encodePacked(_mint, _value, _gasLimit, _isCreation, _data);

        // Emit a TransactionDeposited event so that the rollup node can derive a deposit
        // transaction for this deposit.
        emit TransactionDeposited(from, _to, DEPOSIT_VERSION, opaqueData);
    }

    /// @notice Sets the gas paying token for the L2 system. This token is used as the
    ///         L2 native asset. Only the SystemConfig contract can call this function.
    function setGasPayingToken(address _token, uint8 _decimals, bytes32 _name, bytes32 _symbol) external {
        if (msg.sender != address(systemConfig)) revert Unauthorized();

        // Set L2 deposit gas as used without paying burning gas. Ensures that deposits cannot use too much L2 gas.
        // This value must be large enough to cover the cost of calling `L1Block.setGasPayingToken`.
        useGas(SYSTEM_DEPOSIT_GAS_LIMIT);

        // Emit the special deposit transaction directly that sets the gas paying
        // token in the L1Block predeploy contract.
        emit TransactionDeposited(
            Constants.DEPOSITOR_ACCOUNT,
            Predeploys.L1_BLOCK_ATTRIBUTES,
            DEPOSIT_VERSION,
            abi.encodePacked(
                uint256(0), // mint
                uint256(0), // value
                uint64(SYSTEM_DEPOSIT_GAS_LIMIT), // gasLimit
                false, // isCreation,
                abi.encodeCall(IL1Block.setGasPayingToken, (_token, _decimals, _name, _symbol))
            )
        );
    }

    /// @notice Determine if a given output is finalized.
    ///         Reverts if the call to l2Oracle.getL2Output reverts.
    ///         Returns a boolean otherwise.
    /// @param _l2OutputIndex Index of the L2 output to check.
    /// @return Whether or not the output is finalized.
    function isOutputFinalized(uint256 _l2OutputIndex) external view returns (bool) {
        return _isFinalizationPeriodElapsed(l2Oracle.getL2Output(_l2OutputIndex).timestamp);
    }

    /// @notice Determines whether the finalization period has elapsed with respect to
    ///         the provided block timestamp.
    /// @param _timestamp Timestamp to check.
    /// @return Whether or not the finalization period has elapsed.
    function _isFinalizationPeriodElapsed(uint256 _timestamp) internal view returns (bool) {
        return block.timestamp > _timestamp + l2Oracle.FINALIZATION_PERIOD_SECONDS();
    }
}
