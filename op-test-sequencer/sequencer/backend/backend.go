package backend

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/google/uuid"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/locks"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/metrics"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/builder"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/frontend"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

var (
	errAlreadyStarted = errors.New("already started")
	errAlreadyStopped = errors.New("already stopped")
	errInactive       = errors.New("inactive")
)

type Backend struct {
	active atomic.Bool
	logger log.Logger
	m      metrics.Metricer

	builders *locks.RWMap[seqtypes.BuilderID, builder.Builder]
	jobs     *locks.RWMap[seqtypes.JobID, builder.BuildJob]
}

var _ frontend.BuildBackend = (*Backend)(nil)
var _ frontend.AdminBackend = (*Backend)(nil)

func NewBackend(log log.Logger, m metrics.Metricer, builders builder.Builders) *Backend {
	b := &Backend{
		logger:   log,
		m:        m,
		builders: locks.RWMapFromMap(builders),
		jobs:     &locks.RWMap[seqtypes.JobID, builder.BuildJob]{},
	}
	b.builders.Range(func(id seqtypes.BuilderID, bu builder.Builder) bool {
		bu.Attach(b)
		return true
	})
	return b
}

func (ba *Backend) CreateJob(id seqtypes.BuilderID) (builder.BuildJob, error) {
	if !ba.active.Load() {
		return nil, errInactive
	}
	bu, ok := ba.builders.Get(id)
	if !ok {
		return nil, seqtypes.ErrUnknownBuilder
	}
	jobID := seqtypes.JobID("job-" + uuid.New().String())
	job, err := bu.NewJob(jobID)
	if err != nil {
		return nil, err
	}
	ba.jobs.Set(jobID, job)
	return job, nil
}

// GetJob returns nil if the job isn't known.
func (ba *Backend) GetJob(id seqtypes.JobID) builder.BuildJob {
	job, _ := ba.jobs.Get(id)
	return job
}

func (ba *Backend) UnregisterJob(id seqtypes.JobID) {
	ba.jobs.Delete(id)
}

func (ba *Backend) Start(ctx context.Context) error {
	if !ba.active.CompareAndSwap(false, true) {
		return errAlreadyStarted
	}
	ba.logger.Info("Starting sequencer backend")
	return nil
}

func (ba *Backend) Stop(ctx context.Context) error {
	if !ba.active.CompareAndSwap(true, false) {
		return errAlreadyStopped
	}
	ba.logger.Info("Stopping sequencer backend")
	var result error
	ba.builders.Range(func(id seqtypes.BuilderID, bu builder.Builder) bool {
		ba.logger.Info("Closing builder", "builder", id)
		if err := bu.Close(); err != nil {
			result = errors.Join(result, fmt.Errorf("failed to close builder %q: %w", id, err))
		}
		return true
	})
	ba.builders.Clear()
	// builders should have closed the build jobs gracefully where needed. We can clear the jobs now.
	ba.jobs.Clear()
	return result
}

func (ba *Backend) Hello(ctx context.Context, name string) (string, error) {
	return "hello " + name + "!", nil
}
