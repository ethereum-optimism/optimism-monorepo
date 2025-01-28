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

/*

Let me help design this reorg handling system. First, let's outline the key requirements and constraints:

# Requirements

1. Handle reorgs separately for Safe and Unsafe block categories
2. For each safety category, handle Local and Cross DBs together
3. Find the correct rollback height when divergence is detected
4. Maintain consistency between Cross and Local DBs
5. Handle edge cases like Safe head being ahead of Unsafe head
6. Provide clean interface to L2 node for block verification

# Core Algorithm Pseudocode

```go
type BlockSafety int
const (
    Safe BlockSafety = iota
    Unsafe
)

type ReorgResult struct {
    LocalRollbackHeight  uint64
    CrossRollbackHeight  uint64
    NewCanonicalHeight   uint64
    NeedsReorg          bool
}

// Main entry point for handling reorgs
func AttemptReorg(safety BlockSafety, badBlock Block) (ReorgResult, error) {
    // Get current heights from our DBs
    localHeight := GetLocalHeight(safety)
    crossHeight := GetCrossHeight(safety)

    // Find latest common ancestor with L2 node
    ancestor, err := findLatestCommonAncestor(safety, localHeight)
    if err != nil {
        return ReorgResult{}, err
    }

    // If ancestor is our head, no reorg needed
    if ancestor == localHeight {
        return ReorgResult{NeedsReorg: false}, nil
    }

    // Determine rollback heights
    crossRollback := min(ancestor, crossHeight)
    localRollback := ancestor

    return ReorgResult{
        LocalRollbackHeight: localRollback,
        CrossRollbackHeight: crossRollback,
        NewCanonicalHeight: ancestor,
        NeedsReorg: true,
    }, nil
}

// Binary search to find latest common ancestor
func findLatestCommonAncestor(safety BlockSafety, localHeight uint64) (uint64, error) {
    start := uint64(0)
    end := localHeight

    for start <= end {
        mid := (start + end) / 2

        localHash := GetBlockHashFromDB(safety, mid)
        nodeHash, err := L2Node.BlockHashAtHeight(mid)
        if err != nil {
            return 0, err
        }

        if localHash == nodeHash {
            // Check if next block diverges
            if mid == localHeight {
                return mid, nil
            }

            nextLocalHash := GetBlockHashFromDB(safety, mid+1)
            nextNodeHash, err := L2Node.BlockHashAtHeight(mid+1)
            if err != nil {
                return 0, err
            }

            if nextLocalHash != nextNodeHash {
                return mid, nil
            }

            // Continue searching higher
            start = mid + 1
        } else {
            // Diverged, search lower
            end = mid - 1
        }
    }

    return start, nil
}
```

# Key Considerations

1. **Cross DB Consistency**
   - Cross DB is always a subset of Local DB
   - When rolling back Cross DB, must roll back Local DB to at least that point
   - Local DB can be rolled back independently if divergence is after Cross height

2. **Safe vs Unsafe Interaction**
   - Handle independently to avoid complexity
   - Need to handle case where Safe head > Unsafe head
   - Consider adding validation to ensure Safe blocks don't get rolled back to before Unsafe blocks

3. **Error Handling**
   - L2 node communication failures
   - DB consistency checks
   - Height validation
   - Cross/Local DB synchronization issues

4. **Performance Optimization**
   - Binary search for finding divergence point
   - Minimize L2 node API calls
   - Batch DB operations where possible

# Implementation Steps

1. Implement core reorg detection logic
2. Add DB interfaces for Safe/Unsafe and Local/Cross combinations
3. Implement rollback mechanics for each DB type
4. Add validation and error handling
5. Add metrics and logging
6. Add recovery mechanisms for failed reorgs
7. Implement tests covering edge cases


*/

type rewinderDB interface {
	RewindLocalUnsafe(eth.ChainID, types.BlockSeal) error
	RewindCrossUnsafe(eth.ChainID, types.BlockSeal) error
	RewindLocalSafe(eth.ChainID, types.BlockSeal) error
	RewindCrossSafe(eth.ChainID, types.BlockSeal) error

	FindSealedBlock(eth.ChainID, uint64) (types.BlockSeal, error)
	LocalUnsafe(eth.ChainID) (types.BlockSeal, error)
	CrossUnsafe(eth.ChainID) (types.BlockSeal, error)
	LocalSafe(eth.ChainID) (types.DerivedBlockSealPair, error)
	CrossSafe(eth.ChainID) (types.DerivedBlockSealPair, error)
	Finalized(eth.ChainID) (types.BlockSeal, error)
	InvalidateLocalSafe(eth.ChainID, types.DerivedBlockRefPair) error
}

type syncNode interface {
	BlockRefByNumber(ctx context.Context, number uint64) (eth.BlockRef, error)
}

// Rewinder is responsible for handling invalidation events by coordinating
// the rewind of databases and resetting of chain processors.
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

func (r *Rewinder) handleEventRewindChain(ev superevents.RewindChainEvent) error {
	if err := r.attemptRewindSafe(ev.ChainID, ev.BadBlock); err != nil {
		return fmt.Errorf("failed to rewind safe chain %s: %w", ev.ChainID, err)
	}
	if err := r.attemptRewindUnsafe(ev.ChainID, ev.BadBlock); err != nil {
		return fmt.Errorf("failed to rewind unsafe chain %s: %w", ev.ChainID, err)
	}
	return nil
}

func (r *Rewinder) attemptRewindUnsafe(chainID eth.ChainID, badBlock eth.L2BlockRef) error {
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

	// Find the latest common newHead between the bad block's parent and the finalized head
	newHead, err := r.findLatestCommonAncestor(chainID, badBlock.ParentID(), finalizedHead)
	if err != nil {
		return fmt.Errorf("failed to find common ancestor for chain %s: %w", chainID, err)
	}

	// Reset cross-unsafe if it's newer than the newHead
	crossUnsafe, err := r.db.CrossUnsafe(chainID)
	if err != nil {
		return fmt.Errorf("failed to get cross-unsafe for chain %s: %w", chainID, err)
	}
	if crossUnsafe.Number >= newHead.Number {
		r.log.Info("rewinding cross-unsafe", "chain", chainID, "newHead", newHead.Number)
		if err := r.db.RewindCrossUnsafe(chainID, newHead); err != nil {
			return fmt.Errorf("failed to rewind cross-unsafe for chain %s: %w", chainID, err)
		}
	}

	// Rewind local-unsafe if it's newer than the newHead
	localHead, err := r.db.LocalUnsafe(chainID)
	if err != nil {
		return err
	}
	if localHead.Number >= newHead.Number {
		r.log.Info("rewinding local-unsafe", "chain", chainID, "newHead", newHead.Number)
		if err := r.db.RewindLocalUnsafe(chainID, newHead); err != nil {
			return fmt.Errorf("failed to rewind local-unsafe for chain %s: %w", chainID, err)
		}
	}

	return nil
}

func (r *Rewinder) attemptRewindSafe(chainID eth.ChainID, badBlock eth.L2BlockRef) error {
	finalizedHead, err := r.db.Finalized(chainID)
	if err != nil {
		// TODO: handle this
		finalizedHead = types.BlockSeal{}
	}

	// If we're not ahead of the finalized head, no reorg needed
	if badBlock.Number-1 < finalizedHead.Number {
		r.log.Warn("requested head is not ahead of finalized head", "chain", chainID, "requested", badBlock.Number-1, "finalized", finalizedHead.Number)
		return nil
	}

	// Find the latest common newHead between the candidate's parent and the finalized head
	newHead, err := r.findLatestCommonAncestor(chainID, badBlock.ParentID(), finalizedHead)
	if err != nil {
		return fmt.Errorf("failed to find common ancestor for chain %s: %w", chainID, err)
	}

	// Reset cross-safe if it's newer than the ancestor
	crossSafePair, err := r.db.CrossSafe(chainID)
	if err != nil {
		return fmt.Errorf("failed to get cross-safe for chain %s: %w", chainID, err)
	}
	crossSafe := crossSafePair.Derived
	if crossSafe.Number >= newHead.Number {
		r.log.Info("rewinding cross-safe", "chain", chainID, "newHead", newHead.Number)
		if err := r.db.RewindCrossSafe(chainID, newHead); err != nil {
			return fmt.Errorf("failed to rewind cross-safe for chain %s: %w", chainID, err)
		}
	}

	// Rewind local-safe if it's newer than the newHead
	localHeadPair, err := r.db.LocalSafe(chainID)
	if err != nil {
		return err
	}
	localHead := localHeadPair.Derived
	if localHead.Number >= newHead.Number {
		r.log.Info("rewinding local-safe", "chain", chainID, "newHead", newHead.Number)
		if err := r.db.RewindLocalSafe(chainID, newHead); err != nil {
			return fmt.Errorf("failed to rewind local-safe for chain %s: %w", chainID, err)
		}
	}

	return nil
}

func (r *Rewinder) findLatestCommonAncestor(chainID eth.ChainID, startBlock eth.BlockID, finalizedBlock types.BlockSeal) (types.BlockSeal, error) {
	syncNode, ok := r.syncNodes.Get(chainID)
	if !ok {
		return types.BlockSeal{}, fmt.Errorf("sync node not found for chain %s", chainID)
	}

	// Linear search from local height down to finalized height
	r.log.Info("searching for common ancestor", "chain", chainID, "local", startBlock.Number, "finalized", finalizedBlock.Number)
	finalizedHeight := int64(finalizedBlock.Number)
	for height := int64(startBlock.Number); height >= finalizedHeight; height-- {
		remoteRef, err := syncNode.BlockRefByNumber(context.Background(), uint64(height))
		if err != nil {
			return types.BlockSeal{}, err
		}

		localRef, err := r.db.FindSealedBlock(chainID, uint64(height))
		if err != nil {
			return types.BlockSeal{}, err
		}

		if localRef.Hash == remoteRef.Hash {
			return localRef, nil
		}
	}

	return finalizedBlock, nil
}
