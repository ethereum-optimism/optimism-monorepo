package derive

import (
	"context"
	"io"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

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
