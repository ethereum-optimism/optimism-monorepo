package config

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work/builders/noopbuilder"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type BuilderEntry struct {
	Noop *noopbuilder.Config `yaml:"noop,omitempty"`
}

func (b *BuilderEntry) Start(ctx context.Context, id seqtypes.BuilderID, collection work.Collection, opts *work.StartOpts) (work.Builder, error) {
	switch {
	case b.Noop != nil:
		return b.Noop.Start(ctx, id, collection, opts)
	default:
		return nil, seqtypes.ErrUnknownKind
	}
}
