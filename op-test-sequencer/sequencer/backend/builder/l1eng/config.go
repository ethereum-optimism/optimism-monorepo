package l1eng

import (
	"context"
	"github.com/ethereum-optimism/optimism/op-service/endpoint"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type Config struct {
	// L1 execution-layer RPC endpoint
	L1EL endpoint.MustRPC `yaml:"l1EL,omitempty"`

	// L1 engine-API RPC endpoint
	L1Engine endpoint.MustRPC `yaml:"l1Engine,omitempty"`
}

func (c *Config) Start(ctx context.Context, id seqtypes.BuilderID, chainID eth.ChainID, opts *work.StartOpts) (*Builder, error) {
	// TODO dial RPCs

	bu := &Builder{
		id:       id,
		registry: nil,
	}
	return bu, nil
}
