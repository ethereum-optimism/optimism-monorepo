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

// The BatchQueue is responsible for ordering unordered batches & generating empty batches
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
//
// Holocene replaces the BatchQueue with the [BatchStage].
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
