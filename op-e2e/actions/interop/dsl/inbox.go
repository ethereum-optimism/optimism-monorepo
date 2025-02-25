package dsl

import (
	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/interop/contracts/bindings/inbox"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
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

type ExecuteOpts struct {
	Identifier *inbox.Identifier
	Payload    *[]byte
}

func WithIdentifier(ident inbox.Identifier) func(opts *ExecuteOpts) {
	return func(opts *ExecuteOpts) {
		opts.Identifier = &ident
	}
}

func WithPayload(payload []byte) func(opts *ExecuteOpts) {
	return func(opts *ExecuteOpts) {
		opts.Payload = &payload
	}
}

func (i *InboxContract) Execute(user *DSLUser, initTx *GeneratedTransaction, args ...func(opts *ExecuteOpts)) TransactionCreator {
	opts := ExecuteOpts{}
	for _, arg := range args {
		arg(&opts)
	}
	return func(chain *Chain) *GeneratedTransaction {
		// Wait until we're actually creating this transaction to call initTx methods.
		// This allows the init tx to be in the same block as the exec tx as the actual initTx is only
		// created when it gets included in the block.
		var ident inbox.Identifier
		if opts.Identifier != nil {
			ident = *opts.Identifier
		} else {
			ident = initTx.Identifier()
		}
		var payload []byte
		if opts.Payload != nil {
			payload = *opts.Payload
		} else {
			payload = initTx.MessagePayload()
		}
		txOpts, from := user.TransactOpts(chain)
		contract, err := inbox.NewInbox(predeploys.CrossL2InboxAddr, chain.SequencerEngine.EthClient())
		require.NoError(i.t, err)
		tx, err := contract.ValidateMessage(txOpts, ident, crypto.Keccak256Hash(payload))
		require.NoError(i.t, err)
		genTx := NewGeneratedTransaction(i.t, chain, tx, from)
		i.Transactions = append(i.Transactions, genTx)
		return genTx
	}
}

func (i *InboxContract) LastTransaction() *GeneratedTransaction {
	require.NotZero(i.t, i.Transactions, "no transactions created")
	return i.Transactions[len(i.Transactions)-1]
}
