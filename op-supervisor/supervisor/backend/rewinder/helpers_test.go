package rewinder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-supervisor/metrics"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db/fromda"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db/logs"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type testSetup struct {
	t        *testing.T
	logger   log.Logger
	dataDir  string
	chainsDB *db.ChainsDB
	chains   map[eth.ChainID]*testChainSetup
}

type testChainSetup struct {
	chainID eth.ChainID
	logDB   *logs.DB
	localDB *fromda.DB
	crossDB *fromda.DB
	l1Node  *mockL1Node
}

// setupTestChains creates multiple test chains with their own DBs and sync nodes
func setupTestChains(t *testing.T, chainIDs ...eth.ChainID) *testSetup {
	// logger := testlog.Logger(t, log.LvlInfo)
	logger := testlog.Logger(t, log.LvlDebug)
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
	chainsDB := db.NewChainsDB(logger, depSet, metrics.NoopMetrics)
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
			chainID: chainID,
			logDB:   logDB,
			localDB: localDB,
			crossDB: crossDB,
			l1Node:  newMockL1Node(t),
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

func (s *testSetup) makeBlockSafe(chainID eth.ChainID, block eth.L2BlockRef, l1Block eth.BlockRef, makeCrossSafe bool) {
	// Add the L1 derivation
	s.chainsDB.UpdateLocalSafe(chainID, l1Block, eth.BlockRef{
		Hash:       block.Hash,
		Number:     block.Number,
		Time:       block.Time,
		ParentHash: block.ParentHash,
	})

	err := s.chainsDB.UpdateCrossUnsafe(chainID, types.BlockSeal{
		Hash:      block.Hash,
		Number:    block.Number,
		Timestamp: block.Time,
	})
	require.NoError(s.t, err)

	if makeCrossSafe {
		err = s.chainsDB.UpdateCrossSafe(chainID, l1Block, eth.BlockRef{
			Hash:       block.Hash,
			Number:     block.Number,
			Time:       block.Time,
			ParentHash: block.ParentHash,
		})
		require.NoError(s.t, err)
	}
}

func (s *testSetup) verifyHeads(chainID eth.ChainID, expectedHead eth.BlockID, msg string) {
	s.verifyLocalSafe(chainID, expectedHead, msg)
	s.verifyCrossSafe(chainID, expectedHead, msg)
}

func (s *testSetup) verifyLocalSafe(chainID eth.ChainID, expectedHead eth.BlockID, msg string) {
	localSafe, err := s.chainsDB.LocalSafe(chainID)
	require.NoError(s.t, err)
	require.Equal(s.t, expectedHead.Hash, localSafe.Derived.Hash, msg)
}

func (s *testSetup) verifyCrossSafe(chainID eth.ChainID, expectedHead eth.BlockID, msg string) {
	crossSafe, err := s.chainsDB.CrossSafe(chainID)
	require.NoError(s.t, err)
	require.Equal(s.t, expectedHead.Hash, crossSafe.Derived.Hash, msg)
}

func (s *testSetup) verifyLogsHead(chainID eth.ChainID, expectedHead eth.BlockID, msg string) {
	head, ok := s.chains[chainID].logDB.LatestSealedBlock()
	require.True(s.t, ok)
	require.Equal(s.t, expectedHead, head, msg)
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

func createTestBlocks() (genesis, block1, block2, block3A, block3B eth.L2BlockRef) {
	genesis = eth.L2BlockRef{
		Hash:           common.HexToHash("0x1110"),
		Number:         0,
		ParentHash:     common.Hash{},
		Time:           1000,
		L1Origin:       eth.BlockID{Hash: common.HexToHash("0xaaa0"), Number: 0},
		SequenceNumber: 0,
	}
	block1 = eth.L2BlockRef{
		Hash:           common.HexToHash("0x1111"),
		Number:         1,
		ParentHash:     genesis.Hash,
		Time:           1001,
		L1Origin:       eth.BlockID{Hash: common.HexToHash("0xaaa1"), Number: 1},
		SequenceNumber: 1,
	}
	block2 = eth.L2BlockRef{
		Hash:           common.HexToHash("0x1112"),
		Number:         2,
		ParentHash:     block1.Hash,
		Time:           1002,
		L1Origin:       eth.BlockID{Hash: common.HexToHash("0xaaa2"), Number: 2},
		SequenceNumber: 2,
	}
	block3A = eth.L2BlockRef{
		Hash:           common.HexToHash("0x1113a"),
		Number:         3,
		ParentHash:     block2.Hash,
		Time:           1003,
		L1Origin:       eth.BlockID{Hash: common.HexToHash("0xaaa3"), Number: 3},
		SequenceNumber: 3,
	}
	block3B = eth.L2BlockRef{
		Hash:           common.HexToHash("0x1113b"),
		Number:         3,
		ParentHash:     block2.Hash,
		Time:           1003,
		L1Origin:       eth.BlockID{Hash: common.HexToHash("0xbbb3"), Number: 3},
		SequenceNumber: 3,
	}
	return
}

// Mock implementations

type mockEmitter struct {
	events []event.Event
}

func (m *mockEmitter) Emit(ev event.Event) {
	m.events = append(m.events, ev)
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

type mockL1Node struct {
	t      *testing.T
	blocks map[uint64]eth.BlockRef
}

func newMockL1Node(t *testing.T) *mockL1Node {
	return &mockL1Node{
		t:      t,
		blocks: make(map[uint64]eth.BlockRef),
	}
}

// AddBlock adds a new block to the L1 chain. It asserts that the block doesn't already exist.
func (m *mockL1Node) AddBlock(block eth.BlockRef) {
	m.t.Helper()
	_, exists := m.blocks[block.Number]
	require.False(m.t, exists, "block %d already exists", block.Number)
	m.blocks[block.Number] = block
}

// ReorgToBlock replaces the block at the given height with a new block and removes all subsequent blocks.
// It asserts that the block height already exists.
func (m *mockL1Node) ReorgToBlock(newBlock eth.BlockRef) {
	m.t.Helper()
	_, exists := m.blocks[newBlock.Number]
	require.True(m.t, exists, "block %d does not exist", newBlock.Number)

	// First remove all blocks after this height
	for height := range m.blocks {
		if height > newBlock.Number {
			delete(m.blocks, height)
		}
	}
	// Then add the new block
	m.blocks[newBlock.Number] = newBlock
}

func (m *mockL1Node) L1BlockRefByNumber(ctx context.Context, number uint64) (eth.L1BlockRef, error) {
	block, ok := m.blocks[number]
	if !ok {
		return eth.L1BlockRef{}, fmt.Errorf("block %d not found: %w", number, ethereum.NotFound)
	}
	return eth.L1BlockRef{
		Hash:       block.Hash,
		Number:     block.Number,
		Time:       block.Time,
		ParentHash: block.ParentHash,
	}, nil
}

// chainBuilder helps construct test chains with proper L1/L2 relationships
type chainBuilder struct {
	l1Blocks []eth.BlockRef
	l2Blocks []eth.L2BlockRef
}

func newChainBuilder() *chainBuilder {
	return &chainBuilder{}
}

// AddL1Block adds a new L1 block to the chain
func (b *chainBuilder) AddL1Block(hash common.Hash, number uint64, time uint64) eth.BlockRef {
	block := eth.BlockRef{
		Hash:   hash,
		Number: number,
		Time:   time,
	}
	if len(b.l1Blocks) > 0 {
		block.ParentHash = b.l1Blocks[len(b.l1Blocks)-1].Hash
	}
	b.l1Blocks = append(b.l1Blocks, block)
	return block
}

// AddL2Block adds a new L2 block derived from the given L1 block
func (b *chainBuilder) AddL2Block(hash common.Hash, number uint64, time uint64, l1Origin eth.BlockID, seqNum uint64) eth.L2BlockRef {
	block := eth.L2BlockRef{
		Hash:           hash,
		Number:         number,
		Time:           time,
		L1Origin:       l1Origin,
		SequenceNumber: seqNum,
	}
	if len(b.l2Blocks) > 0 {
		block.ParentHash = b.l2Blocks[len(b.l2Blocks)-1].Hash
	}
	b.l2Blocks = append(b.l2Blocks, block)
	return block
}

// AddL2Blocks adds multiple L2 blocks derived from the same L1 block
func (b *chainBuilder) AddL2Blocks(l1Block eth.BlockRef, count int) []eth.L2BlockRef {
	var blocks []eth.L2BlockRef
	startNum := uint64(0)
	if len(b.l2Blocks) > 0 {
		startNum = b.l2Blocks[len(b.l2Blocks)-1].Number + 1
	}
	startTime := l1Block.Time

	for i := 0; i < count; i++ {
		num := startNum + uint64(i)
		hash := common.HexToHash(fmt.Sprintf("0x%d", num))
		block := b.AddL2Block(hash, num, startTime+uint64(i), l1Block.ID(), uint64(i))
		blocks = append(blocks, block)
	}
	return blocks
}

// L1Head returns the current L1 head block
func (b *chainBuilder) L1Head() eth.BlockRef {
	if len(b.l1Blocks) == 0 {
		return eth.BlockRef{}
	}
	return b.l1Blocks[len(b.l1Blocks)-1]
}

// L2Head returns the current L2 head block
func (b *chainBuilder) L2Head() eth.L2BlockRef {
	if len(b.l2Blocks) == 0 {
		return eth.L2BlockRef{}
	}
	return b.l2Blocks[len(b.l2Blocks)-1]
}

// SetupL1Node configures the L1 node with all L1 blocks
func (b *chainBuilder) SetupL1Node(node *mockL1Node) {
	for _, block := range b.l1Blocks {
		node.AddBlock(block)
	}
}

// stateVerifier helps verify chain state
type stateVerifier struct {
	t        *testing.T
	chainsDB *db.ChainsDB
	chains   map[eth.ChainID]*testChainSetup
}

func newStateVerifier(t *testing.T, chainsDB *db.ChainsDB, chains map[eth.ChainID]*testChainSetup) *stateVerifier {
	return &stateVerifier{
		t:        t,
		chainsDB: chainsDB,
		chains:   chains,
	}
}

// VerifyChainState verifies the complete state of a chain
func (v *stateVerifier) VerifyChainState(chainID eth.ChainID, state ChainState) {
	v.t.Helper()
	v.VerifyLocalSafe(chainID, state.LocalSafe)
	v.VerifyCrossSafe(chainID, state.CrossSafe)
	v.VerifyLogsHead(chainID, state.LogsHead)
}

// VerifyLocalSafe verifies the local-safe head of a chain
func (v *stateVerifier) VerifyLocalSafe(chainID eth.ChainID, expected eth.BlockID) {
	v.t.Helper()
	localSafe, err := v.chainsDB.LocalSafe(chainID)
	require.NoError(v.t, err)
	require.Equal(v.t, expected.Hash, localSafe.Derived.Hash, "local-safe head mismatch")
	require.Equal(v.t, expected.Number, localSafe.Derived.Number, "local-safe head number mismatch")
}

// VerifyCrossSafe verifies the cross-safe head of a chain
func (v *stateVerifier) VerifyCrossSafe(chainID eth.ChainID, expected eth.BlockID) {
	v.t.Helper()
	crossSafe, err := v.chainsDB.CrossSafe(chainID)
	require.NoError(v.t, err)
	require.Equal(v.t, expected.Hash, crossSafe.Derived.Hash, "cross-safe head mismatch")
	require.Equal(v.t, expected.Number, crossSafe.Derived.Number, "cross-safe head number mismatch")
}

// VerifyLogsHead verifies the latest sealed block in the logs DB
func (v *stateVerifier) VerifyLogsHead(chainID eth.ChainID, expected eth.BlockID) {
	v.t.Helper()
	head, ok := v.chains[chainID].logDB.LatestSealedBlock()
	require.True(v.t, ok)
	require.Equal(v.t, expected.Hash, head.Hash, "logs head mismatch")
	require.Equal(v.t, expected.Number, head.Number, "logs head number mismatch")
}

// ChainState represents the expected state of a chain
type ChainState struct {
	LocalSafe eth.BlockID
	CrossSafe eth.BlockID
	LogsHead  eth.BlockID
}

// SetupGenesisOnlyChain creates a test chain with only a genesis block
func SetupGenesisOnlyChain(t *testing.T) (*testSetup, eth.L2BlockRef) {
	s := setupTestChain(t)
	chainID := eth.ChainID{1}
	chain := s.chains[chainID]

	// Create genesis block
	genesis := eth.L2BlockRef{
		Hash:           common.HexToHash("0x1110"),
		Number:         0,
		ParentHash:     common.Hash{},
		Time:           1000,
		L1Origin:       eth.BlockID{Hash: common.HexToHash("0xaaa0"), Number: 0},
		SequenceNumber: 0,
	}

	// Setup L1 genesis block
	l1Genesis := eth.BlockRef{
		Hash:   common.HexToHash("0xaaa0"),
		Number: 0,
		Time:   900,
	}
	chain.l1Node.AddBlock(l1Genesis)

	// Seal genesis block
	s.sealBlocks(chainID, genesis)

	// Make genesis block safe and derived from L1 genesis
	s.makeBlockSafe(chainID, genesis, l1Genesis, true)

	return s, genesis
}

// Helper functions to generate deterministic hashes for testing
func hash(n uint64) common.Hash {
	return common.HexToHash(fmt.Sprintf("0x%d", n))
}

func hashA(n uint64) common.Hash {
	return common.HexToHash(fmt.Sprintf("0xaaa%d", n))
}

func hashB(n uint64) common.Hash {
	return common.HexToHash(fmt.Sprintf("0xbbb%d", n))
}
