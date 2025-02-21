package interop

import (
	"fmt"
	"log/slog"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/super"
	challengerTypes "github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/interop/dsl"
	fpHelpers "github.com/ethereum-optimism/optimism/op-e2e/actions/proofs/helpers"
	"github.com/ethereum-optimism/optimism/op-program/client/claim"
	"github.com/ethereum-optimism/optimism/op-program/client/interop"
	"github.com/ethereum-optimism/optimism/op-program/client/interop/types"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	supervisortypes "github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestInteropFaultProofs_TraceExtensionActivation(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	system := dsl.NewInteropDSL(t)

	system.AddL2Block(system.Actors.ChainA)
	system.AddL2Block(system.Actors.ChainB)

	// Submit batch data for each chain in separate L1 blocks so tests can have one chain safe and one unsafe
	system.SubmitBatchData()

	endTimestamp := system.Actors.ChainA.Sequencer.L2Safe().Time

	agreedClaim := system.Outputs.SuperRoot(endTimestamp).Marshal()
	disputedClaim := system.Outputs.TransitionState(endTimestamp, 1,
		system.Outputs.OptimisticBlockAtTimestamp(system.Actors.ChainA, endTimestamp+1)).Marshal()
	disputedTraceIndex := int64(1024)
	tests := []*transitionTest{
		{
			name:               "CorrectlyDidNotActivate",
			agreedClaim:        agreedClaim,
			disputedClaim:      disputedClaim,
			disputedTraceIndex: disputedTraceIndex,
			// Trace extension does not activate because we have not reached the proposal timestamp yet
			proposalTimestamp: endTimestamp + 1,
			expectValid:       true,
		},
		{
			name:               "IncorrectlyDidNotActivate",
			agreedClaim:        agreedClaim,
			disputedClaim:      disputedClaim,
			disputedTraceIndex: disputedTraceIndex,
			// Trace extension should have activated because we have gone past the proposal timestamp yet, but did not
			proposalTimestamp: endTimestamp,
			expectValid:       false,
		},
		{
			name:               "CorrectlyActivated",
			agreedClaim:        agreedClaim,
			disputedClaim:      agreedClaim,
			disputedTraceIndex: disputedTraceIndex,
			// Trace extension does not activate because we have not reached the proposal timestamp yet
			proposalTimestamp: endTimestamp,
			expectValid:       true,
		},
		{
			name:               "IncorrectlyActivated",
			agreedClaim:        agreedClaim,
			disputedClaim:      agreedClaim,
			disputedTraceIndex: disputedTraceIndex,
			// Trace extension does not activate because we have not reached the proposal timestamp yet
			proposalTimestamp: endTimestamp + 1,
			expectValid:       false,
		},
	}
	runFppAndChallengerTests(gt, system, tests)
}

func TestInteropFaultProofs_ConsolidateValidCrossChainMessage(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	system := dsl.NewInteropDSL(t)
	actors := system.Actors

	alice := system.CreateUser()
	emitter := dsl.NewEmitterContract(t)
	system.AddL2Block(actors.ChainA, dsl.WithL2BlockTransactions(emitter.Deploy(alice)))
	system.AddL2Block(actors.ChainB, dsl.WithL2BlockTransactions(emitter.Deploy(alice)))

	system.AddL2Block(system.Actors.ChainA, dsl.WithL2BlockTransactions(emitter.EmitMessage(alice, "hello")))
	initMsg := emitter.LastEmittedMessage()
	system.AddL2Block(system.Actors.ChainB, dsl.WithL2BlockTransactions(system.InboxContract.Execute(alice, initMsg)))

	// Submit batch data for each chain in separate L1 blocks so tests can have one chain safe and one unsafe
	system.SubmitBatchData(func(opts *dsl.SubmitBatchDataOpts) {
		opts.SetChains(system.Actors.ChainA)
	})
	system.SubmitBatchData(func(opts *dsl.SubmitBatchDataOpts) {
		opts.SetChains(system.Actors.ChainB)
	})

	endTimestamp := system.Actors.ChainA.Sequencer.L2Safe().Time
	startTimestamp := endTimestamp - 1
	end := system.Outputs.SuperRoot(endTimestamp)

	paddingStep := func(step uint64) []byte {
		return system.Outputs.TransitionState(startTimestamp, step,
			system.Outputs.OptimisticBlockAtTimestamp(actors.ChainA, endTimestamp),
			system.Outputs.OptimisticBlockAtTimestamp(actors.ChainB, endTimestamp),
		).Marshal()
	}

	tests := []*transitionTest{
		{
			name:               "Consolidate-AllValid",
			agreedClaim:        paddingStep(1023),
			disputedClaim:      end.Marshal(),
			disputedTraceIndex: 1023,
			expectValid:        true,
		},
		{
			name:               "Consolidate-AllValid-InvalidNoChange",
			agreedClaim:        paddingStep(1023),
			disputedClaim:      paddingStep(1023),
			disputedTraceIndex: 1023,
			expectValid:        false,
		},
	}
	runFppAndChallengerTests(gt, system, tests)
}

func TestInteropFaultProofs(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	system := dsl.NewInteropDSL(t)

	system.AddL2Block(system.Actors.ChainA)
	system.AddL2Block(system.Actors.ChainB)

	// Submit batch data for each chain in separate L1 blocks so tests can have one chain safe and one unsafe
	system.SubmitBatchData(func(opts *dsl.SubmitBatchDataOpts) {
		opts.SetChains(system.Actors.ChainA)
	})
	system.SubmitBatchData(func(opts *dsl.SubmitBatchDataOpts) {
		opts.SetChains(system.Actors.ChainB)
	})

	actors := system.Actors

	endTimestamp := actors.ChainA.RollupCfg.Genesis.L2Time + actors.ChainA.RollupCfg.BlockTime
	startTimestamp := endTimestamp - 1

	start := system.Outputs.SuperRoot(startTimestamp)
	end := system.Outputs.SuperRoot(endTimestamp)

	step1Expected := system.Outputs.TransitionState(startTimestamp, 1,
		system.Outputs.OptimisticBlockAtTimestamp(actors.ChainA, endTimestamp),
	).Marshal()

	step2Expected := system.Outputs.TransitionState(startTimestamp, 2,
		system.Outputs.OptimisticBlockAtTimestamp(actors.ChainA, endTimestamp),
		system.Outputs.OptimisticBlockAtTimestamp(actors.ChainB, endTimestamp),
	).Marshal()

	paddingStep := func(step uint64) []byte {
		return system.Outputs.TransitionState(startTimestamp, step,
			system.Outputs.OptimisticBlockAtTimestamp(actors.ChainA, endTimestamp),
			system.Outputs.OptimisticBlockAtTimestamp(actors.ChainB, endTimestamp),
		).Marshal()
	}

	tests := []*transitionTest{
		{
			name:               "ClaimDirectToNextTimestamp",
			agreedClaim:        start.Marshal(),
			disputedClaim:      end.Marshal(),
			disputedTraceIndex: 0,
			expectValid:        false,
		},
		{
			name:               "FirstChainOptimisticBlock",
			agreedClaim:        start.Marshal(),
			disputedClaim:      step1Expected,
			disputedTraceIndex: 0,
			expectValid:        true,
		},
		{
			name:               "FirstChainOptimisticBlock-InvalidNoChange",
			agreedClaim:        start.Marshal(),
			disputedClaim:      start.Marshal(),
			disputedTraceIndex: 0,
			expectValid:        false,
		},
		{
			name:               "SecondChainOptimisticBlock",
			agreedClaim:        step1Expected,
			disputedClaim:      step2Expected,
			disputedTraceIndex: 1,
			expectValid:        true,
		},
		{
			name:               "SecondChainOptimisticBlock-InvalidNoChange",
			agreedClaim:        step1Expected,
			disputedClaim:      step1Expected,
			disputedTraceIndex: 1,
			expectValid:        false,
		},
		{
			name:               "FirstPaddingStep",
			agreedClaim:        step2Expected,
			disputedClaim:      paddingStep(3),
			disputedTraceIndex: 2,
			expectValid:        true,
		},
		{
			name:               "FirstPaddingStep-InvalidNoChange",
			agreedClaim:        step2Expected,
			disputedClaim:      step2Expected,
			disputedTraceIndex: 2,
			expectValid:        false,
		},
		{
			name:               "SecondPaddingStep",
			agreedClaim:        paddingStep(3),
			disputedClaim:      paddingStep(4),
			disputedTraceIndex: 3,
			expectValid:        true,
		},
		{
			name:               "SecondPaddingStep-InvalidNoChange",
			agreedClaim:        paddingStep(3),
			disputedClaim:      paddingStep(3),
			disputedTraceIndex: 3,
			expectValid:        false,
		},
		{
			name:               "LastPaddingStep",
			agreedClaim:        paddingStep(1022),
			disputedClaim:      paddingStep(1023),
			disputedTraceIndex: 1022,
			expectValid:        true,
		},
		{
			// The proposed block timestamp is after the unsafe head block timestamp.
			// Expect to transition to invalid because the unsafe head is reached but challenger needs to handle
			// not having any data at the next timestamp because the chain doesn't extend that far.
			name:        "DisputeTimestampAfterChainHeadChainA",
			agreedClaim: end.Marshal(),
			// With 2 second block times, we haven't yet reached the next block on the first chain so it's still valid
			disputedClaim: system.Outputs.TransitionState(endTimestamp, 1,
				system.Outputs.OptimisticBlockAtTimestamp(actors.ChainA, endTimestamp+1),
			).Marshal(),
			proposalTimestamp:  endTimestamp + 100,
			disputedTraceIndex: 1024,
			expectValid:        true,
		},
		{
			// The proposed block timestamp is after the unsafe head block timestamp.
			// Expect to transition to invalid because the unsafe head is reached but challenger needs to handle
			// not having any data at the next timestamp because the chain doesn't extend that far.
			name: "DisputeTimestampAfterChainHeadChainB",
			agreedClaim: system.Outputs.TransitionState(endTimestamp, 1,
				system.Outputs.OptimisticBlockAtTimestamp(actors.ChainA, endTimestamp+1),
			).Marshal(),
			// With 2 second block times, we haven't yet reached the next block on the second chain so it's still valid
			disputedClaim: system.Outputs.TransitionState(endTimestamp, 2,
				system.Outputs.OptimisticBlockAtTimestamp(actors.ChainA, endTimestamp+1),
				system.Outputs.OptimisticBlockAtTimestamp(actors.ChainB, endTimestamp+1),
			).Marshal(),
			proposalTimestamp:  endTimestamp + 100,
			disputedTraceIndex: 1025,
			expectValid:        true,
		},
		{
			// The proposed block timestamp is after the unsafe head block timestamp.
			// Expect to transition to invalid because the unsafe head is reached but challenger needs to handle
			// not having any data at the next timestamp because the chain doesn't extend that far.
			name: "DisputeTimestampAfterChainHeadConsolidate",
			agreedClaim: system.Outputs.TransitionState(endTimestamp, 1023,
				system.Outputs.OptimisticBlockAtTimestamp(actors.ChainA, endTimestamp+1),
				system.Outputs.OptimisticBlockAtTimestamp(actors.ChainB, endTimestamp+1),
			).Marshal(),
			// With 2 second block times, we haven't yet reached the next block on either chain so it's still valid
			// It will have an incremented timestamp but the same chain output roots
			disputedClaim:      system.Outputs.SuperRoot(endTimestamp + 1).Marshal(),
			proposalTimestamp:  endTimestamp + 100,
			disputedTraceIndex: 2047,
			expectValid:        true,
		},
		{
			// The proposed block timestamp is after the unsafe head block timestamp.
			// Expect to transition to invalid because the unsafe head is reached but challenger needs to handle
			// not having any data at the next timestamp because the chain doesn't extend that far.
			name:        "DisputeBlockAfterChainHead-FirstChain",
			agreedClaim: system.Outputs.SuperRoot(endTimestamp + 1).Marshal(),
			// Timestamp has advanced enough to expect the next block now, but it doesn't exit so transition to invalid
			disputedClaim:      interop.InvalidTransition,
			proposalTimestamp:  endTimestamp + 100,
			disputedTraceIndex: 2048,
			expectValid:        true,
		},
		{
			// The agreed and disputed claim are both after the current chain head
			name:               "AgreedBlockAfterChainHead-Consolidate",
			agreedClaim:        interop.InvalidTransition,
			disputedClaim:      interop.InvalidTransition,
			proposalTimestamp:  endTimestamp + 100,
			disputedTraceIndex: 3071,
			expectValid:        true,
		},
		{
			// The agreed and disputed claim are both after the current chain head and disputing an optimistic block
			name:               "AgreedBlockAfterChainHead-Optimistic",
			agreedClaim:        interop.InvalidTransition,
			disputedClaim:      interop.InvalidTransition,
			proposalTimestamp:  endTimestamp + 100,
			disputedTraceIndex: 3072,
			expectValid:        true,
		},

		{
			name:               "FirstChainReachesL1Head",
			agreedClaim:        start.Marshal(),
			disputedClaim:      interop.InvalidTransition,
			disputedTraceIndex: 0,
			// The derivation reaches the L1 head before the next block can be created
			l1Head:      actors.L1Miner.L1Chain().Genesis().Hash(),
			expectValid: true,
		},
		{
			name:               "SecondChainReachesL1Head",
			agreedClaim:        step1Expected,
			disputedClaim:      interop.InvalidTransition,
			disputedTraceIndex: 1,
			// The derivation reaches the L1 head before the next block can be created
			l1Head:      actors.L1Miner.L1Chain().GetCanonicalHash(1),
			expectValid: true,
		},
		{
			name:               "SuperRootInvalidIfUnsupportedByL1Data",
			agreedClaim:        start.Marshal(),
			disputedClaim:      step1Expected,
			disputedTraceIndex: 0,
			// The derivation reaches the L1 head before the next block can be created
			l1Head:      actors.L1Miner.L1Chain().Genesis().Hash(),
			expectValid: false,
		},
		{
			name:               "FromInvalidTransitionHash",
			agreedClaim:        interop.InvalidTransition,
			disputedClaim:      interop.InvalidTransition,
			disputedTraceIndex: 2,
			// The derivation reaches the L1 head before the next block can be created
			l1Head:      actors.L1Miner.L1Chain().Genesis().Hash(),
			expectValid: true,
		},
	}

	runFppAndChallengerTests(gt, system, tests)
}

func TestInteropFaultProofs_Cycle(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	// TODO(#14425): Handle cyclic valid messages
	t.Skip("Cyclic valid messages does not work")

	system := dsl.NewInteropDSL(t)
	actors := system.Actors

	alice := system.CreateUser()
	emitter := dsl.NewEmitterContract(t)
	system.AddL2Block(actors.ChainA, dsl.WithL2BlockTransactions(emitter.Deploy(alice)))
	system.AddL2Block(actors.ChainB, dsl.WithL2BlockTransactions(emitter.Deploy(alice)))

	assertHeads(t, actors.ChainA, 1, 0, 1, 0)
	assertHeads(t, actors.ChainB, 1, 0, 1, 0)

	actEmitA := emitter.EmitMessage(alice, "hello")
	actEmitB := emitter.EmitMessage(alice, "world")

	actors.ChainA.Sequencer.ActL2StartBlock(t)
	actors.ChainB.Sequencer.ActL2StartBlock(t)

	// create init messages
	emitTxA := actEmitA(actors.ChainA)
	emitTxA.Include()
	emitTxB := actEmitB(actors.ChainB)
	emitTxB.Include()

	// execute them within the same block
	actExecA := system.InboxContract.Execute(alice, emitTxB) // Exec msg on chain A referencing chain B
	actExecB := system.InboxContract.Execute(alice, emitTxA) // Exec msg on chain B referencing chain A
	actExecA(actors.ChainA).Include()
	actExecB(actors.ChainB).Include()

	actors.ChainA.Sequencer.ActL2EndBlock(t)
	actors.ChainB.Sequencer.ActL2EndBlock(t)
	actors.ChainA.Sequencer.SyncSupervisor(t)
	actors.ChainB.Sequencer.SyncSupervisor(t)
	actors.Supervisor.ProcessFull(t)
	actors.ChainA.Sequencer.ActL2PipelineFull(t)
	actors.ChainB.Sequencer.ActL2PipelineFull(t)

	assertHeads(t, actors.ChainA, 2, 0, 2, 0)
	assertHeads(t, actors.ChainB, 2, 0, 2, 0)

	system.SubmitBatchData()
	assertHeads(t, actors.ChainA, 2, 2, 2, 2)
	assertHeads(t, actors.ChainB, 2, 2, 2, 2)

	endTimestamp := system.Actors.ChainA.Sequencer.L2Safe().Time
	startTimestamp := endTimestamp - 1
	end := system.Outputs.SuperRoot(endTimestamp)

	paddingStep := func(step uint64) []byte {
		return system.Outputs.TransitionState(startTimestamp, step,
			system.Outputs.OptimisticBlockAtTimestamp(actors.ChainA, endTimestamp),
			system.Outputs.OptimisticBlockAtTimestamp(actors.ChainB, endTimestamp),
		).Marshal()
	}

	tests := []*transitionTest{
		{
			name:               "Consolidate-AllValid",
			agreedClaim:        paddingStep(1023),
			disputedClaim:      end.Marshal(),
			disputedTraceIndex: 1023,
			expectValid:        true,
		},
		{
			name:               "Consolidate-AllValid-InvalidNoChange",
			agreedClaim:        paddingStep(1023),
			disputedClaim:      paddingStep(1023),
			disputedTraceIndex: 1023,
			expectValid:        false,
		},
	}
	runFppAndChallengerTests(gt, system, tests)
}

func TestInteropFaultProofs_CascadeInvalidBlock(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	// TODO(#14307): Support cascading invalidation in op-supervisor
	t.Skip("Cascading invalidation not yet working")

	system := dsl.NewInteropDSL(t)

	actors := system.Actors
	alice := system.CreateUser()
	emitterContract := dsl.NewEmitterContract(t)
	// Deploy emitter contract to both chains
	system.AddL2Block(actors.ChainA, dsl.WithL2BlockTransactions(
		emitterContract.Deploy(alice),
	))
	system.AddL2Block(actors.ChainB, dsl.WithL2BlockTransactions(
		emitterContract.Deploy(alice),
	))

	// Initiating messages on chain A
	system.AddL2Block(actors.ChainA, dsl.WithL2BlockTransactions(
		emitterContract.EmitMessage(alice, "chainA message"),
	))
	chainAInitTx := emitterContract.LastEmittedMessage()
	system.AddL2Block(actors.ChainB)
	system.SubmitBatchData()

	// Create a message with a conflicting payload on chain B, that also emits an initiating message
	system.AddL2Block(actors.ChainB, dsl.WithL2BlockTransactions(
		system.InboxContract.Execute(alice, chainAInitTx, dsl.WithPayload([]byte("this message was never emitted"))),
		emitterContract.EmitMessage(alice, "chainB message"),
	), dsl.WithL1BlockCrossUnsafe())
	chainBExecTx := system.InboxContract.LastTransaction()
	chainBExecTx.CheckIncluded()
	chainBInitTx := emitterContract.LastEmittedMessage()

	// Create a message with a valid message on chain A, pointing to the initiating message on B from the same block
	// as an invalid message.
	system.AddL2Block(actors.ChainA,
		dsl.WithL2BlockTransactions(system.InboxContract.Execute(alice, chainBInitTx)),
		// Block becomes cross-unsafe because the init msg is currently present, but it should not become cross-safe.
	)
	chainAExecTx := system.InboxContract.LastTransaction()
	chainAExecTx.CheckIncluded()

	system.SubmitBatchData(func(opts *dsl.SubmitBatchDataOpts) {
		opts.SkipCrossSafeUpdate = true
	})

	endTimestamp := actors.ChainB.Sequencer.L2Unsafe().Time
	startTimestamp := endTimestamp - 1
	optimisticEnd := system.Outputs.SuperRoot(endTimestamp)

	preConsolidation := system.Outputs.TransitionState(startTimestamp, 1023,
		system.Outputs.OptimisticBlockAtTimestamp(actors.ChainA, endTimestamp),
		system.Outputs.OptimisticBlockAtTimestamp(actors.ChainB, endTimestamp),
	).Marshal()

	// Induce block replacement
	system.ProcessCrossSafe()
	// assert that the invalid message txs were reorged out
	chainBExecTx.CheckNotIncluded()
	chainBInitTx.CheckNotIncluded() // Should have been reorged out with chainBExecTx
	chainAExecTx.CheckNotIncluded() // Reorged out because chainBInitTx was reorged out

	crossSafeEnd := system.Outputs.SuperRoot(endTimestamp)

	tests := []*transitionTest{
		{
			name:               "Consolidate-ExpectInvalidPendingBlock",
			agreedClaim:        preConsolidation,
			disputedClaim:      optimisticEnd.Marshal(),
			disputedTraceIndex: 1023,
			expectValid:        false,
			// TODO(#14306): Support cascading re-orgs in op-program
			skipProgram:    true,
			skipChallenger: true,
		},
		{
			name:               "Consolidate-ReplaceInvalidBlocks",
			agreedClaim:        preConsolidation,
			disputedClaim:      crossSafeEnd.Marshal(),
			disputedTraceIndex: 1023,
			expectValid:        true,
			// TODO(#14306): Support cascading re-orgs in op-program
			skipProgram:    true,
			skipChallenger: true,
		},
	}
	runFppAndChallengerTests(gt, system, tests)
}

func TestInteropFaultProofs_MessageExpiry(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	// TODO(#14234): Check message expiry in op-supervisor
	t.Skip("Message expiry not yet implemented")

	system := dsl.NewInteropDSL(t)

	actors := system.Actors
	alice := system.CreateUser()
	emitterContract := dsl.NewEmitterContract(t)
	system.AddL2Block(actors.ChainA, dsl.WithL2BlockTransactions(
		emitterContract.Deploy(alice),
	))
	system.AddL2Block(actors.ChainA, dsl.WithL2BlockTransactions(
		emitterContract.EmitMessage(alice, "test message"),
	))
	emitTx := emitterContract.LastEmittedMessage()

	// Bring ChainB to the same height and timestamp
	system.AddL2Block(actors.ChainB, dsl.WithL2BlocksUntilTimestamp(actors.ChainA.Sequencer.L2Unsafe().Time))
	system.SubmitBatchData()

	// Advance the chain until the init msg expires
	msgExpiryTime := actors.ChainA.RollupCfg.GetMessageExpiryTimeInterop()
	end := emitTx.Identifier().Timestamp.Uint64() + msgExpiryTime
	system.AddL2Block(actors.ChainA, dsl.WithL2BlocksUntilTimestamp(end))
	system.AddL2Block(actors.ChainB, dsl.WithL2BlocksUntilTimestamp(end))
	system.SubmitBatchData()

	system.AddL2Block(actors.ChainB, func(opts *dsl.AddL2BlockOpts) {
		opts.TransactionCreators = []dsl.TransactionCreator{system.InboxContract.Execute(alice, emitTx)}
		opts.BlockIsNotCrossUnsafe = true
	})
	system.AddL2Block(actors.ChainA)

	system.SubmitBatchData(func(opts *dsl.SubmitBatchDataOpts) {
		opts.SkipCrossSafeUpdate = true
	})
	execTx := system.InboxContract.LastTransaction()
	execTx.CheckIncluded()

	endTimestamp := actors.ChainB.Sequencer.L2Unsafe().Time
	startTimestamp := endTimestamp - 1
	optimisticEnd := system.Outputs.SuperRoot(endTimestamp)

	preConsolidation := system.Outputs.TransitionState(startTimestamp, 1023,
		system.Outputs.OptimisticBlockAtTimestamp(actors.ChainA, endTimestamp),
		system.Outputs.OptimisticBlockAtTimestamp(actors.ChainB, endTimestamp),
	).Marshal()

	// Induce block replacement
	system.ProcessCrossSafe()
	// assert that the invalid message txs were reorged out
	execTx.CheckNotIncluded()
	crossSafeEnd := system.Outputs.SuperRoot(endTimestamp)

	tests := []*transitionTest{
		{
			name:               "Consolidate-ExpectInvalidPendingBlock",
			agreedClaim:        preConsolidation,
			disputedClaim:      optimisticEnd.Marshal(),
			disputedTraceIndex: 1023,
			expectValid:        false,
		},
		{
			name:               "Consolidate-ReplaceInvalidBlocks",
			agreedClaim:        preConsolidation,
			disputedClaim:      crossSafeEnd.Marshal(),
			disputedTraceIndex: 1023,
			expectValid:        true,
		},
	}
	runFppAndChallengerTests(gt, system, tests)
}

func TestInteropFaultProofsInvalidBlock(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)

	system := dsl.NewInteropDSL(t)

	actors := system.Actors
	alice := system.CreateUser()
	emitterContract := dsl.NewEmitterContract(t)
	system.AddL2Block(actors.ChainA, dsl.WithL2BlockTransactions(
		emitterContract.Deploy(alice),
	))
	system.AddL2Block(actors.ChainA, dsl.WithL2BlockTransactions(
		emitterContract.EmitMessage(alice, "test message"),
	))
	emitTx := emitterContract.LastEmittedMessage()

	// Bring ChainB to the same height and timestamp
	system.AddL2Block(actors.ChainB)
	system.AddL2Block(actors.ChainB)
	system.SubmitBatchData()

	// Create a message with a conflicting payload
	fakeMessage := []byte("this message was never emitted")
	system.AddL2Block(actors.ChainB, func(opts *dsl.AddL2BlockOpts) {
		opts.TransactionCreators = []dsl.TransactionCreator{system.InboxContract.Execute(alice, emitTx, dsl.WithPayload(fakeMessage))}
		opts.BlockIsNotCrossUnsafe = true
	})
	system.AddL2Block(actors.ChainA)

	system.SubmitBatchData(func(opts *dsl.SubmitBatchDataOpts) {
		opts.SkipCrossSafeUpdate = true
	})

	execTx := system.InboxContract.LastTransaction()
	execTx.CheckIncluded()

	// safe head is still behind until we verify cross-safe
	assertHeads(t, actors.ChainA, 3, 3, 3, 2) // Chain A's block is cross unsafe
	assertHeads(t, actors.ChainB, 3, 3, 2, 2) // Chain B's block is not
	endTimestamp := actors.ChainB.Sequencer.L2Unsafe().Time

	startTimestamp := endTimestamp - 1
	start := system.Outputs.SuperRoot(startTimestamp)
	end := system.Outputs.SuperRoot(endTimestamp)

	step1Expected := system.Outputs.TransitionState(startTimestamp, 1,
		system.Outputs.OptimisticBlockAtTimestamp(actors.ChainA, endTimestamp),
	).Marshal()

	// Capture optimistic blocks now before the invalid block is reorg'd out
	// Otherwise later calls to paddingStep would incorrectly use the deposit-only block
	allOptimisticBlocks := []types.OptimisticBlock{
		system.Outputs.OptimisticBlockAtTimestamp(actors.ChainA, endTimestamp),
		system.Outputs.OptimisticBlockAtTimestamp(actors.ChainB, endTimestamp),
	}
	step2Expected := system.Outputs.TransitionState(startTimestamp, 2,
		allOptimisticBlocks...,
	).Marshal()

	paddingStep := func(step uint64) []byte {
		return system.Outputs.TransitionState(startTimestamp, step, allOptimisticBlocks...).Marshal()
	}

	// Induce block replacement
	system.ProcessCrossSafe()
	// assert that the invalid message tx was reorged out
	execTx.CheckNotIncluded()
	assertHeads(t, actors.ChainA, 3, 3, 3, 3)
	assertHeads(t, actors.ChainB, 3, 3, 3, 3)

	crossSafeSuperRootEnd := system.Outputs.SuperRoot(endTimestamp)

	tests := []*transitionTest{
		{
			name:               "FirstChainOptimisticBlock",
			agreedClaim:        start.Marshal(),
			disputedClaim:      step1Expected,
			disputedTraceIndex: 0,
			expectValid:        true,
		},
		{
			name:               "SecondChainOptimisticBlock",
			agreedClaim:        step1Expected,
			disputedClaim:      step2Expected,
			disputedTraceIndex: 1,
			expectValid:        true,
		},
		{
			name:               "FirstPaddingStep",
			agreedClaim:        step2Expected,
			disputedClaim:      paddingStep(3),
			disputedTraceIndex: 2,
			expectValid:        true,
		},
		{
			name:               "SecondPaddingStep",
			agreedClaim:        paddingStep(3),
			disputedClaim:      paddingStep(4),
			disputedTraceIndex: 3,
			expectValid:        true,
		},
		{
			name:               "LastPaddingStep",
			agreedClaim:        paddingStep(1022),
			disputedClaim:      paddingStep(1023),
			disputedTraceIndex: 1022,
			expectValid:        true,
		},
		{
			name:               "Consolidate-ExpectInvalidPendingBlock",
			agreedClaim:        paddingStep(1023),
			disputedClaim:      end.Marshal(),
			disputedTraceIndex: 1023,
			expectValid:        false,
		},
		{
			name:               "Consolidate-ReplaceInvalidBlock",
			agreedClaim:        paddingStep(1023),
			disputedClaim:      crossSafeSuperRootEnd.Marshal(),
			disputedTraceIndex: 1023,
			expectValid:        true,
		},
		{
			name:               "AlreadyAtClaimedTimestamp",
			agreedClaim:        crossSafeSuperRootEnd.Marshal(),
			disputedClaim:      crossSafeSuperRootEnd.Marshal(),
			disputedTraceIndex: 5000,
			expectValid:        true,
		},

		{
			name:               "FirstChainReachesL1Head",
			agreedClaim:        start.Marshal(),
			disputedClaim:      interop.InvalidTransition,
			disputedTraceIndex: 0,
			// The derivation reaches the L1 head before the next block can be created
			l1Head:      actors.L1Miner.L1Chain().Genesis().Hash(),
			expectValid: true,
		},
		{
			name:               "SuperRootInvalidIfUnsupportedByL1Data",
			agreedClaim:        start.Marshal(),
			disputedClaim:      step1Expected,
			disputedTraceIndex: 0,
			// The derivation reaches the L1 head before the next block can be created
			l1Head:      actors.L1Miner.L1Chain().Genesis().Hash(),
			expectValid: false,
		},
		{
			name:               "FromInvalidTransitionHash",
			agreedClaim:        interop.InvalidTransition,
			disputedClaim:      interop.InvalidTransition,
			disputedTraceIndex: 2,
			// The derivation reaches the L1 head before the next block can be created
			l1Head:      actors.L1Miner.L1Chain().Genesis().Hash(),
			expectValid: true,
		},
	}

	runFppAndChallengerTests(gt, system, tests)
}

func runFppAndChallengerTests(gt *testing.T, system *dsl.InteropDSL, tests []*transitionTest) {
	for _, test := range tests {
		test := test
		gt.Run(fmt.Sprintf("%s-fpp", test.name), func(gt *testing.T) {
			runFppTest(gt, test, system.Actors)
		})

		gt.Run(fmt.Sprintf("%s-challenger", test.name), func(gt *testing.T) {
			runChallengerTest(gt, test, system.Actors)
		})
	}
}

func runFppTest(gt *testing.T, test *transitionTest, actors *dsl.InteropActors) {
	t := helpers.NewDefaultTesting(gt)
	if test.skipProgram {
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
	proposalTimestamp := test.proposalTimestamp
	if proposalTimestamp == 0 {
		proposalTimestamp = actors.ChainA.Sequencer.L2Unsafe().Time
	}
	fpHelpers.RunFaultProofProgram(
		t,
		logger,
		actors.L1Miner,
		checkResult,
		WithInteropEnabled(t, actors, test.agreedClaim, crypto.Keccak256Hash(test.disputedClaim), proposalTimestamp),
		fpHelpers.WithL1Head(l1Head),
	)
}

func runChallengerTest(gt *testing.T, test *transitionTest, actors *dsl.InteropActors) {
	t := helpers.NewDefaultTesting(gt)
	if test.skipChallenger {
		t.Skip("Not yet implemented")
		return
	}
	logger := testlog.Logger(t, slog.LevelInfo)
	endTimestamp := test.proposalTimestamp
	if endTimestamp == 0 {
		endTimestamp = actors.ChainA.Sequencer.L2Unsafe().Time
	}
	startTimestamp := actors.ChainA.Sequencer.L2Unsafe().Time - 1
	prestateProvider := super.NewSuperRootPrestateProvider(actors.Supervisor, startTimestamp)
	var l1Head eth.BlockID
	if test.l1Head == (common.Hash{}) {
		l1Head = eth.ToBlockID(eth.HeaderBlockInfo(actors.L1Miner.L1Chain().CurrentBlock()))
	} else {
		l1Head = eth.ToBlockID(actors.L1Miner.L1Chain().GetBlockByHash(test.l1Head))
	}
	gameDepth := challengerTypes.Depth(30)
	rollupCfgs, err := super.NewRollupConfigsFromParsed(actors.ChainA.RollupCfg, actors.ChainB.RollupCfg)
	require.NoError(t, err)
	provider := super.NewSuperTraceProvider(logger, rollupCfgs, prestateProvider, actors.Supervisor, l1Head, gameDepth, startTimestamp, endTimestamp)
	var agreedPrestate []byte
	if test.disputedTraceIndex > 0 {
		agreedPrestate, err = provider.GetPreimageBytes(t.Ctx(), challengerTypes.NewPosition(gameDepth, big.NewInt(test.disputedTraceIndex-1)))
		require.NoError(t, err)
	} else {
		superRoot, err := provider.AbsolutePreState(t.Ctx())
		require.NoError(t, err)
		agreedPrestate = superRoot.Marshal()
	}
	require.Equal(t, test.agreedClaim, agreedPrestate, "agreed prestate mismatch")

	disputedClaim, err := provider.GetPreimageBytes(t.Ctx(), challengerTypes.NewPosition(gameDepth, big.NewInt(test.disputedTraceIndex)))
	require.NoError(t, err)
	if test.expectValid {
		require.Equal(t, test.disputedClaim, disputedClaim, "Claim is correct so should match challenger's opinion")
	} else {
		require.NotEqual(t, test.disputedClaim, disputedClaim, "Claim is incorrect so should not match challenger's opinion")
	}
}

func WithInteropEnabled(t helpers.StatefulTesting, actors *dsl.InteropActors, agreedPrestate []byte, disputedClaim common.Hash, claimTimestamp uint64) fpHelpers.FixtureInputParam {
	return func(f *fpHelpers.FixtureInputs) {
		f.InteropEnabled = true
		f.AgreedPrestate = agreedPrestate
		f.L2OutputRoot = crypto.Keccak256Hash(agreedPrestate)
		f.L2Claim = disputedClaim
		f.L2BlockNumber = claimTimestamp

		deps := map[eth.ChainID]*depset.StaticConfigDependency{
			actors.ChainA.ChainID: {
				ChainIndex:     supervisortypes.ChainIndex(0),
				ActivationTime: 0,
				HistoryMinTime: 0,
			},
			actors.ChainB.ChainID: {
				ChainIndex:     supervisortypes.ChainIndex(1),
				ActivationTime: 0,
				HistoryMinTime: 0,
			},
		}
		var err error
		f.DependencySet, err = depset.NewStaticConfigDependencySet(deps)
		require.NoError(t, err)

		for _, chain := range []*dsl.Chain{actors.ChainA, actors.ChainB} {
			f.L2Sources = append(f.L2Sources, &fpHelpers.FaultProofProgramL2Source{
				Node:        chain.Sequencer.L2Verifier,
				Engine:      chain.SequencerEngine,
				ChainConfig: chain.L2Genesis.Config,
			})
		}
	}
}

type transitionTest struct {
	name               string
	agreedClaim        []byte
	disputedClaim      []byte
	disputedTraceIndex int64
	l1Head             common.Hash // Defaults to current L1 head if not set
	proposalTimestamp  uint64      // Defaults to latest L2 block timestamp if 0
	expectValid        bool
	skipProgram        bool
	skipChallenger     bool
}
