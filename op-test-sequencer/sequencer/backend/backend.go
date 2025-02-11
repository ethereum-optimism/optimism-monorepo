package backend

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/metrics"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/builder"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/frontend"
)

var (
	errAlreadyStarted = errors.New("already started")
	errAlreadyStopped = errors.New("already stopped")
)

type Backend struct {
	started atomic.Bool
	logger  log.Logger
	m       metrics.Metricer
	builder builder.Builder
}

func NewBackend(log log.Logger, m metrics.Metricer) *Backend {
	return &Backend{
		logger:  log,
		m:       m,
		builder: builder.NewLocalBuilder(log, m),
	}
}

func (ba *Backend) Builder() frontend.BuildBackend {
	return ba.builder
}

func (ba *Backend) Start(ctx context.Context) error {
	if !ba.started.CompareAndSwap(false, true) {
		return errAlreadyStarted
	}
	ba.logger.Info("Starting sequencer backend")
	return nil
}

func (ba *Backend) Stop(ctx context.Context) error {
	if !ba.started.CompareAndSwap(true, false) {
		return errAlreadyStopped
	}
	ba.logger.Info("Stopping sequencer backend")
	var result error
	if err := ba.builder.Close(); err != nil {
		result = errors.Join(result, fmt.Errorf("failed to close builder: %w", err))
	}
	return result
}

func (ba *Backend) Hello(ctx context.Context, name string) (string, error) {
	return "hello " + name + "!", nil
}
