package dsl

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/interop/contracts/bindings/emit"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

type EmitterContract struct {
	t              helpers.Testing
	addressByChain map[eth.ChainID]common.Address

	EmittedMessages []*GeneratedTransaction
}

func NewEmitterContract(t helpers.Testing) *EmitterContract {
	return &EmitterContract{
		t:              t,
		addressByChain: make(map[eth.ChainID]common.Address),
	}
}

func (c *EmitterContract) Deploy(user *DSLUser) TransactionCreator {
	return func(chain *Chain) (*types.Transaction, common.Address) {
		opts, from := user.TransactOpts(chain)
		emitContract, tx, _, err := emit.DeployEmit(opts, chain.SequencerEngine.EthClient())
		require.NoError(c.t, err)
		c.addressByChain[chain.ChainID] = emitContract
		return tx, from
	}
}

func (c *EmitterContract) EmitMessage(user *DSLUser, message string) TransactionCreator {
	return func(chain *Chain) (*types.Transaction, common.Address) {
		opts, from := user.TransactOpts(chain)
		address, ok := c.addressByChain[chain.ChainID]
		if !ok {
			c.t.Fatal(fmt.Errorf("not deployed on chain %d", chain.ChainID))
		}
		bindings, err := emit.NewEmitTransactor(address, chain.SequencerEngine.EthClient())
		require.NoError(c.t, err)
		tx, err := bindings.EmitData(opts, []byte(message))
		require.NoError(c.t, err)
		c.EmittedMessages = append(c.EmittedMessages, NewGeneratedTransaction(c.t, chain, tx))
		return tx, from
	}
}

func (c *EmitterContract) LastEmittedMessage() *GeneratedTransaction {
	require.NotZero(c.t, c.EmittedMessages, "no messages have been emitted")
	return c.EmittedMessages[len(c.EmittedMessages)-1]
}
