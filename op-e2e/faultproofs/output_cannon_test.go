package faultproofs

import (
	"context"
	"fmt"
	"testing"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/utils"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	gameTypes "github.com/ethereum-optimism/optimism/op-challenger/game/types"
	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"
	"github.com/ethereum-optimism/optimism/op-e2e/config"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/challenger"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/disputegame"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/disputegame/preimage"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestOutputCannonGame_Standard(t *testing.T) {
	testOutputCannonGame(t, config.AllocTypeStandard)
}

func TestOutputCannonGame_Multithreaded(t *testing.T) {
	testOutputCannonGame(t, config.AllocTypeMTCannon)
}

func testOutputCannonGame(t *testing.T, allocType config.AllocType) {
	op_e2e.InitParallel(t, op_e2e.UsesCannon)
	ctx := context.Background()
	sys, l1Client := StartFaultDisputeSystem(t, WithAllocType(allocType))
	t.Cleanup(sys.Close)

	disputeGameFactory := disputegame.NewFactoryHelper(t, ctx, sys)
	game := disputeGameFactory.StartOutputCannonGame(ctx, "sequencer", 4, common.Hash{0x01})
	game.LogGameData(ctx)

	game.StartChallenger(ctx, "Challenger", challenger.WithPrivKey(sys.Cfg.Secrets.Alice))

	game.LogGameData(ctx)

	// Challenger should post an output root to counter claims down to the leaf level of the top game
	claim := game.RootClaim(ctx)
	for claim.IsOutputRoot(ctx) && !claim.IsOutputRootLeaf(ctx) {
		if claim.AgreesWithOutputRoot() {
			// If the latest claim agrees with the output root, expect the honest challenger to counter it
			claim = claim.WaitForCounterClaim(ctx)
			game.LogGameData(ctx)
			claim.RequireCorrectOutputRoot(ctx)
		} else {
			// Otherwise we should counter
			claim = claim.Attack(ctx, common.Hash{0xaa})
			game.LogGameData(ctx)
		}
	}

	// Wait for the challenger to post the first claim in the cannon trace
	claim = claim.WaitForCounterClaim(ctx)
	game.LogGameData(ctx)

	// Attack the root of the cannon trace subgame
	claim = claim.Attack(ctx, common.Hash{0x00, 0xcc})
	for !claim.IsMaxDepth(ctx) {
		if claim.AgreesWithOutputRoot() {
			// If the latest claim supports the output root, wait for the honest challenger to respond
			claim = claim.WaitForCounterClaim(ctx)
			game.LogGameData(ctx)
		} else {
			// Otherwise we need to counter the honest claim
			claim = claim.Defend(ctx, common.Hash{0x00, 0xdd})
			game.LogGameData(ctx)
		}
	}
	// Challenger should be able to call step and counter the leaf claim.
	claim.WaitForCountered(ctx)
	game.LogGameData(ctx)

	sys.TimeTravelClock.AdvanceTime(game.MaxClockDuration(ctx))
	require.NoError(t, wait.ForNextBlock(ctx, l1Client))
	game.WaitForGameStatus(ctx, gameTypes.GameStatusChallengerWon)
}

func TestOutputCannon_ChallengeAllZeroClaim_Standard(t *testing.T) {
	testOutputCannonChallengeAllZeroClaim(t, config.AllocTypeStandard)
}

func TestOutputCannon_ChallengeAllZeroClaim_Multithreaded(t *testing.T) {
	testOutputCannonChallengeAllZeroClaim(t, config.AllocTypeMTCannon)
}

func testOutputCannonChallengeAllZeroClaim(t *testing.T, allocType config.AllocType) {
	// The dishonest actor always posts claims with all zeros.
	op_e2e.InitParallel(t, op_e2e.UsesCannon)
	ctx := context.Background()
	sys, l1Client := StartFaultDisputeSystem(t, WithAllocType(allocType))
	t.Cleanup(sys.Close)

	disputeGameFactory := disputegame.NewFactoryHelper(t, ctx, sys)
	game := disputeGameFactory.StartOutputCannonGame(ctx, "sequencer", 3, common.Hash{})
	game.LogGameData(ctx)

	claim := game.DisputeLastBlock(ctx)
	game.StartChallenger(ctx, "Challenger", challenger.WithPrivKey(sys.Cfg.Secrets.Alice))

	game.DefendClaim(ctx, claim, func(parent *disputegame.ClaimHelper) *disputegame.ClaimHelper {
		if parent.IsBottomGameRoot(ctx) {
			return parent.Attack(ctx, common.Hash{})
		}
		return parent.Defend(ctx, common.Hash{})
	})

	game.LogGameData(ctx)

	sys.TimeTravelClock.AdvanceTime(game.MaxClockDuration(ctx))
	require.NoError(t, wait.ForNextBlock(ctx, l1Client))
	game.WaitForGameStatus(ctx, gameTypes.GameStatusChallengerWon)
	game.LogGameData(ctx)
}

func TestOutputCannon_PublishCannonRootClaim_Standard(t *testing.T) {
	testOutputCannonPublishCannonRootClaim(t, config.AllocTypeStandard)
}

func TestOutputCannon_PublishCannonRootClaim_Multithreaded(t *testing.T) {
	testOutputCannonPublishCannonRootClaim(t, config.AllocTypeMTCannon)
}

func testOutputCannonPublishCannonRootClaim(t *testing.T, allocType config.AllocType) {
	op_e2e.InitParallel(t, op_e2e.UsesCannon)
	tests := []struct {
		disputeL2BlockNumber uint64
	}{
		{7}, // Post-state output root is invalid
		{8}, // Post-state output root is valid
	}
	for _, test := range tests {
		test := test
		t.Run(fmt.Sprintf("Dispute_%v", test.disputeL2BlockNumber), func(t *testing.T) {
			op_e2e.InitParallel(t, op_e2e.UsesCannon)

			ctx := context.Background()
			sys, _ := StartFaultDisputeSystem(t, WithAllocType(allocType))

			disputeGameFactory := disputegame.NewFactoryHelper(t, ctx, sys)
			game := disputeGameFactory.StartOutputCannonGame(ctx, "sequencer", test.disputeL2BlockNumber, common.Hash{0x01})
			game.DisputeLastBlock(ctx)
			game.LogGameData(ctx)

			game.StartChallenger(ctx, "Challenger", challenger.WithPrivKey(sys.Cfg.Secrets.Alice))

			splitDepth := game.SplitDepth(ctx)
			game.WaitForClaimAtDepth(ctx, splitDepth+1)
		})
	}
}

func TestOutputCannonDisputeGame_Standard(t *testing.T) {
	testOutputCannonDisputeGame(t, config.AllocTypeStandard)
}

func TestOutputCannonDisputeGame_Multithreaded(t *testing.T) {
	testOutputCannonDisputeGame(t, config.AllocTypeMTCannon)
}

func testOutputCannonDisputeGame(t *testing.T, allocType config.AllocType) {

	op_e2e.InitParallel(t, op_e2e.UsesCannon)
	tests := []struct {
		name             string
		defendClaimDepth types.Depth
	}{
		{"StepFirst", 0},
		{"StepMiddle", 28},
		{"StepInExtension", 1},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			op_e2e.InitParallel(t, op_e2e.UsesCannon)

			ctx := context.Background()
			sys, l1Client := StartFaultDisputeSystem(t, WithAllocType(allocType))
			t.Cleanup(sys.Close)

			disputeGameFactory := disputegame.NewFactoryHelper(t, ctx, sys)
			game := disputeGameFactory.StartOutputCannonGame(ctx, "sequencer", 1, common.Hash{0x01, 0xaa})
			require.NotNil(t, game)
			game.LogGameData(ctx)

			outputClaim := game.DisputeLastBlock(ctx)
			splitDepth := game.SplitDepth(ctx)

			game.StartChallenger(ctx, "Challenger", challenger.WithPrivKey(sys.Cfg.Secrets.Alice))

			game.DefendClaim(
				ctx,
				outputClaim,
				func(claim *disputegame.ClaimHelper) *disputegame.ClaimHelper {
					if claim.Depth()+1 == splitDepth+test.defendClaimDepth {
						return claim.Defend(ctx, common.Hash{byte(claim.Depth())})
					} else {
						return claim.Attack(ctx, common.Hash{byte(claim.Depth())})
					}
				})

			sys.TimeTravelClock.AdvanceTime(game.MaxClockDuration(ctx))
			require.NoError(t, wait.ForNextBlock(ctx, l1Client))

			game.LogGameData(ctx)
			game.WaitForGameStatus(ctx, gameTypes.GameStatusChallengerWon)
		})
	}
}

func TestOutputCannonDefendStep_Standard(t *testing.T) {
	testOutputCannonDefendStep(t, config.AllocTypeStandard)
}

func TestOutputCannonDefendStep_Multithreaded(t *testing.T) {
	testOutputCannonDefendStep(t, config.AllocTypeMTCannon)
}

func testOutputCannonDefendStep(t *testing.T, allocType config.AllocType) {
	op_e2e.InitParallel(t, op_e2e.UsesCannon)

	ctx := context.Background()
	sys, l1Client := StartFaultDisputeSystem(t, WithAllocType(allocType))
	t.Cleanup(sys.Close)

	disputeGameFactory := disputegame.NewFactoryHelper(t, ctx, sys)
	game := disputeGameFactory.StartOutputCannonGame(ctx, "sequencer", 1, common.Hash{0x01, 0xaa})
	require.NotNil(t, game)
	outputRootClaim := game.DisputeLastBlock(ctx)
	game.LogGameData(ctx)

	game.StartChallenger(ctx, "Challenger", challenger.WithPrivKey(sys.Cfg.Secrets.Alice))

	correctTrace := game.CreateHonestActor(ctx, "sequencer", disputegame.WithPrivKey(sys.Cfg.Secrets.Mallory))

	maxDepth := game.MaxDepth(ctx)
	game.DefendClaim(ctx, outputRootClaim, func(claim *disputegame.ClaimHelper) *disputegame.ClaimHelper {
		// Post invalid claims for most steps to get down into the early part of the trace
		if claim.Depth() < maxDepth-3 {
			return claim.Attack(ctx, common.Hash{0xaa})
		} else {
			// Post our own counter but using the correct hash in low levels to force a defense step
			return correctTrace.AttackClaim(ctx, claim)
		}
	})

	sys.TimeTravelClock.AdvanceTime(game.MaxClockDuration(ctx))
	require.NoError(t, wait.ForNextBlock(ctx, l1Client))

	game.WaitForGameStatus(ctx, gameTypes.GameStatusChallengerWon)
}

func TestOutputCannonStepWithLargePreimage_Standard(t *testing.T) {
	testOutputCannonStepWithLargePreimage(t, config.AllocTypeStandard)
}

func TestOutputCannonStepWithLargePreimage_Multithreaded(t *testing.T) {
	testOutputCannonStepWithLargePreimage(t, config.AllocTypeMTCannon)
}

func testOutputCannonStepWithLargePreimage(t *testing.T, allocType config.AllocType) {
	op_e2e.InitParallel(t, op_e2e.UsesCannon)

	ctx := context.Background()
	sys, _ := StartFaultDisputeSystem(t, WithBatcherStopped(), WithAllocType(allocType))
	t.Cleanup(sys.Close)

	// Manually send a tx from the correct batcher key to the batcher input with very large (invalid) data
	// This forces op-program to load a large preimage.
	sys.BatcherHelper().SendLargeInvalidBatch(ctx)

	require.NoError(t, sys.BatchSubmitter.Start(ctx))

	safeHead, err := wait.ForNextSafeBlock(ctx, sys.NodeClient("sequencer"))
	require.NoError(t, err, "Batcher should resume submitting valid batches")

	l2BlockNumber := safeHead.NumberU64()
	disputeGameFactory := disputegame.NewFactoryHelper(t, ctx, sys)
	// Dispute any block - it will have to read the L1 batches to see if the block is reached
	game := disputeGameFactory.StartOutputCannonGame(ctx, "sequencer", l2BlockNumber, common.Hash{0x01, 0xaa})
	require.NotNil(t, game)
	outputRootClaim := game.DisputeBlock(ctx, l2BlockNumber)
	game.LogGameData(ctx)

	game.StartChallenger(ctx, "Challenger", challenger.WithPrivKey(sys.Cfg.Secrets.Alice))

	// Wait for the honest challenger to dispute the outputRootClaim.
	// This creates a root of an execution game that we challenge by
	// coercing a step at a preimage trace index.
	outputRootClaim = outputRootClaim.WaitForCounterClaim(ctx)

	game.LogGameData(ctx)
	// Now the honest challenger is positioned as the defender of the
	// execution game. We then move to challenge it to induce a large preimage load.
	sender := sys.Cfg.Secrets.Addresses().Alice
	preimageLoadCheck := game.CreateStepLargePreimageLoadCheck(ctx, sender)
	game.ChallengeToPreimageLoad(ctx, outputRootClaim, sys.Cfg.Secrets.Alice, utils.PreimageLargerThan(preimage.MinPreimageSize), preimageLoadCheck, false)
	// The above method already verified the image was uploaded and step called successfully
	// So we don't waste time resolving the game - that's tested elsewhere.
}

func TestOutputCannonStepWithPreimage_Standard(t *testing.T) {
	testOutputCannonStepWithPreimage(t, config.AllocTypeStandard)
}

func TestOutputCannonStepWithPreimage_Multithreaded(t *testing.T) {
	testOutputCannonStepWithPreimage(t, config.AllocTypeMTCannon)
}

func testOutputCannonStepWithPreimage(t *testing.T, allocType config.AllocType) {
	op_e2e.InitParallel(t, op_e2e.UsesCannon)
	testPreimageStep := func(t *testing.T, preimageType utils.PreimageOpt, preloadPreimage bool) {
		op_e2e.InitParallel(t, op_e2e.UsesCannon)

		ctx := context.Background()
		sys, _ := StartFaultDisputeSystem(t, WithBlobBatches(), WithAllocType(allocType))
		t.Cleanup(sys.Close)

		disputeGameFactory := disputegame.NewFactoryHelper(t, ctx, sys)
		game := disputeGameFactory.StartOutputCannonGame(ctx, "sequencer", 1, common.Hash{0x01, 0xaa})
		require.NotNil(t, game)
		outputRootClaim := game.DisputeLastBlock(ctx)
		game.LogGameData(ctx)

		game.StartChallenger(ctx, "Challenger", challenger.WithPrivKey(sys.Cfg.Secrets.Alice))

		// Wait for the honest challenger to dispute the outputRootClaim. This creates a root of an execution game that we challenge by coercing
		// a step at a preimage trace index.
		outputRootClaim = outputRootClaim.WaitForCounterClaim(ctx)

		// Now the honest challenger is positioned as the defender of the execution game
		// We then move to challenge it to induce a preimage load
		preimageLoadCheck := game.CreateStepPreimageLoadCheck(ctx)
		game.ChallengeToPreimageLoad(ctx, outputRootClaim, sys.Cfg.Secrets.Alice, preimageType, preimageLoadCheck, preloadPreimage)
		// The above method already verified the image was uploaded and step called successfully
		// So we don't waste time resolving the game - that's tested elsewhere.
	}

	preimageConditions := []string{"keccak", "sha256", "blob"}
	for _, preimageType := range preimageConditions {
		preimageType := preimageType
		t.Run("non-existing preimage-"+preimageType, func(t *testing.T) {
			testPreimageStep(t, utils.FirstPreimageLoadOfType(preimageType), false)
		})
	}
	// Only test pre-existing images with one type to save runtime
	t.Run("preimage already exists", func(t *testing.T) {
		testPreimageStep(t, utils.FirstKeccakPreimageLoad(), true)
	})
}

func TestOutputCannonStepWithKZGPointEvaluation_Standard(t *testing.T) {
	testOutputCannonStepWithKzgPointEvaluation(t, config.AllocTypeStandard)
}

func TestOutputCannonStepWithKZGPointEvaluation_Multithreaded(t *testing.T) {
	testOutputCannonStepWithKzgPointEvaluation(t, config.AllocTypeMTCannon)
}

func testOutputCannonStepWithKzgPointEvaluation(t *testing.T, allocType config.AllocType) {
	op_e2e.InitParallel(t, op_e2e.UsesCannon)

	testPreimageStep := func(t *testing.T, preloadPreimage bool) {
		op_e2e.InitParallel(t, op_e2e.UsesCannon)

		ctx := context.Background()
		sys, _ := StartFaultDisputeSystem(t, WithEcotone(), WithAllocType(allocType))
		t.Cleanup(sys.Close)

		// NOTE: Flake prevention
		// Ensure that the L1 origin including the point eval tx isn't on the genesis epoch.
		safeBlock, err := wait.ForNextSafeBlock(ctx, sys.NodeClient("sequencer"))
		require.NoError(t, err)
		require.NoError(t, wait.ForSafeBlock(ctx, sys.RollupClient("sequencer"), safeBlock.NumberU64()+3))

		receipt := SendKZGPointEvaluationTx(t, sys, "sequencer", sys.Cfg.Secrets.Alice)
		precompileBlock := receipt.BlockNumber
		t.Logf("KZG Point Evaluation block number: %d", precompileBlock)

		disputeGameFactory := disputegame.NewFactoryHelper(t, ctx, sys)
		game := disputeGameFactory.StartOutputCannonGame(ctx, "sequencer", precompileBlock.Uint64(), common.Hash{0x01, 0xaa})
		require.NotNil(t, game)
		outputRootClaim := game.DisputeLastBlock(ctx)
		game.LogGameData(ctx)

		game.StartChallenger(ctx, "Challenger", challenger.WithPrivKey(sys.Cfg.Secrets.Alice))

		// Wait for the honest challenger to dispute the outputRootClaim. This creates a root of an execution game that we challenge by coercing
		// a step at a preimage trace index.
		outputRootClaim = outputRootClaim.WaitForCounterClaim(ctx)

		// Now the honest challenger is positioned as the defender of the execution game
		// We then move to challenge it to induce a preimage load
		preimageLoadCheck := game.CreateStepPreimageLoadCheck(ctx)
		game.ChallengeToPreimageLoad(ctx, outputRootClaim, sys.Cfg.Secrets.Alice, utils.FirstPrecompilePreimageLoad(), preimageLoadCheck, preloadPreimage)
		// The above method already verified the image was uploaded and step called successfully
		// So we don't waste time resolving the game - that's tested elsewhere.
	}

	t.Run("non-existing preimage", func(t *testing.T) {
		testPreimageStep(t, false)
	})
	t.Run("preimage already exists", func(t *testing.T) {
		testPreimageStep(t, true)
	})
}

func TestOutputCannonProposedOutputRootValid_Standard(t *testing.T) {
	testOutputCannonProposedOutputRootValid(t, config.AllocTypeStandard)
}

func TestOutputCannonProposedOutputRootValid_Multithreaded(t *testing.T) {
	testOutputCannonProposedOutputRootValid(t, config.AllocTypeMTCannon)
}

func testOutputCannonProposedOutputRootValid(t *testing.T, allocType config.AllocType) {
	op_e2e.InitParallel(t, op_e2e.UsesCannon)
	// honestStepsFail attempts to perform both an attack and defend step using the correct trace.
	honestStepsFail := func(ctx context.Context, game *disputegame.OutputCannonGameHelper, correctTrace *disputegame.OutputHonestHelper, parentClaimIdx int64) {
		// Attack step should fail
		correctTrace.StepFails(ctx, parentClaimIdx, true)
		// Defending should fail too
		correctTrace.StepFails(ctx, parentClaimIdx, false)
	}
	tests := []struct {
		// name is the name of the test
		name string

		// performMove is called to respond to each claim posted by the honest op-challenger.
		// It should either attack or defend the claim at parentClaimIdx
		performMove func(ctx context.Context, game *disputegame.OutputCannonGameHelper, correctTrace *disputegame.OutputHonestHelper, claim *disputegame.ClaimHelper) *disputegame.ClaimHelper

		// performStep is called once the maximum game depth is reached. It should perform a step to counter the
		// claim at parentClaimIdx. Since the proposed output root is invalid, the step call should always revert.
		performStep func(ctx context.Context, game *disputegame.OutputCannonGameHelper, correctTrace *disputegame.OutputHonestHelper, parentClaimIdx int64)
	}{
		{
			name: "AttackWithCorrectTrace",
			performMove: func(ctx context.Context, game *disputegame.OutputCannonGameHelper, correctTrace *disputegame.OutputHonestHelper, claim *disputegame.ClaimHelper) *disputegame.ClaimHelper {
				// Attack everything but oddly using the correct hash.
				// Except the root of the cannon game must have an invalid VM status code.
				if claim.IsOutputRootLeaf(ctx) {
					return claim.Attack(ctx, common.Hash{0x01})
				}
				return correctTrace.AttackClaim(ctx, claim)
			},
			performStep: honestStepsFail,
		},
		{
			name: "DefendWithCorrectTrace",
			performMove: func(ctx context.Context, game *disputegame.OutputCannonGameHelper, correctTrace *disputegame.OutputHonestHelper, claim *disputegame.ClaimHelper) *disputegame.ClaimHelper {
				// Can only attack the root claim or the first cannon claim
				if claim.IsRootClaim() {
					return correctTrace.AttackClaim(ctx, claim)
				}
				// The root of the cannon game must have an invalid VM status code
				// Attacking ensure we're running the cannon trace between two different blocks
				// instead of being in the trace extension of the output root bisection
				if claim.IsOutputRootLeaf(ctx) {
					return claim.Attack(ctx, common.Hash{0x01})
				}
				// Otherwise, defend everything using the correct hash.
				return correctTrace.DefendClaim(ctx, claim)
			},
			performStep: honestStepsFail,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			op_e2e.InitParallel(t, op_e2e.UsesCannon)

			ctx := context.Background()
			sys, l1Client := StartFaultDisputeSystem(t, WithAllocType(allocType))
			t.Cleanup(sys.Close)

			disputeGameFactory := disputegame.NewFactoryHelper(t, ctx, sys)
			game := disputeGameFactory.StartOutputCannonGameWithCorrectRoot(ctx, "sequencer", 1)
			correctTrace := game.CreateHonestActor(ctx, "sequencer", disputegame.WithPrivKey(sys.Cfg.Secrets.Mallory))

			game.StartChallenger(ctx, "Challenger", challenger.WithPrivKey(sys.Cfg.Secrets.Alice))

			// Now maliciously play the game and it should be impossible to win
			game.ChallengeClaim(ctx,
				game.RootClaim(ctx),
				func(claim *disputegame.ClaimHelper) *disputegame.ClaimHelper {
					return test.performMove(ctx, game, correctTrace, claim)
				},
				func(parentClaimIdx int64) {
					test.performStep(ctx, game, correctTrace, parentClaimIdx)
				})

			// Time travel past when the game will be resolvable.
			sys.TimeTravelClock.AdvanceTime(game.MaxClockDuration(ctx))
			require.NoError(t, wait.ForNextBlock(ctx, l1Client))

			game.WaitForGameStatus(ctx, gameTypes.GameStatusDefenderWon)
		})
	}
}

func TestOutputCannonPoisonedPostState_Standard(t *testing.T) {
	testOutputCannonPoisonedPostState(t, config.AllocTypeStandard)
}

func TestOutputCannonPoisonedPostState_Multithreaded(t *testing.T) {
	testOutputCannonPoisonedPostState(t, config.AllocTypeMTCannon)
}

func testOutputCannonPoisonedPostState(t *testing.T, allocType config.AllocType) {
	op_e2e.InitParallel(t, op_e2e.UsesCannon)

	ctx := context.Background()
	sys, l1Client := StartFaultDisputeSystem(t, WithAllocType(allocType))
	t.Cleanup(sys.Close)

	disputeGameFactory := disputegame.NewFactoryHelper(t, ctx, sys)
	// Root claim is dishonest
	game := disputeGameFactory.StartOutputCannonGame(ctx, "sequencer", 1, common.Hash{0xaa})
	correctTrace := game.CreateHonestActor(ctx, "sequencer", disputegame.WithPrivKey(sys.Cfg.Secrets.Alice))

	// Honest first attack at "honest" level
	claim := correctTrace.AttackClaim(ctx, game.RootClaim(ctx))

	// Honest defense at "dishonest" level
	claim = correctTrace.DefendClaim(ctx, claim)

	// Dishonest attack at "honest" level - honest move would be to ignore
	claimToIgnore1 := claim.Attack(ctx, common.Hash{0x03, 0xaa})

	// Honest attack at "dishonest" level - honest move would be to ignore
	claimToIgnore2 := correctTrace.AttackClaim(ctx, claimToIgnore1)
	game.LogGameData(ctx)

	// Start the honest challenger
	game.StartChallenger(ctx, "Honest", challenger.WithPrivKey(sys.Cfg.Secrets.Bob))

	// Start dishonest challenger that posts correct claims
	for {
		game.LogGameData(ctx)
		// Wait for the challenger to counter
		// Note that we need to ignore claimToIgnore1 which already counters this...
		claim = claim.WaitForCounterClaim(ctx, claimToIgnore1)

		// Respond with our own move
		if claim.IsBottomGameRoot(ctx) {
			// Root of the cannon game must have the right VM status code (so it can't be honest).
			// Note this occurs when there are splitDepth + 4 claims because there are multiple forks in this game.
			claim = claim.Attack(ctx, common.Hash{0x01})
		} else {
			claim = correctTrace.DefendClaim(ctx, claim)
		}

		// Defender moves last. If we're at max depth, then we're done
		if claim.IsMaxDepth(ctx) {
			break
		}
	}

	// Wait for the challenger to call step
	claim.WaitForCountered(ctx)
	// Verify that the challenger didn't challenge our poisoned claims
	claimToIgnore1.RequireOnlyCounteredBy(ctx, claimToIgnore2)
	claimToIgnore2.RequireOnlyCounteredBy(ctx /* nothing */)

	// Time travel past when the game will be resolvable.
	sys.TimeTravelClock.AdvanceTime(game.MaxClockDuration(ctx))
	require.NoError(t, wait.ForNextBlock(ctx, l1Client))

	game.LogGameData(ctx)
	game.WaitForGameStatus(ctx, gameTypes.GameStatusChallengerWon)
}

func TestDisputeOutputRootBeyondProposedBlock_ValidOutputRoot_Standard(t *testing.T) {
	testDisputeOutputRootBeyondProposedBlockValidOutputRoot(t, config.AllocTypeStandard)
}

func TestDisputeOutputRootBeyondProposedBlock_ValidOutputRoot_Multithreaded(t *testing.T) {
	testDisputeOutputRootBeyondProposedBlockValidOutputRoot(t, config.AllocTypeMTCannon)
}

func testDisputeOutputRootBeyondProposedBlockValidOutputRoot(t *testing.T, allocType config.AllocType) {
	op_e2e.InitParallel(t, op_e2e.UsesCannon)

	ctx := context.Background()
	sys, l1Client := StartFaultDisputeSystem(t, WithAllocType(allocType))
	t.Cleanup(sys.Close)

	disputeGameFactory := disputegame.NewFactoryHelper(t, ctx, sys)
	// Root claim is dishonest
	game := disputeGameFactory.StartOutputCannonGameWithCorrectRoot(ctx, "sequencer", 1)
	correctTrace := game.CreateHonestActor(ctx, "sequencer", disputegame.WithPrivKey(sys.Cfg.Secrets.Alice))
	// Start the honest challenger
	game.StartChallenger(ctx, "Honest", challenger.WithPrivKey(sys.Cfg.Secrets.Bob))

	claim := game.RootClaim(ctx)
	// Attack the output root
	claim = correctTrace.AttackClaim(ctx, claim)
	// Wait for the challenger to respond
	claim = claim.WaitForCounterClaim(ctx)
	// Then defend until the split depth to force the game into the extension part of the output root bisection
	// ie. the output root we wind up disputing is theoretically for a block after block number 1
	for !claim.IsOutputRootLeaf(ctx) {
		claim = correctTrace.DefendClaim(ctx, claim)
		claim = claim.WaitForCounterClaim(ctx)
	}
	game.LogGameData(ctx)
	// At this point we've reached the bottom of the output root bisection and every claim
	// will have the same, valid, output root. We now need to post a cannon trace root that claims its invalid.
	claim = claim.Defend(ctx, common.Hash{0x01, 0xaa})
	// Now defend with the correct trace
	for {
		game.LogGameData(ctx)
		claim = claim.WaitForCounterClaim(ctx)
		if claim.IsMaxDepth(ctx) {
			break
		}
		claim = correctTrace.DefendClaim(ctx, claim)
	}
	// Should not be able to step either attacking or defending
	correctTrace.StepClaimFails(ctx, claim, true)
	correctTrace.StepClaimFails(ctx, claim, false)

	// Time travel past when the game will be resolvable.
	sys.TimeTravelClock.AdvanceTime(game.MaxClockDuration(ctx))
	require.NoError(t, wait.ForNextBlock(ctx, l1Client))

	game.WaitForGameStatus(ctx, gameTypes.GameStatusDefenderWon)
	game.LogGameData(ctx)
}

func TestDisputeOutputRootBeyondProposedBlock_InvalidOutputRoot_Standard(t *testing.T) {
	testDisputeOutputRootBeyondProposedBlockInvalidOutputRoot(t, config.AllocTypeStandard)
}

func TestDisputeOutputRootBeyondProposedBlock_InvalidOutputRoot_Multithreaded(t *testing.T) {
	testDisputeOutputRootBeyondProposedBlockInvalidOutputRoot(t, config.AllocTypeMTCannon)
}

func testDisputeOutputRootBeyondProposedBlockInvalidOutputRoot(t *testing.T, allocType config.AllocType) {
	op_e2e.InitParallel(t, op_e2e.UsesCannon)

	ctx := context.Background()
	sys, l1Client := StartFaultDisputeSystem(t, WithAllocType(allocType))
	t.Cleanup(sys.Close)

	disputeGameFactory := disputegame.NewFactoryHelper(t, ctx, sys)
	// Root claim is dishonest
	game := disputeGameFactory.StartOutputCannonGame(ctx, "sequencer", 1, common.Hash{0xaa})
	correctTrace := game.CreateHonestActor(ctx, "sequencer", disputegame.WithPrivKey(sys.Cfg.Secrets.Alice))

	// Start the honest challenger
	game.StartChallenger(ctx, "Honest", challenger.WithPrivKey(sys.Cfg.Secrets.Bob))

	claim := game.RootClaim(ctx)
	// Wait for the honest challenger to counter the root
	claim = claim.WaitForCounterClaim(ctx)
	// Then defend until the split depth to force the game into the extension part of the output root bisection
	// ie. the output root we wind up disputing is theoretically for a block after block number 1
	// The dishonest actor challenges with the correct roots
	for claim.IsOutputRoot(ctx) {
		claim = correctTrace.DefendClaim(ctx, claim)
		claim = claim.WaitForCounterClaim(ctx)
	}
	game.LogGameData(ctx)
	// Now defend with the correct trace
	for !claim.IsMaxDepth(ctx) {
		game.LogGameData(ctx)
		if claim.IsBottomGameRoot(ctx) {
			claim = correctTrace.AttackClaim(ctx, claim)
		} else {
			claim = correctTrace.DefendClaim(ctx, claim)
		}
		if !claim.IsMaxDepth(ctx) {
			// Have to attack the root of the cannon trace
			claim = claim.WaitForCounterClaim(ctx)
		}
	}

	// Wait for our final claim to be countered by the challenger calling step
	claim.WaitForCountered(ctx)

	// Time travel past when the game will be resolvable.
	sys.TimeTravelClock.AdvanceTime(game.MaxClockDuration(ctx))
	require.NoError(t, wait.ForNextBlock(ctx, l1Client))

	game.WaitForGameStatus(ctx, gameTypes.GameStatusChallengerWon)
	game.LogGameData(ctx)
}

func TestTestDisputeOutputRoot_ChangeClaimedOutputRoot_Standard(t *testing.T) {
	testTestDisputeOutputRootChangeClaimedOutputRoot(t, config.AllocTypeStandard)
}

func TestTestDisputeOutputRoot_ChangeClaimedOutputRoot_Multithreaded(t *testing.T) {
	testTestDisputeOutputRootChangeClaimedOutputRoot(t, config.AllocTypeMTCannon)
}

func testTestDisputeOutputRootChangeClaimedOutputRoot(t *testing.T, allocType config.AllocType) {
	op_e2e.InitParallel(t, op_e2e.UsesCannon)

	ctx := context.Background()
	sys, l1Client := StartFaultDisputeSystem(t, WithAllocType(allocType))
	t.Cleanup(sys.Close)

	disputeGameFactory := disputegame.NewFactoryHelper(t, ctx, sys)
	// Root claim is dishonest
	game := disputeGameFactory.StartOutputCannonGame(ctx, "sequencer", 1, common.Hash{0xaa})
	correctTrace := game.CreateHonestActor(ctx, "sequencer", disputegame.WithPrivKey(sys.Cfg.Secrets.Alice))

	// Start the honest challenger
	game.StartChallenger(ctx, "Honest", challenger.WithPrivKey(sys.Cfg.Secrets.Bob))

	claim := game.RootClaim(ctx)
	// Wait for the honest challenger to counter the root
	claim = claim.WaitForCounterClaim(ctx)

	// Then attack every claim until the leaf of output root bisection
	for {
		claim = claim.Attack(ctx, common.Hash{0xbb})
		claim = claim.WaitForCounterClaim(ctx)
		if claim.Depth() == game.SplitDepth(ctx)-1 {
			// Post the correct output root as the leaf.
			// This is for block 1 which is what the original output root was for too
			claim = correctTrace.AttackClaim(ctx, claim)
			// Challenger should post the first cannon trace
			claim = claim.WaitForCounterClaim(ctx)
			break
		}
	}

	game.LogGameData(ctx)

	// Now defend with the correct trace
	for !claim.IsMaxDepth(ctx) {
		game.LogGameData(ctx)
		if claim.IsBottomGameRoot(ctx) {
			claim = correctTrace.AttackClaim(ctx, claim)
		} else {
			claim = correctTrace.DefendClaim(ctx, claim)
		}
		if !claim.IsMaxDepth(ctx) {
			// Have to attack the root of the cannon trace
			claim = claim.WaitForCounterClaim(ctx)
		}
	}

	// Wait for our final claim to be countered by the challenger calling step
	claim.WaitForCountered(ctx)

	// Time travel past when the game will be resolvable.
	sys.TimeTravelClock.AdvanceTime(game.MaxClockDuration(ctx))
	require.NoError(t, wait.ForNextBlock(ctx, l1Client))

	game.WaitForGameStatus(ctx, gameTypes.GameStatusChallengerWon)
	game.LogGameData(ctx)
}

func TestInvalidateUnsafeProposal_Standard(t *testing.T) {
	testInvalidateUnsafeProposal(t, config.AllocTypeStandard)
}

func TestInvalidateUnsafeProposal_Multithreaded(t *testing.T) {
	testInvalidateUnsafeProposal(t, config.AllocTypeMTCannon)
}

func testInvalidateUnsafeProposal(t *testing.T, allocType config.AllocType) {
	op_e2e.InitParallel(t, op_e2e.UsesCannon)
	ctx := context.Background()

	tests := []struct {
		name     string
		strategy func(correctTrace *disputegame.OutputHonestHelper, parent *disputegame.ClaimHelper) *disputegame.ClaimHelper
	}{
		{
			name: "Attack",
			strategy: func(correctTrace *disputegame.OutputHonestHelper, parent *disputegame.ClaimHelper) *disputegame.ClaimHelper {
				return correctTrace.AttackClaim(ctx, parent)
			},
		},
		{
			name: "Defend",
			strategy: func(correctTrace *disputegame.OutputHonestHelper, parent *disputegame.ClaimHelper) *disputegame.ClaimHelper {
				return correctTrace.DefendClaim(ctx, parent)
			},
		},
		{
			name: "Counter",
			strategy: func(correctTrace *disputegame.OutputHonestHelper, parent *disputegame.ClaimHelper) *disputegame.ClaimHelper {
				return correctTrace.CounterClaim(ctx, parent)
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			op_e2e.InitParallel(t, op_e2e.UsesCannon)
			sys, l1Client := StartFaultDisputeSystem(t, WithSequencerWindowSize(100000), WithBatcherStopped(), WithAllocType(allocType))
			t.Cleanup(sys.Close)

			blockNum := uint64(1)
			disputeGameFactory := disputegame.NewFactoryHelper(t, ctx, sys)
			// Root claim is _dishonest_ because the required data is not available on L1
			game := disputeGameFactory.StartOutputCannonGameWithCorrectRoot(ctx, "sequencer", blockNum, disputegame.WithUnsafeProposal())

			correctTrace := game.CreateHonestActor(ctx, "sequencer", disputegame.WithPrivKey(sys.Cfg.Secrets.Alice))

			// Start the honest challenger
			game.StartChallenger(ctx, "Challenger", challenger.WithPrivKey(sys.Cfg.Secrets.Bob))

			game.DefendClaim(ctx, game.RootClaim(ctx), func(parent *disputegame.ClaimHelper) *disputegame.ClaimHelper {
				if parent.IsBottomGameRoot(ctx) {
					return correctTrace.AttackClaim(ctx, parent)
				}
				return test.strategy(correctTrace, parent)
			})

			// Time travel past when the game will be resolvable.
			sys.TimeTravelClock.AdvanceTime(game.MaxClockDuration(ctx))
			require.NoError(t, wait.ForNextBlock(ctx, l1Client))

			game.WaitForGameStatus(ctx, gameTypes.GameStatusChallengerWon)
			game.LogGameData(ctx)
		})
	}
}

func TestInvalidateProposalForFutureBlock_Standard(t *testing.T) {
	testInvalidateProposalForFutureBlock(t, config.AllocTypeStandard)
}

func TestInvalidateProposalForFutureBlock_Multithreaded(t *testing.T) {
	testInvalidateProposalForFutureBlock(t, config.AllocTypeMTCannon)
}

func testInvalidateProposalForFutureBlock(t *testing.T, allocType config.AllocType) {
	op_e2e.InitParallel(t, op_e2e.UsesCannon)
	ctx := context.Background()

	tests := []struct {
		name     string
		strategy func(correctTrace *disputegame.OutputHonestHelper, parent *disputegame.ClaimHelper) *disputegame.ClaimHelper
	}{
		{
			name: "Attack",
			strategy: func(correctTrace *disputegame.OutputHonestHelper, parent *disputegame.ClaimHelper) *disputegame.ClaimHelper {
				return correctTrace.AttackClaim(ctx, parent)
			},
		},
		{
			name: "Defend",
			strategy: func(correctTrace *disputegame.OutputHonestHelper, parent *disputegame.ClaimHelper) *disputegame.ClaimHelper {
				return correctTrace.DefendClaim(ctx, parent)
			},
		},
		{
			name: "Counter",
			strategy: func(correctTrace *disputegame.OutputHonestHelper, parent *disputegame.ClaimHelper) *disputegame.ClaimHelper {
				return correctTrace.CounterClaim(ctx, parent)
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			op_e2e.InitParallel(t, op_e2e.UsesCannon)
			sys, l1Client := StartFaultDisputeSystem(t, WithSequencerWindowSize(100000), WithAllocType(allocType))
			t.Cleanup(sys.Close)

			farFutureBlockNum := uint64(10_000_000)
			disputeGameFactory := disputegame.NewFactoryHelper(t, ctx, sys)
			// Root claim is _dishonest_ because the required data is not available on L1
			game := disputeGameFactory.StartOutputCannonGame(ctx, "sequencer", farFutureBlockNum, common.Hash{0xaa}, disputegame.WithFutureProposal())

			correctTrace := game.CreateHonestActor(ctx, "sequencer", disputegame.WithPrivKey(sys.Cfg.Secrets.Alice))

			// Start the honest challenger
			game.StartChallenger(ctx, "Honest", challenger.WithPrivKey(sys.Cfg.Secrets.Bob))

			game.DefendClaim(ctx, game.RootClaim(ctx), func(parent *disputegame.ClaimHelper) *disputegame.ClaimHelper {
				if parent.IsBottomGameRoot(ctx) {
					return correctTrace.AttackClaim(ctx, parent)
				}
				return test.strategy(correctTrace, parent)
			})

			// Time travel past when the game will be resolvable.
			sys.TimeTravelClock.AdvanceTime(game.MaxClockDuration(ctx))
			require.NoError(t, wait.ForNextBlock(ctx, l1Client))

			game.WaitForGameStatus(ctx, gameTypes.GameStatusChallengerWon)
			game.LogGameData(ctx)
		})
	}
}

func TestInvalidateCorrectProposalFutureBlock_Standard(t *testing.T) {
	testInvalidateCorrectProposalFutureBlock(t, config.AllocTypeStandard)
}

func TestInvalidateCorrectProposalFutureBlock_Multithreaded(t *testing.T) {
	testInvalidateCorrectProposalFutureBlock(t, config.AllocTypeMTCannon)
}

func testInvalidateCorrectProposalFutureBlock(t *testing.T, allocType config.AllocType) {
	op_e2e.InitParallel(t, op_e2e.UsesCannon)
	ctx := context.Background()
	// Spin up the system without the batcher so the safe head doesn't advance
	sys, l1Client := StartFaultDisputeSystem(t, WithBatcherStopped(), WithSequencerWindowSize(100000), WithAllocType(allocType))
	t.Cleanup(sys.Close)

	// Create a dispute game factory helper.
	disputeGameFactory := disputegame.NewFactoryHelper(t, ctx, sys)

	// No batches submitted so safe head is genesis
	output, err := sys.RollupClient("sequencer").OutputAtBlock(ctx, 0)
	require.NoError(t, err, "Failed to get output at safe head")
	// Create a dispute game with an output root that is valid at `safeHead`, but that claims to correspond to block
	// `safeHead.Number + 10000`. This is dishonest, because this block does not exist yet.
	game := disputeGameFactory.StartOutputCannonGame(ctx, "sequencer", 10_000, common.Hash(output.OutputRoot), disputegame.WithFutureProposal())

	// Start the honest challenger.
	game.StartChallenger(ctx, "Honest", challenger.WithPrivKey(sys.Cfg.Secrets.Bob))

	game.WaitForL2BlockNumberChallenged(ctx)

	// Time travel past when the game will be resolvable.
	sys.TimeTravelClock.AdvanceTime(game.MaxClockDuration(ctx))
	require.NoError(t, wait.ForNextBlock(ctx, l1Client))

	// The game should resolve as `CHALLENGER_WINS` always, because the root claim signifies a claim that does not exist
	// yet in the L2 chain.
	game.WaitForGameStatus(ctx, gameTypes.GameStatusChallengerWon)
	game.LogGameData(ctx)
}

func TestOutputCannonHonestSafeTraceExtension_ValidRoot_Standard(t *testing.T) {
	testOutputCannonHonestSafeTraceExtensionValidRoot(t, config.AllocTypeStandard)
}

func TestOutputCannonHonestSafeTraceExtension_ValidRoot_Multithreaded(t *testing.T) {
	testOutputCannonHonestSafeTraceExtensionValidRoot(t, config.AllocTypeMTCannon)
}

func testOutputCannonHonestSafeTraceExtensionValidRoot(t *testing.T, allocType config.AllocType) {
	op_e2e.InitParallel(t, op_e2e.UsesCannon)

	ctx := context.Background()
	sys, l1Client := StartFaultDisputeSystem(t, WithAllocType(allocType))
	t.Cleanup(sys.Close)

	// Wait for there to be there are safe L2 blocks past the claimed safe head that have data available on L1 within
	// the commitment stored in the dispute game.
	safeHeadNum := uint64(3)
	require.NoError(t, wait.ForSafeBlock(ctx, sys.RollupClient("sequencer"), safeHeadNum))

	// Create a dispute game with an honest claim
	disputeGameFactory := disputegame.NewFactoryHelper(t, ctx, sys)
	game := disputeGameFactory.StartOutputCannonGameWithCorrectRoot(ctx, "sequencer", safeHeadNum-1)
	require.NotNil(t, game)

	// Create a correct trace actor with an honest trace extending to L2 block #4
	correctTrace := game.CreateHonestActor(ctx, "sequencer", disputegame.WithPrivKey(sys.Cfg.Secrets.Mallory))

	// Create a correct trace actor with an honest trace extending to L2 block #5
	// Notably, L2 block #5 is a valid block within the safe chain, and the data required to reproduce it
	// will be committed to within the L1 head of the dispute game.
	correctTracePlus1 := game.CreateHonestActor(ctx, "sequencer",
		disputegame.WithPrivKey(sys.Cfg.Secrets.Mallory),
		disputegame.WithClaimedL2BlockNumber(safeHeadNum))

	// Start the honest challenger. They will defend the root claim.
	game.StartChallenger(ctx, "Challenger", challenger.WithPrivKey(sys.Cfg.Secrets.Alice))

	claim := game.RootClaim(ctx)
	game.ChallengeClaim(ctx, claim, func(parent *disputegame.ClaimHelper) *disputegame.ClaimHelper {
		// Have to disagree with the root claim - we're trying to invalidate a valid output root
		if parent.IsRootClaim() {
			return parent.Attack(ctx, common.Hash{0xdd})
		}
		return correctTracePlus1.CounterClaim(ctx, parent)
	}, func(parentClaimIdx int64) {
		correctTrace.StepFails(ctx, parentClaimIdx, true)
		correctTrace.StepFails(ctx, parentClaimIdx, false)
		correctTracePlus1.StepFails(ctx, parentClaimIdx, true)
		correctTracePlus1.StepFails(ctx, parentClaimIdx, false)
	})
	game.LogGameData(ctx)

	// Time travel past when the game will be resolvable.
	sys.TimeTravelClock.AdvanceTime(game.MaxClockDuration(ctx))
	require.NoError(t, wait.ForNextBlock(ctx, l1Client))

	game.WaitForGameStatus(ctx, gameTypes.GameStatusDefenderWon)
}

func TestOutputCannonHonestSafeTraceExtension_InvalidRoot_Standard(t *testing.T) {
	testOutputCannonHonestSafeTraceExtensionInvalidRoot(t, config.AllocTypeStandard)
}

func TestOutputCannonHonestSafeTraceExtension_InvalidRoot_Multithreaded(t *testing.T) {
	testOutputCannonHonestSafeTraceExtensionInvalidRoot(t, config.AllocTypeMTCannon)
}

func testOutputCannonHonestSafeTraceExtensionInvalidRoot(t *testing.T, allocType config.AllocType) {
	op_e2e.InitParallel(t, op_e2e.UsesCannon)

	ctx := context.Background()
	sys, l1Client := StartFaultDisputeSystem(t, WithAllocType(allocType))
	t.Cleanup(sys.Close)

	// Wait for there to be there are safe L2 blocks past the claimed safe head that have data available on L1 within
	// the commitment stored in the dispute game.
	safeHeadNum := uint64(2)
	require.NoError(t, wait.ForSafeBlock(ctx, sys.RollupClient("sequencer"), safeHeadNum))

	// Create a dispute game with a dishonest claim @ L2 block #4
	disputeGameFactory := disputegame.NewFactoryHelper(t, ctx, sys)
	game := disputeGameFactory.StartOutputCannonGame(ctx, "sequencer", safeHeadNum-1, common.Hash{0xCA, 0xFE})
	require.NotNil(t, game)

	// Create a correct trace actor with an honest trace extending to L2 block #5
	// Notably, L2 block #5 is a valid block within the safe chain, and the data required to reproduce it
	// will be committed to within the L1 head of the dispute game.
	correctTracePlus1 := game.CreateHonestActor(ctx, "sequencer",
		disputegame.WithPrivKey(sys.Cfg.Secrets.Mallory),
		disputegame.WithClaimedL2BlockNumber(safeHeadNum))

	// Start the honest challenger. They will challenge the root claim.
	game.StartChallenger(ctx, "Challenger", challenger.WithPrivKey(sys.Cfg.Secrets.Alice))

	claim := game.RootClaim(ctx)
	game.DefendClaim(ctx, claim, func(parent *disputegame.ClaimHelper) *disputegame.ClaimHelper {
		return correctTracePlus1.CounterClaim(ctx, parent)
	})

	// Time travel past when the game will be resolvable.
	sys.TimeTravelClock.AdvanceTime(game.MaxClockDuration(ctx))
	require.NoError(t, wait.ForNextBlock(ctx, l1Client))

	game.WaitForGameStatus(ctx, gameTypes.GameStatusChallengerWon)
}

func TestAgreeFirstBlockWithOriginOf1_Standard(t *testing.T) {
	testAgreeFirstBlockWithOriginOf1(t, config.AllocTypeStandard)
}

func TestAgreeFirstBlockWithOriginOf1_Multithreaded(t *testing.T) {
	testAgreeFirstBlockWithOriginOf1(t, config.AllocTypeMTCannon)
}

func testAgreeFirstBlockWithOriginOf1(t *testing.T, allocType config.AllocType) {
	op_e2e.InitParallel(t, op_e2e.UsesCannon)

	ctx := context.Background()
	sys, _ := StartFaultDisputeSystem(t, WithAllocType(allocType))
	t.Cleanup(sys.Close)

	rollupClient := sys.RollupClient("sequencer")
	blockNum := uint64(0)
	limit := uint64(100)
	for ; blockNum <= limit; blockNum++ {
		require.NoError(t, wait.ForBlock(ctx, sys.NodeClient("sequencer"), blockNum))
		output, err := rollupClient.OutputAtBlock(ctx, blockNum)
		require.NoError(t, err)
		if output.BlockRef.L1Origin.Number == 1 {
			break
		}
	}
	require.Less(t, blockNum, limit)

	// Create a dispute game with a dishonest claim @ L2 block #4
	disputeGameFactory := disputegame.NewFactoryHelper(t, ctx, sys)
	// Make the agreed block the first one with L1 origin of block 1 so the claim is blockNum+1
	game := disputeGameFactory.StartOutputCannonGame(ctx, "sequencer", blockNum+1, common.Hash{0xCA, 0xFE})
	require.NotNil(t, game)
	outputRootClaim := game.DisputeLastBlock(ctx)
	game.LogGameData(ctx)

	honestChallenger := game.StartChallenger(ctx, "HonestActor", challenger.WithPrivKey(sys.Cfg.Secrets.Alice))

	// Wait for the honest challenger to dispute the outputRootClaim. This creates a root of an execution game that we challenge by coercing
	// a step at a preimage trace index.
	outputRootClaim = outputRootClaim.WaitForCounterClaim(ctx)
	game.LogGameData(ctx)

	// Should claim output root is invalid, but actually panics.
	outputRootClaim.RequireInvalidStatusCode()
	// The above method already verified the image was uploaded and step called successfully
	// So we don't waste time resolving the game - that's tested elsewhere.
	require.NoError(t, honestChallenger.Close())
}
