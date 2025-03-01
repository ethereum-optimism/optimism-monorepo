package disputegame

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

type ClaimHelper struct {
	require     *require.Assertions
	game        *SplitGameHelper
	Index       int64
	ParentIndex int
	Position    types.Position
	claim       common.Hash
}

func newClaimHelper(game *SplitGameHelper, idx int64, claim types.Claim) *ClaimHelper {
	return &ClaimHelper{
		require:     game.Require,
		game:        game,
		Index:       idx,
		ParentIndex: claim.ParentContractIndex,
		Position:    claim.Position,
		claim:       claim.Value,
	}
}

func (c *ClaimHelper) AgreesWithOutputRoot() bool {
	return c.Position.Depth()%2 == 0
}

func (c *ClaimHelper) IsRootClaim() bool {
	return c.Position.IsRootPosition()
}

func (c *ClaimHelper) IsOutputRoot(ctx context.Context) bool {
	splitDepth := c.game.SplitDepth(ctx)
	return c.Position.Depth() <= splitDepth
}

func (c *ClaimHelper) IsOutputRootLeaf(ctx context.Context) bool {
	splitDepth := c.game.SplitDepth(ctx)
	return c.Position.Depth() == splitDepth
}

func (c *ClaimHelper) IsBottomGameRoot(ctx context.Context) bool {
	splitDepth := c.game.SplitDepth(ctx)
	return c.Position.Depth() == splitDepth+1
}

func (c *ClaimHelper) IsMaxDepth(ctx context.Context) bool {
	maxDepth := c.game.MaxDepth(ctx)
	return c.Position.Depth() == maxDepth
}

func (c *ClaimHelper) Depth() types.Depth {
	return c.Position.Depth()
}

// WaitForCounterClaim waits for the claim to be countered by another claim being posted.
// It returns a helper for the claim that countered this one.
func (c *ClaimHelper) WaitForCounterClaim(ctx context.Context, ignoreClaims ...*ClaimHelper) *ClaimHelper {
	timeout := defaultTimeout
	if c.IsOutputRootLeaf(ctx) {
		// This is the first claim we need to run cannon on, so give it more time
		timeout = timeout * 2
	}
	counterIdx, counterClaim := c.game.waitForClaim(ctx, timeout, fmt.Sprintf("failed to find claim with parent idx %v", c.Index), func(claimIdx int64, claim types.Claim) bool {
		return int64(claim.ParentContractIndex) == c.Index && !containsClaim(claimIdx, ignoreClaims)
	})
	return newClaimHelper(c.game, counterIdx, counterClaim)
}

// WaitForCountered waits until the claim is countered either by a child claim or by a step call.
func (c *ClaimHelper) WaitForCountered(ctx context.Context) {
	timedCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	err := wait.For(timedCtx, time.Second, func() (bool, error) {
		latestData := c.game.getClaim(ctx, c.Index)
		return latestData.CounteredBy != common.Address{}, nil
	})
	if err != nil { // Avoid waiting time capturing game data when there's no error
		c.require.NoErrorf(err, "Claim %v was not countered\n%v", c.Index, c.game.GameData(ctx))
	}
}

func (c *ClaimHelper) RequireCorrectOutputRoot(ctx context.Context) {
	c.require.True(c.IsOutputRoot(ctx), "Should not expect a valid output root in the bottom game")
	expected, err := c.game.CorrectOutputProvider.Get(ctx, c.Position)
	c.require.NoError(err, "Failed to get correct output root")
	c.require.Equalf(expected, c.claim, "Should have correct output root in claim %v and position %v", c.Index, c.Position)
}

func (c *ClaimHelper) RequireInvalidStatusCode() {
	c.require.Equal(byte(mipsevm.VMStatusInvalid), c.claim[0], "should have had invalid status code")
}

func (c *ClaimHelper) Attack(ctx context.Context, value common.Hash, opts ...MoveOpt) *ClaimHelper {
	c.game.Attack(ctx, c.Index, value, opts...)
	return c.WaitForCounterClaim(ctx)
}

func (c *ClaimHelper) Defend(ctx context.Context, value common.Hash, opts ...MoveOpt) *ClaimHelper {
	c.game.Defend(ctx, c.Index, value, opts...)
	return c.WaitForCounterClaim(ctx)
}

func (c *ClaimHelper) RequireDifferentClaimValue(other *ClaimHelper) {
	c.require.NotEqual(c.claim, other.claim, "should have posted different claims")
}

func (c *ClaimHelper) RequireOnlyCounteredBy(ctx context.Context, expected ...*ClaimHelper) {
	claims := c.game.getAllClaims(ctx)
	for idx, claim := range claims {
		if int64(claim.ParentContractIndex) != c.Index {
			// Doesn't counter this claim, so ignore
			continue
		}
		if !containsClaim(int64(idx), expected) {
			// Found a countering claim not in the expected list. Fail.
			c.require.FailNowf("Found unexpected countering claim", "Parent claim index: %v Game state:\n%v", c.Index, c.game.GameData(ctx))
		}
	}
}

func containsClaim(claimIdx int64, haystack []*ClaimHelper) bool {
	return slices.ContainsFunc(haystack, func(candidate *ClaimHelper) bool {
		return candidate.Index == claimIdx
	})
}
