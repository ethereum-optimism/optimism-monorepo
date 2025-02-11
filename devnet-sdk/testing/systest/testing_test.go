package systest

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/constraints"
	"github.com/ethereum-optimism/optimism/devnet-sdk/interfaces"
	"github.com/ethereum-optimism/optimism/devnet-sdk/shell/env"
	"github.com/ethereum-optimism/optimism/devnet-sdk/system"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/stretchr/testify/require"
)

// mockTB implements a minimal testing.TB for testing
type mockTB struct {
	testing.TB
	name string
}

func (m *mockTB) Helper()                  {}
func (m *mockTB) Name() string             { return m.name }
func (m *mockTB) Cleanup(func())           {}
func (m *mockTB) Error(args ...any)        {}
func (m *mockTB) Errorf(string, ...any)    {}
func (m *mockTB) Fail()                    {}
func (m *mockTB) FailNow()                 {}
func (m *mockTB) Failed() bool             { return false }
func (m *mockTB) Fatal(args ...any)        {}
func (m *mockTB) Fatalf(string, ...any)    {}
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
func (m *mockChain) ID() types.ChainID                               { return types.ChainID(big.NewInt(1)) }
func (m *mockChain) ContractsRegistry() interfaces.ContractsRegistry { return nil }
func (m *mockChain) Wallet(ctx context.Context, constraints ...constraints.WalletConstraint) (types.Wallet, error) {
	return nil, nil
}

// mockSystem implements a minimal system.System for testing
type mockSystem struct{}

func (m *mockSystem) Identifier() string     { return "mock" }
func (m *mockSystem) L1() system.Chain       { return &mockChain{} }
func (m *mockSystem) L2(uint64) system.Chain { return &mockChain{} }
func (m *mockSystem) Close() error           { return nil }

// mockInteropSet implements a minimal system.InteropSet for testing
type mockInteropSet struct{}

func (m *mockInteropSet) L2(uint64) system.Chain { return &mockChain{} }

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

func (p *testPackage) NewSystemFromEnv(string) (system.System, error) {
	return p.creator()
}

// withTestSystem runs a test with a custom system creator
func withTestSystem(t *testing.T, creator testSystemCreator, f func(t *testing.T)) {
	// Save original env var
	origEnvFile := os.Getenv(env.EnvFileVar)
	defer os.Setenv(env.EnvFileVar, origEnvFile)

	// Set empty env var for testing
	os.Setenv(env.EnvFileVar, "")

	// Create a test-specific package
	pkg := &testPackage{creator: creator}
	origPkg := currentPackage
	currentPackage = pkg
	defer func() {
		currentPackage = origPkg
	}()

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
