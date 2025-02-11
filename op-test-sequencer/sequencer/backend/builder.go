package backend

import (
	"context"
	"errors"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/metrics"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/frontend"
)

type Builder struct {
	log log.Logger
	m   metrics.Metricer
}

var _ frontend.BuildBackend = (*Builder)(nil)

func NewBuilder(log log.Logger, m metrics.Metricer) *Builder {
	return &Builder{
		log: log,
		m:   m,
	}
}

func (ba *Builder) Open(ctx context.Context) (frontend.JobID, error) {
	//TODO implement me
	panic("implement me")
}

func (ba *Builder) Cancel(ctx context.Context, jobID frontend.JobID) error {
	//TODO implement me
	panic("implement me")
}

func (ba *Builder) Seal(ctx context.Context, jobID frontend.JobID) (eth.BlockRef, error) {
	//TODO implement me
	panic("implement me")
}

func (ba *Builder) Close() error {
	// TODO
	return nil
}

var ErrNoBuild = errors.New("no building supported")

type NoopBuilder struct{}

func (n *NoopBuilder) Open(ctx context.Context) (frontend.JobID, error) {
	return "", ErrNoBuild
}

func (n *NoopBuilder) Cancel(ctx context.Context, jobID frontend.JobID) error {
	return ErrNoBuild
}

func (n *NoopBuilder) Seal(ctx context.Context, jobID frontend.JobID) (eth.BlockRef, error) {
	return eth.BlockRef{}, ErrNoBuild
}

var _ frontend.BuildBackend = (*NoopBuilder)(nil)
