package multithreaded

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/memory"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/program"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/testutil"
)

func vmFactory(state *State, po mipsevm.PreimageOracle, stdOut, stdErr io.Writer, log log.Logger, meta *program.Metadata) mipsevm.FPVM {
	return NewInstrumentedState(state, po, stdOut, stdErr, log, meta)
}

func TestInstrumentedState_OpenMips(t *testing.T) {
	t.Parallel()
	testutil.RunVMTests_OpenMips(t, CreateEmptyState, vmFactory, "clone.bin")
}

func TestInstrumentedState_Hello(t *testing.T) {
	t.Parallel()
	testutil.RunVMTest_Hello(t, CreateInitialState, vmFactory, false)
}

func TestInstrumentedState_Claim(t *testing.T) {
	t.Parallel()
	testutil.RunVMTest_Claim(t, CreateInitialState, vmFactory, false)
}

func TestInstrumentedState_MultithreadedProgram(t *testing.T) {
	t.Parallel()
	state, _ := testutil.LoadELFProgram(t, testutil.ProgramPath("multithreaded"), CreateInitialState, false)
	oracle := testutil.StaticOracle(t, []byte{})

	var stdOutBuf, stdErrBuf bytes.Buffer
	us := NewInstrumentedState(state, oracle, io.MultiWriter(&stdOutBuf, os.Stdout), io.MultiWriter(&stdErrBuf, os.Stderr), testutil.CreateLogger(), nil)
	for i := 0; i < 2_000_000; i++ {
		if us.GetState().GetExited() {
			break
		}
		_, err := us.Step(false)
		require.NoError(t, err)
	}
	t.Logf("Completed in %d steps", state.Step)

	require.True(t, state.Exited, "must complete program")
	require.Equal(t, uint8(0), state.ExitCode, "exit with 0")
	require.Contains(t, "waitgroup result: 42", stdErrBuf.String())
	require.Contains(t, "channels result: 1234", stdErrBuf.String())
	require.Equal(t, "", stdErrBuf.String(), "should not print any errors")
}

func TestInstrumentedState_Alloc(t *testing.T) {
	if os.Getenv("SKIP_SLOW_TESTS") == "true" {
		t.Skip("Skipping slow test because SKIP_SLOW_TESTS is enabled")
	}

	const MiB = 1024 * 1024

	cases := []struct {
		name                string
		numAllocs           int
		allocSize           int
		maxMemoryUsageCheck int
	}{
		{name: "10 32MiB allocations", numAllocs: 10, allocSize: 32 * MiB, maxMemoryUsageCheck: 256 * MiB},
		{name: "5 64MiB allocations", numAllocs: 5, allocSize: 64 * MiB, maxMemoryUsageCheck: 256 * MiB},
		{name: "5 128MiB allocations", numAllocs: 5, allocSize: 128 * MiB, maxMemoryUsageCheck: 128 * 3 * MiB},
	}

	for _, test := range cases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			state, meta := testutil.LoadELFProgram(t, testutil.ProgramPath("alloc"), CreateInitialState, false)
			oracle := testutil.AllocOracle(t, test.numAllocs, test.allocSize)

			us := NewInstrumentedState(state, oracle, os.Stdout, os.Stderr, testutil.CreateLogger(), meta)
			require.NoError(t, us.InitDebug())
			// emulation shouldn't take more than 20 B steps
			for i := 0; i < 20_000_000_000; i++ {
				if us.GetState().GetExited() {
					break
				}
				_, err := us.Step(false)
				require.NoError(t, err)
				if state.Step%10_000_000 == 0 {
					t.Logf("Completed %d steps", state.Step)
				}
			}
			memUsage := state.Memory.PageCount() * memory.PageSize
			t.Logf("Completed in %d steps. cannon memory usage: %d KiB", state.Step, memUsage/1024/1024.0)
			require.True(t, state.Exited, "must complete program")
			require.Equal(t, uint8(0), state.ExitCode, "exit with 0")
			require.Less(t, memUsage, test.maxMemoryUsageCheck, "memory allocation is too large")
		})
	}
}
