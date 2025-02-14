package super

import (
	"context"
	"errors"
	"testing"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/stretchr/testify/require"
)

func TestSyncStatusProvider(t *testing.T) {
	requestErr := errors.New("boom")
	tests := []struct {
		name          string
		gameL1Head    uint64
		syncStatus    eth.SupervisorSyncStatus
		statusReqErr  error
		expectedError error
	}{
		{
			name:          "ErrorFetchingStatus",
			gameL1Head:    100,
			statusReqErr:  requestErr,
			expectedError: requestErr,
		},
		{
			name:       "MinSyncedL1BehindGameHead",
			gameL1Head: 100,
			syncStatus: eth.SupervisorSyncStatus{
				MinSyncedL1: eth.L1BlockRef{Number: 99},
			},
			expectedError: types.ErrNotInSync,
		},
		{
			name:       "MinSyncedL1EqualToGameHead",
			gameL1Head: 100,
			syncStatus: eth.SupervisorSyncStatus{
				MinSyncedL1: eth.L1BlockRef{Number: 100},
			},
			expectedError: types.ErrNotInSync,
		},
		{
			name:       "InSync",
			gameL1Head: 100,
			syncStatus: eth.SupervisorSyncStatus{
				MinSyncedL1: eth.L1BlockRef{Number: 101},
			},
			expectedError: nil,
		},
	}

	for _, test := range tests {
		test := test // capture range variable
		t.Run(test.name, func(t *testing.T) {
			stubProvider := &stubSyncStatusProvider{
				status: test.syncStatus,
				err:    test.statusReqErr,
			}
			validator := NewSyncValidator(stubProvider)
			err := validator.ValidateNodeSynced(context.Background(), eth.BlockID{Number: test.gameL1Head})
			require.ErrorIs(t, err, test.expectedError, "expected error to be %v, got %v", test.expectedError, err)
		})
	}
}

type stubSyncStatusProvider struct {
	status eth.SupervisorSyncStatus
	err    error
}

func (f *stubSyncStatusProvider) SyncStatus(ctx context.Context) (eth.SupervisorSyncStatus, error) {
	return f.status, f.err
}
