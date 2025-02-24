package config

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work/publishers/nooppublisher"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type PublisherEntry struct {
	RPC  any                   `yaml:"rpc,omitempty"`
	Noop *nooppublisher.Config `yaml:"noop,omitempty"`
}

func (b *PublisherEntry) Start(ctx context.Context, id seqtypes.PublisherID, collection work.Collection, opts *work.StartOpts) (work.Publisher, error) {
	switch {
	case b.Noop != nil:
		return b.Noop.Start(ctx, id, collection, opts)
	default:
		return nil, seqtypes.ErrUnknownKind
	}
}
