package nooppublisher

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type Config struct {
}

func (c *Config) Start(ctx context.Context, id seqtypes.PublisherID, collection work.Collection, opts *work.StartOpts) (work.Publisher, error) {
	return &Publisher{
		id:  id,
		log: opts.Log,
	}, nil
}
