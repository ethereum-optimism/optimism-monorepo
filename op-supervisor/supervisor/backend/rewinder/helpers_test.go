package rewinder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

const (
	l1BlockTime = uint64(12)
	l2BlockTime = uint64(2)
	genesisTime = uint64(1000)
)

type testEmitter struct {
	events []event.Event
}

func (m *testEmitter) Emit(ev event.Event) {
	m.events = append(m.events, ev)
}

// testChain represents a single chain's test data and state
type testChain struct {
	logDB   *logs.DB
	localDB *fromda.DB
	crossDB *fromda.DB
	chainID eth.ChainID
}

func (c *testChain) close() {
	if c.logDB != nil {
		c.logDB.Close()
	}
	if c.localDB != nil {
		c.localDB.Close()
	}
	if c.crossDB != nil {
		c.crossDB.Close()
	}
}

func (c *testChain) sealBlocks(t *testing.T, blocks ...eth.L2BlockRef) {
	for _, block := range blocks {
		require.NoError(t, c.logDB.SealBlock(block.ParentHash, block.ID(), block.Time))
		t.Logf("Sealed block %d (hash: %s, parent: %s) for chain %v", block.Number, block.Hash, block.ParentHash, c.chainID)
	}
}

func (c *testChain) makeBlockSafe(t *testing.T, chainsDB *db.ChainsDB, l2Block eth.L2BlockRef, l1Block eth.BlockRef, makeCrossSafe bool) {
	t.Logf("Making block %d (hash: %s) safe for chain %v with L1 block %d (hash: %s)",
		l2Block.Number, l2Block.Hash, c.chainID, l1Block.Number, l1Block.Hash)

	// Make cross-unsafe
	err := chainsDB.UpdateCrossUnsafe(c.chainID, types.BlockSealFromRef(l2Block.BlockRef()))
	require.NoError(t, err)

	// Make local-safe
	chainsDB.UpdateLocalSafe(c.chainID, l1Block, l2Block.BlockRef())

	// Make cross-safe
	if makeCrossSafe {
		err = chainsDB.UpdateCrossSafe(c.chainID, l1Block, l2Block.BlockRef())
		require.NoError(t, err)
	}
}

func (c *testChain) assertHeads(t *testing.T, chainsDB *db.ChainsDB, expectedLogsHead eth.BlockID, expectedLocalSafe eth.BlockID, expectedCrossSafe eth.BlockID, msg string) {
	// Check logs head
	head, ok := c.logDB.LatestSealedBlock()
	require.True(t, ok)
	require.Equal(t, expectedLogsHead, head, msg+": logs head mismatch")

	// Check local-safe head
	localSafe, err := chainsDB.LocalSafe(c.chainID)
	require.NoError(t, err)
	require.Equal(t, expectedLocalSafe.Hash, localSafe.Derived.Hash, msg+": local-safe head mismatch")

	// Check cross-safe head
	crossSafe, err := chainsDB.CrossSafe(c.chainID)
	require.NoError(t, err)
	require.Equal(t, expectedCrossSafe.Hash, crossSafe.Derived.Hash, msg+": cross-safe head mismatch")
}

func (c *testChain) assertAllHeads(t *testing.T, chainsDB *db.ChainsDB, head eth.BlockID, msg string) {
	c.assertHeads(t, chainsDB, head, head, head, msg)
}

// testCluster represents the entire test environment with multiple chains
type testCluster struct {
	t        *testing.T
	logger   log.Logger
	dataDir  string
	chainsDB *db.ChainsDB
	emitter  *testEmitter
	l1Node   *mockL1Node
	rewinder *Rewinder
	chains   map[eth.ChainID]*testChain
}

func newTestCluster(t *testing.T, chainIDs ...eth.ChainID) *testCluster {
	logger := testlog.Logger(t, log.LvlDebug)
	dataDir := t.TempDir()

	// Create dependency set
	deps := make(map[eth.ChainID]*depset.StaticConfigDependency)
	for i, chainID := range chainIDs {
		deps[chainID] = &depset.StaticConfigDependency{
			ChainIndex:     types.ChainIndex(i + 1), // Start indices at 1
			ActivationTime: 42,
			HistoryMinTime: 100,
		}
	}
	depSet, err := depset.NewStaticConfigDependencySet(deps)
	require.NoError(t, err)

	// Create shared emitter
	sharedEmitter := &testEmitter{}

	// Create ChainsDB with shared emitter and metrics
	chainsDB := db.NewChainsDB(logger, depSet, metrics.NoopMetrics)
	chainsDB.AttachEmitter(sharedEmitter)

	// Create the components
	l1Node := newMockL1Node(t)
	rewinder := New(logger, chainsDB, l1Node)
	rewinder.AttachEmitter(sharedEmitter)

	cluster := &testCluster{
		t:        t,
		logger:   logger,
		dataDir:  dataDir,
		chainsDB: chainsDB,
		emitter:  sharedEmitter,
		l1Node:   l1Node,
		rewinder: rewinder,
		chains:   make(map[eth.ChainID]*testChain),
	}

	// Initialize each chain
	for _, chainID := range chainIDs {
		chain := &testChain{
			chainID: chainID,
		}

		// Create the chain directory
		chainDir := filepath.Join(dataDir, "001", chainID.String())
		err = os.MkdirAll(chainDir, 0o755)
		require.NoError(t, err)
		metrics := &stubDBMetrics{}

		// Create and open the log DB
		chain.logDB, err = logs.NewFromFile(logger, metrics, filepath.Join(chainDir, "log.db"), true)
		require.NoError(t, err)
		chainsDB.AddLogDB(chainID, chain.logDB)

		// Create and open the local derived-from DB
		chain.localDB, err = fromda.NewFromFile(logger, metrics, filepath.Join(chainDir, "local_safe.db"))
		require.NoError(t, err)
		chainsDB.AddLocalDerivationDB(chainID, chain.localDB)

		// Create and open the cross derived-from DB
		chain.crossDB, err = fromda.NewFromFile(logger, metrics, filepath.Join(chainDir, "cross_safe.db"))
		require.NoError(t, err)
		chainsDB.AddCrossDerivationDB(chainID, chain.crossDB)

		// Add cross-unsafe tracker
		chainsDB.AddCrossUnsafeTracker(chainID)

		cluster.chains[chainID] = chain
	}

	return cluster
}

func (c *testCluster) close() {
	c.chainsDB.Close()
	for _, chain := range c.chains {
		chain.close()
	}
}

func (c *testCluster) processEvents() {
	for len(c.emitter.events) > 0 {
		// Dequeue the first event
		ev := c.emitter.events[0]
		c.emitter.events = c.emitter.events[1:]

		// Send the event to our components
		c.logger.Debug("Processing event", "event", ev)
		c.chainsDB.OnEvent(ev)
		c.rewinder.OnEvent(ev)
	}
}

type mockL1Node struct {
	t         *testing.T
	blocks    map[uint64]eth.BlockRef // Stores all blocks including non-canonical ones
	canonical []eth.BlockRef          // Stores only canonical blocks in sequence
}

func newMockL1Node(t *testing.T) *mockL1Node {
	return &mockL1Node{
		t:         t,
		blocks:    make(map[uint64]eth.BlockRef),
		canonical: make([]eth.BlockRef, 0),
	}
}

func (m *mockL1Node) L1BlockRefByNumber(ctx context.Context, number uint64) (eth.L1BlockRef, error) {
	if number >= uint64(len(m.canonical)) {
		return eth.L1BlockRef{}, fmt.Errorf("block %d not found: %w", number, ethereum.NotFound)
	}
	block := m.canonical[number]
	return eth.L1BlockRef{
		Hash:       block.Hash,
		Number:     block.Number,
		Time:       block.Time,
		ParentHash: block.ParentHash,
	}, nil
}

func (m *mockL1Node) reorg(newBlock eth.BlockRef) error {
	m.t.Helper()
	if newBlock.Number >= uint64(len(m.canonical)) {
		m.t.Fatalf("cannot reorg to future block %d, current height is %d", newBlock.Number, len(m.canonical)-1)
	}

	// Verify parent hash matches
	if newBlock.Number > 0 {
		parent := m.canonical[newBlock.Number-1]
		if parent.Hash != newBlock.ParentHash {
			m.t.Fatalf("block %d parent hash %s does not match canonical parent %s", newBlock.Number, newBlock.ParentHash, parent.Hash)
		}
	}

	m.t.Logf("L1: Starting reorg to block %d (hash: %s)", newBlock.Number, newBlock.Hash)
	oldCanonical := make([]eth.BlockRef, len(m.canonical))
	copy(oldCanonical, m.canonical)

	// Remove all blocks after reorg point from blocks map
	for i := newBlock.Number; i < uint64(len(m.canonical)); i++ {
		m.t.Logf("L1: Removing block %d (hash: %s) from blocks map", i, m.blocks[i].Hash)
		delete(m.blocks, i)
	}

	// Store the new block in blocks map
	m.blocks[newBlock.Number] = newBlock

	// Truncate canonical chain at reorg point and add new block
	m.canonical = m.canonical[:newBlock.Number]
	m.canonical = append(m.canonical, newBlock)

	m.t.Logf("L1: Completed reorg from %s to %s at height %d",
		oldCanonical[newBlock.Number].Hash,
		newBlock.Hash,
		newBlock.Number)

	return nil
}

type chainBuilder struct {
	t *testing.T

	l1Node   *mockL1Node
	l2Blocks []eth.L2BlockRef

	// Track number of duplicates at each height
	l1ConflictCount map[uint64]uint64
	l2ConflictCount map[uint64]uint64
}

func newChainBuilder(l1Node *mockL1Node) *chainBuilder {
	return &chainBuilder{
		t:               l1Node.t,
		l1Node:          l1Node,
		l1ConflictCount: make(map[uint64]uint64),
		l2ConflictCount: make(map[uint64]uint64),
	}
}

func (b *chainBuilder) createL1Block(parent eth.BlockRef) eth.BlockRef {
	height := uint64(0)
	blockTime := genesisTime
	if parent != (eth.BlockRef{}) {
		height = parent.Number + 1
		blockTime = parent.Time + l1BlockTime
	}

	block := eth.BlockRef{
		Hash:       newBlockHash(height, b.l1ConflictCount[height]),
		Number:     height,
		Time:       blockTime,
		ParentHash: parent.Hash,
	}

	if height >= uint64(len(b.l1Node.canonical)) {
		b.l1ConflictCount[height]++
		b.l1Node.blocks[block.Number] = block
		b.l1Node.canonical = append(b.l1Node.canonical, block)
		b.t.Logf("L1: Created canonical block %d (hash: %s, parent: %s)", height, block.Hash, parent.Hash)
	}

	return block
}

func (b *chainBuilder) createL2Block(parent eth.L2BlockRef, l1Origin eth.BlockID, seqNum uint64) eth.L2BlockRef {
	height := uint64(0)
	blockTime := genesisTime
	if parent != (eth.L2BlockRef{}) {
		height = parent.Number + 1
		blockTime = parent.Time + l2BlockTime
	}

	block := eth.L2BlockRef{
		Hash:           newBlockHash(height, b.l2ConflictCount[height]),
		Number:         height,
		Time:           blockTime,
		ParentHash:     parent.Hash,
		L1Origin:       l1Origin,
		SequenceNumber: seqNum,
	}

	b.l2ConflictCount[height]++
	b.l2Blocks = append(b.l2Blocks, block)
	b.t.Logf("L2: Created block %d (hash: %s, parent: %s, l1Origin: %s)", height, block.Hash, parent.Hash, l1Origin)
	return block
}

// newBlockHash generates a hash for a block based on its height and variant count
// variant=0 is the canonical chain, variant>0 are conflicting blocks.
// It enables to us to easily differentiate between canonical and conflicting blocks.
func newBlockHash(height uint64, variant uint64) common.Hash {
	letter := rune('a' + variant)
	return common.HexToHash(fmt.Sprintf("0x%s%d", strings.Repeat(string(letter), 2), height))
}

type stubDBMetrics struct{}

func (s *stubDBMetrics) RecordDBEntryCount(_ string, _ int64) {}
func (s *stubDBMetrics) RecordDBSearchEntriesRead(_ int64)    {}
func (s *stubDBMetrics) RecordDBDerivedEntryCount(_ int64)    {}

var _ logs.Metrics = (*stubDBMetrics)(nil)
