package derive

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

// The batch queue is responsible for ordering unordered batches & generating empty batches
// when the sequence window has passed. This is a very stateful stage.
//
// It receives batches that are tagged with the L1 Inclusion block of the batch. It only considers
// batches that are inside the sequencing window of a specific L1 Origin.
// It tries to eagerly pull batches based on the current L2 safe head.
// Otherwise it filters/creates an entire epoch's worth of batches at once.
//
// This stage tracks a range of L1 blocks with the assumption that all batches with an L1 inclusion
// block inside that range have been added to the stage by the time that it attempts to advance a
// full epoch.
//
// It is internally responsible for making sure that batches with L1 inclusions block outside it's
// working range are not considered or pruned.

type NextBatchProvider interface {
	ChannelFlusher
	Origin() eth.L1BlockRef
	NextBatch(ctx context.Context) (Batch, error)
}

type SafeBlockFetcher interface {
	L2BlockRefByNumber(context.Context, uint64) (eth.L2BlockRef, error)
	PayloadByNumber(context.Context, uint64) (*eth.ExecutionPayloadEnvelope, error)
}

// The baseBatchStage is a shared implementation of basic channel stage functionality. It is
// currently shared between the legacy BatchQueue, which buffers future batches, and the
// post-Holocene BatchStage, which requires strictly ordered batches.
type baseBatchStage struct {
	log    log.Logger
	config *rollup.Config
	prev   NextBatchProvider
	l2     SafeBlockFetcher

	origin eth.L1BlockRef

	// l1Blocks contains consecutive eth.L1BlockRef sorted by time.
	// Every L1 origin of unsafe L2 blocks must be eventually included in l1Blocks.
	// Batch queue's job is to ensure below two rules:
	//  If every L2 block corresponding to single L1 block becomes safe, it will be popped from l1Blocks.
	//  If new L2 block's L1 origin is not included in l1Blocks, fetch and push to l1Blocks.
	// length of l1Blocks never exceeds SequencerWindowSize
	l1Blocks []eth.L1BlockRef

	// nextSpan is cached SingularBatches derived from SpanBatch
	nextSpan []*SingularBatch
}

func newBaseBatchStage(log log.Logger, cfg *rollup.Config, prev NextBatchProvider, l2 SafeBlockFetcher) baseBatchStage {
	return baseBatchStage{
		log:    log,
		config: cfg,
		prev:   prev,
		l2:     l2,
	}
}

func (bs *baseBatchStage) base() *baseBatchStage {
	return bs
}

func (bs *baseBatchStage) Log() log.Logger {
	if len(bs.l1Blocks) == 0 {
		return bs.log.New("origin", bs.origin.ID())
	} else {
		return bs.log.New("origin", bs.origin.ID(), "epoch", bs.l1Blocks[0])
	}
}

// BatchQueue contains a set of batches for every L1 block.
// L1 blocks are contiguous and this does not support reorgs.
type BatchQueue struct {
	baseBatchStage

	// batches in order of when we've first seen them
	batches []*BatchWithL1InclusionBlock
}

var _ SingularBatchProvider = (*BatchQueue)(nil)

// NewBatchQueue creates a BatchQueue, which should be Reset(origin) before use.
func NewBatchQueue(log log.Logger, cfg *rollup.Config, prev NextBatchProvider, l2 SafeBlockFetcher) *BatchQueue {
	return &BatchQueue{baseBatchStage: newBaseBatchStage(log, cfg, prev, l2)}
}

func (bs *baseBatchStage) Origin() eth.L1BlockRef {
	return bs.prev.Origin()
}

// popNextBatch pops the next batch from the current queued up span-batch nextSpan.
// The queue must be non-empty, or the function will panic.
func (bs *baseBatchStage) popNextBatch(parent eth.L2BlockRef) *SingularBatch {
	if len(bs.nextSpan) == 0 {
		panic("popping non-existent span-batch, invalid state")
	}
	nextBatch := bs.nextSpan[0]
	bs.nextSpan = bs.nextSpan[1:]
	// Must set ParentHash before return. we can use parent because the parentCheck is verified in CheckBatch().
	nextBatch.ParentHash = parent.Hash
	bs.log.Debug("pop next batch from the cached span batch")
	return nextBatch
}

// NextBatch return next valid batch upon the given safe head.
// It also returns the boolean that indicates if the batch is the last block in the batch.
func (bs *baseBatchStage) nextFromSpanBatch(parent eth.L2BlockRef) (*SingularBatch, bool) {
	if len(bs.nextSpan) > 0 {
		// There are cached singular batches derived from the span batch.
		// Check if the next cached batch matches the given parent block.
		if bs.nextSpan[0].Timestamp == parent.Time+bs.config.BlockTime {
			// Pop first one and return.
			nextBatch := bs.popNextBatch(parent)
			// len(bq.nextSpan) == 0 means it's the last batch of the span.
			return nextBatch, len(bs.nextSpan) == 0
		} else {
			// Given parent block does not match the next batch. It means the previously returned batch is invalid.
			// Drop cached batches and find another batch.
			bs.log.Warn("parent block does not match the next batch. dropped cached batches", "parent", parent.ID(), "nextBatchTime", bs.nextSpan[0].GetTimestamp())
			bs.nextSpan = bs.nextSpan[:0]
		}
	}
	return nil, false
}

func (bs *baseBatchStage) updateOrigins(parent eth.L2BlockRef) {
	// Note: We use the origin that we will have to determine if it's behind. This is important
	// because it's the future origin that gets saved into the l1Blocks array.
	// We always update the origin of this stage if it is not the same so after the update code
	// runs, this is consistent.
	originBehind := bs.originBehind(parent)

	// Advance origin if needed
	// Note: The entire pipeline has the same origin
	// We just don't accept batches prior to the L1 origin of the L2 safe head
	if bs.origin != bs.prev.Origin() {
		bs.origin = bs.prev.Origin()
		if !originBehind {
			bs.l1Blocks = append(bs.l1Blocks, bs.origin)
		} else {
			// This is to handle the special case of startup. At startup we call Reset & include
			// the L1 origin. That is the only time where immediately after `Reset` is called
			// originBehind is false.
			bs.l1Blocks = bs.l1Blocks[:0]
		}
		bs.log.Info("Advancing bq origin", "origin", bs.origin, "originBehind", originBehind)
	}

	// If the epoch is advanced, update bq.l1Blocks
	// Before Holocene, advancing the epoch must be done after the pipeline successfully applied the entire span batch to the chain.
	// This is because the entire span batch can be reverted after finding an invalid batch.
	// So we must preserve the existing l1Blocks to verify the epochs of the next candidate batch.
	if len(bs.l1Blocks) > 0 && parent.L1Origin.Number > bs.l1Blocks[0].Number {
		for i, l1Block := range bs.l1Blocks {
			if parent.L1Origin.Number == l1Block.Number {
				bs.l1Blocks = bs.l1Blocks[i:]
				bs.log.Debug("Advancing internal L1 blocks", "next_epoch", bs.l1Blocks[0].ID(), "next_epoch_time", bs.l1Blocks[0].Time)
				break
			}
		}
		// If we can't find the origin of parent block, we have to advance bq.origin.
	}
}

func (bs *baseBatchStage) originBehind(parent eth.L2BlockRef) bool {
	return bs.prev.Origin().Number < parent.L1Origin.Number
}

func (bq *BatchQueue) NextBatch(ctx context.Context, parent eth.L2BlockRef) (*SingularBatch, bool, error) {
	// Early return if there are singular batches from a span batch queued up
	if batch, last := bq.nextFromSpanBatch(parent); batch != nil {
		return batch, last, nil
	}

	bq.updateOrigins(parent)

	originBehind := bq.originBehind(parent)
	// Load more data into the batch queue
	outOfData := false
	if batch, err := bq.prev.NextBatch(ctx); err == io.EOF {
		outOfData = true
	} else if err != nil {
		return nil, false, err
	} else if !originBehind {
		bq.AddBatch(ctx, batch, parent)
	}

	// Skip adding data unless we are up to date with the origin, but do fully
	// empty the previous stages
	if originBehind {
		if outOfData {
			return nil, false, io.EOF
		} else {
			return nil, false, NotEnoughData
		}
	}

	// Finally attempt to derive more batches
	batch, err := bq.deriveNextBatch(ctx, outOfData, parent)
	if err == io.EOF && outOfData {
		return nil, false, io.EOF
	} else if err == io.EOF {
		return nil, false, NotEnoughData
	} else if err != nil {
		return nil, false, err
	}

	var nextBatch *SingularBatch
	switch typ := batch.GetBatchType(); typ {
	case SingularBatchType:
		singularBatch, ok := batch.AsSingularBatch()
		if !ok {
			return nil, false, NewCriticalError(errors.New("failed type assertion to SingularBatch"))
		}
		nextBatch = singularBatch
	case SpanBatchType:
		spanBatch, ok := batch.AsSpanBatch()
		if !ok {
			return nil, false, NewCriticalError(errors.New("failed type assertion to SpanBatch"))
		}
		// If next batch is SpanBatch, convert it to SingularBatches.
		singularBatches, err := spanBatch.GetSingularBatches(bq.l1Blocks, parent)
		if err != nil {
			return nil, false, NewCriticalError(err)
		}
		bq.nextSpan = singularBatches
		// span-batches are non-empty, so the below pop is safe.
		nextBatch = bq.popNextBatch(parent)
	default:
		return nil, false, NewCriticalError(fmt.Errorf("unrecognized batch type: %d", typ))
	}

	// If the nextBatch is derived from the span batch, len(bq.nextSpan) == 0 means it's the last batch of the span.
	// For singular batches, len(bq.nextSpan) == 0 is always true.
	return nextBatch, len(bq.nextSpan) == 0, nil
}

func (bs *baseBatchStage) reset(base eth.L1BlockRef) {
	// Copy over the Origin from the next stage
	// It is set in the engine queue (two stages away) such that the L2 Safe Head origin is the progress
	bs.origin = base
	bs.l1Blocks = bs.l1Blocks[:0]
	// Include the new origin as an origin to build on
	// Note: This is only for the initialization case. During normal resets we will later
	// throw out this block.
	bs.l1Blocks = append(bs.l1Blocks, base)
	bs.nextSpan = bs.nextSpan[:0]
}

func (bq *BatchQueue) Reset(_ context.Context, base eth.L1BlockRef, _ eth.SystemConfig) error {
	bq.baseBatchStage.reset(base)
	bq.batches = bq.batches[:0]
	return io.EOF
}

func (bq *BatchQueue) FlushChannel() {
	// We need to implement the ChannelFlusher interface with the BatchQueue but it's never called
	// of which the BatchMux takes care.
	panic("BatchQueue: invalid FlushChannel call")
}

func (bq *BatchQueue) AddBatch(ctx context.Context, batch Batch, parent eth.L2BlockRef) {
	if len(bq.l1Blocks) == 0 {
		panic(fmt.Errorf("cannot add batch with timestamp %d, no origin was prepared", batch.GetTimestamp()))
	}
	data := BatchWithL1InclusionBlock{
		L1InclusionBlock: bq.origin,
		Batch:            batch,
	}
	validity := CheckBatch(ctx, bq.config, bq.log, bq.l1Blocks, parent, &data, bq.l2)
	if validity == BatchDrop {
		return // if we do drop the batch, CheckBatch will log the drop reason with WARN level.
	}
	batch.LogContext(bq.log).Debug("Adding batch")
	bq.batches = append(bq.batches, &data)
}

// deriveNextBatch derives the next batch to apply on top of the current L2 safe head,
// following the validity rules imposed on consecutive batches,
// based on currently available buffered batch and L1 origin information.
// If no batch can be derived yet, then (nil, io.EOF) is returned.
func (bq *BatchQueue) deriveNextBatch(ctx context.Context, outOfData bool, parent eth.L2BlockRef) (Batch, error) {
	if len(bq.l1Blocks) == 0 {
		return nil, NewCriticalError(errors.New("cannot derive next batch, no origin was prepared"))
	}
	epoch := bq.l1Blocks[0]
	bq.log.Trace("Deriving the next batch", "epoch", epoch, "parent", parent, "outOfData", outOfData)

	// Note: epoch origin can now be one block ahead of the L2 Safe Head
	// This is in the case where we auto generate all batches in an epoch & advance the epoch
	// but don't advance the L2 Safe Head's epoch
	if parent.L1Origin != epoch.ID() && parent.L1Origin.Number != epoch.Number-1 {
		return nil, NewResetError(fmt.Errorf("buffered L1 chain epoch %s in batch queue does not match safe head origin %s", epoch, parent.L1Origin))
	}

	// Find the first-seen batch that matches all validity conditions.
	// We may not have sufficient information to proceed filtering, and then we stop.
	// There may be none: in that case we force-create an empty batch
	var nextBatch *BatchWithL1InclusionBlock

	// Go over all batches, in order of inclusion, and find the first batch we can accept.
	// We filter in-place by only remembering the batches that may be processed in the future, or those we are undecided on.
	var remaining []*BatchWithL1InclusionBlock
batchLoop:
	for i, batch := range bq.batches {
		validity := CheckBatch(ctx, bq.config, bq.log.New("batch_index", i), bq.l1Blocks, parent, batch, bq.l2)
		switch validity {
		case BatchFuture:
			remaining = append(remaining, batch)
			continue
		case BatchDrop:
			batch.Batch.LogContext(bq.log).Warn("Dropping batch",
				"parent", parent.ID(),
				"parent_time", parent.Time,
			)
			continue
		case BatchAccept:
			nextBatch = batch
			// don't keep the current batch in the remaining items since we are processing it now,
			// but retain every batch we didn't get to yet.
			remaining = append(remaining, bq.batches[i+1:]...)
			break batchLoop
		case BatchUndecided:
			remaining = append(remaining, bq.batches[i:]...)
			bq.batches = remaining
			return nil, io.EOF
		default:
			return nil, NewCriticalError(fmt.Errorf("unknown batch validity type: %d", validity))
		}
	}
	bq.batches = remaining

	if nextBatch != nil {
		nextBatch.Batch.LogContext(bq.log).Info("Found next batch")
		return nextBatch.Batch, nil
	}
	return bq.deriveNextEmptyBatch(ctx, outOfData, parent)
}

// deriveNextEmptyBatch may derive an empty batch if the sequencing window is expired
func (bs *baseBatchStage) deriveNextEmptyBatch(ctx context.Context, outOfData bool, parent eth.L2BlockRef) (*SingularBatch, error) {
	epoch := bs.l1Blocks[0]
	// If the current epoch is too old compared to the L1 block we are at,
	// i.e. if the sequence window expired, we create empty batches for the current epoch
	expiryEpoch := epoch.Number + bs.config.SeqWindowSize
	forceEmptyBatches := (expiryEpoch == bs.origin.Number && outOfData) || expiryEpoch < bs.origin.Number
	firstOfEpoch := epoch.Number == parent.L1Origin.Number+1
	nextTimestamp := parent.Time + bs.config.BlockTime

	bs.log.Trace("Potentially generating an empty batch",
		"expiryEpoch", expiryEpoch, "forceEmptyBatches", forceEmptyBatches, "nextTimestamp", nextTimestamp,
		"epoch_time", epoch.Time, "len_l1_blocks", len(bs.l1Blocks), "firstOfEpoch", firstOfEpoch)

	if !forceEmptyBatches {
		// sequence window did not expire yet, still room to receive batches for the current epoch,
		// no need to force-create empty batch(es) towards the next epoch yet.
		return nil, io.EOF
	}
	if len(bs.l1Blocks) < 2 {
		// need next L1 block to proceed towards
		return nil, io.EOF
	}

	nextEpoch := bs.l1Blocks[1]
	// Fill with empty L2 blocks of the same epoch until we meet the time of the next L1 origin,
	// to preserve that L2 time >= L1 time. If this is the first block of the epoch, always generate a
	// batch to ensure that we at least have one batch per epoch.
	if nextTimestamp < nextEpoch.Time || firstOfEpoch {
		bs.log.Info("Generating next batch", "epoch", epoch, "timestamp", nextTimestamp)
		return &SingularBatch{
			ParentHash:   parent.Hash,
			EpochNum:     rollup.Epoch(epoch.Number),
			EpochHash:    epoch.Hash,
			Timestamp:    nextTimestamp,
			Transactions: nil,
		}, nil
	}

	// At this point we have auto generated every batch for the current epoch
	// that we can, so we can advance to the next epoch.
	// TODO(12444): Instead of manually advancing the epoch here, it may be better to generate a
	// batch for the next epoch, so that updateOrigins then properly advances the origin.
	bs.log.Trace("Advancing internal L1 blocks", "next_timestamp", nextTimestamp, "next_epoch_time", nextEpoch.Time)
	bs.l1Blocks = bs.l1Blocks[1:]
	return nil, io.EOF
}
