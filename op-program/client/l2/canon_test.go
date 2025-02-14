package l2

import (
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func TestCanonicalBlockNumberOracle_GetHeaderByNumber(t *testing.T) {
	headBlockNumber := 3
	blockCount := 3
	chainCfg, blocks, oracle := setupOracle(t, blockCount, headBlockNumber, true)
	head := blocks[headBlockNumber].Header()

	// Ensure we don't walk back from head on every lookup by failing if a block is loaded multiple times.
	requestedBlocks := make(map[common.Hash]bool)
	blockByHash := func(hash common.Hash) *types.Block {
		if requestedBlocks[hash] {
			t.Fatalf("Requested duplicate block: %v", hash)
		}
		requestedBlocks[hash] = true
		return oracle.BlockByHash(hash, eth.ChainIDFromBig(chainCfg.ChainID))
	}
	canon := NewCanonicalBlockHeaderOracle(head, blockByHash)
	require.Equal(t, head.Hash(), canon.CurrentHeader().Hash())
	require.Nil(t, canon.GetHeaderByNumber(4))

	oracle.Blocks[blocks[3].Hash()] = blocks[3]
	h := canon.GetHeaderByNumber(3)
	require.Equal(t, blocks[3].Hash(), h.Hash())

	oracle.Blocks[blocks[2].Hash()] = blocks[2]
	h = canon.GetHeaderByNumber(2)
	require.Equal(t, blocks[2].Hash(), h.Hash())

	oracle.Blocks[blocks[1].Hash()] = blocks[1]
	h = canon.GetHeaderByNumber(1)
	require.Equal(t, blocks[1].Hash(), h.Hash())

	oracle.Blocks[blocks[0].Hash()] = blocks[0]
	h = canon.GetHeaderByNumber(0)
	require.Equal(t, blocks[0].Hash(), h.Hash())

	// Test that the block hash is cached. Do not expect oracle requests for any other blocks.
	// Allow requesting block 1 again as we're specifically asking for it and only the hash is cached
	requestedBlocks[blocks[1].Hash()] = false
	oracle.Blocks = map[common.Hash]*types.Block{
		blocks[1].Hash(): blocks[1],
	}
	require.Equal(t, blocks[1].Hash(), canon.GetHeaderByNumber(1).Hash())
}

func TestCanonicalBlockNumberOracle_SetCanonical(t *testing.T) {
	headBlockNumber := 3
	blockCount := 3

	t.Run("set canonical on fork", func(t *testing.T) {
		chainCfg, blocks, oracle := setupOracle(t, blockCount, headBlockNumber, true)
		head := blocks[headBlockNumber].Header()

		blockRequestCount := 0
		blockByHash := func(hash common.Hash) *types.Block {
			blockRequestCount++
			return oracle.BlockByHash(hash, eth.ChainIDFromBig(chainCfg.ChainID))
		}
		canon := NewCanonicalBlockHeaderOracle(head, blockByHash)
		oracle.Blocks[blocks[2].Hash()] = blocks[2]
		oracle.Blocks[blocks[1].Hash()] = blocks[1]
		oracle.Blocks[blocks[0].Hash()] = blocks[0]
		h := canon.GetHeaderByNumber(0)
		require.Equal(t, blocks[0].Hash(), h.Hash())

		// Create an alternate block 2
		header2b := *blocks[2].Header()
		header2b.Time = header2b.Time + 1
		block2b := types.NewBlockWithHeader(&header2b)
		require.NotEqual(t, blocks[2].Hash(), block2b.Hash())

		oracle.Blocks[block2b.Hash()] = block2b

		canon.SetCanonical(block2b.Header())
		require.Equal(t, block2b.Hash(), canon.CurrentHeader().Hash())
		blockRequestCount = 0
		require.Nil(t, canon.GetHeaderByNumber(3), "Should have removed block 3 from cache")
		require.Equal(t, 0, blockRequestCount, "Should not have needed to fetch a block")

		h = canon.GetHeaderByNumber(2)
		require.Equal(t, block2b.Hash(), h.Hash(), "Should replace block 2 in cache")
		require.Equal(t, 1, blockRequestCount, "Should not have used cache")

		blockRequestCount = 0
		h = canon.GetHeaderByNumber(1)
		require.Equal(t, blocks[1].Hash(), h.Hash(), "Should retain block 1")
		require.Equal(t, 1, blockRequestCount, "Should not have used cache")
	})
	t.Run("set canonical on same chain", func(t *testing.T) {
		chainCfg, blocks, oracle := setupOracle(t, blockCount, headBlockNumber, true)
		head := blocks[headBlockNumber].Header()

		blockByHash := func(hash common.Hash) *types.Block {
			return oracle.BlockByHash(hash, eth.ChainIDFromBig(chainCfg.ChainID))
		}
		canon := NewCanonicalBlockHeaderOracle(head, blockByHash)
		oracle.Blocks[blocks[2].Hash()] = blocks[2]
		oracle.Blocks[blocks[1].Hash()] = blocks[1]
		oracle.Blocks[blocks[0].Hash()] = blocks[0]
		h := canon.GetHeaderByNumber(0)
		require.Equal(t, blocks[0].Hash(), h.Hash())

		canon.SetCanonical(blocks[2].Header())
		require.Equal(t, blocks[2].Hash(), canon.CurrentHeader().Hash())
		require.Nil(t, canon.GetHeaderByNumber(3))
		// earliest block cache is unchanged.
		oracle.Blocks = map[common.Hash]*types.Block{
			blocks[1].Hash(): blocks[1],
		}
		require.Equal(t, blocks[1].Hash(), canon.GetHeaderByNumber(1).Hash())
	})
	t.Run("set canonical with cache reset up to genesis", func(t *testing.T) {
		chainCfg, blocks, oracle := setupOracle(t, blockCount, headBlockNumber, true)
		head := blocks[headBlockNumber].Header()

		blockRequestCount := 0
		blockByHash := func(hash common.Hash) *types.Block {
			blockRequestCount++
			return oracle.BlockByHash(hash, eth.ChainIDFromBig(chainCfg.ChainID))
		}
		canon := NewCanonicalBlockHeaderOracle(head, blockByHash)
		oracle.Blocks[blocks[2].Hash()] = blocks[2]
		oracle.Blocks[blocks[1].Hash()] = blocks[1]
		oracle.Blocks[blocks[0].Hash()] = blocks[0]

		// create a fork at genesis
		header1b := *blocks[1].Header()
		header1b.Time = header1b.Time + 2
		block1b := types.NewBlockWithHeader(&header1b)
		require.NotEqual(t, blocks[1].Hash(), block1b.Hash())
		oracle.Blocks[block1b.Hash()] = block1b

		canon.SetCanonical(block1b.Header())
		blockRequestCount = 0
		require.Equal(t, block1b.Hash(), canon.CurrentHeader().Hash())
		require.Equal(t, 0, blockRequestCount, "Should not have needed to fetch a block")
	})
}
