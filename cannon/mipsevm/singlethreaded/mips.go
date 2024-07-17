package singlethreaded

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm/exec"
)

func (m *InstrumentedState) handleSyscall() error {
	syscallNum, a0, a1, a2 := exec.GetSyscallArgs(&m.state.Registers)

	v0 := uint32(0)
	v1 := uint32(0)

	//fmt.Printf("syscall: %d\n", syscallNum)
	switch syscallNum {
	case exec.SysMmap:
		var newHeap uint32
		v0, v1, newHeap = exec.HandleSysMmap(a0, a1, m.state.Heap)
		m.state.Heap = newHeap
	case exec.SysBrk:
		v0 = 0x40000000
	case exec.SysClone: // clone (not supported)
		v0 = 1
	case exec.SysExitGroup:
		m.state.Exited = true
		m.state.ExitCode = uint8(a0)
		return nil
	case exec.SysRead:
		var newPreimageOffset uint32
		v0, v1, newPreimageOffset = exec.HandleSysRead(a0, a1, a2, m.state.PreimageKey, m.state.PreimageOffset, m.preimageOracle, m.state.Memory, m.memoryTracker)
		m.state.PreimageOffset = newPreimageOffset
	case exec.SysWrite:
		var newLastHint hexutil.Bytes
		var newPreimageKey common.Hash
		var newPreimageOffset uint32
		v0, v1, newLastHint, newPreimageKey, newPreimageOffset = exec.HandleSysWrite(a0, a1, a2, m.state.LastHint, m.state.PreimageKey, m.state.PreimageOffset, m.preimageOracle, m.state.Memory, m.memoryTracker, m.stdOut, m.stdErr)
		m.state.LastHint = newLastHint
		m.state.PreimageKey = newPreimageKey
		m.state.PreimageOffset = newPreimageOffset
	case exec.SysFcntl:
		v0, v1 = exec.HandleSysFcntl(a0, a1)
	}

	exec.HandleSyscallUpdates(&m.state.Cpu, &m.state.Registers, v0, v1)
	return nil
}

func (m *InstrumentedState) PushStack(target uint32) {
	if !m.debugEnabled {
		return
	}
	m.debug.stack = append(m.debug.stack, target)
	m.debug.caller = append(m.debug.caller, m.state.Cpu.PC)
}

func (m *InstrumentedState) PopStack() {
	if !m.debugEnabled {
		return
	}
	if len(m.debug.stack) != 0 {
		fn := m.debug.meta.LookupSymbol(m.state.Cpu.PC)
		topFn := m.debug.meta.LookupSymbol(m.debug.stack[len(m.debug.stack)-1])
		if fn != topFn {
			// most likely the function was inlined. Snap back to the last return.
			i := len(m.debug.stack) - 1
			for ; i >= 0; i-- {
				if m.debug.meta.LookupSymbol(m.debug.stack[i]) == fn {
					m.debug.stack = m.debug.stack[:i]
					m.debug.caller = m.debug.caller[:i]
					break
				}
			}
		} else {
			m.debug.stack = m.debug.stack[:len(m.debug.stack)-1]
			m.debug.caller = m.debug.caller[:len(m.debug.caller)-1]
		}
	} else {
		fmt.Printf("ERROR: stack underflow at pc=%x. step=%d\n", m.state.Cpu.PC, m.state.Step)
	}
}

func (m *InstrumentedState) Traceback() {
	fmt.Printf("traceback at pc=%x. step=%d\n", m.state.Cpu.PC, m.state.Step)
	for i := len(m.debug.stack) - 1; i >= 0; i-- {
		s := m.debug.stack[i]
		idx := len(m.debug.stack) - i - 1
		fmt.Printf("\t%d %x in %s caller=%08x\n", idx, s, m.debug.meta.LookupSymbol(s), m.debug.caller[i])
	}
}

func (m *InstrumentedState) mipsStep() error {
	if m.state.Exited {
		return nil
	}
	m.state.Step += 1
	// instruction fetch
	insn, opcode, fun := exec.GetInstructionDetails(m.state.Cpu.PC, m.state.Memory)

	// Handle syscall separately
	// syscall (can read and write)
	if opcode == 0 && fun == 0xC {
		return m.handleSyscall()
	}

	// Exec the rest of the step logic
	return exec.ExecMipsCoreStepLogic(&m.state.Cpu, &m.state.Registers, m.state.Memory, insn, opcode, fun, m.memoryTracker, m)
}
