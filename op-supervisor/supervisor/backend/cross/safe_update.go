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
	NextSource(chain eth.ChainID, source eth.BlockID) (after eth.BlockRef, err error)
	PreviousDerived(chain eth.ChainID, derived eth.BlockID) (prevDerived types.BlockSeal, err error)

	OpenBlock(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error)

	UpdateCrossSafe(chain eth.ChainID, l1View eth.BlockRef, lastCrossDerived eth.BlockRef) error

	// InvalidateLocalSafe is called when a local block cannot be upgraded to cross-safe, and has to be dropped.
	// This is called relative to what was determined based on the l1Scope.
	// It is called with the candidate, the block that will be invalidated.
	// The replacement of this candidate will effectively be "derived from"
	// the scope that the candidate block was invalidated at.
	InvalidateLocalSafe(chainID eth.ChainID, candidate types.DerivedBlockRefPair) error

	// FindDependentBlocks returns a list of blocks across all chains that depend on the given block
	// through executing messages referencing initiating messages in the invalidated block.
	FindDependentBlocks(chainID eth.ChainID, invalidatedBlock eth.BlockID) ([]types.DependentBlock, error)
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
		return InvalidateLocalSafeWithDependents(logger, chainID, d, candidate)
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
	return d.UpdateCrossSafe(chainID, newScope, crossSafeRef)
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

	execMsgSlice := sliceOfExecMsgs(execMsgs)

	hazards, err := CrossSafeHazards(d, chainID, candidate.Source.ID(), types.BlockSealFromRef(opened), execMsgSlice)
	if err != nil {
		return candidate, fmt.Errorf("failed to determine dependencies of cross-safe candidate %s: %w", candidate.Derived, err)
	}
	if err := HazardSafeFrontierChecks(d, candidate.Source.ID(), hazards); err != nil {
		return candidate, fmt.Errorf("failed to verify block %s in cross-safe frontier: %w", candidate.Derived, err)
	}
	if err := HazardCycleChecks(d.DependencySet(), d, candidate.Derived.Time, hazards); err != nil {
		return candidate, fmt.Errorf("failed to verify block %s in cross-safe check for cycle hazards: %w", candidate.Derived, err)
	}
	if err := ValidateCrossSafeDependencies(d, candidate.Source.ID(), hazards, execMsgSlice); err != nil {
		return candidate, fmt.Errorf("failed to verify block %s dependency cross validation: %w", candidate.Derived, err)
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

// ValidateCrossSafeDependencies verifies that all executing messages in the hazard blocks
// reference initiating messages that are either:
//   - already cross-safe
//   - part of the current hazard set
//
// It tracks dependencies between blocks in the hazard set and ensures they are all
// promoted to cross-safe together.
func ValidateCrossSafeDependencies(d SafeFrontierCheckDeps, inL1Source eth.BlockID, hazards map[types.ChainIndex]types.BlockSeal, execMsgs []*types.ExecutingMessage) error {
	depSet := d.DependencySet()
	// Track dependencies between blocks in the hazard set
	deferredPromotions := make(map[eth.BlockID][]eth.BlockID)

	// First pass: check all dependencies and build deferredPromotions map
	for _, msg := range execMsgs {
		// Get the chain ID for the initiating message
		initChainID, err := depSet.ChainIDFromIndex(msg.Chain)
		if err != nil {
			return fmt.Errorf("cannot verify dependency on unknown chain index %s: %w", msg.Chain, types.ErrConflict)
		}

		// Check if the initiating message's block is in the current hazard set
		initBlockInHazards := false
		var initBlockID eth.BlockID
		for chainIndex, hazardBlock := range hazards {
			if chainIndex == msg.Chain && hazardBlock.Number == msg.BlockNum {
				initBlockInHazards = true
				initBlockID = eth.BlockID{
					Hash:   hazardBlock.Hash,
					Number: hazardBlock.Number,
				}
				break
			}
		}

		// If not in hazards, the block must already be cross-safe
		if !initBlockInHazards {
			initBlockID = eth.BlockID{
				Hash:   msg.Hash,
				Number: msg.BlockNum,
			}
			// Check if the block is cross-safe and within the L1 scope
			source, err := d.CrossDerivedToSource(initChainID, initBlockID)
			if err != nil {
				// Block is not cross-safe and not in hazard set - must wait
				return fmt.Errorf("executing message depends on block %s (chain %s) that is not cross-safe: %w", initBlockID, initChainID, types.ErrFuture)
			}
			if source.Number > inL1Source.Number {
				return fmt.Errorf("executing message depends on block %s (chain %s) derived from L1 block %s that is after scope %s: %w",
					initBlockID, initChainID, source, inL1Source, types.ErrOutOfScope)
			}
			continue
		}

		// Block is in hazard set - track the dependency
		for chainIndex, hazardBlock := range hazards {
			if chainIndex == msg.Chain {
				continue // Skip self-dependencies
			}
			execBlockID := eth.BlockID{
				Hash:   hazardBlock.Hash,
				Number: hazardBlock.Number,
			}
			deferredPromotions[execBlockID] = append(deferredPromotions[execBlockID], initBlockID)
		}
	}

	// Second pass: verify all dependencies are ready for promotion
	for execBlockID, dependencies := range deferredPromotions {
		// Check if the executing block is in the hazard set
		execFound := false
		for _, hazardBlock := range hazards {
			if hazardBlock.Number == execBlockID.Number && hazardBlock.Hash == execBlockID.Hash {
				execFound = true
				break
			}
		}
		if !execFound {
			return fmt.Errorf("executing block %s not found in hazard set: %w", execBlockID, types.ErrConflict)
		}

		// Check if all dependencies are in the hazard set
		for _, depID := range dependencies {
			depFound := false
			for _, hazardBlock := range hazards {
				if hazardBlock.Number == depID.Number && hazardBlock.Hash == depID.Hash {
					depFound = true
					break
				}
			}
			if !depFound {
				return fmt.Errorf("deferred promotion dependency %s not found in hazard set: %w", depID, types.ErrConflict)
			}
		}
	}

	return nil
}

// InvalidateLocalSafeWithDependents invalidates a block and all blocks that depend on it through
// executing messages referencing initiating messages in the invalidated block.
func InvalidateLocalSafeWithDependents(logger log.Logger, chainID eth.ChainID, d CrossSafeDeps, candidate types.DerivedBlockRefPair) error {
	logger.Info("Starting cascading invalidation",
		"chain", chainID,
		"block", candidate.Derived,
		"source", candidate.Source)

	// First invalidate the target block
	if err := d.InvalidateLocalSafe(chainID, candidate); err != nil {
		logger.Error("Failed to invalidate block",
			"chain", chainID,
			"block", candidate.Derived,
			"err", err)
		return fmt.Errorf("failed to invalidate block %s: %w", candidate.Derived, err)
	}
	logger.Info("Successfully invalidated block",
		"chain", chainID,
		"block", candidate.Derived)

	// Find all blocks that depend on this block
	dependentBlocks, err := d.FindDependentBlocks(chainID, candidate.Derived.ID())
	if err != nil {
		logger.Error("Failed to find dependent blocks",
			"chain", chainID,
			"block", candidate.Derived,
			"err", err)
		return fmt.Errorf("failed to find dependent blocks of %s: %w", candidate.Derived, err)
	}
	logger.Info("Found dependent blocks",
		"dependent_count", len(dependentBlocks))

	// Recursively invalidate all dependent blocks
	for _, depBlock := range dependentBlocks {
		logger.Info("Invalidating dependent block",
			"dependent_chain", depBlock.ChainID,
			"dependent_block", depBlock.Block.Derived,
			"dependent_source", depBlock.Block.Source,
			"parent_chain", chainID,
			"parent_block", candidate.Derived)
		if err := InvalidateLocalSafeWithDependents(logger, depBlock.ChainID, d, depBlock.Block); err != nil {
			logger.Error("Failed to invalidate dependent block",
				"block", depBlock.Block.Derived,
				"source", depBlock.Block.Source,
				"parent_block", candidate.Derived,
				"chain", depBlock.ChainID,
				"err", err)
			return fmt.Errorf("failed to invalidate dependent block %s: %w", depBlock.Block.Derived, err)
		}
	}

	return nil
}
