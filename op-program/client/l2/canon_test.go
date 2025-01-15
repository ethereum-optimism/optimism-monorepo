package l2

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func TestCanonicalBlockNumberOracle_GetHeaderByNumber(t *testing.T) {
	headBlockNumber := 3
	blockCount := 3
	chainCfg, blocks, oracle := setupOracle(t, blockCount, headBlockNumber, true)
	head := blocks[headBlockNumber].Header()

	blockByHash := func(hash common.Hash) *types.Block {
		return oracle.BlockByHash(hash, chainCfg.ChainID.Uint64())
	}
	canon := NewCanonicalBlockHeaderOracle(head, blockByHash)
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

	// Test eraliest block short-circuiting. Do not expect oracle requests for other blocks.
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

		blockByHash := func(hash common.Hash) *types.Block {
			return oracle.BlockByHash(hash, chainCfg.ChainID.Uint64())
		}
		canon := NewCanonicalBlockHeaderOracle(head, blockByHash)
		oracle.Blocks[blocks[2].Hash()] = blocks[2]
		oracle.Blocks[blocks[1].Hash()] = blocks[1]
		oracle.Blocks[blocks[0].Hash()] = blocks[0]
		h := canon.GetHeaderByNumber(0)
		require.Equal(t, blocks[0].Hash(), h.Hash())

		_, fork, forkOracle := setupOracle(t, blockCount, headBlockNumber, true)

		canon.SetCanonical(fork[2].Header())
		require.Nil(t, canon.GetHeaderByNumber(3))

		forkOracle.Blocks[fork[2].Hash()] = fork[2]
		h = canon.GetHeaderByNumber(2)
		require.Equal(t, fork[2].Hash(), h.Hash())

		forkOracle.Blocks[fork[1].Hash()] = fork[1]
		h = canon.GetHeaderByNumber(1)
		require.Equal(t, fork[1].Hash(), h.Hash())

		// Test eraliest block short-circuiting. Do not expect oracle requests for other blocks.
		oracle.Blocks = map[common.Hash]*types.Block{
			fork[1].Hash(): fork[1],
		}
		require.Equal(t, fork[1].Hash(), canon.GetHeaderByNumber(1).Hash())
	})
	t.Run("set canonical on same chain", func(t *testing.T) {
		chainCfg, blocks, oracle := setupOracle(t, blockCount, headBlockNumber, true)
		head := blocks[headBlockNumber].Header()

		blockByHash := func(hash common.Hash) *types.Block {
			return oracle.BlockByHash(hash, chainCfg.ChainID.Uint64())
		}
		canon := NewCanonicalBlockHeaderOracle(head, blockByHash)
		oracle.Blocks[blocks[2].Hash()] = blocks[2]
		oracle.Blocks[blocks[1].Hash()] = blocks[1]
		oracle.Blocks[blocks[0].Hash()] = blocks[0]
		h := canon.GetHeaderByNumber(0)
		require.Equal(t, blocks[0].Hash(), h.Hash())

		canon.SetCanonical(blocks[2].Header())
		require.Nil(t, canon.GetHeaderByNumber(3))
		// earliest block cache is unchanged.
		oracle.Blocks = map[common.Hash]*types.Block{
			blocks[1].Hash(): blocks[1],
		}
		require.Equal(t, blocks[1].Hash(), canon.GetHeaderByNumber(1).Hash())
	})
}
