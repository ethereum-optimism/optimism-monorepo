package derive

import (
	"context"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/log"
	"golang.org/x/exp/slices"
)

// BatchMux multiplexes between different batch stages.
// Stages are swapped on demand during Reset calls, or explicitly with Transform.
// It currently chooses the BatchQueue pre-Holocene and the BatchStage post-Holocene.
type BatchMux struct {
	log  log.Logger
	cfg  *rollup.Config
	prev NextBatchProvider
	l2   SafeBlockFetcher

	// embedded active stage
	SingularBatchProvider
}

var _ SingularBatchProvider = (*BatchMux)(nil)

// NewBatchMux returns an uninitialized BatchMux. Reset has to be called before
// calling other methods, to activate the right stage for a given L1 origin.
func NewBatchMux(lgr log.Logger, cfg *rollup.Config, prev NextBatchProvider, l2 SafeBlockFetcher) *BatchMux {
	return &BatchMux{log: lgr, cfg: cfg, prev: prev, l2: l2}
}

func (b *BatchMux) Reset(ctx context.Context, base eth.L1BlockRef, sysCfg eth.SystemConfig) error {
	// TODO(12490): change to a switch over b.cfg.ActiveFork(base.Time)
	switch {
	default:
		if _, ok := b.SingularBatchProvider.(*BatchQueue); !ok {
			b.log.Info("BatchMux: activating pre-Holocene stage during reset", "origin", base)
			b.SingularBatchProvider = NewBatchQueue(b.log, b.cfg, b.prev, b.l2)
		}
	case b.cfg.IsHolocene(base.Time):
		if _, ok := b.SingularBatchProvider.(*BatchStage); !ok {
			b.log.Info("BatchMux: activating Holocene stage during reset", "origin", base)
			b.SingularBatchProvider = NewBatchStage(b.log, b.cfg, b.prev, b.l2)
		}
	}
	return b.SingularBatchProvider.Reset(ctx, base, sysCfg)
}

func (b *BatchMux) Transform(f rollup.ForkName) {
	switch f {
	case rollup.Holocene:
		b.TransformHolocene()
	}
}

func (b *BatchMux) TransformHolocene() {
	switch bp := b.SingularBatchProvider.(type) {
	case *BatchQueue:
		b.log.Info("BatchMux: transforming to Holocene stage")
		bs := NewBatchStage(b.log, b.cfg, b.prev, b.l2)
		// Even though any ongoing span batch or queued batches are dropped at Holocene activation, the
		// post-Holocene batch stage still needs access to the collected l1Blocks pre-Holocene because
		// the first Holocene channel will contain pre-Holocene batches.
		bs.l1Blocks = slices.Clone(bp.l1Blocks)
		bs.origin = bp.origin
		b.SingularBatchProvider = bs
	case *BatchStage:
		// Even if the pipeline is Reset to the activation block, the previous origin will be the
		// same, so transfromStages isn't called.
		panic(fmt.Sprintf("Holocene BatchStage already active, old origin: %v", bp.Origin()))
	default:
		panic(fmt.Sprintf("unknown batch stage type: %T", bp))
	}
}
