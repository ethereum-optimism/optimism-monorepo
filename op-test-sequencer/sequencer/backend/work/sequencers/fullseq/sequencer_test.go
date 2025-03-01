package fullseq

import (
	"context"
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/metrics"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type mockBlock struct{}

func (m *mockBlock) ID() eth.BlockID {
	return eth.BlockID{Number: 123, Hash: common.Hash{0xff}}
}

func (m *mockBlock) String() string {
	return "mock block"
}

var _ work.Block = (*mockBlock)(nil)

type mockSignedBlock struct {
	bl work.Block
}

func (m *mockSignedBlock) ID() eth.BlockID {
	return m.bl.ID()
}

func (m *mockSignedBlock) String() string {
	return "mock signed block"
}

func (m *mockSignedBlock) VerifySignature() error {
	return nil
}

var _ work.SignedBlock = (*mockSignedBlock)(nil)

type mockBuildJob struct {
	id  seqtypes.BuildJobID
	bl  work.Block
	err error
}

func (m *mockBuildJob) ID() seqtypes.BuildJobID {
	return m.id
}

func (m *mockBuildJob) Cancel(ctx context.Context) error {
	return nil
}

func (m *mockBuildJob) Seal(ctx context.Context) (work.Block, error) {
	return m.bl, m.err
}

func (m *mockBuildJob) String() string {
	return "mock build job"
}

var _ work.BuildJob = (*mockBuildJob)(nil)

type mockBuilder struct {
	id     seqtypes.BuilderID
	closed bool
	job    func() (work.BuildJob, error)
}

func (m *mockBuilder) NewJob(ctx context.Context, id seqtypes.BuildJobID, opts *seqtypes.BuildOpts) (work.BuildJob, error) {
	return m.job()
}

func (m *mockBuilder) String() string {
	return "mock-builder-" + m.id.String()
}

func (m *mockBuilder) ID() seqtypes.BuilderID {
	return m.id
}

func (m *mockBuilder) Close() error {
	m.closed = true
	return nil
}

var _ work.Builder = (*mockBuilder)(nil)

type mockSigner struct {
	id     seqtypes.SignerID
	closed bool
	err    error
}

func (m *mockSigner) String() string {
	return "mock-signer-" + m.id.String()
}

func (m *mockSigner) ID() seqtypes.SignerID {
	return m.id
}

func (m *mockSigner) Close() error {
	m.closed = true
	return nil
}

func (m *mockSigner) Sign(ctx context.Context, block work.Block) (work.SignedBlock, error) {
	return &mockSignedBlock{bl: block}, m.err
}

var _ work.Signer = (*mockSigner)(nil)

type mockCommitter struct {
	id     seqtypes.CommitterID
	closed bool
	err    error
}

func (m *mockCommitter) String() string {
	return "mock-committer-" + m.id.String()
}

func (m *mockCommitter) ID() seqtypes.CommitterID {
	return m.id
}

func (m *mockCommitter) Close() error {
	m.closed = true
	return nil
}

func (m *mockCommitter) Commit(ctx context.Context, block work.SignedBlock) error {
	return m.err
}

var _ work.Committer = (*mockCommitter)(nil)

type mockPublisher struct {
	id     seqtypes.PublisherID
	closed bool
	err    error
}

func (m *mockPublisher) String() string {
	return "mock-publisher-" + m.id.String()
}

func (m *mockPublisher) ID() seqtypes.PublisherID {
	return m.id
}

func (m *mockPublisher) Close() error {
	m.closed = true
	return nil
}

func (m *mockPublisher) Publish(ctx context.Context, block work.SignedBlock) error {
	return m.err
}

var _ work.Publisher = (*mockPublisher)(nil)

func TestSequencer(t *testing.T) {
	logger := testlog.Logger(t, log.LevelDebug)
	seqID := seqtypes.SequencerID("seq")
	builder := &mockBuilder{id: seqtypes.BuilderID("builder")}
	signer := &mockSigner{id: seqtypes.SignerID("signer")}
	committer := &mockCommitter{id: seqtypes.CommitterID("committer")}
	publisher := &mockPublisher{id: seqtypes.PublisherID("publisher")}
	seq := &Sequencer{
		id:        seqID,
		chainID:   eth.ChainIDFromUInt64(1),
		log:       logger,
		m:         &metrics.NoopMetrics{},
		builder:   builder,
		signer:    signer,
		committer: committer,
		publisher: publisher,
	}
	require.Contains(t, seq.String(), "sequencer")
	ctx := context.Background()

	resetBuildJob := func() {
		job := &mockBuildJob{
			id:  seqtypes.RandomJobID(),
			bl:  &mockBlock{},
			err: nil,
		}
		builder.job = func() (work.BuildJob, error) {
			return job, nil
		}
	}
	t.Run("no action without the pre-requisite action", func(t *testing.T) {
		seq.reset()
		require.ErrorIs(t, seq.Seal(ctx), seqtypes.ErrUnknownJob)
		require.ErrorIs(t, seq.Sign(ctx), seqtypes.ErrNotSealed)
		require.ErrorIs(t, seq.Commit(ctx), seqtypes.ErrUnsigned)
		require.ErrorIs(t, seq.Publish(ctx), seqtypes.ErrUncommitted)
	})

	t.Run("do a full routine", func(t *testing.T) {
		seq.reset()
		resetBuildJob()
		require.Nil(t, seq.BuildJob(), "no job yet")
		require.NoError(t, seq.Open(ctx))
		require.ErrorIs(t, seq.Open(ctx), seqtypes.ErrConflictingJob)
		require.NotNil(t, seq.BuildJob(), "job is opened")
		require.NoError(t, seq.Seal(ctx))
		require.ErrorIs(t, seq.Seal(ctx), seqtypes.ErrAlreadySealed)
		require.NoError(t, seq.Sign(ctx))
		require.ErrorIs(t, seq.Sign(ctx), seqtypes.ErrAlreadySigned)
		require.NoError(t, seq.Commit(ctx))
		require.ErrorIs(t, seq.Commit(ctx), seqtypes.ErrAlreadyCommitted)
		require.NoError(t, seq.Publish(ctx))
		require.NoError(t, seq.Publish(ctx), "re-publishing blocks is allowed")
	})

	t.Run("continue from scratch", func(t *testing.T) {
		seq.reset()
		resetBuildJob()
		require.NoError(t, seq.Next(ctx))
	})

	t.Run("continue from open block", func(t *testing.T) {
		seq.reset()
		resetBuildJob()
		require.NoError(t, seq.Open(ctx))
		require.NoError(t, seq.Next(ctx))
	})

	t.Run("continue from sealed block", func(t *testing.T) {
		seq.reset()
		resetBuildJob()
		require.NoError(t, seq.Open(ctx))
		require.NoError(t, seq.Seal(ctx))
		require.NoError(t, seq.Next(ctx))
	})

	t.Run("continue from signed block", func(t *testing.T) {
		seq.reset()
		resetBuildJob()
		require.NoError(t, seq.Open(ctx))
		require.NoError(t, seq.Seal(ctx))
		require.NoError(t, seq.Sign(ctx))
		require.NoError(t, seq.Next(ctx))
	})

	t.Run("continue from committed block", func(t *testing.T) {
		seq.reset()
		resetBuildJob()
		require.NoError(t, seq.Open(ctx))
		require.NoError(t, seq.Seal(ctx))
		require.NoError(t, seq.Sign(ctx))
		require.NoError(t, seq.Commit(ctx))
		require.NoError(t, seq.Next(ctx))
	})

	t.Run("continue from published block", func(t *testing.T) {
		seq.reset()
		resetBuildJob()
		require.NoError(t, seq.Open(ctx))
		require.NoError(t, seq.Seal(ctx))
		require.NoError(t, seq.Sign(ctx))
		require.NoError(t, seq.Commit(ctx))
		require.NoError(t, seq.Publish(ctx))
		require.NoError(t, seq.Next(ctx))
	})

	t.Run("continue from prebuilt block", func(t *testing.T) {
		seq.reset()
		resetBuildJob()
		bl := &mockBlock{}
		require.NoError(t, seq.Prebuilt(ctx, bl))
		require.NoError(t, seq.Sign(ctx))
		require.Equal(t, bl, seq.signed.(*mockSignedBlock).bl)
		require.NoError(t, seq.Commit(ctx))
		require.NoError(t, seq.Next(ctx))
	})

	t.Run("no prebuilt after job open", func(t *testing.T) {
		seq.reset()
		resetBuildJob()
		bl := &mockBlock{}
		require.NoError(t, seq.Open(ctx))
		require.ErrorIs(t, seq.Prebuilt(ctx, bl), seqtypes.ErrConflictingJob)
	})

	t.Run("no prebuilt after job", func(t *testing.T) {
		seq.reset()
		resetBuildJob()
		bl := &mockBlock{}
		require.NoError(t, seq.Open(ctx))
		require.NoError(t, seq.Seal(ctx))
		require.ErrorIs(t, seq.Prebuilt(ctx, bl), seqtypes.ErrConflictingJob)
	})

	t.Run("no duplicate prebuilt", func(t *testing.T) {
		seq.reset()
		resetBuildJob()
		bl := &mockBlock{}
		require.NoError(t, seq.Prebuilt(ctx, bl))
		require.ErrorIs(t, seq.Prebuilt(ctx, bl), seqtypes.ErrAlreadySealed)
	})

	t.Run("fail to open", func(t *testing.T) {
		seq.reset()
		testErr := errors.New("test open err")
		builder.job = func() (work.BuildJob, error) {
			return nil, testErr
		}
		require.ErrorIs(t, seq.Open(ctx), testErr)
		require.ErrorIs(t, seq.Next(ctx), testErr)
	})

	t.Run("fail to seal", func(t *testing.T) {
		seq.reset()
		testErr := errors.New("test seal err")
		job := &mockBuildJob{
			id:  seqtypes.RandomJobID(),
			bl:  nil,
			err: testErr,
		}
		builder.job = func() (work.BuildJob, error) {
			return job, nil
		}
		require.NoError(t, seq.Open(ctx))
		require.ErrorIs(t, seq.Seal(ctx), testErr)
		require.ErrorIs(t, seq.Next(ctx), testErr)
	})

	t.Run("fail to sign", func(t *testing.T) {
		seq.reset()
		resetBuildJob()
		testErr := errors.New("test sign err")
		signer.err = testErr
		require.NoError(t, seq.Open(ctx))
		require.NoError(t, seq.Seal(ctx))
		require.ErrorIs(t, seq.Sign(ctx), testErr)
		require.ErrorIs(t, seq.Next(ctx), testErr)
		signer.err = nil
	})

	t.Run("fail to commit", func(t *testing.T) {
		seq.reset()
		resetBuildJob()
		testErr := errors.New("test commit err")
		committer.err = testErr
		require.NoError(t, seq.Open(ctx))
		require.NoError(t, seq.Seal(ctx))
		require.NoError(t, seq.Sign(ctx))
		require.ErrorIs(t, seq.Commit(ctx), testErr)
		require.ErrorIs(t, seq.Next(ctx), testErr)
		committer.err = nil
	})

	t.Run("fail to publish", func(t *testing.T) {
		seq.reset()
		resetBuildJob()
		testErr := errors.New("test publish err")
		publisher.err = testErr
		require.NoError(t, seq.Open(ctx))
		require.NoError(t, seq.Seal(ctx))
		require.NoError(t, seq.Sign(ctx))
		require.NoError(t, seq.Commit(ctx))
		require.ErrorIs(t, seq.Publish(ctx), testErr)
		require.ErrorIs(t, seq.Next(ctx), testErr)
		publisher.err = nil
	})

	t.Run("sequencer start/stop", func(t *testing.T) {
		seq.reset()
		resetBuildJob()
		require.False(t, seq.Active())
		// Start/stop not supported yet
		require.ErrorIs(t, seq.Start(ctx, common.Hash{}), seqtypes.ErrNotImplemented)
		seq.active = true
		_, err := seq.Stop(ctx)
		require.ErrorIs(t, err, seqtypes.ErrNotImplemented)
		// inactive again
		require.False(t, seq.Active())
	})

	t.Run("no duplicate start", func(t *testing.T) {
		seq.reset()
		resetBuildJob()
		seq.active = true
		require.ErrorIs(t, seq.Start(ctx, common.Hash{}), seqtypes.ErrSequencerAlreadyActive)
		seq.active = false
	})

	t.Run("no duplicate stop", func(t *testing.T) {
		seq.reset()
		resetBuildJob()
		seq.active = false
		_, err := seq.Stop(ctx)
		require.ErrorIs(t, err, seqtypes.ErrSequencerInactive)
	})

	require.NoError(t, seq.Close())
	// other services are independent, not closed as part of the sequencer,
	// but closed as part of the total ensemble.
	require.False(t, builder.closed)
	require.False(t, signer.closed)
	require.False(t, committer.closed)
	require.False(t, publisher.closed)

}
