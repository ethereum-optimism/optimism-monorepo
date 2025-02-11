package fault

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
		name         string
		gameL1Head   eth.BlockID
		syncStatus   *eth.SyncStatus
		statusReqErr error
		expected     error
	}{
		{
			name:         "ErrorFetchingStatus",
			gameL1Head:   eth.BlockID{Number: 100},
			syncStatus:   nil,
			statusReqErr: requestErr,
			expected:     requestErr,
		},
		{
			name:       "CurrentL1BelowGameL1Head",
			gameL1Head: eth.BlockID{Number: 100},
			syncStatus: &eth.SyncStatus{
				CurrentL1: eth.L1BlockRef{
					Number: 99,
				},
			},
			statusReqErr: nil,
			expected:     types.ErrNotInSync,
		},
		{
			name:       "CurrentL1EqualToGameL1Head",
			gameL1Head: eth.BlockID{Number: 100},
			syncStatus: &eth.SyncStatus{
				CurrentL1: eth.L1BlockRef{
					Number: 100,
				},
			},
			statusReqErr: nil,
			expected:     types.ErrNotInSync,
		},
		{
			name:       "CurrentL1AboveGameL1Head",
			gameL1Head: eth.BlockID{Number: 100},
			syncStatus: &eth.SyncStatus{
				CurrentL1: eth.L1BlockRef{
					Number: 101,
				},
			},
			statusReqErr: nil,
			expected:     nil,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			provider := &stubSyncStatusProvider{
				status: test.syncStatus,
				err:    test.statusReqErr,
			}
			validator := newSyncStatusValidator(provider)
			err := validator.ValidateNodeSynced(context.Background(), test.gameL1Head)
			require.ErrorIs(t, err, test.expected)
		})
	}
}

type stubSyncStatusProvider struct {
	status *eth.SyncStatus
	err    error
}

func (s *stubSyncStatusProvider) SyncStatus(_ context.Context) (*eth.SyncStatus, error) {
	return s.status, s.err
}
