package l2remote

import (
	"context"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/metrics"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/builder"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type Builder struct {
	id       seqtypes.BuilderID
	registry builder.Registry
	log      log.Logger
	m        metrics.Metricer
}

var _ builder.Builder = (*Builder)(nil)

func (b *Builder) Attach(registry builder.Registry) {
	b.registry = registry
}

func (b *Builder) NewJob(ctx context.Context, id seqtypes.JobID, opts *seqtypes.BuildOpts) (builder.BuildJob, error) {
	if b.registry == nil {
		return nil, builder.ErrNoRegistry
	}
	// TODO
	return nil, nil
}

func (b *Builder) Close() error {
	// TODO close RPC clients etc.
	return nil
}

func (b *Builder) String() string {
	return b.id.String()
}

func (b *Builder) ID() seqtypes.BuilderID {
	return b.id
}
