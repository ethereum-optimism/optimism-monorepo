package rewinder

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/locks"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/superevents"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type l1Node interface {
	L1BlockRefByNumber(ctx context.Context, number uint64) (eth.L1BlockRef, error)
}

type rewinderDB interface {
	LastDerivedFrom(chainID eth.ChainID, derivedFrom eth.BlockID) (derived types.BlockSeal, err error)

	LocalUnsafe(eth.ChainID) (types.BlockSeal, error)
	CrossUnsafe(eth.ChainID) (types.BlockSeal, error)
	LocalSafe(eth.ChainID) (types.DerivedBlockSealPair, error)
	CrossSafe(eth.ChainID) (types.DerivedBlockSealPair, error)

	RewindLocalUnsafe(eth.ChainID, types.BlockSeal) error
	RewindCrossUnsafe(eth.ChainID, types.BlockSeal) error
	RewindLocalSafe(eth.ChainID, types.BlockSeal) error
	RewindCrossSafe(eth.ChainID, types.BlockSeal) error
	RewindLogs(chainID eth.ChainID, newHead types.BlockSeal) error

	FindSealedBlock(eth.ChainID, uint64) (types.BlockSeal, error)
	Finalized(eth.ChainID) (types.BlockSeal, error)
}

type syncNode interface {
	BlockRefByNumber(ctx context.Context, number uint64) (eth.BlockRef, error)
}

// controller holds the methods for a rewind operation
type controller struct {
	isSafe bool

	getMinBlock func(eth.ChainID) (types.BlockSeal, error)
	getLocal    func(eth.ChainID) (types.BlockSeal, error)
	getCross    func(eth.ChainID) (types.BlockSeal, error)

	rewindLocal func(eth.ChainID, types.BlockSeal) error
	rewindCross func(eth.ChainID, types.BlockSeal) error
}

func (rc controller) safetyStr() string {
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
	l1Node    l1Node
	syncNodes locks.RWMap[eth.ChainID, syncNode]
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
	case superevents.RewindL2ChainEvent:
		r.handleRewindL2ChainEvent(x)
		return true
	case superevents.RewindL1Event:
		r.handleRewindL1Event(x)
		return true
	case superevents.LocalDerivedEvent:
		r.handleLocalDerivedEvent(x)
		return true
	default:
		return false
	}
}

func (r *Rewinder) AttachSyncNode(chainID eth.ChainID, source syncNode) {
	r.syncNodes.Set(chainID, source)
}

func (r *Rewinder) handleRewindL2ChainEvent(ev superevents.RewindL2ChainEvent) {
	if err := r.rewindBothChainsL2(ev); err != nil {
		r.log.Error("failed to rewind chain %s: %w", ev.ChainID, err)
	}
}

// handleRewindL1Event iterates known chains and checks each one for a reorg
// If a reorg is detected, it will rewind the chain to the latest common ancestor
// between the local-safe head and the finalized head.
func (r *Rewinder) handleRewindL1Event(ev superevents.RewindL1Event) {
	r.syncNodes.Range(func(chainID eth.ChainID, source syncNode) bool {
		if err := r.rewindL1ChainIfReorged(chainID, ev.IncomingBlock); err != nil {
			r.log.Error("failed to rewind L1 event for chain %s: %w", chainID, err)
		}
		return true
	})
}

// handleLocalDerivedEvent checks if the newly derived block matches what we have in our unsafe DB
// If it doesn't match, we need to rewind the logs DB to the common ancestor between
// the LocalUnsafe head and the new LocalSafe block
func (r *Rewinder) handleLocalDerivedEvent(ev superevents.LocalDerivedEvent) {
	derived := ev.Derived.Derived

	// Get the current unsafe head
	unsafeHead, err := r.db.LocalUnsafe(ev.ChainID)
	if err != nil {
		r.log.Error("failed to get unsafe head", "chain", ev.ChainID, "err", err)
		return
	}

	// If the unsafe head is before the derived block, nothing to do
	if unsafeHead.Number < derived.Number {
		return
	}

	// Get the block at the derived height from our unsafe chain
	unsafeAtDerived, err := r.db.FindSealedBlock(ev.ChainID, derived.Number)
	if err != nil {
		r.log.Error("failed to get unsafe block at derived height", "chain", ev.ChainID, "height", derived.Number, "err", err)
		return
	}

	// If the block hashes match, our unsafe chain is still valid
	if unsafeAtDerived.Hash == derived.Hash {
		return
	}

	// The new LocalSafe block is different than what we had so rewind to its parent
	unsafeParent := types.BlockSeal{
		Number: derived.Number - 1,
		Hash:   derived.ParentHash,
	}

	if err := r.db.RewindLogs(ev.ChainID, unsafeParent); err != nil {
		r.log.Error("failed to rewind logs DB", "chain", ev.ChainID, "err", err)
		return
	}

	// Emit event to trigger node reset with new heads
	r.emitter.Emit(superevents.ChainRewoundEvent{
		ChainID: ev.ChainID,
	})
}

// rewindL1ChainIfReorged rewinds the L1 chain for the given chain ID if a reorg is detected
// It checks the local-safe head against the canonical L1 block at the same height
func (r *Rewinder) rewindL1ChainIfReorged(chainID eth.ChainID, newTip eth.BlockID) error {
	// First check CrossSafe DB for reorg
	foundReorg, err := r.checkCrossSafeForReorg(chainID, newTip)
	if err != nil {
		return fmt.Errorf("failed to check cross-safe for reorg for chain %s: %w", chainID, err)
	}
	if !foundReorg {
		return nil
	}

	// We have a reorg so find the latest common L1 block and its last derived L2 block
	finalized, err := r.db.Finalized(chainID)
	if err != nil {
		return fmt.Errorf("failed to get finalized for chain %s: %w", chainID, err)
	}
	commonAncestor, err := r.findLatestCommonAncestorL1(chainID, newTip.Number, finalized)
	if err != nil {
		return fmt.Errorf("failed to find latest common ancestor for chain %s: %w", chainID, err)
	}
	crossSafeDerived, err := r.db.LastDerivedFrom(chainID, commonAncestor.ID())
	if err != nil {
		return fmt.Errorf("failed to get derived from for chain %s: %w", chainID, err)
	}

	// Rewind CrossSafe to the derived block
	if err := r.db.RewindCrossSafe(chainID, crossSafeDerived); err != nil {
		return fmt.Errorf("failed to rewind cross-safe for chain %s: %w", chainID, err)
	}

	// Now check LocalSafe DB for reorg
	localSafeDerived, err := r.checkLocalSafeForReorg(chainID, crossSafeDerived)
	if err != nil {
		return fmt.Errorf("failed to check local-safe for reorg for chain %s: %w", chainID, err)
	}

	// Rewind LocalSafe to the derived block
	if err := r.db.RewindLocalSafe(chainID, localSafeDerived); err != nil {
		return fmt.Errorf("failed to rewind local-safe for chain %s: %w", chainID, err)
	}

	// Emit rewound event for sync node
	r.emitter.Emit(superevents.ChainRewoundEvent{
		ChainID: chainID,
	})
	return nil
}

// checkCrossSafeForReorg checks if the CrossSafe DB needs to be rewound due to an L1 reorg
// It returns whether a reorg was found and the common ancestor block if one was found
func (r *Rewinder) checkCrossSafeForReorg(chainID eth.ChainID, newTip eth.BlockID) (bool, error) {
	// Get the L1 block of the cross-safe head
	crossSafe, err := r.db.CrossSafe(chainID)
	if err != nil {
		return false, fmt.Errorf("failed to get cross safe for chain %s: %w", chainID, err)
	}
	crossSafeL1 := crossSafe.DerivedFrom.ID()

	// Get the L1 block that's currently at our cross-safe height
	candidateBlock := newTip
	if newTip.Number != crossSafeL1.Number {
		candidateBlockRef, err := r.l1Node.L1BlockRefByNumber(context.Background(), crossSafeL1.Number)
		if err != nil {
			return false, fmt.Errorf("failed to get L1 block at cross-safe height for chain %s: %w", chainID, err)
		}
		candidateBlock = candidateBlockRef.ID()
	}

	// If the hashes match then we're still on the canonical chain and don't need to rewind
	if candidateBlock.Hash == crossSafeL1.Hash {
		return false, nil
	}

	return true, nil
}

// checkLocalSafeForReorg checks if the LocalSafe DB needs to be rewound due to an L1 reorg
// It takes the common ancestor found during the CrossSafe reorg check as a starting point
func (r *Rewinder) checkLocalSafeForReorg(chainID eth.ChainID, commonAncestor types.BlockSeal) (types.BlockSeal, error) {
	// Get the L1 block of the local-safe head
	localSafe, err := r.db.LocalSafe(chainID)
	if err != nil {
		return types.BlockSeal{}, fmt.Errorf("failed to get local safe for chain %s: %w", chainID, err)
	}

	// Get the last L2 block derived from the common ancestor
	derived, err := r.db.LastDerivedFrom(chainID, commonAncestor.ID())
	if err != nil {
		return types.BlockSeal{}, fmt.Errorf("failed to get derived from for chain %s: %w", chainID, err)
	}

	// If the local-safe head is ahead of the derived block, we need to rewind to the derived block
	if localSafe.Derived.Number > derived.Number {
		return derived, nil
	}

	// Otherwise we can keep the local-safe head
	return localSafe.Derived, nil
}

// rewindTo rewinds the given chain to the given head unconditionally
// It's generic over both Safe and Unsafe rewinds and handles both local and cross databases
func (r *Rewinder) rewindTo(chainID eth.ChainID, newHead types.BlockSeal, ctrl controller) error {
	// Rewind the local db if it's newer than the newHead
	localHead, err := ctrl.getLocal(chainID)
	if err != nil {
		return err
	}

	// If the newHead is after our local head then there's nothing to do
	if newHead.Number >= localHead.Number {
		return nil
	}

	// Rewind the local db
	r.log.Info("rewinding local", "safety", ctrl.safetyStr(), "chain", chainID, "newHead", newHead.Number)
	if err := ctrl.rewindLocal(chainID, newHead); err != nil {
		return fmt.Errorf("failed to rewind local for chain %s: %w", chainID, err)
	}

	// If the newHead is before our cross head then rewind the cross db
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

	return nil
}

func (r *Rewinder) controllerSafe() controller {
	return controller{
		isSafe:      true,
		getMinBlock: r.db.Finalized,
		getLocal:    pairToSealGetter(r.db.LocalSafe),
		getCross:    pairToSealGetter(r.db.CrossSafe),
		rewindLocal: r.db.RewindLocalSafe,
		rewindCross: r.db.RewindCrossSafe,
	}
}

func (r *Rewinder) controllerUnsafe() controller {
	return controller{
		isSafe: false,

		// use local safe as earliest block for rewinding the unsafe chain
		getMinBlock: pairToSealGetter(r.db.LocalSafe),
		getLocal:    r.db.LocalUnsafe,
		getCross:    r.db.CrossUnsafe,
		rewindLocal: r.db.RewindLocalUnsafe,
		rewindCross: r.db.RewindCrossUnsafe,
	}
}

func (r *Rewinder) findLatestCommonAncestorL1(chainID eth.ChainID, maxBlock uint64, minBlock types.BlockSeal) (types.BlockSeal, error) {
	r.log.Info("searching for L1 common ancestor", "chain", chainID, "maxBlock", maxBlock, "minBlock", minBlock.Number)
	minHeight := int64(minBlock.Number)
	for height := int64(maxBlock); height >= minHeight; height-- {
		// Load the L1 block at this height from the node and the local db
		remoteRef, err := r.l1Node.L1BlockRefByNumber(context.Background(), uint64(height))
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

	// If we didn't find a common ancestor then return the min block
	r.log.Warn("no common L1 ancestor found for chain %s, rewinding to the minimum block", chainID)
	return minBlock, nil
}

// pairToSealGetter wraps a DerivedBlockSealPair getter and makes it a BlockSeal getter
// by returning the derived block seal from the pair instead of the pair itself.
// This allows us to utilize pair getters in our rewind interface that requires seal getters.
func pairToSealGetter(fn func(eth.ChainID) (types.DerivedBlockSealPair, error)) func(eth.ChainID) (types.BlockSeal, error) {
	return func(chainID eth.ChainID) (types.BlockSeal, error) {
		pair, err := fn(chainID)
		if err != nil {
			return types.BlockSeal{}, err
		}
		return pair.Derived, nil
	}
}

//
// OLD
//

// rewindBothChainsL2 is the main entrypoint into a rewind call.
// It attempts to rewind the local and cross databases for the given chain for both safety levels.
// It returns an error if the rewind fails for either safety level.
// For each safety level, it will check between the bad block's parent and the finalized head
// until it finds a match and then will rewind to that point.
func (r *Rewinder) rewindBothChainsL2(ev superevents.RewindL2ChainEvent) error {
	if err := r.attemptRewindL2Safe(ev.ChainID, ev.BadBlockHeight); err != nil {
		return fmt.Errorf("failed to rewind safe chain %s: %w", ev.ChainID, err)
	}
	if err := r.attemptRewindL2Unsafe(ev.ChainID, ev.BadBlockHeight); err != nil {
		return fmt.Errorf("failed to rewind unsafe chain %s: %w", ev.ChainID, err)
	}
	return nil
}

// attemptRewindL2 attempts to rewind the local and cross databases for the given chain and safety level
func (r *Rewinder) attemptRewindL2(chainID eth.ChainID, badBlockHeight uint64, ctrl controller) error {
	// First get the minimum block we can rewind to
	minBlock, err := ctrl.getMinBlock(chainID)
	if err != nil {
		if errors.Is(err, types.ErrFuture) {
			minBlock = types.BlockSeal{}
		} else {
			return fmt.Errorf("failed to get finalized head for chain %s: %w", chainID, err)
		}
	}

	// If the badBlock target would remove our min block, return early.
	// The min block content is assumed to be irreversible for this safety level.
	if badBlockHeight <= minBlock.Number {
		r.log.Warn("requested head is not ahead of finalized head", "chain", chainID, "requested", badBlockHeight, "minBlock", minBlock.Number)
		return nil
	}

	// Find the latest common newHead between the parent and the min block
	newHead, err := r.findLatestCommonAncestor(chainID, badBlockHeight-1, minBlock)
	if err != nil {
		return fmt.Errorf("failed to find common ancestor for chain %s: %w", chainID, err)
	}

	// Rewind to the common ancestor
	return r.rewindTo(chainID, newHead, ctrl)
}

// attemptRewindL2Unsafe attempts to rewind unsafe blocks
func (r *Rewinder) attemptRewindL2Unsafe(chainID eth.ChainID, badBlockHeight uint64) error {
	return r.attemptRewindL2(chainID, badBlockHeight, r.controllerUnsafe())
}

// attemptRewindL2Safe attempts to rewind safe blocks
func (r *Rewinder) attemptRewindL2Safe(chainID eth.ChainID, badBlockHeight uint64) error {
	return r.attemptRewindL2(chainID, badBlockHeight, r.controllerSafe())
}

// findLatestCommonAncestor finds the latest common ancestor between the startBlock and the finalizedBlock
// by searching for the last block that exists in both the local db and the L2 node.
// If no common ancestor is found then the finalized head is returned.
func (r *Rewinder) findLatestCommonAncestor(chainID eth.ChainID, maxBlock uint64, minBlock types.BlockSeal) (types.BlockSeal, error) {
	syncNode, ok := r.syncNodes.Get(chainID)
	if !ok {
		return types.BlockSeal{}, fmt.Errorf("sync node not found for chain %s", chainID)
	}

	// Linear search from max block down to min block
	r.log.Info("searching for common ancestor", "chain", chainID, "local", maxBlock, "finalized", minBlock.Number)
	minHeight := int64(minBlock.Number)
	for height := int64(maxBlock); height >= minHeight; height-- {
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

	// If we didn't find a common ancestor then return the min block
	r.log.Warn("no common ancestor found for chain %s, rewinding to the minimum block", chainID)
	return minBlock, nil
}
