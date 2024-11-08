package cross

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type CrossSafeDeps interface {
	CrossSafe(chainID types.ChainID) (derivedFrom types.BlockSeal, derived types.BlockSeal, err error)

	SafeFrontierCheckDeps
	SafeStartDeps

	CandidateCrossSafe(chain types.ChainID) (derivedFromScope, crossSafe eth.BlockRef, err error)
	NextDerivedFrom(chain types.ChainID, derivedFrom eth.BlockID) (after eth.BlockRef, err error)
	PreviousDerived(chain types.ChainID, derived eth.BlockID) (prevDerived types.BlockSeal, err error)

	OpenBlock(chainID types.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error)

	UpdateCrossSafe(chain types.ChainID, l1View eth.BlockRef, lastCrossDerived eth.BlockRef) error
}

func CrossSafeUpdate(ctx context.Context, logger log.Logger, chainID types.ChainID, d CrossSafeDeps) error {
	logger.Debug("Cross-safe update call")
	// TODO(#11693): establish L1 reorg-lock of scopeDerivedFrom
	// defer unlock once we are done checking the chain
	candidateScope, err := scopedCrossSafeUpdate(logger, chainID, d)
	if err == nil {
		// if we made progress, and no errors, then there is no need to bump the L1 scope yet.
		return nil
	}
	if !errors.Is(err, types.ErrOutOfScope) {
		return err
	}
	// candidateScope is expected to be set if ErrOutOfScope is returned.
	if candidateScope == (eth.BlockRef{}) {
		return fmt.Errorf("expected L1 scope to be defined with ErrOutOfScope: %w", err)
	}
	logger.Debug("Cross-safe updating ran out of L1 scope", "scope", candidateScope, "err", err)
	// bump the L1 scope up, and repeat the prev L2 block, not the candidate
	newScope, err := d.NextDerivedFrom(chainID, candidateScope.ID())
	if err != nil {
		return fmt.Errorf("failed to identify new L1 scope to expand to after %s: %w", candidateScope, err)
	}
	_, currentCrossSafe, err := d.CrossSafe(chainID)
	if err != nil {
		// TODO: if genesis isn't cross-safe by default, then we can't register something as cross-safe here
		return fmt.Errorf("failed to identify cross-safe scope to repeat: %w", err)
	}
	parent, err := d.PreviousDerived(chainID, currentCrossSafe.ID())
	if err != nil {
		return fmt.Errorf("cannot find parent-block of cross-safe: %w", err)
	}
	crossSafeRef := currentCrossSafe.MustWithParent(parent.ID())
	logger.Debug("Bumping cross-safe scope", "scope", newScope, "crossSafe", crossSafeRef)
	if err := d.UpdateCrossSafe(chainID, newScope, crossSafeRef); err != nil {
		return fmt.Errorf("failed to update cross-safe head with L1 scope increment to %s and repeat of L2 block %s: %w", candidateScope, crossSafeRef, err)
	}
	return nil
}

// scopedCrossSafeUpdate runs through the cross-safe update checks.
// If no L2 cross-safe progress can be made without additional L1 input data,
// then a types.ErrOutOfScope error is returned,
// with the current scope that will need to be expanded for further progress.
func scopedCrossSafeUpdate(logger log.Logger, chainID types.ChainID, d CrossSafeDeps) (scope eth.BlockRef, err error) {
	candidateScope, candidate, err := d.CandidateCrossSafe(chainID)
	if err != nil {
		return candidateScope, fmt.Errorf("failed to determine candidate block for cross-safe: %w", err)
	}
	logger.Debug("Candidate cross-safe", "scope", candidateScope, "candidate", candidate)
	opened, _, execMsgs, err := d.OpenBlock(chainID, candidate.Number)
	if err != nil {
		return candidateScope, fmt.Errorf("failed to open block %s: %w", candidate, err)
	}
	if opened.ID() != candidate.ID() {
		return candidateScope, fmt.Errorf("unsafe L2 DB has %s, but candidate cross-safe was %s: %w", opened, candidate, types.ErrConflict)
	}
	hazards, err := CrossSafeHazards(d, chainID, candidateScope.ID(), types.BlockSealFromRef(opened), sliceOfExecMsgs(execMsgs))
	if err != nil {
		return candidateScope, fmt.Errorf("failed to determine dependencies of cross-safe candidate %s: %w", candidate, err)
	}
	if err := HazardSafeFrontierChecks(d, candidateScope.ID(), hazards); err != nil {
		return candidateScope, fmt.Errorf("failed to verify block %s in cross-safe frontier: %w", candidate, err)
	}
	if err := HazardCycleChecks(d, candidate.Time, hazards); err != nil {
		return candidateScope, fmt.Errorf("failed to verify block %s in cross-safe check for cycle hazards: %w", candidate, err)
	}

	// promote the candidate block to cross-safe
	if err := d.UpdateCrossSafe(chainID, candidateScope, candidate); err != nil {
		return candidateScope, fmt.Errorf("failed to update cross-safe head to %s, derived from scope %s: %w", candidate, candidateScope, err)
	}
	return candidateScope, nil
}

func NewCrossSafeWorker(logger log.Logger, chainID types.ChainID, d CrossSafeDeps) *Worker {
	logger = logger.New("chain", chainID)
	return NewWorker(logger, func(ctx context.Context) error {
		return CrossSafeUpdate(ctx, logger, chainID, d)
	})
}

func sliceOfExecMsgs(execMsgs map[uint32]*types.ExecutingMessage) []*types.ExecutingMessage {
	msgs := make([]*types.ExecutingMessage, 0, len(execMsgs))
	for _, msg := range execMsgs {
		msgs = append(msgs, msg)
	}
	return msgs
}
