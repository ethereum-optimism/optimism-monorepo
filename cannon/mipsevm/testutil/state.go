package testutil

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/memory"
)

func AddHintLengthPrefix(data []byte) []byte {
	dataLen := len(data)
	prefixed := make([]byte, 0, dataLen+4)
	prefixed = binary.BigEndian.AppendUint32(prefixed, uint32(dataLen))
	prefixed = append(prefixed, data...)

	return prefixed
}

func AddPreimageLengthPrefix(data []byte) []byte {
	dataLen := len(data)
	prefixed := make([]byte, 0, dataLen+8)
	prefixed = binary.BigEndian.AppendUint64(prefixed, uint64(dataLen))
	prefixed = append(prefixed, data...)

	return prefixed
}

type StateMutator interface {
	SetPreimageKey(val common.Hash)
	SetPreimageOffset(val arch.Word)
	SetPC(val arch.Word)
	SetNextPC(val arch.Word)
	SetHI(val arch.Word)
	SetLO(val arch.Word)
	SetHeap(addr arch.Word)
	SetExitCode(val uint8)
	SetExited(val bool)
	SetStep(val uint64)
	SetLastHint(val hexutil.Bytes)
	Randomize(randSeed int64)
}

type StateOption func(state StateMutator)

func WithPC(pc arch.Word) StateOption {
	return func(state StateMutator) {
		state.SetPC(pc)
	}
}

func WithNextPC(nextPC arch.Word) StateOption {
	return func(state StateMutator) {
		state.SetNextPC(nextPC)
	}
}

func WithPCAndNextPC(pc arch.Word) StateOption {
	return func(state StateMutator) {
		state.SetPC(pc)
		state.SetNextPC(pc + 4)
	}
}

func WithHI(hi arch.Word) StateOption {
	return func(state StateMutator) {
		state.SetHI(hi)
	}
}

func WithLO(lo arch.Word) StateOption {
	return func(state StateMutator) {
		state.SetLO(lo)
	}
}

func WithHeap(addr arch.Word) StateOption {
	return func(state StateMutator) {
		state.SetHeap(addr)
	}
}

func WithLastHint(lastHint hexutil.Bytes) StateOption {
	return func(state StateMutator) {
		state.SetLastHint(lastHint)
	}
}

func WithPreimageKey(key common.Hash) StateOption {
	return func(state StateMutator) {
		state.SetPreimageKey(key)
	}
}

func WithPreimageOffset(offset arch.Word) StateOption {
	return func(state StateMutator) {
		state.SetPreimageOffset(offset)
	}
}

func WithStep(step uint64) StateOption {
	return func(state StateMutator) {
		state.SetStep(step)
	}
}

func WithRandomization(seed int64) StateOption {
	return func(mut StateMutator) {
		mut.Randomize(seed)
	}
}

func AlignPC(pc arch.Word) arch.Word {
	// Memory-align random pc and leave room for nextPC
	pc = pc & arch.AddressMask // Align address
	if pc >= arch.AddressMask {
		// Leave room to set and then increment nextPC
		pc = arch.AddressMask - 8
	}
	return pc
}

func BoundStep(step uint64) uint64 {
	// Leave room to increment step at least once
	if step == ^uint64(0) {
		step -= 1
	}
	return step
}

type ExpectedState struct {
	PreimageKey    common.Hash
	PreimageOffset arch.Word
	PC             arch.Word
	NextPC         arch.Word
	HI             arch.Word
	LO             arch.Word
	Heap           arch.Word
	ExitCode       uint8
	Exited         bool
	Step           uint64
	LastHint       hexutil.Bytes
	Registers      [32]arch.Word
	MemoryRoot     common.Hash
	expectedMemory *memory.Memory
}

func NewExpectedState(fromState mipsevm.FPVMState) *ExpectedState {
	return &ExpectedState{
		PreimageKey:    fromState.GetPreimageKey(),
		PreimageOffset: fromState.GetPreimageOffset(),
		PC:             fromState.GetPC(),
		NextPC:         fromState.GetCpu().NextPC,
		HI:             fromState.GetCpu().HI,
		LO:             fromState.GetCpu().LO,
		Heap:           fromState.GetHeap(),
		ExitCode:       fromState.GetExitCode(),
		Exited:         fromState.GetExited(),
		Step:           fromState.GetStep(),
		LastHint:       fromState.GetLastHint(),
		Registers:      *fromState.GetRegistersRef(),
		MemoryRoot:     fromState.GetMemory().MerkleRoot(),
		expectedMemory: fromState.GetMemory().Copy(),
	}
}

func (e *ExpectedState) ExpectStep() {
	// Set some standard expectations for a normal step
	e.Step += 1
	e.PC += 4
	e.NextPC += 4
}

func (e *ExpectedState) ExpectMemoryWriteWord(addr arch.Word, val arch.Word) {
	e.expectedMemory.SetWord(addr, val)
	e.MemoryRoot = e.expectedMemory.MerkleRoot()
}

func (e *ExpectedState) Validate(t testing.TB, actualState mipsevm.FPVMState) {
	require.Equal(t, e.PreimageKey, actualState.GetPreimageKey(), fmt.Sprintf("Expect preimageKey = %v", e.PreimageKey))
	require.Equal(t, e.PreimageOffset, actualState.GetPreimageOffset(), fmt.Sprintf("Expect preimageOffset = %v", e.PreimageOffset))
	require.Equal(t, e.PC, actualState.GetCpu().PC, fmt.Sprintf("Expect PC = 0x%x", e.PC))
	require.Equal(t, e.NextPC, actualState.GetCpu().NextPC, fmt.Sprintf("Expect nextPC = 0x%x", e.NextPC))
	require.Equal(t, e.HI, actualState.GetCpu().HI, fmt.Sprintf("Expect HI = 0x%x", e.HI))
	require.Equal(t, e.LO, actualState.GetCpu().LO, fmt.Sprintf("Expect LO = 0x%x", e.LO))
	require.Equal(t, e.Heap, actualState.GetHeap(), fmt.Sprintf("Expect heap = 0x%x", e.Heap))
	require.Equal(t, e.ExitCode, actualState.GetExitCode(), fmt.Sprintf("Expect exitCode = 0x%x", e.ExitCode))
	require.Equal(t, e.Exited, actualState.GetExited(), fmt.Sprintf("Expect exited = %v", e.Exited))
	require.Equal(t, e.Step, actualState.GetStep(), fmt.Sprintf("Expect step = %d", e.Step))
	require.Equal(t, e.LastHint, actualState.GetLastHint(), fmt.Sprintf("Expect lastHint = %v", e.LastHint))
	require.Equal(t, e.Registers, *actualState.GetRegistersRef(), fmt.Sprintf("Expect registers = %v", e.Registers))
	require.Equal(t, e.MemoryRoot, common.Hash(actualState.GetMemory().MerkleRoot()), fmt.Sprintf("Expect memory root = %v", e.MemoryRoot))
}
