package engineapi

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/misc/eip1559"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/stateless"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

type EngineBackend interface {
	CurrentSafeBlock() *types.Header
	CurrentFinalBlock() *types.Header
	GetBlockByHash(hash common.Hash) *types.Block
	GetBlock(hash common.Hash, number uint64) *types.Block
	HasBlockAndState(hash common.Hash, number uint64) bool
	GetCanonicalHash(n uint64) common.Hash

	GetVMConfig() *vm.Config
	Config() *params.ChainConfig
	// Engine retrieves the chain's consensus engine.
	Engine() consensus.Engine

	StateAt(root common.Hash) (*state.StateDB, error)

	InsertBlockWithoutSetHead(block *types.Block, makeWitness bool) (*stateless.Witness, error)
	SetCanonical(head *types.Block) (common.Hash, error)
	SetFinalized(header *types.Header)
	SetSafe(header *types.Header)

	consensus.ChainHeaderReader
}

type CachingEngineBackend interface {
	EngineBackend
	AssembleAndInsertBlockWithoutSetHead(processor *BlockProcessor) (*types.Block, error)
}

// L2EngineAPI wraps an engine actor, and implements the RPC backend required to serve the engine API.
// This re-implements some of the Geth API work, but changes the API backend so we can deterministically
// build and control the L2 block contents to reach very specific edge cases as desired for testing.
type L2EngineAPI struct {
	log     log.Logger
	backend EngineBackend

	// Functionality for snap sync
	remotes    map[common.Hash]*types.Block
	downloader *downloader.Downloader

	// L2 block building data
	blockProcessor *BlockProcessor
	pendingIndices map[common.Address]uint64 // per account, how many txs from the pool were already included in the block, since the pool is lagging behind block mining.
	l2ForceEmpty   bool                      // when no additional txs may be processed (i.e. when sequencer drift runs out)
	l2TxFailed     []*types.Transaction      // log of failed transactions which could not be included

	payloadID engine.PayloadID // ID of payload that is currently being built
}

func NewL2EngineAPI(log log.Logger, backend EngineBackend, downloader *downloader.Downloader) *L2EngineAPI {
	return &L2EngineAPI{
		log:        log,
		backend:    backend,
		remotes:    make(map[common.Hash]*types.Block),
		downloader: downloader,
	}
}

var (
	STATUS_INVALID = &eth.ForkchoiceUpdatedResult{PayloadStatus: eth.PayloadStatusV1{Status: eth.ExecutionInvalid}, PayloadID: nil}
	STATUS_SYNCING = &eth.ForkchoiceUpdatedResult{PayloadStatus: eth.PayloadStatusV1{Status: eth.ExecutionSyncing}, PayloadID: nil}
)

// computePayloadId computes a pseudo-random payloadid, based on the parameters.
func computePayloadId(headBlockHash common.Hash, attrs *eth.PayloadAttributes) engine.PayloadID {
	// Hash
	hasher := sha256.New()
	hasher.Write(headBlockHash[:])
	_ = binary.Write(hasher, binary.BigEndian, attrs.Timestamp)
	hasher.Write(attrs.PrevRandao[:])
	hasher.Write(attrs.SuggestedFeeRecipient[:])
	_ = binary.Write(hasher, binary.BigEndian, attrs.NoTxPool)
	_ = binary.Write(hasher, binary.BigEndian, uint64(len(attrs.Transactions)))
	for _, tx := range attrs.Transactions {
		_ = binary.Write(hasher, binary.BigEndian, uint64(len(tx))) // length-prefix to avoid collisions
		hasher.Write(tx)
	}
	_ = binary.Write(hasher, binary.BigEndian, *attrs.GasLimit)
	if attrs.EIP1559Params != nil {
		hasher.Write(attrs.EIP1559Params[:])
	}
	var out engine.PayloadID
	copy(out[:], hasher.Sum(nil)[:8])
	return out
}

func (ea *L2EngineAPI) RemainingBlockGas() uint64 {
	if ea.blockProcessor == nil {
		return 0
	}
	return ea.blockProcessor.gasPool.Gas()
}

func (ea *L2EngineAPI) ForcedEmpty() bool {
	return ea.l2ForceEmpty
}

func (ea *L2EngineAPI) PendingIndices(from common.Address) uint64 {
	return ea.pendingIndices[from]
}

var ErrNotBuildingBlock = errors.New("not currently building a block, cannot include tx from queue")

func (ea *L2EngineAPI) IncludeTx(tx *types.Transaction, from common.Address) error {
	if ea.blockProcessor == nil {
		return ErrNotBuildingBlock
	}

	if ea.l2ForceEmpty {
		ea.log.Info("Skipping including a transaction because e.L2ForceEmpty is true")
		return nil
	}

	err := ea.blockProcessor.CheckTxWithinGasLimit(tx)
	if err != nil {
		return err
	}

	ea.pendingIndices[from] = ea.pendingIndices[from] + 1 // won't retry the tx
	err = ea.blockProcessor.AddTx(tx)
	if err != nil {
		ea.l2TxFailed = append(ea.l2TxFailed, tx)
		return fmt.Errorf("invalid L2 block (tx %d): %w", len(ea.blockProcessor.transactions), err)
	}
	return nil
}

func (ea *L2EngineAPI) startBlock(parent common.Hash, attrs *eth.PayloadAttributes) error {
	if ea.blockProcessor != nil {
		ea.log.Warn("started building new block without ending previous block", "previous", ea.blockProcessor.header, "prev_payload_id", ea.payloadID)
	}

	processor, err := NewBlockProcessorFromPayloadAttributes(ea.backend, parent, attrs)
	if err != nil {
		return err
	}

	ea.blockProcessor = processor
	ea.pendingIndices = make(map[common.Address]uint64)
	ea.l2ForceEmpty = attrs.NoTxPool
	ea.payloadID = computePayloadId(parent, attrs)

	// pre-process the deposits
	for i, otx := range attrs.Transactions {
		var tx types.Transaction
		if err := tx.UnmarshalBinary(otx); err != nil {
			return fmt.Errorf("transaction %d is not valid: %w", i, err)
		}
		err := ea.blockProcessor.AddTx(&tx)
		if err != nil {
			ea.l2TxFailed = append(ea.l2TxFailed, &tx)
			return fmt.Errorf("failed to apply deposit transaction to L2 block (tx %d): %w", i, err)
		}
	}
	return nil
}

func (ea *L2EngineAPI) endBlock() (*types.Block, error) {
	if ea.blockProcessor == nil {
		return nil, fmt.Errorf("no block is being built currently (id %s)", ea.payloadID)
	}
	processor := ea.blockProcessor
	ea.blockProcessor = nil

	var block *types.Block
	var err error
	// If the backend supports it, write the newly created block to the database without making it canonical.
	// This avoids needing to reprocess the block if it is sent back via newPayload.
	// The block is not made canonical so if it is never sent back via newPayload worst case it just wastes some storage
	// In the context of the OP Stack derivation, the created block is always immediately imported so it makes sense to
	// optimise.
	if cachingBackend, ok := ea.backend.(CachingEngineBackend); ok {
		block, err = cachingBackend.AssembleAndInsertBlockWithoutSetHead(processor)
	} else {
		block, err = processor.Assemble()
	}
	if err != nil {
		return nil, fmt.Errorf("assemble block: %w", err)
	}
	return block, nil
}

func (ea *L2EngineAPI) GetPayloadV1(ctx context.Context, payloadId eth.PayloadID) (*eth.ExecutionPayload, error) {
	res, err := ea.getPayload(ctx, payloadId)
	if err != nil {
		return nil, err
	}
	return res.ExecutionPayload, nil
}

func (ea *L2EngineAPI) GetPayloadV2(ctx context.Context, payloadId eth.PayloadID) (*eth.ExecutionPayloadEnvelope, error) {
	return ea.getPayload(ctx, payloadId)
}

func (ea *L2EngineAPI) GetPayloadV3(ctx context.Context, payloadId eth.PayloadID) (*eth.ExecutionPayloadEnvelope, error) {
	return ea.getPayload(ctx, payloadId)
}

func (ea *L2EngineAPI) config() *params.ChainConfig {
	return ea.backend.Config()
}

func (ea *L2EngineAPI) ForkchoiceUpdatedV1(ctx context.Context, state *eth.ForkchoiceState, attr *eth.PayloadAttributes) (*eth.ForkchoiceUpdatedResult, error) {
	if attr != nil {
		if attr.Withdrawals != nil {
			return STATUS_INVALID, engine.InvalidParams.With(errors.New("withdrawals not supported in V1"))
		}
		if ea.config().IsShanghai(ea.config().LondonBlock, uint64(attr.Timestamp)) {
			return STATUS_INVALID, engine.InvalidParams.With(errors.New("forkChoiceUpdateV1 called post-shanghai"))
		}
	}

	return ea.forkchoiceUpdated(ctx, state, attr)
}

func (ea *L2EngineAPI) ForkchoiceUpdatedV2(ctx context.Context, state *eth.ForkchoiceState, attr *eth.PayloadAttributes) (*eth.ForkchoiceUpdatedResult, error) {
	if attr != nil {
		if err := ea.verifyPayloadAttributes(attr); err != nil {
			return STATUS_INVALID, engine.InvalidParams.With(err)
		}
	}

	return ea.forkchoiceUpdated(ctx, state, attr)
}

// Ported from: https://github.com/ethereum-optimism/op-geth/blob/c50337a60a1309a0f1dca3bf33ed1bb38c46cdd7/eth/catalyst/api.go#L197C1-L205C1
func (ea *L2EngineAPI) ForkchoiceUpdatedV3(ctx context.Context, state *eth.ForkchoiceState, attr *eth.PayloadAttributes) (*eth.ForkchoiceUpdatedResult, error) {
	if attr != nil {
		if err := ea.verifyPayloadAttributes(attr); err != nil {
			return STATUS_INVALID, engine.InvalidParams.With(err)
		}
	}
	return ea.forkchoiceUpdated(ctx, state, attr)
}

// Ported from: https://github.com/ethereum-optimism/op-geth/blob/c50337a60a1309a0f1dca3bf33ed1bb38c46cdd7/eth/catalyst/api.go#L206-L218
func (ea *L2EngineAPI) verifyPayloadAttributes(attr *eth.PayloadAttributes) error {
	c := ea.config()
	t := uint64(attr.Timestamp)

	// Verify withdrawals attribute for Shanghai.
	if err := checkAttribute(c.IsShanghai, attr.Withdrawals != nil, c.LondonBlock, t); err != nil {
		return fmt.Errorf("invalid withdrawals: %w", err)
	}
	// Verify beacon root attribute for Cancun.
	if err := checkAttribute(c.IsCancun, attr.ParentBeaconBlockRoot != nil, c.LondonBlock, t); err != nil {
		return fmt.Errorf("invalid parent beacon block root: %w", err)
	}
	// Verify EIP-1559 params for Holocene.
	if c.IsHolocene(t) {
		if attr.EIP1559Params == nil {
			return errors.New("got nil eip-1559 params while Holocene is active")
		}
		if err := eip1559.ValidateHolocene1559Params(attr.EIP1559Params[:]); err != nil {
			return fmt.Errorf("invalid Holocene params: %w", err)
		}
	} else if attr.EIP1559Params != nil {
		return errors.New("got Holocene params though fork not active")
	}

	return nil
}

func checkAttribute(active func(*big.Int, uint64) bool, exists bool, block *big.Int, time uint64) error {
	if active(block, time) && !exists {
		return errors.New("fork active, missing expected attribute")
	}
	if !active(block, time) && exists {
		return errors.New("fork inactive, unexpected attribute set")
	}
	return nil
}

func (ea *L2EngineAPI) NewPayloadV1(ctx context.Context, payload *eth.ExecutionPayload) (*eth.PayloadStatusV1, error) {
	if payload.Withdrawals != nil {
		return &eth.PayloadStatusV1{Status: eth.ExecutionInvalid}, engine.InvalidParams.With(errors.New("withdrawals not supported in V1"))
	}

	return ea.newPayload(ctx, payload, nil, nil)
}

func (ea *L2EngineAPI) NewPayloadV2(ctx context.Context, payload *eth.ExecutionPayload) (*eth.PayloadStatusV1, error) {
	if ea.config().IsShanghai(new(big.Int).SetUint64(uint64(payload.BlockNumber)), uint64(payload.Timestamp)) {
		if payload.Withdrawals == nil {
			return &eth.PayloadStatusV1{Status: eth.ExecutionInvalid}, engine.InvalidParams.With(errors.New("nil withdrawals post-shanghai"))
		}
	} else if payload.Withdrawals != nil {
		return &eth.PayloadStatusV1{Status: eth.ExecutionInvalid}, engine.InvalidParams.With(errors.New("non-nil withdrawals pre-shanghai"))
	}

	return ea.newPayload(ctx, payload, nil, nil)
}

// Ported from: https://github.com/ethereum-optimism/op-geth/blob/c50337a60a1309a0f1dca3bf33ed1bb38c46cdd7/eth/catalyst/api.go#L486C1-L507
func (ea *L2EngineAPI) NewPayloadV3(ctx context.Context, params *eth.ExecutionPayload, versionedHashes []common.Hash, beaconRoot *common.Hash) (*eth.PayloadStatusV1, error) {
	if params.ExcessBlobGas == nil {
		return &eth.PayloadStatusV1{Status: eth.ExecutionInvalid}, engine.InvalidParams.With(errors.New("nil excessBlobGas post-cancun"))
	}
	if params.BlobGasUsed == nil {
		return &eth.PayloadStatusV1{Status: eth.ExecutionInvalid}, engine.InvalidParams.With(errors.New("nil params.BlobGasUsed post-cancun"))
	}
	if versionedHashes == nil {
		return &eth.PayloadStatusV1{Status: eth.ExecutionInvalid}, engine.InvalidParams.With(errors.New("nil versionedHashes post-cancun"))
	}
	if beaconRoot == nil {
		return &eth.PayloadStatusV1{Status: eth.ExecutionInvalid}, engine.InvalidParams.With(errors.New("nil parentBeaconBlockRoot post-cancun"))
	}

	if !ea.config().IsCancun(new(big.Int).SetUint64(uint64(params.BlockNumber)), uint64(params.Timestamp)) {
		return &eth.PayloadStatusV1{Status: eth.ExecutionInvalid}, engine.UnsupportedFork.With(errors.New("newPayloadV3 called pre-cancun"))
	}

	// Payload must have eip-1559 params in ExtraData after Holocene
	if ea.config().IsHolocene(uint64(params.Timestamp)) {
		if err := eip1559.ValidateHoloceneExtraData(params.ExtraData); err != nil {
			return &eth.PayloadStatusV1{Status: eth.ExecutionInvalid}, engine.UnsupportedFork.With(errors.New("invalid holocene extraData post-holoocene"))
		}
	}

	return ea.newPayload(ctx, params, versionedHashes, beaconRoot)
}

func (ea *L2EngineAPI) getPayload(_ context.Context, payloadId eth.PayloadID) (*eth.ExecutionPayloadEnvelope, error) {
	ea.log.Trace("L2Engine API request received", "method", "GetPayload", "id", payloadId)
	if ea.payloadID != payloadId {
		ea.log.Warn("unexpected payload ID requested for block building", "expected", ea.payloadID, "got", payloadId)
		return nil, engine.UnknownPayload
	}
	bl, err := ea.endBlock()
	if err != nil {
		ea.log.Error("failed to finish block building", "err", err)
		return nil, engine.UnknownPayload
	}

	return eth.BlockAsPayloadEnv(bl, ea.config().ShanghaiTime)
}

func (ea *L2EngineAPI) forkchoiceUpdated(_ context.Context, state *eth.ForkchoiceState, attr *eth.PayloadAttributes) (*eth.ForkchoiceUpdatedResult, error) {
	ea.log.Trace("L2Engine API request received", "method", "ForkchoiceUpdated", "head", state.HeadBlockHash, "finalized", state.FinalizedBlockHash, "safe", state.SafeBlockHash)
	if state.HeadBlockHash == (common.Hash{}) {
		ea.log.Warn("Forkchoice requested update to zero hash")
		return STATUS_INVALID, nil
	}
	// Check whether we have the block yet in our database or not. If not, we'll
	// need to either trigger a sync, or to reject this forkchoice update for a
	// reason.
	block := ea.backend.GetBlockByHash(state.HeadBlockHash)
	if block == nil {
		if ea.downloader == nil {
			ea.log.Warn("Must register downloader to be able to snap sync")
			return STATUS_SYNCING, nil
		}
		// If the head hash is unknown (was not given to us in a newPayload request),
		// we cannot resolve the header, so not much to do. This could be extended in
		// the future to resolve from the `eth` network, but it's an unexpected case
		// that should be fixed, not papered over.
		header := ea.remotes[state.HeadBlockHash]
		if header == nil {
			ea.log.Warn("Forkchoice requested unknown head", "hash", state.HeadBlockHash)
			return STATUS_SYNCING, nil
		}

		ea.log.Info("Forkchoice requested sync to new head", "number", header.Number(), "hash", header.Hash())
		if err := ea.downloader.BeaconSync(downloader.SnapSync, header.Header(), nil); err != nil {
			return STATUS_SYNCING, err
		}
		return STATUS_SYNCING, nil
	}
	// Block is known locally, just sanity check that the beacon client does not
	// attempt to push us back to before the merge.
	// Note: Differs from op-geth implementation as pre-merge blocks are never supported here
	if block.Difficulty().BitLen() > 0 && block.NumberU64() > 0 {
		return STATUS_INVALID, errors.New("pre-merge blocks not supported")
	}
	valid := func(id *engine.PayloadID) *eth.ForkchoiceUpdatedResult {
		return &eth.ForkchoiceUpdatedResult{
			PayloadStatus: eth.PayloadStatusV1{Status: eth.ExecutionValid, LatestValidHash: &state.HeadBlockHash},
			PayloadID:     id,
		}
	}
	if ea.backend.GetCanonicalHash(block.NumberU64()) != state.HeadBlockHash {
		// Block is not canonical, set head.
		if latestValid, err := ea.backend.SetCanonical(block); err != nil {
			return &eth.ForkchoiceUpdatedResult{PayloadStatus: eth.PayloadStatusV1{Status: eth.ExecutionInvalid, LatestValidHash: &latestValid}}, err
		}
	} else if ea.backend.CurrentHeader().Hash() == state.HeadBlockHash {
		// If the specified head matches with our local head, do nothing and keep
		// generating the payload. It's a special corner case that a few slots are
		// missing and we are requested to generate the payload in slot.
	} else if ea.backend.Config().Optimism == nil { // minor L2Engine API divergence: allow proposers to reorg their own chain
		panic("engine not configured as optimism engine")
	}

	// If the beacon client also advertised a finalized block, mark the local
	// chain final and completely in PoS mode.
	if state.FinalizedBlockHash != (common.Hash{}) {
		// If the finalized block is not in our canonical tree, somethings wrong
		finalHeader := ea.backend.GetHeaderByHash(state.FinalizedBlockHash)
		if finalHeader == nil {
			ea.log.Warn("Final block not available in database", "hash", state.FinalizedBlockHash)
			return STATUS_INVALID, engine.InvalidForkChoiceState.With(errors.New("final block not available in database"))
		} else if ea.backend.GetCanonicalHash(finalHeader.Number.Uint64()) != state.FinalizedBlockHash {
			ea.log.Warn("Final block not in canonical chain", "number", block.NumberU64(), "hash", state.HeadBlockHash)
			return STATUS_INVALID, engine.InvalidForkChoiceState.With(errors.New("final block not in canonical chain"))
		}
		// Set the finalized block
		ea.backend.SetFinalized(finalHeader)
	}
	// Check if the safe block hash is in our canonical tree, if not somethings wrong
	if state.SafeBlockHash != (common.Hash{}) {
		safeHeader := ea.backend.GetHeaderByHash(state.SafeBlockHash)
		if safeHeader == nil {
			ea.log.Warn("Safe block not available in database")
			return STATUS_INVALID, engine.InvalidForkChoiceState.With(errors.New("safe block not available in database"))
		}
		if ea.backend.GetCanonicalHash(safeHeader.Number.Uint64()) != state.SafeBlockHash {
			ea.log.Warn("Safe block not in canonical chain")
			return STATUS_INVALID, engine.InvalidForkChoiceState.With(errors.New("safe block not in canonical chain"))
		}
		// Set the safe block
		ea.backend.SetSafe(safeHeader)
	}
	// If payload generation was requested, create a new block to be potentially
	// sealed by the beacon client. The payload will be requested later, and we
	// might replace it arbitrarily many times in between.
	if attr != nil {
		err := ea.startBlock(state.HeadBlockHash, attr)
		if err != nil {
			ea.log.Error("Failed to start block building", "err", err, "noTxPool", attr.NoTxPool, "txs", len(attr.Transactions), "timestamp", attr.Timestamp)
			return STATUS_INVALID, engine.InvalidPayloadAttributes.With(err)
		}

		return valid(&ea.payloadID), nil
	}
	return valid(nil), nil
}

func toGethWithdrawals(payload *eth.ExecutionPayload) []*types.Withdrawal {
	if payload.Withdrawals == nil {
		return nil
	}

	result := make([]*types.Withdrawal, 0, len(*payload.Withdrawals))

	for _, w := range *payload.Withdrawals {
		result = append(result, &types.Withdrawal{
			Index:     w.Index,
			Validator: w.Validator,
			Address:   w.Address,
			Amount:    w.Amount,
		})
	}

	return result
}

func (ea *L2EngineAPI) newPayload(_ context.Context, payload *eth.ExecutionPayload, hashes []common.Hash, root *common.Hash) (*eth.PayloadStatusV1, error) {
	ea.log.Trace("L2Engine API request received", "method", "ExecutePayload", "number", payload.BlockNumber, "hash", payload.BlockHash)
	txs := make([][]byte, len(payload.Transactions))
	for i, tx := range payload.Transactions {
		txs[i] = tx
	}
	block, err := engine.ExecutableDataToBlock(engine.ExecutableData{
		ParentHash:    payload.ParentHash,
		FeeRecipient:  payload.FeeRecipient,
		StateRoot:     common.Hash(payload.StateRoot),
		ReceiptsRoot:  common.Hash(payload.ReceiptsRoot),
		LogsBloom:     payload.LogsBloom[:],
		Random:        common.Hash(payload.PrevRandao),
		Number:        uint64(payload.BlockNumber),
		GasLimit:      uint64(payload.GasLimit),
		GasUsed:       uint64(payload.GasUsed),
		Timestamp:     uint64(payload.Timestamp),
		ExtraData:     payload.ExtraData,
		BaseFeePerGas: (*uint256.Int)(&payload.BaseFeePerGas).ToBig(),
		BlockHash:     payload.BlockHash,
		Transactions:  txs,
		Withdrawals:   toGethWithdrawals(payload),
		ExcessBlobGas: (*uint64)(payload.ExcessBlobGas),
		BlobGasUsed:   (*uint64)(payload.BlobGasUsed),
	}, hashes, root)
	if err != nil {
		log.Debug("Invalid NewPayload params", "params", payload, "error", err)
		return &eth.PayloadStatusV1{Status: eth.ExecutionInvalidBlockHash}, nil
	}
	// If we already have the block locally, ignore the entire execution and just
	// return a fake success.
	if block := ea.backend.GetBlock(payload.BlockHash, uint64(payload.BlockNumber)); block != nil {
		ea.log.Info("Using existing beacon payload", "number", payload.BlockNumber, "hash", payload.BlockHash, "age", common.PrettyAge(time.Unix(int64(block.Time()), 0)))
		hash := block.Hash()
		return &eth.PayloadStatusV1{Status: eth.ExecutionValid, LatestValidHash: &hash}, nil
	}

	// Skip invalid ancestor check (i.e. not remembering previously failed blocks)

	parent := ea.backend.GetBlock(block.ParentHash(), block.NumberU64()-1)
	if parent == nil {
		ea.remotes[block.Hash()] = block
		// Return accepted if we don't know the parent block. Note that there's no actual sync to activate.
		return &eth.PayloadStatusV1{Status: eth.ExecutionAccepted, LatestValidHash: nil}, nil
	}

	if block.Time() <= parent.Time() {
		log.Warn("Invalid timestamp", "parent", block.Time(), "block", block.Time())
		return ea.invalid(errors.New("invalid timestamp"), parent.Header()), nil
	}

	if !ea.backend.HasBlockAndState(block.ParentHash(), block.NumberU64()-1) {
		ea.log.Warn("State not available, ignoring new payload")
		return &eth.PayloadStatusV1{Status: eth.ExecutionAccepted}, nil
	}
	log.Trace("Inserting block without sethead", "hash", block.Hash(), "number", block.Number)
	if _, err := ea.backend.InsertBlockWithoutSetHead(block, false); err != nil {
		ea.log.Warn("NewPayloadV1: inserting block failed", "error", err)
		// Skip remembering the block was invalid, but do return the invalid response.
		return ea.invalid(err, parent.Header()), nil
	}
	hash := block.Hash()
	return &eth.PayloadStatusV1{Status: eth.ExecutionValid, LatestValidHash: &hash}, nil
}

func (ea *L2EngineAPI) invalid(err error, latestValid *types.Header) *eth.PayloadStatusV1 {
	currentHash := ea.backend.CurrentHeader().Hash()
	if latestValid != nil {
		// Set latest valid hash to 0x0 if parent is PoW block
		currentHash = common.Hash{}
		if latestValid.Difficulty.BitLen() == 0 {
			// Otherwise set latest valid hash to parent hash
			currentHash = latestValid.Hash()
		}
	}
	errorMsg := err.Error()
	return &eth.PayloadStatusV1{Status: eth.ExecutionInvalid, LatestValidHash: &currentHash, ValidationError: &errorMsg}
}
