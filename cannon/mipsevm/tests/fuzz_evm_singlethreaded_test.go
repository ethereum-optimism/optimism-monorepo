//go:build !cannon64
// +build !cannon64

package tests

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/testutil"
)

func FuzzStateSyscallCloneST(f *testing.F) {
	v := GetSingleThreadedTestCase(f)
	f.Fuzz(func(t *testing.T, seed int64) {
		goVm := v.VMFactory(nil, os.Stdout, os.Stderr, testutil.CreateLogger(), testutil.WithRandomization(seed))
		state := goVm.GetState()
		state.GetRegistersRef()[2] = arch.SysClone
		testutil.StoreInstruction(state.GetMemory(), state.GetPC(), syscallInsn)
		step := state.GetStep()

		expected := testutil.NewExpectedState(state)
		expected.Step += 1
		expected.PC = state.GetCpu().NextPC
		expected.NextPC = state.GetCpu().NextPC + 4
		expected.Registers[2] = 0x1
		expected.Registers[7] = 0

		stepWitness, err := goVm.Step(true)
		require.NoError(t, err)
		require.False(t, stepWitness.HasPreimage())

		expected.Validate(t, state)
		testutil.ValidateEVM(t, stepWitness, step, goVm, v.StateHashFn, v.Contracts)
	})
}
