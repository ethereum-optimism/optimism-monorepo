package cross

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/log"
)

type SafeStartDeps interface {
	Contains(chain eth.ChainID, query types.ContainsQuery) (includedIn types.BlockSeal, err error)

	CrossDerivedToSource(chainID eth.ChainID, derived eth.BlockID) (source types.BlockSeal, err error)

	DependencySet() depset.DependencySet

	OpenBlock(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error)
}

// CrossSafeHazards checks if the given messages all exist and pass invariants.
// It returns a hazard-set: if any intra-block messaging happened,
// these hazard blocks have to be verified.
func CrossSafeHazards(d SafeStartDeps, logger log.Logger, chainID eth.ChainID, inL1Source eth.BlockID, candidate types.BlockSeal) (*HazardSet, error) {
	safeDeps := &SafeHazardDeps{SafeStartDeps: d, inL1Source: inL1Source}
	return NewHazardSet(safeDeps, logger, chainID, candidate)
}

type SafeHazardDeps struct {
	SafeStartDeps
	inL1Source eth.BlockID
}

func (d *SafeHazardDeps) IsCrossValidBlock(chainID eth.ChainID, derived eth.BlockID) error {
	initSource, err := d.CrossDerivedToSource(chainID, derived)
	if err != nil {
		return fmt.Errorf("non-cross-safe block %s: %w", derived, err)
	}
	if initSource.Number > d.inL1Source.Number {
		return fmt.Errorf("block %s derived from %s which is not in cross-safe scope %s: %w",
			derived, initSource, d.inL1Source, types.ErrOutOfScope)
	}
	return nil
}
