package db

import (
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

func (db *ChainsDB) Rewind(chain eth.ChainID, headBlockNum uint64) error {
	logDB, ok := db.logDBs.Get(chain)
	if !ok {
		return fmt.Errorf("cannot Rewind: %w: %s", types.ErrUnknownChain, chain)
	}
	return logDB.Rewind(headBlockNum)
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
		db.logger.Warn("Failed to update local safe")
		db.emitter.Emit(superevents.LocalSafeOutOfSyncEvent{
			ChainID: chain,
			L1Ref:   derivedFrom,
			Err:     err,
		})
		return
	}
	db.logger.Info("Updated local safe DB")
	db.emitter.Emit(superevents.LocalSafeUpdateEvent{
		ChainID: chain,
		NewLocalSafe: types.DerivedBlockSealPair{
			DerivedFrom: types.BlockSealFromRef(derivedFrom),
			Derived:     types.BlockSealFromRef(lastDerived),
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
			DerivedFrom: types.BlockSealFromRef(l1View),
			Derived:     types.BlockSealFromRef(lastCrossDerived),
		},
	})
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
