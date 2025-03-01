package frontend

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type mockBlock struct {
	From    seqtypes.BuildJobID `json:"from"`
	BlockID eth.BlockID
}

func (m *mockBlock) ID() eth.BlockID {
	return m.BlockID
}

func (m *mockBlock) String() string {
	return m.From.String() + "-" + m.BlockID.String()
}

var _ work.Block = (*mockBlock)(nil)

type mockBuildJob struct {
	id       seqtypes.BuildJobID
	canceled bool
	sealed   bool
	result   work.Block
	err      error
}

func (m *mockBuildJob) ID() seqtypes.BuildJobID {
	return m.id
}

func (m *mockBuildJob) Cancel(ctx context.Context) error {
	m.canceled = true
	return m.err
}

func (m *mockBuildJob) Seal(ctx context.Context) (work.Block, error) {
	m.sealed = true
	return m.result, m.err
}

func (m *mockBuildJob) String() string {
	return "mock-build-job-" + m.id.String()
}

var _ work.BuildJob = (*mockBuildJob)(nil)

type mockBuildBackend struct {
	jobs map[seqtypes.BuildJobID]*mockBuildJob
}

const (
	testBuilderA = seqtypes.BuilderID("builder-a")
	testBuilderB = seqtypes.BuilderID("builder-b")
)

func (m *mockBuildBackend) CreateJob(ctx context.Context, id seqtypes.BuilderID, opts *seqtypes.BuildOpts) (work.BuildJob, error) {
	if id == testBuilderB {
		return nil, seqtypes.ErrUnknownBuilder
	}
	jobID := seqtypes.RandomJobID()
	job := &mockBuildJob{
		id:       jobID,
		canceled: false,
		sealed:   false,
		result: &mockBlock{
			From:    jobID,
			BlockID: eth.BlockID{Number: 123, Hash: crypto.Keccak256Hash([]byte(jobID))},
		},
	}
	m.jobs[jobID] = job
	return job, nil
}

func (m *mockBuildBackend) GetJob(id seqtypes.BuildJobID) work.BuildJob {
	job, ok := m.jobs[id]
	if !ok {
		return nil
	}
	return job
}

var _ BuildBackend = (*mockBuildBackend)(nil)

func TestBuildFrontend(t *testing.T) {
	backend := &mockBuildBackend{
		jobs: make(map[seqtypes.BuildJobID]*mockBuildJob),
	}
	front := &BuildFrontend{Backend: backend}
	ctx := context.Background()

	t.Run("unknown builder", func(t *testing.T) {
		_, err := front.Open(ctx, testBuilderB, nil)
		require.ErrorIs(t, err, seqtypes.ErrUnknownBuilder)
	})

	t.Run("non-existent jobs", func(t *testing.T) {
		err := front.Cancel(ctx, seqtypes.RandomJobID())
		require.ErrorIs(t, err, seqtypes.ErrUnknownJob)

		_, err = front.Seal(ctx, seqtypes.RandomJobID())
		require.ErrorIs(t, err, seqtypes.ErrUnknownJob)
	})

	t.Run("seal", func(t *testing.T) {
		jobID, err := front.Open(ctx, testBuilderA, nil)
		require.NoError(t, err)

		require.False(t, backend.jobs[jobID].sealed)

		block, err := front.Seal(ctx, jobID)
		require.NoError(t, err)
		require.Equal(t, uint64(123), block.ID().Number)
		require.Equal(t, crypto.Keccak256Hash([]byte(jobID)), block.ID().Hash)

		require.True(t, backend.jobs[jobID].sealed)
	})

	t.Run("seal error", func(t *testing.T) {
		jobID, err := front.Open(ctx, testBuilderA, nil)
		require.NoError(t, err)
		require.False(t, backend.jobs[jobID].sealed)
		backend.jobs[jobID].err = seqtypes.ErrAlreadySealed
		_, err = front.Seal(ctx, jobID)
		require.ErrorIs(t, err, seqtypes.ErrAlreadySealed)
	})

	t.Run("cancel", func(t *testing.T) {
		jobID, err := front.Open(ctx, testBuilderA, nil)
		require.NoError(t, err)
		require.False(t, backend.jobs[jobID].canceled)
		err = front.Cancel(ctx, jobID)
		require.NoError(t, err)
		require.True(t, backend.jobs[jobID].canceled)
	})

	t.Run("cancel error", func(t *testing.T) {
		jobID, err := front.Open(ctx, testBuilderA, nil)
		require.NoError(t, err)
		require.False(t, backend.jobs[jobID].canceled)
		backend.jobs[jobID].err = seqtypes.ErrAlreadySealed
		err = front.Cancel(ctx, jobID)
		require.ErrorIs(t, err, seqtypes.ErrAlreadySealed)
	})
}
