package disputegame

import (
	"context"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/transactions"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching/rpcblock"
	"github.com/stretchr/testify/require"
)

const getTraceTimeout = 10 * time.Minute

type OutputHonestHelper struct {
	t            *testing.T
	require      *require.Assertions
	game         *SplitGameHelper
	contract     contracts.FaultDisputeGameContract
	correctTrace types.TraceAccessor
}

func NewOutputHonestHelper(t *testing.T, require *require.Assertions, game *SplitGameHelper, contract contracts.FaultDisputeGameContract, correctTrace types.TraceAccessor) *OutputHonestHelper {
	return &OutputHonestHelper{
		t:            t,
		require:      require,
		game:         game,
		contract:     contract,
		correctTrace: correctTrace,
	}
}

func (h *OutputHonestHelper) CounterClaim(ctx context.Context, claim *ClaimHelper, opts ...MoveOpt) *ClaimHelper {
	game, target := h.loadState(ctx, claim.Index)
	value, err := h.correctTrace.Get(ctx, game, target, target.Position)
	h.require.NoErrorf(err, "Failed to determine correct claim at position %v with g index %v", target.Position, target.Position.ToGIndex())
	if value == claim.claim {
		return h.DefendClaim(ctx, claim, opts...)
	} else {
		return h.AttackClaim(ctx, claim, opts...)
	}
}

func (h *OutputHonestHelper) AttackClaim(ctx context.Context, claim *ClaimHelper, opts ...MoveOpt) *ClaimHelper {
	h.Attack(ctx, claim.Index, opts...)
	return claim.WaitForCounterClaim(ctx)
}

func (h *OutputHonestHelper) DefendClaim(ctx context.Context, claim *ClaimHelper, opts ...MoveOpt) *ClaimHelper {
	h.Defend(ctx, claim.Index, opts...)
	return claim.WaitForCounterClaim(ctx)
}

func (h *OutputHonestHelper) Attack(ctx context.Context, claimIdx int64, opts ...MoveOpt) {
	// Ensure the claim exists
	h.game.WaitForClaimCount(ctx, claimIdx+1)

	ctx, cancel := context.WithTimeout(ctx, getTraceTimeout)
	defer cancel()

	game, claim := h.loadState(ctx, claimIdx)
	attackPos := claim.Position.Attack()
	h.t.Logf("Attacking claim %v at position %v with g index %v", claimIdx, attackPos, attackPos.ToGIndex())
	value, err := h.correctTrace.Get(ctx, game, claim, attackPos)
	h.require.NoErrorf(err, "Get correct claim at position %v with g index %v", attackPos, attackPos.ToGIndex())
	h.t.Log("Performing attack")
	h.game.Attack(ctx, claimIdx, value, opts...)
	h.t.Log("Attack complete")
}

func (h *OutputHonestHelper) Defend(ctx context.Context, claimIdx int64, opts ...MoveOpt) {
	// Ensure the claim exists
	h.game.WaitForClaimCount(ctx, claimIdx+1)

	ctx, cancel := context.WithTimeout(ctx, getTraceTimeout)
	defer cancel()
	game, claim := h.loadState(ctx, claimIdx)
	defendPos := claim.Position.Defend()
	value, err := h.correctTrace.Get(ctx, game, claim, defendPos)
	h.game.Require.NoErrorf(err, "Get correct claim at position %v with g index %v", defendPos, defendPos.ToGIndex())
	h.game.Defend(ctx, claimIdx, value, opts...)
}

func (h *OutputHonestHelper) StepClaimFails(ctx context.Context, claim *ClaimHelper, isAttack bool) {
	h.StepFails(ctx, claim.Index, isAttack)
}

func (h *OutputHonestHelper) StepFails(ctx context.Context, claimIdx int64, isAttack bool) {
	// Ensure the claim exists
	h.game.WaitForClaimCount(ctx, claimIdx+1)

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	game, claim := h.loadState(ctx, claimIdx)
	pos := claim.Position
	if !isAttack {
		// If we're defending, then the step will be from the trace to the next one
		pos = pos.MoveRight()
	}
	prestate, proofData, preimage, err := h.correctTrace.GetStepData(ctx, game, claim, pos)
	h.require.NoError(err, "Get step data")
	if preimage != nil {
		tx, err := h.game.Game.UpdateOracleTx(ctx, uint64(claimIdx), preimage)
		h.require.NoError(err)
		transactions.RequireSendTx(h.t, ctx, h.game.Client, tx, h.game.PrivKey)
	}
	h.game.StepFails(ctx, claimIdx, isAttack, prestate, proofData)
}

func (h *OutputHonestHelper) loadState(ctx context.Context, claimIdx int64) (types.Game, types.Claim) {
	claims, err := h.contract.GetAllClaims(ctx, rpcblock.Latest)
	h.require.NoError(err, "Failed to load claims from game")
	game := types.NewGameState(claims, h.game.MaxDepth(ctx))

	claim := game.Claims()[claimIdx]
	return game, claim
}
