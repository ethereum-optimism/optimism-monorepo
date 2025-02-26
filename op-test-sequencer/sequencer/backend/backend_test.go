package backend

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/metrics"
)

func TestBackend(t *testing.T) {
	logger := testlog.Logger(t, log.LevelWarn)
	b := NewBackend(logger, metrics.NoopMetrics{})
	require.NoError(t, b.Start(context.Background()))
	require.ErrorIs(t, b.Start(context.Background()), errAlreadyStarted)

	result, err := b.Hello(context.Background(), "alice")
	require.NoError(t, err)
	require.Contains(t, result, "alice")

	require.NoError(t, b.Stop(context.Background()))
	require.ErrorIs(t, b.Stop(context.Background()), errAlreadyStopped)
}
