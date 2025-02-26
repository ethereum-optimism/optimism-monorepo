package solver

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

	faulttest "github.com/ethereum-optimism/optimism/op-challenger/game/fault/test"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestCalculateNextActions_ChallengeL2BlockNumber(t *testing.T) {
	startingBlock := big.NewInt(5)
	maxDepth := types.Depth(6)
	challenge := &types.InvalidL2BlockNumberChallenge{
		Output: &eth.OutputResponse{OutputRoot: eth.Bytes32{0xbb}},
	}
	claimBuilder := faulttest.NewAlphabetClaimBuilder(t, startingBlock, maxDepth)
	traceProvider := faulttest.NewAlphabetWithProofProvider(t, startingBlock, maxDepth, nil)
	solver := NewGameSolver(maxDepth, trace.NewSimpleTraceAccessor(traceProvider))

	// Do not challenge when provider returns error indicating l2 block is valid
	actions, err := solver.CalculateNextActions(context.Background(), claimBuilder.GameBuilder().Game)
	require.NoError(t, err)
	require.Len(t, actions, 0)

	// Do challenge when the provider returns a challenge
	traceProvider.L2BlockChallenge = challenge
	actions, err = solver.CalculateNextActions(context.Background(), claimBuilder.GameBuilder().Game)
	require.NoError(t, err)
	require.Len(t, actions, 1)
	action := actions[0]
	require.Equal(t, types.ActionTypeChallengeL2BlockNumber, action.Type)
	require.Equal(t, challenge, action.InvalidL2BlockNumberChallenge)
}

func TestCalculateNextActions(t *testing.T) {
	maxDepth := types.Depth(6)
	startingL2BlockNumber := big.NewInt(0)
	claimBuilder := faulttest.NewAlphabetClaimBuilder(t, startingL2BlockNumber, maxDepth)

	tests := []struct {
		name             string
		rootClaimCorrect bool
		setupGame        func(builder *faulttest.GameBuilder)
	}{
		{
			name: "AttackRootClaim",
			setupGame: func(builder *faulttest.GameBuilder) {
				builder.Seq().ExpectAttack()
			},
		},
		{
			name:             "DoNotAttackCorrectRootClaim_AgreeWithOutputRoot",
			rootClaimCorrect: true,
			setupGame:        func(builder *faulttest.GameBuilder) {},
		},
		{
			name: "DoNotPerformDuplicateMoves",
			setupGame: func(builder *faulttest.GameBuilder) {
				// Expected move has already been made.
				builder.Seq().Attack()
			},
		},
		{
			name: "RespondToAllClaimsAtDisagreeingLevel",
			setupGame: func(builder *faulttest.GameBuilder) {
				honestClaim := builder.Seq().Attack()
				honestClaim.Attack().ExpectDefend()
				honestClaim.Defend().ExpectDefend()
				honestClaim.Attack(faulttest.WithValue(common.Hash{0xaa})).ExpectAttack()
				honestClaim.Attack(faulttest.WithValue(common.Hash{0xbb})).ExpectAttack()
				honestClaim.Defend(faulttest.WithValue(common.Hash{0xcc})).ExpectAttack()
				honestClaim.Defend(faulttest.WithValue(common.Hash{0xdd})).ExpectAttack()
			},
		},
		{
			name: "StepAtMaxDepth",
			setupGame: func(builder *faulttest.GameBuilder) {
				lastHonestClaim := builder.Seq().
					Attack().
					Attack().
					Defend().
					Defend().
					Defend()
				lastHonestClaim.Attack().ExpectStepDefend()
				lastHonestClaim.Attack(faulttest.WithValue(common.Hash{0xdd})).ExpectStepAttack()
			},
		},
		{
			name: "PoisonedPreState",
			setupGame: func(builder *faulttest.GameBuilder) {
				// A claim hash that has no pre-image
				maliciousStateHash := common.Hash{0x01, 0xaa}

				// Dishonest actor counters their own claims to set up a situation with an invalid prestate
				// The honest actor should ignore path created by the dishonest actor, only supporting its own attack on the root claim
				honestMove := builder.Seq().Attack() // This expected action is the winning move.
				dishonestMove := honestMove.Attack(faulttest.WithValue(maliciousStateHash))
				// The expected action by the honest actor
				dishonestMove.ExpectAttack()
				// The honest actor will ignore this poisoned path
				dishonestMove.
					Defend(faulttest.WithValue(maliciousStateHash)).
					Attack(faulttest.WithValue(maliciousStateHash))
			},
		},
		{
			name: "Freeloader-ValidClaimAtInvalidAttackPosition",
			setupGame: func(builder *faulttest.GameBuilder) {
				builder.Seq().
					Attack().                // Honest response to invalid root
					Defend().ExpectDefend(). // Defender agrees at this point, we should defend
					Attack().ExpectDefend()  // Freeloader attacks instead of defends
			},
		},
		{
			name: "Freeloader-InvalidClaimAtInvalidAttackPosition",
			setupGame: func(builder *faulttest.GameBuilder) {
				builder.Seq().
					Attack().                                                     // Honest response to invalid root
					Defend().ExpectDefend().                                      // Defender agrees at this point, we should defend
					Attack(faulttest.WithValue(common.Hash{0xbb})).ExpectAttack() // Freeloader attacks with wrong claim instead of defends
			},
		},
		{
			name: "Freeloader-InvalidClaimAtValidDefensePosition",
			setupGame: func(builder *faulttest.GameBuilder) {
				builder.Seq().
					Attack().                                                     // Honest response to invalid root
					Defend().ExpectDefend().                                      // Defender agrees at this point, we should defend
					Defend(faulttest.WithValue(common.Hash{0xbb})).ExpectAttack() // Freeloader defends with wrong claim, we should attack
			},
		},
		{
			name: "Freeloader-InvalidClaimAtValidAttackPosition",
			setupGame: func(builder *faulttest.GameBuilder) {
				builder.Seq().
					Attack().                                                      // Honest response to invalid root
					Defend(faulttest.WithValue(common.Hash{0xaa})).ExpectAttack(). // Defender disagrees at this point, we should attack
					Attack(faulttest.WithValue(common.Hash{0xbb})).ExpectAttack()  // Freeloader attacks with wrong claim instead of defends
			},
		},
		{
			name: "Freeloader-InvalidClaimAtInvalidDefensePosition",
			setupGame: func(builder *faulttest.GameBuilder) {
				builder.Seq().
					Attack().                                                      // Honest response to invalid root
					Defend(faulttest.WithValue(common.Hash{0xaa})).ExpectAttack(). // Defender disagrees at this point, we should attack
					Defend(faulttest.WithValue(common.Hash{0xbb}))                 // Freeloader defends with wrong claim but we must not respond to avoid poisoning
			},
		},
		{
			name: "Freeloader-ValidClaimAtInvalidAttackPosition-RespondingToDishonestButCorrectAttack",
			setupGame: func(builder *faulttest.GameBuilder) {
				builder.Seq().
					Attack().                // Honest response to invalid root
					Attack().ExpectDefend(). // Defender attacks with correct value, we should defend
					Attack().ExpectDefend()  // Freeloader attacks with wrong claim, we should defend
			},
		},
		{
			name: "Freeloader-DoNotCounterOwnClaim",
			setupGame: func(builder *faulttest.GameBuilder) {
				builder.Seq().
					Attack().                // Honest response to invalid root
					Attack().ExpectDefend(). // Defender attacks with correct value, we should defend
					Attack().                // Freeloader attacks instead, we should defend
					Defend()                 // We do defend and we shouldn't counter our own claim
			},
		},
		{
			name: "Freeloader-ContinueDefendingAgainstFreeloader",
			setupGame: func(builder *faulttest.GameBuilder) {
				builder.Seq(). // invalid root
						Attack().                                       // Honest response to invalid root
						Attack().ExpectDefend().                        // Defender attacks with correct value, we should defend
						Attack().                                       // Freeloader attacks instead, we should defend
						Defend().                                       // We do defend
						Attack(faulttest.WithValue(common.Hash{0xaa})). // freeloader attacks our defense, we should attack
						ExpectAttack()
			},
		},
		{
			name: "Freeloader-FreeloaderCountersRootClaim",
			setupGame: func(builder *faulttest.GameBuilder) {
				builder.Seq().
					ExpectAttack().                                 // Honest response to invalid root
					Attack(faulttest.WithValue(common.Hash{0xaa})). // freeloader
					ExpectAttack()                                  // Honest response to freeloader
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			builder := claimBuilder.GameBuilder(faulttest.WithInvalidValue(!test.rootClaimCorrect))
			test.setupGame(builder)
			game := builder.Game

			solver := NewGameSolver(maxDepth, trace.NewSimpleTraceAccessor(claimBuilder.CorrectTraceProvider()))
			postState, actions := runStep(t, solver, game, claimBuilder.CorrectTraceProvider())
			for i, action := range builder.ExpectedActions {
				t.Logf("Expect %v: Type: %v, ParentIdx: %v, Attack: %v, Value: %v, PreState: %v, ProofData: %v",
					i, action.Type, action.ParentClaim.ContractIndex, action.IsAttack, action.Value, hex.EncodeToString(action.PreState), hex.EncodeToString(action.ProofData))
				require.Containsf(t, actions, action, "Expected claim %v missing", i)
			}
			require.Len(t, actions, len(builder.ExpectedActions), "Incorrect number of actions")

			verifyGameRules(t, postState, test.rootClaimCorrect)
		})
	}
}

func runStep(t *testing.T, solver *GameSolver, game types.Game, correctTraceProvider types.TraceProvider) (types.Game, []types.Action) {
	actions, err := solver.CalculateNextActions(context.Background(), game)
	require.NoError(t, err)

	postState := applyActions(game, challengerAddr, actions)

	for i, action := range actions {
		t.Logf("Move %v: Type: %v, ParentIdx: %v, Attack: %v, Value: %v, PreState: %v, ProofData: %v",
			i, action.Type, action.ParentClaim.ContractIndex, action.IsAttack, action.Value, hex.EncodeToString(action.PreState), hex.EncodeToString(action.ProofData))
		// Check that every move the solver returns meets the generic validation rules
		require.NoError(t, checkRules(game, action, correctTraceProvider), "Attempting to perform invalid action")
	}
	return postState, actions
}

func TestMultipleRounds(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		actor actor
	}{
		{
			name:  "SingleRoot",
			actor: doNothingActor,
		},
		{
			name:  "LinearAttackCorrect",
			actor: correctAttackLastClaim,
		},
		{
			name:  "LinearDefendCorrect",
			actor: correctDefendLastClaim,
		},
		{
			name:  "LinearAttackIncorrect",
			actor: incorrectAttackLastClaim,
		},
		{
			name:  "LinearDefendIncorrect",
			actor: incorrectDefendLastClaim,
		},
		{
			name:  "LinearDefendIncorrectDefendCorrect",
			actor: combineActors(incorrectDefendLastClaim, correctDefendLastClaim),
		},
		{
			name:  "LinearAttackIncorrectDefendCorrect",
			actor: combineActors(incorrectAttackLastClaim, correctDefendLastClaim),
		},
		{
			name:  "LinearDefendIncorrectDefendIncorrect",
			actor: combineActors(incorrectDefendLastClaim, incorrectDefendLastClaim),
		},
		{
			name:  "LinearAttackIncorrectDefendIncorrect",
			actor: combineActors(incorrectAttackLastClaim, incorrectDefendLastClaim),
		},
		{
			name:  "AttackEverythingCorrect",
			actor: attackEverythingCorrect,
		},
		{
			name:  "DefendEverythingCorrect",
			actor: defendEverythingCorrect,
		},
		{
			name:  "AttackEverythingIncorrect",
			actor: attackEverythingIncorrect,
		},
		{
			name:  "DefendEverythingIncorrect",
			actor: defendEverythingIncorrect,
		},
		{
			name:  "Exhaustive",
			actor: exhaustive,
		},
	}
	for _, test := range tests {
		test := test
		for _, rootClaimCorrect := range []bool{true, false} {
			rootClaimCorrect := rootClaimCorrect
			t.Run(fmt.Sprintf("%v-%v", test.name, rootClaimCorrect), func(t *testing.T) {
				t.Parallel()

				maxDepth := types.Depth(6)
				startingL2BlockNumber := big.NewInt(50)
				claimBuilder := faulttest.NewAlphabetClaimBuilder(t, startingL2BlockNumber, maxDepth)
				builder := claimBuilder.GameBuilder(faulttest.WithInvalidValue(!rootClaimCorrect))
				game := builder.Game

				correctTrace := claimBuilder.CorrectTraceProvider()
				solver := NewGameSolver(maxDepth, trace.NewSimpleTraceAccessor(correctTrace))

				roundNum := 0
				done := false
				for !done {
					t.Logf("------ ROUND %v ------", roundNum)
					game, _ = runStep(t, solver, game, correctTrace)
					verifyGameRules(t, game, rootClaimCorrect)

					game, done = test.actor.Apply(t, game, correctTrace)
					roundNum++
				}
			})
		}
	}
}

func applyActions(game types.Game, claimant common.Address, actions []types.Action) types.Game {
	claims := game.Claims()
	for _, action := range actions {
		switch action.Type {
		case types.ActionTypeMove:
			newPosition := action.ParentClaim.Position.Attack()
			if !action.IsAttack {
				newPosition = action.ParentClaim.Position.Defend()
			}
			claim := types.Claim{
				ClaimData: types.ClaimData{
					Value:    action.Value,
					Bond:     big.NewInt(0),
					Position: newPosition,
				},
				Claimant:            claimant,
				ContractIndex:       len(claims),
				ParentContractIndex: action.ParentClaim.ContractIndex,
			}
			claims = append(claims, claim)
		case types.ActionTypeStep:
			counteredClaim := claims[action.ParentClaim.ContractIndex]
			counteredClaim.CounteredBy = claimant
			claims[action.ParentClaim.ContractIndex] = counteredClaim
		default:
			panic(fmt.Errorf("unknown move type: %v", action.Type))
		}
	}
	return types.NewGameState(claims, game.MaxDepth())
}
