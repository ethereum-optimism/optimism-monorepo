package cross

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

func TestWorker(t *testing.T) {
	logger := testlog.Logger(t, log.LevelDebug)
	t.Run("do work", func(t *testing.T) {
		var count int32
		w := NewWorker(logger, func(ctx context.Context) error {
			atomic.AddInt32(&count, 1)
			return nil
		})
		t.Cleanup(w.Close)
		// when ProcessWork is called, the workFn is called once
		require.NoError(t, w.ProcessWork())
		require.EqualValues(t, 1, atomic.LoadInt32(&count))
	})
	t.Run("background worker", func(t *testing.T) {
		var count int32
		w := NewWorker(logger, func(ctx context.Context) error {
			atomic.AddInt32(&count, 1)
			return nil
		})
		t.Cleanup(w.Close)
		// set a long poll duration so the worker does not auto-run
		w.pollDuration = 100 * time.Second
		// when StartBackground is called, the worker runs in the background
		// the count should increment once
		w.StartBackground()
		require.Eventually(t, func() bool {
			return atomic.LoadInt32(&count) == 1
		}, 2*time.Second, 100*time.Millisecond)
	})
	t.Run("background worker OnNewData", func(t *testing.T) {
		var count int32
		w := NewWorker(logger, func(ctx context.Context) error {
			atomic.AddInt32(&count, 1)
			return nil
		})
		t.Cleanup(w.Close)
		// set a long poll duration so the worker does not auto-run
		w.pollDuration = 100 * time.Second
		// when StartBackground is called, the worker runs in the background
		// the count should increment once
		w.StartBackground()
		require.Eventually(t, func() bool {
			return atomic.LoadInt32(&count) == 1
		}, 2*time.Second, 100*time.Millisecond)
		// when OnNewData is called, the worker runs again
		w.OnNewData()
		require.Eventually(t, func() bool {
			return atomic.LoadInt32(&count) == 2
		}, 2*time.Second, 100*time.Millisecond)
		// and due to the long poll duration, the worker does not run again
		require.Never(t, func() bool {
			return atomic.LoadInt32(&count) > 2
		}, time.Second, 100*time.Millisecond)
	})
	t.Run("background fast poll", func(t *testing.T) {
		var count int32
		w := NewWorker(logger, func(ctx context.Context) error {
			atomic.AddInt32(&count, 1)
			return nil
		})
		t.Cleanup(w.Close)
		// set a long poll duration so the worker does not auto-run
		w.pollDuration = 100 * time.Millisecond
		// when StartBackground is called, the worker runs in the background
		// the count should increment rapidly and reach at least 10 in 1 second
		w.StartBackground()
		require.Eventually(t, func() bool {
			return atomic.LoadInt32(&count) >= 10
		}, 2*time.Second, 100*time.Millisecond)
	})
	t.Run("close", func(t *testing.T) {
		var count int32
		w := NewWorker(logger, func(ctx context.Context) error {
			atomic.AddInt32(&count, 1)
			return nil
		})
		t.Cleanup(w.Close) // close on cleanup in case of early error
		// set a long poll duration so the worker does not auto-run
		w.pollDuration = 100 * time.Millisecond
		// when StartBackground is called, the worker runs in the background
		// the count should increment rapidly and reach at least 10 in 1 second
		w.StartBackground()
		require.Eventually(t, func() bool {
			return atomic.LoadInt32(&count) >= 10
		}, 10*time.Second, time.Second)
		// once the worker is closed, it stops running
		// and the count does not increment
		w.Close()
		stopCount := atomic.LoadInt32(&count)
		require.Never(t, func() bool {
			return atomic.LoadInt32(&count) != stopCount
		}, time.Second, 100*time.Millisecond)
	})
}
