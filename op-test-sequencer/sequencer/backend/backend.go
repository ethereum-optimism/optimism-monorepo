package backend

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/metrics"
)

var (
	errAlreadyStarted = errors.New("already started")
	errAlreadyStopped = errors.New("already stopped")
)

type Backend struct {
	started atomic.Bool
	logger  log.Logger
	m       metrics.Metricer
}

func NewBackend(log log.Logger, m metrics.Metricer) *Backend {
	return &Backend{
		logger: log,
		m:      m,
	}
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
	return nil
}

func (ba *Backend) Hello(ctx context.Context, name string) (string, error) {
	return "hello " + name + "!", nil
}
