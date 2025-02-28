package dsl

import (
	"math/big"

	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/interop/contracts/bindings/inbox"
	stypes "github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

type TxIncluder interface {
	IncludeTx(transaction *types.Transaction, from common.Address) (*types.Receipt, error)
}
type GeneratedTransaction struct {
	t     helpers.Testing
	chain *Chain
	tx    *types.Transaction
	from  common.Address

	// rcpt is only available after inclusion
	rcpt *types.Receipt
}

func NewGeneratedTransaction(t helpers.Testing, chain *Chain, tx *types.Transaction, from common.Address) *GeneratedTransaction {
	return &GeneratedTransaction{
		t:     t,
		chain: chain,
		tx:    tx,
		from:  from,
	}
}

func (m *GeneratedTransaction) Include() {
	rcpt, err := m.chain.SequencerEngine.EngineApi.IncludeTx(m.tx, m.from)
	require.NoError(m.t, err)
	m.rcpt = rcpt
}

func (m *GeneratedTransaction) Identifier() inbox.Identifier {
	require.NotZero(m.t, len(m.rcpt.Logs), "Transaction did not include any logs to reference")

	return Identifier(m.chain, m.tx, m.rcpt)
}

func Identifier(chain *Chain, tx *types.Transaction, rcpt *types.Receipt) inbox.Identifier {
	blockTime := chain.RollupCfg.TimestampForBlock(rcpt.BlockNumber.Uint64())
	return inbox.Identifier{
		Origin:      *tx.To(),
		BlockNumber: rcpt.BlockNumber,
		LogIndex:    new(big.Int).SetUint64(uint64(rcpt.Logs[0].Index)),
		Timestamp:   new(big.Int).SetUint64(blockTime),
		ChainId:     chain.RollupCfg.L2ChainID,
	}
}

func (m *GeneratedTransaction) MessagePayload() []byte {
	require.NotZero(m.t, len(m.rcpt.Logs), "Transaction did not include any logs to reference")
	return stypes.LogToMessagePayload(m.rcpt.Logs[0])
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
