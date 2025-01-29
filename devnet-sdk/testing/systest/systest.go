package systest

import (
	"context"

	"github.com/ethereum-optimism/optimism/devnet-sdk/shell/env"
	"github.com/ethereum-optimism/optimism/devnet-sdk/system"
)

type PreconditionValidator func(t T, sys system.System) (context.Context, error)

type SystemTestFunc func(t T, sys system.System)

func SystemTest(t BasicT, f SystemTestFunc, validators ...PreconditionValidator) {
	wt := NewT(t)
	wt.Helper()

	ctx, cancel := context.WithCancel(wt.Context())
	defer cancel()

	wt = wt.WithContext(ctx)

	sys, err := currentPackage.NewSystemFromEnv(env.EnvFileVar)
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

type InteropSystemTestFunc func(t T, sys system.InteropSystem)

func InteropSystemTest(t BasicT, f InteropSystemTestFunc, validators ...PreconditionValidator) {
	SystemTest(t, func(t T, sys system.System) {
		if sys, ok := sys.(system.InteropSystem); ok {
			f(t, sys)
		} else {
			t.Skipf("interop test requested, but system is not an interop system")
		}
	}, validators...)
}
