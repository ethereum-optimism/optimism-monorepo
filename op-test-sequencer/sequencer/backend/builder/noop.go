package builder

import (
	"context"
	"errors"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/frontend"
)

var ErrNoBuild = errors.New("no building supported")

type NoopBuilder struct{}

var _ Builder = (*NoopBuilder)(nil)

func (n NoopBuilder) Open(ctx context.Context) (frontend.JobID, error) {
	return "", ErrNoBuild
}

func (n NoopBuilder) Cancel(ctx context.Context, jobID frontend.JobID) error {
	return ErrNoBuild
}

func (n NoopBuilder) Seal(ctx context.Context, jobID frontend.JobID) (eth.BlockRef, error) {
	return eth.BlockRef{}, ErrNoBuild
}

func (n NoopBuilder) Close() error {
	return nil
}
