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
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

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

	start := system.SuperRoot(startTimestamp)
	end := system.SuperRoot(endTimestamp)

	chain1End := system.OutputRootAtTimestamp(actors.ChainA, endTimestamp)
	chain2End := system.OutputRootAtTimestamp(actors.ChainB, endTimestamp)

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
			name:               "ClaimNoChange",
			agreedClaim:        start.Marshal(),
			disputedClaim:      start.Marshal(),
			disputedTraceIndex: 0,
			expectValid:        false,
		},
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
			name:               "Consolidate-AllValid",
			agreedClaim:        paddingStep(1023),
			disputedClaim:      end.Marshal(),
			disputedTraceIndex: 1023,
			expectValid:        true,
		},
		{
			name:               "AlreadyAtClaimedTimestamp",
			agreedClaim:        end.Marshal(),
			disputedClaim:      end.Marshal(),
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

	for _, test := range tests {
		test := test
		gt.Run(fmt.Sprintf("%s-fpp", test.name), func(gt *testing.T) {
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
			fpHelpers.RunFaultProofProgram(
				t,
				logger,
				actors.L1Miner,
				checkResult,
				WithInteropEnabled(actors, test.agreedClaim, crypto.Keccak256Hash(test.disputedClaim), endTimestamp),
				fpHelpers.WithL1Head(l1Head),
			)
		})

		gt.Run(fmt.Sprintf("%s-challenger", test.name), func(gt *testing.T) {
			t := helpers.NewDefaultTesting(gt)
			if test.skipChallenger {
				t.Skip("Not yet implemented")
				return
			}
			logger := testlog.Logger(t, slog.LevelInfo)
			prestateProvider := super.NewSuperRootPrestateProvider(&actors.Supervisor.QueryFrontend, startTimestamp)
			var l1Head eth.BlockID
			if test.l1Head == (common.Hash{}) {
				l1Head = eth.ToBlockID(eth.HeaderBlockInfo(actors.L1Miner.L1Chain().CurrentBlock()))
			} else {
				l1Head = eth.ToBlockID(actors.L1Miner.L1Chain().GetBlockByHash(test.l1Head))
			}
			gameDepth := challengerTypes.Depth(30)
			rollupCfgs, err := super.NewRollupConfigsFromParsed(actors.ChainA.RollupCfg, actors.ChainB.RollupCfg)
			require.NoError(t, err)
			provider := super.NewSuperTraceProvider(logger, rollupCfgs, prestateProvider, &actors.Supervisor.QueryFrontend, l1Head, gameDepth, startTimestamp, endTimestamp)
			var agreedPrestate []byte
			if test.disputedTraceIndex > 0 {
				agreedPrestate, err = provider.GetPreimageBytes(t.Ctx(), challengerTypes.NewPosition(gameDepth, big.NewInt(test.disputedTraceIndex-1)))
				require.NoError(t, err)
			} else {
				superRoot, err := provider.AbsolutePreState(t.Ctx())
				require.NoError(t, err)
				agreedPrestate = superRoot.Marshal()
			}
			require.Equal(t, test.agreedClaim, agreedPrestate)

			disputedClaim, err := provider.GetPreimageBytes(t.Ctx(), challengerTypes.NewPosition(gameDepth, big.NewInt(test.disputedTraceIndex)))
			require.NoError(t, err)
			if test.expectValid {
				require.Equal(t, test.disputedClaim, disputedClaim, "Claim is correct so should match challenger's opinion")
			} else {
				require.NotEqual(t, test.disputedClaim, disputedClaim, "Claim is incorrect so should not match challenger's opinion")
			}
		})
	}
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
	inboxContract := dsl.NewInboxContract(t)
	system.AddL2Block(actors.ChainB, func(opts *dsl.AddL2BlockOpts) {
		opts.TransactionCreators = []dsl.TransactionCreator{inboxContract.Execute(alice, emitTx.Identifier(), fakeMessage)}
		opts.BlockIsNotCrossSafe = true
	})
	system.AddL2Block(actors.ChainA)

	// TODO: I wonder if it would be better to have `opts.ExpectInvalid` that specifies the invalid tx
	// then the DSL can assert that it becomes local safe and is then reorged out automatically
	// We could still grab the superroot and output roots for the invalid block while it is unsafe
	// Other tests may still want to have SkipCrossUnsafeUpdate but generally nicer to be more declarative and
	// high level to avoid leaking the details of when supervisor will trigger the reorg if possible.
	system.SubmitBatchData(func(opts *dsl.SubmitBatchDataOpts) {
		opts.SkipCrossSafeUpdate = true
	})

	execTx := inboxContract.LastTransaction()
	execTx.CheckIncluded()

	// safe head is still behind until we verify cross-safe
	assertHeads(t, actors.ChainA, 3, 3, 3, 2) // Chain A's block is cross unsafe
	assertHeads(t, actors.ChainB, 3, 3, 2, 2) // Chain B's block is not
	endTimestamp := actors.ChainB.Sequencer.L2Unsafe().Time

	startTimestamp := endTimestamp - 1
	start := system.SuperRoot(startTimestamp)
	end := system.SuperRoot(endTimestamp)

	chain1End := system.OutputRootAtTimestamp(actors.ChainA, endTimestamp)
	chain2End := system.OutputRootAtTimestamp(actors.ChainB, endTimestamp)

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

	// Induce block replacement
	system.ProcessCrossSafe()
	// assert that the invalid message tx was reorged out
	execTx.CheckNotIncluded()
	assertHeads(t, actors.ChainA, 3, 3, 3, 3)
	assertHeads(t, actors.ChainB, 3, 3, 3, 3)

	crossSafeSuperRootEnd := system.SuperRoot(endTimestamp)

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
			// skipChallenger because the challenger's reorg view won't match the pre-reorg disputed claim
			skipChallenger: true,
		},
		{
			name:               "FirstPaddingStep",
			agreedClaim:        step2Expected,
			disputedClaim:      paddingStep(3),
			disputedTraceIndex: 2,
			expectValid:        true,
			// skipChallenger because the challenger's reorg view won't match the pre-reorg disputed claim
			skipChallenger: true,
		},
		{
			name:               "SecondPaddingStep",
			agreedClaim:        paddingStep(3),
			disputedClaim:      paddingStep(4),
			disputedTraceIndex: 3,
			expectValid:        true,
			// skipChallenger because the challenger's reorg view won't match the pre-reorg disputed claim
			skipChallenger: true,
		},
		{
			name:               "LastPaddingStep",
			agreedClaim:        paddingStep(1022),
			disputedClaim:      paddingStep(1023),
			disputedTraceIndex: 1022,
			expectValid:        true,
			// skipChallenger because the challenger's reorg view won't match the pre-reorg disputed claim
			skipChallenger: true,
		},
		{
			name:               "Consolidate-ExpectInvalidPendingBlock",
			agreedClaim:        paddingStep(1023),
			disputedClaim:      end.Marshal(),
			disputedTraceIndex: 1023,
			expectValid:        false,
			skipProgram:        true,
			skipChallenger:     true,
		},
		{
			name:               "Consolidate-ReplaceInvalidBlock",
			agreedClaim:        paddingStep(1023),
			disputedClaim:      crossSafeSuperRootEnd.Marshal(),
			disputedTraceIndex: 1023,
			expectValid:        true,
			skipProgram:        true,
			skipChallenger:     true,
		},
		{
			name: "Consolidate-ReplaceBlockInvalidatedByFirstInvalidatedBlock",
			// Will need to generate an invalid block before this can be enabled
			// Check that if a block B depends on a log in block A, and block A is found to have an invalid message
			// that block B is also replaced with a deposit only block because A no longer contains the log it needs
			skipProgram:    true,
			skipChallenger: true,
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

	for _, test := range tests {
		test := test
		gt.Run(fmt.Sprintf("%s-fpp", test.name), func(gt *testing.T) {
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
			fpHelpers.RunFaultProofProgram(
				t,
				logger,
				actors.L1Miner,
				checkResult,
				WithInteropEnabled(actors, test.agreedClaim, crypto.Keccak256Hash(test.disputedClaim), endTimestamp),
				fpHelpers.WithL1Head(l1Head),
			)
		})

		gt.Run(fmt.Sprintf("%s-challenger", test.name), func(gt *testing.T) {
			t := helpers.NewDefaultTesting(gt)
			if test.skipChallenger {
				t.Skip("Not yet implemented")
				return
			}
			logger := testlog.Logger(t, slog.LevelInfo)
			prestateProvider := super.NewSuperRootPrestateProvider(&actors.Supervisor.QueryFrontend, startTimestamp)
			var l1Head eth.BlockID
			if test.l1Head == (common.Hash{}) {
				l1Head = eth.ToBlockID(eth.HeaderBlockInfo(actors.L1Miner.L1Chain().CurrentBlock()))
			} else {
				l1Head = eth.ToBlockID(actors.L1Miner.L1Chain().GetBlockByHash(test.l1Head))
			}
			gameDepth := challengerTypes.Depth(30)
			rollupCfgs, err := super.NewRollupConfigsFromParsed(actors.ChainA.RollupCfg, actors.ChainB.RollupCfg)
			require.NoError(t, err)
			provider := super.NewSuperTraceProvider(logger, rollupCfgs, prestateProvider, &actors.Supervisor.QueryFrontend, l1Head, gameDepth, startTimestamp, endTimestamp)
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
		})
	}
}

func WithInteropEnabled(actors *dsl.InteropActors, agreedPrestate []byte, disputedClaim common.Hash, claimTimestamp uint64) fpHelpers.FixtureInputParam {
	return func(f *fpHelpers.FixtureInputs) {
		f.InteropEnabled = true
		f.AgreedPrestate = agreedPrestate
		f.L2OutputRoot = crypto.Keccak256Hash(agreedPrestate)
		f.L2Claim = disputedClaim
		f.L2BlockNumber = claimTimestamp

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
	expectValid        bool
	skipProgram        bool
	skipChallenger     bool
}
