package rewinder

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/locks"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/superevents"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type rewinderDB interface {
	LocalUnsafe(eth.ChainID) (types.BlockSeal, error)
	CrossUnsafe(eth.ChainID) (types.BlockSeal, error)
	LocalSafe(eth.ChainID) (types.DerivedBlockSealPair, error)
	CrossSafe(eth.ChainID) (types.DerivedBlockSealPair, error)

	RewindLocalUnsafe(eth.ChainID, types.BlockSeal) error
	RewindCrossUnsafe(eth.ChainID, types.BlockSeal) error
	RewindLocalSafe(eth.ChainID, types.BlockSeal) error
	RewindCrossSafe(eth.ChainID, types.BlockSeal) error

	FindSealedBlock(eth.ChainID, uint64) (types.BlockSeal, error)
	Finalized(eth.ChainID) (types.BlockSeal, error)
}

type syncNode interface {
	BlockRefByNumber(ctx context.Context, number uint64) (eth.BlockRef, error)
}

// rewindController holds the methods for a rewind operation
type rewindController struct {
	isSafe      bool
	getLocal    func(eth.ChainID) (types.BlockSeal, error)
	getCross    func(eth.ChainID) (types.BlockSeal, error)
	rewindLocal func(eth.ChainID, types.BlockSeal) error
	rewindCross func(eth.ChainID, types.BlockSeal) error
}

func (rc rewindController) safetyStr() string {
	if rc.isSafe {
		return "safe"
	}
	return "unsafe"
}

// Rewinder is responsible for handling the rewinding of databases to the latest common ancestor between
// the local databases and L2 node.
type Rewinder struct {
	log       log.Logger
	emitter   event.Emitter
	db        rewinderDB
	syncNodes locks.RWMap[eth.ChainID, syncNode]
}

func New(log log.Logger, db rewinderDB) *Rewinder {
	return &Rewinder{
		log: log.New("component", "invalidator"),
		db:  db,
	}
}

func (r *Rewinder) AttachEmitter(em event.Emitter) {
	r.emitter = em
}

func (r *Rewinder) OnEvent(ev event.Event) bool {
	switch x := ev.(type) {
	case superevents.RewindChainEvent:
		r.handleEventRewindChain(x)
		return true
	case superevents.RewindAllChainsEvent:
		r.syncNodes.Range(func(chainID eth.ChainID, source syncNode) bool {
			r.emitter.Emit(superevents.RewindChainEvent{
				ChainID:  chainID,
				BadBlock: x.BadBlock,
			})
			return true
		})
		return true
	default:
		return false
	}
}

func (r *Rewinder) AttachSyncNode(chainID eth.ChainID, source syncNode) {
	r.syncNodes.Set(chainID, source)
}

// handleEventRewindChain handles the rewind chain event by attempting to rewind the local and cross databases
// for the given chain for both safety level.
func (r *Rewinder) handleEventRewindChain(ev superevents.RewindChainEvent) error {
	if err := r.attemptRewindSafe(ev.ChainID, ev.BadBlock); err != nil {
		return fmt.Errorf("failed to rewind safe chain %s: %w", ev.ChainID, err)
	}
	if err := r.attemptRewindUnsafe(ev.ChainID, ev.BadBlock); err != nil {
		return fmt.Errorf("failed to rewind unsafe chain %s: %w", ev.ChainID, err)
	}
	return nil
}

// attemptRewind attempts to rewind the local and cross databases for the given chain and controller
func (r *Rewinder) attemptRewind(chainID eth.ChainID, badBlock eth.BlockID, ctrl rewindController) error {
	// First get the finalized head
	finalizedHead, err := r.db.Finalized(chainID)
	if err != nil {
		// TODO: handle this
		finalizedHead = types.BlockSeal{}
	}

	// If the bad block's parent is before the finalized head then stop
	if badBlock.Number-1 < finalizedHead.Number {
		r.log.Warn("requested head is not ahead of finalized head", "chain", chainID, "requested", badBlock.Number-1, "finalized", finalizedHead.Number)
		return nil
	}

	// Find the latest common newHead between the parent and the finalized head
	newHead, err := r.findLatestCommonAncestor(chainID, badBlock.Number-1, finalizedHead)
	if err != nil {
		return fmt.Errorf("failed to find common ancestor for chain %s: %w", chainID, err)
	}

	// Reset the cross db if it's newer than the newHead
	crossHead, err := ctrl.getCross(chainID)
	if err != nil {
		return fmt.Errorf("failed to get cross for chain %s: %w", chainID, err)
	}
	if crossHead.Number >= newHead.Number {
		r.log.Info("rewinding cross", "safety", ctrl.safetyStr(), "chain", chainID, "newHead", newHead.Number)
		if err := ctrl.rewindCross(chainID, newHead); err != nil {
			return fmt.Errorf("failed to rewind cross for chain %s: %w", chainID, err)
		}
	}

	// Rewind the local db if it's newer than the newHead
	localHead, err := ctrl.getLocal(chainID)
	if err != nil {
		return err
	}
	if localHead.Number >= newHead.Number {
		r.log.Info("rewinding local", "safety", ctrl.safetyStr(), "chain", chainID, "newHead", newHead.Number)
		if err := ctrl.rewindLocal(chainID, newHead); err != nil {
			return fmt.Errorf("failed to rewind local for chain %s: %w", chainID, err)
		}
	}

	return nil
}

// attemptRewindUnsafe attempts to rewind the local and cross databases for unsafe blocks.
func (r *Rewinder) attemptRewindUnsafe(chainID eth.ChainID, badBlock eth.BlockID) error {
	return r.attemptRewind(chainID, badBlock, rewindController{
		isSafe:      false,
		getLocal:    r.db.LocalUnsafe,
		getCross:    r.db.CrossUnsafe,
		rewindLocal: r.db.RewindLocalUnsafe,
		rewindCross: r.db.RewindCrossUnsafe,
	})
}

// attemptRewindSafe attempts to rewind the local and cross databases for safe blocks.
func (r *Rewinder) attemptRewindSafe(chainID eth.ChainID, badBlock eth.BlockID) error {
	return r.attemptRewind(chainID, badBlock, rewindController{
		isSafe:      true,
		getLocal:    derivedFromPairGetter(r.db.LocalSafe),
		getCross:    derivedFromPairGetter(r.db.CrossSafe),
		rewindLocal: r.db.RewindLocalSafe,
		rewindCross: r.db.RewindCrossSafe,
	})
}

// findLatestCommonAncestor finds the latest common ancestor between the startBlock and the finalizedBlock
// by searching for the last block that exists in both the local db and the L2 node.
// If no common ancestor is found then the finalized head is returned.
func (r *Rewinder) findLatestCommonAncestor(chainID eth.ChainID, startBlock uint64, finalizedBlock types.BlockSeal) (types.BlockSeal, error) {
	syncNode, ok := r.syncNodes.Get(chainID)
	if !ok {
		return types.BlockSeal{}, fmt.Errorf("sync node not found for chain %s", chainID)
	}

	// Linear search from local height down to finalized height
	r.log.Info("searching for common ancestor", "chain", chainID, "local", startBlock, "finalized", finalizedBlock.Number)
	finalizedHeight := int64(finalizedBlock.Number)
	for height := int64(startBlock); height >= finalizedHeight; height-- {
		// Load the block at this height from the node and the local db
		remoteRef, err := syncNode.BlockRefByNumber(context.Background(), uint64(height))
		if err != nil {
			return types.BlockSeal{}, err
		}
		localRef, err := r.db.FindSealedBlock(chainID, uint64(height))
		if err != nil {
			return types.BlockSeal{}, err
		}

		// If the block is the same then we've found the common ancestor
		if localRef.Hash == remoteRef.Hash {
			return localRef, nil
		}
	}

	// If we didn't find a common ancestor then return the finalized head
	return finalizedBlock, nil
}

// derivedFromPairGetter wraps a (chainID) -> (types.DerivedBlockSealPair) function
// and returns the derived block seal from the pair instead of the pair itself.
func derivedFromPairGetter(fn func(eth.ChainID) (types.DerivedBlockSealPair, error)) func(eth.ChainID) (types.BlockSeal, error) {
	return func(chainID eth.ChainID) (types.BlockSeal, error) {
		pair, err := fn(chainID)
		if err != nil {
			return types.BlockSeal{}, err
		}
		return pair.Derived, nil
	}
}
