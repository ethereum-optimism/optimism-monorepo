package batcher

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-batcher/metrics"
	"github.com/ethereum-optimism/optimism/op-service/dial"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/queue"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

type mockL2EndpointProvider struct {
	ethClient       *testutils.MockL2Client
	ethClientErr    error
	rollupClient    *testutils.MockRollupClient
	rollupClientErr error
}

func newEndpointProvider() *mockL2EndpointProvider {
	return &mockL2EndpointProvider{
		ethClient:    new(testutils.MockL2Client),
		rollupClient: new(testutils.MockRollupClient),
	}
}

func (p *mockL2EndpointProvider) EthClient(context.Context) (dial.EthClientInterface, error) {
	return p.ethClient, p.ethClientErr
}

func (p *mockL2EndpointProvider) RollupClient(context.Context) (dial.RollupClientInterface, error) {
	return p.rollupClient, p.rollupClientErr
}

func (p *mockL2EndpointProvider) Close() {}

const genesisL1Origin = uint64(123)

func setup(t *testing.T) (*BatchSubmitter, *mockL2EndpointProvider) {
	ep := newEndpointProvider()

	cfg := defaultTestRollupConfig
	cfg.Genesis.L1.Number = genesisL1Origin

	return NewBatchSubmitter(DriverSetup{
		Log:              testlog.Logger(t, log.LevelDebug),
		Metr:             metrics.NoopMetrics,
		RollupConfig:     cfg,
		ChannelConfig:    defaultTestChannelConfig(),
		EndpointProvider: ep,
	}), ep
}

func TestBatchSubmitter_SafeL1Origin(t *testing.T) {
	bs, ep := setup(t)

	tests := []struct {
		name                   string
		currentSafeOrigin      uint64
		failsToFetchSyncStatus bool
		expectResult           uint64
		expectErr              bool
	}{
		{
			name:              "ExistingSafeL1Origin",
			currentSafeOrigin: 999,
			expectResult:      999,
		},
		{
			name:              "NoExistingSafeL1OriginUsesGenesis",
			currentSafeOrigin: 0,
			expectResult:      genesisL1Origin,
		},
		{
			name:                   "ErrorFetchingSyncStatus",
			failsToFetchSyncStatus: true,
			expectErr:              true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.failsToFetchSyncStatus {
				ep.rollupClient.ExpectSyncStatus(&eth.SyncStatus{}, errors.New("failed to fetch sync status"))
			} else {
				ep.rollupClient.ExpectSyncStatus(&eth.SyncStatus{
					SafeL2: eth.L2BlockRef{
						L1Origin: eth.BlockID{
							Number: tt.currentSafeOrigin,
						},
					},
				}, nil)
			}

			id, err := bs.safeL1Origin(context.Background())

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectResult, id.Number)
			}
		})
	}
}

func TestBatchSubmitter_SafeL1Origin_FailsToResolveRollupClient(t *testing.T) {
	bs, ep := setup(t)

	ep.rollupClientErr = errors.New("failed to resolve rollup client")

	_, err := bs.safeL1Origin(context.Background())
	require.Error(t, err)
}

type testChannelStatuser struct {
	latestL2                 eth.BlockID
	inclusionBlock           uint64
	fullySubmitted, timedOut bool
}

func (tcs testChannelStatuser) LatestL2() eth.BlockID {
	return tcs.latestL2
}

func (tcs testChannelStatuser) maxInclusionBlock() uint64 {
	return tcs.inclusionBlock
}
func (tcs testChannelStatuser) isFullySubmitted() bool {
	return tcs.fullySubmitted
}

func (tcs testChannelStatuser) isTimedOut() bool {
	return tcs.timedOut
}

func TestBatchSubmitter_computeSyncActions(t *testing.T) {

	block101 := types.NewBlockWithHeader(&types.Header{Number: big.NewInt(101)})
	block102 := types.NewBlockWithHeader(&types.Header{Number: big.NewInt(102)})
	block103 := types.NewBlockWithHeader(&types.Header{Number: big.NewInt(103)})

	channel103 := testChannelStatuser{
		latestL2:       eth.ToBlockID(block103),
		inclusionBlock: 1,
		fullySubmitted: true,
		timedOut:       false,
	}

	type TestCase struct {
		name string
		// inputs
		newSyncStatus, prevSyncStatus *eth.SyncStatus
		blocks                        queue.Queue[*types.Block]
		channels                      []ChannelStatuser
		// expectations
		expectedSyncActions SyncActions
		expectedLogs        []string
	}

	testCases := []TestCase{
		{name: "empty sync status",
			newSyncStatus:       &eth.SyncStatus{},
			expectedSyncActions: SyncActions{waitForNodeSync: true},
		},
		{name: "happy path",
			newSyncStatus: &eth.SyncStatus{
				HeadL1:    eth.BlockRef{Number: 2},
				CurrentL1: eth.BlockRef{Number: 2},
				SafeL2:    eth.L2BlockRef{Number: 103, Hash: block103.Hash()},
				UnsafeL2:  eth.L2BlockRef{Number: 109},
			},
			prevSyncStatus: &eth.SyncStatus{
				CurrentL1: eth.BlockRef{Number: 1},
				SafeL2:    eth.L2BlockRef{Number: 100},
			},
			blocks:   queue.Queue[*types.Block]{block101, block102, block103},
			channels: []ChannelStatuser{channel103},
			expectedSyncActions: SyncActions{
				blocksToPrune:   3,
				channelsToPrune: 1,
				blocksToLoad:    [2]uint64{104, 109},
			},
		},
	}

	for _, tc := range testCases {

		t.Run(tc.name, func(t *testing.T) {

			testLogger := testlog.Logger(t, log.LevelDebug)
			result := computeSyncActions(
				tc.newSyncStatus, tc.prevSyncStatus, tc.blocks, tc.channels, testLogger,
			)

			require.Equal(t, tc.expectedSyncActions, result)
			// TODO log assertions
		})
	}
}
