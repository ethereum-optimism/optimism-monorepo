package systest

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/ethereum-optimism/optimism/devnet-sdk/shell/env"
	"github.com/ethereum-optimism/optimism/devnet-sdk/system"
)

const (
	EnvVarExpectPreconditionsMet = "DEVNET_EXPECT_PRECONDITIONS_MET"
)

// envGetter abstracts environment variable access
type envGetter interface {
	Getenv(key string) string
}

// osEnvGetter implements envGetter using os package
type osEnvGetter struct{}

func (g osEnvGetter) Getenv(key string) string {
	return os.Getenv(key)
}

// defaultHelper is the default implementation used by the package-level functions
var defaultHelper systemTestHelper

func init() {
	defaultHelper = newBasicSystemTestHelper(osEnvGetter{})
}

// PreconditionError represents an error that occurs when a test precondition is not met
type PreconditionError struct {
	err error
}

func (e *PreconditionError) Error() string {
	return fmt.Sprintf("precondition not met: %v", e.err)
}

func (e *PreconditionError) Unwrap() error {
	return e.err
}

type PreconditionValidator func(t T, sys system.System) (context.Context, error)
type SystemTestFunc func(t T, sys system.System)
type InteropSystemTestFunc func(t T, sys system.InteropSystem)

// systemTestHelper defines the interface for system test functionality
type systemTestHelper interface {
	SystemTest(t BasicT, f SystemTestFunc, validators ...PreconditionValidator)
	InteropSystemTest(t BasicT, f InteropSystemTestFunc, validators ...PreconditionValidator)
}

// basicSystemTestHelper provides a basic implementation of systemTestHelper using environment variables
type basicSystemTestHelper struct {
	expectPreconditionsMet bool
}

func (h *basicSystemTestHelper) handlePreconditionError(t BasicT, err error) {
	t.Helper()
	precondErr := &PreconditionError{err: err}
	if h.expectPreconditionsMet {
		t.Fatalf("%v", precondErr)
	} else {
		t.Skipf("%v", precondErr)
	}
}

func (h *basicSystemTestHelper) SystemTest(t BasicT, f SystemTestFunc, validators ...PreconditionValidator) {
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
			h.handlePreconditionError(t, err)
		}
		wt = wt.WithContext(ctx)
	}

	f(wt, sys)
}

func (h *basicSystemTestHelper) InteropSystemTest(t BasicT, f InteropSystemTestFunc, validators ...PreconditionValidator) {
	t.Helper()
	h.SystemTest(t, func(t T, sys system.System) {
		if sys, ok := sys.(system.InteropSystem); ok {
			f(t, sys)
		} else {
			h.handlePreconditionError(t, fmt.Errorf("interop test requested, but system is not an interop system"))
		}
	}, validators...)
}

// newBasicSystemTestHelper creates a new basicSystemTestHelper using environment variables
func newBasicSystemTestHelper(envGetter envGetter) *basicSystemTestHelper {
	val := envGetter.Getenv(EnvVarExpectPreconditionsMet)
	expectPreconditionsMet, err := strconv.ParseBool(val)
	if err != nil {
		expectPreconditionsMet = false // empty string or invalid value returns false
	}
	return &basicSystemTestHelper{
		expectPreconditionsMet: expectPreconditionsMet,
	}
}

// SystemTest delegates to the default helper
func SystemTest(t BasicT, f SystemTestFunc, validators ...PreconditionValidator) {
	defaultHelper.SystemTest(t, f, validators...)
}

// InteropSystemTest delegates to the default helper
func InteropSystemTest(t BasicT, f InteropSystemTestFunc, validators ...PreconditionValidator) {
	defaultHelper.InteropSystemTest(t, f, validators...)
}
