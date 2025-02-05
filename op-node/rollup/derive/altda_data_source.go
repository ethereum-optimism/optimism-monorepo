package derive

import (
	"context"
	"errors"
	"fmt"

	altda "github.com/ethereum-optimism/optimism/op-alt-da"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive/params"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/log"
)

// AltDADataSource is a data source that fetches inputs from a AltDA provider given
// their onchain commitments. Same as CalldataSource it will keep attempting to fetch.
type AltDADataSource struct {
	log     log.Logger
	src     DataIter
	fetcher AltDAInputFetcher
	l1      L1Fetcher
	id      eth.L1BlockRef
	// keep track of a pending commitment so we can keep trying to fetch the input.
	comm altda.CommitmentData
}

func NewAltDADataSource(log log.Logger, src DataIter, l1 L1Fetcher, fetcher AltDAInputFetcher, id eth.L1BlockRef) *AltDADataSource {
	return &AltDADataSource{
		log:     log,
		src:     src,
		fetcher: fetcher,
		l1:      l1,
		id:      id,
	}
}

func (s *AltDADataSource) Next(ctx context.Context) (eth.Data, error) {
	// Process origin syncs the challenge contract events and updates the local challenge states
	// before we can proceed to fetch the input data. This function can be called multiple times
	// for the same origin and noop if the origin was already processed. It is also called if
	// there is not commitment in the current origin.
	if err := s.fetcher.AdvanceL1Origin(ctx, s.l1, s.id.ID()); err != nil {
		if errors.Is(err, altda.ErrReorgRequired) {
			return nil, NewResetError(errors.New("new expired challenge"))
		}
		return nil, NewTemporaryError(fmt.Errorf("failed to advance altDA L1 origin: %w", err))
	}

	if s.comm == nil {
		// the l1 source returns the input commitment for the batch.
		data, err := s.src.Next(ctx)
		if err != nil {
			return nil, err
		}

		if len(data) == 0 {
			return nil, NotEnoughData
		}
		// If the tx data type is not altDA, we forward it downstream to let the next
		// steps validate and potentially parse it as L1 DA inputs.
		if data[0] != params.DerivationVersion1 {
			return data, nil
		}

		// validate batcher inbox data is a commitment.
		// strip the transaction data version byte from the data before decoding.
		comm, err := altda.DecodeCommitmentData(data[1:])
		if err != nil {
			s.log.Warn("invalid commitment", "commitment", data, "err", err)
			return nil, NotEnoughData
		}
		s.comm = comm
	}
	// use the commitment to fetch the input from the AltDA provider.
	data, err := s.fetcher.GetInput(ctx, s.l1, s.comm, s.id)
	// GetInput may call for a reorg if the pipeline is stalled and the AltDA manager
	// continued syncing origins detached from the pipeline origin.
	if errors.Is(err, altda.ErrReorgRequired) {
		// challenge for a new previously derived commitment expired.
		return nil, NewResetError(err)
	} else if errors.Is(err, altda.ErrExpiredChallenge) {
		// this commitment was challenged and the challenge expired.
		s.log.Warn("challenge expired, skipping batch", "comm", s.comm)
		s.comm = nil
		// skip the input
		return s.Next(ctx)
	} else if errors.Is(err, altda.ErrMissingPastWindow) {
		return nil, NewCriticalError(fmt.Errorf("data for comm %s not available: %w", s.comm, err))
	} else if errors.Is(err, altda.ErrPendingChallenge) {
		// continue stepping without slowing down.
		return nil, NotEnoughData
	} else if err != nil {
		// return temporary error so we can keep retrying.
		return nil, NewTemporaryError(fmt.Errorf("failed to fetch input data with comm %s from da service: %w", s.comm, err))
	}
	// inputs are limited to a max size to ensure they can be challenged in the DA contract.
	if s.comm.CommitmentType() == altda.Keccak256CommitmentType && len(data) > altda.MaxInputSize {
		s.log.Warn("input data exceeds max size", "size", len(data), "max", altda.MaxInputSize)
		s.comm = nil
		return s.Next(ctx)
	}
	// reset the commitment so we can fetch the next one from the source at the next iteration.
	s.comm = nil
	return data, nil
}
