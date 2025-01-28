package db

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db/fromda"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db/logs"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type mockEmitter struct{}

func (m *mockEmitter) Emit(ev event.Event) {}

func TestLocalUnsafeReorg(t *testing.T) {
	logger := testlog.Logger(t, log.LvlInfo)
	dataDir := t.TempDir()

	// Create a simple dependency set with one chain
	chainID := eth.ChainID{1}
	depSet, err := depset.NewStaticConfigDependencySet(
		map[eth.ChainID]*depset.StaticConfigDependency{
			chainID: {
				ChainIndex:     1,
				ActivationTime: 42,
				HistoryMinTime: 100,
			},
		})
	require.NoError(t, err)

	// Create ChainsDB with mock emitter
	chainsDB := NewChainsDB(logger, depSet)
	chainsDB.emitter = &mockEmitter{}
	defer chainsDB.Close()

	// Create the chain directory
	chainDir := filepath.Join(dataDir, "001", "1")
	err = os.MkdirAll(chainDir, 0o755)
	require.NoError(t, err)

	// Create and open the log DB
	logDB, err := logs.NewFromFile(logger, &stubMetrics{}, filepath.Join(chainDir, "log.db"), true)
	require.NoError(t, err)
	defer logDB.Close()
	chainsDB.AddLogDB(chainID, logDB)

	// Create and open the local derived-from DB
	localDB, err := fromda.NewFromFile(logger, &stubMetrics{}, filepath.Join(chainDir, "local_safe.db"))
	require.NoError(t, err)
	defer localDB.Close()
	chainsDB.AddLocalDerivedFromDB(chainID, localDB)

	// Create and open the cross derived-from DB
	crossDB, err := fromda.NewFromFile(logger, &stubMetrics{}, filepath.Join(chainDir, "cross_safe.db"))
	require.NoError(t, err)
	defer crossDB.Close()
	chainsDB.AddCrossDerivedFromDB(chainID, crossDB)

	// Add cross-unsafe tracker
	chainsDB.AddCrossUnsafeTracker(chainID)

	// Create a sequence of blocks
	genesis := eth.L2BlockRef{
		Hash:       common.HexToHash("0x1111"),
		Number:     0,
		ParentHash: common.Hash{},
		Time:       1000,
		L1Origin: eth.BlockID{
			Hash:   common.HexToHash("0xaaa1"),
			Number: 1,
		},
		SequenceNumber: 0,
	}

	block1 := eth.L2BlockRef{
		Hash:       common.HexToHash("0x2222"),
		Number:     1,
		ParentHash: genesis.Hash,
		Time:       1001,
		L1Origin: eth.BlockID{
			Hash:   common.HexToHash("0xaaa1"),
			Number: 1,
		},
		SequenceNumber: 1,
	}

	block2A := eth.L2BlockRef{
		Hash:       common.HexToHash("0x3333"),
		Number:     2,
		ParentHash: block1.Hash,
		Time:       1002,
		L1Origin: eth.BlockID{
			Hash:   common.HexToHash("0xaaa1"),
			Number: 1,
		},
		SequenceNumber: 2,
	}

	block2B := eth.L2BlockRef{
		Hash:       common.HexToHash("0x4444"),
		Number:     2,
		ParentHash: block1.Hash,
		Time:       1002,
		L1Origin: eth.BlockID{
			Hash:   common.HexToHash("0xaaa1"),
			Number: 1,
		},
		SequenceNumber: 2,
	}

	// Seal genesis and block1
	require.NoError(t, logDB.SealBlock(common.Hash{}, genesis.ID(), genesis.Time))
	require.NoError(t, logDB.SealBlock(genesis.Hash, block1.ID(), block1.Time))

	// Make block1 local-safe by recording it was derived from L1 block
	require.NoError(t, localDB.AddDerived(eth.BlockRef{
		Hash:   common.HexToHash("0xaaa1"),
		Number: 1,
		Time:   900,
	}, eth.BlockRef{
		Hash:   block1.Hash,
		Number: block1.Number,
		Time:   block1.Time,
	}))

	// Make block1 cross-safe by recording it was derived from L1 block
	require.NoError(t, crossDB.AddDerived(eth.BlockRef{
		Hash:   common.HexToHash("0xaaa1"),
		Number: 1,
		Time:   900,
	}, eth.BlockRef{
		Hash:   block1.Hash,
		Number: block1.Number,
		Time:   block1.Time,
	}))

	// Add block2A to the chain
	require.NoError(t, logDB.SealBlock(block1.Hash, block2A.ID(), block2A.Time))

	// Verify the latest sealed block is block2A
	head, ok := logDB.LatestSealedBlock()
	require.True(t, ok)
	require.Equal(t, block2A.ID(), head, "should have set block2A as latest sealed block")

	// Now try to reorg to block2B
	require.NoError(t, chainsDB.InvalidateLocalUnsafe(chainID, block2B))

	// Verify the reorg happened
	head, ok = logDB.LatestSealedBlock()
	require.True(t, ok)
	require.Equal(t, block1.ID(), head, "should have rewound to block1")

	// Add block2B
	require.NoError(t, logDB.SealBlock(block1.Hash, block2B.ID(), block2B.Time))

	// Verify we're now on the new chain
	head, ok = logDB.LatestSealedBlock()
	require.True(t, ok)
	require.Equal(t, block2B.ID(), head, "should be on block2B")
}

func TestCrossUnsafeReorg(t *testing.T) {
	logger := testlog.Logger(t, log.LvlInfo)
	dataDir := t.TempDir()

	// Create a simple dependency set with one chain
	chainID := eth.ChainID{1}
	depSet, err := depset.NewStaticConfigDependencySet(
		map[eth.ChainID]*depset.StaticConfigDependency{
			chainID: {
				ChainIndex:     1,
				ActivationTime: 42,
				HistoryMinTime: 100,
			},
		})
	require.NoError(t, err)

	// Create ChainsDB with mock emitter
	chainsDB := NewChainsDB(logger, depSet)
	chainsDB.emitter = &mockEmitter{}
	defer chainsDB.Close()

	// Create the chain directory
	chainDir := filepath.Join(dataDir, "001", "1")
	err = os.MkdirAll(chainDir, 0o755)
	require.NoError(t, err)

	// Create and open the log DB
	logDB, err := logs.NewFromFile(logger, &stubMetrics{}, filepath.Join(chainDir, "log.db"), true)
	require.NoError(t, err)
	defer logDB.Close()
	chainsDB.AddLogDB(chainID, logDB)

	// Create and open the local derived-from DB
	localDB, err := fromda.NewFromFile(logger, &stubMetrics{}, filepath.Join(chainDir, "local_safe.db"))
	require.NoError(t, err)
	defer localDB.Close()
	chainsDB.AddLocalDerivedFromDB(chainID, localDB)

	// Create and open the cross derived-from DB
	crossDB, err := fromda.NewFromFile(logger, &stubMetrics{}, filepath.Join(chainDir, "cross_safe.db"))
	require.NoError(t, err)
	defer crossDB.Close()
	chainsDB.AddCrossDerivedFromDB(chainID, crossDB)

	// Add cross-unsafe tracker
	chainsDB.AddCrossUnsafeTracker(chainID)

	// Create a sequence of blocks
	genesis := eth.L2BlockRef{
		Hash:       common.HexToHash("0x1111"),
		Number:     0,
		ParentHash: common.Hash{},
		Time:       1000,
		L1Origin: eth.BlockID{
			Hash:   common.HexToHash("0xaaa1"),
			Number: 1,
		},
		SequenceNumber: 0,
	}

	block1 := eth.L2BlockRef{
		Hash:       common.HexToHash("0x2222"),
		Number:     1,
		ParentHash: genesis.Hash,
		Time:       1001,
		L1Origin: eth.BlockID{
			Hash:   common.HexToHash("0xaaa1"),
			Number: 1,
		},
		SequenceNumber: 1,
	}

	block2 := eth.L2BlockRef{
		Hash:       common.HexToHash("0x3333"),
		Number:     2,
		ParentHash: block1.Hash,
		Time:       1002,
		L1Origin: eth.BlockID{
			Hash:   common.HexToHash("0xaaa1"),
			Number: 1,
		},
		SequenceNumber: 2,
	}

	// Seal all blocks
	require.NoError(t, logDB.SealBlock(common.Hash{}, genesis.ID(), genesis.Time))
	require.NoError(t, logDB.SealBlock(genesis.Hash, block1.ID(), block1.Time))
	require.NoError(t, logDB.SealBlock(block1.Hash, block2.ID(), block2.Time))

	// Make block1 local-safe and cross-safe
	l1Block1 := eth.BlockRef{
		Hash:   common.HexToHash("0xaaa1"),
		Number: 1,
		Time:   900,
	}
	require.NoError(t, localDB.AddDerived(l1Block1, eth.BlockRef{
		Hash:   block1.Hash,
		Number: block1.Number,
		Time:   block1.Time,
	}))
	require.NoError(t, crossDB.AddDerived(l1Block1, eth.BlockRef{
		Hash:   block1.Hash,
		Number: block1.Number,
		Time:   block1.Time,
	}))

	// Verify block1 is cross-safe
	crossSafe, err := crossDB.Latest()
	require.NoError(t, err)
	require.Equal(t, block1.Hash, crossSafe.Derived.Hash, "block1 should be cross-safe")

	// Make block2 local-safe but not cross-safe
	l1Block2 := eth.BlockRef{
		Hash:       common.HexToHash("0xaaa2"),
		Number:     2,
		Time:       901,
		ParentHash: l1Block1.Hash,
	}
	require.NoError(t, localDB.AddDerived(l1Block2, eth.BlockRef{
		Hash:       block2.Hash,
		Number:     block2.Number,
		Time:       block2.Time,
		ParentHash: block1.Hash,
	}))

	// Set block2 as cross-unsafe
	crossUnsafe, ok := chainsDB.crossUnsafe.Get(chainID)
	require.True(t, ok)
	crossUnsafe.Lock()
	crossUnsafe.Value = types.BlockSeal{
		Hash:      block2.Hash,
		Number:    block2.Number,
		Timestamp: block2.Time,
	}
	crossUnsafe.Unlock()

	// Verify block2 is the latest sealed block
	head, ok := logDB.LatestSealedBlock()
	require.True(t, ok)
	require.Equal(t, block2.ID(), head, "should have set block2 as latest sealed block")

	// Now try to invalidate block2 as cross-unsafe
	require.NoError(t, chainsDB.InvalidateCrossUnsafe(chainID, block2))

	// Verify we rewound to block1 (the cross-safe head)
	head, ok = logDB.LatestSealedBlock()
	require.True(t, ok)
	require.Equal(t, block1.ID(), head, "should have rewound to block1")
}

type stubMetrics struct {
	entryCount           int64
	entriesReadForSearch int64
	derivedEntryCount    int64
}

func (s *stubMetrics) RecordDBEntryCount(kind string, count int64) {
	s.entryCount = count
}

func (s *stubMetrics) RecordDBSearchEntriesRead(count int64) {
	s.entriesReadForSearch = count
}

func (s *stubMetrics) RecordDBDerivedEntryCount(count int64) {
	s.derivedEntryCount = count
}
