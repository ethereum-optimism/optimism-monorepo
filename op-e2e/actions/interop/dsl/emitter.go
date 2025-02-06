package dsl

import (
	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/interop/contracts/bindings/emit"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

type EmitterContract struct {
	t        helpers.Testing
	bindings *emit.Emit
	address  common.Address

	EmittedMessages []*GeneratedTransaction
}

func NewEmitterContract(t helpers.Testing) *EmitterContract {
	return &EmitterContract{
		t: t,
	}
}

func (c *EmitterContract) Deploy(user *DSLUser) TransactionCreator {
	return func(chain *Chain) (*types.Transaction, common.Address) {
		opts, from := user.TransactOpts(chain)
		emitContract, tx, emitBindings, err := emit.DeployEmit(opts, chain.SequencerEngine.EthClient())
		require.NoError(c.t, err)
		c.bindings = emitBindings
		c.address = emitContract
		return tx, from
	}
}

func (c *EmitterContract) EmitMessage(user *DSLUser, message string) TransactionCreator {
	return func(chain *Chain) (*types.Transaction, common.Address) {
		opts, from := user.TransactOpts(chain)
		tx, err := c.bindings.EmitData(opts, []byte(message))
		require.NoError(c.t, err)
		c.EmittedMessages = append(c.EmittedMessages, NewGeneratedTransaction(c.t, chain, tx))
		return tx, from
	}
}

func (c *EmitterContract) LastEmittedMessage() *GeneratedTransaction {
	require.NotZero(c.t, c.EmittedMessages, "no messages have been emitted")
	return c.EmittedMessages[len(c.EmittedMessages)-1]
}
