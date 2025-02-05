package multithreaded

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/exec"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/program"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/register"
)

type Word = arch.Word

func (m *InstrumentedState) handleSyscall() error {
	thread := m.state.GetCurrentThread()

	syscallNum, a0, a1, a2 := exec.GetSyscallArgs(m.state.GetRegistersRef())
	v0 := Word(0)
	v1 := Word(0)

	//fmt.Printf("syscall: %d\n", syscallNum)
	switch syscallNum {
	case arch.SysMmap:
		var newHeap Word
		v0, v1, newHeap = exec.HandleSysMmap(a0, a1, m.state.Heap)
		m.state.Heap = newHeap
	case arch.SysBrk:
		v0 = program.PROGRAM_BREAK
	case arch.SysClone: // clone
		// a0 = flag bitmask, a1 = stack pointer
		if exec.ValidCloneFlags != a0 {
			m.state.Exited = true
			m.state.ExitCode = mipsevm.VMStatusPanic
			return nil
		}

		v0 = m.state.NextThreadId
		v1 = 0
		newThread := &ThreadState{
			ThreadId: m.state.NextThreadId,
			ExitCode: 0,
			Exited:   false,
			Cpu: mipsevm.CpuScalars{
				PC:     thread.Cpu.NextPC,
				NextPC: thread.Cpu.NextPC + 4,
				HI:     thread.Cpu.HI,
				LO:     thread.Cpu.LO,
			},
			Registers: thread.Registers,
		}

		newThread.Registers[register.RegSP] = a1
		// the child will perceive a 0 value as returned value instead, and no error
		newThread.Registers[register.RegSyscallRet1] = 0
		newThread.Registers[register.RegSyscallErrno] = 0
		m.state.NextThreadId++

		// Preempt this thread for the new one. But not before updating PCs
		stackCaller := thread.Cpu.PC
		stackTarget := thread.Cpu.NextPC
		exec.HandleSyscallUpdates(&thread.Cpu, &thread.Registers, v0, v1)
		m.pushThread(newThread)
		// Note: We need to call stackTracker after pushThread
		// to ensure we are tracking in the context of the new thread
		m.stackTracker.PushStack(stackCaller, stackTarget)
		return nil
	case arch.SysExitGroup:
		m.state.Exited = true
		m.state.ExitCode = uint8(a0)
		return nil
	case arch.SysRead:
		var newPreimageOffset Word
		var memUpdated bool
		var memAddr Word
		v0, v1, newPreimageOffset, memUpdated, memAddr = exec.HandleSysRead(a0, a1, a2, m.state.PreimageKey, m.state.PreimageOffset, m.preimageOracle, m.state.Memory, m.memoryTracker)
		m.state.PreimageOffset = newPreimageOffset
		if memUpdated {
			m.handleMemoryUpdate(memAddr)
		}
	case arch.SysWrite:
		var newLastHint hexutil.Bytes
		var newPreimageKey common.Hash
		var newPreimageOffset Word
		v0, v1, newLastHint, newPreimageKey, newPreimageOffset = exec.HandleSysWrite(a0, a1, a2, m.state.LastHint, m.state.PreimageKey, m.state.PreimageOffset, m.preimageOracle, m.state.Memory, m.memoryTracker, m.stdOut, m.stdErr)
		m.state.LastHint = newLastHint
		m.state.PreimageKey = newPreimageKey
		m.state.PreimageOffset = newPreimageOffset
	case arch.SysFcntl:
		v0, v1 = exec.HandleSysFcntl(a0, a1)
	case arch.SysGetTID:
		v0 = thread.ThreadId
		v1 = 0
	case arch.SysExit:
		thread.Exited = true
		thread.ExitCode = uint8(a0)
		if m.lastThreadRemaining() {
			m.state.Exited = true
			m.state.ExitCode = uint8(a0)
		}
		return nil
	case arch.SysFutex:
		// args: a0 = addr, a1 = op, a2 = val, a3 = timeout
		// Futex value is 32-bit, so clear the lower 2 bits to get an effective address targeting a 4-byte value
		effFutexAddr := a0 & ^Word(0x3)
		switch a1 {
		case exec.FutexWaitPrivate:
			futexVal := m.getFutexValue(effFutexAddr)
			targetVal := uint32(a2)
			if futexVal != targetVal {
				v0 = exec.SysErrorSignal
				v1 = exec.MipsEAGAIN
			} else {
				m.syscallYield(thread)
				return nil
			}
		case exec.FutexWakePrivate:
			m.syscallYield(thread)
			return nil
		default:
			v0 = exec.SysErrorSignal
			v1 = exec.MipsEINVAL
		}
	case arch.SysSchedYield, arch.SysNanosleep:
		m.syscallYield(thread)
		return nil
	case arch.SysOpen:
		v0 = exec.SysErrorSignal
		v1 = exec.MipsEBADF
	case arch.SysClockGetTime:
		switch a0 {
		case exec.ClockGettimeRealtimeFlag, exec.ClockGettimeMonotonicFlag:
			v0, v1 = 0, 0
			var secs, nsecs Word
			if a0 == exec.ClockGettimeMonotonicFlag {
				// monotonic clock_gettime is used by Go guest programs for goroutine scheduling and to implement
				// `time.Sleep` (and other sleep related operations).
				secs = Word(m.state.Step / exec.HZ)
				nsecs = Word((m.state.Step % exec.HZ) * (1_000_000_000 / exec.HZ))
			} // else realtime set to Unix Epoch

			effAddr := a1 & arch.AddressMask
			m.memoryTracker.TrackMemAccess(effAddr)
			m.state.Memory.SetWord(effAddr, secs)
			m.handleMemoryUpdate(effAddr)
			m.memoryTracker.TrackMemAccess2(effAddr + arch.WordSizeBytes)
			m.state.Memory.SetWord(effAddr+arch.WordSizeBytes, nsecs)
			m.handleMemoryUpdate(effAddr + arch.WordSizeBytes)
		default:
			v0 = exec.SysErrorSignal
			v1 = exec.MipsEINVAL
		}
	case arch.SysGetpid:
		v0 = 0
		v1 = 0
	case arch.SysMunmap:
	case arch.SysGetAffinity:
	case arch.SysMadvise:
	case arch.SysRtSigprocmask:
	case arch.SysSigaltstack:
	case arch.SysRtSigaction:
	case arch.SysPrlimit64:
	case arch.SysClose:
	case arch.SysPread64:
	case arch.SysStat:
	case arch.SysFstat:
	case arch.SysOpenAt:
	case arch.SysReadlink:
	case arch.SysReadlinkAt:
	case arch.SysIoctl:
	case arch.SysEpollCreate1:
	case arch.SysPipe2:
	case arch.SysEpollCtl:
	case arch.SysEpollPwait:
	case arch.SysGetRandom:
	case arch.SysUname:
	case arch.SysGetuid:
	case arch.SysGetgid:
	case arch.SysMinCore:
	case arch.SysTgkill:
	case arch.SysSetITimer:
	case arch.SysTimerCreate:
	case arch.SysTimerSetTime:
	case arch.SysTimerDelete:
	case arch.SysGetRLimit:
	case arch.SysLseek:
	default:
		// These syscalls have the same values on 64-bit. So we use if-stmts here to avoid "duplicate case" compiler error for the cannon64 build
		if arch.IsMips32 && (syscallNum == arch.SysFstat64 || syscallNum == arch.SysStat64 || syscallNum == arch.SysLlseek) {
			// noop
		} else {
			m.Traceback()
			panic(fmt.Sprintf("unrecognized syscall: %d", syscallNum))
		}
	}

	exec.HandleSyscallUpdates(&thread.Cpu, &thread.Registers, v0, v1)
	return nil
}

func (m *InstrumentedState) syscallYield(thread *ThreadState) {
	v0 := Word(0)
	v1 := Word(0)
	exec.HandleSyscallUpdates(&thread.Cpu, &thread.Registers, v0, v1)
	m.preemptThread(thread)
}

func (m *InstrumentedState) mipsStep() error {
	err := m.doMipsStep()
	if err != nil {
		return err
	}

	m.assertPostStateChecks()
	return err
}

func (m *InstrumentedState) assertPostStateChecks() {
	activeStack := m.state.getActiveThreadStack()
	if len(activeStack) == 0 {
		panic("post-state active thread stack is empty")
	}
}

func (m *InstrumentedState) doMipsStep() error {
	if m.state.Exited {
		return nil
	}
	m.state.Step += 1
	thread := m.state.GetCurrentThread()

	if thread.Exited {
		m.popThread()
		m.stackTracker.DropThread(thread.ThreadId)
		return nil
	}

	if m.state.StepsSinceLastContextSwitch >= exec.SchedQuantum {
		// Force a context switch as this thread has been active too long
		if m.state.ThreadCount() > 1 {
			// Log if we're hitting our context switch limit - only matters if we have > 1 thread
			if m.log.Enabled(context.Background(), log.LevelTrace) {
				msg := fmt.Sprintf("Thread has reached maximum execution steps (%v) - preempting.", exec.SchedQuantum)
				m.log.Trace(msg, "threadId", thread.ThreadId, "threadCount", m.state.ThreadCount(), "pc", thread.Cpu.PC)
			}
		}
		m.preemptThread(thread)
		m.statsTracker.trackForcedPreemption()
		return nil
	}
	m.state.StepsSinceLastContextSwitch += 1

	//instruction fetch
	insn, opcode, fun := exec.GetInstructionDetails(m.state.GetPC(), m.state.Memory)

	// Handle syscall separately
	// syscall (can read and write)
	if opcode == 0 && fun == 0xC {
		return m.handleSyscall()
	}

	// Handle RMW (read-modify-write) ops
	if opcode == exec.OpLoadLinked || opcode == exec.OpStoreConditional {
		return m.handleRMWOps(insn, opcode)
	}
	if opcode == exec.OpLoadLinked64 || opcode == exec.OpStoreConditional64 {
		if arch.IsMips32 {
			panic(fmt.Sprintf("invalid instruction: %x", insn))
		}
		return m.handleRMWOps(insn, opcode)
	}

	// Exec the rest of the step logic
	memUpdated, effMemAddr, err := exec.ExecMipsCoreStepLogic(m.state.getCpuRef(), m.state.GetRegistersRef(), m.state.Memory, insn, opcode, fun, m.memoryTracker, m.stackTracker)
	if err != nil {
		return err
	}
	if memUpdated {
		m.handleMemoryUpdate(effMemAddr)
	}

	return nil
}

func (m *InstrumentedState) handleMemoryUpdate(effMemAddr Word) {
	if effMemAddr == (arch.AddressMask & m.state.LLAddress) {
		// Reserved address was modified, clear the reservation
		m.clearLLMemoryReservation()
		m.statsTracker.trackReservationInvalidation()
	}
}

func (m *InstrumentedState) clearLLMemoryReservation() {
	m.state.LLReservationStatus = LLStatusNone
	m.state.LLAddress = 0
	m.state.LLOwnerThread = 0
}

// handleRMWOps handles LL and SC operations which provide the primitives to implement read-modify-write operations
func (m *InstrumentedState) handleRMWOps(insn, opcode uint32) error {
	baseReg := (insn >> 21) & 0x1F
	base := m.state.GetRegistersRef()[baseReg]
	rtReg := Word((insn >> 16) & 0x1F)
	offset := exec.SignExtendImmediate(insn)
	addr := base + offset

	// Determine some opcode-specific parameters
	targetStatus := LLStatusActive32bit
	byteLength := Word(4)
	if opcode == exec.OpLoadLinked64 || opcode == exec.OpStoreConditional64 {
		// Use 64-bit params
		targetStatus = LLStatusActive64bit
		byteLength = Word(8)
	}

	var retVal Word
	threadId := m.state.GetCurrentThread().ThreadId
	switch opcode {
	case exec.OpLoadLinked, exec.OpLoadLinked64:
		retVal = exec.LoadSubWord(m.state.GetMemory(), addr, byteLength, true, m.memoryTracker)

		m.state.LLReservationStatus = targetStatus
		m.state.LLAddress = addr
		m.state.LLOwnerThread = threadId

		m.statsTracker.trackLL(threadId, m.GetState().GetStep())
	case exec.OpStoreConditional, exec.OpStoreConditional64:
		if m.state.LLReservationStatus == targetStatus && m.state.LLOwnerThread == threadId && m.state.LLAddress == addr {
			// Complete atomic update: set memory and return 1 for success
			m.clearLLMemoryReservation()

			val := m.state.GetRegistersRef()[rtReg]
			exec.StoreSubWord(m.state.GetMemory(), addr, byteLength, val, m.memoryTracker)

			retVal = 1

			m.statsTracker.trackSCSuccess(threadId, m.GetState().GetStep())
		} else {
			// Atomic update failed, return 0 for failure
			retVal = 0

			m.statsTracker.trackSCFailure(threadId, m.GetState().GetStep())
		}
	default:
		panic(fmt.Sprintf("Invalid instruction passed to handleRMWOps (opcode %08x)", opcode))
	}

	return exec.HandleRd(m.state.getCpuRef(), m.state.GetRegistersRef(), rtReg, retVal, true)
}

func (m *InstrumentedState) preemptThread(thread *ThreadState) bool {
	// Pop thread from the current stack and push to the other stack
	if m.state.TraverseRight {
		rtThreadCnt := len(m.state.RightThreadStack)
		if rtThreadCnt == 0 {
			panic("empty right thread stack")
		}
		m.state.RightThreadStack = m.state.RightThreadStack[:rtThreadCnt-1]
		m.state.LeftThreadStack = append(m.state.LeftThreadStack, thread)
	} else {
		lftThreadCnt := len(m.state.LeftThreadStack)
		if lftThreadCnt == 0 {
			panic("empty left thread stack")
		}
		m.state.LeftThreadStack = m.state.LeftThreadStack[:lftThreadCnt-1]
		m.state.RightThreadStack = append(m.state.RightThreadStack, thread)
	}

	changeDirections := false
	current := m.state.getActiveThreadStack()
	if len(current) == 0 {
		m.state.TraverseRight = !m.state.TraverseRight
		changeDirections = true
	}

	m.state.StepsSinceLastContextSwitch = 0

	m.statsTracker.trackThreadActivated(m.state.GetCurrentThread().ThreadId, m.state.GetStep())
	return changeDirections
}

func (m *InstrumentedState) pushThread(thread *ThreadState) {
	if m.state.TraverseRight {
		m.state.RightThreadStack = append(m.state.RightThreadStack, thread)
	} else {
		m.state.LeftThreadStack = append(m.state.LeftThreadStack, thread)
	}
	m.state.StepsSinceLastContextSwitch = 0
}

func (m *InstrumentedState) popThread() {
	if m.state.TraverseRight {
		m.state.RightThreadStack = m.state.RightThreadStack[:len(m.state.RightThreadStack)-1]
	} else {
		m.state.LeftThreadStack = m.state.LeftThreadStack[:len(m.state.LeftThreadStack)-1]
	}

	current := m.state.getActiveThreadStack()
	if len(current) == 0 {
		m.state.TraverseRight = !m.state.TraverseRight
	}
	m.state.StepsSinceLastContextSwitch = 0
}

func (m *InstrumentedState) lastThreadRemaining() bool {
	return m.state.ThreadCount() == 1
}

func (m *InstrumentedState) getFutexValue(vAddr Word) uint32 {
	subword := exec.LoadSubWord(m.state.GetMemory(), vAddr, Word(4), false, m.memoryTracker)
	return uint32(subword)
}
