package builder

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/metrics"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

// ErrNoRegistry is returned by Builder.NewJob when a Registry has
// not yet been attached to the Builder with Builder.Attach.
var ErrNoRegistry = errors.New("no registry attached")

// Builder provides access to block-building work.
// Different implementations are available, e.g. for local or remote block-building.
type Builder interface {
	Attach(registry Registry)
	NewJob(ctx context.Context, id seqtypes.JobID, opts *seqtypes.BuildOpts) (BuildJob, error)
	io.Closer
	String() string
	ID() seqtypes.BuilderID
}

// BuildJob provides access to the building work of a single protocol block.
// This may include extra access, such as inclusion of individual txs or block-building steps.
type BuildJob interface {
	ID() seqtypes.JobID
	Cancel(ctx context.Context) error
	Seal(ctx context.Context) (eth.BlockRef, error)
	String() string
}

// Registry is the interface provided to the builders,
// to cleanup their block-building jobs with.
type Registry interface {
	UnregisterJob(id seqtypes.JobID)
}

// Loader loads a configuration, ready to start builders with.
type Loader interface {
	Load(ctx context.Context) (Starter, error)
}

type StartOpts struct {
	Log     log.Logger
	Metrics metrics.Metricer
}

// Starter starts a group of builders from some form of setup.
type Starter interface {
	Start(ctx context.Context, opts *StartOpts) (Builders, error)
}

// Builders represents a group of active builder implementations.
type Builders map[seqtypes.BuilderID]Builder

var _ Loader = Builders(nil)

// Load is a short-cut to skip the config phase, and use an existing group of Builders.
func (bs Builders) Load(ctx context.Context) (Starter, error) {
	return bs, nil
}

var _ Starter = Builders(nil)

// Start is a short-cut to skip the start phase, and use an existing group of Builders.
func (bs Builders) Start(ctx context.Context, opts *StartOpts) (Builders, error) {
	return bs, nil
}

func (bs Builders) Close() error {
	var result error
	for id, b := range bs {
		if err := b.Close(); err != nil {
			result = errors.Join(result, fmt.Errorf("failed to close builder %q: %w", id, err))
		}
	}
	return result
}
