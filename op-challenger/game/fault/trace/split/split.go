package split

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
)

var (
	errRefClaimNotDeepEnough = errors.New("reference claim is not deep enough")
)

type ProviderCreator func(ctx context.Context, depth types.Depth, pre types.Claim, post types.Claim) (types.TraceProvider, error)

func NewSplitProviderSelector(topProvider types.TraceProvider, topDepth types.Depth, bottomProviderCreator ProviderCreator) trace.ProviderSelector {
	return func(ctx context.Context, game types.Game, ref types.Claim, pos types.Position) (types.TraceProvider, error) {
		if pos.Depth() <= topDepth {
			return topProvider, nil
		}
		if ref.Position.Depth() < topDepth {
			return nil, fmt.Errorf("%w, claim depth: %v, depth required: %v", errRefClaimNotDeepEnough, ref.Position.Depth(), topDepth)
		}

		// Find the ancestor claim at the leaf level for the top game.
		topLeaf, err := findAncestorAtDepth(game, ref, topDepth)
		if err != nil {
			return nil, err
		}

		var pre, post types.Claim
		// If pos is to the right of the leaf from the top game, we must be defending that output root
		// otherwise, we're attacking it.
		if pos.IndexAtDepth().Cmp(topLeaf.TraceIndex(pos.Depth())) > 0 {
			// Defending the top leaf claim, so use it as the pre-claim and find the post
			pre = topLeaf
			postTraceIdx := new(big.Int).Add(pre.TraceIndex(topDepth), big.NewInt(1))
			post, err = findAncestorWithTraceIndex(game, topLeaf, topDepth, postTraceIdx)
			if err != nil {
				return nil, fmt.Errorf("failed to find post claim: %w", err)
			}
		} else {
			// Attacking the top leaf claim, so use it as the post-claim and find the pre
			post = topLeaf
			postTraceIdx := post.TraceIndex(topDepth)
			if postTraceIdx.Cmp(big.NewInt(0)) == 0 {
				pre = types.Claim{}
			} else {
				preTraceIdx := new(big.Int).Sub(postTraceIdx, big.NewInt(1))
				pre, err = findAncestorWithTraceIndex(game, topLeaf, topDepth, preTraceIdx)
				if err != nil {
					return nil, fmt.Errorf("failed to find pre claim: %w", err)
				}
			}
		}
		// The top game runs from depth 0 to split depth *inclusive*.
		// The - 1 here accounts for the fact that the split depth is included in the top game.
		bottomDepth := game.MaxDepth() - topDepth - 1
		provider, err := bottomProviderCreator(ctx, bottomDepth, pre, post)
		if err != nil {
			return nil, err
		}
		// Translate such that the root of the bottom game is the level below the top game leaf
		return trace.Translate(provider, topDepth+1), nil
	}
}

func findAncestorAtDepth(game types.Game, claim types.Claim, depth types.Depth) (types.Claim, error) {
	for claim.Depth() > depth {
		parent, err := game.GetParent(claim)
		if err != nil {
			return types.Claim{}, fmt.Errorf("failed to find ancestor at depth %v: %w", depth, err)
		}
		claim = parent
	}
	return claim, nil
}

func findAncestorWithTraceIndex(game types.Game, ref types.Claim, depth types.Depth, traceIdx *big.Int) (types.Claim, error) {
	candidate := ref
	for candidate.TraceIndex(depth).Cmp(traceIdx) != 0 {
		parent, err := game.GetParent(candidate)
		if err != nil {
			return types.Claim{}, fmt.Errorf("failed to get parent of claim %v: %w", candidate.ContractIndex, err)
		}
		candidate = parent
	}
	return candidate, nil
}
