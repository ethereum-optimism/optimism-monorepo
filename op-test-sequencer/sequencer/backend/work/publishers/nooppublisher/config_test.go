package nooppublisher

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/metrics"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

func TestConfig(t *testing.T) {
	logger := testlog.Logger(t, log.LevelInfo)
	cfg := &Config{}
	id := seqtypes.PublisherID("test")
	ensemble := &work.Ensemble{}
	opts := &work.StartOpts{
		Log:     logger,
		Metrics: &metrics.NoopMetrics{},
	}
	publisher, err := cfg.Start(context.Background(), id, ensemble, opts)
	require.NoError(t, err)
	require.Equal(t, id, publisher.ID())
}
