package invalidator

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/locks"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/superevents"
)

type syncSource interface {
	BlockRefByNumber(ctx context.Context, num uint64) (eth.BlockRef, error)
}

type db interface {
	InvalidateLocalUnsafe(chainID eth.ChainID, candidate eth.L2BlockRef) error
	InvalidateCrossUnsafe(chainID eth.ChainID, candidate eth.L2BlockRef) error
	InvalidateCrossSafe(chainID eth.ChainID, candidate eth.L2BlockRef) error
}

// Invalidator is responsible for handling invalidation events by coordinating
// the rewind of databases and resetting of chain processors.
type Invalidator struct {
	log         log.Logger
	emitter     event.Emitter
	db          db
	syncSources *locks.RWMap[eth.ChainID, syncSource]
}

func New(log log.Logger, db db) *Invalidator {
	return &Invalidator{
		log:         log.New("component", "invalidator"),
		db:          db,
		syncSources: &locks.RWMap[eth.ChainID, syncSource]{},
	}
}

func (i *Invalidator) AttachEmitter(em event.Emitter) {
	i.emitter = em
}

func (i *Invalidator) AttachSyncSource(chainID eth.ChainID, src syncSource) error {
	_, ok := i.syncSources.Get(chainID)
	if !ok {
		return fmt.Errorf("unknown chain %s, cannot attach RPC to sync source", chainID)
	}
	i.syncSources.Set(chainID, src)
	return nil
}

func (i *Invalidator) OnEvent(ev event.Event) bool {
	switch x := ev.(type) {
	case superevents.InvalidateLocalUnsafeEvent:
		i.handleLocalUnsafeInvalidation(x)
		return true
	case superevents.InvalidateCrossUnsafeEvent:
		i.handleCrossUnsafeInvalidation(x)
		return true
	// case superevents.InvalidateLocalSafeEvent:
	// i.handleLocalSafeInvalidation(x)
	// return true
	// case superevents.InvalidateCrossSafeEvent:
	// i.handleCrossSafeInvalidation(x)
	// return true
	default:
		return false
	}
}

// handleLocalUnsafeInvalidation handles the invalidation of a local-unsafe block.
// This is simpler than safe block invalidation as it only affects one chain and
// doesn't require coordination across multiple chains.
func (i *Invalidator) handleLocalUnsafeInvalidation(ev superevents.InvalidateLocalUnsafeEvent) {
	i.log.Info("Processing local-unsafe invalidation",
		"chain", ev.ChainID,
		"block", ev.Candidate)
	// TODO: Move InvalidateCrossUnsafe, etc from ChainsDB to Invalidator (here)
	i.db.InvalidateLocalUnsafe(ev.ChainID, ev.Candidate)
}

// handleCrossUnsafeInvalidation handles the invalidation of a cross-unsafe block.
func (i *Invalidator) handleCrossUnsafeInvalidation(ev superevents.InvalidateCrossUnsafeEvent) {
	i.log.Info("Processing cross-unsafe invalidation",
		"chain", ev.ChainID,
		"block", ev.Candidate)
	i.db.InvalidateCrossUnsafe(ev.ChainID, ev.Candidate)
}

// handleCrossSafeInvalidation handles the invalidation of a cross-safe block.
func (i *Invalidator) handleCrossSafeInvalidation(ev superevents.InvalidateCrossSafeEvent) {
	i.log.Info("Processing cross-safe invalidation",
		"chain", ev.ChainID,
		"block", ev.Candidate)
	i.db.InvalidateCrossSafe(ev.ChainID, ev.Candidate)
}

// findCommonL2Ancestor starts from the given bad block and walks backwards to find the
// latest common ancestor between the local db and remote chain.
// func (i *Invalidator) findCommonL2Ancestor(chainID eth.ChainID, badRef eth.BlockRef) eth.L2BlockRef {
// 	syncSource, ok := i.syncSources.Get(chainID)
// 	if !ok {
// 		i.log.Error("No sync source found for chain", "chain", chainID)
// 		return eth.L2BlockRef{}
// 	}

// 	height := badRef.Number - 1
// 	badRefs := make([]types.BlockSeal, 0)
// 	for height > 0 {
// 		remoteCandidateRef, err := syncSource.BlockRefByNumber(context.Background(), height)
// 		if err != nil {
// 			i.log.Error("Failed to fetch block ref", "chain", chainID, "height", height, "error", err)
// 			return eth.L2BlockRef{}
// 		}

// 		localCandidateRef, err := i.chainsDB.FindSealedBlock(chainID, height)
// 		if err != nil {
// 			i.log.Error("Failed to fetch block from database", "chain", chainID, "height", height, "error", err)
// 			return eth.L2BlockRef{}
// 		}

// 		if remoteCandidateRef.ID() == localCandidateRef.ID() {
// 			// Found a common ancestor
// 			return eth.L2BlockRef{
// 				Hash:       remoteCandidateRef.Hash,
// 				Number:     height,
// 				ParentHash: remoteCandidateRef.ParentHash,
// 				Time:       remoteCandidateRef.Time,
// 			}
// 		}

// 		badRefs = append(badRefs, localCandidateRef)
// 		height--
// 	}

// 	// TODO: Implement this
// 	return eth.L2BlockRef{}
// }

// findCommonUnsafeL2Ancestor starts from the given bad unsafe block and walks backwards to find the
// latest unsafe block that is common to the local db and remote chain. If we run into the safe head
// first, we return the safe head.
// func (i *Invalidator) findCommonUnsafeL2Ancestor(chainID eth.ChainID, badRef eth.BlockRef) eth.L2BlockRef {
// 	// Get safe head height
// 	// TODO: get the cross-safe head height like in RewindIfLaterThan or w/e
// 	safeHeadHeight := uint64(10)

// 	// Walk backwards from badRef down to safeHeadHeight
// 	// Stop early if we find a common ancestor
// 	for height := badRef.Number; height > safeHeadHeight; height-- {
// 		remoteCandidateRef, err := syncSource.BlockRefByNumber(context.Background(), height)
// 		if err != nil {
// 			i.log.Error("Failed to fetch block ref", "chain", chainID, "height", height, "error", err)
// 			return eth.L2BlockRef{}
// 		}

// 		localCandidateRef, err := i.chainsDB.FindSealedBlock(chainID, height)
// 		if err != nil {
// 			i.log.Error("Failed to fetch block from database", "chain", chainID, "height", height, "error", err)
// 			return eth.L2BlockRef{}
// 		}

// 		if remoteCandidateRef.ID() == localCandidateRef.ID() {
// 			// Found a common ancestor
// 			return eth.L2BlockRef{
// 				Hash:       remoteCandidateRef.Hash,
// 				Number:     height,
// 				ParentHash: remoteCandidateRef.ParentHash,
// 				Time:       remoteCandidateRef.Time,
// 			}
// 		}

// 		badRefs = append(badRefs, localCandidateRef)
// 		height--
// 	}

// 	// Return the safe head
// 	return eth.L2BlockRef{
// 		Number: safeHeadHeight,
// 	}
// }
