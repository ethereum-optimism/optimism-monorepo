package driver

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

type fakeEnd struct {
	closing bool
	result  error
}

func (d *fakeEnd) Closing() bool {
	return d.closing
}

func (d *fakeEnd) Result() error {
	return d.result
}

func TestDriver(t *testing.T) {
	newTestDriver := func(t *testing.T, onEvent func(d *Driver, end *fakeEnd, ev event.Event)) *Driver {
		logger := testlog.Logger(t, log.LevelInfo)
		end := &fakeEnd{}
		d := &Driver{
			logger: logger,
			end:    end,
		}
		d.deriver = event.DeriverFunc(func(ev event.Event) bool {
			onEvent(d, end, ev)
			return true
		})
		return d
	}

	t.Run("insta complete", func(t *testing.T) {
		d := newTestDriver(t, func(d *Driver, end *fakeEnd, ev event.Event) {
			end.closing = true
		})
		require.NoError(t, d.RunComplete())
	})

	t.Run("insta error", func(t *testing.T) {
		mockErr := errors.New("mock error")
		d := newTestDriver(t, func(d *Driver, end *fakeEnd, ev event.Event) {
			end.closing = true
			end.result = mockErr
		})
		require.ErrorIs(t, mockErr, d.RunComplete())
	})

	t.Run("success after a few events", func(t *testing.T) {
		count := 0
		d := newTestDriver(t, func(d *Driver, end *fakeEnd, ev event.Event) {
			if count > 3 {
				end.closing = true
				return
			}
			count += 1
			d.Emit(TestEvent{})
		})
		require.NoError(t, d.RunComplete())
	})

	t.Run("error after a few events", func(t *testing.T) {
		count := 0
		mockErr := errors.New("mock error")
		d := newTestDriver(t, func(d *Driver, end *fakeEnd, ev event.Event) {
			if count > 3 {
				end.closing = true
				end.result = mockErr
				return
			}
			count += 1
			d.Emit(TestEvent{})
		})
		require.ErrorIs(t, mockErr, d.RunComplete())
	})

	t.Run("exhaust events", func(t *testing.T) {
		count := 0
		d := newTestDriver(t, func(d *Driver, end *fakeEnd, ev event.Event) {
			if count < 3 { // stop generating events after a while, without changing end condition
				d.Emit(TestEvent{})
			}
			count += 1
		})
		// No further processing to be done so evaluate if the claims output root is correct.
		require.NoError(t, d.RunComplete())
	})

	t.Run("queued events", func(t *testing.T) {
		count := 0
		d := newTestDriver(t, func(d *Driver, end *fakeEnd, ev event.Event) {
			if count < 3 {
				d.Emit(TestEvent{})
				d.Emit(TestEvent{})
			}
			count += 1
		})
		require.NoError(t, d.RunComplete())
		// add 1 for initial event that RunComplete fires
		require.Equal(t, 1+3*2, count, "must have queued up 2 events 3 times")
	})
}
