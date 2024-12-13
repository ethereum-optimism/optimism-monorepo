package db

import (
	"fmt"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db/fromda"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db/logs"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

func OpenLogDB(logger log.Logger, chainID types.ChainID, dataDir string, m logs.Metrics) (*logs.DB, error) {
	path, err := prepLogDBPath(chainID, dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create datadir for chain %s: %w", chainID, err)
	}
	logDB, err := logs.NewFromFile(logger, m, path, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create logdb for chain %s at %v: %w", chainID, path, err)
	}
	return logDB, nil
}

func OpenLocalDerivedFromDB(logger log.Logger, chainID types.ChainID, dataDir string, m fromda.ChainMetrics) (*fromda.DB, error) {
	path, err := prepLocalDerivedFromDBPath(chainID, dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare datadir for chain %s: %w", chainID, err)
	}
	db, err := fromda.NewFromFile(logger, fromda.AdaptMetrics(m, "local_derived"), path)
	if err != nil {
		return nil, fmt.Errorf("failed to create local-derived for chain %s at %q: %w", chainID, path, err)
	}
	return db, nil
}

func OpenCrossDerivedFromDB(logger log.Logger, chainID types.ChainID, dataDir string, m fromda.ChainMetrics) (*fromda.DB, error) {
	path, err := prepCrossDerivedFromDBPath(chainID, dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare datadir for chain %s: %w", chainID, err)
	}
	db, err := fromda.NewFromFile(logger, fromda.AdaptMetrics(m, "cross_derived"), path)
	if err != nil {
		return nil, fmt.Errorf("failed to create cross-derived for chain %s at %q: %w", chainID, path, err)
	}
	return db, nil
}
