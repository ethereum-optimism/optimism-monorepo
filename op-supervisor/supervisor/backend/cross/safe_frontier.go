package cross

import (
	"errors"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type SafeFrontierCheckDeps interface {
	CandidateCrossSafe(chain types.ChainID) (derivedFromScope, crossSafe eth.BlockRef, err error)

	CrossDerivedFrom(chainID types.ChainID, derived eth.BlockID) (derivedFrom types.BlockSeal, err error)

	DependencySet() depset.DependencySet
}

// HazardSafeFrontierChecks verifies all the hazard blocks are either:
//   - already cross-safe.
//   - the first (if not first: local blocks to verify before proceeding)
//     local-safe block, after the cross-safe block.
func HazardSafeFrontierChecks(d SafeFrontierCheckDeps, inL1DerivedFrom eth.BlockID, hazards map[types.ChainIndex]types.BlockSeal) error {
	depSet := d.DependencySet()
	for hazardChainIndex, hazardBlock := range hazards {
		hazardChainID, err := depSet.ChainIDFromIndex(hazardChainIndex)
		if err != nil {
			if errors.Is(err, types.ErrUnknownChain) {
				err = fmt.Errorf("cannot cross-safe verify block %s of unknown chain index %s: %w", hazardBlock, hazardChainIndex, types.ErrConflict)
			}
			return err
		}
		initDerivedFrom, err := d.CrossDerivedFrom(hazardChainID, hazardBlock.ID())
		if err != nil {
			if errors.Is(err, types.ErrFuture) {
				// If not in cross-safe scope, then check if it's the candidate cross-safe block.
				initDerivedFrom, initSelf, err := d.CandidateCrossSafe(hazardChainID)
				if err != nil {
					return fmt.Errorf("failed to determine cross-safe candidate block of hazard dependency %s (chain %s): %w", hazardBlock, hazardChainID, err)
				}
				if initSelf.Number == hazardBlock.Number && initSelf.ID() != hazardBlock.ID() {
					return fmt.Errorf("expected block %s (chain %d) does not match candidate local-safe block %s: %w",
						hazardBlock, hazardChainID, initSelf, types.ErrConflict)
				}
				if initDerivedFrom.Number > inL1DerivedFrom.Number {
					return fmt.Errorf("local-safe hazard block %s derived from L1 block %s is after scope %s: %w",
						hazardBlock.ID(), initDerivedFrom, inL1DerivedFrom, types.ErrOutOfScope)
				}
			} else {
				return fmt.Errorf("failed to determine cross-derived of hazard block %s (chain %s): %w", hazardBlock, hazardChainID, err)
			}
		} else if initDerivedFrom.Number > inL1DerivedFrom.Number {
			return fmt.Errorf("cross-safe hazard block %s derived from L1 block %s is after scope %s: %w",
				hazardBlock.ID(), initDerivedFrom, inL1DerivedFrom, types.ErrOutOfScope)
		}
	}
	return nil
}
