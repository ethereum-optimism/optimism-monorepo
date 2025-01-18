package interop

import (
	"context"
	"log/slog"
	"testing"

	fpHelpers "github.com/ethereum-optimism/optimism/op-e2e/actions/proofs/helpers"
	"github.com/ethereum-optimism/optimism/op-program/client/claim"
	"github.com/ethereum-optimism/optimism/op-program/client/interop"
	"github.com/ethereum-optimism/optimism/op-program/client/interop/types"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
)

func TestFullInterop(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)

	is := SetupInterop(t)
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

	actors.ChainA.Sequencer.ActL1HeadSignal(t) // TODO: two sources of L1 head
	status = actors.ChainA.Sequencer.SyncStatus()
	require.Equal(t, head, status.UnsafeL2.ID())
	require.Equal(t, head, status.CrossUnsafeL2.ID())
	require.Equal(t, head, status.LocalSafeL2.ID())
	require.Equal(t, uint64(0), status.SafeL2.Number)
	require.Equal(t, uint64(0), status.FinalizedL2.Number)
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
}

// TestFinality confirms that when L1 finality is updated on the supervisor,
// the L2 finality signal updates to the appropriate value.
// Sub-tests control how many additional blocks might be submitted to the L1 chain,
// affecting the way Finality would be determined.
func TestFinality(gt *testing.T) {
	testFinality := func(t helpers.StatefulTesting, extraBlocks int) {
		is := SetupInterop(t)
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
		l1Finalized := actors.Supervisor.backend.FinalizedL1()
		l2Finalized, err := actors.Supervisor.backend.Finalized(context.Background(), actors.ChainA.ChainID)
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

func TestInteropFaultProofs(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)

	is := SetupInterop(t)
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

	// sync chain A and B
	actors.Supervisor.ProcessFull(t)

	// Build L2 block on chain A
	actors.ChainA.Sequencer.ActL2StartBlock(t)
	actors.ChainA.Sequencer.ActL2EndBlock(t)
	require.Equal(t, uint64(1), actors.ChainA.Sequencer.L2Unsafe().Number)

	// Build L2 block on chain B
	actors.ChainB.Sequencer.ActL2StartBlock(t)
	actors.ChainB.Sequencer.ActL2EndBlock(t)
	require.Equal(t, uint64(1), actors.ChainB.Sequencer.L2Unsafe().Number)

	// Ingest the new unsafe-block events
	actors.ChainA.Sequencer.SyncSupervisor(t)
	actors.ChainB.Sequencer.SyncSupervisor(t)

	// Verify as cross-unsafe with supervisor
	actors.Supervisor.ProcessFull(t)
	actors.ChainA.Sequencer.ActL2PipelineFull(t)
	status = actors.ChainA.Sequencer.SyncStatus()
	require.Equal(gt, uint64(1), status.UnsafeL2.Number)
	require.Equal(gt, uint64(1), status.CrossUnsafeL2.Number)
	actors.ChainB.Sequencer.ActL2PipelineFull(t)
	status = actors.ChainB.Sequencer.SyncStatus()
	require.Equal(gt, uint64(1), status.UnsafeL2.Number)
	require.Equal(gt, uint64(1), status.CrossUnsafeL2.Number)

	// Submit the L2 blocks, sync the local-safe data
	actors.ChainA.Batcher.ActSubmitAll(t)
	actors.ChainB.Batcher.ActSubmitAll(t)
	actors.L1Miner.ActL1StartBlock(12)(t)
	actors.L1Miner.ActL1IncludeTx(actors.ChainA.BatcherAddr)(t)
	actors.L1Miner.ActL1IncludeTx(actors.ChainB.BatcherAddr)(t)
	actors.L1Miner.ActL1EndBlock(t)
	actors.Supervisor.SignalLatestL1(t)
	// The node will exhaust L1 data,
	// it needs the supervisor to see the L1 block first, and provide it to the node.
	actors.ChainA.Sequencer.ActL2EventsUntil(t, event.Is[derive.ExhaustedL1Event], 100, false)
	actors.ChainB.Sequencer.ActL2EventsUntil(t, event.Is[derive.ExhaustedL1Event], 100, false)
	actors.ChainA.Sequencer.SyncSupervisor(t)    // supervisor to react to exhaust-L1
	actors.ChainB.Sequencer.SyncSupervisor(t)    // supervisor to react to exhaust-L1
	actors.ChainA.Sequencer.ActL2PipelineFull(t) // node to complete syncing to L1 head.
	actors.ChainB.Sequencer.ActL2PipelineFull(t) // node to complete syncing to L1 head.

	actors.ChainA.Sequencer.ActL1HeadSignal(t)
	status = actors.ChainA.Sequencer.SyncStatus()
	require.Equal(gt, uint64(1), status.LocalSafeL2.Number)
	actors.ChainB.Sequencer.ActL1HeadSignal(t)
	status = actors.ChainB.Sequencer.SyncStatus()
	require.Equal(gt, uint64(1), status.LocalSafeL2.Number)

	// Ingest the new local-safe event
	actors.ChainA.Sequencer.SyncSupervisor(t)
	actors.ChainB.Sequencer.SyncSupervisor(t)

	// Cross-safe verify it
	actors.Supervisor.ProcessFull(t)
	actors.ChainA.Sequencer.ActL2PipelineFull(t)
	status = actors.ChainA.Sequencer.SyncStatus()
	require.Equal(gt, uint64(1), status.SafeL2.Number)
	actors.ChainB.Sequencer.ActL2PipelineFull(t)
	status = actors.ChainB.Sequencer.SyncStatus()
	require.Equal(gt, uint64(1), status.SafeL2.Number)

	require.Equal(gt, uint64(1), actors.ChainA.Sequencer.L2Safe().Number)
	require.Equal(gt, uint64(1), actors.ChainB.Sequencer.L2Safe().Number)

	chainAClient := actors.ChainA.Sequencer.RollupClient()
	chainBClient := actors.ChainB.Sequencer.RollupClient()

	ctx := context.Background()
	endTimestamp := actors.ChainA.RollupCfg.Genesis.L2Time + actors.ChainA.RollupCfg.BlockTime
	startTimestamp := endTimestamp - 1
	source, err := NewSuperRootSource(ctx, chainAClient, chainBClient)
	require.NoError(t, err)
	start, err := source.CreateSuperRoot(ctx, startTimestamp)
	require.NoError(t, err)
	end, err := source.CreateSuperRoot(ctx, endTimestamp)
	require.NoError(t, err)

	endBlockNumA, err := actors.ChainA.RollupCfg.TargetBlockNumber(endTimestamp)
	require.NoError(t, err)
	chain1End, err := chainAClient.OutputAtBlock(ctx, endBlockNumA)
	require.NoError(t, err)

	endBlockNumB, err := actors.ChainB.RollupCfg.TargetBlockNumber(endTimestamp)
	require.NoError(t, err)
	chain2End, err := chainBClient.OutputAtBlock(ctx, endBlockNumB)
	require.NoError(t, err)

	step1Expected := (&types.TransitionState{
		SuperRoot: start.Marshal(),
		PendingProgress: []types.OptimisticBlock{
			{BlockHash: chain1End.BlockRef.Hash, OutputRoot: chain1End.OutputRoot},
		},
		Step: 1,
	}).Marshal()

	step2Expected := (&types.TransitionState{
		SuperRoot: start.Marshal(),
		PendingProgress: []types.OptimisticBlock{
			{BlockHash: chain1End.BlockRef.Hash, OutputRoot: chain1End.OutputRoot},
			{BlockHash: chain2End.BlockRef.Hash, OutputRoot: chain2End.OutputRoot},
		},
		Step: 2,
	}).Marshal()

	paddingStep := func(step uint64) []byte {
		return (&types.TransitionState{
			SuperRoot: start.Marshal(),
			PendingProgress: []types.OptimisticBlock{
				{BlockHash: chain1End.BlockRef.Hash, OutputRoot: chain1End.OutputRoot},
				{BlockHash: chain2End.BlockRef.Hash, OutputRoot: chain2End.OutputRoot},
			},
			Step: step,
		}).Marshal()
	}

	tests := []*transitionTest{
		{
			name:           "ClaimNoChange",
			startTimestamp: startTimestamp,
			agreedClaim:    start.Marshal(),
			disputedClaim:  start.Marshal(),
			expectValid:    false,
		},
		{
			name:           "ClaimDirectToNextTimestamp",
			startTimestamp: startTimestamp,
			agreedClaim:    start.Marshal(),
			disputedClaim:  end.Marshal(),
			expectValid:    false,
		},
		{
			name:           "FirstChainOptimisticBlock",
			startTimestamp: startTimestamp,
			agreedClaim:    start.Marshal(),
			disputedClaim:  step1Expected,
			expectValid:    true,
		},
		{
			name:           "SecondChainOptimisticBlock",
			startTimestamp: startTimestamp,
			agreedClaim:    step1Expected,
			disputedClaim:  step2Expected,
			expectValid:    true,
		},
		{
			name:           "FirstPaddingStep",
			startTimestamp: startTimestamp,
			agreedClaim:    step2Expected,
			disputedClaim:  paddingStep(3),
			expectValid:    true,
		},
		{
			name:           "SecondPaddingStep",
			startTimestamp: startTimestamp,
			agreedClaim:    paddingStep(3),
			disputedClaim:  paddingStep(4),
			expectValid:    true,
		},
		{
			name:           "LastPaddingStep",
			startTimestamp: startTimestamp,
			agreedClaim:    paddingStep(1022),
			disputedClaim:  paddingStep(1023),
			expectValid:    true,
		},
		{
			name:           "Consolidate",
			startTimestamp: startTimestamp,
			agreedClaim:    paddingStep(1023),
			disputedClaim:  end.Marshal(),
			expectValid:    true,
		},

		{
			name:           "FirstChainReachesL1Head",
			startTimestamp: startTimestamp,
			agreedClaim:    start.Marshal(),
			disputedClaim:  interop.InvalidTransition,
			// The derivation reaches the L1 head before the next block can be created
			l1Head:      actors.L1Miner.L1Chain().Genesis().Hash(),
			expectValid: true,
		},
		{
			name:           "SecondChainReachesL1Head",
			startTimestamp: startTimestamp,
			agreedClaim:    step1Expected,
			disputedClaim:  interop.InvalidTransition,
			// The derivation reaches the L1 head before the next block can be created
			l1Head:      actors.L1Miner.L1Chain().Genesis().Hash(),
			expectValid: true,
		},
		{
			name:           "SuperRootInvalidIfUnsupportedByL1Data",
			startTimestamp: startTimestamp,
			agreedClaim:    step1Expected,
			disputedClaim:  step2Expected,
			// The derivation reaches the L1 head before the next block can be created
			l1Head:      actors.L1Miner.L1Chain().Genesis().Hash(),
			expectValid: false,
		},
		{
			name:           "FromInvalidTransitionHash",
			startTimestamp: startTimestamp,
			agreedClaim:    interop.InvalidTransition,
			disputedClaim:  interop.InvalidTransition,
			// The derivation reaches the L1 head before the next block can be created
			l1Head:      actors.L1Miner.L1Chain().Genesis().Hash(),
			expectValid: true,
		},
	}

	for _, test := range tests {
		test := test
		gt.Run(test.name, func(gt *testing.T) {
			t := helpers.NewDefaultTesting(gt)
			if test.skip {
				t.Skip("Not yet implemented")
				return
			}
			logger := testlog.Logger(t, slog.LevelInfo)
			checkResult := fpHelpers.ExpectNoError()
			if !test.expectValid {
				checkResult = fpHelpers.ExpectError(claim.ErrClaimNotValid)
			}
			l1Head := test.l1Head
			if l1Head == (common.Hash{}) {
				l1Head = actors.L1Miner.L1Chain().CurrentBlock().Hash()
			}
			fpHelpers.RunFaultProofProgram(
				t,
				logger,
				actors.L1Miner,
				checkResult,
				WithInteropEnabled(actors, test.agreedClaim, crypto.Keccak256Hash(test.disputedClaim), endTimestamp),
				fpHelpers.WithL1Head(l1Head),
			)
		})
	}
}

func WithInteropEnabled(actors *InteropActors, agreedPrestate []byte, disputedClaim common.Hash, claimTimestamp uint64) fpHelpers.FixtureInputParam {
	return func(f *fpHelpers.FixtureInputs) {
		f.InteropEnabled = true
		f.AgreedPrestate = agreedPrestate
		f.L2OutputRoot = crypto.Keccak256Hash(agreedPrestate)
		f.L2Claim = disputedClaim
		f.L2BlockNumber = claimTimestamp

		// TODO: Remove these once hints all specify the L2 chain ID
		f.L2ChainID = actors.ChainA.ChainID.ToBig().Uint64()
		f.L2Head = actors.ChainA.SequencerEngine.L2Chain().CurrentHeader().ParentHash

		for _, chain := range []*Chain{actors.ChainA, actors.ChainB} {
			f.L2Sources = append(f.L2Sources, &fpHelpers.FaultProofProgramL2Source{
				Node:        chain.Sequencer.L2Verifier,
				Engine:      chain.SequencerEngine,
				ChainConfig: chain.L2Genesis.Config,
			})
		}
	}
}

type transitionTest struct {
	name           string
	startTimestamp uint64
	agreedClaim    []byte
	disputedClaim  []byte
	l1Head         common.Hash // Defaults to current L1 head if not set
	expectValid    bool
	skip           bool
}
