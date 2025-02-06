package interop

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-e2e/actions/interop/dsl"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/interop/contracts/bindings/inbox"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-node/rollup/interop/managed"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
)

func TestFullInterop(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)

	is := dsl.SetupInterop(t)
	actors := is.CreateActors()

	// get both sequencers set up
	actors.ChainA.Sequencer.ActL2PipelineFull(t)
	actors.ChainB.Sequencer.ActL2PipelineFull(t)

	// sync the supervisor, handle initial events emitted by the nodes
	actors.ChainA.Sequencer.SyncSupervisor(t)
	actors.ChainB.Sequencer.SyncSupervisor(t)

	// No blocks yet
	status := actors.ChainA.Sequencer.SyncStatus()
	require.Equal(t, uint64(0), status.UnsafeL2.Number)

	// sync initial chain A and B
	actors.Supervisor.ProcessFull(t)

	// Build L2 block on chain A
	actors.ChainA.Sequencer.ActL2StartBlock(t)
	actors.ChainA.Sequencer.ActL2EndBlock(t)
	status = actors.ChainA.Sequencer.SyncStatus()
	head := status.UnsafeL2.ID()
	require.Equal(t, uint64(1), head.Number)
	require.Equal(t, uint64(0), status.CrossUnsafeL2.Number)
	require.Equal(t, uint64(0), status.LocalSafeL2.Number)
	require.Equal(t, uint64(0), status.SafeL2.Number)
	require.Equal(t, uint64(0), status.FinalizedL2.Number)

	// Ingest the new unsafe-block event
	actors.ChainA.Sequencer.SyncSupervisor(t)

	// Verify as cross-unsafe with supervisor
	actors.Supervisor.ProcessFull(t)
	actors.ChainA.Sequencer.ActL2PipelineFull(t)
	status = actors.ChainA.Sequencer.SyncStatus()
	require.Equal(t, head, status.UnsafeL2.ID())
	require.Equal(t, head, status.CrossUnsafeL2.ID())
	require.Equal(t, uint64(0), status.LocalSafeL2.Number)
	require.Equal(t, uint64(0), status.SafeL2.Number)
	require.Equal(t, uint64(0), status.FinalizedL2.Number)
	supervisorStatus, err := actors.Supervisor.SyncStatus()
	require.NoError(t, err)
	require.Equal(t, head, supervisorStatus.Chains[actors.ChainA.ChainID].LocalUnsafe.ID())
	require.Equal(t, uint64(0), supervisorStatus.MinSyncedL1.Number)

	// Submit the L2 block, sync the local-safe data
	actors.ChainA.Batcher.ActSubmitAll(t)
	actors.L1Miner.ActL1StartBlock(12)(t)
	actors.L1Miner.ActL1IncludeTx(actors.ChainA.BatcherAddr)(t)
	actors.L1Miner.ActL1EndBlock(t)

	// The node will exhaust L1 data,
	// it needs the supervisor to see the L1 block first,
	// and provide it to the node.
	actors.ChainA.Sequencer.ActL2EventsUntil(t, event.Is[derive.ExhaustedL1Event], 100, false)
	actors.Supervisor.SignalLatestL1(t)          // supervisor will be aware of latest L1
	actors.ChainA.Sequencer.SyncSupervisor(t)    // supervisor to react to exhaust-L1
	actors.ChainA.Sequencer.ActL2PipelineFull(t) // node to complete syncing to L1 head.

	// TODO(#13972): two sources of L1 head
	actors.ChainA.Sequencer.ActL1HeadSignal(t)
	status = actors.ChainA.Sequencer.SyncStatus()
	require.Equal(t, head, status.UnsafeL2.ID())
	require.Equal(t, head, status.CrossUnsafeL2.ID())
	require.Equal(t, head, status.LocalSafeL2.ID())
	require.Equal(t, uint64(0), status.SafeL2.Number)
	require.Equal(t, uint64(0), status.FinalizedL2.Number)
	supervisorStatus, err = actors.Supervisor.SyncStatus()
	require.NoError(t, err)
	require.Equal(t, head, supervisorStatus.Chains[actors.ChainA.ChainID].LocalUnsafe.ID())
	require.Equal(t, uint64(0), supervisorStatus.MinSyncedL1.Number)
	// Local-safe does not count as "safe" in RPC
	n := actors.ChainA.SequencerEngine.L2Chain().CurrentSafeBlock().Number.Uint64()
	require.Equal(t, uint64(0), n)

	// Make the supervisor aware of the new L1 block
	actors.Supervisor.SignalLatestL1(t)

	// Ingest the new local-safe event
	actors.ChainA.Sequencer.SyncSupervisor(t)

	// Cross-safe verify it
	actors.Supervisor.ProcessFull(t)
	actors.ChainA.Sequencer.ActL2PipelineFull(t)
	status = actors.ChainA.Sequencer.SyncStatus()
	require.Equal(t, head, status.UnsafeL2.ID())
	require.Equal(t, head, status.CrossUnsafeL2.ID())
	require.Equal(t, head, status.LocalSafeL2.ID())
	require.Equal(t, head, status.SafeL2.ID())
	require.Equal(t, uint64(0), status.FinalizedL2.Number)
	supervisorStatus, err = actors.Supervisor.SyncStatus()
	require.NoError(t, err)
	require.Equal(t, head, supervisorStatus.Chains[actors.ChainA.ChainID].LocalUnsafe.ID())
	require.Equal(t, uint64(1), supervisorStatus.MinSyncedL1.Number)
	h := actors.ChainA.SequencerEngine.L2Chain().CurrentSafeBlock().Hash()
	require.Equal(t, head.Hash, h)

	// Finalize L1, and see if the supervisor updates the op-node finality accordingly.
	// The supervisor then determines finality, which the op-node can use.
	actors.L1Miner.ActL1SafeNext(t)
	actors.L1Miner.ActL1FinalizeNext(t)
	actors.ChainA.Sequencer.ActL1SafeSignal(t) // TODO old source of finality
	actors.ChainA.Sequencer.ActL1FinalizedSignal(t)
	actors.Supervisor.SignalFinalizedL1(t)
	actors.Supervisor.ProcessFull(t)
	actors.ChainA.Sequencer.ActL2PipelineFull(t)
	finalizedL2BlockID, err := actors.Supervisor.Finalized(t.Ctx(), actors.ChainA.ChainID)
	require.NoError(t, err)
	require.Equal(t, head, finalizedL2BlockID)

	actors.ChainA.Sequencer.ActL2PipelineFull(t)
	h = actors.ChainA.SequencerEngine.L2Chain().CurrentFinalBlock().Hash()
	require.Equal(t, head.Hash, h)
	status = actors.ChainA.Sequencer.SyncStatus()
	require.Equal(t, head, status.UnsafeL2.ID())
	require.Equal(t, head, status.CrossUnsafeL2.ID())
	require.Equal(t, head, status.LocalSafeL2.ID())
	require.Equal(t, head, status.SafeL2.ID())
	require.Equal(t, head, status.FinalizedL2.ID())
	supervisorStatus, err = actors.Supervisor.SyncStatus()
	require.NoError(t, err)
	require.Equal(t, head, supervisorStatus.Chains[actors.ChainA.ChainID].LocalUnsafe.ID())
	require.Equal(t, uint64(1), supervisorStatus.MinSyncedL1.Number)
}

// TestFinality confirms that when L1 finality is updated on the supervisor,
// the L2 finality signal updates to the appropriate value.
// Sub-tests control how many additional blocks might be submitted to the L1 chain,
// affecting the way Finality would be determined.
func TestFinality(gt *testing.T) {
	testFinality := func(t helpers.StatefulTesting, extraBlocks int) {
		is := dsl.SetupInterop(t)
		actors := is.CreateActors()

		// set up a blank ChainA
		actors.ChainA.Sequencer.ActL2PipelineFull(t)
		actors.ChainA.Sequencer.SyncSupervisor(t)

		actors.Supervisor.ProcessFull(t)

		// Build L2 block on chain A
		actors.ChainA.Sequencer.ActL2StartBlock(t)
		actors.ChainA.Sequencer.ActL2EndBlock(t)

		// Sync and process the supervisor, updating cross-unsafe
		actors.ChainA.Sequencer.SyncSupervisor(t)
		actors.Supervisor.ProcessFull(t)
		actors.ChainA.Sequencer.ActL2PipelineFull(t)

		// Submit the L2 block, sync the local-safe data
		actors.ChainA.Batcher.ActSubmitAll(t)
		actors.L1Miner.ActL1StartBlock(12)(t)
		actors.L1Miner.ActL1IncludeTx(actors.ChainA.BatcherAddr)(t)
		actors.L1Miner.ActL1EndBlock(t)
		actors.L1Miner.ActL1SafeNext(t)

		// Run the node until the L1 is exhausted
		// and have the supervisor provide the latest L1 block
		actors.ChainA.Sequencer.ActL2EventsUntil(t, event.Is[derive.ExhaustedL1Event], 100, false)
		actors.Supervisor.SignalLatestL1(t)
		actors.ChainA.Sequencer.SyncSupervisor(t)
		actors.ChainA.Sequencer.ActL2PipelineFull(t)
		actors.ChainA.Sequencer.ActL1HeadSignal(t)
		// Make the supervisor aware of the new L1 block
		actors.Supervisor.SignalLatestL1(t)
		// Ingest the new local-safe event
		actors.ChainA.Sequencer.SyncSupervisor(t)
		// Cross-safe verify it
		actors.Supervisor.ProcessFull(t)
		actors.ChainA.Sequencer.ActL2PipelineFull(t)

		// Submit more blocks to the L1, to bury the L2 block
		for i := 0; i < extraBlocks; i++ {
			actors.L1Miner.ActL1StartBlock(12)(t)
			actors.L1Miner.ActL1EndBlock(t)
			actors.L1Miner.ActL1SafeNext(t)
			actors.Supervisor.SignalLatestL1(t)
			actors.Supervisor.ProcessFull(t)
		}

		tip := actors.L1Miner.SafeNum()

		// Update finality on the supervisor to the latest block
		actors.L1Miner.ActL1Finalize(t, tip)
		actors.Supervisor.SignalFinalizedL1(t)

		// Process the supervisor to update the finality, and pull L1, L2 finality
		actors.Supervisor.ProcessFull(t)
		l1Finalized := actors.Supervisor.FinalizedL1()
		l2Finalized, err := actors.Supervisor.Finalized(context.Background(), actors.ChainA.ChainID)
		require.NoError(t, err)
		require.Equal(t, uint64(tip), l1Finalized.Number)
		// the L2 finality is the latest L2 block, because L1 finality is beyond anything the L2 used to derive
		require.Equal(t, uint64(1), l2Finalized.Number)

		// confirm the node also sees the finality
		actors.ChainA.Sequencer.ActL2PipelineFull(t)
		status := actors.ChainA.Sequencer.SyncStatus()
		require.Equal(t, uint64(1), status.FinalizedL2.Number)
	}
	statefulT := helpers.NewDefaultTesting(gt)
	gt.Run("FinalizeBeyondDerived", func(t *testing.T) {
		testFinality(statefulT, 10)
	})
	gt.Run("Finalize", func(t *testing.T) {
		testFinality(statefulT, 0)
	})
}

func TestInteropLocalSafeInvalidation(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)

	is := dsl.SetupInterop(t)
	actors := is.CreateActors()

	// get both sequencers set up
	actors.ChainA.Sequencer.ActL2PipelineFull(t)
	actors.ChainB.Sequencer.ActL2PipelineFull(t)

	// sync the supervisor, handle initial events emitted by the nodes
	actors.ChainA.Sequencer.SyncSupervisor(t)
	actors.ChainB.Sequencer.SyncSupervisor(t)
	actors.Supervisor.ProcessFull(t)

	genesisB := actors.ChainB.Sequencer.SyncStatus()

	// build L2 block on chain B with invalid executing message pointing to A.
	fakeMessage := []byte("this message was never emitted")
	aliceB := setupUser(t, is, actors.ChainB, 0)
	auth := newL2TxOpts(t, aliceB.secret, actors.ChainB)
	id := inbox.Identifier{
		Origin:      common.Address{0x42},
		BlockNumber: new(big.Int).SetUint64(genesisB.UnsafeL2.Number),
		LogIndex:    common.Big0,
		Timestamp:   new(big.Int).SetUint64(genesisB.UnsafeL2.Time),
		ChainId:     actors.ChainA.RollupCfg.L2ChainID,
	}
	contract, err := inbox.NewInbox(predeploys.CrossL2InboxAddr, actors.ChainB.SequencerEngine.EthClient())
	require.NoError(t, err)
	tx, err := contract.ValidateMessage(auth, id, crypto.Keccak256Hash(fakeMessage))
	require.NoError(t, err)

	actors.ChainB.Sequencer.ActL2StartBlock(t)
	require.NoError(t, actors.ChainB.SequencerEngine.EngineApi.IncludeTx(tx, aliceB.address))
	actors.ChainB.Sequencer.ActL2EndBlock(t)
	actors.ChainB.Sequencer.ActL2PipelineFull(t)
	originalBlock := actors.ChainB.Sequencer.SyncStatus().UnsafeL2
	require.Equal(t, uint64(1), originalBlock.Number)
	originalOutput, err := actors.ChainB.Sequencer.RollupClient().OutputAtBlock(t.Ctx(), originalBlock.Number)
	require.NoError(t, err)

	// build another empty L2 block, that will get reorged out
	actors.ChainB.Sequencer.ActL2StartBlock(t)
	actors.ChainB.Sequencer.ActL2EndBlock(t)
	actors.ChainB.Sequencer.ActL2PipelineFull(t)
	extraBlock := actors.ChainB.Sequencer.SyncStatus().UnsafeL2
	require.Equal(t, uint64(2), extraBlock.Number)

	// batch-submit the L2 block to L1
	actors.ChainB.Batcher.ActSubmitAll(t)
	actors.L1Miner.ActL1StartBlock(12)(t)
	actors.L1Miner.ActL1IncludeTx(actors.ChainB.BatcherAddr)(t)
	actors.L1Miner.ActL1EndBlock(t)

	// Signal the supervisor there is a new L1 block
	actors.Supervisor.SignalLatestL1(t)
	// sync the op-node, to signal that derivation needs the new L1 block
	t.Log("awaiting L1-exhaust event")
	actors.ChainB.Sequencer.ActL2PipelineFull(t)
	// sync the supervisor, so it can pass the L1 block to op-node
	t.Log("awaiting supervisor to provide L1 data")
	actors.ChainB.Sequencer.SyncSupervisor(t)
	actors.Supervisor.ProcessFull(t)
	// sync the op-node, so it derives the local-safe head
	t.Log("awaiting node to sync")
	actors.ChainB.Sequencer.ActL2PipelineFull(t)
	// Both L2 blocks were derived from the same L1 batch, and should have been processed into local-safe updates
	require.Equal(t, uint64(2), actors.ChainB.Sequencer.SyncStatus().LocalSafeL2.Number)
	// Make the supervisor process the derivation work from the op-node.
	// It should determine that the local-safe block needs replacement.
	t.Log("Expecting supervisor to sync and catch local-safe dependency issue")
	actors.ChainB.Sequencer.SyncSupervisor(t)
	actors.Supervisor.ProcessFull(t)
	// check supervisor head, expect it to be rewound
	localUnsafe, err := actors.Supervisor.LocalUnsafe(t.Ctx(), actors.ChainB.ChainID)
	require.NoError(t, err)
	require.Equal(t, uint64(0), localUnsafe.Number, "unsafe chain needs to be rewound")

	// Make the op-node do the processing to build the replacement
	t.Log("Expecting op-node to build replacement block")
	actors.ChainB.Sequencer.ActL2PipelineFull(t)
	// Make the supervisor pick up the replacement
	actors.ChainB.Sequencer.SyncSupervisor(t)
	actors.Supervisor.ProcessFull(t)
	// Check that the replacement is recognized as cross-safe
	crossSafe, err := actors.Supervisor.CrossSafe(t.Ctx(), actors.ChainB.ChainID)
	require.NoError(t, err)
	require.NotEqual(t, originalBlock.ID(), crossSafe.Derived)
	require.NotEqual(t, extraBlock.ID(), crossSafe.Derived)
	require.Equal(t, uint64(1), crossSafe.Derived.Number)

	actors.ChainB.Sequencer.ActL2PipelineFull(t)
	// check op-node head matches replacement block
	status := actors.ChainB.Sequencer.SyncStatus()
	require.Equal(t, crossSafe.Derived, status.SafeL2.ID())

	// Parse system tx from replacement block, assert it matches the original block
	replacementBlock, err := actors.ChainB.SequencerEngine.EthClient().BlockByHash(t.Ctx(), status.SafeL2.Hash)
	require.NoError(t, err)
	txs := replacementBlock.Transactions()
	out, err := managed.DecodeInvalidatedBlockTx(txs[len(txs)-1])
	require.NoError(t, err)
	require.Equal(t, originalOutput.OutputRoot, eth.OutputRoot(out))

	// Now check if we can continue to build L2 blocks on top of the new chain.
	// Build a new L2 block
	actors.ChainB.Sequencer.ActL2StartBlock(t)
	actors.ChainB.Sequencer.ActL2EndBlock(t)
	actors.ChainB.Sequencer.ActL2PipelineFull(t)
	// Batch submit the L2 block to L1
	actors.ChainB.Batcher.ActSubmitAll(t)
	actors.L1Miner.ActL1StartBlock(12)(t)
	actors.L1Miner.ActL1IncludeTx(actors.ChainB.BatcherAddr)(t)
	actors.L1Miner.ActL1EndBlock(t)
	// Sync the sequencer / supervisor, so the indexing, local-safe, cross-safe changes all propagate.
	actors.ChainB.Sequencer.ActL2PipelineFull(t)
	actors.ChainB.Sequencer.SyncSupervisor(t)
	actors.Supervisor.SignalLatestL1(t)
	actors.Supervisor.ProcessFull(t)
	actors.ChainB.Sequencer.ActL2PipelineFull(t)
	status = actors.ChainB.Sequencer.SyncStatus()
	require.Equal(t, uint64(2), status.SafeL2.Number)
}
