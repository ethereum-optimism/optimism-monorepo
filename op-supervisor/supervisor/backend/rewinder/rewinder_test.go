package rewinder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db/fromda"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db/logs"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/superevents"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

// TestRewindLocalUnsafe syncs a chain up to block2A, rewinds to block1, then adds block2B.
// block2A is local-unsafe but not cross-unsafe.
func TestRewindLocalUnsafe(t *testing.T) {
	s := setupTestChain(t)
	defer s.Close()

	chainID := eth.ChainID{1}
	chain := s.chains[chainID]

	genesis, block1, block2A, block2B := createTestBlocks()

	// Setup sync node with all blocks
	chain.setupSyncNodeBlocks(genesis, block1, block2A, block2B)

	// Seal genesis and block1
	s.sealBlocks(chainID, genesis, block1)

	// Make block1 local-safe and cross-safe
	l1Block1 := eth.BlockRef{
		Hash:   common.HexToHash("0xaaa1"),
		Number: 1,
		Time:   900,
	}
	s.makeBlockSafe(chainID, block1, l1Block1, true)

	// Add block2A but don't make it safe - it should stay local-unsafe
	s.sealBlocks(chainID, block2A)

	// Verify the latest sealed block is block2A
	s.verifyHead(chainID, block2A.ID(), "should have set block2A as latest sealed block")

	// Now try to reorg to block2B
	i := New(s.logger, s.chainsDB)
	i.AttachSyncNode(chainID, chain.syncNode)
	require.NoError(t, i.rewindChain(superevents.RewindL2ChainEvent{
		ChainID:        chainID,
		BadBlockHeight: block2A.Number,
	}))

	// Verify the reorg happened
	s.verifyHead(chainID, block1.ID(), "should have rewound to block1")

	// Add block2B
	s.sealBlocks(chainID, block2B)

	// Verify we're now on the new chain
	s.verifyHead(chainID, block2B.ID(), "should be on block2B")
}

// TestRewindCrossUnsafe syncs a chain up to block2A, rewinds to block1, then adds block2B.
// block2A is cross-unsafe.
func TestRewindCrossUnsafe(t *testing.T) {
	s := setupTestChain(t)
	defer s.Close()

	chainID := eth.ChainID{1}
	chain := s.chains[chainID]

	genesis, block1, block2A, block2B := createTestBlocks()

	// Setup sync node with all blocks
	chain.setupSyncNodeBlocks(genesis, block1, block2A, block2B)

	// Seal initial chain
	s.sealBlocks(chainID, genesis, block1, block2A)

	// Make block1 cross-safe
	l1Block1 := eth.BlockRef{
		Hash:   common.HexToHash("0xaaa1"),
		Number: 1,
		Time:   900,
	}
	s.makeBlockSafe(chainID, block1, l1Block1, true)
	s.verifyCrossSafe(chainID, block1, "block1 should be cross-safe")

	// Make block2A local-safe but not cross-safe
	l1Block2 := eth.BlockRef{
		Hash:       common.HexToHash("0xaaa2"),
		Number:     2,
		Time:       901,
		ParentHash: l1Block1.Hash,
	}
	// Only make block2A local-safe, not cross-safe
	s.chainsDB.UpdateLocalSafe(chainID, l1Block2, eth.BlockRef{
		Hash:       block2A.Hash,
		Number:     block2A.Number,
		Time:       block2A.Time,
		ParentHash: block2A.ParentHash,
	})

	// Set block2A as cross-unsafe
	require.NoError(t, s.chainsDB.UpdateCrossUnsafe(chainID, types.BlockSeal{
		Hash:      block2A.Hash,
		Number:    block2A.Number,
		Timestamp: block2A.Time,
	}))

	// Verify block2A is the latest sealed block
	s.verifyHead(chainID, block2A.ID(), "should have set block2A as latest sealed block")

	// Now try to rewind block2A
	i := New(s.logger, s.chainsDB)
	i.AttachSyncNode(chainID, chain.syncNode)
	require.NoError(t, i.rewindChain(superevents.RewindL2ChainEvent{
		ChainID:        chainID,
		BadBlockHeight: block2A.Number,
	}))

	// Verify we rewound to block1
	s.verifyHead(chainID, block1.ID(), "should have rewound to block1")
	s.verifyCrossSafe(chainID, block1, "block1 should still be cross-safe")

	// Add block2B
	s.sealBlocks(chainID, block2B)

	// Verify we're now on the new chain
	s.verifyHead(chainID, block2B.ID(), "should be on block2B")
}

// TestRewindLocalSafe syncs a chain up to block2A, rewinds to block1, then adds block2B.
// block2A is local-safe but not cross-safe.
func TestRewindLocalSafe(t *testing.T) {
	s := setupTestChain(t)
	defer s.Close()

	chainID := eth.ChainID{1}
	chain := s.chains[chainID]

	genesis, block1, block2A, block2B := createTestBlocks()

	// Setup sync node with all blocks
	chain.setupSyncNodeBlocks(genesis, block1, block2A, block2B)

	// Seal genesis and block1
	s.sealBlocks(chainID, genesis, block1)

	// Make block1 local-safe and cross-safe
	l1Block1 := eth.BlockRef{
		Hash:   common.HexToHash("0xaaa1"),
		Number: 1,
		Time:   900,
	}
	s.makeBlockSafe(chainID, block1, l1Block1, true)

	// Add block2A and make it local-safe but not cross-safe
	s.sealBlocks(chainID, block2A)
	l1Block2 := eth.BlockRef{
		Hash:       common.HexToHash("0xaaa2"),
		Number:     2,
		Time:       901,
		ParentHash: l1Block1.Hash,
	}
	s.makeBlockSafe(chainID, block2A, l1Block2, false)

	// Verify the latest sealed block is block2A
	s.verifyHead(chainID, block2A.ID(), "should have set block2A as latest sealed block")

	// Now try to reorg to block2B
	i := New(s.logger, s.chainsDB)
	i.AttachSyncNode(chainID, chain.syncNode)
	require.NoError(t, i.rewindChain(superevents.RewindL2ChainEvent{
		ChainID:        chainID,
		BadBlockHeight: block2A.Number,
	}))

	// Verify the reorg happened
	s.verifyHead(chainID, block1.ID(), "should have rewound to block1")

	// Add block2B
	s.sealBlocks(chainID, block2B)

	// Verify we're now on the new chain
	s.verifyHead(chainID, block2B.ID(), "should be on block2B")
}

// TestRewindCrossSafe syncs a chain up to block2A, rewinds to block1, then adds block2B.
// block2A is cross-safe.
func TestRewindCrossSafe(t *testing.T) {
	s := setupTestChain(t)
	defer s.Close()

	chainID := eth.ChainID{1}
	chain := s.chains[chainID]

	genesis, block1, block2A, block2B := createTestBlocks()

	// Setup sync node with all blocks
	chain.setupSyncNodeBlocks(genesis, block1, block2A, block2B)

	// Seal initial chain
	s.sealBlocks(chainID, genesis, block1, block2A)

	// Make block1 cross-safe
	l1Block1 := eth.BlockRef{
		Hash:   common.HexToHash("0xaaa1"),
		Number: 1,
		Time:   900,
	}
	s.makeBlockSafe(chainID, block1, l1Block1, true)
	s.verifyCrossSafe(chainID, block1, "block1 should be cross-safe")

	// Make block2A cross-safe
	l1Block2 := eth.BlockRef{
		Hash:       common.HexToHash("0xaaa2"),
		Number:     2,
		Time:       901,
		ParentHash: l1Block1.Hash,
	}
	s.makeBlockSafe(chainID, block2A, l1Block2, true)
	s.verifyCrossSafe(chainID, block2A, "block2A should be cross-safe")

	// Verify block2A is the latest sealed block
	s.verifyHead(chainID, block2A.ID(), "should have set block2A as latest sealed block")

	// Now try to rewind block2A
	i := New(s.logger, s.chainsDB)
	i.AttachSyncNode(chainID, chain.syncNode)
	require.NoError(t, i.rewindChain(superevents.RewindL2ChainEvent{
		ChainID:        chainID,
		BadBlockHeight: block2A.Number,
	}))

	// Verify we rewound to block1
	s.verifyHead(chainID, block1.ID(), "should have rewound to block1")
	s.verifyCrossSafe(chainID, block1, "block1 should still be cross-safe")

	// Add block2B
	s.sealBlocks(chainID, block2B)

	// Verify we're now on the new chain
	s.verifyHead(chainID, block2B.ID(), "should be on block2B")
}

// TestRewindLongChain syncs a long chain and rewinds many blocks.
func TestRewindLongChain(t *testing.T) {
	s := setupTestChain(t)
	defer s.Close()

	chainID := eth.ChainID{1}
	chain := s.chains[chainID]

	// Create a chain with blocks 0-100
	var blocks []eth.L2BlockRef
	var l1Blocks []eth.BlockRef

	// Create L1 blocks first (one per 10 L2 blocks)
	for i := uint64(0); i <= 10; i++ {
		l1Block := eth.BlockRef{
			Hash:   common.HexToHash(fmt.Sprintf("0xaaa%d", i)),
			Number: i,
			Time:   900 + i*12,
		}
		if i > 0 {
			l1Block.ParentHash = l1Blocks[i-1].Hash
		}
		l1Blocks = append(l1Blocks, l1Block)
	}

	// Create L2 genesis block
	blocks = append(blocks, eth.L2BlockRef{
		Hash:           common.HexToHash("0x0000"),
		Number:         0,
		ParentHash:     common.Hash{},
		Time:           1000,
		L1Origin:       l1Blocks[0].ID(),
		SequenceNumber: 0,
	})

	// Create L2 blocks 1-100
	for i := uint64(1); i <= 100; i++ {
		l1Index := i / 10
		blocks = append(blocks, eth.L2BlockRef{
			Hash:           common.HexToHash(fmt.Sprintf("0x%d", i)),
			Number:         i,
			ParentHash:     blocks[i-1].Hash,
			Time:           1000 + i,
			L1Origin:       l1Blocks[l1Index].ID(),
			SequenceNumber: i % 10,
		})
	}

	// Setup sync node with all blocks
	chain.setupSyncNodeBlocks(blocks...)

	// Seal all blocks
	for _, block := range blocks {
		s.sealBlocks(chainID, block)
	}

	// Make blocks up to 95 safe
	for i := uint64(0); i <= 95; i++ {
		l1Index := i / 10
		s.makeBlockSafe(chainID, blocks[i], l1Blocks[l1Index], true)
	}

	// Verify we're at block 100
	s.verifyHead(chainID, blocks[100].ID(), "should be at block 100")

	// Now try to rewind to block 95 (simulating a reorg above that)
	i := New(s.logger, s.chainsDB)
	i.AttachSyncNode(chainID, chain.syncNode)
	require.NoError(t, i.rewindChain(superevents.RewindL2ChainEvent{
		ChainID:        chainID,
		BadBlockHeight: blocks[96].Number,
	}))

	// Verify we rewound to block 95
	s.verifyHead(chainID, blocks[95].ID(), "should have rewound to block 95")
}

// TestRewindMultiChain syncs two chains and rewinds both
func TestRewindMultiChain(t *testing.T) {
	chain1ID := eth.ChainID{1}
	chain2ID := eth.ChainID{2}
	s := setupTestChains(t, chain1ID, chain2ID)
	defer s.Close()

	// Create common blocks for both chains
	genesis, block1, block2A, _ := createTestBlocks()

	// Setup sync nodes for both chains
	for _, chain := range s.chains {
		chain.setupSyncNodeBlocks(genesis, block1, block2A)
		s.sealBlocks(chain.chainID, genesis, block1, block2A)
	}

	// Make block1 safe on both chains
	l1Block1 := eth.BlockRef{
		Hash:   common.HexToHash("0xaaa1"),
		Number: 1,
		Time:   900,
	}

	for chainID := range s.chains {
		s.makeBlockSafe(chainID, block1, l1Block1, true)
	}

	// Create rewinder and attach both chains
	i := New(s.logger, s.chainsDB)
	for chainID, chain := range s.chains {
		i.AttachSyncNode(chainID, chain.syncNode)
	}

	// Rewind both chains
	for chainID := range s.chains {
		require.NoError(t, i.rewindChain(superevents.RewindL2ChainEvent{
			ChainID:        chainID,
			BadBlockHeight: block2A.Number,
		}))
	}

	// Verify both chains rewound to block1 and maintained proper state
	for chainID := range s.chains {
		s.verifyHead(chainID, block1.ID(), fmt.Sprintf("chain %v should have rewound to block1", chainID))
		s.verifyCrossSafe(chainID, block1, fmt.Sprintf("chain %v block1 should be cross-safe", chainID))
	}
}

type testSetup struct {
	t        *testing.T
	logger   log.Logger
	dataDir  string
	chainsDB *db.ChainsDB
	chains   map[eth.ChainID]*testChainSetup
}

type testChainSetup struct {
	chainID  eth.ChainID
	logDB    *logs.DB
	localDB  *fromda.DB
	crossDB  *fromda.DB
	syncNode *mockSyncNode
}

// setupTestChains creates multiple test chains with their own DBs and sync nodes
func setupTestChains(t *testing.T, chainIDs ...eth.ChainID) *testSetup {
	logger := testlog.Logger(t, log.LvlInfo)
	dataDir := t.TempDir()

	// Create dependency set for all chains
	deps := make(map[eth.ChainID]*depset.StaticConfigDependency)
	for i, chainID := range chainIDs {
		deps[chainID] = &depset.StaticConfigDependency{
			ChainIndex:     types.ChainIndex(i + 1),
			ActivationTime: 42,
			HistoryMinTime: 100,
		}
	}
	depSet, err := depset.NewStaticConfigDependencySet(deps)
	require.NoError(t, err)

	// Create ChainsDB with mock emitter
	chainsDB := db.NewChainsDB(logger, depSet)
	chainsDB.AttachEmitter(&mockEmitter{})

	setup := &testSetup{
		t:        t,
		logger:   logger,
		dataDir:  dataDir,
		chainsDB: chainsDB,
		chains:   make(map[eth.ChainID]*testChainSetup),
	}

	// Setup each chain
	for _, chainID := range chainIDs {
		// Create the chain directory
		chainDir := filepath.Join(dataDir, fmt.Sprintf("00%d", chainID[0]), "1")
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

		setup.chains[chainID] = &testChainSetup{
			chainID:  chainID,
			logDB:    logDB,
			localDB:  localDB,
			crossDB:  crossDB,
			syncNode: newMockSyncNode(),
		}
	}

	return setup
}

func (s *testSetup) Close() {
	s.chainsDB.Close()
	for _, chain := range s.chains {
		chain.Close()
	}
}

func (s *testChainSetup) Close() {
	s.logDB.Close()
	s.localDB.Close()
	s.crossDB.Close()
}

// setupSyncNodeBlocks adds the given blocks to the sync node's block map
func (s *testChainSetup) setupSyncNodeBlocks(blocks ...eth.L2BlockRef) {
	for _, block := range blocks {
		s.syncNode.blocks[block.Number] = eth.BlockRef{
			Hash:       block.Hash,
			Number:     block.Number,
			Time:       block.Time,
			ParentHash: block.ParentHash,
		}
	}
}

func (s *testSetup) makeBlockSafe(chainID eth.ChainID, block eth.L2BlockRef, l1Block eth.BlockRef, makeCrossSafe bool) {
	// Add the L1 derivation
	s.chainsDB.UpdateLocalSafe(chainID, l1Block, eth.BlockRef{
		Hash:       block.Hash,
		Number:     block.Number,
		Time:       block.Time,
		ParentHash: block.ParentHash,
	})

	if makeCrossSafe {
		require.NoError(s.t, s.chainsDB.UpdateCrossUnsafe(chainID, types.BlockSeal{
			Hash:      block.Hash,
			Number:    block.Number,
			Timestamp: block.Time,
		}))
		require.NoError(s.t, s.chainsDB.UpdateCrossSafe(chainID, l1Block, eth.BlockRef{
			Hash:       block.Hash,
			Number:     block.Number,
			Time:       block.Time,
			ParentHash: block.ParentHash,
		}))
	}
}

func (s *testSetup) verifyHead(chainID eth.ChainID, expectedHead eth.BlockID, msg string) {
	head, ok := s.chains[chainID].logDB.LatestSealedBlock()
	require.True(s.t, ok)
	require.Equal(s.t, expectedHead, head, msg)
}

func (s *testSetup) verifyCrossSafe(chainID eth.ChainID, block eth.L2BlockRef, msg string) {
	crossSafe, err := s.chainsDB.CrossSafe(chainID)
	require.NoError(s.t, err)
	require.Equal(s.t, block.Hash, crossSafe.Derived.Hash, msg)
}

func (s *testSetup) sealBlocks(chainID eth.ChainID, blocks ...eth.L2BlockRef) {
	for _, block := range blocks {
		require.NoError(s.t, s.chains[chainID].logDB.SealBlock(block.ParentHash, block.ID(), block.Time))
	}
}

func setupTestChain(t *testing.T) *testSetup {
	chainID := eth.ChainID{1}
	return setupTestChains(t, chainID)
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

type mockEmitter struct {
	events []event.Event
}

func (m *mockEmitter) Emit(ev event.Event) {
	m.events = append(m.events, ev)
}

type mockSyncNode struct {
	blocks map[uint64]eth.BlockRef
}

func newMockSyncNode() *mockSyncNode {
	return &mockSyncNode{
		blocks: make(map[uint64]eth.BlockRef),
	}
}

func (m *mockSyncNode) BlockRefByNumber(ctx context.Context, number uint64) (eth.BlockRef, error) {
	return m.blocks[number], nil
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

var _ logs.Metrics = (*stubMetrics)(nil)
