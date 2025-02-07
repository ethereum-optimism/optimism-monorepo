package db

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/superevents"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

func (db *ChainsDB) AddLog(
	chain eth.ChainID,
	logHash common.Hash,
	parentBlock eth.BlockID,
	logIdx uint32,
	execMsg *types.ExecutingMessage) error {
	logDB, ok := db.logDBs.Get(chain)
	if !ok {
		return fmt.Errorf("cannot AddLog: %w: %v", types.ErrUnknownChain, chain)
	}
	return logDB.AddLog(logHash, parentBlock, logIdx, execMsg)
}

func (db *ChainsDB) SealBlock(chain eth.ChainID, block eth.BlockRef) error {
	logDB, ok := db.logDBs.Get(chain)
	if !ok {
		return fmt.Errorf("cannot SealBlock: %w: %v", types.ErrUnknownChain, chain)
	}
	err := logDB.SealBlock(block.ParentHash, block.ID(), block.Time)
	if err != nil {
		return fmt.Errorf("failed to seal block %v: %w", block, err)
	}
	db.logger.Info("Updated local unsafe", "chain", chain, "block", block)
	db.emitter.Emit(superevents.LocalUnsafeUpdateEvent{
		ChainID:        chain,
		NewLocalUnsafe: block,
	})
	return nil
}

func (db *ChainsDB) Rewind(chain eth.ChainID, headBlock eth.BlockID) error {
	// Rewind the logDB
	logDB, ok := db.logDBs.Get(chain)
	if !ok {
		return fmt.Errorf("cannot Rewind: %w: %s", types.ErrUnknownChain, chain)
	}
	if err := logDB.Rewind(headBlock); err != nil {
		return fmt.Errorf("failed to rewind to block %v: %w", headBlock, err)
	}

	// Rewind the localDB
	localDB, ok := db.localDBs.Get(chain)
	if !ok {
		return fmt.Errorf("cannot Rewind (localDB not found): %w: %s", types.ErrUnknownChain, chain)
	}
	if err := localDB.RewindToFirstDerived(headBlock); err != nil {
		return fmt.Errorf("failed to rewind localDB to block %v: %w", headBlock, err)
	}

	// Rewind the crossDB
	crossDB, ok := db.crossDBs.Get(chain)
	if !ok {
		return fmt.Errorf("cannot Rewind (crossDB not found): %w: %s", types.ErrUnknownChain, chain)
	}
	if err := crossDB.RewindToFirstDerived(headBlock); err != nil {
		return fmt.Errorf("failed to rewind crossDB to block %v: %w", headBlock, err)
	}
	return nil
}

func (db *ChainsDB) UpdateLocalSafe(chain eth.ChainID, derivedFrom eth.BlockRef, lastDerived eth.BlockRef) {
	logger := db.logger.New("chain", chain, "derivedFrom", derivedFrom, "lastDerived", lastDerived)
	localDB, ok := db.localDBs.Get(chain)
	if !ok {
		logger.Error("Cannot update local-safe DB, unknown chain")
		return
	}
	logger.Debug("Updating local safe DB")
	if err := localDB.AddDerived(derivedFrom, lastDerived); err != nil {
		if errors.Is(err, types.ErrIneffective) {
			logger.Info("Node is syncing known source blocks on known latest local-safe block", "err", err)
			return
		}
		logger.Warn("Failed to update local safe", "err", err)
		db.emitter.Emit(superevents.LocalSafeOutOfSyncEvent{
			ChainID: chain,
			L1Ref:   derivedFrom,
			Err:     err,
		})
		return
	}
	logger.Info("Updated local safe DB")
	db.emitter.Emit(superevents.LocalSafeUpdateEvent{
		ChainID: chain,
		NewLocalSafe: types.DerivedBlockSealPair{
			Source:  types.BlockSealFromRef(derivedFrom),
			Derived: types.BlockSealFromRef(lastDerived),
		},
	})
}

func (db *ChainsDB) UpdateCrossUnsafe(chain eth.ChainID, crossUnsafe types.BlockSeal) error {
	v, ok := db.crossUnsafe.Get(chain)
	if !ok {
		return fmt.Errorf("cannot UpdateCrossUnsafe: %w: %s", types.ErrUnknownChain, chain)
	}
	v.Set(crossUnsafe)
	db.logger.Info("Updated cross-unsafe", "chain", chain, "crossUnsafe", crossUnsafe)
	db.emitter.Emit(superevents.CrossUnsafeUpdateEvent{
		ChainID:        chain,
		NewCrossUnsafe: crossUnsafe,
	})
	db.m.RecordCrossUnsafeRef(chain, eth.BlockRef{
		Number: crossUnsafe.Number,
		Time:   crossUnsafe.Timestamp,
		Hash:   crossUnsafe.Hash,
	})
	return nil
}

func (db *ChainsDB) UpdateCrossSafe(chain eth.ChainID, l1View eth.BlockRef, lastCrossDerived eth.BlockRef) error {
	crossDB, ok := db.crossDBs.Get(chain)
	if !ok {
		return fmt.Errorf("cannot UpdateCrossSafe: %w: %s", types.ErrUnknownChain, chain)
	}
	if err := crossDB.AddDerived(l1View, lastCrossDerived); err != nil {
		return err
	}
	db.logger.Info("Updated cross-safe", "chain", chain, "l1View", l1View, "lastCrossDerived", lastCrossDerived)
	db.emitter.Emit(superevents.CrossSafeUpdateEvent{
		ChainID: chain,
		NewCrossSafe: types.DerivedBlockSealPair{
			Source:  types.BlockSealFromRef(l1View),
			Derived: types.BlockSealFromRef(lastCrossDerived),
		},
	})
	db.m.RecordCrossSafeRef(chain, lastCrossDerived)
	return nil
}

func (db *ChainsDB) onFinalizedL1(finalized eth.BlockRef) {
	// Lock, so we avoid race-conditions in-between getting (for comparison) and setting.
	// Unlock is managed explicitly, in this function so we can call NotifyL2Finalized after releasing the lock.
	db.finalizedL1.Lock()

	if v := db.finalizedL1.Value; v != (eth.BlockRef{}) && v.Number > finalized.Number {
		db.finalizedL1.Unlock()
		db.logger.Warn("Cannot rewind finalized L1 block", "current", v, "signal", finalized)
		return
	}
	db.finalizedL1.Value = finalized
	db.logger.Info("Updated finalized L1", "finalizedL1", finalized)
	db.finalizedL1.Unlock()

	db.emitter.Emit(superevents.FinalizedL1UpdateEvent{
		FinalizedL1: finalized,
	})
	// whenever the L1 Finalized changes, the L2 Finalized may change, notify subscribers
	for _, chain := range db.depSet.Chains() {
		fin, err := db.Finalized(chain)
		if err != nil {
			db.logger.Warn("Unable to determine finalized L2 block", "chain", chain, "l1Finalized", finalized)
			continue
		}
		db.emitter.Emit(superevents.FinalizedL2UpdateEvent{ChainID: chain, FinalizedL2: fin})
	}
}

func (db *ChainsDB) InvalidateLocalSafe(chainID eth.ChainID, candidate types.DerivedBlockRefPair) error {
	// Get databases to invalidate data in.
	eventsDB, ok := db.logDBs.Get(chainID)
	if !ok {
		return fmt.Errorf("cannot find events DB of chain %s for invalidation: %w", chainID, types.ErrUnknownChain)
	}
	localSafeDB, ok := db.localDBs.Get(chainID)
	if !ok {
		return fmt.Errorf("cannot find local-safe DB of chain %s for invalidation: %w", chainID, types.ErrUnknownChain)
	}

	// Now invalidate the local-safe data.
	// We insert a marker, so we don't build on top of the invalidated block, until it is replaced.
	// And we won't index unsafe blocks, until it is replaced.
	if err := localSafeDB.RewindAndInvalidate(candidate); err != nil {
		return fmt.Errorf("failed to invalidate entry in local-safe DB: %w", err)
	}

	// Change cross-unsafe, if it's equal or past the invalidated block.
	if err := db.ResetCrossUnsafeIfNewerThan(chainID, candidate.Derived.Number); err != nil {
		return fmt.Errorf("failed to reset cross-unsafe: %w", err)
	}

	// Drop the events of the invalidated block and after,
	// by rewinding to only keep the parent-block.
	if err := eventsDB.Rewind(candidate.Derived.ParentID()); err != nil {
		return fmt.Errorf("failed to rewind unsafe-chain: %w", err)
	}

	// Create an event, that subscribed sync-nodes can listen to,
	// to start finding the replacement block.
	db.emitter.Emit(superevents.InvalidateLocalSafeEvent{
		ChainID:   chainID,
		Candidate: candidate,
	})
	return nil
}

// RewindLocalSafe removes all local-safe blocks after the given new derived-from scope.
// Note that this drop L1 blocks that resulted in a previously invalidated local-safe block.
// This returns ErrFuture if the block is newer than the last known block.
// This returns ErrConflict if a different block at the given height is known.
func (db *ChainsDB) RewindLocalSafe(chainID eth.ChainID, scope eth.BlockID) error {
	localSafeDB, ok := db.localDBs.Get(chainID)
	if !ok {
		return fmt.Errorf("cannot find local-safe DB of chain %s for invalidation: %w", chainID, types.ErrUnknownChain)
	}
	if err := localSafeDB.RewindToScope(scope); err != nil {
		return fmt.Errorf("failed to rewind local-safe: %w", err)
	}
	return nil
}

// RewindCrossSafe removes all cross-safe blocks after the given new derived-from scope.
// This returns ErrFuture if the block is newer than the last known block.
// This returns ErrConflict if a different block at the given height is known.
func (db *ChainsDB) RewindCrossSafe(chainID eth.ChainID, scope eth.BlockID) error {
	crossSafeDB, ok := db.crossDBs.Get(chainID)
	if !ok {
		return fmt.Errorf("cannot find cross-safe DB of chain %s for invalidation: %w", chainID, types.ErrUnknownChain)
	}
	if err := crossSafeDB.RewindToScope(scope); err != nil {
		return fmt.Errorf("failed to rewind cross-safe: %w", err)
	}
	return nil
}

func (db *ChainsDB) RewindLogs(chainID eth.ChainID, newHead types.BlockSeal) error {
	eventsDB, ok := db.logDBs.Get(chainID)
	if !ok {
		return fmt.Errorf("cannot find events DB of chain %s for invalidation: %w", chainID, types.ErrUnknownChain)
	}
	if err := eventsDB.Rewind(newHead.ID()); err != nil {
		return fmt.Errorf("failed to rewind logs of chain %s: %w", chainID, err)
	}
	return nil
}

func (db *ChainsDB) ResetCrossUnsafeIfNewerThan(chainID eth.ChainID, number uint64) error {
	crossUnsafe, ok := db.crossUnsafe.Get(chainID)
	if !ok {
		return nil
	}

	crossSafeDB, ok := db.crossDBs.Get(chainID)
	if !ok {
		return fmt.Errorf("cannot find cross-safe DB of chain %s for invalidation: %w", chainID, types.ErrUnknownChain)
	}
	crossSafe, err := crossSafeDB.Last()
	if err != nil {
		return fmt.Errorf("cannot get cross-safe of chain %s: %w", chainID, err)
	}

	// Reset cross-unsafe if it's equal or newer than the given block number
	crossUnsafe.Lock()
	x := crossUnsafe.Value
	defer crossUnsafe.Unlock()
	if x.Number >= number {
		db.logger.Warn("Resetting cross-unsafe to cross-safe, since prior block was invalidated",
			"crossUnsafe", x, "crossSafe", crossSafe, "number", number)
		crossUnsafe.Value = crossSafe.Derived
	}
	return nil
}

func (db *ChainsDB) onReplaceBlock(chainID eth.ChainID, replacement eth.BlockRef, invalidated common.Hash) {
	localSafeDB, ok := db.localDBs.Get(chainID)
	if !ok {
		db.logger.Error("Cannot find DB for replacement block", "chain", chainID)
		return
	}

	result, err := localSafeDB.ReplaceInvalidatedBlock(replacement, invalidated)
	if err != nil {
		db.logger.Error("Cannot replace invalidated block in local-safe DB",
			"invalidated", invalidated, "replacement", replacement, "err", err)
		return
	}
	// Consider the replacement as a new local-unsafe block, so we can try to index the new event-data.
	db.emitter.Emit(superevents.LocalUnsafeReceivedEvent{
		ChainID:        chainID,
		NewLocalUnsafe: replacement,
	})
	// The local-safe DB changed, so emit an event, so other sub-systems can react to the change.
	db.emitter.Emit(superevents.LocalSafeUpdateEvent{
		ChainID:      chainID,
		NewLocalSafe: result,
	})

	// TODO Make sure the events-DB has a matching block-hash with the replacement, roll it back otherwise.
}
