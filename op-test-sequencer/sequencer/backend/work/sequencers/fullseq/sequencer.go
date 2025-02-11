package fullseq

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/metrics"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type Sequencer struct {
	id seqtypes.SequencerID

	chainID eth.ChainID

	log log.Logger
	m   metrics.Metricer

	stateMu sync.RWMutex

	currentJob work.BuildJob
	unsigned   work.Block
	signed     work.SignedBlock
	committed  bool
	published  bool

	// active is true when it's currently running through block-building automatically
	active bool

	builder   work.Builder
	signer    work.Signer
	committer work.Committer
	publisher work.Publisher
}

var _ work.Sequencer = (*Sequencer)(nil)

func (s *Sequencer) String() string {
	return "sequencer-" + s.id.String()
}

func (s *Sequencer) ID() seqtypes.SequencerID {
	return s.id
}

func (s *Sequencer) Close() error {
	return nil
}

func (s *Sequencer) Open(ctx context.Context) error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.unsigned != nil {
		return seqtypes.ErrAlreadySealed
	}
	if s.currentJob != nil {
		return seqtypes.ErrConflictingJob
	}
	id := seqtypes.RandomJobID()

	opts := &seqtypes.BuildOpts{
		Parent:   common.Hash{}, // TODO
		L1Origin: nil,
	}

	job, err := s.builder.NewJob(ctx, id, opts)
	if err != nil {
		return fmt.Errorf("failed to start new build job: %w", err)
	}
	s.currentJob = job
	return nil
}

func (s *Sequencer) BuildJob() work.BuildJob {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.currentJob
}

func (s *Sequencer) Seal(ctx context.Context) error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.unsigned != nil {
		return seqtypes.ErrAlreadySealed
	}
	if s.currentJob == nil {
		return seqtypes.ErrUnknownJob
	}
	block, err := s.currentJob.Seal(ctx)
	if err != nil {
		return fmt.Errorf("failed to seal block: %w", err)
	}
	s.unsigned = block
	return nil
}

func (s *Sequencer) Prebuilt(ctx context.Context, block work.Block) error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.currentJob != nil {
		return seqtypes.ErrConflictingJob
	}
	if s.unsigned != nil {
		return seqtypes.ErrAlreadySealed
	}
	s.unsigned = block
	return nil
}

func (s *Sequencer) Sign(ctx context.Context) error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.signed != nil {
		return seqtypes.ErrAlreadySigned
	}
	if s.unsigned == nil {
		return seqtypes.ErrNotSealed
	}
	result, err := s.signer.Sign(ctx, s.unsigned)
	if err != nil {
		return err
	}
	s.signed = result
	return nil
}

func (s *Sequencer) Commit(ctx context.Context) error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.committed {
		return seqtypes.ErrAlreadyCommitted
	}
	if s.signed == nil {
		return seqtypes.ErrUnsigned
	}
	if err := s.committer.Commit(ctx, s.signed); err != nil {
		return err
	}
	s.committed = true
	return nil
}

func (s *Sequencer) Publish(ctx context.Context) error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	// re-publishing is allowed
	return s.publish(ctx)
}

var errAlreadyPublished = errors.New("block alreadyb published")

func (s *Sequencer) publishMaybe(ctx context.Context) error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.published {
		// Re-publishing is allowed, but not in the Next() routine
		// which skips already-completed actions.
		return errAlreadyPublished
	}
	return s.publish(ctx)
}

func (s *Sequencer) publish(ctx context.Context) error {
	if !s.committed {
		return seqtypes.ErrUncommitted
	}
	if err := s.publisher.Publish(ctx, s.signed); err != nil {
		return err
	}
	s.published = true
	return nil
}

func (s *Sequencer) Next(ctx context.Context) error {
	if err := s.Open(ctx); err != nil && !(errors.Is(err, seqtypes.ErrAlreadySealed) ||
		errors.Is(err, seqtypes.ErrConflictingJob)) { // forced-in blocks don't count as job
		return fmt.Errorf("block-open failed: %w", err)
	}
	if err := s.Seal(ctx); err != nil && !errors.Is(err, seqtypes.ErrAlreadySealed) {
		return fmt.Errorf("block-seal failed: %w", err)
	}
	if err := s.Sign(ctx); err != nil && !errors.Is(err, seqtypes.ErrAlreadySigned) {
		return fmt.Errorf("block-sign failed: %w", err)
	}
	if err := s.Commit(ctx); err != nil && !errors.Is(err, seqtypes.ErrAlreadyCommitted) {
		return fmt.Errorf("block-commit failed: %w", err)
	}
	if err := s.publishMaybe(ctx); err != nil && !errors.Is(err, errAlreadyPublished) {
		return fmt.Errorf("block-publish failed: %w", err)
	}
	s.lockingReset()
	return nil
}

func (s *Sequencer) lockingReset() {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	s.reset()
}

func (s *Sequencer) reset() {
	s.currentJob = nil
	s.unsigned = nil
	s.signed = nil
	s.committed = false
	s.published = false
}

func (s *Sequencer) Start(ctx context.Context, head common.Hash) error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	if s.active {
		return seqtypes.ErrSequencerAlreadyActive
	}
	return s.forceStart()
}

func (s *Sequencer) forceStart() error {
	// TODO start schedule
	s.reset()

	return seqtypes.ErrNotImplemented
}

func (s *Sequencer) Stop(ctx context.Context) (hash common.Hash, err error) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	if !s.active {
		return common.Hash{}, seqtypes.ErrSequencerInactive
	}

	s.active = false
	return common.Hash{}, seqtypes.ErrNotImplemented
	/*
		// TODO stop schedule
		var last common.Hash
		s.reset()
		return last, nil
	*/
}

func (s *Sequencer) Active() bool {
	s.stateMu.RLock()
	active := s.active
	s.stateMu.RUnlock()
	return active
}
