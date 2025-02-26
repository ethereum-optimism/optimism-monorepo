package cross

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/log"
)

type UnsafeStartDeps interface {
	Contains(chain eth.ChainID, query types.ContainsQuery) (includedIn types.BlockSeal, err error)

	IsCrossUnsafe(chainID eth.ChainID, block eth.BlockID) error

	DependencySet() depset.DependencySet

	OpenBlock(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error)
}

// CrossUnsafeHazards checks if the given messages all exist and pass invariants.
// It returns a hazard-set: if any intra-block messaging happened,
// these hazard blocks have to be verified.
func CrossUnsafeHazards(d UnsafeStartDeps, logger log.Logger, chainID eth.ChainID,
	candidate types.BlockSeal) (*HazardSet, error) {
	unsafeDeps := &UnsafeHazardDeps{UnsafeStartDeps: d}
	return NewHazardSet(unsafeDeps, logger, chainID, candidate)
}

// UnsafeHazardDeps adapts UnsafeStartDeps to HazardDeps
type UnsafeHazardDeps struct {
	UnsafeStartDeps
}

// VerifyBlock implements HazardDeps by checking cross-unsafe status
func (d *UnsafeHazardDeps) IsCrossValidBlock(chainID eth.ChainID, block eth.BlockID) error {
	if err := d.IsCrossUnsafe(chainID, block); err != nil {
		return fmt.Errorf("block %s is not cross-unsafe: %w", block, err)
	}
	return nil
}
