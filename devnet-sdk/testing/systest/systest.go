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

type Validator func(t T, sys system.System) (context.Context, error)

func SystemTest(t *testing.T, f func(t T, sys system.System), validators ...Validator) {
	wt := wrapT(t)
	wt.Helper()

	ctx, cancel := context.WithCancel(wt.Context())
	defer cancel()

	wt = wt.WithContext(ctx)

	sys, err := system.NewSystemFromEnv("TEST_DEVNET_FILE")
	if err != nil {
		t.Fatalf("failed to parse system from environment: %v", err)
	}

	for _, validator := range validators {
		ctx, err := validator(wt, sys)
		if err != nil {
			t.Skipf("validator failed: %v", err)
		}
		wt = wt.WithContext(ctx)
	}

	f(wt, sys)
}

func InteropSystemTest(t *testing.T, f func(t T, sys system.InteropSystem), validators ...Validator) {
	SystemTest(t, func(t T, sys system.System) {
		if sys, ok := sys.(system.InteropSystem); ok {
			f(t, sys)
		} else {
			t.Skipf("interop test requested, but system is not an interop system")
		}
	})
}
