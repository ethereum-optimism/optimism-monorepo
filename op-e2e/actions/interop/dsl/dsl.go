package dsl

import (
	"context"
	"time"

	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

type ChainOpts struct {
	Chains []*Chain
}

func (c *ChainOpts) SetChains(chains ...*Chain) {
	c.Chains = chains
}

func (c *ChainOpts) AddChain(chain *Chain) {
	c.Chains = append(c.Chains, chain)
}

// InteropDSL provides a high-level API to drive interop action tests so that the actual test reads more declaratively
// and is separated from the details of how each action is actually executed.
// DSL methods will typically:
//  1. Check (and if needed, wait) for any required preconditions
//  2. Perform the action, allowing components to fully process the effects of it
//  3. Assert that the action completed. These are intended to be a sanity check to ensure tests fail fast if something
//     doesn't work as expected. Options may be provided to perform more detailed or specific assertions
//
// Optional inputs can be used to control lower level details of the operation. While it is also possible to directly
// access the Actors and low level actions, this should only be required when specifically testing low level details of
// that functionality. It is generally preferable to use optional inputs to the DSL methods to achieve the desired result
// rather than having to use the low level APIs directly.
//
// Methods may also be provided specifically to verify some state.
// Methods may return some data from the system (e.g. OutputRootAtTimestamp) but it is generally preferred to provide
// an assertion method rather than a getter where that is viable. Assertion methods allow the DSL to provide more helpful
// information in failure messages and ensure the comparison is done correctly and consistently across tests rather than
// duplicating the assertion code in many tests.
//
// Required inputs to methods are specified as normal parameters, so type checking enforces their presence.
// Optional inputs to methods are specified by a config struct and accept a vararg of functions that can update that struct.
// This is roughly inline with the typical opts pattern in Golang but with significantly reduced boilerplate code since
// so many methods wil define their own config. With* methods are only provided for the most common optional args and
// tests will normally supply a custom function that sets all the optional values they need at once.
// Common options can be extracted to a reusable struct (e.g. ChainOpts above) which may expose helper methods to aid
// test readability and reduce boilerplate.
type InteropDSL struct {
	t               helpers.Testing
	Actors          *InteropActors
	SuperRootSource *SuperRootSource
	setup           *InteropSetup

	// allChains contains all chains in the interop set.
	// Currently this is always two chains, but as the setup code becomes more flexible it could be more
	// and likely this array would be replaced by something in InteropActors
	allChains    []*Chain
	createdUsers uint64
}

func NewInteropDSL(t helpers.Testing) *InteropDSL {
	setup := SetupInterop(t)
	actors := setup.CreateActors()

	t.Logf("ChainA: %v, ChainB: %v", actors.ChainA.ChainID, actors.ChainB.ChainID)

	allChains := []*Chain{actors.ChainA, actors.ChainB}

	// Get all the initial events processed
	for _, chain := range allChains {
		chain.Sequencer.ActL2PipelineFull(t)
		chain.Sequencer.SyncSupervisor(t)
	}
	actors.Supervisor.ProcessFull(t)

	superRootSource, err := NewSuperRootSource(
		t.Ctx(),
		actors.ChainA.Sequencer.RollupClient(),
		actors.ChainB.Sequencer.RollupClient())
	require.NoError(t, err)

	return &InteropDSL{
		t:               t,
		Actors:          actors,
		SuperRootSource: superRootSource,
		setup:           setup,

		allChains: allChains,
	}
}

func (d *InteropDSL) defaultChainOpts() ChainOpts {
	return ChainOpts{
		// Defensive copy to make sure the original slice isn't modified
		Chains: append([]*Chain{}, d.allChains...),
	}
}

func (d *InteropDSL) CreateUser() *DSLUser {
	keyIndex := d.createdUsers
	d.createdUsers++
	return &DSLUser{
		t:     d.t,
		index: keyIndex,
		keys:  d.setup.Keys,
	}
}

func (d *InteropDSL) SuperRoot(timestamp uint64) eth.Super {
	ctx, cancel := context.WithTimeout(d.t.Ctx(), 30*time.Second)
	defer cancel()
	root, err := d.SuperRootSource.CreateSuperRoot(ctx, timestamp)
	require.NoError(d.t, err)
	return root
}

func (d *InteropDSL) OutputRootAtTimestamp(chain *Chain, timestamp uint64) *eth.OutputResponse {
	ctx, cancel := context.WithTimeout(d.t.Ctx(), 30*time.Second)
	defer cancel()
	blockNum, err := chain.RollupCfg.TargetBlockNumber(timestamp)
	require.NoError(d.t, err)
	output, err := chain.Sequencer.RollupClient().OutputAtBlock(ctx, blockNum)
	require.NoError(d.t, err)
	return output
}

type TransactionCreator func(chain *Chain) (*types.Transaction, common.Address)
type AddL2BlockOpts struct {
	BlockIsNotCrossSafe bool
	TransactionCreators []TransactionCreator
}

func WithL2BlockTransactions(mkTxs ...TransactionCreator) func(*AddL2BlockOpts) {
	return func(o *AddL2BlockOpts) {
		o.TransactionCreators = mkTxs
	}
}

// AddL2Block adds a new unsafe block to the specified chain and fully processes it in the supervisor
func (d *InteropDSL) AddL2Block(chain *Chain, optionalArgs ...func(*AddL2BlockOpts)) {
	opts := AddL2BlockOpts{}
	for _, arg := range optionalArgs {
		arg(&opts)
	}
	priorSyncStatus := chain.Sequencer.SyncStatus()
	chain.Sequencer.ActL2StartBlock(d.t)
	for _, creator := range opts.TransactionCreators {
		tx, from := creator(chain)
		err := chain.SequencerEngine.EngineApi.IncludeTx(tx, from)
		require.NoError(d.t, err)
	}
	chain.Sequencer.ActL2EndBlock(d.t)
	chain.Sequencer.SyncSupervisor(d.t)
	d.Actors.Supervisor.ProcessFull(d.t)
	chain.Sequencer.ActL2PipelineFull(d.t)

	status := chain.Sequencer.SyncStatus()
	expectedBlockNum := priorSyncStatus.UnsafeL2.Number + 1
	require.Equal(d.t, expectedBlockNum, status.UnsafeL2.Number, "Unsafe head did not advance")
	if opts.BlockIsNotCrossSafe {
		require.Equal(d.t, priorSyncStatus.CrossUnsafeL2.Number, status.CrossUnsafeL2.Number, "CrossUnsafe head advanced unexpectedly")
	} else {
		require.Equal(d.t, expectedBlockNum, status.CrossUnsafeL2.Number, "CrossUnsafe head did not advance")
	}
}

type SubmitBatchDataOpts struct {
	ChainOpts
	SkipCrossSafeUpdate bool
}

// SubmitBatchData submits batch data to L1 and processes the new L1 blocks, advancing the safe heads.
// By default, submits all batch data for all chains.
func (d *InteropDSL) SubmitBatchData(optionalArgs ...func(*SubmitBatchDataOpts)) {
	opts := SubmitBatchDataOpts{
		ChainOpts: d.defaultChainOpts(),
	}
	for _, arg := range optionalArgs {
		arg(&opts)
	}
	txInclusion := make([]helpers.Action, 0, len(opts.Chains))
	for _, chain := range opts.Chains {
		chain.Batcher.ActSubmitAll(d.t)
		txInclusion = append(txInclusion, d.Actors.L1Miner.ActL1IncludeTx(chain.BatcherAddr))
	}
	d.AdvanceL1(func(l1Opts *AdvanceL1Opts) {
		l1Opts.TxInclusion = txInclusion
	})

	// Verify the local safe head advanced on each chain
	for _, chain := range opts.Chains {
		status := chain.Sequencer.SyncStatus()
		require.Equalf(d.t, status.UnsafeL2, status.LocalSafeL2, "Chain %v did not fully advance local safe head", chain.ChainID)

		// Ingest the new local-safe event
		chain.Sequencer.SyncSupervisor(d.t)
	}

	if !opts.SkipCrossSafeUpdate {
		d.ProcessCrossSafe(func(o *ProcessCrossSafeOpts) {
			o.Chains = opts.Chains
		})
	}
}

type ProcessCrossSafeOpts struct {
	ChainOpts
}

// ProcessCrossSafe processes evens in the supervisor and nodes to ensure the cross-safe head is fully updated.
func (d *InteropDSL) ProcessCrossSafe(optionalArgs ...func(*ProcessCrossSafeOpts)) {
	opts := ProcessCrossSafeOpts{
		ChainOpts: d.defaultChainOpts(),
	}
	for _, arg := range optionalArgs {
		arg(&opts)
	}
	// Process cross-safe updates
	d.Actors.Supervisor.ProcessFull(d.t)

	// Process updates on each chain and verify the cross-safe head advanced
	for _, chain := range opts.Chains {
		chain.Sequencer.ActL2PipelineFull(d.t)
		status := chain.Sequencer.SyncStatus()
		require.Equalf(d.t, status.UnsafeL2, status.SafeL2, "Chain %v did not fully advance safe head", chain.ChainID)

		chain.Sequencer.SyncSupervisor(d.t)
	}

	// Re-run in case there was an invalid block that was replaced so it can now be considered safe
	// TODO: Should this just loop until the cross safe heads stop updating or is once enough?
	d.Actors.Supervisor.ProcessFull(d.t)
	// Process updates on each chain and verify the cross-safe head advanced
	for _, chain := range opts.Chains {
		chain.Sequencer.ActL2PipelineFull(d.t)
		status := chain.Sequencer.SyncStatus()
		require.Equalf(d.t, status.UnsafeL2, status.SafeL2, "Chain %v did not fully advance safe head", chain.ChainID)
	}
}

type AdvanceL1Opts struct {
	ChainOpts
	L1BlockTimeSeconds uint64
	TxInclusion        []helpers.Action
}

// AdvanceL1 adds a new L1 block with the specified transactions and ensures it is processed by the specified chains
// and the supervisor.
func (d *InteropDSL) AdvanceL1(optionalArgs ...func(*AdvanceL1Opts)) {
	opts := AdvanceL1Opts{
		ChainOpts:          d.defaultChainOpts(),
		L1BlockTimeSeconds: 12,
	}
	for _, arg := range optionalArgs {
		arg(&opts)
	}
	expectedL1BlockNum := d.Actors.L1Miner.L1Chain().CurrentBlock().Number.Uint64() + 1
	d.Actors.L1Miner.ActL1StartBlock(opts.L1BlockTimeSeconds)(d.t)
	for _, txInclusion := range opts.TxInclusion {
		txInclusion(d.t)
	}
	d.Actors.L1Miner.ActL1EndBlock(d.t)
	newBlock := eth.InfoToL1BlockRef(eth.HeaderBlockInfo(d.Actors.L1Miner.L1Chain().CurrentBlock()))
	require.Equal(d.t, expectedL1BlockNum, newBlock.Number, "L1 head did not advance")
	d.Actors.Supervisor.SignalLatestL1(d.t)

	// The node will exhaust L1 data, it needs the supervisor to see the L1 block first, and provide it to the node.
	for _, chain := range opts.Chains {
		chain.Sequencer.ActL2EventsUntil(d.t, event.Is[derive.ExhaustedL1Event], 100, false)
		chain.Sequencer.SyncSupervisor(d.t)
		chain.Sequencer.ActL2PipelineFull(d.t)
		chain.Sequencer.ActL1HeadSignal(d.t)
	}

	// Verify that the new L1 block was processed everywhere
	for _, chain := range opts.Chains {
		status := chain.Sequencer.SyncStatus()
		require.Equalf(d.t, newBlock, status.HeadL1, "Chain %v did not detect new L1 head", chain.ChainID)
		require.Equalf(d.t, newBlock, status.CurrentL1, "Chain %v did not process new L1 head", chain.ChainID)
	}
}
