package cross

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/superevents"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type CrossSafeDeps interface {
	CrossSafe(chainID eth.ChainID) (pair types.DerivedBlockSealPair, err error)

	SafeFrontierCheckDeps
	SafeStartDeps

	CandidateCrossSafe(chain eth.ChainID) (candidate types.DerivedBlockRefPair, err error)
	NextSource(chain eth.ChainID, derivedFrom eth.BlockID) (after eth.BlockRef, err error)
	PreviousDerived(chain eth.ChainID, derived eth.BlockID) (prevDerived types.BlockSeal, err error)

	OpenBlock(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error)

	UpdateCrossSafe(chain eth.ChainID, l1View eth.BlockRef, lastCrossDerived eth.BlockRef) error

	// InvalidateLocalSafe is called when a local block cannot be upgraded to cross-safe, and has to be dropped.
	// This is called relative to what was determined based on the l1Scope.
	// It is called with the candidate, the block that will be invalidated.
	// The replacement of this candidate will effectively be "derived from"
	// the scope that the candidate block was invalidated at.
	InvalidateLocalSafe(chainID eth.ChainID, candidate types.DerivedBlockRefPair) error
}

func CrossSafeUpdate(logger log.Logger, chainID eth.ChainID, d CrossSafeDeps) error {
	logger.Debug("Cross-safe update call")
	// TODO(#11693): establish L1 reorg-lock of scopeDerivedFrom
	// defer unlock once we are done checking the chain
	candidate, err := scopedCrossSafeUpdate(logger, chainID, d)
	if err == nil {
		// if we made progress, and no errors, then there is no need to bump the L1 scope yet.
		return nil
	}
	if errors.Is(err, types.ErrAwaitReplacementBlock) {
		logger.Info("Awaiting replacement block", "err", err)
		return err
	}
	if errors.Is(err, types.ErrConflict) {
		logger.Warn("Found a conflicting local-safe block that cannot be promoted to cross-safe",
			"scope", candidate.Source, "invalidated", candidate, "err", err)
		return d.InvalidateLocalSafe(chainID, candidate)
	}
	if !errors.Is(err, types.ErrOutOfScope) {
		return fmt.Errorf("failed to determine cross-safe update scope of chain %s: %w", chainID, err)
	}
	// candidate scope is expected to be set if ErrOutOfScope is returned.
	if candidate.Source == (eth.BlockRef{}) {
		return fmt.Errorf("expected L1 scope to be defined with ErrOutOfScope: %w", err)
	}
	logger.Debug("Cross-safe updating ran out of L1 scope", "scope", candidate.Source, "err", err)
	// bump the L1 scope up, and repeat the prev L2 block, not the candidate
	newScope, err := d.NextSource(chainID, candidate.Source.ID())
	if err != nil {
		return fmt.Errorf("failed to identify new L1 scope to expand to after %s: %w", candidate.Source, err)
	}
	currentCrossSafe, err := d.CrossSafe(chainID)
	if err != nil {
		// TODO: if genesis isn't cross-safe by default, then we can't register something as cross-safe here
		return fmt.Errorf("failed to identify cross-safe scope to repeat: %w", err)
	}
	parent, err := d.PreviousDerived(chainID, currentCrossSafe.Derived.ID())
	if err != nil {
		return fmt.Errorf("cannot find parent-block of cross-safe: %w", err)
	}
	crossSafeRef := currentCrossSafe.Derived.MustWithParent(parent.ID())
	logger.Debug("Bumping cross-safe scope", "scope", newScope, "crossSafe", crossSafeRef)
	if err := d.UpdateCrossSafe(chainID, newScope, crossSafeRef); err != nil {
		return fmt.Errorf("failed to update cross-safe head with L1 scope increment to %s and repeat of L2 block %s: %w", candidate.Source, crossSafeRef, err)
	}
	return nil
}

// scopedCrossSafeUpdate runs through the cross-safe update checks.
// If no L2 cross-safe progress can be made without additional L1 input data,
// then a types.ErrOutOfScope error is returned,
// with the current scope that will need to be expanded for further progress.
func scopedCrossSafeUpdate(logger log.Logger, chainID eth.ChainID, d CrossSafeDeps) (update types.DerivedBlockRefPair, err error) {
	candidate, err := d.CandidateCrossSafe(chainID)
	if err != nil {
		return candidate, fmt.Errorf("failed to determine candidate block for cross-safe: %w", err)
	}
	logger.Debug("Candidate cross-safe", "scope", candidate.Source, "candidate", candidate.Derived)
	opened, _, execMsgs, err := d.OpenBlock(chainID, candidate.Derived.Number)
	if err != nil {
		return candidate, fmt.Errorf("failed to open block %s: %w", candidate.Derived, err)
	}
	if opened.ID() != candidate.Derived.ID() {
		return candidate, fmt.Errorf("unsafe L2 DB has %s, but candidate cross-safe was %s: %w", opened, candidate.Derived, types.ErrConflict)
	}
	hazards, err := CrossSafeHazards(d, chainID, candidate.Source.ID(), types.BlockSealFromRef(opened), sliceOfExecMsgs(execMsgs))
	if err != nil {
		return candidate, fmt.Errorf("failed to determine dependencies of cross-safe candidate %s: %w", candidate.Derived, err)
	}
	if err := HazardSafeFrontierChecks(d, candidate.Source.ID(), hazards); err != nil {
		return candidate, fmt.Errorf("failed to verify block %s in cross-safe frontier: %w", candidate.Derived, err)
	}
	if err := HazardCycleChecks(d.DependencySet(), d, candidate.Derived.Time, hazards); err != nil {
		return candidate, fmt.Errorf("failed to verify block %s in cross-safe check for cycle hazards: %w", candidate, err)
	}

	// promote the candidate block to cross-safe
	if err := d.UpdateCrossSafe(chainID, candidate.Source, candidate.Derived); err != nil {
		return candidate, fmt.Errorf("failed to update cross-safe head to %s, derived from scope %s: %w", candidate.Derived, candidate.Source, err)
	}
	return candidate, nil
}

func sliceOfExecMsgs(execMsgs map[uint32]*types.ExecutingMessage) []*types.ExecutingMessage {
	msgs := make([]*types.ExecutingMessage, 0, len(execMsgs))
	for _, msg := range execMsgs {
		msgs = append(msgs, msg)
	}
	return msgs
}

type CrossSafeWorker struct {
	logger  log.Logger
	chainID eth.ChainID
	d       CrossSafeDeps
}

func (c *CrossSafeWorker) OnEvent(ev event.Event) bool {
	switch ev.(type) {
	case superevents.UpdateCrossSafeRequestEvent:
		if err := CrossSafeUpdate(c.logger, c.chainID, c.d); err != nil {
			if errors.Is(err, types.ErrFuture) {
				c.logger.Debug("Worker awaits additional blocks", "err", err)
			} else {
				c.logger.Warn("Failed to process work", "err", err)
			}
		}
	default:
		return false
	}
	return true
}

var _ event.Deriver = (*CrossUnsafeWorker)(nil)

func NewCrossSafeWorker(logger log.Logger, chainID eth.ChainID, d CrossSafeDeps) *CrossSafeWorker {
	logger = logger.New("chain", chainID)
	return &CrossSafeWorker{
		logger:  logger,
		chainID: chainID,
		d:       d,
	}
}
