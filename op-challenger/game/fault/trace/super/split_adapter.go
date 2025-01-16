package super

import (
	"context"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/split"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
)

type ProposalTraceProviderCreator func(ctx context.Context, localContext common.Hash, depth types.Depth, claimInfo ClaimInfo) (types.TraceProvider, error)

type PreimageTraceProvider interface {
	AbsolutePreState(ctx context.Context) (eth.Super, error)
	GetPreimageBytes(ctx context.Context, pos types.Position) ([]byte, error)
	ComputeStep(pos types.Position) (timestamp uint64, step uint64, err error)
}

func SuperRootSplitAdapter(topProvider PreimageTraceProvider, creator ProposalTraceProviderCreator) split.ProviderCreator {
	return func(ctx context.Context, depth types.Depth, pre types.Claim, post types.Claim) (types.TraceProvider, error) {
		localContext := split.CreateLocalContext(pre, post)
		claimInfo, err := FetchClaimInfo(ctx, topProvider, pre, post)
		if err != nil {
			return nil, err
		}
		return creator(ctx, localContext, depth, claimInfo)
	}
}

func FetchClaimInfo(ctx context.Context, topProvider PreimageTraceProvider, pre types.Claim, post types.Claim) (ClaimInfo, error) {
	usePrestateBlock := pre == (types.Claim{})
	var claimInfo ClaimInfo
	if usePrestateBlock {
		absolutePrestatePreimage, err := topProvider.AbsolutePreState(ctx)
		if err != nil {
			return ClaimInfo{}, fmt.Errorf("failed to retrieve absolute prestate preimage: %w", err)
		}
		claimInfo.AgreedPrestate = absolutePrestatePreimage.Marshal()
	} else {
		agreedPrestate, err := topProvider.GetPreimageBytes(ctx, pre.Position)
		if err != nil {
			return ClaimInfo{}, fmt.Errorf("failed to get prestate preimage: %w", err)
		}
		claimInfo.AgreedPrestate = agreedPrestate
	}
	claimInfo.Claim = post.Value
	return claimInfo, nil
}

type ClaimInfo struct {
	AgreedPrestate []byte
	Claim          common.Hash
}
