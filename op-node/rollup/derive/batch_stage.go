package derive

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/log"
)

type BatchStage struct {
	baseBatchStage
}

var _ SingularBatchProvider = (*BatchStage)(nil)

func NewBatchStage(log log.Logger, cfg *rollup.Config, prev NextBatchProvider, l2 SafeBlockFetcher) *BatchStage {
	return &BatchStage{baseBatchStage: newBaseBatchStage(log, cfg, prev, l2)}
}

func (bs *BatchStage) Reset(_ context.Context, base eth.L1BlockRef, _ eth.SystemConfig) error {
	bs.reset(base)
	return io.EOF
}

func (bs *BatchStage) FlushChannel() {
	bs.nextSpan = bs.nextSpan[:0]
	bs.prev.FlushChannel()
}

func (bs *BatchStage) NextBatch(ctx context.Context, parent eth.L2BlockRef) (*SingularBatch, bool, error) {
	// with Holocene, we can always update (and prune) the origins because we don't backwards-invalidate.
	bs.updateOrigins(parent)

	// If origin behind (or at parent), we drain previous stage(s), and then return.
	// Note that a channel from the parent's L1 origin block can only contain past batches, so we
	// can just skip them.
	// TODO(12444): we may be able to change the definition of originBehind to include equality,
	// also for the pre-Holocene BatchQueue. This may also allow us to remove the edge case in
	// updateOrigins.
	if bs.originBehind(parent) || parent.L1Origin.Number == bs.origin.Number {
		if _, err := bs.prev.NextBatch(ctx); err != nil {
			// includes io.EOF and NotEnoughData
			return nil, false, err
		}
		// continue draining
		return nil, false, NotEnoughData
	}

	if len(bs.l1Blocks) < 2 {
		// This can only happen if derivation erroneously doesn't start at a safe head.
		// By now, the L1 origin of the first safe head and the following L1 block must be in the
		// l1Blocks.
		return nil, false, NewCriticalError(fmt.Errorf(
			"unexpected low buffered origin count, origin: %v, parent: %v", bs.origin, parent))
	}

	// Note: epoch origin can now be one block ahead of the L2 Safe Head
	// This is in the case where we auto generate all batches in an epoch & advance the epoch in
	// deriveNextEmptyBatch but don't advance the L2 Safe Head's epoch
	if epoch := bs.l1Blocks[0]; parent.L1Origin != epoch.ID() && parent.L1Origin.Number != epoch.Number-1 {
		return nil, false, NewResetError(fmt.Errorf("buffered L1 chain epoch %s in batch queue does not match safe head origin %s", epoch, parent.L1Origin))
	}

	batch, err := bs.nextSingularBatchCandidate(ctx, parent)
	if err == io.EOF {
		// We only consider empty batch generation after we've drained all batches from the local
		// span batch queue and the previous stage.
		empty, err := bs.deriveNextEmptyBatch(ctx, true, parent)
		// An empty batch always advances the (local) safe head.
		return empty, true, err
	} else if err != nil {
		return nil, false, err
	}

	// check candidate validity
	validity := checkSingularBatch(bs.config, bs.Log(), bs.l1Blocks, parent, batch, bs.origin)
	switch validity {
	case BatchAccept: // continue
		batch.LogContext(bs.Log()).Debug("Found next singular batch")
		// BatchStage is only used with Holocene, where blocks immediately become (local) safe
		return batch, true, nil
	case BatchPast:
		batch.LogContext(bs.Log()).Warn("Dropping past singular batch")
		// NotEnoughData to read in next batch until we're through all past batches
		return nil, false, NotEnoughData
	case BatchDrop: // drop, flush, move onto next channel
		batch.LogContext(bs.Log()).Warn("Dropping invalid singular batch, flushing channel")
		bs.FlushChannel()
		// NotEnoughData will cause derivation from previous stages until they're empty, at which
		// point empty batch derivation will happen.
		return nil, false, NotEnoughData
	case BatchUndecided: // l2 fetcher error, try again
		batch.LogContext(bs.Log()).Warn("Undecided span batch")
		return nil, false, NotEnoughData
	case BatchFuture: // panic, can't happen
		return nil, false, NewCriticalError(fmt.Errorf("impossible batch validity: %v", validity))
	default:
		return nil, false, NewCriticalError(fmt.Errorf("unknown batch validity type: %d", validity))
	}
}

func (bs *BatchStage) nextSingularBatchCandidate(ctx context.Context, parent eth.L2BlockRef) (*SingularBatch, error) {
	// First check for next span-derived batch
	nextBatch, _ := bs.nextFromSpanBatch(parent)

	if nextBatch != nil {
		return nextBatch, nil
	}

	// If the next batch is a singular batch, we forward it as the candidate.
	// If it is a span batch, we check its validity and then forward its first singular batch.
	batch, err := bs.prev.NextBatch(ctx)
	if err != nil { // includes io.EOF
		return nil, err
	}
	switch typ := batch.GetBatchType(); typ {
	case SingularBatchType:
		singularBatch, ok := batch.AsSingularBatch()
		if !ok {
			return nil, NewCriticalError(errors.New("failed type assertion to SingularBatch"))
		}
		return singularBatch, nil
	case SpanBatchType:
		spanBatch, ok := batch.AsSpanBatch()
		if !ok {
			return nil, NewCriticalError(errors.New("failed type assertion to SpanBatch"))
		}

		validity, _ := checkSpanBatchPrefix(ctx, bs.config, bs.Log(), bs.l1Blocks, parent, spanBatch, bs.origin, bs.l2)
		switch validity {
		case BatchAccept: // continue
			spanBatch.LogContext(bs.Log()).Info("Found next valid span batch")
		case BatchPast:
			spanBatch.LogContext(bs.Log()).Warn("Dropping past span batch")
			// NotEnoughData to read in next batch until we're through all past batches
			return nil, NotEnoughData
		case BatchDrop: // drop, try next
			spanBatch.LogContext(bs.Log()).Warn("Dropping invalid span batch, flushing channel")
			bs.FlushChannel()
			return nil, NotEnoughData
		case BatchUndecided: // l2 fetcher error, try again
			spanBatch.LogContext(bs.Log()).Warn("Undecided span batch")
			return nil, NotEnoughData
		case BatchFuture: // can't happen with Holocene
			return nil, NewCriticalError(errors.New("impossible future batch validity"))
		}

		// If next batch is SpanBatch, convert it to SingularBatches.
		// TODO(12444): maybe create iterator here instead, save to nextSpan
		//   Need to make sure this doesn't error where the iterator wouldn't,
		//   otherwise this wouldn't be correctly implementing partial span batch invalidation.
		//   From what I can tell, it is fine because the only error case is if the l1Blocks are
		//   missing a block, which would be a logic error. Although, if the node restarts mid-way
		//   through a span batch and the sync start only goes back one channel timeout from the
		//   mid-way safe block, it may actually miss l1 blocks! Need to check.
		//   We could fix this by fast-dropping past batches from the span batch.
		singularBatches, err := spanBatch.GetSingularBatches(bs.l1Blocks, parent)
		if err != nil {
			return nil, NewCriticalError(err)
		}
		bs.nextSpan = singularBatches
		// span-batches are non-empty, so the below pop is safe.
		return bs.popNextBatch(parent), nil
	default:
		return nil, NewCriticalError(fmt.Errorf("unrecognized batch type: %d", typ))
	}
}
