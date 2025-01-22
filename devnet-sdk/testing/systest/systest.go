package systest

import (
	"context"
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/system"
)

type T interface {
	testing.TB
	Context() context.Context
	WithContext(ctx context.Context) T
}

type tWrapper struct {
	*testing.T
	ctx context.Context
}

func (t *tWrapper) Context() context.Context {
	return t.ctx
}

func (t *tWrapper) WithContext(ctx context.Context) T {
	return &tWrapper{
		T:   t.T,
		ctx: ctx,
	}
}

func wrapT(t *testing.T) T {
	return &tWrapper{
		T:   t,
		ctx: context.TODO(),
	}
}

func SystemTest(t *testing.T, f func(t T, sys system.System)) {
	wt := wrapT(t)
	wt.Helper()

	ctx, cancel := context.WithCancel(wt.Context())
	defer cancel()

	wt = wt.WithContext(ctx)
	//TODO Stefano
	// this is consuming some env descriptor
	// depending on whether that descriptor contains the "interop" feature, we
	// will build an InteropSystem or a System.
	var sys system.System

	f(wt, sys)
}

func InteropSystemTest(t *testing.T, f func(t T, sys system.InteropSystem)) {
	SystemTest(t, func(t T, sys system.System) {
		if sys, ok := sys.(system.InteropSystem); ok {
			f(t, sys)
		} else {
			t.Skipf("interop test requested, but system is not an interop system")
		}
	})
}
