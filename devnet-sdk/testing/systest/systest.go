package systest

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/ethereum-optimism/optimism/devnet-sdk/shell/env"
	"github.com/ethereum-optimism/optimism/devnet-sdk/system"
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

// SystemAcquirer attempts to create a System instance.
// Returns (nil, nil) to indicate this acquirer should be skipped (e.g., when prerequisites are not met).
// Any other result indicates this acquirer was selected and its result (success or failure) should be used.
type SystemAcquirer func(t BasicT) (system.System, error)

// tryAcquirers attempts to acquire a system using the provided acquirers in order.
// Each acquirer is tried in sequence until one returns a non-(nil,nil) result.
// If an acquirer returns (nil, nil), it is skipped and the next one is tried.
// Any other result from an acquirer (success or failure) is returned immediately.
func tryAcquirers(t BasicT, acquirers []SystemAcquirer) (system.System, error) {
	for _, acquirer := range acquirers {
		sys, err := acquirer(t)
		if sys == nil && err == nil {
			// Acquirer signaled it should be skipped
			continue
		}
		// Any other result means this acquirer was selected, return its result
		return sys, err
	}
	return nil, fmt.Errorf("no acquirer was able to create a system")
}

type PreconditionValidator func(t T, sys system.System) (context.Context, error)
type SystemTestFunc func(t T, sys system.System)
type InteropSystemTestFunc func(t T, sys system.InteropSystem)

// systemTestHelper defines the interface for system test functionality
type systemTestHelper interface {
	SystemTest(t BasicT, f SystemTestFunc, validators ...PreconditionValidator)
	InteropSystemTest(t BasicT, f InteropSystemTestFunc, validators ...PreconditionValidator)
	WithAcquirers(acquirers []SystemAcquirer) *basicSystemTestHelper
	WithProvider(provider systemProvider) *basicSystemTestHelper
}

// basicSystemTestHelper provides a basic implementation of systemTestHelper using environment variables
type basicSystemTestHelper struct {
	expectPreconditionsMet bool
	acquirers              []SystemAcquirer
	provider               systemProvider
	envGetter              envGetter
}

// acquireFromEnvURL attempts to create a system from the URL specified in the environment variable.
func (h *basicSystemTestHelper) acquireFromEnvURL(t BasicT) (system.System, error) {
	url := h.envGetter.Getenv(env.EnvURLVar)
	if url == "" {
		return nil, nil // Skip this acquirer
	}
	sys, err := h.provider.NewSystemFromURL(url)
	if err != nil {
		return nil, fmt.Errorf("failed to create system from URL %q: %w", url, err)
	}
	return sys, nil
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

	sys, err := tryAcquirers(t, h.acquirers)
	if err != nil {
		h.handlePreconditionError(t, err)
		return
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
	val := envGetter.Getenv(env.ExpectPreconditionsMet)
	expectPreconditionsMet, err := strconv.ParseBool(val)
	if err != nil {
		expectPreconditionsMet = false // empty string or invalid value returns false
	}

	helper := &basicSystemTestHelper{
		expectPreconditionsMet: expectPreconditionsMet,
		provider:               &defaultProvider{},
		envGetter:              envGetter,
	}

	// Set up acquirers after helper is constructed so we can use the method
	helper.acquirers = []SystemAcquirer{
		helper.acquireFromEnvURL,
	}

	return helper
}

// WithAcquirers returns a new helper with the specified acquirers
func (h *basicSystemTestHelper) WithAcquirers(acquirers []SystemAcquirer) *basicSystemTestHelper {
	newHelper := *h
	newHelper.acquirers = acquirers
	return &newHelper
}

// WithProvider returns a new helper with the specified provider
func (h *basicSystemTestHelper) WithProvider(provider systemProvider) *basicSystemTestHelper {
	newHelper := *h
	newHelper.provider = provider
	return &newHelper
}

// SystemTest delegates to the default helper
func SystemTest(t BasicT, f SystemTestFunc, validators ...PreconditionValidator) {
	defaultHelper.SystemTest(t, f, validators...)
}

// InteropSystemTest delegates to the default helper
func InteropSystemTest(t BasicT, f InteropSystemTestFunc, validators ...PreconditionValidator) {
	defaultHelper.InteropSystemTest(t, f, validators...)
}
