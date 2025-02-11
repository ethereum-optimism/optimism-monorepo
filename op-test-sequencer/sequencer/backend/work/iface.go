package work

import (
	"context"
	"errors"
	"io"

	"github.com/ethereum/go-ethereum/common"
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

type Block interface {
	ID() eth.BlockID
	String() string
}

type SignedBlock interface {
	Block
	VerifySignature() error
}

// BuildJob provides access to the building work of a single protocol block.
// This may include extra access, such as inclusion of individual txs or block-building steps.
type BuildJob interface {
	ID() seqtypes.BuildJobID
	Cancel(ctx context.Context) error
	Seal(ctx context.Context) (Block, error)
	String() string
}

// Signer signs a block to be published
type Signer interface {
	String() string
	ID() seqtypes.SignerID
	io.Closer
	Sign(ctx context.Context, block Block) (SignedBlock, error)
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
	Commit(ctx context.Context, block SignedBlock) error
}

// Publisher publishes a (signed) block to external actors.
// Publishing may fail.
// E.g. publish the block to node(s) for propagation via P2P.
type Publisher interface {
	String() string
	ID() seqtypes.PublisherID
	io.Closer
	Publish(ctx context.Context, block SignedBlock) error
}

// Sequencer utilizes Builder, Committer, Signer, Publisher to
// perform all the responsibilities to extend the chain.
// A Sequencer may internally pipeline work,
// but does not expose parallel work like a builder does.
type Sequencer interface {
	String() string
	ID() seqtypes.SequencerID

	// Close the sequencer. After closing successfully the sequencer is no longer usable.
	Close() error

	// Open starts a next sequencing slot
	Open(ctx context.Context) error

	// BuildJob identifies the current block-building work.
	// This work may be interacted with through the block-building API.
	BuildJob() (BuildJob, error)

	// Seal seals the current ongoing block-building job.
	// Returns an error if there was no block-building job open.
	Seal(ctx context.Context) error

	// Prebuilt inserts a pre-built block, skipping block-building.
	Prebuilt(ctx context.Context, block Block) error

	// Sign the previously built block.
	// Returns seqtypes.ErrAlreadySigned if already signed.
	// Returns an error if the block could not be signed.
	Sign(ctx context.Context) error

	// Commit the previously signed block, this ensures we can persist it before relying on it as canonical block.
	// Returns seqtypes.ErrAlreadyCommitted if already committed.
	// Returns an error if the block failed to commit.
	Commit(ctx context.Context) error

	// Publish the previously committed block.
	// Publishing is not respected by next steps.
	// For harder guarantees, use Commit to ensure the payload is successfully shared before the next phase.
	// Re-publishing is allowed.
	// Returns an error if none was previously successfully committed.
	// Returns an error if publishing fails.
	Publish(ctx context.Context) error

	// Next continues to the next sequencing slot.
	// If the current slot is unfinished, then it will finish the current slot.
	// If the current slot is fresh, it will be built.
	// An error is returned if any step fails. It is safe to re-attempt with Next if so.
	Next(ctx context.Context) error

	// Start starts automatic sequencing, on top of the given chain head.
	// An error is returned if the head of the chain does not match the provided head,
	// as safety measure to prevent reorgs during sequencer rotations.
	// An seqtypes.ErrSequencerAlreadyActive error is returned if the sequencer has already been started.
	Start(ctx context.Context, head common.Hash) error

	// Stop stops automatic sequencing, and returns the block-hash of the last sequenced block.
	// An seqtypes.ErrSequencerInactive error is returned if the sequencer has already been stopped.
	Stop(ctx context.Context) (last common.Hash, err error)

	// Active returns true if the automatic sequencing (see Start and Stop) is actively running.
	Active() bool

	// TODO: later, to fit old sequencer functionality fully
	//OverrideLeader(ctx context.Context) error
	//ConductorEnabled(ctx context.Context) bool
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
