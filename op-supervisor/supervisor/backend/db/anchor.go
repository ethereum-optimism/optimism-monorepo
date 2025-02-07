package db

import (
	"errors"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

// maybeInitSafeDB initializes the chain database if it is not already initialized
// it checks if the Local Safe database is empty, and loads it with the Anchor Point if so
func (db *ChainsDB) maybeInitSafeDB(id eth.ChainID, anchor types.DerivedBlockRefPair) {
	logger := db.logger.New("chain", id, "derived", anchor.Derived, "source", anchor.Source)
	_, err := db.LocalSafe(id)
	if errors.Is(err, types.ErrFuture) {
		logger.Info("initializing chain database")
		if err := db.UpdateCrossSafe(id, anchor.Source, anchor.Derived); err != nil {
			logger.Warn("failed to initialize cross safe", "err", err)
		}
		db.UpdateLocalSafe(id, anchor.Source, anchor.Derived)
	} else if err != nil {
		logger.Warn("failed to check if chain database is initialized", "err", err)
	} else {
		logger.Debug("chain database already initialized")
	}
}

func (db *ChainsDB) maybeInitEventsDB(id eth.ChainID, anchor types.DerivedBlockRefPair) {
	logger := db.logger.New("chain", id, "derived", anchor.Derived, "source", anchor.Source)
	_, _, _, err := db.OpenBlock(id, 0)
	if errors.Is(err, types.ErrFuture) {
		logger.Debug("initializing events database")
		err := db.SealBlock(id, anchor.Derived)
		if err != nil {
			logger.Warn("failed to seal initial block", "err", err)
		}
		logger.Info("Initialized events database")
	} else if err != nil {
		logger.Warn("Failed to check if logDB is initialized", "err", err)
	} else {
		logger.Debug("Events database already initialized")
	}
}
