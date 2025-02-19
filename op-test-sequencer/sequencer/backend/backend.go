package backend

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/locks"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/metrics"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
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

	ensemble *work.Ensemble
	jobs     *locks.RWMap[seqtypes.BuildJobID, work.BuildJob]
}

var _ frontend.BuildBackend = (*Backend)(nil)
var _ frontend.AdminBackend = (*Backend)(nil)

func NewBackend(log log.Logger, m metrics.Metricer, ensemble *work.Ensemble) *Backend {
	b := &Backend{
		logger:   log,
		m:        m,
		ensemble: ensemble,
		jobs:     &locks.RWMap[seqtypes.BuildJobID, work.BuildJob]{},
	}
	return b
}

func (ba *Backend) CreateJob(ctx context.Context, id seqtypes.BuilderID, opts *seqtypes.BuildOpts) (work.BuildJob, error) {
	if !ba.active.Load() {
		return nil, errInactive
	}
	bu := ba.ensemble.Builder(id)
	if bu == nil {
		return nil, seqtypes.ErrUnknownBuilder
	}
	jobID := seqtypes.RandomJobID()
	job, err := bu.NewJob(ctx, jobID, opts)
	if err != nil {
		return nil, err
	}
	ba.jobs.Set(jobID, job)
	return job, nil
}

// GetJob returns nil if the job isn't known.
func (ba *Backend) GetJob(id seqtypes.BuildJobID) work.BuildJob {
	job, _ := ba.jobs.Get(id)
	return job
}

func (ba *Backend) UnregisterJob(id seqtypes.BuildJobID) {
	ba.jobs.Delete(id)
}

func (ba *Backend) Start(ctx context.Context) error {
	if !ba.active.CompareAndSwap(false, true) {
		return errAlreadyStarted
	}
	ba.logger.Info("Starting sequencer backend")

	//for _, seq := range ba.ensemble.Sequencers() {
	//	TODO: setup RPC server route for the sequencer
	//}

	return nil
}

func (ba *Backend) Stop(ctx context.Context) error {
	if !ba.active.CompareAndSwap(true, false) {
		return errAlreadyStopped
	}
	ba.logger.Info("Stopping sequencer backend")

	// TODO stop sequencer RPC routes

	result := ba.ensemble.Close()
	// builders should have closed the build jobs gracefully where needed. We can clear the jobs now.
	ba.jobs.Clear()
	return result
}

func (ba *Backend) Hello(ctx context.Context, name string) (string, error) {
	return "hello " + name + "!", nil
}
