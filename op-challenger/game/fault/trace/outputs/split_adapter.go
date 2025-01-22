package outputs

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/split"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/utils"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum/go-ethereum/common"
)

type ProposalTraceProviderCreator func(ctx context.Context, localContext common.Hash, depth types.Depth, agreed utils.Proposal, claimed utils.Proposal) (types.TraceProvider, error)

func OutputRootSplitAdapter(topProvider *OutputTraceProvider, creator ProposalTraceProviderCreator) split.ProviderCreator {
	return func(ctx context.Context, depth types.Depth, pre types.Claim, post types.Claim) (types.TraceProvider, error) {
		localContext := split.CreateLocalContext(pre, post)
		agreed, disputed, err := FetchProposals(ctx, topProvider, pre, post)
		if err != nil {
			return nil, err
		}
		return creator(ctx, localContext, depth, agreed, disputed)
	}
}

func FetchProposals(ctx context.Context, topProvider *OutputTraceProvider, pre types.Claim, post types.Claim) (utils.Proposal, utils.Proposal, error) {
	usePrestateBlock := pre == (types.Claim{})
	var agreed utils.Proposal
	if usePrestateBlock {
		prestateRoot, err := topProvider.AbsolutePreStateCommitment(ctx)
		if err != nil {
			return utils.Proposal{}, utils.Proposal{}, fmt.Errorf("failed to retrieve absolute prestate output root: %w", err)
		}
		agreed = utils.Proposal{
			L2BlockNumber: new(big.Int).SetUint64(topProvider.prestateBlock),
			OutputRoot:    prestateRoot,
		}
	} else {
		preBlockNum, err := topProvider.HonestBlockNumber(ctx, pre.Position)
		if err != nil {
			return utils.Proposal{}, utils.Proposal{}, fmt.Errorf("unable to calculate pre-claim block number: %w", err)
		}
		agreed = utils.Proposal{
			L2BlockNumber: new(big.Int).SetUint64(preBlockNum),
			OutputRoot:    pre.Value,
		}
	}
	postBlockNum, err := topProvider.ClaimedBlockNumber(post.Position)
	if err != nil {
		return utils.Proposal{}, utils.Proposal{}, fmt.Errorf("unable to calculate post-claim block number: %w", err)
	}
	claimed := utils.Proposal{
		L2BlockNumber: new(big.Int).SetUint64(postBlockNum),
		OutputRoot:    post.Value,
	}
	return agreed, claimed, nil
}
