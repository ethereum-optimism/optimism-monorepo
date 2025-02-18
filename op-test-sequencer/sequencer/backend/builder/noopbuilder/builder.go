package noopbuilder

import (
	"context"
	"errors"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/builder"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

var ErrNoBuild = errors.New("no building supported")

type Builder struct {
	id       seqtypes.BuilderID
	registry builder.Registry
}

var _ builder.Builder = (*Builder)(nil)

func (n *Builder) Attach(registry builder.Registry) {
	n.registry = registry
}

func (n *Builder) NewJob(ctx context.Context, id seqtypes.JobID, opts *seqtypes.BuildOpts) (builder.BuildJob, error) {
	if n.registry == nil {
		return nil, builder.ErrNoRegistry
	}
	return &Job{id: id, registry: n.registry}, nil
}

func (n *Builder) Close() error {
	return nil
}

func (n *Builder) String() string {
	return n.id.String()
}

func (n *Builder) ID() seqtypes.BuilderID {
	return n.id
}
