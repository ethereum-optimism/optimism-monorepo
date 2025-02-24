package source

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

func TestSupervisorSource_SyncStatus(t *testing.T) {
	t.Run("Single-Success", func(t *testing.T) {
		response := eth.SupervisorSyncStatus{
			MinSyncedL1: eth.L1BlockRef{
				Hash:       common.Hash{0xbc},
				Number:     48292,
				ParentHash: common.Hash{0xdd},
				Time:       98599217,
			},
			SafeTimestamp:      1234,
			FinalizedTimestamp: 45523,
		}
		client := &mockSupervisorClient{
			status: response,
		}
		source := NewSupervisorProposalSource(testlog.Logger(t, log.LvlInfo), client)
		actual, err := source.SyncStatus(context.Background())
		require.NoError(t, err)
		expected := SyncStatus{
			CurrentL1:   response.MinSyncedL1,
			SafeL2:      response.SafeTimestamp,
			FinalizedL2: response.FinalizedTimestamp,
		}
		require.Equal(t, expected, actual)
	})

	t.Run("Single-Error", func(t *testing.T) {
		expected := errors.New("test error")
		client := &mockSupervisorClient{
			statusErr: expected,
		}
		source := NewSupervisorProposalSource(testlog.Logger(t, log.LvlInfo), client)
		_, err := source.SyncStatus(context.Background())
		require.ErrorIs(t, err, expected)
	})

	t.Run("Multi-ReturnLowestMinSyncedL1", func(t *testing.T) {
		response1 := eth.SupervisorSyncStatus{
			MinSyncedL1: eth.L1BlockRef{
				Hash:       common.Hash{0xdd},
				Number:     9999999999,
				ParentHash: common.Hash{0xee},
				Time:       4,
			},
			SafeTimestamp:      2,
			FinalizedTimestamp: 4,
		}
		response2 := eth.SupervisorSyncStatus{
			MinSyncedL1: eth.L1BlockRef{
				Hash:       common.Hash{0xbc},
				Number:     48292,
				ParentHash: common.Hash{0xdd},
				Time:       98599217,
			},
			SafeTimestamp:      1234,
			FinalizedTimestamp: 45523,
		}
		client1 := &mockSupervisorClient{
			status: response1,
		}
		client2 := &mockSupervisorClient{
			status: response2,
		}
		source := NewSupervisorProposalSource(testlog.Logger(t, log.LvlInfo), client1, client2)
		actual, err := source.SyncStatus(context.Background())
		require.NoError(t, err)
		// Should use the response with the lowest MinSyncedL1 block number
		expected := SyncStatus{
			CurrentL1:   response2.MinSyncedL1,
			SafeL2:      response2.SafeTimestamp,
			FinalizedL2: response2.FinalizedTimestamp,
		}
		require.Equal(t, expected, actual)
	})

	t.Run("Multi-SkipFailingClients", func(t *testing.T) {
		response := eth.SupervisorSyncStatus{
			MinSyncedL1: eth.L1BlockRef{
				Hash:       common.Hash{0xdd},
				Number:     9999999999,
				ParentHash: common.Hash{0xee},
				Time:       4,
			},
			SafeTimestamp:      2,
			FinalizedTimestamp: 4,
		}
		client1 := &mockSupervisorClient{
			statusErr: errors.New("test error"),
		}
		client2 := &mockSupervisorClient{
			status: response,
		}
		logger, logs := testlog.CaptureLogger(t, log.LvlInfo)
		source := NewSupervisorProposalSource(logger, client1, client2)
		actual, err := source.SyncStatus(context.Background())
		require.NoError(t, err)
		// Should use the one successful response
		expected := SyncStatus{
			CurrentL1:   response.MinSyncedL1,
			SafeL2:      response.SafeTimestamp,
			FinalizedL2: response.FinalizedTimestamp,
		}
		require.Equal(t, expected, actual)
		require.NotNil(t, logs.FindLog(
			testlog.NewLevelFilter(slog.LevelWarn),
			testlog.NewAttributesFilter("err", client1.statusErr.Error())))
	})

	t.Run("Multi-AllFailingClients", func(t *testing.T) {
		client1 := &mockSupervisorClient{
			statusErr: errors.New("test error1"),
		}
		client2 := &mockSupervisorClient{
			statusErr: errors.New("test error2"),
		}
		logger, logs := testlog.CaptureLogger(t, log.LvlInfo)
		source := NewSupervisorProposalSource(logger, client1, client2)
		_, err := source.SyncStatus(context.Background())
		// Should return both errors
		require.ErrorIs(t, err, client1.statusErr)
		require.ErrorIs(t, err, client2.statusErr)
		require.NotNil(t, logs.FindLog(
			testlog.NewLevelFilter(slog.LevelWarn),
			testlog.NewAttributesFilter("err", client1.statusErr.Error())))
		require.NotNil(t, logs.FindLog(
			testlog.NewLevelFilter(slog.LevelWarn),
			testlog.NewAttributesFilter("err", client2.statusErr.Error())))
	})
}

func TestSupervisorSource_ProposalAtSequenceNum(t *testing.T) {
	response := eth.SuperRootResponse{
		CrossSafeDerivedFrom: eth.BlockID{
			Hash:   common.Hash{0x11},
			Number: 589111,
		},
		Timestamp: 59298244,
		SuperRoot: eth.Bytes32{0xaa, 0xbb},
		Version:   3,
		Chains:    nil,
	}
	expected := Proposal{
		Version:     eth.Bytes32{3},
		Root:        common.Hash(response.SuperRoot),
		SequenceNum: response.Timestamp,
		CurrentL1:   response.CrossSafeDerivedFrom,
		Legacy:      LegacyProposalData{},
	}
	sequenceNum := uint64(599)
	t.Run("Single-Success", func(t *testing.T) {
		client := &mockSupervisorClient{
			roots: map[uint64]eth.SuperRootResponse{
				sequenceNum: response,
			},
		}
		source := NewSupervisorProposalSource(testlog.Logger(t, log.LvlInfo), client)
		actual, err := source.ProposalAtSequenceNum(context.Background(), sequenceNum)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})

	t.Run("Single-Error", func(t *testing.T) {
		expected := errors.New("test error")
		client := &mockSupervisorClient{
			rootErr: expected,
		}
		source := NewSupervisorProposalSource(testlog.Logger(t, log.LvlInfo), client)
		_, err := source.ProposalAtSequenceNum(context.Background(), 294)
		require.ErrorIs(t, err, expected)
	})

	t.Run("Multi-FirstSourceSuccess", func(t *testing.T) {
		client1 := &mockSupervisorClient{
			roots: map[uint64]eth.SuperRootResponse{
				sequenceNum: response,
			},
		}
		client2 := &mockSupervisorClient{}
		source := NewSupervisorProposalSource(testlog.Logger(t, log.LvlInfo), client1, client2)
		actual, err := source.ProposalAtSequenceNum(context.Background(), sequenceNum)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
		require.Equal(t, 1, client1.rootRequestCount)
		require.Equal(t, 0, client2.rootRequestCount)
	})

	t.Run("Multi-FailOverToSecondSource", func(t *testing.T) {
		client1 := &mockSupervisorClient{
			rootErr: errors.New("test error"),
		}
		client2 := &mockSupervisorClient{
			roots: map[uint64]eth.SuperRootResponse{
				sequenceNum: response,
			},
		}
		logger, logs := testlog.CaptureLogger(t, log.LvlInfo)
		source := NewSupervisorProposalSource(logger, client1, client2)
		actual, err := source.ProposalAtSequenceNum(context.Background(), sequenceNum)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
		require.Equal(t, 1, client1.rootRequestCount)
		require.Equal(t, 1, client2.rootRequestCount)
		require.NotNil(t, logs.FindLog(testlog.NewLevelFilter(slog.LevelWarn), testlog.NewAttributesFilter("err", client1.rootErr.Error())))
	})

	t.Run("Multi-AllFail", func(t *testing.T) {
		client1 := &mockSupervisorClient{
			rootErr: errors.New("test error1"),
		}
		client2 := &mockSupervisorClient{
			rootErr: errors.New("test error2"),
		}
		logger, logs := testlog.CaptureLogger(t, log.LvlInfo)
		source := NewSupervisorProposalSource(logger, client1, client2)
		_, err := source.ProposalAtSequenceNum(context.Background(), sequenceNum)
		// Should provide all errors
		require.ErrorIs(t, err, client1.rootErr)
		require.ErrorIs(t, err, client2.rootErr)
		require.Equal(t, 1, client1.rootRequestCount)
		require.Equal(t, 1, client2.rootRequestCount)
		require.NotNil(t, logs.FindLog(testlog.NewLevelFilter(slog.LevelWarn), testlog.NewAttributesFilter("err", client1.rootErr.Error())))
		require.NotNil(t, logs.FindLog(testlog.NewLevelFilter(slog.LevelWarn), testlog.NewAttributesFilter("err", client2.rootErr.Error())))
	})
}

type mockSupervisorClient struct {
	status    eth.SupervisorSyncStatus
	statusErr error

	roots            map[uint64]eth.SuperRootResponse
	rootErr          error
	rootRequestCount int
}

func (m *mockSupervisorClient) SyncStatus(ctx context.Context) (eth.SupervisorSyncStatus, error) {
	if m.statusErr != nil {
		return eth.SupervisorSyncStatus{}, m.statusErr
	}
	return m.status, nil
}

func (m *mockSupervisorClient) SuperRootAtTimestamp(ctx context.Context, timestamp hexutil.Uint64) (eth.SuperRootResponse, error) {
	m.rootRequestCount++
	if m.rootErr != nil {
		return eth.SuperRootResponse{}, m.rootErr
	}
	root, ok := m.roots[uint64(timestamp)]
	if !ok {
		return eth.SuperRootResponse{}, ethereum.NotFound
	}
	return root, nil
}

func (m *mockSupervisorClient) Close() {}
