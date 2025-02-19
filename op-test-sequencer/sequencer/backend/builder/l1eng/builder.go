package l1eng

import (
	"context"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/metrics"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type Builder struct {
	id  seqtypes.BuilderID
	log log.Logger
	m   metrics.Metricer
}

var _ work.Builder = (*Builder)(nil)

func (b *Builder) NewJob(ctx context.Context, id seqtypes.BuildJobID, opts *seqtypes.BuildOpts) (work.BuildJob, error) {
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
