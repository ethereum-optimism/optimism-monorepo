package tasks

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-service/clock"
)

const eventualTimeout = 10 * time.Second

func TestPoller(t *testing.T) {
	cl := clock.NewDeterministicClock(time.Now())
	counter := new(atomic.Int64)
	poller := NewPoller(func() {
		counter.Add(1)
	}, cl, time.Second*5)

	poller.Start()

	cl.AdvanceTime(time.Second * 6) // hit the first tick

	require.Eventually(t, func() bool {
		t.Log("counter", counter.Load())
		return counter.Load() == 1
	}, eventualTimeout, time.Millisecond*100)

	cl.AdvanceTime(time.Second * 3) // no hit yet, 9 seconds have passsed now

	require.Never(t, func() bool {
		return counter.Load() == 2
	}, time.Second, time.Millisecond*100)

	// hit the second tick at 10s
	cl.AdvanceTime(time.Second * 2) // 11 seconds have passed now
	require.Eventually(t, func() bool {
		return counter.Load() == 2
	}, eventualTimeout, time.Millisecond*100)

	poller.Stop()

	// Poller was stopped, this shouldn't affect it
	cl.AdvanceTime(time.Second * 1000)

	// We should have stopped counting
	require.Never(t, func() bool {
		return counter.Load() > 2
	}, time.Second, time.Millisecond*100)

	// Start back up
	poller.Start()
	// No previously buffered ticks
	require.Never(t, func() bool {
		return counter.Load() > 2
	}, time.Second, time.Millisecond*100)

	// Change the interval, so we poll faster
	poller.SetInterval(time.Second * 2)

	cl.AdvanceTime(time.Second * 3)
	require.Eventually(t, func() bool {
		t.Log("counter", counter.Load())
		return counter.Load() == 3
	}, eventualTimeout, time.Millisecond*100)

	poller.Stop()
}
