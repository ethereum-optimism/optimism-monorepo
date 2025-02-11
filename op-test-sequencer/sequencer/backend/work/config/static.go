package config

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type Ensemble struct {
	// Endpoints is a no-op list of endpoints
	// to prevent repetitive endpoints in a yaml.
	// This list can all be declared at the top, and items can be referenced with yaml refs.
	Endpoints []string `yaml:"endpoints"`

	Builders   map[seqtypes.BuilderID]*BuilderEntry     `yaml:"builders"`
	Signers    map[seqtypes.SignerID]*SignerEntry       `yaml:"signers"`
	Committers map[seqtypes.CommitterID]*CommitterEntry `yaml:"committers"`
	Publishers map[seqtypes.PublisherID]*PublisherEntry `yaml:"publishers"`
	Sequencers map[seqtypes.SequencerID]*SequencerEntry `yaml:"sequencers"`
}

var _ work.Loader = (*Ensemble)(nil)

// Load is a short-cut to skip the config-loading phase, and use an existing config instead.
// This can be used by tests to plug in a config directly,
// without having to store it on disk somewhere.
func (c *Ensemble) Load(ctx context.Context) (work.Starter, error) {
	return c, nil
}

var _ work.Starter = (*Ensemble)(nil)

// Start sets up the configured group of builders.
func (c *Ensemble) Start(ctx context.Context, opts *work.StartOpts) (ensemble *work.Ensemble, errResult error) {
	ensemble = new(work.Ensemble)
	defer func() {
		if errResult == nil {
			return
		}
		// If there is any error, close the builders we may have opened already
		errResult = errors.Join(errResult, ensemble.Close())
	}()
	if err := startAndAdd(ctx, c.Builders, ensemble, opts, ensemble.AddBuilder); err != nil {
		return nil, fmt.Errorf("failed to start builders: %w", err)
	}
	if err := startAndAdd(ctx, c.Signers, ensemble, opts, ensemble.AddSigner); err != nil {
		return nil, fmt.Errorf("failed to start signers: %w", err)
	}
	if err := startAndAdd(ctx, c.Committers, ensemble, opts, ensemble.AddCommitter); err != nil {
		return nil, fmt.Errorf("failed to start committers: %w", err)
	}
	if err := startAndAdd(ctx, c.Publishers, ensemble, opts, ensemble.AddPublisher); err != nil {
		return nil, fmt.Errorf("failed to start publishers: %w", err)
	}
	if err := startAndAdd(ctx, c.Sequencers, ensemble, opts, ensemble.AddSequencer); err != nil {
		return nil, fmt.Errorf("failed to start sequencers: %w", err)
	}
	return ensemble, nil
}

type valueIface[K comparable, R any] interface {
	Start(ctx context.Context, id K, collection work.Collection, opts *work.StartOpts) (result R, err error)
}

type keyIface interface {
	comparable
	String() string
}

// startAndAdd is a util function to start entities from a configuration map, and add them to the ensemble.
func startAndAdd[K keyIface, R any, E valueIface[K, R]](
	ctx context.Context,
	entries map[K]E,
	collection work.Collection,
	opts *work.StartOpts,
	addFn func(v R) error) error {
	for id, conf := range entries {
		v, err := conf.Start(ctx, id, collection, opts)
		if err != nil {
			return fmt.Errorf("failed to start %q: %w", id, err)
		}
		if err := addFn(v); err != nil {
			return fmt.Errorf("failed to add %q: %w", id, err)
		}
	}
	return nil
}
