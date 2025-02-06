package dsl

import (
	"math/big"

	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/interop/contracts/bindings/inbox"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

type GeneratedTransaction struct {
	t     helpers.Testing
	chain *Chain
	tx    *types.Transaction
}

func NewGeneratedTransaction(t helpers.Testing, chain *Chain, tx *types.Transaction) *GeneratedTransaction {
	return &GeneratedTransaction{
		t:     t,
		chain: chain,
		tx:    tx,
	}
}

func (m *GeneratedTransaction) Identifier() inbox.Identifier {
	rcpt, err := m.chain.SequencerEngine.EthClient().TransactionReceipt(m.t.Ctx(), m.tx.Hash())
	require.NoError(m.t, err)
	block, err := m.chain.SequencerEngine.EthClient().BlockByHash(m.t.Ctx(), rcpt.BlockHash)
	require.NoError(m.t, err)
	require.NotZero(m.t, len(rcpt.Logs), "Transaction did not include any logs to reference")

	return inbox.Identifier{
		Origin:      *m.tx.To(),
		BlockNumber: rcpt.BlockNumber,
		LogIndex:    new(big.Int).SetUint64(uint64(rcpt.Logs[0].Index)),
		Timestamp:   new(big.Int).SetUint64(block.Time()),
		ChainId:     m.chain.RollupCfg.L2ChainID,
	}
}

func (m *GeneratedTransaction) CheckIncluded() {
	rcpt, err := m.chain.SequencerEngine.EthClient().TransactionReceipt(m.t.Ctx(), m.tx.Hash())
	require.NoError(m.t, err)
	require.NotNil(m.t, rcpt)
}

func (m *GeneratedTransaction) CheckNotIncluded() {
	rcpt, err := m.chain.SequencerEngine.EthClient().TransactionReceipt(m.t.Ctx(), m.tx.Hash())
	require.ErrorIs(m.t, err, ethereum.NotFound)
	require.Nil(m.t, rcpt)
}
