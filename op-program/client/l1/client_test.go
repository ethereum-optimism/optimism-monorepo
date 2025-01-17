package l1

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-program/client/l1/test"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
)

var _ derive.L1Fetcher = (*OracleL1Client)(nil)

var head = blockNum(1000)

func TestInfoByHash(t *testing.T) {
	client, oracle := newClient(t)
	hash := common.HexToHash("0xAABBCC")
	expected := &testutils.MockBlockInfo{}
	oracle.Blocks[hash] = expected

	info, err := client.InfoByHash(context.Background(), hash)
	require.NoError(t, err)
	require.Equal(t, expected, info)
}

func TestL1BlockRefByHash(t *testing.T) {
	client, oracle := newClient(t)
	hash := common.HexToHash("0xAABBCC")
	header := &testutils.MockBlockInfo{}
	oracle.Blocks[hash] = header
	expected := eth.InfoToL1BlockRef(header)

	ref, err := client.L1BlockRefByHash(context.Background(), hash)
	require.NoError(t, err)
	require.Equal(t, expected, ref)
}

func TestFetchReceipts(t *testing.T) {
	client, oracle := newClient(t)
	hash := common.HexToHash("0xAABBCC")
	expectedInfo := &testutils.MockBlockInfo{}
	expectedReceipts := types.Receipts{
		&types.Receipt{},
	}
	oracle.Blocks[hash] = expectedInfo
	oracle.Rcpts[hash] = expectedReceipts

	info, rcpts, err := client.FetchReceipts(context.Background(), hash)
	require.NoError(t, err)
	require.Equal(t, expectedInfo, info)
	require.Equal(t, expectedReceipts, rcpts)
}

func TestInfoAndTxsByHash(t *testing.T) {
	client, oracle := newClient(t)
	hash := common.HexToHash("0xAABBCC")
	expectedInfo := &testutils.MockBlockInfo{}
	expectedTxs := make([]eth.GenericTx, 1)
	oracle.Blocks[hash] = expectedInfo
	oracle.Txs[hash] = expectedTxs

	info, txs, err := client.InfoAndTxsByHash(context.Background(), hash)
	require.NoError(t, err)
	require.Equal(t, expectedInfo, info)
	require.Equal(t, expectedTxs, txs)
}

func TestL1BlockRefByLabel(t *testing.T) {
	t.Run("Unsafe", func(t *testing.T) {
		client, _ := newClient(t)
		ref, err := client.L1BlockRefByLabel(context.Background(), eth.Unsafe)
		require.NoError(t, err)
		require.Equal(t, eth.InfoToL1BlockRef(head), ref)
	})
	t.Run("Safe", func(t *testing.T) {
		client, _ := newClient(t)
		ref, err := client.L1BlockRefByLabel(context.Background(), eth.Safe)
		require.NoError(t, err)
		require.Equal(t, eth.InfoToL1BlockRef(head), ref)
	})
	t.Run("Finalized", func(t *testing.T) {
		client, _ := newClient(t)
		ref, err := client.L1BlockRefByLabel(context.Background(), eth.Finalized)
		require.NoError(t, err)
		require.Equal(t, eth.InfoToL1BlockRef(head), ref)
	})
	t.Run("UnknownLabel", func(t *testing.T) {
		client, _ := newClient(t)
		ref, err := client.L1BlockRefByLabel(context.Background(), eth.BlockLabel("unknown"))
		require.ErrorIs(t, err, ErrUnknownLabel)
		require.Equal(t, eth.L1BlockRef{}, ref)
	})
}

func TestL1BlockRefByNumber(t *testing.T) {
	t.Run("Head", func(t *testing.T) {
		client, _ := newClient(t)
		ref, err := client.L1BlockRefByNumber(context.Background(), head.NumberU64())
		require.NoError(t, err)
		require.Equal(t, eth.InfoToL1BlockRef(head), ref)
	})
	t.Run("AfterHead", func(t *testing.T) {
		client, _ := newClient(t)
		ref, err := client.L1BlockRefByNumber(context.Background(), head.NumberU64()+1)
		// Must be ethereum.NotFound error so the derivation pipeline knows it has gone past the chain head
		require.ErrorIs(t, err, ethereum.NotFound)
		require.Equal(t, eth.L1BlockRef{}, ref)
	})
	t.Run("ParentOfHead", func(t *testing.T) {
		client, oracle := newClient(t)
		parent := blockNum(head.NumberU64() - 1)
		oracle.Blocks[parent.Hash()] = parent

		ref, err := client.L1BlockRefByNumber(context.Background(), parent.NumberU64())
		require.NoError(t, err)
		require.Equal(t, eth.InfoToL1BlockRef(parent), ref)
	})
	createBlocks := func(oracle *test.StubOracle) []eth.BlockInfo {
		block := head
		blocks := []eth.BlockInfo{block}
		for i := 0; i < 10; i++ {
			block = blockNum(block.NumberU64() - 1)
			oracle.Blocks[block.Hash()] = block
			blocks = append(blocks, block)
		}
		return blocks
	}
	t.Run("AncestorsAccessForwards", func(t *testing.T) {
		client, oracle := newClient(t)
		blocks := createBlocks(oracle)

		for _, block := range blocks {
			ref, err := client.L1BlockRefByNumber(context.Background(), block.NumberU64())
			require.NoError(t, err)
			require.Equal(t, eth.InfoToL1BlockRef(block), ref)
		}
	})
	t.Run("AncestorsAccessReverse", func(t *testing.T) {
		client, oracle := newClient(t)
		blocks := createBlocks(oracle)

		for i := len(blocks) - 1; i >= 0; i-- {
			block := blocks[i]
			ref, err := client.L1BlockRefByNumber(context.Background(), block.NumberU64())
			require.NoError(t, err)
			require.Equal(t, eth.InfoToL1BlockRef(block), ref)
		}
	})
}

func newClient(t *testing.T) (*OracleL1Client, *test.StubOracle) {
	stub := test.NewStubOracle(t)
	stub.Blocks[head.Hash()] = head
	client := NewOracleL1Client(testlog.Logger(t, log.LevelDebug), stub, head.Hash())
	return client, stub
}

func blockNum(num uint64) eth.BlockInfo {
	parentNum := num - 1
	return &testutils.MockBlockInfo{
		InfoHash:        common.BytesToHash(big.NewInt(int64(num)).Bytes()),
		InfoParentHash:  common.BytesToHash(big.NewInt(int64(parentNum)).Bytes()),
		InfoCoinbase:    common.Address{},
		InfoRoot:        common.Hash{},
		InfoNum:         num,
		InfoTime:        num * 2,
		InfoMixDigest:   [32]byte{},
		InfoBaseFee:     nil,
		InfoReceiptRoot: common.Hash{},
		InfoGasUsed:     0,
	}
}
