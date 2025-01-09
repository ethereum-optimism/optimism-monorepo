package backend

import (
	"github.com/ethereum-optimism/optimism/op-chain-ops/script/forking"
	supervisortypes "github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

type BuilderInput interface {
	NextTx() (*types.Transaction, error)
}

type Worker struct {
	log log.Logger

	chainCfg *params.ChainConfig
	env      *vm.EVM

	state     *forking.ForkableState
	baseState *state.StateDB
}

func (b *Worker) Build(input BuilderInput) error {

}

func (b *Worker) blockPrefix() {
	// TODO get randomness
	// TODO calc basefee
	// TODO holocene basefee

	// TODO beacon block root
	// TODO parent block hash
	// TODO excess blob gas

	// TODO apply beacon block root
	// TODO apply block hash acc ()

	// TODO EnsureCreate2Deployer
	// TODO initial deposits
}

func (b *Worker) blockTx() {
	// check gas pool
	// check DA limit
	// SetTxContext
	// check tx-conditional
	// core.ApplyTransactionExtended with PostValidation
}

func (b *Worker) blockSuffix() {
	// TODO requests root
	// TODO withdrawals root
}

type CrossBuilder struct {
	byChain map[supervisortypes.ChainID]*Worker
}

func (c *CrossBuilder) Build(input BuilderInput) {
	// TODO two classes of state:
	// 1. the canonical block building state
	// 2. worker forks on top of the canonical building state,
	//    using a state-fork over uncommited state
	// 3.

	// get tx
	// get chain ID
	// simulate tx
	//   halt on emit event
	//   check interop-gas, enforce priority fee.
	//   if exec:
	//  	check if emitted by other chain
	//      if same chain, must already be in buffer
	//      if wildcard, use solver to create tx
	//   if init:
	//    append to event buffer, usable by others.
	//    unblock any prior exec of other chain
	//

}
