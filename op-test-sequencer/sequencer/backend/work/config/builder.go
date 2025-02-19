package config

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work/builders/l2eng"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work/builders/noopbuilder"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type BuilderEntry struct {
	L2Eng *l2eng.Config       `yaml:"l2Eng,omitempty"`
	Noop  *noopbuilder.Config `yaml:"noop,omitempty"`
}

func (b *BuilderEntry) Start(ctx context.Context, id seqtypes.BuilderID, collection work.Collection, opts *work.StartOpts) (work.Builder, error) {
	switch {
	case b.L2Eng != nil:
		return b.L2Eng.Start(ctx, id, collection, opts)
	case b.Noop != nil:
		return b.Noop.Start(ctx, id, collection, opts)
	default:
		return nil, seqtypes.ErrUnknownKind
	}
}
