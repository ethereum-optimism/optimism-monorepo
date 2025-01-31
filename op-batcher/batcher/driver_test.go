package batcher

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"
	"time"

	altda "github.com/ethereum-optimism/optimism/op-alt-da"
	"github.com/ethereum-optimism/optimism/op-batcher/compressor"
	"github.com/ethereum-optimism/optimism/op-batcher/metrics"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-service/dial"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/ethereum/go-ethereum/common"
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
	fmt.Println(err)
	require.Error(t, err)
}

// ======= ALTDA TESTS =======

// fakeL1Client is just a dummy struct. All fault injection is done via the fakeTxMgr (which doesn't interact with this fakeL1Client).
type fakeL1Client struct {
}

func (f *fakeL1Client) HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error) {
	if number == nil {
		number = big.NewInt(0)
	}
	return &types.Header{
		Number:     number,
		ParentHash: common.Hash{},
		Time:       0,
	}, nil
}
func (f *fakeL1Client) NonceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (uint64, error) {
	return 0, nil
}

func altDASetup(t *testing.T, log log.Logger) (*BatchSubmitter, *mockL2EndpointProvider, *altda.MockDAClient, *testutils.FakeTxMgr) {
	ep := newEndpointProvider()

	rollupCfg := &rollup.Config{
		Genesis:   rollup.Genesis{L2: eth.BlockID{Number: 0}, L1: eth.BlockID{Number: genesisL1Origin}},
		L2ChainID: big.NewInt(1234),
	}
	batcherCfg := BatcherConfig{
		PollInterval: 10 * time.Millisecond,
		UseAltDA:     true,
	}

	fakeTxMgr := testutils.NewFakeTxMgr(log.With("subsystem", "fake-txmgr"), common.Address{0})
	l1Client := &fakeL1Client{}

	channelCfg := ChannelConfig{
		// SeqWindowSize:      15,
		// SubSafetyMargin:    4,
		ChannelTimeout:  10,
		MaxFrameSize:    150, // so that each channel has exactly 1 frame
		TargetNumFrames: 1,
		BatchType:       derive.SingularBatchType,
		CompressorConfig: compressor.Config{
			Kind: compressor.NoneKind,
		},
	}
	mockAltDAClient := altda.NewCountingGenericCommitmentMockDAClient(log.With("subsystem", "da-client"))
	return NewBatchSubmitter(DriverSetup{
		Log:              log,
		Metr:             metrics.NoopMetrics,
		RollupConfig:     rollupCfg,
		ChannelConfig:    channelCfg,
		Config:           batcherCfg,
		EndpointProvider: ep,
		Txmgr:            fakeTxMgr,
		L1Client:         l1Client,
		AltDA:            mockAltDAClient,
	}), ep, mockAltDAClient, fakeTxMgr
}

func fakeSyncStatus(unsafeL2BlockNum uint64, L1BlockRef eth.L1BlockRef) *eth.SyncStatus {
	return &eth.SyncStatus{
		UnsafeL2: eth.L2BlockRef{
			Number: unsafeL2BlockNum,
			L1Origin: eth.BlockID{
				Number: 0,
			},
		},
		SafeL2: eth.L2BlockRef{
			Number: 0,
			L1Origin: eth.BlockID{
				Number: 0,
			},
		},
		HeadL1: L1BlockRef,
	}
}

// There are 4 failure cases (unhappy paths) that the op-batcher has to deal with.
// They are outlined in https://github.com/ethereum-optimism/optimism/tree/develop/op-batcher#happy-path
// This test suite covers these 4 cases in the context of AltDA.
func TestBatchSubmitter_AltDA_FailureCase1_L2Reorg(t *testing.T) {
	t.Parallel()
	log := testlog.Logger(t, log.LevelDebug)
	bs, ep, mockAltDAClient, fakeTxMgr := altDASetup(t, log)

	L1Block0 := types.NewBlock(&types.Header{
		Number: big.NewInt(0),
	}, nil, nil, nil, types.DefaultBlockConfig)
	L1Block0Ref := eth.L1BlockRef{
		Hash:   L1Block0.Hash(),
		Number: L1Block0.NumberU64(),
	}
	// We return incremental syncStatuses to force the op-batcher to entirely process each L2 block one by one.
	// To test multi channel behavior, we could return a sync status that is multiple blocks ahead of the current L2 block.
	ep.rollupClient.Mock.On("SyncStatus").Times(10).Return(fakeSyncStatus(1, L1Block0Ref), nil)
	ep.rollupClient.Mock.On("SyncStatus").Times(10).Return(fakeSyncStatus(2, L1Block0Ref), nil)
	ep.rollupClient.Mock.On("SyncStatus").Times(10).Return(fakeSyncStatus(3, L1Block0Ref), nil)
	ep.rollupClient.Mock.On("SyncStatus").Times(10).Return(fakeSyncStatus(1, L1Block0Ref), nil)
	ep.rollupClient.Mock.On("SyncStatus").Times(10).Return(fakeSyncStatus(2, L1Block0Ref), nil)
	ep.rollupClient.Mock.On("SyncStatus").Return(fakeSyncStatus(3, L1Block0Ref), nil)

	L2Block0 := newMiniL2BlockWithNumberParent(1, big.NewInt(0), common.HexToHash("0x0"))
	L2Block1 := newMiniL2BlockWithNumberParent(1, big.NewInt(1), L2Block0.Hash())
	L2Block2 := newMiniL2BlockWithNumberParent(1, big.NewInt(2), L2Block1.Hash())
	L2Block2Prime := newMiniL2BlockWithNumberParentAndL1Information(1, big.NewInt(2), L2Block1.Hash(), 101, 0)
	L2Block3Prime := newMiniL2BlockWithNumberParent(1, big.NewInt(3), L2Block2Prime.Hash())

	// L2block0 is the genesis block which is considered safe, so never loaded into the state.
	ep.ethClient.Mock.On("BlockByNumber", big.NewInt(1)).Twice().Return(L2Block1, nil)
	ep.ethClient.Mock.On("BlockByNumber", big.NewInt(2)).Once().Return(L2Block2, nil)
	ep.ethClient.Mock.On("BlockByNumber", big.NewInt(2)).Once().Return(L2Block2Prime, nil)
	ep.ethClient.Mock.On("BlockByNumber", big.NewInt(3)).Twice().Return(L2Block3Prime, nil)

	err := bs.StartBatchSubmitting()
	require.NoError(t, err)
	time.Sleep(1 * time.Second) // 1 second is enough to process all blocks at 10ms poll interval
	err = bs.StopBatchSubmitting(context.Background())
	require.NoError(t, err)

	// After the reorg, block 1 needs to be reprocessed, hence why we see 5 store calls: 1, 2, 1, 2', 3'
	require.Equal(t, 5, mockAltDAClient.StoreCount)
	require.Equal(t, uint64(5), fakeTxMgr.Nonce)

}

func TestBatchSubmitter_AltDA_FailureCase2_FailedL1Tx(t *testing.T) {
	t.Parallel()
	log := testlog.Logger(t, log.LevelDebug)
	bs, ep, mockAltDAClient, fakeTxMgr := altDASetup(t, log)

	L1Block0 := types.NewBlock(&types.Header{
		Number: big.NewInt(0),
	}, nil, nil, nil, types.DefaultBlockConfig)
	L1Block0Ref := eth.L1BlockRef{
		Hash:   L1Block0.Hash(),
		Number: L1Block0.NumberU64(),
	}
	// We return incremental syncStatuses to force the op-batcher to entirely process each L2 block one by one.
	// To test multi channel behavior, we could return a sync status that is multiple blocks ahead of the current L2 block.
	ep.rollupClient.Mock.On("SyncStatus").Times(10).Return(fakeSyncStatus(1, L1Block0Ref), nil)
	ep.rollupClient.Mock.On("SyncStatus").Times(10).Return(fakeSyncStatus(2, L1Block0Ref), nil)
	ep.rollupClient.Mock.On("SyncStatus").Times(10).Return(fakeSyncStatus(3, L1Block0Ref), nil)
	ep.rollupClient.Mock.On("SyncStatus").Return(fakeSyncStatus(4, L1Block0Ref), nil)

	L2Block0 := newMiniL2BlockWithNumberParent(1, big.NewInt(0), common.HexToHash("0x0"))
	L2Block1 := newMiniL2BlockWithNumberParent(1, big.NewInt(1), L2Block0.Hash())
	L2Block2 := newMiniL2BlockWithNumberParent(1, big.NewInt(2), L2Block1.Hash())
	L2Block3 := newMiniL2BlockWithNumberParent(1, big.NewInt(3), L2Block2.Hash())
	L2Block4 := newMiniL2BlockWithNumberParent(1, big.NewInt(4), L2Block3.Hash())

	// L2block0 is the genesis block which is considered safe, so never loaded into the state.
	ep.ethClient.Mock.On("BlockByNumber", big.NewInt(1)).Once().Return(L2Block1, nil)
	ep.ethClient.Mock.On("BlockByNumber", big.NewInt(2)).Once().Return(L2Block2, nil)
	ep.ethClient.Mock.On("BlockByNumber", big.NewInt(3)).Once().Return(L2Block3, nil)
	ep.ethClient.Mock.On("BlockByNumber", big.NewInt(4)).Once().Return(L2Block4, nil)

	fakeTxMgr.ErrorEveryNthSend(2)
	err := bs.StartBatchSubmitting()
	require.NoError(t, err)
	time.Sleep(1 * time.Second) // 1 second is enough to process all blocks at 10ms poll interval
	err = bs.StopBatchSubmitting(context.Background())
	require.NoError(t, err)

	require.Equal(t, 4, mockAltDAClient.StoreCount)
	// TODO: we should prob also check that the commitments are in order?
	require.Equal(t, uint64(4), fakeTxMgr.Nonce)
}

func TestBatchSubmitter_AltDA_FailureCase3_ChannelTimeout(t *testing.T) {
	// TODO: implement this test
}

func TestBatchSubmitter_AltDA_FailureCase4_FailedBlobSubmission(t *testing.T) {
	t.Parallel()
	log := testlog.Logger(t, log.LevelDebug)
	bs, ep, mockAltDAClient, fakeTxMgr := altDASetup(t, log)

	L1Block0 := types.NewBlock(&types.Header{
		Number: big.NewInt(0),
	}, nil, nil, nil, types.DefaultBlockConfig)
	L1Block0Ref := eth.L1BlockRef{
		Hash:   L1Block0.Hash(),
		Number: L1Block0.NumberU64(),
	}
	ep.rollupClient.Mock.On("SyncStatus").Return(fakeSyncStatus(4, L1Block0Ref), nil)

	L2Block0 := newMiniL2BlockWithNumberParent(1, big.NewInt(0), common.HexToHash("0x0"))
	L2Block1 := newMiniL2BlockWithNumberParent(1, big.NewInt(1), L2Block0.Hash())
	L2Block2 := newMiniL2BlockWithNumberParent(1, big.NewInt(2), L2Block1.Hash())
	L2Block3 := newMiniL2BlockWithNumberParent(1, big.NewInt(3), L2Block2.Hash())
	L2Block4 := newMiniL2BlockWithNumberParent(1, big.NewInt(4), L2Block3.Hash())

	// L2block0 is the genesis block which is considered safe, so never loaded into the state.
	ep.ethClient.Mock.On("BlockByNumber", big.NewInt(1)).Once().Return(L2Block1, nil)
	ep.ethClient.Mock.On("BlockByNumber", big.NewInt(2)).Once().Return(L2Block2, nil)
	ep.ethClient.Mock.On("BlockByNumber", big.NewInt(3)).Once().Return(L2Block3, nil)
	ep.ethClient.Mock.On("BlockByNumber", big.NewInt(4)).Once().Return(L2Block4, nil)

	mockAltDAClient.DropEveryNthPut(2)

	err := bs.StartBatchSubmitting()
	require.NoError(t, err)
	time.Sleep(1 * time.Second) // 1 second is enough to process all blocks at 10ms poll interval
	err = bs.StopBatchSubmitting(context.Background())
	require.NoError(t, err)

	require.Equal(t, 4, mockAltDAClient.StoreCount)
	require.Equal(t, uint64(4), fakeTxMgr.Nonce)
}
