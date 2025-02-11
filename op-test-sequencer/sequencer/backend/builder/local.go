package builder

import (
	"context"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/metrics"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/frontend"
)

type LocalBuilder struct {
	log log.Logger
	m   metrics.Metricer
}

var _ Builder = (*LocalBuilder)(nil)

func NewLocalBuilder(log log.Logger, m metrics.Metricer) *LocalBuilder {
	return &LocalBuilder{
		log: log,
		m:   m,
	}
}

func (ba *LocalBuilder) Open(ctx context.Context) (frontend.JobID, error) {
	//TODO implement me
	panic("implement me")
}

func (ba *LocalBuilder) Cancel(ctx context.Context, jobID frontend.JobID) error {
	//TODO implement me
	panic("implement me")
}

func (ba *LocalBuilder) Seal(ctx context.Context, jobID frontend.JobID) (eth.BlockRef, error) {
	//TODO implement me
	panic("implement me")
}

func (ba *LocalBuilder) Close() error {
	// TODO
	return nil
}
