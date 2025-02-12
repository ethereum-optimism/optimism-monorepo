package l2

import (
	"math/rand"
	"testing"

	"github.com/ethereum-optimism/optimism/op-program/client/l2/test"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

func TestFastCanonBlockHeaderOracle_GetHeaderByNumber(t *testing.T) {
	t.Parallel()

	logger, _ := testlog.CaptureLogger(t, log.LvlInfo)
	miner, backend := test.NewMiner(t, logger, 0)
	stateOracle := &test.KvStateOracle{T: t, Source: backend.TrieDB().Disk()}
	miner.Mine(t, nil)
	miner.Mine(t, nil)
	miner.Mine(t, nil)
	head := backend.CurrentHeader()
	require.Equal(t, uint64(3), head.Number.Uint64())

	// Create invalid fallback to assert that it's never used.
	fatalBlockByHash := func(hash common.Hash) *types.Block {
		t.Fatalf("Unexpected fallback for block: %v", hash)
		return nil
	}
	invalidHeader := testutils.RandomHeader(rand.New(rand.NewSource(12)))
	fallback := NewCanonicalBlockHeaderOracle(invalidHeader, fatalBlockByHash)

	// Ensure we read directly from historical state on every lookup by failing if a block is loaded multiple times.
	requestedBlocks := make(map[common.Hash]bool)
	blockByHash := func(hash common.Hash) *types.Block {
		if requestedBlocks[hash] {
			t.Fatalf("Requested duplicate block: %v", hash)
		}
		requestedBlocks[hash] = true
		return backend.GetBlockByHash(hash)
	}
	canon := NewFastCanonicalBlockHeaderOracle(head, blockByHash, backend.Config(), stateOracle, rawdb.NewMemoryDatabase(), fallback)
	require.Equal(t, head.Hash(), canon.CurrentHeader().Hash())
	require.Nil(t, canon.GetHeaderByNumber(4))

	h := canon.GetHeaderByNumber(3)
	require.Equal(t, backend.GetBlockByNumber(3).Hash(), h.Hash())
	h = canon.GetHeaderByNumber(2)
	require.Equal(t, backend.GetBlockByNumber(2).Hash(), h.Hash())
	h = canon.GetHeaderByNumber(1)
	require.Equal(t, backend.GetBlockByNumber(1).Hash(), h.Hash())
	h = canon.GetHeaderByNumber(0)
	require.Equal(t, backend.GetBlockByNumber(0).Hash(), h.Hash())
}

func TestFastCanonBlockHeaderOracle_LargeWindow(t *testing.T) {
	t.Parallel()

	logger, _ := testlog.CaptureLogger(t, log.LvlInfo)
	miner, backend := test.NewMiner(t, logger, 0)
	stateOracle := &test.KvStateOracle{T: t, Source: backend.TrieDB().Disk()}
	numBlocks := 16384 // params.HistoryServeWindow * 2
	for i := 0; i < numBlocks; i++ {
		miner.Mine(t, nil)
	}
	head := backend.CurrentHeader()
	headNum := head.Number.Uint64()
	require.Equal(t, uint64(numBlocks), headNum)
	// Note: we have three non-overlapping historical block windows
	// head: [8193, 16383]
	// 8193: [2, 8192]
	// 2:    [0, 1]

	// Create invalid fallback to assert that it's never used.
	fatalBlockByHash := func(hash common.Hash) *types.Block {
		t.Fatalf("Unexpected fallback for block: %v", hash)
		return nil
	}
	invalidHeader := testutils.RandomHeader(rand.New(rand.NewSource(12)))
	fallback := NewCanonicalBlockHeaderOracle(invalidHeader, fatalBlockByHash)

	tracker := newTrackingBlockByHash(backend.GetBlockByHash)
	canon := NewFastCanonicalBlockHeaderOracle(head, tracker.BlockByHash, backend.Config(), stateOracle, rawdb.NewMemoryDatabase(), fallback)
	require.Equal(t, head.Hash(), canon.CurrentHeader().Hash())
	require.Nil(t, canon.GetHeaderByNumber(headNum+1))

	h := canon.GetHeaderByNumber(headNum)
	require.Equal(t, backend.GetBlockByNumber(headNum).Hash(), h.Hash())

	for i := int(headNum - 1); i >= 0; i-- {
		expect := backend.GetBlockByNumber(uint64(i)).Hash()
		h = canon.GetHeaderByNumber(uint64(i))
		require.Equal(t, expect, h.Hash())
		// Since we're iterating backwards, we will fetch exactly one block from the oracle.
		// Because, other than the historical window at head, all other canonical queries will short-circuit to a cached historical block.
		require.Equalf(t, 1, tracker.requests[expect], "Unexpected number of requests for block: %v (%d)", expect, i)
	}

	runCanonicalCacheTest(t, backend, 0, 3)
	runCanonicalCacheTest(t, backend, 1, 3)
	runCanonicalCacheTest(t, backend, 2, 2)
	runCanonicalCacheTest(t, backend, 3, 2)
	runCanonicalCacheTest(t, backend, 4, 2)
	runCanonicalCacheTest(t, backend, 8191, 2)
	runCanonicalCacheTest(t, backend, 8192, 2)
	runCanonicalCacheTest(t, backend, 8193, 1)
	runCanonicalCacheTest(t, backend, 16382, 1)
	runCanonicalCacheTest(t, backend, 16383, 1)
}

func TestFastCannonBlockHeaderOracle_WithFallback(t *testing.T) {
	t.Parallel()

	logger, _ := testlog.CaptureLogger(t, log.LvlInfo)
	isthmusTime := uint64(4)
	isthmusBlockActivation := 2 // isthmusTime / blockTime
	miner, backend := test.NewMiner(t, logger, isthmusTime)
	stateOracle := &test.KvStateOracle{T: t, Source: backend.TrieDB().Disk()}
	numBlocks := 5
	for i := 0; i < numBlocks; i++ {
		miner.Mine(t, nil)
	}
	head := backend.CurrentHeader()
	headNum := head.Number.Uint64()
	require.Equal(t, uint64(numBlocks), headNum)

	fallbackBlockByHash := newTrackingBlockByHash(backend.GetBlockByHash)
	fallback := NewCanonicalBlockHeaderOracle(head, fallbackBlockByHash.BlockByHash)
	canon := NewFastCanonicalBlockHeaderOracle(head, backend.GetBlockByHash, backend.Config(), stateOracle, rawdb.NewMemoryDatabase(), fallback)

	for i := 0; i <= int(isthmusBlockActivation); i++ {
		i := uint64(i)
		expected := backend.GetBlockByNumber(i).Hash()
		require.Equalf(t, expected, canon.GetHeaderByNumber(i).Hash(), "Expected block %d to be canonical", i)
		require.Equalf(t, 1, fallbackBlockByHash.requests[expected], "Expected 1 fallback request for block %d", i)
	}
	fallbackBlockByHash.requests = make(map[common.Hash]int)
	for i := int(isthmusBlockActivation) + 1; i < numBlocks; i++ {
		i := uint64(i)
		expected := backend.GetBlockByNumber(i).Hash()
		require.Equalf(t, expected, canon.GetHeaderByNumber(i).Hash(), "Expected block %d to be canonical", i)
		require.Equalf(t, 0, fallbackBlockByHash.requests[expected], "Expected 0 fallback requests for block %d", i)
	}
}

func TestFastCanonBlockHeaderOracle_PreIsthmus(t *testing.T) {
	t.Parallel()

	logger, _ := testlog.CaptureLogger(t, log.LvlInfo)
	isthmusTime := uint64(4)
	isthmusBlockActivation := 2 // isthmusTime / blockTime
	miner, backend := test.NewMiner(t, logger, isthmusTime)
	stateOracle := &test.KvStateOracle{T: t, Source: backend.TrieDB().Disk()}
	numBlocks := 5
	for i := 0; i < numBlocks; i++ {
		miner.Mine(t, nil)
	}
	head := backend.CurrentHeader()
	headNum := head.Number.Uint64()
	require.Equal(t, uint64(numBlocks), headNum)
	preIsthmusHead := backend.GetBlockByNumber(uint64(isthmusBlockActivation) - 1).Header()

	fallbackBlockByHash := newTrackingBlockByHash(backend.GetBlockByHash)
	fallback := NewCanonicalBlockHeaderOracle(preIsthmusHead, fallbackBlockByHash.BlockByHash)
	canon := NewFastCanonicalBlockHeaderOracle(preIsthmusHead, backend.GetBlockByHash, backend.Config(), stateOracle, rawdb.NewMemoryDatabase(), fallback)

	for i := uint64(0); i <= preIsthmusHead.Number.Uint64(); i++ {
		head := canon.GetHeaderByNumber(i)
		require.Equal(t, backend.GetBlockByNumber(i).Hash(), head.Hash())
	}
}

func TestFastCanonBlockHeaderOracle_SetCanonical(t *testing.T) {
	t.Parallel()

	t.Run("rollback", func(t *testing.T) {
		t.Parallel()
		logger, _ := testlog.CaptureLogger(t, log.LvlInfo)
		miner, backend := test.NewMiner(t, logger, 0)
		stateOracle := &test.KvStateOracle{T: t, Source: backend.TrieDB().Disk()}
		numBlocks := 5
		for i := 0; i < numBlocks; i++ {
			miner.Mine(t, nil)
		}
		head := backend.CurrentHeader()
		fallback := NewCanonicalBlockHeaderOracle(head, backend.GetBlockByHash)
		canon := NewFastCanonicalBlockHeaderOracle(head, backend.GetBlockByHash, backend.Config(), stateOracle, rawdb.NewMemoryDatabase(), fallback)
		canon.SetCanonical(head)
		require.Equal(t, head.Hash(), canon.CurrentHeader().Hash())
		require.Equal(t, head.Hash(), fallback.CurrentHeader().Hash())

		parent := backend.GetBlockByNumber(head.Number.Uint64() - 1)
		canon.SetCanonical(parent.Header())
		require.Nil(t, canon.GetHeaderByNumber(head.Number.Uint64()))
		require.Equal(t, parent.Hash(), canon.CurrentHeader().Hash())
		require.Equal(t, parent.Hash(), fallback.CurrentHeader().Hash())
	})

	t.Run("fork", func(t *testing.T) {
		t.Parallel()
		logger, _ := testlog.CaptureLogger(t, log.LvlInfo)
		miner, backend := test.NewMiner(t, logger, 0)
		stateOracle := &test.KvStateOracle{T: t, Source: backend.TrieDB().Disk()}
		numBlocks := uint64(16384) // params.HistoryServeWindow * 2
		for i := uint64(0); i < numBlocks; i++ {
			miner.Mine(t, nil)
		}
		head := backend.CurrentHeader()
		headNum := head.Number.Uint64()

		fallback := NewCanonicalBlockHeaderOracle(head, backend.GetBlockByHash)
		tracker := newTrackingBlockByHash(backend.GetBlockByHash)
		canon := NewFastCanonicalBlockHeaderOracle(head, tracker.BlockByHash, backend.Config(), stateOracle, rawdb.NewMemoryDatabase(), fallback)
		for i := uint64(0); i <= headNum; i++ {
			// prime the cache
			canon.GetHeaderByNumber(i)
		}

		forkBlockNumber := uint64(7000)
		miner.Fork(t, forkBlockNumber, nil)
		for i := forkBlockNumber + 1; i < numBlocks; i++ {
			miner.Mine(t, nil)
		}
		forkHead := backend.CurrentHeader()
		require.NotEqual(t, head.Hash(), forkHead.Hash())
		require.Equal(t, numBlocks, forkHead.Number.Uint64())

		newCanonHeadNumber := uint64(9000)
		canon.SetCanonical(backend.GetBlockByNumber(newCanonHeadNumber).Header())

		require.Nil(t, canon.GetHeaderByNumber(newCanonHeadNumber+1))
		for i := uint64(0); i <= newCanonHeadNumber; i++ {
			expect := backend.GetBlockByNumber(i).Hash()
			h := canon.GetHeaderByNumber(i)
			require.Equalf(t, expect, h.Hash(), "Unexpected block hash for block: %d", i)
		}
	})
}

// runCanonicalCacheTest asserts the number of oracle requests made for a given block number.
// It also asserts that the retrieved block hash at the specified height is canonical
func runCanonicalCacheTest(t *testing.T, backend *core.BlockChain, blockNum uint64, expectedNumRequests int) {
	head := backend.CurrentHeader()
	tracker := newTrackingBlockByHash(backend.GetBlockByHash)
	stateOracle := &test.KvStateOracle{T: t, Source: backend.TrieDB().Disk()}
	// Create invalid fallback to assert that it's never used.
	fatalBlockByHash := func(hash common.Hash) *types.Block {
		t.Fatalf("Unexpected fallback for block: %v", hash)
		return nil
	}
	invalidHeader := testutils.RandomHeader(rand.New(rand.NewSource(12)))
	fallback := NewCanonicalBlockHeaderOracle(invalidHeader, fatalBlockByHash)
	canon := NewFastCanonicalBlockHeaderOracle(head, tracker.BlockByHash, backend.Config(), stateOracle, rawdb.NewMemoryDatabase(), fallback)

	expect := backend.GetBlockByNumber(blockNum).Hash()
	h := canon.GetHeaderByNumber(blockNum)
	require.Equal(t, expect, h.Hash())
	require.Equalf(t, expectedNumRequests, tracker.numRequests, "Unexpected number of requests for block: %v (%d)", expect, blockNum)

	// query again and assert that it's cached
	tracker.numRequests = 0
	h = canon.GetHeaderByNumber(blockNum)
	require.Equal(t, expect, h.Hash())
	require.Equalf(t, 1, tracker.numRequests, "Unexpected number of requests for block: %v (%d)", expect, blockNum)
}

type trackingBlockByHash struct {
	fn          BlockByHashFn
	numRequests int
	requests    map[common.Hash]int
}

func newTrackingBlockByHash(fn BlockByHashFn) *trackingBlockByHash {
	return &trackingBlockByHash{
		fn:       fn,
		requests: make(map[common.Hash]int),
	}
}

func (o *trackingBlockByHash) BlockByHash(hash common.Hash) *types.Block {
	o.numRequests += 1
	o.requests[hash] += 1
	return o.fn(hash)
}
