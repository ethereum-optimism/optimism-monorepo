package config

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work/sequencers/fullseq"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work/sequencers/noopseq"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type SequencerEntry struct {
	Full *fullseq.Config `yaml:"full,omitempty"`
	Noop *noopseq.Config `yaml:"noop,omitempty"`
}

func (b *SequencerEntry) Start(ctx context.Context, id seqtypes.SequencerID, collection work.Collection, opts *work.StartOpts) (work.Sequencer, error) {
	switch {
	case b.Full != nil:
		return b.Full.Start(ctx, id, collection, opts)
	case b.Noop != nil:
		return b.Noop.Start(ctx, id, collection, opts)
	default:
		return nil, seqtypes.ErrUnknownKind
	}
}
