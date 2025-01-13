package interop

import (
	"context"
	"log/slog"
	"testing"

	fpHelpers "github.com/ethereum-optimism/optimism/op-e2e/actions/proofs/helpers"
	"github.com/ethereum-optimism/optimism/op-program/client/claim"
	"github.com/ethereum-optimism/optimism/op-program/client/interop/types"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
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

	// sync chain A
	actors.Supervisor.SyncEvents(t, actors.ChainA.ChainID)
	actors.Supervisor.SyncCrossUnsafe(t, actors.ChainA.ChainID)
	actors.Supervisor.SyncCrossSafe(t, actors.ChainA.ChainID)

	// sync chain B
	actors.Supervisor.SyncEvents(t, actors.ChainB.ChainID)
	actors.Supervisor.SyncCrossUnsafe(t, actors.ChainB.ChainID)
	actors.Supervisor.SyncCrossSafe(t, actors.ChainB.ChainID)

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
	actors.Supervisor.SyncEvents(t, actors.ChainA.ChainID)
	actors.Supervisor.SyncCrossUnsafe(t, actors.ChainA.ChainID)
	actors.ChainA.Sequencer.AwaitSentCrossUnsafeUpdate(t, 1)
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

	// Ingest the new local-safe event
	actors.ChainA.Sequencer.SyncSupervisor(t)

	// Cross-safe verify it
	actors.Supervisor.SyncCrossSafe(t, actors.ChainA.ChainID)
	actors.ChainA.Sequencer.AwaitSentCrossSafeUpdate(t, 1)
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
	actors.Supervisor.SyncFinalizedL1(t, status.HeadL1)
	actors.ChainA.Sequencer.AwaitSentFinalizedUpdate(t, 1)
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

	// sync chain A
	actors.Supervisor.SyncEvents(t, actors.ChainA.ChainID)
	actors.Supervisor.SyncCrossUnsafe(t, actors.ChainA.ChainID)
	actors.Supervisor.SyncCrossSafe(t, actors.ChainA.ChainID)

	// sync chain B
	actors.Supervisor.SyncEvents(t, actors.ChainB.ChainID)
	actors.Supervisor.SyncCrossUnsafe(t, actors.ChainB.ChainID)
	actors.Supervisor.SyncCrossSafe(t, actors.ChainB.ChainID)

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
	actors.Supervisor.SyncEvents(t, actors.ChainA.ChainID)
	actors.Supervisor.SyncEvents(t, actors.ChainB.ChainID)
	actors.Supervisor.SyncCrossUnsafe(t, actors.ChainA.ChainID)
	actors.Supervisor.SyncCrossUnsafe(t, actors.ChainB.ChainID)
	actors.ChainA.Sequencer.AwaitSentCrossUnsafeUpdate(t, 1)
	actors.ChainA.Sequencer.ActL2PipelineFull(t)
	status = actors.ChainA.Sequencer.SyncStatus()
	require.Equal(gt, uint64(1), status.UnsafeL2.Number)
	require.Equal(gt, uint64(1), status.CrossUnsafeL2.Number)
	actors.ChainB.Sequencer.AwaitSentCrossUnsafeUpdate(t, 1)
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
	actors.Supervisor.SyncCrossSafe(t, actors.ChainA.ChainID)
	actors.Supervisor.SyncCrossSafe(t, actors.ChainB.ChainID)
	actors.ChainA.Sequencer.AwaitSentCrossSafeUpdate(t, 1)
	actors.ChainA.Sequencer.ActL2PipelineFull(t)
	status = actors.ChainA.Sequencer.SyncStatus()
	require.Equal(gt, uint64(1), status.SafeL2.Number)
	actors.ChainB.Sequencer.AwaitSentCrossSafeUpdate(t, 1)
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

	serializeIntermediateRoot := func(root *types.TransitionState) []byte {
		data, err := root.Marshal()
		require.NoError(t, err)
		return data
	}

	endBlockNumA, err := actors.ChainA.RollupCfg.TargetBlockNumber(endTimestamp)
	require.NoError(t, err)
	chain1End, err := chainAClient.OutputAtBlock(ctx, endBlockNumA)
	require.NoError(t, err)

	endBlockNumB, err := actors.ChainB.RollupCfg.TargetBlockNumber(endTimestamp)
	require.NoError(t, err)
	chain2End, err := chainBClient.OutputAtBlock(ctx, endBlockNumB)
	require.NoError(t, err)

	step1Expected := serializeIntermediateRoot(&types.TransitionState{
		SuperRoot: start.Marshal(),
		PendingProgress: []types.OptimisticBlock{
			{BlockHash: chain1End.BlockRef.Hash, OutputRoot: chain1End.OutputRoot},
		},
		Step: 1,
	})

	step2Expected := serializeIntermediateRoot(&types.TransitionState{
		SuperRoot: start.Marshal(),
		PendingProgress: []types.OptimisticBlock{
			{BlockHash: chain1End.BlockRef.Hash, OutputRoot: chain1End.OutputRoot},
			{BlockHash: chain2End.BlockRef.Hash, OutputRoot: chain2End.OutputRoot},
		},
		Step: 2,
	})

	step3Expected := serializeIntermediateRoot(&types.TransitionState{
		SuperRoot: start.Marshal(),
		PendingProgress: []types.OptimisticBlock{
			{BlockHash: chain1End.BlockRef.Hash, OutputRoot: chain1End.OutputRoot},
			{BlockHash: chain2End.BlockRef.Hash, OutputRoot: chain2End.OutputRoot},
		},
		Step: 3,
	})

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
			skip:           true,
		},
		{
			name:           "PaddingStep",
			startTimestamp: startTimestamp,
			agreedClaim:    step2Expected,
			disputedClaim:  step3Expected,
			expectValid:    true,
			skip:           true,
		},
		{
			name:           "Consolidate",
			startTimestamp: startTimestamp,
			agreedClaim:    step3Expected,
			disputedClaim:  end.Marshal(),
			expectValid:    true,
			skip:           true,
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
			fpHelpers.RunFaultProofProgram(
				t,
				logger,
				actors.L1Miner,
				actors.ChainA.Sequencer.L2Verifier,
				actors.ChainA.SequencerEngine,
				actors.ChainA.L2Genesis,
				chain1End.BlockRef.Number,
				checkResult,
				fpHelpers.WithInteropEnabled(),
				fpHelpers.WithAgreedPrestate(test.agreedClaim),
				fpHelpers.WithL2Claim(crypto.Keccak256Hash(test.disputedClaim)),
				fpHelpers.WithL2BlockNumber(endBlockNumA),
			)
		})
	}
}

type transitionTest struct {
	name           string
	startTimestamp uint64
	agreedClaim    []byte
	disputedClaim  []byte
	expectValid    bool
	skip           bool
}
