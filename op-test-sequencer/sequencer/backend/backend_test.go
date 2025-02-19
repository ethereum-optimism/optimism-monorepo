package backend

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/metrics"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work/builders/noopbuilder"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

func TestBackend(t *testing.T) {
	logger := testlog.Logger(t, log.LevelWarn)
	builderID := seqtypes.BuilderID("test-builder")
	ensemble := &work.Ensemble{}
	require.NoError(t, ensemble.AddBuilder(noopbuilder.NewBuilder(builderID)))
	b := NewBackend(logger, metrics.NoopMetrics{}, ensemble)
	require.NoError(t, b.Start(context.Background()))
	require.ErrorIs(t, b.Start(context.Background()), errAlreadyStarted)

	result, err := b.Hello(context.Background(), "alice")
	require.NoError(t, err)
	require.Contains(t, result, "alice")

	_, err = b.CreateJob(context.Background(), "not there", nil)
	require.ErrorIs(t, err, seqtypes.ErrUnknownBuilder)

	job, err := b.CreateJob(context.Background(), builderID, nil)
	require.NoError(t, err)

	_, err = job.Seal(context.Background())
	require.ErrorIs(t, err, noopbuilder.ErrNoBuild)

	require.Equal(t, job, b.GetJob(job.ID()))

	require.NoError(t, b.Stop(context.Background()))
	require.ErrorIs(t, b.Stop(context.Background()), errAlreadyStopped)

	require.Zero(t, b.jobs.Len())
}
