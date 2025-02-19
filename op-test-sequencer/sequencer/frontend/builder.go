package frontend

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type BuildBackend interface {
	CreateJob(ctx context.Context, id seqtypes.BuilderID, opts *seqtypes.BuildOpts) (work.BuildJob, error)
	GetJob(id seqtypes.BuildJobID) work.BuildJob
}

type BuildFrontend struct {
	Backend BuildBackend
}

func (bf *BuildFrontend) Open(ctx context.Context, builderID seqtypes.BuilderID, opts *seqtypes.BuildOpts) (seqtypes.BuildJobID, error) {
	job, err := bf.Backend.CreateJob(ctx, builderID, opts)
	if err != nil {
		return "", err
	}
	return job.ID(), nil
}

func (bf *BuildFrontend) Cancel(ctx context.Context, jobID seqtypes.BuildJobID) error {
	job := bf.Backend.GetJob(jobID)
	if job == nil {
		return nil
	}
	return toJsonError(job.Cancel(ctx))
}

func (bf *BuildFrontend) Seal(ctx context.Context, jobID seqtypes.BuildJobID) (work.Block, error) {
	job := bf.Backend.GetJob(jobID)
	if job == nil {
		return eth.BlockRef{}, seqtypes.ErrUnknownJob
	}
	result, err := job.Seal(ctx)
	if err != nil {
		return nil, toJsonError(err)
	}
	return result, nil
}
