package systest

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/interfaces"
	"github.com/ethereum-optimism/optimism/devnet-sdk/shell/env"
	"github.com/ethereum-optimism/optimism/devnet-sdk/system"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"
)

var (
	_ system.Chain = (*mockChain)(nil)
)

// mockTB implements a minimal testing.TB for testing
type mockTB struct {
	testing.TB
	name      string
	failed    bool
	lastError string
}

func (m *mockTB) Helper()               {}
func (m *mockTB) Name() string          { return m.name }
func (m *mockTB) Cleanup(func())        {}
func (m *mockTB) Error(args ...any)     {}
func (m *mockTB) Errorf(string, ...any) {}
func (m *mockTB) Fail()                 {}
func (m *mockTB) FailNow()              {}
func (m *mockTB) Failed() bool          { return false }
func (m *mockTB) Fatal(args ...any) {
	m.failed = true
	m.lastError = fmt.Sprint(args...)
}
func (m *mockTB) Fatalf(format string, args ...any) {
	m.failed = true
	m.lastError = fmt.Sprintf(format, args...)
}
func (m *mockTB) Log(args ...any)          {}
func (m *mockTB) Logf(string, ...any)      {}
func (m *mockTB) Skip(args ...any)         {}
func (m *mockTB) SkipNow()                 {}
func (m *mockTB) Skipf(string, ...any)     {}
func (m *mockTB) Skipped() bool            { return false }
func (m *mockTB) TempDir() string          { return "" }
func (m *mockTB) Setenv(key, value string) {}

// mockTBRecorder extends mockTB to record test outcomes
type mockTBRecorder struct {
	mockTB
	skipped  bool
	failed   bool
	skipMsg  string
	fatalMsg string
}

func (m *mockTBRecorder) Skip(args ...any) { m.skipped = true }
func (m *mockTBRecorder) Skipf(f string, args ...any) {
	m.skipped = true
	m.skipMsg = fmt.Sprintf(f, args...)
}
func (m *mockTBRecorder) Fatal(args ...any) { m.failed = true }
func (m *mockTBRecorder) Fatalf(f string, args ...any) {
	m.failed = true
	m.fatalMsg = fmt.Sprintf(f, args...)
}
func (m *mockTBRecorder) Failed() bool  { return m.failed }
func (m *mockTBRecorder) Skipped() bool { return m.skipped }

// mockChain implements a minimal system.Chain for testing
type mockChain struct{}

func (m *mockChain) RPCURL() string                                  { return "http://localhost:8545" }
func (m *mockChain) Client() (*ethclient.Client, error)              { return ethclient.Dial(m.RPCURL()) }
func (m *mockChain) ID() types.ChainID                               { return types.ChainID(big.NewInt(1)) }
func (m *mockChain) ContractsRegistry() interfaces.ContractsRegistry { return nil }
func (m *mockChain) Wallets(ctx context.Context) ([]system.Wallet, error) {
	return nil, nil
}
func (m *mockChain) GasPrice(ctx context.Context) (*big.Int, error) {
	return big.NewInt(1), nil
}
func (m *mockChain) GasLimit(ctx context.Context, tx system.TransactionData) (uint64, error) {
	return 1000000, nil
}
func (m *mockChain) PendingNonceAt(ctx context.Context, address common.Address) (uint64, error) {
	return 0, nil
}
func (m *mockChain) SupportsEIP(ctx context.Context, eip uint64) bool {
	return true
}

// mockSystem implements a minimal system.System for testing
type mockSystem struct{}

func (m *mockSystem) Identifier() string  { return "mock" }
func (m *mockSystem) L1() system.Chain    { return &mockChain{} }
func (m *mockSystem) L2s() []system.Chain { return []system.Chain{&mockChain{}} }
func (m *mockSystem) Close() error        { return nil }

// mockInteropSet implements a minimal system.InteropSet for testing
type mockInteropSet struct{}

func (m *mockInteropSet) L2s() []system.Chain { return []system.Chain{&mockChain{}} }

// mockInteropSystem implements a minimal system.InteropSystem for testing
type mockInteropSystem struct {
	mockSystem
}

func (m *mockInteropSystem) InteropSet() system.InteropSet { return &mockInteropSet{} }

// newMockSystem creates a new mock system for testing
func newMockSystem() system.System {
	return &mockSystem{}
}

// newMockInteropSystem creates a new mock interop system for testing
func newMockInteropSystem() system.InteropSystem {
	return &mockInteropSystem{}
}

// testSystemCreator is a function that creates a system for testing
type testSystemCreator func() (system.System, error)

// testPackage is a test-specific implementation of the package
type testPackage struct {
	creator testSystemCreator
}

func (p *testPackage) NewSystemFromURL(string) (system.System, error) {
	return p.creator()
}

// withTestSystem runs a test with a custom system creator
func withTestSystem(t *testing.T, creator testSystemCreator, f func(t *testing.T)) {
	// Save original acquirers and restore after test
	origAcquirers := systemAcquirers
	defer func() {
		systemAcquirers = origAcquirers
	}()

	// Replace acquirers with just our test creator
	systemAcquirers = []SystemAcquirer{
		func(t BasicT) (system.System, error) {
			return creator()
		},
	}

	f(t)
}

// TestNewT tests the creation and basic functionality of the test wrapper
func TestNewT(t *testing.T) {
	t.Run("wraps *testing.T correctly", func(t *testing.T) {
		wrapped := NewT(t)
		require.NotNil(t, wrapped)
		require.NotNil(t, wrapped.Context())
	})

	t.Run("preserves existing T implementation", func(t *testing.T) {
		original := NewT(t)
		wrapped := NewT(original)
		require.Equal(t, original, wrapped)
	})
}

// TestTWrapper tests the tbWrapper functionality
func TestTWrapper(t *testing.T) {
	t.Run("context operations", func(t *testing.T) {
		wrapped := NewT(t)
		key := &struct{}{}
		ctx := context.WithValue(context.Background(), key, "value")
		newWrapped := wrapped.WithContext(ctx)

		require.NotEqual(t, wrapped, newWrapped)
		require.Equal(t, "value", newWrapped.Context().Value(key))
	})

	t.Run("deadline", func(t *testing.T) {
		mock := &mockTB{name: "mock"}
		wrapped := NewT(mock)
		deadline, ok := wrapped.Deadline()
		require.False(t, ok, "deadline should not be set")
		require.True(t, deadline.IsZero(), "deadline should be zero time")
	})

	t.Run("parallel execution", func(t *testing.T) {
		wrapped := NewT(t)
		// Should not panic
		wrapped.Parallel()
	})

	t.Run("sub-tests", func(t *testing.T) {
		wrapped := NewT(t)
		subTestCalled := false
		wrapped.Run("sub-test", func(t T) {
			subTestCalled = true
			require.NotNil(t, t)
			require.NotNil(t, t.Context())
		})
		require.True(t, subTestCalled)
	})

	t.Run("nested sub-tests", func(t *testing.T) {
		wrapped := NewT(t)
		level1Called := false
		level2Called := false

		wrapped.Run("level-1", func(t T) {
			level1Called = true
			t.Run("level-2", func(t T) {
				level2Called = true
			})
		})

		require.True(t, level1Called)
		require.True(t, level2Called)
	})
}

// mockAcquirer creates a SystemAcquirer that returns the given system and error
func mockAcquirer(sys system.System, err error) SystemAcquirer {
	return func(t BasicT) (system.System, error) {
		return sys, err
	}
}

// TestTryAcquirers tests the tryAcquirers helper function directly
func TestTryAcquirers(t *testing.T) {
	t.Run("empty acquirers list", func(t *testing.T) {
		sys, err := tryAcquirers(t, nil)
		require.EqualError(t, err, "no acquirer was able to create a system")
		require.Nil(t, sys)
	})

	t.Run("skips nil,nil results", func(t *testing.T) {
		sys1 := newMockSystem()
		acquirers := []SystemAcquirer{
			mockAcquirer(nil, nil),  // skipped
			mockAcquirer(nil, nil),  // skipped
			mockAcquirer(sys1, nil), // selected and succeeds
		}
		sys, err := tryAcquirers(t, acquirers)
		require.NoError(t, err)
		require.Equal(t, sys1, sys)
	})

	t.Run("returns first non-skip result (success)", func(t *testing.T) {
		sys1, sys2 := newMockSystem(), newMockSystem()
		acquirers := []SystemAcquirer{
			mockAcquirer(nil, nil),  // skipped
			mockAcquirer(sys1, nil), // selected and succeeds
			mockAcquirer(sys2, nil), // not reached
		}
		sys, err := tryAcquirers(t, acquirers)
		require.NoError(t, err)
		require.Equal(t, sys1, sys)
	})

	t.Run("returns first non-skip result (failure)", func(t *testing.T) {
		expectedErr := fmt.Errorf("selected acquirer failed")
		sys1 := newMockSystem()
		acquirers := []SystemAcquirer{
			mockAcquirer(nil, nil),         // skipped
			mockAcquirer(nil, expectedErr), // selected and fails
			mockAcquirer(sys1, nil),        // not reached
		}
		sys, err := tryAcquirers(t, acquirers)
		require.ErrorIs(t, err, expectedErr)
		require.Nil(t, sys)
	})

	t.Run("all acquirers skip", func(t *testing.T) {
		acquirers := []SystemAcquirer{
			mockAcquirer(nil, nil),
			mockAcquirer(nil, nil),
		}
		sys, err := tryAcquirers(t, acquirers)
		require.EqualError(t, err, "no acquirer was able to create a system")
		require.Nil(t, sys)
	})
}

// Update TestSystemAcquisition to match new behavior
func TestSystemAcquisition(t *testing.T) {
	// Save original acquirers and restore after test
	origAcquirers := systemAcquirers
	defer func() {
		systemAcquirers = origAcquirers
	}()

	t.Run("uses first non-skip acquirer (success)", func(t *testing.T) {
		sys1, sys2 := newMockSystem(), newMockSystem()
		acquirers := []SystemAcquirer{
			mockAcquirer(nil, nil),  // skipped
			mockAcquirer(sys1, nil), // selected and succeeds
			mockAcquirer(sys2, nil), // not reached
		}
		systemAcquirers = acquirers

		var acquiredSys system.System
		SystemTest(t, func(t T, sys system.System) {
			acquiredSys = sys
		})
		require.Equal(t, sys1, acquiredSys)
	})

	t.Run("fails when selected acquirer fails", func(t *testing.T) {
		expectedErr := fmt.Errorf("selected acquirer failed")
		systemAcquirers = []SystemAcquirer{
			mockAcquirer(nil, nil),         // skipped
			mockAcquirer(nil, expectedErr), // selected and fails
		}

		mock := &mockTB{name: "mock"}
		SystemTest(mock, func(t T, sys system.System) {
			require.Fail(t, "should not reach here")
		})
		require.True(t, mock.failed)
		require.Contains(t, mock.lastError, expectedErr.Error())
	})

	t.Run("fails when all acquirers skip", func(t *testing.T) {
		systemAcquirers = []SystemAcquirer{
			mockAcquirer(nil, nil),
			mockAcquirer(nil, nil),
		}

		mock := &mockTB{name: "mock"}
		SystemTest(mock, func(t T, sys system.System) {
			require.Fail(t, "should not reach here")
		})
		require.True(t, mock.failed)
		require.Contains(t, mock.lastError, "no acquirer was able to create a system")
	})

	t.Run("acquireFromEnvURL behavior", func(t *testing.T) {
		// Save original env var and package
		origEnvFile := os.Getenv(env.EnvURLVar)
		origPkg := currentPackage
		defer func() {
			os.Setenv(env.EnvURLVar, origEnvFile)
			currentPackage = origPkg
		}()

		t.Run("skips when env var not set", func(t *testing.T) {
			os.Unsetenv(env.EnvURLVar)
			sys, err := acquireFromEnvURL(t)
			require.NoError(t, err)
			require.Nil(t, sys)
		})

		t.Run("fails with error for invalid URL", func(t *testing.T) {
			os.Setenv(env.EnvURLVar, "invalid://url")
			sys, err := acquireFromEnvURL(t)
			require.Error(t, err)
			require.Nil(t, sys)
		})

		t.Run("succeeds with valid URL", func(t *testing.T) {
			// Set up test package that returns a mock system
			currentPackage = &testPackage{
				creator: func() (system.System, error) {
					return newMockSystem(), nil
				},
			}
			os.Setenv(env.EnvURLVar, "file:///valid/url")
			sys, err := acquireFromEnvURL(t)
			require.NoError(t, err)
			require.NotNil(t, sys)
		})
	})
}
