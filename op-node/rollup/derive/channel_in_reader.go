package derive

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

// ChannelInReader reads a batch from the channel
// This does decompression and limits the max RLP size
// This is a pure function from the channel, but each channel (or channel fragment)
// must be tagged with an L1 inclusion block to be passed to the batch queue.
type ChannelInReader struct {
	log         log.Logger
	spec        *rollup.ChainSpec
	cfg         *rollup.Config
	nextBatchFn func() (*BatchData, error)
	prev        RawChannelProvider
	metrics     Metrics
}

var (
	_ ResettableStage = (*ChannelInReader)(nil)
	_ ChannelFlusher  = (*ChannelInReader)(nil)
)

type RawChannelProvider interface {
	ResettableStage
	ChannelFlusher
	Origin() eth.L1BlockRef
	NextRawChannel(ctx context.Context) ([]byte, error)
}

// NewChannelInReader creates a ChannelInReader, which should be Reset(origin) before use.
func NewChannelInReader(cfg *rollup.Config, log log.Logger, prev RawChannelProvider, metrics Metrics) *ChannelInReader {
	return &ChannelInReader{
		spec:    rollup.NewChainSpec(cfg),
		cfg:     cfg,
		log:     log,
		prev:    prev,
		metrics: metrics,
	}
}

func (cr *ChannelInReader) Origin() eth.L1BlockRef {
	return cr.prev.Origin()
}

// TODO: Take full channel for better logging
func (cr *ChannelInReader) WriteChannel(data []byte) error {
	if f, err := BatchReader(bytes.NewBuffer(data), cr.spec.MaxRLPBytesPerChannel(cr.prev.Origin().Time), cr.cfg.IsFjord(cr.prev.Origin().Time)); err == nil {
		cr.nextBatchFn = f
		cr.metrics.RecordChannelInputBytes(len(data))
		return nil
	} else {
		cr.log.Error("Error creating batch reader from channel data", "err", err)
		return err
	}
}

// NextChannel forces the next read to continue with the next channel,
// resetting any decoding/decompression state to a fresh start.
func (cr *ChannelInReader) NextChannel() {
	cr.nextBatchFn = nil
}

// NextBatch pulls out the next batch from the channel if it has it.
// It returns io.EOF when it cannot make any more progress.
// It will return a temporary error if it needs to be called again to advance some internal state.
func (cr *ChannelInReader) NextBatch(ctx context.Context) (Batch, error) {
	if cr.nextBatchFn == nil {
		if data, err := cr.prev.NextRawChannel(ctx); err == io.EOF {
			return nil, io.EOF
		} else if err != nil {
			return nil, err
		} else {
			if err := cr.WriteChannel(data); err != nil {
				return nil, NewTemporaryError(err)
			}
		}
	}

	// TODO: can batch be non nil while err == io.EOF
	// This depends on the behavior of rlp.Stream
	batchData, err := cr.nextBatchFn()
	if err == io.EOF {
		cr.NextChannel()
		return nil, NotEnoughData
	} else if err != nil {
		cr.log.Warn("failed to read batch from channel reader, skipping to next channel now", "err", err)
		cr.NextChannel()
		return nil, NotEnoughData
	}

	batch := batchWithMetadata{comprAlgo: batchData.ComprAlgo}
	switch batchData.GetBatchType() {
	case SingularBatchType:
		batch.Batch, err = GetSingularBatch(batchData)
		if err != nil {
			return nil, err
		}
		batch.LogContext(cr.log).Debug("decoded singular batch from channel", "stage_origin", cr.Origin())
		cr.metrics.RecordDerivedBatches("singular")
		return batch, nil
	case SpanBatchType:
		if origin := cr.Origin(); !cr.cfg.IsDelta(origin.Time) {
			// Check hard fork activation with the L1 inclusion block time instead of the L1 origin block time.
			// Therefore, even if the batch passed this rule, it can be dropped in the batch queue.
			// This is just for early dropping invalid batches as soon as possible.
			return nil, NewTemporaryError(fmt.Errorf("cannot accept span batch in L1 block %s at time %d", origin, origin.Time))
		}
		batch.Batch, err = DeriveSpanBatch(batchData, cr.cfg.BlockTime, cr.cfg.Genesis.L2Time, cr.cfg.L2ChainID)
		if err != nil {
			return nil, err
		}
		batch.LogContext(cr.log).Debug("decoded span batch from channel", "stage_origin", cr.Origin())
		cr.metrics.RecordDerivedBatches("span")
		return batch, nil
	default:
		// error is bubbled up to user, but pipeline can skip the batch and continue after.
		return nil, NewTemporaryError(fmt.Errorf("unrecognized batch type: %d", batchData.GetBatchType()))
	}
}

func (cr *ChannelInReader) Reset(ctx context.Context, _ eth.L1BlockRef, _ eth.SystemConfig) error {
	cr.nextBatchFn = nil
	return io.EOF
}

func (cr *ChannelInReader) FlushChannel() {
	cr.nextBatchFn = nil
	cr.prev.FlushChannel()
}
