package dial

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/log"
)

type GetL1SyncStatus func(ctx context.Context) (eth.L1BlockRef, error)

func WaitRollupSync(
	ctx context.Context,
	lgr log.Logger,
	rollup SyncStatusProvider,
	l1BlockTarget uint64,
	pollInterval time.Duration,
) error {
	return WaitL1Sync(ctx, lgr, l1BlockTarget, pollInterval, func(ctx context.Context) (eth.L1BlockRef, error) {
		status, err := rollup.SyncStatus(ctx)
		if err != nil {
			return eth.L1BlockRef{}, err
		}
		return status.CurrentL1, nil
	})
}

func WaitL1Sync(
	ctx context.Context,
	lgr log.Logger,
	l1BlockTarget uint64,
	pollInterval time.Duration,
	getStatus GetL1SyncStatus,
) error {
	timer := time.NewTimer(pollInterval)
	defer timer.Stop()

	for {
		currentL1, err := getStatus(ctx)
		if err != nil {
			// don't log assuming caller handles and logs errors
			return fmt.Errorf("getting sync status: %w", err)
		}

		lgr := lgr.With("current_l1", currentL1, "target_l1", l1BlockTarget)
		if currentL1.Number >= l1BlockTarget {
			lgr.Info("rollup current L1 block target reached")
			return nil
		}

		lgr.Info("rollup current L1 block still behind target, retrying")

		timer.Reset(pollInterval)
		select {
		case <-timer.C: // next try
		case <-ctx.Done():
			lgr.Warn("waiting for rollup sync timed out")
			return ctx.Err()
		}
	}
}
