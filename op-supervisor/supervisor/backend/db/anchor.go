package db

import (
	"errors"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

// maybeInitSafeDB initializes the chain database if it is not already initialized
// it checks if the Local Safe database is empty, and loads it with the Anchor Point if so
func (db *ChainsDB) maybeInitSafeDB(id eth.ChainID, anchor types.DerivedBlockRefPair) {
	_, err := db.LocalSafe(id)
	if errors.Is(err, types.ErrFuture) {
		db.logger.Debug("initializing chain database", "chain", id)
		if err := db.UpdateCrossSafe(id, anchor.DerivedFrom, anchor.Derived); err != nil {
			db.logger.Warn("failed to initialize cross safe", "chain", id, "error", err)
		}
		db.UpdateLocalSafe(id, anchor.DerivedFrom, anchor.Derived)
	} else if err != nil {
		db.logger.Warn("failed to check if chain database is initialized", "chain", id, "error", err)
	} else {
		db.logger.Debug("chain database already initialized", "chain", id)
	}
}

func (db *ChainsDB) maybeInitEventsDB(id eth.ChainID, anchor types.DerivedBlockRefPair) {
	_, _, _, err := db.OpenBlock(id, 0)
	if errors.Is(err, types.ErrFuture) {
		db.logger.Debug("initializing events database", "chain", id)
		err := db.SealBlock(id, anchor.Derived)
		if err != nil {
			db.logger.Warn("failed to seal initial block", "chain", id, "error", err)
		}
		db.logger.Debug("initialized events database", "chain", id)
	} else if err != nil {
		db.logger.Warn("failed to check if logDB is initialized", "chain", id, "error", err)
	} else {
		db.logger.Debug("events database already initialized", "chain", id)
	}
}
