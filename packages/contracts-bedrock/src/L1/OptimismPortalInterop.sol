// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { OptimismPortal2 } from "src/L1/OptimismPortal2.sol";

// Libraries
import { Predeploys } from "src/libraries/Predeploys.sol";
import { Constants } from "src/libraries/Constants.sol";
import { Unauthorized } from "src/libraries/PortalErrors.sol";
import { SecureMerkleTrie } from "src/libraries/trie/SecureMerkleTrie.sol";
import { Hashing } from "src/libraries/Hashing.sol";
import { LibSuperRoot, SuperRoot, OutputRootWithChainId, SuperRootProof } from "src/libraries/SuperRoot.sol";
import { Types } from "src/libraries/Types.sol";
import {
    BadTarget,
    InvalidGameType,
    InvalidProof,
    InvalidDisputeGame,
    InvalidMerkleProof,
    LegacyGame
} from "src/libraries/PortalErrors.sol";
import { GameType, Claim, GameStatus } from "src/dispute/lib/Types.sol";

// Interfaces
import { IL1BlockInterop, ConfigType } from "interfaces/L2/IL1BlockInterop.sol";
import { IDisputeGame } from "interfaces/dispute/IDisputeGame.sol";

/// @custom:proxied true
/// @title OptimismPortalInterop
/// @notice The OptimismPortal is a low-level contract responsible for passing messages between L1
///         and L2. Messages sent directly to the OptimismPortal have no form of replayability.
///         Users are encouraged to use the L1CrossDomainMessenger for a higher-level interface.
contract OptimismPortalInterop is OptimismPortal2 {
    constructor(
        uint256 _proofMaturityDelaySeconds,
        uint256 _disputeGameFinalityDelaySeconds
    )
        OptimismPortal2(_proofMaturityDelaySeconds, _disputeGameFinalityDelaySeconds)
    { }

    /// @custom:semver +interop.1
    function version() public pure override returns (string memory) {
        return string.concat(super.version(), "+interop.1");
    }

    /// @notice Sets static configuration options for the L2 system.
    /// @param _type  Type of configuration to set.
    /// @param _value Encoded value of the configuration.
    function setConfig(ConfigType _type, bytes memory _value) external {
        if (msg.sender != address(systemConfig)) revert Unauthorized();

        // Set L2 deposit gas as used without paying burning gas. Ensures that deposits cannot use too much L2 gas.
        // This value must be large enough to cover the cost of calling `L1Block.setConfig`.
        useGas(SYSTEM_DEPOSIT_GAS_LIMIT);

        // Emit the special deposit transaction directly that sets the config in the L1Block predeploy contract.
        emit TransactionDeposited(
            Constants.DEPOSITOR_ACCOUNT,
            Predeploys.L1_BLOCK_ATTRIBUTES,
            DEPOSIT_VERSION,
            abi.encodePacked(
                uint256(0), // mint
                uint256(0), // value
                uint64(SYSTEM_DEPOSIT_GAS_LIMIT), // gasLimit
                false, // isCreation,
                abi.encodeCall(IL1BlockInterop.setConfig, (_type, _value))
            )
        );
    }

    /// @notice Proves a withdrawal transaction.
    /// @param _tx               Withdrawal transaction to finalize.
    /// @param _disputeGameIndex Index of the dispute game to prove the withdrawal against.
    /// @param _superRootProof   Proof of the super root's preimage, the relevant L2 chain ID, and index of output root.
    /// @param _outputRootProof  Proof of the output root's preimage.
    /// @param _withdrawalProof  Inclusion proof of the withdrawal in L2ToL1MessagePasser contract.
    function proveWithdrawalTransaction(
        Types.WithdrawalTransaction memory _tx,
        uint256 _disputeGameIndex,
        SuperRootProof calldata _superRootProof,
        Types.OutputRootProof memory _outputRootProof,
        bytes[] calldata _withdrawalProof
    )
        external
        whenNotPaused
    {
        // Prevent users from creating a deposit transaction where this address is the message
        // sender on L2. Because this is checked here, we do not need to check again in
        // `finalizeWithdrawalTransaction`.
        if (_tx.target == address(this)) revert BadTarget();

        // Fetch the dispute game proxy from the `DisputeGameFactory` contract.
        (GameType gameType,, IDisputeGame gameProxy) = disputeGameFactory.gameAtIndex(_disputeGameIndex);
        Claim superRootCommitment = gameProxy.rootClaim();

        // The game type of the dispute game must be the respected game type.
        if (gameType.raw() != respectedGameType.raw()) revert InvalidGameType();

        // The game type of the DisputeGame must have been the respected game type at creation.
        try gameProxy.wasRespectedGameTypeWhenCreated() returns (bool wasRespected_) {
            if (!wasRespected_) revert InvalidGameType();
        } catch {
            revert LegacyGame();
        }

        // Game must have been created after the respected game type was updated. This check is a
        // strict inequality because we want to prevent users from being able to prove or finalize
        // withdrawals against games that were created in the same block that the retirement
        // timestamp was set. If the retirement timestamp and game type are changed in the same
        // block, such games could still be considered valid even if they used the old game type
        // that we intended to invalidate.
        require(
            gameProxy.createdAt().raw() > respectedGameTypeUpdatedAt,
            "OptimismPortal: dispute game created before respected game type was updated"
        );

        // We do not allow for proving withdrawals against dispute games that have resolved against the favor
        // of the root claim.
        if (gameProxy.status() == GameStatus.CHALLENGER_WINS) revert InvalidDisputeGame();

        // Verify that the super root committed to in the dispute game is the raw super root that was passed.
        if (superRootCommitment.raw() != keccak256(_superRootProof.rawSuperRoot)) revert InvalidProof();

        // Decode the SuperRoot type from the raw bytes. This function also verifies that the encoding
        // of the SuperRoot is correct, and will revert if it is malformatted.
        SuperRoot memory superRoot = LibSuperRoot.decode(_superRootProof.rawSuperRoot);

        // Find the output root within the super root that corresponds to the index passed.
        OutputRootWithChainId memory root = superRoot.outputRoots[_superRootProof.index];

        // Verify that the output root claims to be associated with the L2 chain ID passed.
        if (root.l2ChainId != _superRootProof.l2ChainId) revert InvalidProof();

        // Verify that the output root can be generated with the elements in the proof.
        if (root.outputRoot != Hashing.hashOutputRootProof(_outputRootProof)) revert InvalidProof();

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
        ) revert InvalidMerkleProof();

        // Designate the withdrawalHash as proven by storing the `disputeGameProxy` & `timestamp` in the
        // `provenWithdrawals` mapping. A `withdrawalHash` can only be proven once unless the dispute game it proved
        // against resolves against the favor of the root claim.
        provenWithdrawals[withdrawalHash][msg.sender] =
            ProvenWithdrawal({ disputeGameProxy: gameProxy, timestamp: uint64(block.timestamp) });

        // Emit a `WithdrawalProven` event.
        emit WithdrawalProven(withdrawalHash, _tx.sender, _tx.target);
        // Emit a `WithdrawalProvenExtension1` event.
        emit WithdrawalProvenExtension1(withdrawalHash, msg.sender);

        // Add the proof submitter to the list of proof submitters for this withdrawal hash.
        proofSubmitters[withdrawalHash].push(msg.sender);
    }

    /// @notice Deprecated function signature of `proveWithdrawalTransaction`. This function is no longer supported,
    ///         and will revert if called.
    function proveWithdrawalTransaction(
        Types.WithdrawalTransaction memory,
        uint256,
        Types.OutputRootProof calldata,
        bytes[] calldata
    )
        external
        view
        override
        whenNotPaused
    {
        revert("OptimismPortalInterop: this function is deprecated");
    }
}
