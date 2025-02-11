package noopseq

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type Config struct {
}

func (c *Config) Start(ctx context.Context, id seqtypes.SequencerID, collection work.Collection, opts *work.StartOpts) (work.Sequencer, error) {
	return &Sequencer{
		id:  id,
		log: opts.Log,
	}, nil
}
