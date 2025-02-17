package frontend

import (
	"context"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/builder"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type AdminBackend interface {
	Hello(ctx context.Context, name string) (string, error)
}

type AdminFrontend struct {
	Backend AdminBackend
}

func (af *AdminFrontend) Hello(ctx context.Context, name string) (string, error) {
	return af.Backend.Hello(ctx, name)
}

type BuildBackend interface {
	CreateJob(ctx context.Context, id seqtypes.BuilderID, opts *seqtypes.BuildOpts) (builder.BuildJob, error)
	GetJob(id seqtypes.JobID) builder.BuildJob
}

type BuildFrontend struct {
	Backend BuildBackend
}

func (bf *BuildFrontend) Open(ctx context.Context, builderID seqtypes.BuilderID, opts *seqtypes.BuildOpts) (seqtypes.JobID, error) {
	job, err := bf.Backend.CreateJob(ctx, builderID, opts)
	if err != nil {
		return "", err
	}
	return job.ID(), nil
}

func (bf *BuildFrontend) Cancel(ctx context.Context, jobID seqtypes.JobID) error {
	job := bf.Backend.GetJob(jobID)
	if job == nil {
		return nil
	}
	return job.Cancel(ctx)
}

func (bf *BuildFrontend) Seal(ctx context.Context, jobID seqtypes.JobID) (eth.BlockRef, error) {
	job := bf.Backend.GetJob(jobID)
	if job == nil {
		return eth.BlockRef{}, seqtypes.ErrUnknownJob
	}
	return job.Seal(ctx)
}

func (bf *BuildFrontend) Sign(ctx context.Context, jobID seqtypes.JobID) error {
	return nil
}

func (bf *BuildFrontend) Publish(ctx context.Context, jobID seqtypes.JobID) error {
	return nil
}
