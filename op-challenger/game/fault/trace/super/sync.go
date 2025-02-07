package super

import (
	"context"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

type SyncValidator struct {
	syncStatusProvider SyncStatusProvider
}

type SyncStatusProvider interface {
	SyncStatus(ctx context.Context) (eth.SupervisorSyncStatus, error)
}

func NewSyncValidator(syncStatusProvider SyncStatusProvider) *SyncValidator {
	return &SyncValidator{
		syncStatusProvider: syncStatusProvider,
	}
}

func (s SyncValidator) ValidateNodeSynced(ctx context.Context, gameL1Head eth.BlockID) error {
	syncStatus, err := s.syncStatusProvider.SyncStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve sync status: %w", err)
	}
	if syncStatus.MinSyncedL1.Number <= gameL1Head.Number {
		return fmt.Errorf("%w require L1 block above %v but at %v", types.ErrNotInSync, gameL1Head.Number, syncStatus.MinSyncedL1.Number)
	}
	return nil
}
