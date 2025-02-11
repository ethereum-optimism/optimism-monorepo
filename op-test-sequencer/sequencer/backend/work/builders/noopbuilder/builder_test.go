package noopbuilder

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

func TestNoopBuilder(t *testing.T) {
	x := &Builder{id: seqtypes.BuilderID("foobar")}

	jobID := seqtypes.BuildJobID("foo")
	job, err := x.NewJob(context.Background(), jobID, nil)
	require.NoError(t, err)
	require.Equal(t, "foo", job.String())
	require.Equal(t, jobID, job.ID())

	_, err = job.Seal(context.Background())
	require.ErrorIs(t, err, ErrNoBuild)

	require.NoError(t, job.Cancel(context.Background()))

	require.NoError(t, x.Close())
	require.Equal(t, "noop-builder-foobar", x.String())
}
