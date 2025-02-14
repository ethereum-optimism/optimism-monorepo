package dsl

import (
	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/interop/contracts/bindings/inbox"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

type InboxContract struct {
	t helpers.Testing

	Transactions []*GeneratedTransaction
}

func NewInboxContract(t helpers.Testing) *InboxContract {
	return &InboxContract{
		t: t,
	}
}

func (i *InboxContract) Execute(user *DSLUser, id inbox.Identifier, msg []byte) TransactionCreator {
	return func(chain *Chain) (*types.Transaction, common.Address) {
		opts, from := user.TransactOpts(chain)
		contract, err := inbox.NewInbox(predeploys.CrossL2InboxAddr, chain.SequencerEngine.EthClient())
		require.NoError(i.t, err)
		tx, err := contract.ValidateMessage(opts, id, crypto.Keccak256Hash(msg))
		require.NoError(i.t, err)
		i.Transactions = append(i.Transactions, NewGeneratedTransaction(i.t, chain, tx))
		return tx, from
	}
}

func (i *InboxContract) LastTransaction() *GeneratedTransaction {
	require.NotZero(i.t, i.Transactions, "no transactions created")
	return i.Transactions[len(i.Transactions)-1]
}
