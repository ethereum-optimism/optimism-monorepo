package derive

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

// The attributes queue sits after the batch queue.
// It transforms batches into payload attributes. The outputted payload
// attributes cannot be buffered because each batch->attributes transformation
// pulls in data about the current L2 safe head.
//
// It also buffers batches that have been output because multiple batches can
// be created at once.
//
// This stage can be reset by clearing its batch buffer.
// This stage does not need to retain any references to L1 blocks.

type AttributesBuilder interface {
	PreparePayloadAttributes(ctx context.Context, l2Parent eth.L2BlockRef, epoch eth.BlockID) (attrs *eth.PayloadAttributes, err error)
}

type AttributesWithParent struct {
	Attributes   *eth.PayloadAttributes
	Parent       eth.L2BlockRef
	IsLastInSpan bool

	DerivedFrom eth.L1BlockRef
}

type AttributesQueue struct {
	log          log.Logger
	config       *rollup.Config
	builder      AttributesBuilder
	prev         SingularBatchProvider
	batch        *SingularBatch
	isLastInSpan bool
}

type SingularBatchProvider interface {
	ResettableStage
	Origin() eth.L1BlockRef
	NextBatch(context.Context, eth.L2BlockRef) (*SingularBatch, bool, error)
}

func NewAttributesQueue(log log.Logger, cfg *rollup.Config, builder AttributesBuilder, prev SingularBatchProvider) *AttributesQueue {
	return &AttributesQueue{
		log:     log,
		config:  cfg,
		builder: builder,
		prev:    prev,
	}
}

func (aq *AttributesQueue) Origin() eth.L1BlockRef {
	return aq.prev.Origin()
}

func (aq *AttributesQueue) NextAttributes(ctx context.Context, parent eth.L2BlockRef) (*AttributesWithParent, error) {
	// Get a batch if we need it
	if aq.batch == nil {
		batch, isLastInSpan, err := aq.prev.NextBatch(ctx, parent)
		if err != nil {
			return nil, err
		}
		aq.batch = batch
		aq.isLastInSpan = isLastInSpan
	}

	// Actually generate the next attributes
	if attrs, err := aq.createNextAttributes(ctx, aq.batch, parent); err != nil {
		return nil, err
	} else {
		// Clear out the local state once we will succeed
		attr := AttributesWithParent{
			Attributes:   attrs,
			Parent:       parent,
			IsLastInSpan: aq.isLastInSpan,
			DerivedFrom:  aq.Origin(),
		}
		aq.batch = nil
		aq.isLastInSpan = false
		return &attr, nil
	}

}

// createNextAttributes transforms a batch into a payload attributes. This sets `NoTxPool` and appends the batched transactions
// to the attributes transaction list
func (aq *AttributesQueue) createNextAttributes(ctx context.Context, batch *SingularBatch, l2SafeHead eth.L2BlockRef) (*eth.PayloadAttributes, error) {
	// sanity check parent hash
	if batch.ParentHash != l2SafeHead.Hash {
		return nil, NewResetError(fmt.Errorf("valid batch has bad parent hash %s, expected %s", batch.ParentHash, l2SafeHead.Hash))
	}
	// sanity check timestamp
	if expected := l2SafeHead.Time + aq.config.BlockTime; expected != batch.Timestamp {
		return nil, NewResetError(fmt.Errorf("valid batch has bad timestamp %d, expected %d", batch.Timestamp, expected))
	}
	fetchCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	attrs, err := aq.builder.PreparePayloadAttributes(fetchCtx, l2SafeHead, batch.Epoch())
	if err != nil {
		return nil, err
	}

	// we are verifying, not sequencing, we've got all transactions and do not pull from the tx-pool
	// (that would make the block derivation non-deterministic)
	attrs.NoTxPool = true
	attrs.Transactions = append(attrs.Transactions, batch.Transactions...)

	aq.log.Info("generated attributes in payload queue", "txs", len(attrs.Transactions), "timestamp", batch.Timestamp)

	return attrs, nil
}

func (aq *AttributesQueue) Reset(ctx context.Context, _ eth.L1BlockRef, _ eth.SystemConfig) error {
	aq.batch = nil
	aq.isLastInSpan = false // overwritten later, but set for consistency
	return io.EOF
}
