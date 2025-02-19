package noopbuilder

import (
	"context"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type Job struct {
	id seqtypes.BuildJobID
}

var _ work.BuildJob = (*Job)(nil)

func (job *Job) ID() seqtypes.BuildJobID {
	return job.id
}

func (job *Job) Cancel(ctx context.Context) error {
	return nil
}

func (job *Job) Seal(ctx context.Context) (eth.BlockRef, error) {
	return eth.BlockRef{}, ErrNoBuild
}

func (job *Job) String() string {
	return job.id.String()
}
