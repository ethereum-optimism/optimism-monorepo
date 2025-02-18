package noopbuilder

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/builder"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type Job struct {
	id       seqtypes.JobID
	registry builder.Registry
}

var _ builder.BuildJob = (*Job)(nil)

func (job *Job) ID() seqtypes.JobID {
	return job.id
}

func (job *Job) Cancel(ctx context.Context) error {
	job.registry.UnregisterJob(job.id)
	return nil
}

func (job *Job) Seal(ctx context.Context) (eth.BlockRef, error) {
	return eth.BlockRef{}, ErrNoBuild
}

func (job *Job) String() string {
	return job.id.String()
}
