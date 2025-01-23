package exec

import (
	"encoding/binary"
	"io"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/memory"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/program"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/register"
)

type Word = arch.Word

const (
	AddressMask = arch.AddressMask
)

// File descriptors
const (
	FdStdin         = 0
	FdStdout        = 1
	FdStderr        = 2
	FdHintRead      = 3
	FdHintWrite     = 4
	FdPreimageRead  = 5
	FdPreimageWrite = 6
)

// Errors
const (
	SysErrorSignal = ^Word(0)
	MipsEBADF      = 0x9
	MipsEINVAL     = 0x16
	MipsEAGAIN     = 0xb
	MipsETIMEDOUT  = 0x91
)

// SysFutex-related constants
const (
	FutexWaitPrivate = 128
	FutexWakePrivate = 129
)

// SysClone flags
// Handling is meant to support go runtime use cases
// Pulled from: https://github.com/golang/go/blob/go1.21.3/src/runtime/os_linux.go#L124-L158
const (
	CloneVm            = 0x100
	CloneFs            = 0x200
	CloneFiles         = 0x400
	CloneSighand       = 0x800
	ClonePtrace        = 0x2000
	CloneVfork         = 0x4000
	CloneParent        = 0x8000
	CloneThread        = 0x10000
	CloneNewns         = 0x20000
	CloneSysvsem       = 0x40000
	CloneSettls        = 0x80000
	CloneParentSettid  = 0x100000
	CloneChildCleartid = 0x200000
	CloneUntraced      = 0x800000
	CloneChildSettid   = 0x1000000
	CloneStopped       = 0x2000000
	CloneNewuts        = 0x4000000
	CloneNewipc        = 0x8000000

	ValidCloneFlags = CloneVm |
		CloneFs |
		CloneFiles |
		CloneSighand |
		CloneSysvsem |
		CloneThread
)

// Other constants
const (
	// SchedQuantum is the number of steps dedicated for a thread before it's preempted. Effectively used to emulate thread "time slices"
	SchedQuantum = 100_000

	// HZ is the assumed clock rate of an emulated MIPS32 CPU.
	// The value of HZ is a rough estimate of the Cannon instruction count / second on a typical machine.
	// HZ is used to emulate the clock_gettime syscall used by guest programs that have a Go runtime.
	// The Go runtime consumes the system time to determine when to initiate gc assists and for goroutine scheduling.
	// A HZ value that is too low (i.e. lower than the emulation speed) results in the main goroutine attempting to assist with GC more often.
	// Adjust this value accordingly as the emulation speed changes. The HZ value should be within the same order of magnitude as the emulation speed.
	HZ = 10_000_000

	// ClockGettimeRealtimeFlag is the clock_gettime clock id for Linux's realtime clock: https://github.com/torvalds/linux/blob/ad618736883b8970f66af799e34007475fe33a68/include/uapi/linux/time.h#L49
	ClockGettimeRealtimeFlag = 0
	// ClockGettimeMonotonicFlag is the clock_gettime clock id for Linux's monotonic clock: https://github.com/torvalds/linux/blob/ad618736883b8970f66af799e34007475fe33a68/include/uapi/linux/time.h#L50
	ClockGettimeMonotonicFlag = 1
)

func GetSyscallArgs(registers *[32]Word) (syscallNum, a0, a1, a2 Word) {
	syscallNum = registers[register.RegSyscallNum] // v0

	a0 = registers[register.RegSyscallParam1]
	a1 = registers[register.RegSyscallParam2]
	a2 = registers[register.RegSyscallParam3]

	return syscallNum, a0, a1, a2
}

func HandleSysMmap(a0, a1, heap Word) (v0, v1, newHeap Word) {
	v1 = Word(0)
	newHeap = heap

	sz := a1
	if sz&memory.PageAddrMask != 0 { // adjust size to align with page size
		sz += memory.PageSize - (sz & memory.PageAddrMask)
	}
	if a0 == 0 {
		v0 = heap
		//fmt.Printf("mmap heap 0x%x size 0x%x\n", v0, sz)
		newHeap += sz
		// Fail if new heap exceeds memory limit, newHeap overflows around to low memory, or sz overflows
		if newHeap > program.HEAP_END || newHeap < heap || sz < a1 {
			v0 = SysErrorSignal
			v1 = MipsEINVAL
			return v0, v1, heap
		}
	} else {
		v0 = a0
		//fmt.Printf("mmap hint 0x%x size 0x%x\n", v0, sz)
	}

	return v0, v1, newHeap
}

func HandleSysRead(
	a0, a1, a2 Word,
	preimageKey [32]byte,
	preimageOffset Word,
	preimageReader PreimageReader,
	memory *memory.Memory,
	memTracker MemTracker,
) (v0, v1, newPreimageOffset Word, memUpdated bool, memAddr Word) {
	// args: a0 = fd, a1 = addr, a2 = count
	// returns: v0 = read, v1 = err code
	v0 = Word(0)
	v1 = Word(0)
	newPreimageOffset = preimageOffset

	switch a0 {
	case FdStdin:
		// leave v0 and v1 zero: read nothing, no error
	case FdPreimageRead: // pre-image oracle
		effAddr := a1 & AddressMask
		memTracker.TrackMemAccess(effAddr)
		mem := memory.GetWord(effAddr)
		dat, datLen := preimageReader.ReadPreimage(preimageKey, preimageOffset)
		//fmt.Printf("reading pre-image data: addr: %08x, offset: %d, datLen: %d, data: %x, key: %s  count: %d\n", a1, preimageOffset, datLen, dat[:datLen], preimageKey, a2)
		alignment := a1 & arch.ExtMask
		space := arch.WordSizeBytes - alignment
		if space < datLen {
			datLen = space
		}
		if a2 < datLen {
			datLen = a2
		}
		var outMem [arch.WordSizeBytes]byte
		arch.ByteOrderWord.PutWord(outMem[:], mem)
		copy(outMem[alignment:], dat[:datLen])
		memory.SetWord(effAddr, arch.ByteOrderWord.Word(outMem[:]))
		memUpdated = true
		memAddr = effAddr
		newPreimageOffset += datLen
		v0 = datLen
		//fmt.Printf("read %d pre-image bytes, new offset: %d, eff addr: %08x mem: %08x\n", datLen, m.state.PreimageOffset, effAddr, outMem)
	case FdHintRead: // hint response
		// don't actually read into memory, just say we read it all, we ignore the result anyway
		v0 = a2
	default:
		v0 = ^Word(0)
		v1 = MipsEBADF
	}

	return v0, v1, newPreimageOffset, memUpdated, memAddr
}

func HandleSysWrite(a0, a1, a2 Word,
	lastHint hexutil.Bytes,
	preimageKey [32]byte,
	preimageOffset Word,
	oracle mipsevm.PreimageOracle,
	memory *memory.Memory,
	memTracker MemTracker,
	stdOut, stdErr io.Writer,
) (v0, v1 Word, newLastHint hexutil.Bytes, newPreimageKey common.Hash, newPreimageOffset Word) {
	// args: a0 = fd, a1 = addr, a2 = count
	// returns: v0 = written, v1 = err code
	v1 = Word(0)
	newLastHint = lastHint
	newPreimageKey = preimageKey
	newPreimageOffset = preimageOffset

	switch a0 {
	case FdStdout:
		_, _ = io.Copy(stdOut, memory.ReadMemoryRange(a1, a2))
		v0 = a2
	case FdStderr:
		_, _ = io.Copy(stdErr, memory.ReadMemoryRange(a1, a2))
		v0 = a2
	case FdHintWrite:
		hintData, _ := io.ReadAll(memory.ReadMemoryRange(a1, a2))
		lastHint = append(lastHint, hintData...)
		for len(lastHint) >= 4 { // process while there is enough data to check if there are any hints
			hintLen := binary.BigEndian.Uint32(lastHint[:4])
			if hintLen <= uint32(len(lastHint[4:])) {
				hint := lastHint[4 : 4+hintLen] // without the length prefix
				lastHint = lastHint[4+hintLen:]
				oracle.Hint(hint)
			} else {
				break // stop processing hints if there is incomplete data buffered
			}
		}
		newLastHint = lastHint
		v0 = a2
	case FdPreimageWrite:
		effAddr := a1 & arch.AddressMask
		memTracker.TrackMemAccess(effAddr)
		mem := memory.GetWord(effAddr)
		key := preimageKey
		alignment := a1 & arch.ExtMask
		space := arch.WordSizeBytes - alignment
		if space < a2 {
			a2 = space
		}
		copy(key[:], key[a2:])
		var tmp [arch.WordSizeBytes]byte
		arch.ByteOrderWord.PutWord(tmp[:], mem)
		copy(key[32-a2:], tmp[alignment:])
		newPreimageKey = key
		newPreimageOffset = 0
		//fmt.Printf("updating pre-image key: %s\n", m.state.PreimageKey)
		v0 = a2
	default:
		v0 = ^Word(0)
		v1 = MipsEBADF
	}

	return v0, v1, newLastHint, newPreimageKey, newPreimageOffset
}

func HandleSysFcntl(a0, a1 Word) (v0, v1 Word) {
	// args: a0 = fd, a1 = cmd
	v1 = Word(0)

	if a1 == 1 { // F_GETFD: get file descriptor flags
		switch a0 {
		case FdStdin, FdStdout, FdStderr, FdPreimageRead, FdHintRead, FdPreimageWrite, FdHintWrite:
			v0 = 0 // No flags set
		default:
			v0 = ^Word(0)
			v1 = MipsEBADF
		}
	} else if a1 == 3 { // F_GETFL: get file status flags
		switch a0 {
		case FdStdin, FdPreimageRead, FdHintRead:
			v0 = 0 // O_RDONLY
		case FdStdout, FdStderr, FdPreimageWrite, FdHintWrite:
			v0 = 1 // O_WRONLY
		default:
			v0 = ^Word(0)
			v1 = MipsEBADF
		}
	} else {
		v0 = ^Word(0)
		v1 = MipsEINVAL // cmd not recognized by this kernel
	}

	return v0, v1
}

func HandleSyscallUpdates(cpu *mipsevm.CpuScalars, registers *[32]Word, v0, v1 Word) {
	registers[register.RegSyscallRet1] = v0
	registers[register.RegSyscallErrno] = v1

	cpu.PC = cpu.NextPC
	cpu.NextPC = cpu.NextPC + 4
}
