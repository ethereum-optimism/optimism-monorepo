package invalidator

import (
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/superevents"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type db interface {
	InvalidateLocalUnsafe(chainID eth.ChainID, candidate eth.L2BlockRef) error
	InvalidateCrossUnsafe(chainID eth.ChainID, candidate eth.L2BlockRef) error
	InvalidateCrossSafe(chainID eth.ChainID, candidate types.DerivedBlockRefPair) error
}

// Invalidator is responsible for handling invalidation events by coordinating
// the rewind of databases and resetting of chain processors.
type Invalidator struct {
	log     log.Logger
	emitter event.Emitter
	db      db
}

func New(log log.Logger, db db) *Invalidator {
	return &Invalidator{
		log: log.New("component", "invalidator"),
		db:  db,
	}
}

func (i *Invalidator) AttachEmitter(em event.Emitter) {
	i.emitter = em
}

func (i *Invalidator) OnEvent(ev event.Event) bool {
	switch x := ev.(type) {
	case superevents.InvalidateLocalUnsafeEvent:
		i.log.Info("Processing local-unsafe invalidation", "chain", x.ChainID, "block", x.Candidate)
		i.db.InvalidateLocalUnsafe(x.ChainID, x.Candidate)
		return true
	case superevents.InvalidateCrossUnsafeEvent:
		i.log.Info("Processing cross-unsafe invalidation", "chain", x.ChainID, "block", x.Candidate)
		i.db.InvalidateCrossUnsafe(x.ChainID, x.Candidate)
		return true
	case superevents.InvalidateCrossSafeEvent:
		i.log.Info("Processing cross-safe invalidation", "chain", x.ChainID, "block", x.Candidate)
		i.db.InvalidateCrossSafe(x.ChainID, x.Candidate)
		return true
	default:
		return false
	}
}
