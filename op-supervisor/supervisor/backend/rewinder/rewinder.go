package rewinder

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/superevents"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type l1Node interface {
	L1BlockRefByNumber(ctx context.Context, number uint64) (eth.L1BlockRef, error)
}

type rewinderDB interface {
	DependencySet() depset.DependencySet

	CrossSourceToLastDerived(chainID eth.ChainID, source eth.BlockID) (derived types.BlockSeal, err error)
	PreviousSource(chain eth.ChainID, source eth.BlockID) (prevSource types.BlockSeal, err error)
	CrossDerivedToSourceRef(chainID eth.ChainID, derived eth.BlockID) (source eth.BlockRef, err error)

	LocalSafe(eth.ChainID) (types.DerivedBlockSealPair, error)
	CrossSafe(eth.ChainID) (types.DerivedBlockSealPair, error)

	RewindLocalSafe(eth.ChainID, eth.BlockID) error
	RewindCrossSafe(eth.ChainID, eth.BlockID) error
	RewindLogs(chainID eth.ChainID, newHead types.BlockSeal) error

	FindSealedBlock(eth.ChainID, uint64) (types.BlockSeal, error)
	Finalized(eth.ChainID) (types.BlockSeal, error)

	LocalDerivedToSource(chain eth.ChainID, derived eth.BlockID) (source types.BlockSeal, err error)
}

// Rewinder is responsible for handling the rewinding of databases to the latest common ancestor between
// the local databases and L2 node.
type Rewinder struct {
	log     log.Logger
	emitter event.Emitter
	l1Node  l1Node
	db      rewinderDB
}

func New(log log.Logger, db rewinderDB, l1Node l1Node) *Rewinder {
	return &Rewinder{
		log:    log.New("component", "rewinder"),
		db:     db,
		l1Node: l1Node,
	}
}

func (r *Rewinder) AttachEmitter(em event.Emitter) {
	r.emitter = em
}

func (r *Rewinder) OnEvent(ev event.Event) bool {
	switch x := ev.(type) {
	case superevents.RewindL1Event:
		r.handleRewindL1Event(x)
		return true
	case superevents.LocalSafeUpdateEvent:
		r.handleLocalDerivedEvent(x)
		return true
	default:
		return false
	}
}

// handleRewindL1Event iterates known chains and checks each one for a reorg
// If a reorg is detected, it will rewind the chain to the latest common ancestor
// between the local-safe head and the finalized head.
func (r *Rewinder) handleRewindL1Event(ev superevents.RewindL1Event) {
	for _, chainID := range r.db.DependencySet().Chains() {
		if err := r.rewindL1ChainIfReorged(chainID, ev.IncomingBlock); err != nil {
			r.log.Error("failed to rewind L1 data:", "chain", chainID, "err", err)
		}
	}
}

// handleLocalDerivedEvent checks if the newly derived block matches what we have in our unsafe DB
// If it doesn't match, we need to rewind the logs DB to the common ancestor between
// the LocalUnsafe head and the new LocalSafe block
func (r *Rewinder) handleLocalDerivedEvent(ev superevents.LocalSafeUpdateEvent) {
	// Get the block at the derived height from our unsafe chain
	newSafeHead := ev.NewLocalSafe.Derived
	unsafeVersion, err := r.db.FindSealedBlock(ev.ChainID, newSafeHead.Number)
	if err != nil {
		r.log.Error("failed to get unsafe block at derived height", "chain", ev.ChainID, "height", newSafeHead.Number, "err", err)
		return
	}

	// If the block hashes match, our unsafe chain is still valid
	if unsafeVersion.Hash == newSafeHead.Hash {
		return
	}

	// Try rewinding the logs DB to the parent of the new safe head
	// If it fails with a data conflict walk back through the chain
	// until we find a common ancestor or reach the finalized block
	finalized, err := r.db.Finalized(ev.ChainID)
	if err != nil {
		if errors.Is(err, types.ErrFuture) {
			finalized = types.BlockSeal{Number: 0}
		} else {
			r.log.Error("failed to get finalized block", "chain", ev.ChainID, "err", err)
			return
		}
	}
	var target types.BlockSeal
	for height := int64(newSafeHead.Number - 1); height >= int64(finalized.Number); height-- {
		// Get the block at this height
		target, err = r.db.FindSealedBlock(ev.ChainID, uint64(height))
		if err != nil {
			r.log.Error("failed to get sealed block", "chain", ev.ChainID, "height", height, "err", err)
			return
		}

		_, err := r.db.LocalDerivedToSource(ev.ChainID, target.ID())
		if err != nil {
			if errors.Is(err, types.ErrConflict) || errors.Is(err, types.ErrFuture) {
				continue
			}

			r.log.Error("failed to get derived from block", "chain", ev.ChainID, "block", target.ID(), "err", err)
			return
		}

		break
	}

	// Try to rewind and stop if it succeeds
	err = r.db.RewindLogs(ev.ChainID, target)
	if err != nil {
		r.log.Error("failed to rewind logs DB", "chain", ev.ChainID, "err", err)
		return
	}

	// Emit event to trigger node reset with new heads
	r.emitter.Emit(superevents.ChainRewoundEvent{ChainID: ev.ChainID})
}

// rewindL1ChainIfReorged rewinds the L1 chain for the given chain ID if a reorg is detected
// It checks the local-safe head against the canonical L1 block at the same height
func (r *Rewinder) rewindL1ChainIfReorged(chainID eth.ChainID, newTip eth.BlockID) error {
	// Get the current LocalSafe head and its L1 block
	localSafe, err := r.db.LocalSafe(chainID)
	if err != nil {
		return fmt.Errorf("failed to get local safe for chain %s: %w", chainID, err)
	}
	localSafeL1 := localSafe.Source

	// Get the canonical L1 block at our local head's height
	canonicalL1, err := r.l1Node.L1BlockRefByNumber(context.Background(), localSafeL1.Number)
	if err != nil {
		return fmt.Errorf("failed to get canonical L1 block at height %d: %w", localSafeL1.Number, err)
	}

	// If we're still on the canonical chain, nothing to do
	if canonicalL1.Hash == localSafeL1.Hash {
		return nil
	}

	// Get the finalized block as our lower bound
	finalized, err := r.db.Finalized(chainID)
	if err != nil {
		// If we don't have a finalized block, use the genesis block
		if errors.Is(err, types.ErrFuture) {
			finalized, err = r.db.FindSealedBlock(chainID, 0)
			if err != nil {
				return fmt.Errorf("failed to get index 0 block for chain %s: %w", chainID, err)
			}
		} else {
			return fmt.Errorf("failed to get finalized block for chain %s: %w", chainID, err)
		}
	}
	finalizedL1, err := r.db.CrossDerivedToSourceRef(chainID, finalized.ID())
	if err != nil {
		return fmt.Errorf("failed to get finalized L1 block for chain %s: %w", chainID, err)
	}

	// Find the common ancestor by walking back through L1 blocks
	commonL1Ancestor := finalizedL1.ID()
	currentL1 := localSafeL1.ID()
	for currentL1.Number >= finalizedL1.Number {
		// Get the canonical L1 block at this height from the node
		remoteL1, err := r.l1Node.L1BlockRefByNumber(context.Background(), currentL1.Number)
		if err != nil {
			return fmt.Errorf("failed to get L1 block at height %d: %w", currentL1.Number, err)
		}

		// If hashes match, we found the common ancestor
		if remoteL1.Hash == currentL1.Hash {
			commonL1Ancestor = currentL1
			break
		}

		// Get the previous L1 block from our DB
		prevSource, err := r.db.PreviousSource(chainID, currentL1)
		if err != nil {
			// If we hit the first block, use it as common ancestor
			if errors.Is(err, types.ErrPreviousToFirst) {
				// Still need to verify this block is canonical
				remoteFirst, err := r.l1Node.L1BlockRefByNumber(context.Background(), currentL1.Number)
				if err != nil {
					return fmt.Errorf("failed to get first L1 block: %w", err)
				}
				if remoteFirst.Hash == currentL1.Hash {
					commonL1Ancestor = currentL1
				} else {
					// First block isn't canonical, use finalized
					commonL1Ancestor = finalizedL1.ID()
				}
				break
			}
			return fmt.Errorf("failed to get previous L1 block: %w", err)
		}

		// Move to the parent
		currentL1 = prevSource.ID()
	}

	// Rewind LocalSafe to not include data derived from the old L1 chain
	if err := r.db.RewindLocalSafe(chainID, commonL1Ancestor); err != nil {
		if errors.Is(err, types.ErrFuture) {
			r.log.Warn("Rewinding on L1 reorg, but local-safe DB does not have L1 block", "block", commonL1Ancestor, "err", err)
		} else {
			return fmt.Errorf("failed to rewind local-safe for chain %s: %w", chainID, err)
		}
	}

	// Rewind CrossSafe to not include data derived from the old L1 chain
	if err := r.db.RewindCrossSafe(chainID, commonL1Ancestor); err != nil {
		if errors.Is(err, types.ErrFuture) {
			r.log.Warn("Rewinding on L1 reorg, but cross-safe DB does not have L1 block", "block", commonL1Ancestor, "err", err)
		} else {
			return fmt.Errorf("failed to rewind cross-safe for chain %s: %w", chainID, err)
		}
	}

	// Emit rewound event for sync node
	r.emitter.Emit(superevents.ChainRewoundEvent{
		ChainID: chainID,
	})
	return nil
}
