package altda

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-alt-da/bindings"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

// ErrPendingChallenge is returned when data is not available but can still be challenged/resolved
// so derivation should halt temporarily.
var ErrPendingChallenge = errors.New("not found, pending challenge")

// ErrExpiredChallenge is returned when a challenge was not resolved and derivation should skip this input.
var ErrExpiredChallenge = errors.New("challenge expired")

// ErrMissingPastWindow is returned when the input data is MIA and cannot be challenged.
// This is a protocol fatal error.
var ErrMissingPastWindow = errors.New("data missing past window")

// ErrInvalidChallenge is returned when a challenge event does is decoded but does not
// relate to the actual chain commitments.
var ErrInvalidChallenge = errors.New("invalid challenge")

// L1Fetcher is the required interface for syncing the DA challenge contract state.
type L1Fetcher interface {
	InfoAndTxsByHash(ctx context.Context, hash common.Hash) (eth.BlockInfo, types.Transactions, error)
	FetchReceipts(ctx context.Context, blockHash common.Hash) (eth.BlockInfo, types.Receipts, error)
	L1BlockRefByNumber(context.Context, uint64) (eth.L1BlockRef, error)
}

// DAStorage interface for calling the DA storage server.
type DAStorage interface {
	// GetInput returns the input data for the given commitment bytes.
	// It passes the l1BlockNum at which the commitment was read (part of a batcher tx),
	// in case the validity of the commitment depends on some time component, such as the commitment having
	// landed on L1 not too long after having landed on an altda network.
	GetInput(ctx context.Context, key CommitmentData, l1BlockNum uint64) ([]byte, error)
	SetInput(ctx context.Context, img []byte) (CommitmentData, error)
}

// HeadSignalFn is the callback function to accept head-signals without a context.
type HeadSignalFn func(eth.L1BlockRef)

// Config is the relevant subset of rollup config for AltDA.
type Config struct {
	// Required for filtering contract events
	DAChallengeContractAddress common.Address
	// Allowed CommitmentType
	CommitmentType CommitmentType
	// The number of l1 blocks after the input is committed during which one can challenge.
	ChallengeWindow uint64
	// The number of l1 blocks after a commitment is challenged during which one can resolve.
	ResolveWindow uint64
}

type DA struct {
	log     log.Logger
	cfg     Config
	metrics Metricer
	storage DAStorage
	state   *State // the DA state keeps track of all the commitments and their challenge status.

	challengeOrigin  eth.BlockID    // the highest l1 block we synced challenge contract events from
	commitmentOrigin eth.BlockID    // the highest l1 block we read commitments from
	finalizedHead    eth.L1BlockRef // the latest recorded finalized head as per the challenge contract
	l1FinalizedHead  eth.L1BlockRef // the latest recorded finalized head as per the l1 finalization signal

	// flag the reset function we are resetting because of an expired challenge
	resetting bool

	finalizedHeadSignalHandler HeadSignalFn
}

// NewAltDA creates a new AltDA instance with the given log and CLIConfig.
func NewAltDA(log log.Logger, cli CLIConfig, cfg Config, metrics Metricer) *DA {
	return NewAltDAWithStorage(log, cfg, cli.NewDAClient(), metrics)
}

// NewAltDAWithStorage creates a new AltDA instance with the given log and DAStorage interface.
func NewAltDAWithStorage(log log.Logger, cfg Config, storage DAStorage, metrics Metricer) *DA {
	return &DA{
		log:     log,
		cfg:     cfg,
		storage: storage,
		metrics: metrics,
		state:   NewState(log, metrics, cfg),
	}
}

// NewAltDAWithState creates an AltDA storage from initial state used for testing in isolation.
// We pass the L1Fetcher to each method so it is kept in sync with the conf depth of the pipeline.
func NewAltDAWithState(log log.Logger, cfg Config, storage DAStorage, metrics Metricer, state *State) *DA {
	return &DA{
		log:     log,
		cfg:     cfg,
		storage: storage,
		metrics: metrics,
		state:   state,
	}
}

// OnFinalizedHeadSignal sets the callback function to be called when the finalized head is updated.
// This will signal to the engine queue that will set the proper L2 block as finalized.
func (d *DA) OnFinalizedHeadSignal(f HeadSignalFn) {
	d.finalizedHeadSignalHandler = f
}

// updateFinalizedHead sets the finalized head and prunes the state to the L1 Finalized head.
// the finalized head is set to the latest reference pruned in this way.
// It is called by the Finalize function, as it has an L1 finalized head to use.
func (d *DA) updateFinalizedHead(l1Finalized eth.L1BlockRef) {
	d.l1FinalizedHead = l1Finalized
	// Prune the state to the finalized head
	d.state.Prune(l1Finalized.ID())
	d.finalizedHead = d.state.lastPrunedCommitment
}

// updateFinalizedFromL1 updates the finalized head based on the challenge window.
// it uses the L1 fetcher to get the block reference at the finalized head - challenge window.
// It is called in AdvanceL1Origin if there are no commitments to finalize, as it has an L1 fetcher to use.
func (d *DA) updateFinalizedFromL1(ctx context.Context, l1 L1Fetcher) error {
	// don't update if the finalized head is smaller than the challenge window
	if d.l1FinalizedHead.Number < d.cfg.ChallengeWindow {
		return nil
	}
	ref, err := l1.L1BlockRefByNumber(ctx, d.l1FinalizedHead.Number-d.cfg.ChallengeWindow)
	if err != nil {
		return err
	}
	d.finalizedHead = ref
	return nil
}

// Finalize sets the L1 finalized head signal and calls the handler function if set.
func (d *DA) Finalize(l1Finalized eth.L1BlockRef) {
	d.updateFinalizedHead(l1Finalized)
	d.metrics.RecordChallengesHead("finalized", d.finalizedHead.Number)

	// Record and Log the latest L1 finalized head
	d.log.Info("received l1 finalized signal, forwarding altDA finalization to finalizedHeadSignalHandler",
		"l1", l1Finalized,
		"altDA", d.finalizedHead)

	// execute the handler function if set
	// the handler function is called with the altDA finalized head
	if d.finalizedHeadSignalHandler == nil {
		d.log.Warn("finalized head signal handler not set")
		return
	}
	d.finalizedHeadSignalHandler(d.finalizedHead)
}

// LookAhead increments the challenges origin and process the new block if it exists.
// It is used when the derivation pipeline stalls due to missing data and we need to continue
// syncing challenge events until the challenge is resolved or expires.
func (d *DA) LookAhead(ctx context.Context, l1 L1Fetcher) error {
	blkRef, err := l1.L1BlockRefByNumber(ctx, d.challengeOrigin.Number+1)
	// temporary error, will do a backoff
	if err != nil {
		return err
	}
	return d.AdvanceChallengeOrigin(ctx, l1, blkRef.ID())
}

// Reset the challenge event derivation origin in case of L1 reorg
func (d *DA) Reset(ctx context.Context, base eth.L1BlockRef, baseCfg eth.SystemConfig) error {
	// resetting due to expired challenge, do not clear state.
	// If the DA source returns ErrReset, the pipeline is forced to reset by the rollup driver.
	// In that case the Reset function will be called immediately, BEFORE the pipeline can
	// call any further stage to step. Thus the state will NOT be cleared if the reset originates
	// from this stage of the pipeline.
	if d.resetting {
		d.resetting = false
		d.commitmentOrigin = base.ID()
		d.state.ClearCommitments()
	} else {
		// resetting due to L1 reorg, clear state
		d.challengeOrigin = base.ID()
		d.commitmentOrigin = base.ID()
		d.state.Reset()
	}
	return io.EOF
}

// GetInput returns the input data for the given commitment bytes. blockNumber is required to lookup
// the challenge status in the DataAvailabilityChallenge L1 contract.
func (d *DA) GetInput(ctx context.Context, l1 L1Fetcher, comm CommitmentData, blockId eth.L1BlockRef) (eth.Data, error) {
	// If it's not the right commitment type, report it as an expired commitment in order to skip it
	if d.cfg.CommitmentType != comm.CommitmentType() {
		return nil, fmt.Errorf("invalid commitment type; expected: %v, got: %v: %w", d.cfg.CommitmentType, comm.CommitmentType(), ErrExpiredChallenge)
	}
	status := d.state.GetChallengeStatus(comm, blockId.Number)
	// check if the challenge is expired
	if status == ChallengeExpired {
		// Don't track the expired commitment. If we hit this case we have seen an expired challenge, but never used the data.
		// this indicates that the data which might cause us to reorg is expired (not to be used) so we can optimize by skipping the reorg.
		// If we used the data & then expire the challenge later, we do that during the AdvanceChallengeOrigin step
		return nil, ErrExpiredChallenge
	}
	// Record the commitment for later finalization / invalidation
	d.state.TrackCommitment(comm, blockId)
	d.log.Info("getting input", "comm", comm, "status", status)

	// Fetch the input from the DA storage.
	data, err := d.storage.GetInput(ctx, comm, blockId.Number)
	notFound := errors.Is(ErrNotFound, err)
	if err != nil && !notFound {
		d.log.Error("failed to get preimage", "err", err)
		// the storage client request failed for some other reason
		// in which case derivation pipeline should be retried
		return nil, err
	}

	// If the data is not found, things are handled differently based on the challenge status.
	if notFound {
		log.Warn("data not found for the given commitment", "comm", comm, "status", status, "block", blockId.Number)
		switch status {
		case ChallengeUninitialized:
			// If this commitment was never challenged & we can't find the data, treat it as unrecoverable.
			if d.challengeOrigin.Number > blockId.Number+d.cfg.ChallengeWindow {
				return nil, ErrMissingPastWindow
			}
			// Otherwise continue syncing challenges hoping it eventually is challenged and resolved
			if err := d.LookAhead(ctx, l1); err != nil {
				return nil, err
			}
			return nil, ErrPendingChallenge
		case ChallengeActive:
			// If the commitment is active, we must wait for the challenge to resolve
			// hence we continue syncing new origins to sync the new challenge events.
			// Active challenges are expired by the AdvanceChallengeOrigin function which calls state.ExpireChallenges
			if err := d.LookAhead(ctx, l1); err != nil {
				return nil, err
			}
			return nil, ErrPendingChallenge
		case ChallengeResolved:
			// Generic Commitments don't resolve from L1 so if we still can't find the data we're out of luck
			if comm.CommitmentType() == GenericCommitmentType {
				return nil, ErrMissingPastWindow
			}
			// Keccak commitments resolve from L1, so we should have the data in the challenge resolved input
			if comm.CommitmentType() == Keccak256CommitmentType {
				ch, _ := d.state.GetChallenge(comm, blockId.Number)
				return ch.input, nil
			}
		}
	}
	// regardless of the potential notFound error, if this challenge status is not handled, return an error
	if status != ChallengeUninitialized && status != ChallengeActive && status != ChallengeResolved {
		return nil, fmt.Errorf("unknown challenge status: %v", status)
	}

	return data, nil
}

// AdvanceChallengeOrigin reads & stores challenge events for the given L1 block
func (d *DA) AdvanceChallengeOrigin(ctx context.Context, l1 L1Fetcher, block eth.BlockID) error {
	// do not repeat for the same or old origin
	if block.Number <= d.challengeOrigin.Number {
		return nil
	}

	// load challenge events from the l1 block
	if err := d.loadChallengeEvents(ctx, l1, block); err != nil {
		return err
	}

	// Expire challenges
	d.state.ExpireChallenges(block)

	// set and record the new challenge origin
	d.challengeOrigin = block
	d.metrics.RecordChallengesHead("latest", d.challengeOrigin.Number)
	d.log.Info("processed altDA challenge origin", "origin", block)
	return nil
}

// AdvanceCommitmentOrigin updates the commitment origin and the finalized head.
func (d *DA) AdvanceCommitmentOrigin(ctx context.Context, l1 L1Fetcher, block eth.BlockID) error {
	// do not repeat for the same origin
	if block.Number <= d.commitmentOrigin.Number {
		return nil
	}

	// Expire commitments
	err := d.state.ExpireCommitments(block)
	if err != nil {
		// warn the reset function not to clear the state
		d.resetting = true
		return err
	}

	// set and record the new commitment origin
	d.commitmentOrigin = block
	d.metrics.RecordChallengesHead("latest", d.challengeOrigin.Number)
	d.log.Info("processed altDA l1 origin", "origin", block, "finalized", d.finalizedHead.ID(), "l1-finalize", d.l1FinalizedHead.ID())

	return nil
}

// AdvanceL1Origin syncs any challenge events included in the l1 block, expires any active challenges
// after the new resolveWindow, computes and signals the new finalized head and sets the l1 block
// as the new head for tracking challenges and commitments. If forwards an error if any new challenge have expired to
// trigger a derivation reset.
func (d *DA) AdvanceL1Origin(ctx context.Context, l1 L1Fetcher, block eth.BlockID) error {
	if err := d.AdvanceChallengeOrigin(ctx, l1, block); err != nil {
		return fmt.Errorf("failed to advance challenge origin: %w", err)
	}
	if err := d.AdvanceCommitmentOrigin(ctx, l1, block); err != nil {
		return fmt.Errorf("failed to advance commitment origin: %w", err)
	}
	// if there are no commitments, we can calculate the finalized head based on the challenge window
	// otherwise, the finalization signal is used to set the finalized head
	if d.state.NoCommitments() {
		if err := d.updateFinalizedFromL1(ctx, l1); err != nil {
			return err
		}
		d.metrics.RecordChallengesHead("finalized", d.finalizedHead.Number)
	}
	return nil
}

// loadChallengeEvents fetches the l1 block receipts and updates the challenge status
func (d *DA) loadChallengeEvents(ctx context.Context, l1 L1Fetcher, block eth.BlockID) error {
	// filter any challenge event logs in the block
	logs, err := d.fetchChallengeLogs(ctx, l1, block)
	if err != nil {
		return err
	}

	for _, log := range logs {
		i := log.TxIndex
		status, comm, bn, err := d.decodeChallengeStatus(log)
		if err != nil {
			d.log.Error("failed to decode challenge event", "block", block.Number, "tx", i, "log", log.Index, "err", err)
			continue
		}
		switch status {
		case ChallengeResolved:
			// cached with input resolution call so not expensive
			_, txs, err := l1.InfoAndTxsByHash(ctx, block.Hash)
			if err != nil {
				d.log.Error("failed to fetch l1 block", "block", block.Number, "err", err)
				continue
			}
			// avoid panic in black swan case of faulty rpc
			if uint(len(txs)) <= i {
				d.log.Error("tx/receipt mismatch in InfoAndTxsByHash")
				continue
			}
			// select the transaction corresponding to the receipt
			tx := txs[i]
			// txs and receipts must be in the same order
			if tx.Hash() != log.TxHash {
				d.log.Error("tx hash mismatch", "block", block.Number, "txIdx", i, "log", log.Index, "txHash", tx.Hash(), "receiptTxHash", log.TxHash)
				continue
			}

			var input []byte
			if d.cfg.CommitmentType == Keccak256CommitmentType {
				// Decode the input from resolver tx calldata
				input, err = DecodeResolvedInput(tx.Data())
				if err != nil {
					d.log.Error("failed to decode resolved input", "block", block.Number, "txIdx", i, "err", err)
					continue
				}
				if err := comm.Verify(input); err != nil {
					d.log.Error("failed to verify commitment", "block", block.Number, "txIdx", i, "err", err)
					continue
				}
			}

			d.log.Info("challenge resolved", "block", block, "txIdx", i)
			// Resolve challenge in state
			if err := d.state.ResolveChallenge(comm, block, bn, input); err != nil {
				d.log.Error("failed to resolve challenge", "block", block.Number, "txIdx", i, "err", err)
				continue
			}
		case ChallengeActive:
			// create challenge in state
			d.log.Info("detected new active challenge", "block", block, "comm", comm)
			d.state.CreateChallenge(comm, block, bn)
		default:
			d.log.Warn("skipping unknown challenge status", "block", block.Number, "tx", i, "log", log.Index, "status", status, "comm", comm)
		}
	}
	return nil
}

// fetchChallengeLogs returns logs for challenge events if any for the given block
func (d *DA) fetchChallengeLogs(ctx context.Context, l1 L1Fetcher, block eth.BlockID) ([]*types.Log, error) {
	var logs []*types.Log
	// Don't look at the challenge contract if there is no challenge contract.
	if d.cfg.CommitmentType == GenericCommitmentType {
		return logs, nil
	}
	//cached with deposits events call so not expensive
	_, receipts, err := l1.FetchReceipts(ctx, block.Hash)
	if err != nil {
		return nil, err
	}
	d.log.Info("loading challenges", "epoch", block.Number, "numReceipts", len(receipts))
	for _, rec := range receipts {
		// skip error logs
		if rec.Status != types.ReceiptStatusSuccessful {
			continue
		}
		for _, log := range rec.Logs {
			if log.Address == d.cfg.DAChallengeContractAddress && len(log.Topics) > 0 && log.Topics[0] == ChallengeStatusEventABIHash {
				logs = append(logs, log)
			}
		}
	}

	return logs, nil
}

// decodeChallengeStatus decodes and validates a challenge event from a transaction log, returning the associated commitment bytes.
func (d *DA) decodeChallengeStatus(log *types.Log) (ChallengeStatus, CommitmentData, uint64, error) {
	event, err := DecodeChallengeStatusEvent(log)
	if err != nil {
		return 0, nil, 0, err
	}
	comm, err := DecodeCommitmentData(event.ChallengedCommitment)
	if err != nil {
		return 0, nil, 0, err
	}
	d.log.Debug("decoded challenge status event", "log", log, "event", event, "comm", fmt.Sprintf("%x", comm.Encode()))
	return ChallengeStatus(event.Status), comm, event.ChallengedBlockNumber.Uint64(), nil
}

var (
	ChallengeStatusEventName    = "ChallengeStatusChanged"
	ChallengeStatusEventABI     = "ChallengeStatusChanged(uint256,bytes,uint8)"
	ChallengeStatusEventABIHash = crypto.Keccak256Hash([]byte(ChallengeStatusEventABI))
)

// DecodeChallengeStatusEvent decodes the challenge status event from the log data and the indexed challenged
// hash and block number from the topics.
func DecodeChallengeStatusEvent(log *types.Log) (*bindings.DataAvailabilityChallengeChallengeStatusChanged, error) {
	// abi lazy loaded, cached after decoded once
	dacAbi, err := bindings.DataAvailabilityChallengeMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	var event bindings.DataAvailabilityChallengeChallengeStatusChanged
	err = dacAbi.UnpackIntoInterface(&event, ChallengeStatusEventName, log.Data)
	if err != nil {
		return nil, err
	}
	var indexed abi.Arguments
	for _, arg := range dacAbi.Events[ChallengeStatusEventName].Inputs {
		if arg.Indexed {
			indexed = append(indexed, arg)
		}
	}
	if err := abi.ParseTopics(&event, indexed, log.Topics[1:]); err != nil {
		return nil, err
	}
	return &event, nil
}

// DecodeResolvedInput decodes the preimage bytes from the tx input data.
func DecodeResolvedInput(data []byte) ([]byte, error) {
	dacAbi, err := bindings.DataAvailabilityChallengeMetaData.GetAbi()
	if err != nil {
		return nil, err
	}

	args := make(map[string]interface{})
	err = dacAbi.Methods["resolve"].Inputs.UnpackIntoMap(args, data[4:])
	if err != nil {
		return nil, err
	}
	rd, ok := args["resolveData"].([]byte)
	if !ok || len(rd) == 0 {
		return nil, fmt.Errorf("invalid resolve data")
	}
	return rd, nil
}
