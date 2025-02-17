package source

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type SupervisorProposalSource struct {
	client *sources.SupervisorClient
}

func NewSupervisorProposalSource(client *sources.SupervisorClient) *SupervisorProposalSource {
	return &SupervisorProposalSource{
		client: client,
	}
}

func (s *SupervisorProposalSource) SyncStatus(ctx context.Context) (SyncStatus, error) {
	status, err := s.client.SyncStatus(ctx)
	if err != nil {
		return SyncStatus{}, err
	}
	return SyncStatus{
		CurrentL1:   status.MinSyncedL1,
		SafeL2:      status.SafeTimestamp,
		FinalizedL2: status.FinalizedTimestamp,
	}, nil
}

func (s *SupervisorProposalSource) ProposalAtSequenceNum(ctx context.Context, blockNum uint64) (Proposal, error) {
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
