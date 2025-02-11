package config

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work/committers/noopcommitter"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type CommitterEntry struct {
	Noop *noopcommitter.Config `yaml:"noop,omitempty"`
}

func (b *CommitterEntry) Start(ctx context.Context, id seqtypes.CommitterID, collection work.Collection, opts *work.StartOpts) (work.Committer, error) {
	switch {
	case b.Noop != nil:
		return b.Noop.Start(ctx, id, collection, opts)
	default:
		return nil, seqtypes.ErrUnknownKind
	}
}
