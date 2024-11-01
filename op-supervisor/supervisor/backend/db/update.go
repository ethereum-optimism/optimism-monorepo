package db

import (
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
	return localDB.AddDerived(derivedFrom, lastDerived)
}

func (db *ChainsDB) UpdateCrossUnsafe(chain types.ChainID, crossUnsafe types.BlockSeal) error {
	v, ok := db.crossUnsafe.Get(chain)
	if !ok {
		return fmt.Errorf("cannot UpdateCrossUnsafe: %w: %s", types.ErrUnknownChain, chain)
	}
	v.Set(crossUnsafe)
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
	return nil
}

func (db *ChainsDB) UpdateFinalizedL1(finalized eth.BlockRef) error {
	// Lock, so we avoid race-conditions in-between getting (for comparison) and setting.
	db.finalizedL1.Lock()
	defer db.finalizedL1.Unlock()

	if v := db.finalizedL1.Value; v.Number > finalized.Number {
		return fmt.Errorf("cannot rewind finalized L1 head from %s to %s", v, finalized)
	}
	db.finalizedL1.Value = finalized
	db.logger.Info("Updated finalized L1", "finalizedL1", finalized)
	return nil
}
