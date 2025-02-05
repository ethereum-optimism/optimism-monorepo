package testutil

import (
	"debug/elf"

	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/program"
)

func LoadELFProgram[T mipsevm.FPVMState](t require.TestingT, name string, initState program.CreateInitialFPVMState[T], doPatchGoGC bool) (T, *program.Metadata) {
	elfProgram, err := elf.Open(name)
	require.NoError(t, err, "open ELF file")
	meta, err := program.MakeMetadata(elfProgram)
	require.NoError(t, err, "load metadata")

	state, err := program.LoadELF(elfProgram, initState)
	require.NoError(t, err, "load ELF into state")

	if doPatchGoGC {
		err = program.PatchGoGC(elfProgram, state)
		require.NoError(t, err, "apply Go runtime patches")
	}

	require.NoError(t, program.PatchStack(state), "add initial stack")
	return state, meta
}

// ProgramPath returns the appropriate ELF test program for the current architecture
func ProgramPath(programName string) string {
	basename := programName + ".elf"
	if !arch.IsMips32 {
		basename = programName + ".64.elf"
	}
	return "../../testdata/example/bin/" + basename
}
