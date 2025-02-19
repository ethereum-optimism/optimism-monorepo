package source

import (
	"context"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-service/dial"
	"github.com/ethereum/go-ethereum/common"
)

type RollupProposalSource struct {
	provider dial.RollupProvider
}

func NewRollupProposalSource(provider dial.RollupProvider) *RollupProposalSource {
	return &RollupProposalSource{
		provider: provider,
	}
}

func (r *RollupProposalSource) Close() {
	r.provider.Close()
}

func (r *RollupProposalSource) SyncStatus(ctx context.Context) (SyncStatus, error) {
	client, err := r.provider.RollupClient(ctx)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("failed to select active rollup client: %w", err)
	}
	status, err := client.SyncStatus(ctx)
	if err != nil {
		return SyncStatus{}, err
	}
	return SyncStatus{
		CurrentL1:   status.CurrentL1,
		SafeL2:      status.SafeL2.Number,
		FinalizedL2: status.FinalizedL2.Number,
	}, nil
}

func (r *RollupProposalSource) ProposalAtSequenceNum(ctx context.Context, blockNum uint64) (Proposal, error) {
	client, err := r.provider.RollupClient(ctx)
	if err != nil {
		return Proposal{}, fmt.Errorf("failed to select active rollup client: %w", err)
	}
	output, err := client.OutputAtBlock(ctx, blockNum)
	if err != nil {
		return Proposal{}, err
	}
	return Proposal{
		Version:     output.Version,
		Root:        common.Hash(output.OutputRoot),
		SequenceNum: output.BlockRef.Number,
		CurrentL1:   output.Status.CurrentL1.ID(),
		Legacy: LegacyProposalData{
			HeadL1:      output.Status.HeadL1,
			SafeL2:      output.Status.SafeL2,
			FinalizedL2: output.Status.FinalizedL2,
			BlockRef:    output.BlockRef,
		},
	}, nil
}
