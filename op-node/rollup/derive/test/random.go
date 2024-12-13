package test

import (
	"math/big"
	"math/rand"
	"time"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/trie"
)

// RandomL2Block returns a random block whose first transaction is a random pre-Ecotone upgrade
// L1 Info Deposit transaction.
func RandomL2Block(rng *rand.Rand, txCount int, t time.Time) (*types.Block, []*types.Receipt) {
	body := types.Body{}
	l1Block := types.NewBlock(testutils.RandomHeader(rng), &body, nil, trie.NewStackTrie(nil), types.DefaultBlockConfig)
	rollupCfg := rollup.Config{}
	if testutils.RandomBool(rng) {
		t := uint64(0)
		rollupCfg.RegolithTime = &t
	}
	l1InfoTx, err := derive.L1InfoDeposit(&rollupCfg, eth.SystemConfig{}, 0, eth.BlockToInfo(l1Block), 0)
	if err != nil {
		panic("L1InfoDeposit: " + err.Error())
	}
	if t.IsZero() {
		return testutils.RandomBlockPrependTxs(rng, txCount, types.NewTx(l1InfoTx))
	} else {
		return testutils.RandomBlockPrependTxsWithTime(rng, txCount, uint64(t.Unix()), types.NewTx(l1InfoTx))
	}

}

func RandomL2BlockWithChainId(rng *rand.Rand, txCount int, chainId *big.Int) *types.Block {
	return RandomL2BlockWithChainIdAndTime(rng, txCount, chainId, time.Time{})
}

func RandomL2BlockWithChainIdAndTime(rng *rand.Rand, txCount int, chainId *big.Int, t time.Time) *types.Block {
	signer := types.NewLondonSigner(chainId)
	block, _ := RandomL2Block(rng, 0, t)
	txs := []*types.Transaction{block.Transactions()[0]} // L1 info deposit TX
	for i := 0; i < txCount; i++ {
		txs = append(txs, testutils.RandomTx(rng, big.NewInt(int64(rng.Uint32())), signer))
	}
	return block.WithBody(types.Body{Transactions: txs})
}
