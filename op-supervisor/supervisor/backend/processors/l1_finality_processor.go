package processors

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/log"
)

type chainsDB interface {
	FinalizedL1() eth.BlockRef
	UpdateFinalizedL1(finalized eth.BlockRef) error
}

// MaybeUpdateFinalizedL1Fn returns a HeadSignalFn that updates the database with the new finalized block if it is newer than the current one.
func MaybeUpdateFinalizedL1Fn(ctx context.Context, logger log.Logger, db chainsDB) eth.HeadSignalFn {
	return func(ctx context.Context, ref eth.L1BlockRef) {
		// do something with the new block
		logger.Debug("Received new Finalized L1 block", "block", ref)
		currentFinalized := db.FinalizedL1()
		if currentFinalized.Number > ref.Number {
			logger.Warn("Finalized block in database is newer than subscribed finalized block", "current", currentFinalized, "new", ref)
			return
		}
		if ref.Number > currentFinalized.Number || currentFinalized == (eth.BlockRef{}) {
			// update the database with the new finalized block
			if err := db.UpdateFinalizedL1(ref); err != nil {
				logger.Warn("Failed to update finalized L1", "err", err)
				return
			}
			logger.Debug("Updated finalized L1 block", "block", ref)
		}
	}
}
