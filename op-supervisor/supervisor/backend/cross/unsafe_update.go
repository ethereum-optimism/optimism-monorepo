package cross

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type CrossUnsafeDeps interface {
	CrossUnsafe(chainID types.ChainID) (types.BlockSeal, error)

	UnsafeStartDeps
	UnsafeFrontierCheckDeps

	OpenBlock(chainID types.ChainID, blockNum uint64) (block eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error)

	UpdateCrossUnsafe(chain types.ChainID, crossUnsafe types.BlockSeal) error
}

func CrossUnsafeUpdate(ctx context.Context, logger log.Logger, chainID types.ChainID, d CrossUnsafeDeps) error {
	var candidate types.BlockSeal
	var execMsgs []*types.ExecutingMessage

	// fetch cross-head to determine next cross-unsafe candidate
	if crossUnsafe, err := d.CrossUnsafe(chainID); err != nil {
		if errors.Is(err, types.ErrFuture) {
			// If genesis / no cross-safe block yet, then defer update
			logger.Debug("No cross-unsafe starting point yet")
			return nil
		} else {
			return err
		}
	} else {
		// Open block N+1: this is a local-unsafe block,
		// just after cross-safe, that can be promoted if it passes the dependency checks.
		bl, _, msgs, err := d.OpenBlock(chainID, crossUnsafe.Number+1)
		if err != nil {
			return fmt.Errorf("failed to open block %d: %w", crossUnsafe.Number+1, err)
		}
		if bl.ParentHash != crossUnsafe.Hash {
			return fmt.Errorf("cannot use block %s, it does not build on cross-unsafe block %s: %w", bl, crossUnsafe, types.ErrConflict)
		}
		candidate = types.BlockSealFromRef(bl)
		execMsgs = sliceOfExecMsgs(msgs)
	}

	hazards, err := CrossUnsafeHazards(d, chainID, candidate, execMsgs)
	if err != nil {
		// TODO(#11693): reorgs can be detected by checking if the error is ErrConflict,
		// missing data is identified by ErrFuture,
		// and other errors (e.g. DB issues) are identifier by remaining error kinds.
		return fmt.Errorf("failed to check for cross-chain hazards: %w", err)
	}

	if err := HazardUnsafeFrontierChecks(d, hazards); err != nil {
		return fmt.Errorf("failed to verify block %s in cross-unsafe frontier: %w", candidate, err)
	}
	if err := HazardCycleChecks(d, candidate.Timestamp, hazards); err != nil {
		return fmt.Errorf("failed to verify block %s in cross-unsafe check for cycle hazards: %w", candidate, err)
	}

	// promote the candidate block to cross-unsafe
	if err := d.UpdateCrossUnsafe(chainID, candidate); err != nil {
		return fmt.Errorf("failed to update cross-unsafe head to %s: %w", candidate, err)
	}
	return nil
}

func NewCrossUnsafeWorker(logger log.Logger, chainID types.ChainID, d CrossUnsafeDeps) *Worker {
	logger = logger.New("chain", chainID)
	return NewWorker(logger, func(ctx context.Context) error {
		return CrossUnsafeUpdate(ctx, logger, chainID, d)
	})
}
