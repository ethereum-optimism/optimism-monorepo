package singlethreaded

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/exec"
)

type Word = arch.Word

func (m *InstrumentedState) handleSyscall() error {
	syscallNum, a0, a1, a2 := exec.GetSyscallArgs(&m.state.Registers)

	v0 := Word(0)
	v1 := Word(0)

	//fmt.Printf("syscall: %d\n", syscallNum)
	switch syscallNum {
	case arch.SysMmap:
		var newHeap Word
		v0, v1, newHeap = exec.HandleSysMmap(a0, a1, m.state.Heap)
		m.state.Heap = newHeap
	case arch.SysBrk:
		v0 = arch.ProgramBreak
	case arch.SysClone: // clone (not supported)
		v0 = 1
	case arch.SysExitGroup:
		m.state.Exited = true
		m.state.ExitCode = uint8(a0)
		return nil
	case arch.SysRead:
		var newPreimageOffset Word
		v0, v1, newPreimageOffset, _, _ = exec.HandleSysRead(a0, a1, a2, m.state.PreimageKey, m.state.PreimageOffset, m.preimageOracle, m.state.Memory, m.memoryTracker)
		m.state.PreimageOffset = newPreimageOffset
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
	}

	exec.HandleSyscallUpdates(&m.state.Cpu, &m.state.Registers, v0, v1)
	return nil
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

	// Handle RMW (read-modify-write) ops
	if opcode == exec.OpLoadLinked || opcode == exec.OpStoreConditional {
		return m.handleRMWOps(insn, opcode)
	}

	// Exec the rest of the step logic
	_, _, err := exec.ExecMipsCoreStepLogic(&m.state.Cpu, &m.state.Registers, m.state.Memory, insn, opcode, fun, m.memoryTracker, m.stackTracker)
	return err
}

// handleRMWOps handles LL and SC operations which provide the primitives to implement read-modify-write operations
func (m *InstrumentedState) handleRMWOps(insn, opcode uint32) error {
	baseReg := (insn >> 21) & 0x1F
	base := m.state.Registers[baseReg]
	rtReg := Word((insn >> 16) & 0x1F)
	offset := exec.SignExtendImmediate(insn)

	effAddr := (base + offset) & arch.AddressMask
	m.memoryTracker.TrackMemAccess(effAddr)
	mem := m.state.Memory.GetWord(effAddr)

	var retVal Word
	if opcode == exec.OpLoadLinked {
		retVal = mem
	} else if opcode == exec.OpStoreConditional {
		rt := m.state.Registers[rtReg]
		m.state.Memory.SetWord(effAddr, rt)
		retVal = 1 // 1 for success
	} else {
		panic(fmt.Sprintf("Invalid instruction passed to handleRMWOps (opcode %08x)", opcode))
	}

	return exec.HandleRd(&m.state.Cpu, &m.state.Registers, rtReg, retVal, true)
}
