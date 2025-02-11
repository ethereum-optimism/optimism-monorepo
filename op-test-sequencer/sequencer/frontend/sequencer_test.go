package frontend

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type advancedBuildJob struct {
	mockBuildJob
	txs []hexutil.Bytes
}

var _ work.BuildJob = (*advancedBuildJob)(nil)
var _ IncludeTxSupport = (*advancedBuildJob)(nil)

func (a *advancedBuildJob) IncludeTx(ctx context.Context, tx hexutil.Bytes) error {
	a.txs = append(a.txs, tx)
	return nil
}

type mockSequencer struct {
	id     seqtypes.SequencerID
	job    work.BuildJob
	err    error
	action string
}

func (m *mockSequencer) String() string {
	return "test"
}

func (m *mockSequencer) ID() seqtypes.SequencerID {
	return m.id
}

func (m *mockSequencer) Close() error {
	return m.err
}

func (m *mockSequencer) Open(ctx context.Context) error {
	m.action = "open"
	return m.err
}

func (m *mockSequencer) BuildJob() work.BuildJob {
	return m.job
}

func (m *mockSequencer) Seal(ctx context.Context) error {
	m.action = "seal"
	return m.err
}

func (m *mockSequencer) Prebuilt(ctx context.Context, block work.Block) error {
	m.action = "prebuilt"
	return m.err
}

func (m *mockSequencer) Sign(ctx context.Context) error {
	m.action = "sign"
	return m.err
}

func (m *mockSequencer) Commit(ctx context.Context) error {
	m.action = "commit"
	return m.err
}

func (m *mockSequencer) Publish(ctx context.Context) error {
	m.action = "publish"
	return m.err
}

func (m *mockSequencer) Next(ctx context.Context) error {
	m.action = "next"
	return m.err
}

func (m *mockSequencer) Start(ctx context.Context, head common.Hash) error {
	m.action = "start"
	return m.err
}

func (m *mockSequencer) Stop(ctx context.Context) (last common.Hash, err error) {
	m.action = "stop"
	return common.Hash{0: 42}, m.err
}

func (m *mockSequencer) Active() bool {
	m.action = "active"
	return false
}

var _ work.Sequencer = (*mockSequencer)(nil)

func TestSequencer(t *testing.T) {
	seqId := seqtypes.SequencerID("foobar")
	seq := &mockSequencer{id: seqId}
	front := &SequencerFrontend{Sequencer: seq}

	ctx := context.Background()

	t.Run("unknown job", func(t *testing.T) {
		_, err := front.BuildJob()
		seq.err = seqtypes.ErrUnknownJob
		require.ErrorIs(t, err, seqtypes.ErrUnknownJob)
		seq.err = nil
	})
	t.Run("include tx", func(t *testing.T) {
		dest := &advancedBuildJob{
			mockBuildJob: mockBuildJob{id: seqtypes.RandomJobID()},
			txs:          nil,
		}
		seq.job = dest
		jobId, err := front.BuildJob()
		require.NoError(t, err)
		require.Equal(t, seq.job.ID(), jobId)
		require.NoError(t, front.IncludeTx(ctx, []byte("test")))
		require.NotEmpty(t, dest.txs)
		seq.job = nil
		require.ErrorIs(t, front.IncludeTx(ctx, []byte("abc")), seqtypes.ErrUnknownJob)
		seq.job = &mockBuildJob{}
		require.ErrorContains(t, front.IncludeTx(ctx, []byte("123")), "not supported")
		seq.job = nil
	})
	t.Run("step by step", func(t *testing.T) {
		seq.action = ""
		require.NoError(t, front.Open(ctx))
		require.Equal(t, "open", seq.action)
		require.NoError(t, front.Seal(ctx))
		require.Equal(t, "seal", seq.action)
		require.NoError(t, front.Sign(ctx))
		require.Equal(t, "sign", seq.action)
		require.NoError(t, front.Commit(ctx))
		require.Equal(t, "commit", seq.action)
		require.NoError(t, front.Publish(ctx))
		require.Equal(t, "publish", seq.action)
		require.NoError(t, front.Next(ctx))
		require.Equal(t, "next", seq.action)
		require.NoError(t, front.PrebuiltEnvelope(ctx, nil))
		require.Equal(t, "prebuilt", seq.action)
		seq.action = ""
	})
	t.Run("start stop", func(t *testing.T) {
		seq.err = seqtypes.ErrSequencerAlreadyActive
		require.ErrorIs(t, front.Start(ctx, common.Hash{0: 123}), seqtypes.ErrSequencerAlreadyActive)
		seq.err = nil
		require.NoError(t, front.Start(ctx, common.Hash{0: 123}))
		out, err := front.Stop(ctx)
		require.NoError(t, err)
		require.Equal(t, common.Hash{0: 42}, out)
		seq.err = seqtypes.ErrSequencerInactive
		_, err = front.Stop(ctx)
		require.ErrorIs(t, err, seqtypes.ErrSequencerInactive)
		seq.err = nil
	})
}
