package db

import (
	"errors"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

// ForceInitialized marks the chain database as initialized, even if it is not.
// This function is for testing purposes only and should not be used in production code.
func (db *ChainsDB) ForceInitialized(id eth.ChainID) {
	db.initialized.Set(id, struct{}{})
}

func (db *ChainsDB) isInitialized(id eth.ChainID) bool {
	_, ok := db.initialized.Get(id)
	return ok
}

func (db *ChainsDB) initFromAnchor(id eth.ChainID, anchor types.DerivedBlockRefPair) {
	// Check if the chain database is already initialized
	if db.isInitialized(id) {
		db.logger.Debug("chain database already initialized")
		return
	}
	db.logger.Debug("initializing chain database from anchor point")

	// Initialize the local and cross safe databases
	if err := db.maybeInitSafeDB(id, anchor); err != nil {
		db.logger.Warn("failed to initialize local and cross safe databases", "err", err)
		return
	}

	// Initialize the events database
	if err := db.maybeInitEventsDB(id, anchor); err != nil {
		db.logger.Warn("failed to initialize events database", "err", err)
		return
	}

	// Mark the chain database as initialized
	db.initialized.Set(id, struct{}{})
}

// maybeInitSafeDB initializes the chain database if it is not already initialized
// it checks if the Local Safe database is empty, and loads both the Local and Cross Safe databases
// with the anchor point if they are empty.
func (db *ChainsDB) maybeInitSafeDB(id eth.ChainID, anchor types.DerivedBlockRefPair) error {
	logger := db.logger.New("chain", id, "derived", anchor.Derived, "source", anchor.Source)
	localDB, ok := db.localDBs.Get(id)
	if !ok {
		return types.ErrUnknownChain
	}
	first, err := localDB.First()
	if errors.Is(err, types.ErrFuture) {
		logger.Info("local database is empty, initializing")
		if err := db.initializedUpdateCrossSafe(id, anchor.Source, anchor.Derived); err != nil {
			return err
		}
		db.initializedUpdateLocalSafe(id, anchor.Source, anchor.Derived)
	} else if err != nil {
		return fmt.Errorf("failed to check if chain database is initialized: %w", err)
	} else {
		logger.Debug("chain database already initialized")
		if first.Derived.Hash != anchor.Derived.Hash ||
			first.Source.Hash != anchor.Source.Hash {
			return fmt.Errorf("local database (%s) does not match anchor point (%s): %w",
				first,
				anchor,
				types.ErrConflict)
		}
	}
	return nil
}

func (db *ChainsDB) maybeInitEventsDB(id eth.ChainID, anchor types.DerivedBlockRefPair) error {
	logger := db.logger.New("chain", id, "derived", anchor.Derived, "source", anchor.Source)
	seal, _, _, err := db.OpenBlock(id, 0)
	if errors.Is(err, types.ErrFuture) {
		logger.Debug("initializing events database")
		err := db.initializedSealBlock(id, anchor.Derived)
		if err != nil {
			return err
		}
		logger.Info("Initialized events database")
	} else if err != nil {
		return fmt.Errorf("failed to check if logDB is initialized: %w", err)
	} else {
		logger.Debug("Events database already initialized")
		if seal.Hash != anchor.Derived.Hash {
			return fmt.Errorf("events database (%s) does not match anchor point (%s): %w",
				seal,
				anchor,
				types.ErrConflict)
		}
	}
	return nil
}
