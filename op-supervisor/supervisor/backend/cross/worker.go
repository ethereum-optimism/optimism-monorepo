package cross

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/log"
)

// Worker iterates work
type Worker struct {
	log log.Logger

	// workFn is the function to call to process the scope
	workFn workFn

	// channel with capacity of 1, full if there is work to do
	poke         chan struct{}
	pollDuration time.Duration

	// lifetime management of the chain processor
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// workFn is a function used by the worker
// it is opaque to the worker, and is set by the constructor
type workFn func(ctx context.Context) error

// NewWorker creates a new worker to process updates
func NewWorker(log log.Logger, workFn workFn) *Worker {
	ctx, cancel := context.WithCancel(context.Background())
	out := &Worker{
		log:  log,
		poke: make(chan struct{}, 1),
		// The data may have changed, and we may have missed a poke, so re-attempt regularly.
		pollDuration: 250 * time.Millisecond,
		ctx:          ctx,
		cancel:       cancel,
	}
	out.workFn = workFn
	return out
}

func (s *Worker) StartBackground() {
	s.wg.Add(1)
	go s.worker()
}

func (s *Worker) ProcessWork() error {
	return s.workFn(s.ctx)
}

func (s *Worker) worker() {
	defer s.wg.Done()

	delay := time.NewTicker(s.pollDuration)
	for {
		if s.ctx.Err() != nil { // check if we are closing down
			return
		}

		// do the work
		err := s.workFn(s.ctx)
		if err != nil {
			if errors.Is(err, s.ctx.Err()) {
				return
			}
			if errors.Is(err, types.ErrFuture) {
				s.log.Debug("Worker awaits additional blocks", "err", err)
			} else {
				s.log.Warn("Failed to process work", "err", err)
			}
		}

		// await next time we process, or detect shutdown
		select {
		case <-s.ctx.Done():
			delay.Stop()
			return
		case <-s.poke:
			s.log.Debug("Continuing cross-safe verification after hint of new data")
			continue
		case <-delay.C:
			s.log.Debug("Checking for cross-safe updates")
			continue
		}
	}
}

func (s *Worker) OnNewData() {
	// signal that we have something to process
	select {
	case s.poke <- struct{}{}:
	default:
		// already requested an update
	}
}

func (s *Worker) Close() {
	s.cancel()
	s.wg.Wait()
}
