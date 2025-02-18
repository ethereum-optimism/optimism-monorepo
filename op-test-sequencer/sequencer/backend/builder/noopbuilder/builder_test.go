package noopbuilder

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/builder"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type noopRegistry struct{}

func (m *noopRegistry) UnregisterJob(id seqtypes.JobID) {}

var _ builder.Registry = (*noopRegistry)(nil)

func TestNoopBuilder(t *testing.T) {
	x := &Builder{id: seqtypes.BuilderID("noop")}
	_, err := x.NewJob(context.Background(), "123", nil)
	require.ErrorIs(t, err, builder.ErrNoRegistry)

	x.Attach(&noopRegistry{})

	jobID := seqtypes.JobID("foo")
	job, err := x.NewJob(context.Background(), jobID, nil)
	require.NoError(t, err)
	require.Equal(t, "foo", job.String())
	require.Equal(t, jobID, job.ID())

	_, err = job.Seal(context.Background())
	require.ErrorIs(t, err, ErrNoBuild)

	require.NoError(t, job.Cancel(context.Background()))

	require.NoError(t, x.Close())
	require.Equal(t, "noop", x.String())
}
