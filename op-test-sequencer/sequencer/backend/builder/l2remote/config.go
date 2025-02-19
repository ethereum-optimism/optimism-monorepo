package l2remote

import (
	"context"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"

	"github.com/ethereum-optimism/optimism/op-service/endpoint"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

type Config struct {
	// L1 execution-layer RPC endpoint
	L1EL endpoint.MustRPC `yaml:"l1EL,omitempty"`

	// L2 execution-layer RPC endpoint
	L2EL endpoint.MustRPC `yaml:"l2EL,omitempty"`
	// L2 consensus-layer RPC endpoint
	L2CL endpoint.MustRPC `yaml:"l2CL,omitempty"`
}

func (c *Config) Start(ctx context.Context, id seqtypes.BuilderID, chainID eth.ChainID, opts *work.StartOpts) (*Builder, error) {

	//cl, err := client.NewRPC(ctx, client.WithLazyDial())
	// TODO dial RPCs

	b := &Builder{
		id:  id,
		log: opts.Log,
		m:   opts.Metrics,
	}
	return nil, nil
}
