package backend

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	types2 "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"

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
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

func TestBackendLifetime(t *testing.T) {
	logger := testlog.Logger(t, log.LvlInfo)
	m := metrics.NoopMetrics
	dataDir := t.TempDir()
	chainA := types.ChainIDFromUInt64(900)
	chainB := types.ChainIDFromUInt64(901)
	depSet, err := depset.NewStaticConfigDependencySet(
		map[types.ChainID]*depset.StaticConfigDependency{
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
		L2RPCs:                nil,
		Datadir:               dataDir,
	}

	b, err := NewSupervisorBackend(context.Background(), logger, m, cfg)
	require.NoError(t, err)
	t.Log("initialized!")

	src := &testutils.MockL1Source{}

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

	_, err = b.UnsafeView(context.Background(), chainA, types.ReferenceView{})
	require.ErrorIs(t, err, types.ErrFuture, "no data yet, need local-unsafe")

	src.ExpectL1BlockRefByNumber(0, blockX, nil)
	src.ExpectFetchReceipts(blockX.Hash, &testutils.MockBlockInfo{
		InfoHash:        blockX.Hash,
		InfoParentHash:  blockX.ParentHash,
		InfoNum:         blockX.Number,
		InfoTime:        blockX.Time,
		InfoReceiptRoot: types2.EmptyReceiptsHash,
	}, nil, nil)

	src.ExpectL1BlockRefByNumber(1, blockY, nil)
	src.ExpectFetchReceipts(blockY.Hash, &testutils.MockBlockInfo{
		InfoHash:        blockY.Hash,
		InfoParentHash:  blockY.ParentHash,
		InfoNum:         blockY.Number,
		InfoTime:        blockY.Time,
		InfoReceiptRoot: types2.EmptyReceiptsHash,
	}, nil, nil)

	src.ExpectL1BlockRefByNumber(2, eth.L1BlockRef{}, ethereum.NotFound)

	err = b.UpdateLocalUnsafe(context.Background(), chainA, blockY)
	require.NoError(t, err)
	// Make the processing happen, so we can rely on the new chain information,
	// and not run into errors for future data that isn't mocked at this time.
	proc, _ := b.chainProcessors.Get(chainA)
	proc.ProcessToHead()

	_, err = b.UnsafeView(context.Background(), chainA, types.ReferenceView{})
	require.ErrorIs(t, err, types.ErrFuture, "still no data yet, need cross-unsafe")

	err = b.chainDBs.UpdateCrossUnsafe(chainA, types.BlockSeal{
		Hash:      blockX.Hash,
		Number:    blockX.Number,
		Timestamp: blockX.Time,
	})
	require.NoError(t, err)

	v, err := b.UnsafeView(context.Background(), chainA, types.ReferenceView{})
	require.NoError(t, err, "have a functioning cross/local unsafe view now")
	require.Equal(t, blockX.ID(), v.Cross)
	require.Equal(t, blockY.ID(), v.Local)

	err = b.Stop(context.Background())
	require.NoError(t, err)
	t.Log("stopped!")
}
