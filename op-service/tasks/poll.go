package tasks

import (
	"context"
	"sync"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/clock"
)

// Poller runs a function on repeat at a set interval.
// Warning: ticks can be missed, if the function execution is slow.
type Poller struct {
	fn func()

	clock    clock.Clock
	interval time.Duration

	ticker clock.Ticker // nil if not running

	mu     sync.Mutex
	ctx    context.Context // non-nil when running
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewPoller(fn func(), clock clock.Clock, interval time.Duration) *Poller {
	return &Poller{
		fn:       fn,
		clock:    clock,
		interval: interval,
	}
}

// Start starts polling in a background routine.
// Duplicate start calls are ignored. Only one routine runs.
func (pd *Poller) Start() {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	if pd.ctx != nil {
		return // already running
	}

	pd.ctx, pd.cancel = context.WithCancel(context.Background())

	pd.ticker = pd.clock.NewTicker(pd.interval)

	pd.wg.Add(1)
	go func() {
		defer pd.wg.Done()

		defer pd.ticker.Stop()

		for {
			select {
			case <-pd.ticker.Ch():
				pd.fn()
			case <-pd.ctx.Done():
				return // quiting
			}
		}
	}()
}

// Stop stops the polling. Duplicate calls are ignored.
// Only if active the polling routine is stopped.
func (pd *Poller) Stop() {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	if pd.ctx == nil {
		return // not running, nothing to stop
	}
	pd.cancel()
	pd.wg.Wait()
	pd.ctx = nil
	pd.cancel = nil
	pd.ticker = nil
}

// SetInterval changes the polling interval.
func (pd *Poller) SetInterval(interval time.Duration) {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	pd.interval = interval
	// if we're currently running, change the interval of the active ticker
	if pd.ticker != nil {
		pd.ticker.Reset(interval)
	}
}
