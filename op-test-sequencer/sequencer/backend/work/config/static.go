package config

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/builder/l1eng"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/builder/l2eng"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/builder/l2remote"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type Config struct {
	Builders   map[seqtypes.BuilderID]*BuilderEntry     `yaml:"builders"`
	Signers    map[seqtypes.SignerID]*SignerEntry       `yaml:"signers"`
	Committers map[seqtypes.CommitterID]*CommitterEntry `yaml:"committers"`
	Publishers map[seqtypes.PublisherID]*PublisherEntry `yaml:"publishers"`
	Sequencers map[seqtypes.SequencerID]*SequencerEntry `yaml:"sequencers"`
}

var _ work.Loader = (*Config)(nil)

// Load is a short-cut to skip the config-loading phase, and use an existing config instead.
// This can be used by tests to plug in a config directly,
// without having to store it on disk somewhere.
func (c *Config) Load(ctx context.Context) (work.Starter, error) {
	return c, nil
}

var _ work.Starter = (*Config)(nil)

// Start sets up the configured group of builders.
func (c *Config) Start(ctx context.Context, opts *work.StartOpts) (ensemble *work.Ensemble, errResult error) {
	ensemble = new(work.Ensemble)
	// TODO init maps
	defer func() {
		if errResult == nil {
			return
		}
		// If there is any error, close the builders we may have opened already
		errResult = errors.Join(errResult, ensemble.Close())
	}()
	for id, conf := range c.Builders {
		b, err := conf.Start(ctx, id, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to start %s: %w", id, err)
		}
		ensemble.builders[id] = b
	}
	return ensemble, nil
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

	L1Eng    *l1eng.Config    `yaml:"l1Eng,omitempty"`
	L2Eng    *l2eng.Config    `yaml:"l2Eng,omitempty"`
	L2Remote *l2remote.Config `yaml:"l2Remote,omitempty"`
}

func (b *BuilderEntry) Check() error {
	if b.ChainID == (eth.ChainID{}) {
		return errors.New("cannot build for chain 0")
	}
	count := isNil(b.L1Eng) + isNil(b.L2Eng) + isNil(b.L2Remote)
	if count != 1 {
		return fmt.Errorf("entry may only have 1 config, but have %d", count)
	}
	return nil
}

func (b *BuilderEntry) Start(ctx context.Context, id seqtypes.BuilderID, opts *work.StartOpts) (work.Builder, error) {
	if err := b.Check(); err != nil {
		return nil, err
	}
	if b.L1Eng != nil {
		return b.L1Eng.Start(ctx, id, b.ChainID, opts)
	}
	if b.L2Eng != nil {
		return b.L2Eng.Start(ctx, id, b.ChainID, opts)
	}
	if b.L2Remote != nil {
		return b.L2Remote.Start(ctx, id, b.ChainID, opts)
	}
	return nil, errors.New("unexpected builder config")
}

type SignerEntry struct {
	ChainID eth.ChainID `yaml:"chainID"`

	Endpoint string `yaml:"l2Signer,omitempty"`
}

type CommitterEntry struct {
	ChainID eth.ChainID `yaml:"chainID"`
}

type PublisherEntry struct {
	ChainID eth.ChainID `yaml:"chainID"`
}

type SequencerEntry struct {
	ChainID   eth.ChainID           `yaml:"chainID"`
	Builder   seqtypes.BuilderID    `yaml:"builder"`
	Signer    *seqtypes.SignerID    `yaml:"signer,omitempty"`
	Committer *seqtypes.CommitterID `yaml:"committer,omitempty"`
	Publisher *seqtypes.PublisherID `yaml:"publisher,omitempty"`
}
