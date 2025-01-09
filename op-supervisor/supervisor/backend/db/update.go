package db

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

func (db *ChainsDB) AddLog(
	chain types.ChainID,
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

func (db *ChainsDB) SealBlock(chain types.ChainID, block eth.BlockRef) error {
	logDB, ok := db.logDBs.Get(chain)
	if !ok {
		return fmt.Errorf("cannot SealBlock: %w: %v", types.ErrUnknownChain, chain)
	}
	err := logDB.SealBlock(block.ParentHash, block.ID(), block.Time)
	if err != nil {
		return fmt.Errorf("failed to seal block %v: %w", block, err)
	}
	db.logger.Info("Updated local unsafe", "chain", chain, "block", block)
	feed, ok := db.localUnsafeFeeds.Get(chain)
	if ok {
		feed.Send(types.BlockSealFromRef(block))
	}
	return nil
}

func (db *ChainsDB) Rewind(chain types.ChainID, headBlockNum uint64) error {
	logDB, ok := db.logDBs.Get(chain)
	if !ok {
		return fmt.Errorf("cannot Rewind: %w: %s", types.ErrUnknownChain, chain)
	}
	return logDB.Rewind(headBlockNum)
}

func (db *ChainsDB) UpdateLocalSafe(chain types.ChainID, derivedFrom eth.BlockRef, lastDerived eth.BlockRef) error {
	localDB, ok := db.localDBs.Get(chain)
	if !ok {
		return fmt.Errorf("cannot UpdateLocalSafe: %w: %v", types.ErrUnknownChain, chain)
	}
	db.logger.Debug("Updating local safe", "chain", chain, "derivedFrom", derivedFrom, "lastDerived", lastDerived)
	if err := localDB.AddDerived(derivedFrom, lastDerived); err != nil {
		return err
	}
	feed, ok := db.localSafeFeeds.Get(chain)
	if ok {
		feed.Send(types.DerivedBlockSealPair{
			DerivedFrom: types.BlockSealFromRef(derivedFrom),
			Derived:     types.BlockSealFromRef(lastDerived),
		})
	}
	return nil
}

func (db *ChainsDB) UpdateCrossUnsafe(chain types.ChainID, crossUnsafe types.BlockSeal) error {
	v, ok := db.crossUnsafe.Get(chain)
	if !ok {
		return fmt.Errorf("cannot UpdateCrossUnsafe: %w: %s", types.ErrUnknownChain, chain)
	}
	v.Set(crossUnsafe)
	feed, ok := db.crossUnsafeFeeds.Get(chain)
	if ok {
		feed.Send(crossUnsafe)
	}
	db.logger.Info("Updated cross-unsafe", "chain", chain, "crossUnsafe", crossUnsafe)
	return nil
}

func (db *ChainsDB) UpdateCrossSafe(chain types.ChainID, l1View eth.BlockRef, lastCrossDerived eth.BlockRef) error {
	crossDB, ok := db.crossDBs.Get(chain)
	if !ok {
		return fmt.Errorf("cannot UpdateCrossSafe: %w: %s", types.ErrUnknownChain, chain)
	}
	if err := crossDB.AddDerived(l1View, lastCrossDerived); err != nil {
		return err
	}
	db.logger.Info("Updated cross-safe", "chain", chain, "l1View", l1View, "lastCrossDerived", lastCrossDerived)
	// notify subscribers
	sub, ok := db.crossSafeFeeds.Get(chain)
	if ok {
		sub.Send(types.DerivedBlockSealPair{
			DerivedFrom: types.BlockSealFromRef(l1View),
			Derived:     types.BlockSealFromRef(lastCrossDerived),
		})
	}
	return nil
}

func (db *ChainsDB) UpdateFinalizedL1(finalized eth.BlockRef) error {
	// Lock, so we avoid race-conditions in-between getting (for comparison) and setting.
	// Unlock is managed explicitly, in this function so we can call NotifyL2Finalized after releasing the lock.
	db.finalizedL1.Lock()

	if v := db.finalizedL1.Value; v.Number > finalized.Number {
		db.finalizedL1.Unlock()
		return fmt.Errorf("cannot rewind finalized L1 head from %s to %s", v, finalized)
	}
	db.finalizedL1.Value = finalized
	db.logger.Info("Updated finalized L1", "finalizedL1", finalized)
	db.finalizedL1.Unlock()

	// whenver the L1 Finalized changes, the L2 Finalized may change, notify subscribers
	db.NotifyL2Finalized()

	return nil
}

// NotifyL2Finalized notifies all L2 finality subscribers of the latest L2 finalized block, per chain.
func (db *ChainsDB) NotifyL2Finalized() {
	for _, chain := range db.depSet.Chains() {
		f, err := db.Finalized(chain)
		if err != nil {
			db.logger.Error("Failed to get finalized L1 block", "chain", chain, "err", err)
			continue
		}
		sub, ok := db.l2FinalityFeeds.Get(chain)
		if ok {
			sub.Send(f)
		}
	}
}

// RecordNewL1 records a new L1 block in the database.
// it uses the latest derived L2 block as the derived block for the new L1 block.
func (db *ChainsDB) RecordNewL1(ref eth.BlockRef) error {
	for _, chain := range db.depSet.Chains() {
		// get local derivation database
		ldb, ok := db.localDBs.Get(chain)
		if !ok {
			return fmt.Errorf("cannot RecordNewL1 to chain %s: %w", chain, types.ErrUnknownChain)
		}
		// get the latest derived and derivedFrom blocks
		derivedFrom, derived, err := ldb.Latest()
		if err != nil {
			return fmt.Errorf("failed to get latest derivedFrom for chain %s: %w", chain, err)
		}
		// make a ref from the latest derived block
		derivedParent, err := ldb.PreviousDerived(derived.ID())
		if errors.Is(err, types.ErrFuture) {
			db.logger.Warn("Empty DB, Recording first L1 block", "chain", chain, "err", err)
		} else if err != nil {
			db.logger.Warn("Failed to get latest derivedfrom to insert new L1 block", "chain", chain, "err", err)
			return err
		}
		derivedRef := derived.MustWithParent(derivedParent.ID())
		// don't push the new L1 block if it's not newer than the latest derived block
		if derivedFrom.Number >= ref.Number {
			db.logger.Warn("L1 block has already been processed for this height", "chain", chain, "block", ref, "latest", derivedFrom)
			continue
		}
		// the database is extended with the new L1 and the existing L2
		if err = db.UpdateLocalSafe(chain, ref, derivedRef); err != nil {
			db.logger.Error("Failed to update local safe", "chain", chain, "block", ref, "derived", derived, "err", err)
			return err
		}
	}
	return nil
}
