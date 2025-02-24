package preimages

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-challenger/game/keccak/matrix"
	keccakTypes "github.com/ethereum-optimism/optimism/op-challenger/game/keccak/types"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching/rpcblock"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
)

var _ PreimageUploader = (*LargePreimageUploader)(nil)

// ErrChallengePeriodNotOver is returned when the challenge period is not over.
var ErrChallengePeriodNotOver = errors.New("challenge period not over")

// MaxBlocksPerChunk is the maximum number of keccak blocks per chunk.
const MaxBlocksPerChunk = 500

// MaxChunkSize is the maximum size of a preimage chunk in bytes.
// Notice, the max chunk size must be a multiple of the leaf size.
// The max chunk size is roughly 0.04MB to avoid memory expansion.
const MaxChunkSize = MaxBlocksPerChunk * keccakTypes.BlockSize

// LargePreimageUploader handles uploading large preimages by
// streaming the merkleized preimage to the PreimageOracle contract,
// tightly packed across multiple transactions.
type LargePreimageUploader struct {
	log log.Logger

	clock    types.ClockReader
	txSender TxSender
	contract PreimageOracleContract
}

func NewLargePreimageUploader(logger log.Logger, cl types.ClockReader, txSender TxSender, contract PreimageOracleContract) *LargePreimageUploader {
	return &LargePreimageUploader{logger, cl, txSender, contract}
}

func (p *LargePreimageUploader) UploadPreimage(ctx context.Context, parent uint64, data *types.PreimageOracleData) error {
	p.log.Debug("Upload large preimage", "key", hexutil.Bytes(data.OracleKey))
	stateMatrix, calls, err := p.splitCalls(data)
	if err != nil {
		return fmt.Errorf("failed to split preimage into chunks for data with oracle offset %d: %w", data.OracleOffset, err)
	}

	uuid := NewUUID(p.txSender.From(), data)

	// Fetch the current metadata for this preimage data, if it exists.
	ident := keccakTypes.LargePreimageIdent{Claimant: p.txSender.From(), UUID: uuid}
	metadata, err := p.contract.GetProposalMetadata(ctx, rpcblock.Latest, ident)
	if err != nil {
		return fmt.Errorf("failed to get pre-image oracle metadata: %w", err)
	}

	// The proposal is not initialized if the queried metadata has a claimed size of 0.
	if len(metadata) == 1 && metadata[0].ClaimedSize == 0 {
		err = p.initLargePreimage(uuid, data.OracleOffset, uint32(len(data.GetPreimageWithoutSize())))
		if err != nil {
			return fmt.Errorf("failed to initialize large preimage with uuid: %s: %w", uuid, err)
		}
	}

	// Filter out any chunks that have already been uploaded to the Preimage Oracle.
	if len(metadata) > 0 {
		numSkip := metadata[0].BytesProcessed / MaxChunkSize
		calls = calls[numSkip:]
		// If the timestamp is non-zero, the preimage has been finalized.
		if metadata[0].Timestamp != 0 {
			calls = calls[len(calls):]
		}
	}

	err = p.addLargePreimageData(uuid, calls)
	if err != nil {
		return fmt.Errorf("failed to add leaves to large preimage with uuid: %s: %w", uuid, err)
	}

	return p.Squeeze(ctx, uuid, stateMatrix)
}

// NewUUID generates a new unique identifier for the preimage by hashing the
// concatenated preimage data, preimage offset, and sender address.
func NewUUID(sender common.Address, data *types.PreimageOracleData) *big.Int {
	offset := make([]byte, 4)
	binary.LittleEndian.PutUint32(offset, data.OracleOffset)
	concatenated := append(data.GetPreimageWithoutSize(), offset...)
	concatenated = append(concatenated, sender.Bytes()...)
	hash := crypto.Keccak256Hash(concatenated)
	return hash.Big()
}

// splitChunks splits the preimage data into chunks of size [MaxChunkSize] (except the last chunk).
// It also returns the state matrix and the data for the squeeze call if possible.
func (p *LargePreimageUploader) splitCalls(data *types.PreimageOracleData) (*matrix.StateMatrix, []keccakTypes.InputData, error) {
	// Split the preimage data into chunks of size [MaxChunkSize] (except the last chunk).
	stateMatrix := matrix.NewStateMatrix()
	var calls []keccakTypes.InputData
	in := bytes.NewReader(data.GetPreimageWithoutSize())
	for {
		call, err := stateMatrix.AbsorbUpTo(in, MaxChunkSize)
		if errors.Is(err, io.EOF) {
			calls = append(calls, call)
			break
		} else if err != nil {
			return nil, nil, fmt.Errorf("failed to absorb data: %w", err)
		}
		calls = append(calls, call)
	}
	return stateMatrix, calls, nil
}

func (p *LargePreimageUploader) Squeeze(ctx context.Context, uuid *big.Int, stateMatrix *matrix.StateMatrix) error {
	prestateMatrix := stateMatrix.PrestateMatrix()
	prestate, prestateProof := stateMatrix.PrestateWithProof()
	poststate, poststateProof := stateMatrix.PoststateWithProof()
	challengePeriod, err := p.contract.ChallengePeriod(ctx)
	if err != nil {
		return fmt.Errorf("failed to get challenge period: %w", err)
	}
	currentTimestamp := p.clock.Now().Unix()
	ident := keccakTypes.LargePreimageIdent{Claimant: p.txSender.From(), UUID: uuid}
	metadata, err := p.contract.GetProposalMetadata(ctx, rpcblock.Latest, ident)
	if err != nil {
		return fmt.Errorf("failed to get pre-image oracle metadata: %w", err)
	}
	if len(metadata) == 0 || metadata[0].ClaimedSize == 0 {
		return fmt.Errorf("no metadata found for pre-image oracle with uuid: %s", uuid)
	}
	if uint64(currentTimestamp) < metadata[0].Timestamp+challengePeriod {
		return ErrChallengePeriodNotOver
	}
	if err := p.contract.CallSqueeze(ctx, p.txSender.From(), uuid, prestateMatrix, prestate, prestateProof, poststate, poststateProof); err != nil {
		p.log.Warn("Expected a successful squeeze call", "metadataTimestamp", metadata[0].Timestamp, "currentTimestamp", currentTimestamp, "err", err)
		return fmt.Errorf("failed to call squeeze: %w", err)
	}
	p.log.Info("Squeezing large preimage", "uuid", uuid)
	tx, err := p.contract.Squeeze(p.txSender.From(), uuid, prestateMatrix, prestate, prestateProof, poststate, poststateProof)
	if err != nil {
		return fmt.Errorf("failed to create pre-image oracle tx: %w", err)
	}
	if err := p.txSender.SendAndWaitSimple("squeeze large preimage", tx); err != nil {
		return fmt.Errorf("failed to populate pre-image oracle: %w", err)
	}
	return nil
}

// initLargePreimage initializes the large preimage proposal.
// This method *must* be called before adding any leaves.
func (p *LargePreimageUploader) initLargePreimage(uuid *big.Int, partOffset uint32, claimedSize uint32) error {
	p.log.Info("Init large preimage upload", "uuid", uuid, "partOffset", partOffset, "size", claimedSize)
	candidate, err := p.contract.InitLargePreimage(uuid, partOffset, claimedSize)
	if err != nil {
		return fmt.Errorf("failed to create pre-image oracle tx: %w", err)
	}
	if err := p.txSender.SendAndWaitSimple("init large preimage", candidate); err != nil {
		return fmt.Errorf("failed to populate pre-image oracle: %w", err)
	}
	return nil
}

// addLargePreimageData adds data to the large preimage proposal.
// This method **must** be called after calling [initLargePreimage].
// SAFETY: submits transactions in a [Queue] for latency while preserving submission order.
func (p *LargePreimageUploader) addLargePreimageData(uuid *big.Int, chunks []keccakTypes.InputData) error {
	txs := make([]txmgr.TxCandidate, len(chunks))
	blocksProcessed := int64(0)
	for i, chunk := range chunks {
		tx, err := p.contract.AddLeaves(uuid, big.NewInt(blocksProcessed), chunk.Input, chunk.Commitments, chunk.Finalize)
		if err != nil {
			return fmt.Errorf("failed to create pre-image oracle tx: %w", err)
		}
		blocksProcessed += int64(len(chunk.Input) / keccakTypes.BlockSize)
		txs[i] = tx
	}
	p.log.Info("Adding large preimage leaves", "uuid", uuid, "blocksProcessed", blocksProcessed, "txs", len(txs))
	return p.txSender.SendAndWaitSimple("add leaf to large preimage", txs...)
}
