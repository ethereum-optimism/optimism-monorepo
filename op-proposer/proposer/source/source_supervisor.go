package source

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
)

type SupervisorClient interface {
	SyncStatus(ctx context.Context) (eth.SupervisorSyncStatus, error)
	SuperRootAtTimestamp(ctx context.Context, timestamp hexutil.Uint64) (eth.SuperRootResponse, error)
	Close()
}

type SupervisorProposalSource struct {
	log     log.Logger
	clients []SupervisorClient
}

func NewSupervisorProposalSource(logger log.Logger, clients ...SupervisorClient) *SupervisorProposalSource {
	if len(clients) == 0 {
		panic("no supervisor clients provided")
	}
	return &SupervisorProposalSource{
		log:     logger,
		clients: clients,
	}
}

type statusResult struct {
	idx    int
	status eth.SupervisorSyncStatus
	err    error
}

func (s *SupervisorProposalSource) SyncStatus(ctx context.Context) (SyncStatus, error) {
	var wg sync.WaitGroup
	results := make(chan statusResult, len(s.clients))
	wg.Add(len(s.clients))
	for i, client := range s.clients {
		i := i
		client := client
		go func() {
			defer wg.Done()
			status, err := client.SyncStatus(ctx)
			results <- statusResult{
				idx:    i,
				status: status,
				err:    err,
			}
		}()
	}
	wg.Wait()
	close(results)
	var errs []error
	var earliestResponse eth.SupervisorSyncStatus
	for result := range results {
		if result.err != nil {
			s.log.Warn("Failed to retrieve sync status from supervisor", "idx", result.idx, "err", result.err)
			errs = append(errs, result.err)
			continue
		}
		if earliestResponse.MinSyncedL1 == (eth.L1BlockRef{}) || result.status.MinSyncedL1.Number < earliestResponse.MinSyncedL1.Number {
			earliestResponse = result.status
		}
	}
	if earliestResponse.MinSyncedL1 == (eth.L1BlockRef{}) {
		return SyncStatus{}, fmt.Errorf("no available sync status sources: %w", errors.Join(errs...))
	}
	return SyncStatus{
		CurrentL1:   earliestResponse.MinSyncedL1,
		SafeL2:      earliestResponse.SafeTimestamp,
		FinalizedL2: earliestResponse.FinalizedTimestamp,
	}, nil
}

func (s *SupervisorProposalSource) ProposalAtSequenceNum(ctx context.Context, timestamp uint64) (Proposal, error) {
	var errs []error
	for i, client := range s.clients {
		output, err := client.SuperRootAtTimestamp(ctx, hexutil.Uint64(timestamp))
		if err != nil {
			errs = append(errs, err)
			s.log.Warn("Failed to retrieve proposal from supervisor", "idx", i, "err", err)
			continue
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
	return Proposal{}, fmt.Errorf("no available proposal sources: %w", errors.Join(errs...))
}

func (s *SupervisorProposalSource) Close() {
	for _, client := range s.clients {
		client.Close()
	}
}
