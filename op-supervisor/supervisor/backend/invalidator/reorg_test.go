package invalidator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db/fromda"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db/logs"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

// type mockEmitter struct{}

// func (m *mockEmitter) Emit(ev event.Event) {}

type testChainSetup struct {
	t        *testing.T
	logger   log.Logger
	dataDir  string
	chainID  eth.ChainID
	chainsDB *db.ChainsDB
	logDB    *logs.DB
	localDB  *fromda.DB
	crossDB  *fromda.DB
}

func setupTestChain(t *testing.T) *testChainSetup {
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
	chainsDB := db.NewChainsDB(logger, depSet)
	chainsDB.AttachEmitter(&mockEmitter{})
	// chainsDB.emitter = &mockEmitter{}

	// Create the chain directory
	chainDir := filepath.Join(dataDir, "001", "1")
	err = os.MkdirAll(chainDir, 0o755)
	require.NoError(t, err)

	// Create and open the log DB
	logDB, err := logs.NewFromFile(logger, &stubMetrics{}, filepath.Join(chainDir, "log.db"), true)
	require.NoError(t, err)
	chainsDB.AddLogDB(chainID, logDB)

	// Create and open the local derived-from DB
	localDB, err := fromda.NewFromFile(logger, &stubMetrics{}, filepath.Join(chainDir, "local_safe.db"))
	require.NoError(t, err)
	chainsDB.AddLocalDerivedFromDB(chainID, localDB)

	// Create and open the cross derived-from DB
	crossDB, err := fromda.NewFromFile(logger, &stubMetrics{}, filepath.Join(chainDir, "cross_safe.db"))
	require.NoError(t, err)
	chainsDB.AddCrossDerivedFromDB(chainID, crossDB)

	// Add cross-unsafe tracker
	chainsDB.AddCrossUnsafeTracker(chainID)

	return &testChainSetup{
		t:        t,
		logger:   logger,
		dataDir:  dataDir,
		chainID:  chainID,
		chainsDB: chainsDB,
		logDB:    logDB,
		localDB:  localDB,
		crossDB:  crossDB,
	}
}

func (s *testChainSetup) Close() {
	s.chainsDB.Close()
	s.logDB.Close()
	s.localDB.Close()
	s.crossDB.Close()
}

func createTestBlocks() (genesis, block1, block2A, block2B eth.L2BlockRef) {
	genesis = eth.L2BlockRef{
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

	block1 = eth.L2BlockRef{
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

	block2A = eth.L2BlockRef{
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

	block2B = eth.L2BlockRef{
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

	return
}

func (s *testChainSetup) sealBlocks(blocks ...eth.L2BlockRef) {
	for _, block := range blocks {
		require.NoError(s.t, s.logDB.SealBlock(block.ParentHash, block.ID(), block.Time))
	}
}

func (s *testChainSetup) makeBlockSafe(block eth.L2BlockRef, l1Block eth.BlockRef, makeCrossSafe bool) {
	// Add the L1 derivation
	s.chainsDB.UpdateLocalSafe(s.chainID, l1Block, eth.BlockRef{
		Hash:       block.Hash,
		Number:     block.Number,
		Time:       block.Time,
		ParentHash: block.ParentHash,
	})

	if makeCrossSafe {
		require.NoError(s.t, s.chainsDB.UpdateCrossUnsafe(s.chainID, types.BlockSeal{
			Hash:      block.Hash,
			Number:    block.Number,
			Timestamp: block.Time,
		}))
		require.NoError(s.t, s.chainsDB.UpdateCrossSafe(s.chainID, l1Block, eth.BlockRef{
			Hash:       block.Hash,
			Number:     block.Number,
			Time:       block.Time,
			ParentHash: block.ParentHash,
		}))
	}
}

func (s *testChainSetup) verifyHead(expectedHead eth.BlockID, msg string) {
	head, ok := s.logDB.LatestSealedBlock()
	require.True(s.t, ok)
	require.Equal(s.t, expectedHead, head, msg)
}

func (s *testChainSetup) verifyCrossSafe(block eth.L2BlockRef, msg string) {
	crossSafe, err := s.chainsDB.CrossSafe(s.chainID)
	require.NoError(s.t, err)
	require.Equal(s.t, block.Hash, crossSafe.Derived.Hash, msg)
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

func TestReorgLocalUnsafe(t *testing.T) {
	s := setupTestChain(t)
	defer s.Close()

	genesis, block1, block2A, block2B := createTestBlocks()

	// Seal genesis and block1
	s.sealBlocks(genesis, block1)

	// Make block1 local-safe and cross-safe
	l1Block1 := eth.BlockRef{
		Hash:   common.HexToHash("0xaaa1"),
		Number: 1,
		Time:   900,
	}
	s.makeBlockSafe(block1, l1Block1, true)

	// Add block2A to the chain
	s.sealBlocks(block2A)

	// Verify the latest sealed block is block2A
	s.verifyHead(block2A.ID(), "should have set block2A as latest sealed block")

	// Now try to reorg to block2B
	require.NoError(t, s.chainsDB.InvalidateLocalUnsafe(s.chainID, block2B))

	// Verify the reorg happened
	s.verifyHead(block1.ID(), "should have rewound to block1")

	// Add block2B
	s.sealBlocks(block2B)

	// Verify we're now on the new chain
	s.verifyHead(block2B.ID(), "should be on block2B")
}

func TestReorgCrossUnsafe(t *testing.T) {
	s := setupTestChain(t)
	defer s.Close()

	genesis, block1, block2A, block2B := createTestBlocks()

	// Seal initial chain
	s.sealBlocks(genesis, block1, block2A)

	// Make block1 local-safe and cross-safe
	l1Block1 := eth.BlockRef{
		Hash:   common.HexToHash("0xaaa1"),
		Number: 1,
		Time:   900,
	}
	s.makeBlockSafe(block1, l1Block1, true)

	// Verify block1 is cross-safe
	s.verifyCrossSafe(block1, "block1 should be cross-safe")

	// Make block2 local-safe but not cross-safe
	l1Block2 := eth.BlockRef{
		Hash:       common.HexToHash("0xaaa2"),
		Number:     2,
		Time:       901,
		ParentHash: l1Block1.Hash,
	}
	s.makeBlockSafe(block2A, l1Block2, false)

	// Set block2 as cross-unsafe
	require.NoError(s.t, s.chainsDB.UpdateCrossUnsafe(s.chainID, types.BlockSeal{
		Hash:      block2A.Hash,
		Number:    block2A.Number,
		Timestamp: block2A.Time,
	}))

	// Verify block2 is the latest sealed block
	s.verifyHead(block2A.ID(), "should have set block2 as latest sealed block")

	// Now try to invalidate block2 as cross-unsafe
	require.NoError(t, s.chainsDB.InvalidateCrossUnsafe(s.chainID, block2A))

	// Verify we rewound to block1 (the cross-safe head)
	s.verifyHead(block1.ID(), "should have rewound to block1")

	// Add block2B
	s.sealBlocks(block2B)

	// Verify we're now on the new chain
	s.verifyHead(block2B.ID(), "should be on block2B")
}
