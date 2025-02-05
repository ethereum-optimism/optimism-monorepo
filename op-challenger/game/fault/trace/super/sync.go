package super

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-service/eth"
)

type SyncValidator struct {
}

func NewSyncValidator() *SyncValidator {
	return &SyncValidator{}
}

func (s SyncValidator) ValidateNodeSynced(ctx context.Context, gameL1Head eth.BlockID) error {
	// TODO: Check sync status of supervisor
	return nil
}
