package proposer

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-service/dial"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
)

type ProposalSourceProvider interface {
	// ProposalSource returns an available ProposalSource
	// Note: ctx should be a lifecycle context without an attached timeout as client selection may involve
	// multiple network operations, specifically in the case of failover.
	ProposalSource(ctx context.Context) (ProposalSource, error)

	// Close closes the underlying client or clients
	Close()
}

type Proposal struct {
	Version     eth.Bytes32
	Root        common.Hash
	BlockRef    eth.L2BlockRef
	HeadL1      eth.L1BlockRef
	CurrentL1   eth.L1BlockRef
	SafeL2      eth.L2BlockRef
	FinalizedL2 eth.L2BlockRef
}

type ProposalSource interface {
	ProposalAtBlock(ctx context.Context, blockNum uint64) (Proposal, error)
	SyncStatus(ctx context.Context) (SourceSyncStatus, error)
}

type SourceSyncStatus struct {
	CurrentL1   eth.L1BlockRef
	SafeL2      eth.L2BlockRef
	FinalizedL2 eth.L2BlockRef
}

type RollupProposalSourceProvider struct {
	provider dial.RollupProvider
}

func (r *RollupProposalSourceProvider) ProposalSource(ctx context.Context) (ProposalSource, error) {
	client, err := r.provider.RollupClient(ctx)
	if err != nil {
		return nil, err
	}
	return &RollupProposalSource{
		client: client,
	}, nil
}

func (r *RollupProposalSourceProvider) Close() {
	r.provider.Close()
}

func NewRollupProposalSourceProvider(provider dial.RollupProvider) *RollupProposalSourceProvider {
	return &RollupProposalSourceProvider{
		provider: provider,
	}
}

type RollupProposalSource struct {
	client dial.RollupClientInterface
}

func (r *RollupProposalSource) SyncStatus(ctx context.Context) (SourceSyncStatus, error) {
	status, err := r.client.SyncStatus(ctx)
	if err != nil {
		return SourceSyncStatus{}, err
	}
	return SourceSyncStatus{
		SafeL2:      status.SafeL2,
		FinalizedL2: status.FinalizedL2,
	}, nil
}

func (r *RollupProposalSource) ProposalAtBlock(ctx context.Context, blockNum uint64) (Proposal, error) {
	output, err := r.client.OutputAtBlock(ctx, blockNum)
	if err != nil {
		return Proposal{}, err
	}
	return Proposal{
		Version:     output.Version,
		Root:        common.Hash(output.OutputRoot),
		BlockRef:    output.BlockRef,
		HeadL1:      output.Status.HeadL1,
		CurrentL1:   output.Status.CurrentL1,
		SafeL2:      output.Status.SafeL2,
		FinalizedL2: output.Status.FinalizedL2,
	}, nil
}
