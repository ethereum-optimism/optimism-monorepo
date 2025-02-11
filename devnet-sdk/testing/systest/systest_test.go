package systest

import (
	"context"
	"fmt"
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/system"
	"github.com/stretchr/testify/require"
)

// mockSystemTestHelper is a test implementation of systemTestHelper
type mockSystemTestHelper struct {
	expectPreconditionsMet bool
	systemTestCalls        int
	interopTestCalls       int
	preconditionErrors     []error
}

func (h *mockSystemTestHelper) handlePreconditionError(t BasicT, err error) {
	h.preconditionErrors = append(h.preconditionErrors, err)
	if h.expectPreconditionsMet {
		t.Fatalf("%v", &PreconditionError{err: err})
	} else {
		t.Skipf("%v", &PreconditionError{err: err})
	}
}

func (h *mockSystemTestHelper) SystemTest(t BasicT, f SystemTestFunc, validators ...PreconditionValidator) {
	h.systemTestCalls++
	wt := NewT(t)
	sys := newMockSystem()

	ctx, cancel := context.WithCancel(wt.Context())
	defer cancel()
	wt = wt.WithContext(ctx)

	for _, validator := range validators {
		ctx, err := validator(wt, sys)
		if err != nil {
			h.handlePreconditionError(t, err)
			return
		}
		wt = wt.WithContext(ctx)
	}

	f(wt, sys)
}

func (h *mockSystemTestHelper) InteropSystemTest(t BasicT, f InteropSystemTestFunc, validators ...PreconditionValidator) {
	h.interopTestCalls++
	wt := NewT(t)
	sys := newMockInteropSystem()

	ctx, cancel := context.WithCancel(wt.Context())
	defer cancel()
	wt = wt.WithContext(ctx)

	for _, validator := range validators {
		ctx, err := validator(wt, sys)
		if err != nil {
			h.handlePreconditionError(t, err)
			return
		}
		wt = wt.WithContext(ctx)
	}

	f(wt, sys)
}

// mockEnvGetter implements envGetter for testing
type mockEnvGetter struct {
	values map[string]string
}

func (g mockEnvGetter) Getenv(key string) string {
	return g.values[key]
}

// TestSystemTestHelper tests the basic implementation of systemTestHelper
func TestSystemTestHelper(t *testing.T) {
	t.Run("newBasicSystemTestHelper initialization", func(t *testing.T) {
		testCases := []struct {
			name     string
			envValue string
			want     bool
		}{
			{"empty env", "", false},
			{"invalid value", "invalid", false},
			{"zero", "0", false},
			{"false", "false", false},
			{"FALSE", "FALSE", false},
			{"False", "False", false},
			{"f", "f", false},
			{"F", "F", false},
			{"one", "1", true},
			{"true", "true", true},
			{"TRUE", "TRUE", true},
			{"True", "True", true},
			{"t", "t", true},
			{"T", "T", true},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				env := mockEnvGetter{
					values: map[string]string{
						EnvVarExpectPreconditionsMet: tc.envValue,
					},
				}
				helper := newBasicSystemTestHelper(env)
				require.Equal(t, tc.want, helper.expectPreconditionsMet)
			})
		}
	})
}

// TestSystemTest tests the main SystemTest function
func TestSystemTest(t *testing.T) {
	withTestSystem(t, func() (system.System, error) {
		return newMockSystem(), nil
	}, func(t *testing.T) {
		t.Run("basic system test", func(t *testing.T) {
			called := false
			SystemTest(t, func(t T, sys system.System) {
				called = true
				require.NotNil(t, sys)
			})
			require.True(t, called)
		})

		t.Run("with validator", func(t *testing.T) {
			validatorCalled := false
			testCalled := false

			validator := func(t T, sys system.System) (context.Context, error) {
				validatorCalled = true
				return t.Context(), nil
			}

			SystemTest(t, func(t T, sys system.System) {
				testCalled = true
			}, validator)

			require.True(t, validatorCalled)
			require.True(t, testCalled)
		})

		t.Run("multiple validators", func(t *testing.T) {
			validatorCount := 0

			validator := func(t T, sys system.System) (context.Context, error) {
				validatorCount++
				return t.Context(), nil
			}

			SystemTest(t, func(t T, sys system.System) {}, validator, validator, validator)
			require.Equal(t, 3, validatorCount)
		})
	})
}

// TestInteropSystemTest tests the InteropSystemTest function
func TestInteropSystemTest(t *testing.T) {
	t.Run("skips non-interop system", func(t *testing.T) {
		withTestSystem(t, func() (system.System, error) {
			return newMockSystem(), nil
		}, func(t *testing.T) {
			called := false
			InteropSystemTest(t, func(t T, sys system.InteropSystem) {
				called = true
			})
			require.False(t, called)
		})
	})

	t.Run("runs with interop system", func(t *testing.T) {
		withTestSystem(t, func() (system.System, error) {
			return newMockInteropSystem(), nil
		}, func(t *testing.T) {
			called := false
			InteropSystemTest(t, func(t T, sys system.InteropSystem) {
				called = true
				require.NotNil(t, sys.InteropSet())
			})
			require.True(t, called)
		})
	})
}

// TestPreconditionError tests the PreconditionError type and its behavior
func TestPreconditionError(t *testing.T) {
	t.Run("error wrapping", func(t *testing.T) {
		underlying := fmt.Errorf("test error")
		precondErr := &PreconditionError{err: underlying}

		require.Equal(t, "precondition not met: test error", precondErr.Error())
		require.ErrorIs(t, precondErr, underlying)
	})
}

// TestPreconditionHandling tests the precondition error handling behavior
func TestPreconditionHandling(t *testing.T) {
	testCases := []struct {
		name        string
		expectMet   bool
		expectSkip  bool
		expectFatal bool
	}{
		{
			name:        "preconditions not expected skips test",
			expectMet:   false,
			expectSkip:  true,
			expectFatal: false,
		},
		{
			name:        "preconditions expected fails test",
			expectMet:   true,
			expectSkip:  false,
			expectFatal: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			helper := &mockSystemTestHelper{
				expectPreconditionsMet: tc.expectMet,
			}

			recorder := &mockTBRecorder{mockTB: mockTB{name: "test"}}
			testErr := fmt.Errorf("test precondition error")

			helper.SystemTest(recorder, func(t T, sys system.System) {}, func(t T, sys system.System) (context.Context, error) {
				return t.Context(), testErr
			})

			require.Equal(t, tc.expectSkip, recorder.skipped, "unexpected skip state")
			require.Equal(t, tc.expectFatal, recorder.failed, "unexpected fatal state")
			require.Len(t, helper.preconditionErrors, 1, "expected one precondition error")
			require.Equal(t, testErr, helper.preconditionErrors[0])

			if tc.expectSkip {
				require.Contains(t, recorder.skipMsg, "precondition not met")
			}
			if tc.expectFatal {
				require.Contains(t, recorder.fatalMsg, "precondition not met")
			}
		})
	}
}
