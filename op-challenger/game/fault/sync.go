package fault

import (
	"context"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

type SyncStatusProvider interface {
	SyncStatus(context.Context) (*eth.SyncStatus, error)
}

type syncStatusValidator struct {
	statusProvider SyncStatusProvider
}

func newSyncStatusValidator(statusProvider SyncStatusProvider) *syncStatusValidator {
	return &syncStatusValidator{
		statusProvider: statusProvider,
	}
}

func (s *syncStatusValidator) ValidateNodeSynced(ctx context.Context, gameL1Head eth.BlockID) error {
	syncStatus, err := s.statusProvider.SyncStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve local node sync status: %w", err)
	}
	if syncStatus.CurrentL1.Number <= gameL1Head.Number {
		return fmt.Errorf("%w require L1 block above %v but at %v", types.ErrNotInSync, gameL1Head.Number, syncStatus.CurrentL1.Number)
	}
	return nil
}
