package builder

import (
	"context"
	"errors"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

var ErrNoBuild = errors.New("no building supported")
var ErrNoRegistry = errors.New("no registry attached")

type NoopBuilder struct {
	ID       seqtypes.BuilderID
	registry Registry
}

var _ Builder = (*NoopBuilder)(nil)

func (n *NoopBuilder) Attach(registry Registry) {
	n.registry = registry
}

func (n *NoopBuilder) NewJob(id seqtypes.JobID) (BuildJob, error) {
	if n.registry == nil {
		return nil, ErrNoRegistry
	}
	return &NoopBuildJob{id: id, registry: n.registry}, nil
}

func (n *NoopBuilder) Close() error {
	return nil
}

func (n *NoopBuilder) String() string {
	return n.ID.String()
}

type NoopBuildJob struct {
	id       seqtypes.JobID
	registry Registry
}

func (n *NoopBuildJob) ID() seqtypes.JobID {
	return n.id
}

func (n *NoopBuildJob) Cancel(ctx context.Context) error {
	n.registry.UnregisterJob(n.id)
	return nil
}

func (n *NoopBuildJob) Seal(ctx context.Context) (eth.BlockRef, error) {
	return eth.BlockRef{}, ErrNoBuild
}

func (n *NoopBuildJob) String() string {
	return n.id.String()
}

var _ BuildJob = (*NoopBuildJob)(nil)
