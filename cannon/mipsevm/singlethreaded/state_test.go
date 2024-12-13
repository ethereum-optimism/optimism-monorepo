//go:build !cannon64
// +build !cannon64

package singlethreaded

import (
	"bytes"
	"debug/elf"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/memory"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/program"
)

// Run through all permutations of `exited` / `exitCode` and ensure that the
// correct witness, state hash, and VM Status is produced.
func TestStateHash(t *testing.T) {
	cases := []struct {
		exited   bool
		exitCode uint8
	}{
		{exited: false, exitCode: 0},
		{exited: false, exitCode: 1},
		{exited: false, exitCode: 2},
		{exited: false, exitCode: 3},
		{exited: true, exitCode: 0},
		{exited: true, exitCode: 1},
		{exited: true, exitCode: 2},
		{exited: true, exitCode: 3},
	}

	exitedOffset := 32*2 + 4*6
	for _, c := range cases {
		state := &State{
			Memory:   memory.NewMemory(),
			Exited:   c.exited,
			ExitCode: c.exitCode,
		}

		actualWitness, actualStateHash := state.EncodeWitness()
		require.Equal(t, len(actualWitness), STATE_WITNESS_SIZE, "Incorrect witness size")

		expectedWitness := make(StateWitness, 226)
		memRoot := state.Memory.MerkleRoot()
		copy(expectedWitness[:32], memRoot[:])
		expectedWitness[exitedOffset] = c.exitCode
		var exited uint8
		if c.exited {
			exited = 1
		}
		expectedWitness[exitedOffset+1] = uint8(exited)
		require.EqualValues(t, expectedWitness[:], actualWitness[:], "Incorrect witness")

		expectedStateHash := crypto.Keccak256Hash(actualWitness)
		expectedStateHash[0] = mipsevm.VmStatus(c.exited, c.exitCode)
		require.Equal(t, expectedStateHash, actualStateHash, "Incorrect state hash")
	}
}

func TestStateJSONCodec(t *testing.T) {
	elfProgram, err := elf.Open("../../testdata/example/bin/hello.elf")
	require.NoError(t, err, "open ELF file")
	state, err := program.LoadELF(elfProgram, CreateInitialState)
	require.NoError(t, err, "load ELF into state")

	stateJSON, err := state.MarshalJSON()
	require.NoError(t, err)

	newState := new(State)
	require.NoError(t, newState.UnmarshalJSON(stateJSON))

	require.Equal(t, state.PreimageKey, newState.PreimageKey)
	require.Equal(t, state.PreimageOffset, newState.PreimageOffset)
	require.Equal(t, state.Cpu, newState.Cpu)
	require.Equal(t, state.Heap, newState.Heap)
	require.Equal(t, state.ExitCode, newState.ExitCode)
	require.Equal(t, state.Exited, newState.Exited)
	require.Equal(t, state.Memory.MerkleRoot(), newState.Memory.MerkleRoot())
	require.Equal(t, state.Registers, newState.Registers)
	require.Equal(t, state.Step, newState.Step)
}

func TestStateBinaryCodec(t *testing.T) {
	elfProgram, err := elf.Open("../../testdata/example/bin/hello.elf")
	require.NoError(t, err, "open ELF file")
	state, err := program.LoadELF(elfProgram, CreateInitialState)
	require.NoError(t, err, "load ELF into state")

	buf := new(bytes.Buffer)
	err = state.Serialize(buf)
	require.NoError(t, err)

	newState := new(State)
	require.NoError(t, newState.Deserialize(bytes.NewReader(buf.Bytes())))

	require.Equal(t, state.PreimageKey, newState.PreimageKey)
	require.Equal(t, state.PreimageOffset, newState.PreimageOffset)
	require.Equal(t, state.Cpu, newState.Cpu)
	require.Equal(t, state.Heap, newState.Heap)
	require.Equal(t, state.ExitCode, newState.ExitCode)
	require.Equal(t, state.Exited, newState.Exited)
	require.Equal(t, state.Memory.PageCount(), newState.Memory.PageCount())
	require.Equal(t, state.Memory.MerkleRoot(), newState.Memory.MerkleRoot())
	require.Equal(t, state.Registers, newState.Registers)
	require.Equal(t, state.Step, newState.Step)
}

func TestSerializeStateRoundTrip(t *testing.T) {
	// Construct a test case with populated fields
	mem := memory.NewMemory()
	mem.AllocPage(5)
	p := mem.AllocPage(123)
	p.Data[2] = 0x01
	state := &State{
		Memory:         mem,
		PreimageKey:    common.Hash{0xFF},
		PreimageOffset: 5,
		Cpu: mipsevm.CpuScalars{
			PC:     0xFF,
			NextPC: 0xFF + 4,
			LO:     0xbeef,
			HI:     0xbabe,
		},
		Heap:     0xc0ffee,
		ExitCode: 1,
		Exited:   true,
		Step:     0xdeadbeef,
		Registers: [32]arch.Word{
			0xdeadbeef,
			0xdeadbeef,
			0xc0ffee,
			0xbeefbabe,
			0xdeadc0de,
			0xbadc0de,
			0xdeaddead,
		},
		LastHint: hexutil.Bytes{1, 2, 3, 4, 5},
	}

	ser := new(bytes.Buffer)
	err := state.Serialize(ser)
	require.NoError(t, err, "must serialize state")
	state2 := &State{}
	err = state2.Deserialize(ser)
	require.NoError(t, err, "must deserialize state")
	require.Equal(t, state, state2, "must roundtrip state")
}
