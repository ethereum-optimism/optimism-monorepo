package noopbuilder

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type Config struct {
}

func (c *Config) Start(ctx context.Context, id seqtypes.BuilderID, collection work.Collection, opts *work.StartOpts) (work.Builder, error) {
	return &Builder{
		id: id,
	}, nil
}
