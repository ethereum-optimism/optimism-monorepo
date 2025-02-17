package builder

import (
	"context"
	"errors"
	"fmt"
	"github.com/ethereum-optimism/optimism/op-service/endpoint"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type Config struct {
	Builders map[seqtypes.BuilderID]*BuilderEntry `yaml:"builders"`

	Signers map[seqtypes.SignerID]*SignerEntry `yaml:"signers"`
}

var _ Loader = (*Config)(nil)

// Load is a short-cut to skip the config-loading phase, and use an existing config instead.
// This can be used by tests to plug in a config directly,
// without having to store it on disk somewhere.
func (c *Config) Load(ctx context.Context) (Starter, error) {
	return c, nil
}

var _ Starter = (*Config)(nil)

// Start sets up the configured group of builders.
func (c *Config) Start(ctx context.Context) (builders Builders, errResult error) {
	builders = make(Builders)
	defer func() {
		if errResult == nil {
			return
		}
		// If there is any error, close the builders we may have opened already
		errResult = errors.Join(errResult, builders.Close())
	}()
	for id, conf := range c.Builders {
		b, err := conf.Start(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to start %s: %w", id, err)
		}
		builders[id] = b
	}
	return builders, nil
}

func isNil[T any](v *T) int {
	if v == nil {
		return 0
	} else {
		return 1
	}
}

type BuilderEntry struct {
	ChainID eth.ChainID `yaml:"chainID"`

	L1Builder      *L1Builder      `yaml:"localL1,omitempty"`
	LocalL2Builder *LocalL2Builder `yaml:"localL2,omitempty"`
}

func (b *BuilderEntry) Check() error {
	if b.ChainID == (eth.ChainID{}) {
		return errors.New("cannot build for chain 0")
	}
	count := isNil(b.L1Builder) + isNil(b.LocalL2Builder)
	if count != 1 {
		return fmt.Errorf("entry may only have 1 config, but have %d", count)
	}
	return nil
}

func (b *BuilderEntry) Start(ctx context.Context) (Builder, error) {
	if err := b.Check(); err != nil {
		return nil, err
	}
	if b.L1Builder != nil {
		return b.L1Builder.Start(ctx, b.ChainID)
	}
	if b.LocalL2Builder != nil {
		return b.LocalL2Builder.Start(ctx, b.ChainID)
	}
	return nil, errors.New("unexpected builder config")
}

type SignerEntry struct {
	Endpoint string `yaml:"l2Signer,omitempty"`
}

type RPC endpoint.URL

type L1Builder struct {
	// L1 execution-layer RPC endpoint
	L1EL endpoint.MustRPC `yaml:"l1EL,omitempty"`

	// L1 engine-API RPC endpoint
	L1Engine endpoint.MustRPC `yaml:"l1Engine,omitempty"`
}

func (c *L1Builder) Start(ctx context.Context, chainID eth.ChainID) (Builder, error) {
	// TODO dial RPCs
	return nil, nil
}

type LocalL2Builder struct {
	// L1 execution-layer RPC endpoint
	L1EL endpoint.MustRPC `yaml:"l1EL,omitempty"`

	// L2 execution-layer RPC endpoint
	L2EL endpoint.MustRPC `yaml:"l2EL,omitempty"`
	// L2 consensus-layer RPC endpoint
	L2CL endpoint.MustRPC `yaml:"l2CL,omitempty"`
}

func (c *LocalL2Builder) Start(ctx context.Context, chainID eth.ChainID) (Builder, error) {
	//cl, err := client.NewRPC(ctx, client.WithLazyDial())
	// TODO dial RPCs
	return nil, nil
}
