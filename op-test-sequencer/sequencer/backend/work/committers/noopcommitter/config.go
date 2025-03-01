package noopcommitter

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type Config struct {
}

func (c *Config) Start(ctx context.Context, id seqtypes.CommitterID, collection work.Collection, opts *work.StartOpts) (work.Committer, error) {
	return &Committer{
		id:  id,
		log: opts.Log,
	}, nil
}
