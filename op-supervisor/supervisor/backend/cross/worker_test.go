package cross

import (
	"context"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

func TestWorker(t *testing.T) {
	logger := testlog.Logger(t, log.LevelDebug)
	t.Run("do work", func(t *testing.T) {
		count := 0
		w := NewWorker(logger, func(ctx context.Context) error {
			count++
			return nil
		})
		t.Cleanup(w.Close)
		// when ProcessWork is called, the workFn is called once
		require.NoError(t, w.ProcessWork())
		require.Equal(t, 1, count)
	})
	t.Run("background worker", func(t *testing.T) {
		count := 0
		w := NewWorker(logger, func(ctx context.Context) error {
			count++
			return nil
		})
		t.Cleanup(w.Close)
		// set a long poll duration so the worker does not auto-run
		w.pollDuration = 100 * time.Second
		// when StartBackground is called, the worker runs in the background
		// the count should increment once
		w.StartBackground()
		require.Eventually(t, func() bool {
			return count == 1
		}, 2*time.Second, 100*time.Millisecond)
	})
	t.Run("background worker OnNewData", func(t *testing.T) {
		count := 0
		w := NewWorker(logger, func(ctx context.Context) error {
			count++
			return nil
		})
		t.Cleanup(w.Close)
		// set a long poll duration so the worker does not auto-run
		w.pollDuration = 100 * time.Second
		// when StartBackground is called, the worker runs in the background
		// the count should increment once
		w.StartBackground()
		require.Eventually(t, func() bool {
			return count == 1
		}, 2*time.Second, 100*time.Millisecond)
		// when OnNewData is called, the worker runs again
		require.NoError(t, w.OnNewData())
		require.Eventually(t, func() bool {
			return count == 2
		}, 2*time.Second, 100*time.Millisecond)
		// and due to the long poll duration, the worker does not run again
		require.Never(t, func() bool {
			return count > 2
		}, 10*time.Second, 100*time.Millisecond)
	})
	t.Run("background fast poll", func(t *testing.T) {
		count := 0
		w := NewWorker(logger, func(ctx context.Context) error {
			count++
			return nil
		})
		t.Cleanup(w.Close)
		// set a long poll duration so the worker does not auto-run
		w.pollDuration = 100 * time.Millisecond
		// when StartBackground is called, the worker runs in the background
		// the count should increment rapidly and reach 10 in 1 second
		w.StartBackground()
		require.Eventually(t, func() bool {
			return count == 10
		}, 2*time.Second, 100*time.Millisecond)
	})
	t.Run("close", func(t *testing.T) {
		count := 0
		w := NewWorker(logger, func(ctx context.Context) error {
			count++
			return nil
		})
		t.Cleanup(w.Close) // close on cleanup in case of early error
		// set a long poll duration so the worker does not auto-run
		w.pollDuration = 100 * time.Millisecond
		// when StartBackground is called, the worker runs in the background
		// the count should increment rapidly and reach 10 in 1 second
		w.StartBackground()
		require.Eventually(t, func() bool {
			return count == 10
		}, 2*time.Second, 100*time.Millisecond)
		// once the worker is closed, it stops running
		// and the count does not increment
		w.Close()
		stopCount := count
		require.Never(t, func() bool {
			return count != stopCount
		}, 3*time.Second, 100*time.Millisecond)
	})
}
