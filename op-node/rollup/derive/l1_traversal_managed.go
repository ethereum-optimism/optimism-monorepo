package derive

import (
	"context"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

type ManagedL1Traversal interface {
	ProvideNextL1(ctx context.Context, nextL1 eth.L1BlockRef) error
}

type L1TraversalManagedSource interface {
	FetchReceipts(ctx context.Context, blockHash common.Hash) (eth.BlockInfo, types.Receipts, error)
}

// L1TraversalManaged is an alternative version of L1Traversal,
// that supports manually operated L1 traversal, as used in the Interop upgrade.
type L1TraversalManaged struct {
	block eth.L1BlockRef

	// true = consumed by other stages
	// false = not consumed yet
	done bool

	l1Blocks L1TraversalManagedSource
	log      log.Logger
	sysCfg   eth.SystemConfig
	cfg      *rollup.Config
}

var _ l1TraversalStage = (*L1TraversalManaged)(nil)
var _ ManagedL1Traversal = (*L1TraversalManaged)(nil)

func NewL1TraversalManaged(log log.Logger, cfg *rollup.Config, l1Blocks L1TraversalManagedSource) *L1TraversalManaged {
	return &L1TraversalManaged{
		log:      log,
		l1Blocks: l1Blocks,
		cfg:      cfg,
	}
}

func (l1t *L1TraversalManaged) Origin() eth.L1BlockRef {
	return l1t.block
}

// NextL1Block returns the next block. It does not advance, but it can only be
// called once before returning io.EOF
func (l1t *L1TraversalManaged) NextL1Block(_ context.Context) (eth.L1BlockRef, error) {
	l1t.log.Trace("NextL1Block", "done", l1t.done, "block", l1t.block)
	if !l1t.done {
		l1t.done = true
		return l1t.block, nil
	} else {
		return eth.L1BlockRef{}, io.EOF
	}
}

// AdvanceL1Block advances the internal state of L1 Traversal
func (l1t *L1TraversalManaged) AdvanceL1Block(ctx context.Context) error {
	l1t.log.Trace("AdvanceL1Block", "done", l1t.done, "block", l1t.block)
	if !l1t.done {
		l1t.log.Debug("Need to process current block first", "block", l1t.block)
		return nil
	}
	// At this point we consumed the L1 block, i.e. exhausted available data.
	// The next L1 block will not be available until a manual ProvideNextL1 call.
	return io.EOF
}

// Reset sets the internal L1 block to the supplied base.
func (l1t *L1TraversalManaged) Reset(ctx context.Context, base eth.L1BlockRef, cfg eth.SystemConfig) error {
	l1t.block = base
	l1t.done = true // Retrieval will be at this same L1 block, so technically it has been consumed already.
	l1t.sysCfg = cfg
	l1t.log.Info("completed reset of derivation pipeline", "origin", base)
	return io.EOF
}

func (l1c *L1TraversalManaged) SystemConfig() eth.SystemConfig {
	return l1c.sysCfg
}

// ProvideNextL1 is an override to traverse to the next L1 block.
func (l1t *L1TraversalManaged) ProvideNextL1(ctx context.Context, nextL1 eth.L1BlockRef) error {
	logger := l1t.log.New("current", l1t.block, "next", nextL1)
	if !l1t.done {
		logger.Debug("Not ready for next L1 block yet")
		return nil
	}
	if l1t.block.Number+1 != nextL1.Number {
		logger.Warn("Received signal for L1 block, but needed different block")
		return nil // safe to ignore; we'll signal an exhaust-L1 event, and get the correct next L1 block.
	}
	if l1t.block.Hash != nextL1.ParentHash {
		logger.Warn("Provided next L1 block does not build on last processed L1 block")
		return NewResetError(fmt.Errorf("provided next L1 block %s does not build on last processed L1 block %s", nextL1, l1t.block))
	}

	// Parse L1 receipts of the given block and update the L1 system configuration.
	// If this fails, the caller will just have to ProvideNextL1 again (triggered by revisiting the exhausted-L1 signal).
	_, receipts, err := l1t.l1Blocks.FetchReceipts(ctx, nextL1.Hash)
	if err != nil {
		return NewTemporaryError(fmt.Errorf("failed to fetch receipts of L1 block %s (parent: %s) for L1 sysCfg update: %w",
			nextL1, nextL1.ParentID(), err))
	}
	if err := UpdateSystemConfigWithL1Receipts(&l1t.sysCfg, receipts, l1t.cfg, nextL1.Time); err != nil {
		// the sysCfg changes should always be formatted correctly.
		return NewCriticalError(fmt.Errorf("failed to update L1 sysCfg with receipts from block %s: %w", nextL1, err))
	}

	logger.Info("Derivation continued with next L1 block")
	l1t.block = nextL1
	l1t.done = false
	return nil
}
