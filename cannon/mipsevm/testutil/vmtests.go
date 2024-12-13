package testutil

import (
	"bytes"
	"io"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/program"
)

type VMFactory[T mipsevm.FPVMState] func(state T, po mipsevm.PreimageOracle, stdOut, stdErr io.Writer, log log.Logger, meta *program.Metadata) mipsevm.FPVM
type StateFactory[T mipsevm.FPVMState] func() T

func RunVMTests_OpenMips[T mipsevm.FPVMState](t *testing.T, stateFactory StateFactory[T], vmFactory VMFactory[T], excludedTests ...string) {
	if !arch.IsMips32 {
		// TODO: guard these tests by the cannon32 build tag
		t.Skip("Open MIPS tests are not appropriate for cannon64")
	}
	testFiles, err := os.ReadDir("../tests/open_mips_tests/test/bin")
	require.NoError(t, err)

	for _, f := range testFiles {
		t.Run(f.Name(), func(t *testing.T) {
			for _, skipped := range excludedTests {
				if f.Name() == skipped {
					t.Skipf("Skipping explicitly excluded open_mips testcase: %v", f.Name())
				}
			}

			oracle := SelectOracleFixture(t, f.Name())
			// Short-circuit early for exit_group.bin
			exitGroup := f.Name() == "exit_group.bin"
			expectPanic := strings.HasSuffix(f.Name(), "panic.bin")

			// TODO: currently tests are compiled as flat binary objects
			// We can use more standard tooling to compile them to ELF files and get remove maketests.py
			fn := path.Join("../tests/open_mips_tests/test/bin", f.Name())
			//elfProgram, err := elf.Open()
			//require.NoError(t, err, "must load test ELF binary")
			//state, err := LoadELF(elfProgram)
			//require.NoError(t, err, "must load ELF into state")
			programMem, err := os.ReadFile(fn)
			require.NoError(t, err)
			state := stateFactory()
			err = state.GetMemory().SetMemoryRange(0, bytes.NewReader(programMem))
			require.NoError(t, err, "load program into state")

			// set the return address ($ra) to jump into when test completes
			state.GetRegistersRef()[31] = EndAddr

			us := vmFactory(state, oracle, os.Stdout, os.Stderr, CreateLogger(), nil)

			// Catch panics and check if they are expected
			defer func() {
				if r := recover(); r != nil {
					if expectPanic {
						// Success
					} else {
						t.Errorf("unexpected panic: %v", r)
					}
				}
			}()

			for i := 0; i < 1000; i++ {
				if us.GetState().GetPC() == EndAddr {
					break
				}
				if exitGroup && us.GetState().GetExited() {
					break
				}
				_, err := us.Step(false)
				require.NoError(t, err)
			}

			if exitGroup {
				require.NotEqual(t, arch.Word(EndAddr), us.GetState().GetPC(), "must not reach end")
				require.True(t, us.GetState().GetExited(), "must set exited state")
				require.Equal(t, uint8(1), us.GetState().GetExitCode(), "must exit with 1")
			} else if expectPanic {
				require.NotEqual(t, arch.Word(EndAddr), us.GetState().GetPC(), "must not reach end")
			} else {
				require.Equal(t, arch.Word(EndAddr), us.GetState().GetPC(), "must reach end")
				done, result := state.GetMemory().GetWord(BaseAddrEnd+4), state.GetMemory().GetWord(BaseAddrEnd+8)
				// inspect test result
				require.Equal(t, done, arch.Word(1), "must be done")
				require.Equal(t, result, arch.Word(1), "must have success result")
			}
		})
	}
}

func RunVMTest_Hello[T mipsevm.FPVMState](t *testing.T, initState program.CreateInitialFPVMState[T], vmFactory VMFactory[T], doPatchGo bool) {
	state, meta := LoadELFProgram(t, ProgramPath("hello"), initState, doPatchGo)

	var stdOutBuf, stdErrBuf bytes.Buffer
	us := vmFactory(state, nil, io.MultiWriter(&stdOutBuf, os.Stdout), io.MultiWriter(&stdErrBuf, os.Stderr), CreateLogger(), meta)

	maxSteps := 430_000
	for i := 0; i < maxSteps; i++ {
		if us.GetState().GetExited() {
			break
		}
		_, err := us.Step(false)
		require.NoError(t, err)
	}

	require.Truef(t, state.GetExited(), "must complete program. reached %d of max %d steps", state.GetStep(), maxSteps)
	require.Equal(t, uint8(0), state.GetExitCode(), "exit with 0")

	require.Equal(t, "hello world!\n", stdOutBuf.String(), "stdout says hello")
	require.Equal(t, "", stdErrBuf.String(), "stderr silent")
}

func RunVMTest_Claim[T mipsevm.FPVMState](t *testing.T, initState program.CreateInitialFPVMState[T], vmFactory VMFactory[T], doPatchGo bool) {
	state, meta := LoadELFProgram(t, ProgramPath("claim"), initState, doPatchGo)

	oracle, expectedStdOut, expectedStdErr := ClaimTestOracle(t)

	var stdOutBuf, stdErrBuf bytes.Buffer
	us := vmFactory(state, oracle, io.MultiWriter(&stdOutBuf, os.Stdout), io.MultiWriter(&stdErrBuf, os.Stderr), CreateLogger(), meta)

	for i := 0; i < 2000_000; i++ {
		if us.GetState().GetExited() {
			break
		}
		_, err := us.Step(false)
		require.NoError(t, err)
	}

	require.True(t, state.GetExited(), "must complete program")
	require.Equal(t, uint8(0), state.GetExitCode(), "exit with 0")

	require.Equal(t, expectedStdOut, stdOutBuf.String(), "stdout")
	require.Equal(t, expectedStdErr, stdErrBuf.String(), "stderr")
}
