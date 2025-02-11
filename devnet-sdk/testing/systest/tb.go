package systest

import (
	"context"
	"testing"
	"time"
)

// tbWrapper converts from testingTB to T
type tbWrapper struct {
	testingTB
	ctx context.Context
}

var _ T = (*tbWrapper)(nil)

func (t *tbWrapper) Context() context.Context {
	t.Helper()
	return t.ctx
}

func (t *tbWrapper) WithContext(ctx context.Context) T {
	t.Helper()
	return &tbWrapper{
		testingTB: t.testingTB,
		ctx:       ctx,
	}
}

func (t *tbWrapper) Deadline() (deadline time.Time, ok bool) {
	t.Helper()
	if tt, ok := t.testingTB.(*testing.T); ok {
		return tt.Deadline()
	}
	// TODO: get proper deadline
	return time.Time{}, false
}

func (t *tbWrapper) Parallel() {
	t.Helper()
	if tt, ok := t.testingTB.(*testing.T); ok {
		tt.Parallel()
	}
	// TODO: implement ourselves. For now, just run sequentially
}

func (t *tbWrapper) Run(name string, fn func(t T)) {
	t.Helper()
	if tt, ok := t.testingTB.(*testing.T); ok {
		tt.Run(name, func(t *testing.T) {
			fn(NewT(t))
		})
	} else {
		// TODO: implement proper sub-tests reporting
		done := make(chan struct{})
		go func() {
			defer close(done)
			fn(NewT(t))
		}()
		<-done
	}
}
