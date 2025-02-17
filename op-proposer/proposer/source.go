package proposer

import (
	"context"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-service/dial"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type Proposal struct {
	Version eth.Bytes32
	// Root is the proposal hash
	Root common.Hash
	// SequenceNum identifies the position in the overall state transition.
	// For output roots this is the L2 block number.
	// For super roots this is the timestamp.
	SequenceNum uint64
	CurrentL1   eth.BlockID

	// Legacy provides data that is only available when retrieving data from a single rollup node.
	// It should only be used for L2OO proposals.
	Legacy LegacyProposalData
}

type LegacyProposalData struct {
	HeadL1      eth.L1BlockRef
	SafeL2      eth.L2BlockRef
	FinalizedL2 eth.L2BlockRef

	// Support legacy metrics when possible
	BlockRef eth.L2BlockRef
}

type ProposalSource interface {
	ProposalAtBlock(ctx context.Context, blockNum uint64) (Proposal, error)
	SyncStatus(ctx context.Context) (SourceSyncStatus, error)

	// Close closes the underlying client or clients
	Close()
}

type SourceSyncStatus struct {
	CurrentL1   eth.L1BlockRef
	SafeL2      uint64
	FinalizedL2 uint64
}

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

func (r *RollupProposalSource) SyncStatus(ctx context.Context) (SourceSyncStatus, error) {
	client, err := r.provider.RollupClient(ctx)
	if err != nil {
		return SourceSyncStatus{}, fmt.Errorf("failed to select active rollup client: %w", err)
	}
	status, err := client.SyncStatus(ctx)
	if err != nil {
		return SourceSyncStatus{}, err
	}
	return SourceSyncStatus{
		CurrentL1:   status.CurrentL1,
		SafeL2:      status.SafeL2.Number,
		FinalizedL2: status.FinalizedL2.Number,
	}, nil
}

func (r *RollupProposalSource) ProposalAtBlock(ctx context.Context, blockNum uint64) (Proposal, error) {
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

type SupervisorProposalSource struct {
	client *sources.SupervisorClient
}

func NewSupervisorProposalSource(client *sources.SupervisorClient) *SupervisorProposalSource {
	return &SupervisorProposalSource{
		client: client,
	}
}

func (s *SupervisorProposalSource) SyncStatus(ctx context.Context) (SourceSyncStatus, error) {
	status, err := s.client.SyncStatus(ctx)
	if err != nil {
		return SourceSyncStatus{}, err
	}
	return SourceSyncStatus{
		CurrentL1:   status.MinSyncedL1,
		SafeL2:      status.SafeTimestamp,
		FinalizedL2: status.FinalizedTimestamp,
	}, nil
}

func (s *SupervisorProposalSource) ProposalAtBlock(ctx context.Context, blockNum uint64) (Proposal, error) {
	output, err := s.client.SuperRootAtTimestamp(ctx, hexutil.Uint64(blockNum))
	if err != nil {
		return Proposal{}, err
	}
	return Proposal{
		Version:     eth.Bytes32{output.Version},
		Root:        common.Hash(output.SuperRoot),
		SequenceNum: output.Timestamp,
		CurrentL1:   output.CrossSafeDerivedFrom,

		// Unsupported by super root proposals
		Legacy: LegacyProposalData{},
	}, nil
}

func (s *SupervisorProposalSource) Close() {
	s.client.Close()
}
