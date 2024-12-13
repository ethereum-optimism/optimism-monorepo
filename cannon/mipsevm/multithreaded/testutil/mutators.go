package testutil

import (
	"math"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/exec"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/multithreaded"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/testutil"
)

type StateMutatorMultiThreaded struct {
	state *multithreaded.State
}

var _ testutil.StateMutator = (*StateMutatorMultiThreaded)(nil)

func NewStateMutatorMultiThreaded(state *multithreaded.State) testutil.StateMutator {
	return &StateMutatorMultiThreaded{state: state}
}

func (m *StateMutatorMultiThreaded) Randomize(randSeed int64) {
	r := testutil.NewRandHelper(randSeed)

	step := r.RandStep()

	m.state.PreimageKey = r.RandHash()
	m.state.PreimageOffset = r.Word()
	m.state.Step = step
	m.state.LastHint = r.RandHint()
	m.state.StepsSinceLastContextSwitch = uint64(r.Intn(exec.SchedQuantum))

	// Randomize memory-related fields
	halfMemory := math.MaxUint32 / 2
	m.state.Heap = arch.Word(r.Intn(halfMemory) + halfMemory)
	m.state.LLReservationStatus = multithreaded.LLReservationStatus(r.Intn(3))
	if m.state.LLReservationStatus != multithreaded.LLStatusNone {
		m.state.LLAddress = arch.Word(r.Intn(halfMemory))
		m.state.LLOwnerThread = arch.Word(r.Intn(10))
	}

	// Randomize threads
	activeStackThreads := r.Intn(2) + 1
	inactiveStackThreads := r.Intn(3)
	traverseRight := r.Intn(2) == 1
	SetupThreads(randSeed+1, m.state, traverseRight, activeStackThreads, inactiveStackThreads)
}

func (m *StateMutatorMultiThreaded) SetHI(val arch.Word) {
	m.state.GetCurrentThread().Cpu.HI = val
}

func (m *StateMutatorMultiThreaded) SetLO(val arch.Word) {
	m.state.GetCurrentThread().Cpu.LO = val
}

func (m *StateMutatorMultiThreaded) SetExitCode(val uint8) {
	m.state.ExitCode = val
}

func (m *StateMutatorMultiThreaded) SetExited(val bool) {
	m.state.Exited = val
}

func (m *StateMutatorMultiThreaded) SetPC(val arch.Word) {
	thread := m.state.GetCurrentThread()
	thread.Cpu.PC = val
}

func (m *StateMutatorMultiThreaded) SetHeap(val arch.Word) {
	m.state.Heap = val
}

func (m *StateMutatorMultiThreaded) SetNextPC(val arch.Word) {
	thread := m.state.GetCurrentThread()
	thread.Cpu.NextPC = val
}

func (m *StateMutatorMultiThreaded) SetLastHint(val hexutil.Bytes) {
	m.state.LastHint = val
}

func (m *StateMutatorMultiThreaded) SetPreimageKey(val common.Hash) {
	m.state.PreimageKey = val
}

func (m *StateMutatorMultiThreaded) SetPreimageOffset(val arch.Word) {
	m.state.PreimageOffset = val
}

func (m *StateMutatorMultiThreaded) SetStep(val uint64) {
	m.state.Step = val
}
