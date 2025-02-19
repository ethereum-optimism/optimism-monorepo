package config

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work/sequencers/full"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type SequencerEntry struct {
	Full *full.Config `yaml:"full,omitempty"`
}

func (b *SequencerEntry) Start(ctx context.Context, id seqtypes.SequencerID, collection work.Collection, opts *work.StartOpts) (work.Sequencer, error) {
	switch {
	case b.Full != nil:
		return b.Full.Start(ctx, id, collection, opts)
	// TODO
	default:
		return nil, seqtypes.ErrUnknownKind
	}
}
