package noopbuilder

import (
	"context"
	"errors"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

var ErrNoBuild = errors.New("no building supported")

type Builder struct {
	id seqtypes.BuilderID
}

var _ work.Builder = (*Builder)(nil)

func (n *Builder) NewJob(ctx context.Context, id seqtypes.BuildJobID, opts *seqtypes.BuildOpts) (work.BuildJob, error) {
	return &Job{id: id}, nil
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
