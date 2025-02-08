package backend

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	types2 "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	oprpc "github.com/ethereum-optimism/optimism/op-service/rpc"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/ethereum-optimism/optimism/op-supervisor/config"
	"github.com/ethereum-optimism/optimism/op-supervisor/metrics"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/processors"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/superevents"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/syncnode"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

func TestBackendLifetime(t *testing.T) {
	logger := testlog.Logger(t, log.LvlInfo)
	m := metrics.NoopMetrics
	dataDir := t.TempDir()
	chainA := eth.ChainIDFromUInt64(900)
	chainB := eth.ChainIDFromUInt64(901)
	depSet, err := depset.NewStaticConfigDependencySet(
		map[eth.ChainID]*depset.StaticConfigDependency{
			chainA: {
				ChainIndex:     900,
				ActivationTime: 42,
				HistoryMinTime: 100,
			},
			chainB: {
				ChainIndex:     901,
				ActivationTime: 30,
				HistoryMinTime: 20,
			},
		})
	require.NoError(t, err)
	cfg := &config.Config{
		Version:               "test",
		LogConfig:             oplog.CLIConfig{},
		MetricsConfig:         opmetrics.CLIConfig{},
		PprofConfig:           oppprof.CLIConfig{},
		RPC:                   oprpc.CLIConfig{},
		DependencySetSource:   depSet,
		SynchronousProcessors: true,
		MockRun:               false,
		SyncSources:           &syncnode.CLISyncNodes{},
		Datadir:               dataDir,
	}

	ex := event.NewGlobalSynchronous(context.Background())
	b, err := NewSupervisorBackend(context.Background(), logger, m, cfg, ex)
	require.NoError(t, err)
	t.Log("initialized!")

	l1Src := &testutils.MockL1Source{}
	src := &MockProcessorSource{}

	blockX := eth.BlockRef{
		Hash:       common.Hash{0xaa},
		Number:     0,
		ParentHash: common.Hash{}, // genesis has no parent hash
		Time:       10000,
	}
	blockY := eth.BlockRef{
		Hash:       common.Hash{0xbb},
		Number:     blockX.Number + 1,
		ParentHash: blockX.Hash,
		Time:       blockX.Time + 2,
	}

	b.AttachL1Source(l1Src)
	require.NoError(t, b.AttachProcessorSource(chainA, src))

	require.FileExists(t, filepath.Join(cfg.Datadir, "900", "log.db"), "must have logs DB 900")
	require.FileExists(t, filepath.Join(cfg.Datadir, "901", "log.db"), "must have logs DB 901")
	require.FileExists(t, filepath.Join(cfg.Datadir, "900", "local_safe.db"), "must have local safe DB 900")
	require.FileExists(t, filepath.Join(cfg.Datadir, "901", "local_safe.db"), "must have local safe DB 901")
	require.FileExists(t, filepath.Join(cfg.Datadir, "900", "cross_safe.db"), "must have cross safe DB 900")
	require.FileExists(t, filepath.Join(cfg.Datadir, "901", "cross_safe.db"), "must have cross safe DB 901")

	err = b.Start(context.Background())
	require.NoError(t, err)
	t.Log("started!")

	_, err = b.LocalUnsafe(context.Background(), chainA)
	require.ErrorIs(t, err, types.ErrFuture, "no data yet, need local-unsafe")

	src.ExpectBlockRefByNumber(0, blockX, nil)
	src.ExpectFetchReceipts(blockX.Hash, nil, nil)

	src.ExpectBlockRefByNumber(1, blockY, nil)
	src.ExpectFetchReceipts(blockY.Hash, nil, nil)

	src.ExpectBlockRefByNumber(2, eth.L1BlockRef{}, ethereum.NotFound)

	b.emitter.Emit(superevents.LocalUnsafeReceivedEvent{
		ChainID:        chainA,
		NewLocalUnsafe: blockY,
	})
	// Make the processing happen, so we can rely on the new chain information,
	// and not run into errors for future data that isn't mocked at this time.
	require.NoError(t, ex.Drain())

	_, err = b.CrossUnsafe(context.Background(), chainA)
	require.ErrorIs(t, err, types.ErrFuture, "still no data yet, need cross-unsafe")

	err = b.chainDBs.UpdateCrossUnsafe(chainA, types.BlockSeal{
		Hash:      blockX.Hash,
		Number:    blockX.Number,
		Timestamp: blockX.Time,
	})
	require.NoError(t, err)

	v, err := b.CrossUnsafe(context.Background(), chainA)
	require.NoError(t, err, "have a functioning cross unsafe value now")
	require.Equal(t, blockX.ID(), v)

	err = b.Stop(context.Background())
	require.NoError(t, err)
	t.Log("stopped!")
}

func TestBackendCallsMetrics(t *testing.T) {
	logger := testlog.Logger(t, log.LvlInfo)
	mockMetrics := &MockMetrics{}
	dataDir := t.TempDir()
	chainA := eth.ChainIDFromUInt64(900)

	// Set up mock metrics
	mockMetrics.Mock.On("RecordDBEntryCount", chainA, mock.AnythingOfType("string"), mock.AnythingOfType("int64")).Return()
	mockMetrics.Mock.On("RecordCrossUnsafeRef", chainA, mock.MatchedBy(func(_ eth.BlockRef) bool { return true })).Return()
	mockMetrics.Mock.On("RecordCrossSafeRef", chainA, mock.MatchedBy(func(_ eth.BlockRef) bool { return true })).Return()

	depSet, err := depset.NewStaticConfigDependencySet(
		map[eth.ChainID]*depset.StaticConfigDependency{
			chainA: {
				ChainIndex:     900,
				ActivationTime: 42,
				HistoryMinTime: 100,
			},
		})
	require.NoError(t, err)

	cfg := &config.Config{
		Version:               "test",
		LogConfig:             oplog.CLIConfig{},
		MetricsConfig:         opmetrics.CLIConfig{},
		PprofConfig:           oppprof.CLIConfig{},
		RPC:                   oprpc.CLIConfig{},
		DependencySetSource:   depSet,
		SynchronousProcessors: true,
		MockRun:               false,
		SyncSources:           &syncnode.CLISyncNodes{},
		Datadir:               dataDir,
	}

	ex := event.NewGlobalSynchronous(context.Background())
	b, err := NewSupervisorBackend(context.Background(), logger, mockMetrics, cfg, ex)
	require.NoError(t, err)

	// Assert that the metrics are called at initialization
	mockMetrics.Mock.AssertCalled(t, "RecordDBEntryCount", chainA, "log", int64(0))
	mockMetrics.Mock.AssertCalled(t, "RecordDBEntryCount", chainA, "local_derived", int64(0))
	mockMetrics.Mock.AssertCalled(t, "RecordDBEntryCount", chainA, "cross_derived", int64(0))

	// Start the backend
	err = b.Start(context.Background())
	require.NoError(t, err)

	// Create a test block
	block := eth.BlockRef{
		Hash:       common.Hash{0xaa},
		Number:     42,
		ParentHash: common.Hash{0xbb},
		Time:       10000,
	}

	// Assert that metrics are called on safety level updates
	err = b.chainDBs.UpdateCrossUnsafe(chainA, types.BlockSeal{
		Hash:      block.Hash,
		Number:    block.Number,
		Timestamp: block.Time,
	})
	require.NoError(t, err)
	mockMetrics.Mock.AssertCalled(t, "RecordCrossUnsafeRef", chainA, mock.MatchedBy(func(ref eth.BlockRef) bool {
		return ref.Hash == block.Hash && ref.Number == block.Number && ref.Time == block.Time
	}))

	err = b.chainDBs.UpdateCrossSafe(chainA, block, block)
	require.NoError(t, err)
	mockMetrics.Mock.AssertCalled(t, "RecordDBEntryCount", chainA, "cross_derived", int64(1))
	mockMetrics.Mock.AssertCalled(t, "RecordCrossSafeRef", chainA, mock.MatchedBy(func(ref eth.BlockRef) bool {
		return ref.Hash == block.Hash && ref.Number == block.Number && ref.Time == block.Time
	}))

	// Stop the backend
	err = b.Stop(context.Background())
	require.NoError(t, err)
}

type MockMetrics struct {
	mock.Mock
	event.NoopMetrics
}

var _ Metrics = (*MockMetrics)(nil)

func (m *MockMetrics) CacheAdd(chainID eth.ChainID, label string, cacheSize int, evicted bool) {
	m.Mock.Called(chainID, label, cacheSize, evicted)
}

func (m *MockMetrics) CacheGet(chainID eth.ChainID, label string, hit bool) {
	m.Mock.Called(chainID, label, hit)
}

func (m *MockMetrics) RecordCrossUnsafeRef(chainID eth.ChainID, ref eth.BlockRef) {
	m.Mock.Called(chainID, ref)
}

func (m *MockMetrics) RecordCrossSafeRef(chainID eth.ChainID, ref eth.BlockRef) {
	m.Mock.Called(chainID, ref)
}

func (m *MockMetrics) RecordDBEntryCount(chainID eth.ChainID, kind string, count int64) {
	m.Mock.Called(chainID, kind, count)
}

func (m *MockMetrics) RecordDBSearchEntriesRead(chainID eth.ChainID, count int64) {
	m.Mock.Called(chainID, count)
}

type MockProcessorSource struct {
	mock.Mock
}

var _ processors.Source = (*MockProcessorSource)(nil)

func (m *MockProcessorSource) FetchReceipts(ctx context.Context, blockHash common.Hash) (types2.Receipts, error) {
	out := m.Mock.Called(blockHash)
	return out.Get(0).(types2.Receipts), out.Error(1)
}

func (m *MockProcessorSource) ExpectFetchReceipts(hash common.Hash, receipts types2.Receipts, err error) {
	m.Mock.On("FetchReceipts", hash).Once().Return(receipts, err)
}

func (m *MockProcessorSource) BlockRefByNumber(ctx context.Context, num uint64) (eth.BlockRef, error) {
	out := m.Mock.Called(num)
	return out.Get(0).(eth.BlockRef), out.Error(1)
}

func (m *MockProcessorSource) ExpectBlockRefByNumber(num uint64, ref eth.BlockRef, err error) {
	m.Mock.On("BlockRefByNumber", num).Once().Return(ref, err)
}
