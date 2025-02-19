package work

import (
	"context"
	"errors"
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
	NewJob(ctx context.Context, id seqtypes.BuildJobID, opts *seqtypes.BuildOpts) (BuildJob, error)
	String() string
	ID() seqtypes.BuilderID
	io.Closer
}

// BuildJob provides access to the building work of a single protocol block.
// This may include extra access, such as inclusion of individual txs or block-building steps.
type BuildJob interface {
	ID() seqtypes.BuildJobID
	Cancel(ctx context.Context) error
	Seal(ctx context.Context) (eth.BlockRef, error)
	String() string
}

// Signer signs a block to be published
type Signer interface {
	String() string
	ID() seqtypes.SignerID
	io.Closer
}

// Committer commits to a (signed) block to become canonical.
// This work is critical: if a block cannot be committed,
// the block is not safe to continue to work with, as it can be replaced by another block.
// E.g.:
// - commit a block to be persisted in the local node.
// - commit a block to an op-conductor service.
type Committer interface {
	String() string
	ID() seqtypes.CommitterID
	io.Closer
}

// Publisher publishes a (signed) block to external actors.
// Publishing may fail.
// E.g. publish the block to node(s) for propagation via P2P.
type Publisher interface {
	String() string
	ID() seqtypes.PublisherID
	io.Closer
}

// Sequencer utilizes Builder, Committer, Signer, Publisher to
// perform all the responsibilities to extend the chain.
// A Sequencer may internally pipeline work,
// but does not expose parallel work like a builder does.
type Sequencer interface {
	String() string
	ID() seqtypes.SequencerID
	io.Closer
}

// Loader loads a configuration, ready to start builders with.
type Loader interface {
	Load(ctx context.Context) (Starter, error)
}

type StartOpts struct {
	Log     log.Logger
	Metrics metrics.Metricer
}

// Starter starts an ensemble from some form of setup.
type Starter interface {
	Start(ctx context.Context, opts *StartOpts) (*Ensemble, error)
}
