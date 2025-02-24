package work

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-service/locks"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

var ErrAlreadyExists = errors.New("entry with same ID already exists")

// Ensemble is a group of active services to sequence blocks with.
// Services can only be added to an ensemble,
// the ensemble is meant to run for the full lifetime of the Go service.
type Ensemble struct {
	// builders build unsigned block alternatives
	builders locks.RWMap[seqtypes.BuilderID, Builder]

	// signers sign blocks
	signers locks.RWMap[seqtypes.SignerID, Signer]

	// committers commit to blocks for persistence
	committers locks.RWMap[seqtypes.CommitterID, Committer]

	// publishers publish blocks
	publishers locks.RWMap[seqtypes.PublisherID, Publisher]

	// sequencers perform all block responsibilities
	sequencers locks.RWMap[seqtypes.SequencerID, Sequencer]
}

var _ Loader = (*Ensemble)(nil)

// Load is a short-cut to skip the config phase, and use an existing group of builders.
func (bs *Ensemble) Load(ctx context.Context) (Starter, error) {
	return bs, nil
}

var _ Starter = (*Ensemble)(nil)

// Start is a short-cut to skip the start phase, and use an existing group of builders.
func (bs *Ensemble) Start(ctx context.Context, opts *StartOpts) (*Ensemble, error) {
	return bs, nil
}

func (bs *Ensemble) Close() error {
	// We close all services in reverse order: user-facing first most, then underlying services.
	var result error
	bs.sequencers.Range(func(id seqtypes.SequencerID, v Sequencer) bool {
		if err := v.Close(); err != nil {
			result = errors.Join(result, fmt.Errorf("failed to close sequencer %q: %w", id, err))
		}
		return true
	})
	bs.publishers.Range(func(id seqtypes.PublisherID, v Publisher) bool {
		if err := v.Close(); err != nil {
			result = errors.Join(result, fmt.Errorf("failed to close publisher %q: %w", id, err))
		}
		return true
	})
	bs.committers.Range(func(id seqtypes.CommitterID, v Committer) bool {
		if err := v.Close(); err != nil {
			result = errors.Join(result, fmt.Errorf("failed to close committer %q: %w", id, err))
		}
		return true
	})
	bs.signers.Range(func(id seqtypes.SignerID, v Signer) bool {
		if err := v.Close(); err != nil {
			result = errors.Join(result, fmt.Errorf("failed to close signer %q: %w", id, err))
		}
		return true
	})
	bs.builders.Range(func(id seqtypes.BuilderID, v Builder) bool {
		if err := v.Close(); err != nil {
			result = errors.Join(result, fmt.Errorf("failed to close builder %q: %w", id, err))
		}
		return true
	})
	return result
}

type Collection interface {
	Builder(id seqtypes.BuilderID) Builder
	Signer(id seqtypes.SignerID) Signer
	Committer(id seqtypes.CommitterID) Committer
	Publisher(id seqtypes.PublisherID) Publisher
	Sequencer(id seqtypes.SequencerID) Sequencer
}

var _ Collection = (*Ensemble)(nil)

// Builder gets a builder. Nil is returned if the ID is unknown.
func (bs *Ensemble) Builder(id seqtypes.BuilderID) Builder {
	s, _ := bs.builders.Get(id)
	return s
}

// Signer gets a signer. Nil is returned if the ID is unknown.
func (bs *Ensemble) Signer(id seqtypes.SignerID) Signer {
	s, _ := bs.signers.Get(id)
	return s
}

// Committer gets a committer. Nil is returned if the ID is unknown.
func (bs *Ensemble) Committer(id seqtypes.CommitterID) Committer {
	s, _ := bs.committers.Get(id)
	return s
}

// Publisher gets a publisher. Nil is returned if the ID is unknown.
func (bs *Ensemble) Publisher(id seqtypes.PublisherID) Publisher {
	s, _ := bs.publishers.Get(id)
	return s
}

// Sequencer gets a sequencer. Nil is returned if the ID is unknown.
func (bs *Ensemble) Sequencer(id seqtypes.SequencerID) Sequencer {
	s, _ := bs.sequencers.Get(id)
	return s
}

// AddBuilder adds a builder. Errors if the builder already exists.
func (bs *Ensemble) AddBuilder(v Builder) error {
	if !bs.builders.SetIfMissing(v.ID(), v) {
		return ErrAlreadyExists
	}
	return nil
}

// AddSigner adds a signer. Errors if the signer already exists.
func (bs *Ensemble) AddSigner(v Signer) error {
	if !bs.signers.SetIfMissing(v.ID(), v) {
		return ErrAlreadyExists
	}
	return nil
}

// AddCommitter adds a committer. Errors if the committer already exists.
func (bs *Ensemble) AddCommitter(v Committer) error {
	if !bs.committers.SetIfMissing(v.ID(), v) {
		return ErrAlreadyExists
	}
	return nil
}

// AddPublisher adds a publisher. Errors if the publisher already exists.
func (bs *Ensemble) AddPublisher(v Publisher) error {
	if !bs.publishers.SetIfMissing(v.ID(), v) {
		return ErrAlreadyExists
	}
	return nil
}

// AddSequencer adds a sequencer. Errors if the sequencer already exists.
func (bs *Ensemble) AddSequencer(v Sequencer) error {
	if !bs.sequencers.SetIfMissing(v.ID(), v) {
		return ErrAlreadyExists
	}
	return nil
}

func (bs *Ensemble) Builders() []seqtypes.BuilderID {
	return bs.builders.Keys()
}

func (bs *Ensemble) Signers() []seqtypes.SignerID {
	return bs.signers.Keys()
}

func (bs *Ensemble) Committers() []seqtypes.CommitterID {
	return bs.committers.Keys()
}

func (bs *Ensemble) Publishers() []seqtypes.PublisherID {
	return bs.publishers.Keys()
}

func (bs *Ensemble) Sequencers() []seqtypes.SequencerID {
	return bs.sequencers.Keys()
}
