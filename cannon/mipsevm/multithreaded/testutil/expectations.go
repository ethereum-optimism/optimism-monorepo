package testutil

import (
	"bytes"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/memory"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/multithreaded"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/testutil"
)

// ExpectedMTState is a test utility that basically stores a copy of a state that can be explicitly mutated
// to define an expected post-state.  The post-state is then validated with ExpectedMTState.Validate(t, postState)
type ExpectedMTState struct {
	PreimageKey         common.Hash
	PreimageOffset      arch.Word
	Heap                arch.Word
	LLReservationStatus multithreaded.LLReservationStatus
	LLAddress           arch.Word
	LLOwnerThread       arch.Word
	ExitCode            uint8
	Exited              bool
	Step                uint64
	LastHint            hexutil.Bytes
	MemoryRoot          common.Hash
	expectedMemory      *memory.Memory
	// Threading-related expectations
	StepsSinceLastContextSwitch uint64
	Wakeup                      arch.Word
	TraverseRight               bool
	NextThreadId                arch.Word
	ThreadCount                 int
	RightStackSize              int
	LeftStackSize               int
	prestateActiveThreadId      arch.Word
	prestateActiveThreadOrig    ExpectedThreadState // Cached for internal use
	ActiveThreadId              arch.Word
	threadExpectations          map[arch.Word]*ExpectedThreadState
}

type ExpectedThreadState struct {
	ThreadId         arch.Word
	ExitCode         uint8
	Exited           bool
	FutexAddr        arch.Word
	FutexVal         arch.Word
	FutexTimeoutStep uint64
	PC               arch.Word
	NextPC           arch.Word
	HI               arch.Word
	LO               arch.Word
	Registers        [32]arch.Word
	Dropped          bool
}

func NewExpectedMTState(fromState *multithreaded.State) *ExpectedMTState {
	currentThread := fromState.GetCurrentThread()

	expectedThreads := make(map[arch.Word]*ExpectedThreadState)
	for _, t := range GetAllThreads(fromState) {
		expectedThreads[t.ThreadId] = newExpectedThreadState(t)
	}

	return &ExpectedMTState{
		// General Fields
		PreimageKey:         fromState.GetPreimageKey(),
		PreimageOffset:      fromState.GetPreimageOffset(),
		Heap:                fromState.GetHeap(),
		LLReservationStatus: fromState.LLReservationStatus,
		LLAddress:           fromState.LLAddress,
		LLOwnerThread:       fromState.LLOwnerThread,
		ExitCode:            fromState.GetExitCode(),
		Exited:              fromState.GetExited(),
		Step:                fromState.GetStep(),
		LastHint:            fromState.GetLastHint(),
		MemoryRoot:          fromState.GetMemory().MerkleRoot(),
		// Thread-related global fields
		StepsSinceLastContextSwitch: fromState.StepsSinceLastContextSwitch,
		Wakeup:                      fromState.Wakeup,
		TraverseRight:               fromState.TraverseRight,
		NextThreadId:                fromState.NextThreadId,
		ThreadCount:                 fromState.ThreadCount(),
		RightStackSize:              len(fromState.RightThreadStack),
		LeftStackSize:               len(fromState.LeftThreadStack),
		// ThreadState expectations
		prestateActiveThreadId:   currentThread.ThreadId,
		prestateActiveThreadOrig: *newExpectedThreadState(currentThread), // Cache prestate thread for internal use
		ActiveThreadId:           currentThread.ThreadId,
		threadExpectations:       expectedThreads,
		expectedMemory:           fromState.Memory.Copy(),
	}
}

func newExpectedThreadState(fromThread *multithreaded.ThreadState) *ExpectedThreadState {
	return &ExpectedThreadState{
		ThreadId:         fromThread.ThreadId,
		ExitCode:         fromThread.ExitCode,
		Exited:           fromThread.Exited,
		FutexAddr:        fromThread.FutexAddr,
		FutexVal:         fromThread.FutexVal,
		FutexTimeoutStep: fromThread.FutexTimeoutStep,
		PC:               fromThread.Cpu.PC,
		NextPC:           fromThread.Cpu.NextPC,
		HI:               fromThread.Cpu.HI,
		LO:               fromThread.Cpu.LO,
		Registers:        fromThread.Registers,
		Dropped:          false,
	}
}

func (e *ExpectedMTState) ExpectStep() {
	// Set some standard expectations for a normal step
	e.Step += 1
	e.PrestateActiveThread().PC += 4
	e.PrestateActiveThread().NextPC += 4
	e.StepsSinceLastContextSwitch += 1
}

func (e *ExpectedMTState) ExpectMemoryWriteUint32(t require.TestingT, addr arch.Word, val uint32) {
	// Align address to 4-byte boundaries
	addr = addr & ^arch.Word(3)

	// Set 4 bytes at addr
	data := testutil.Uint32ToBytes(val)
	err := e.expectedMemory.SetMemoryRange(addr, bytes.NewReader(data))
	require.NoError(t, err)

	e.MemoryRoot = e.expectedMemory.MerkleRoot()
}

func (e *ExpectedMTState) ExpectMemoryWordWrite(addr arch.Word, val arch.Word) {
	e.expectedMemory.SetWord(addr, val)
	e.MemoryRoot = e.expectedMemory.MerkleRoot()
}

func (e *ExpectedMTState) ExpectPreemption(preState *multithreaded.State) {
	e.ActiveThreadId = FindNextThread(preState).ThreadId
	e.StepsSinceLastContextSwitch = 0
	if preState.TraverseRight {
		e.TraverseRight = e.RightStackSize > 1
		e.RightStackSize -= 1
		e.LeftStackSize += 1
	} else {
		e.TraverseRight = e.LeftStackSize == 1
		e.LeftStackSize -= 1
		e.RightStackSize += 1
	}
}

func (e *ExpectedMTState) ExpectNewThread() *ExpectedThreadState {
	newThreadId := e.NextThreadId
	e.NextThreadId += 1
	e.ThreadCount += 1

	// Clone expectations from prestate active thread's original state (bf changing any expectations)
	newThread := &ExpectedThreadState{}
	*newThread = e.prestateActiveThreadOrig

	newThread.ThreadId = newThreadId
	e.threadExpectations[newThreadId] = newThread

	return newThread
}

func (e *ExpectedMTState) ActiveThread() *ExpectedThreadState {
	return e.threadExpectations[e.ActiveThreadId]
}

func (e *ExpectedMTState) PrestateActiveThread() *ExpectedThreadState {
	return e.threadExpectations[e.prestateActiveThreadId]
}

func (e *ExpectedMTState) Thread(threadId arch.Word) *ExpectedThreadState {
	return e.threadExpectations[threadId]
}

func (e *ExpectedMTState) Validate(t require.TestingT, actualState *multithreaded.State) {
	require.Equalf(t, e.PreimageKey, actualState.GetPreimageKey(), "Expect preimageKey = %v", e.PreimageKey)
	require.Equalf(t, e.PreimageOffset, actualState.GetPreimageOffset(), "Expect preimageOffset = %v", e.PreimageOffset)
	require.Equalf(t, e.Heap, actualState.GetHeap(), "Expect heap = 0x%x", e.Heap)
	require.Equalf(t, e.LLReservationStatus, actualState.LLReservationStatus, "Expect LLReservationStatus = %v", e.LLReservationStatus)
	require.Equalf(t, e.LLAddress, actualState.LLAddress, "Expect LLAddress = 0x%x", e.LLAddress)
	require.Equalf(t, e.LLOwnerThread, actualState.LLOwnerThread, "Expect LLOwnerThread = %v", e.LLOwnerThread)
	require.Equalf(t, e.ExitCode, actualState.GetExitCode(), "Expect exitCode = 0x%x", e.ExitCode)
	require.Equalf(t, e.Exited, actualState.GetExited(), "Expect exited = %v", e.Exited)
	require.Equalf(t, e.Step, actualState.GetStep(), "Expect step = %d", e.Step)
	require.Equalf(t, e.LastHint, actualState.GetLastHint(), "Expect lastHint = %v", e.LastHint)
	require.Equalf(t, e.MemoryRoot, common.Hash(actualState.GetMemory().MerkleRoot()), "Expect memory root = %v", e.MemoryRoot)
	// Thread-related global fields
	require.Equalf(t, e.StepsSinceLastContextSwitch, actualState.StepsSinceLastContextSwitch, "Expect StepsSinceLastContextSwitch = %v", e.StepsSinceLastContextSwitch)
	require.Equalf(t, e.Wakeup, actualState.Wakeup, "Expect Wakeup = %v", e.Wakeup)
	require.Equalf(t, e.TraverseRight, actualState.TraverseRight, "Expect TraverseRight = %v", e.TraverseRight)
	require.Equalf(t, e.NextThreadId, actualState.NextThreadId, "Expect NextThreadId = %v", e.NextThreadId)
	require.Equalf(t, e.ThreadCount, actualState.ThreadCount(), "Expect thread count = %v", e.ThreadCount)
	require.Equalf(t, e.RightStackSize, len(actualState.RightThreadStack), "Expect right stack size = %v", e.RightStackSize)
	require.Equalf(t, e.LeftStackSize, len(actualState.LeftThreadStack), "Expect right stack size = %v", e.LeftStackSize)

	// Check active thread
	activeThread := actualState.GetCurrentThread()
	require.Equal(t, e.ActiveThreadId, activeThread.ThreadId)
	// Check all threads
	expectedThreadCount := 0
	for tid, exp := range e.threadExpectations {
		actualThread := FindThread(actualState, tid)
		isActive := tid == activeThread.ThreadId
		if exp.Dropped {
			require.Nil(t, actualThread, "Thread %v should have been dropped", tid)
		} else {
			require.NotNil(t, actualThread, "Could not find thread matching expected thread with id %v", tid)
			e.validateThread(t, exp, actualThread, isActive)
			expectedThreadCount++
		}
	}
	require.Equal(t, expectedThreadCount, actualState.ThreadCount(), "Thread expectations do not match thread count")
}

func (e *ExpectedMTState) validateThread(t require.TestingT, et *ExpectedThreadState, actual *multithreaded.ThreadState, isActive bool) {
	threadInfo := fmt.Sprintf("tid = %v, active = %v", actual.ThreadId, isActive)
	require.Equalf(t, et.ThreadId, actual.ThreadId, "Expect ThreadId = 0x%x (%v)", et.ThreadId, threadInfo)
	require.Equalf(t, et.PC, actual.Cpu.PC, "Expect PC = 0x%x (%v)", et.PC, threadInfo)
	require.Equalf(t, et.NextPC, actual.Cpu.NextPC, "Expect nextPC = 0x%x (%v)", et.NextPC, threadInfo)
	require.Equalf(t, et.HI, actual.Cpu.HI, "Expect HI = 0x%x (%v)", et.HI, threadInfo)
	require.Equalf(t, et.LO, actual.Cpu.LO, "Expect LO = 0x%x (%v)", et.LO, threadInfo)
	require.Equalf(t, et.Registers, actual.Registers, "Expect registers to match (%v)", threadInfo)
	require.Equalf(t, et.ExitCode, actual.ExitCode, "Expect exitCode = %v (%v)", et.ExitCode, threadInfo)
	require.Equalf(t, et.Exited, actual.Exited, "Expect exited = %v (%v)", et.Exited, threadInfo)
	require.Equalf(t, et.FutexAddr, actual.FutexAddr, "Expect futexAddr = %v (%v)", et.FutexAddr, threadInfo)
	require.Equalf(t, et.FutexVal, actual.FutexVal, "Expect futexVal = %v (%v)", et.FutexVal, threadInfo)
	require.Equalf(t, et.FutexTimeoutStep, actual.FutexTimeoutStep, "Expect futexTimeoutStep = %v (%v)", et.FutexTimeoutStep, threadInfo)
}
