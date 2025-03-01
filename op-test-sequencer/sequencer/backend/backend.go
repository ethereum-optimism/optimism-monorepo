package backend

import (
	"context"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-service/locks"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/metrics"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/frontend"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type APIRouter interface {
	AddRPC(route string) error
	AddAPIToRPC(route string, api rpc.API) error
}

type Backend struct {
	started  bool
	active   bool
	activeMu sync.RWMutex

	logger log.Logger
	m      metrics.Metricer

	ensemble *work.Ensemble
	jobs     *locks.RWMap[seqtypes.BuildJobID, work.BuildJob]

	router APIRouter
}

var _ frontend.BuildBackend = (*Backend)(nil)
var _ frontend.AdminBackend = (*Backend)(nil)

func NewBackend(log log.Logger, m metrics.Metricer, ensemble *work.Ensemble, router APIRouter) *Backend {
	b := &Backend{
		logger:   log,
		m:        m,
		ensemble: ensemble,
		jobs:     &locks.RWMap[seqtypes.BuildJobID, work.BuildJob]{},
		router:   router,
	}
	return b
}

// setupSequencerFrontend attaches the sequencer with the given ID as a new route, serving a sequencer RPC.
// errors if the route was invalid (when it already exists, or the ID is not a valid HTTP route).
func (ba *Backend) setupSequencerFrontend(id seqtypes.SequencerID) error {
	route := "/sequencers/" + id.String()
	if err := ba.router.AddRPC(route); err != nil {
		return fmt.Errorf("invalid sequencer RPC route: %w", err)
	}
	f := &frontend.SequencerFrontend{Sequencer: ba.ensemble.Sequencer(id)}
	if err := ba.router.AddAPIToRPC(route, rpc.API{
		Namespace: "sequencer",
		Service:   f,
	}); err != nil {
		return fmt.Errorf("invalid sequencer RPC frontend: %w", err)
	}
	ba.logger.Info("Added sequencer RPC route", "route", route)
	return nil
}

func (ba *Backend) CreateJob(ctx context.Context, id seqtypes.BuilderID, opts *seqtypes.BuildOpts) (work.BuildJob, error) {
	ba.activeMu.RLock()
	defer ba.activeMu.RUnlock()
	if !ba.active {
		return nil, seqtypes.ErrBackendInactive
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
	ba.activeMu.Lock()
	defer ba.activeMu.Unlock()
	if ba.started { // can only start once, no restarts after stopping
		return seqtypes.ErrBackendAlreadyStarted
	}
	ba.active = true
	ba.started = true
	ba.logger.Info("Starting sequencer backend")
	// Each sequencer gets its own API route, for a distinct RPC to interact with just that sequencer.
	for _, id := range ba.ensemble.Sequencers() {
		if err := ba.setupSequencerFrontend(id); err != nil {
			return fmt.Errorf("failed to setup sequencer %q RPC: %w", id, err)
		}
	}
	return nil
}

func (ba *Backend) Stop(ctx context.Context) error {
	ba.activeMu.Lock()
	defer ba.activeMu.Unlock()
	if !ba.active {
		return seqtypes.ErrBackendInactive
	}
	ba.active = false
	ba.logger.Info("Stopping backend")
	result := ba.ensemble.Close()
	// builders should have closed the build jobs gracefully where needed. We can clear the jobs now.
	ba.jobs.Clear()
	return result
}

func (ba *Backend) Hello(ctx context.Context, name string) (string, error) {
	return "hello " + name + "!", nil
}
