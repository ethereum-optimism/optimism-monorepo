//go:build cannon64
// +build cannon64

package arch

import "encoding/binary"

type (
	// Word differs from the tradditional meaning in MIPS. The type represents the *maximum* architecture specific access length and value sizes
	Word = uint64
	// SignedInteger specifies the maximum signed integer type used for arithmetic.
	SignedInteger = int64
)

const (
	IsMips32      = false
	WordSize      = 64
	WordSizeBytes = WordSize >> 3
	PageAddrSize  = 12
	PageKeySize   = WordSize - PageAddrSize

	MemProofLeafCount = 60
	MemProofSize      = MemProofLeafCount * 32

	AddressMask = 0xFFFFFFFFFFFFFFF8
	ExtMask     = 0x7

	// Ensure virtual address is limited to 48-bits as many user programs assume such to implement packed pointers
	// limit          0x00_00_FF_FF_FF_FF_FF_FF
	HeapStart       = 0x00_00_10_00_00_00_00_00
	HeapEnd         = 0x00_00_60_00_00_00_00_00
	ProgramBreak    = 0x00_00_40_00_00_00_00_00
	HighMemoryStart = 0x00_00_7F_FF_FF_FF_F0_00
)

// MIPS64 syscall table - https://github.com/torvalds/linux/blob/3efc57369a0ce8f76bf0804f7e673982384e4ac9/arch/mips/kernel/syscalls/syscall_n64.tbl. Generate the syscall numbers using the Makefile in that directory.
// See https://gpages.juszkiewicz.com.pl/syscalls-table/syscalls.html for the generated syscalls

// 64-bit Syscall numbers - new
const (
	SysMmap         = 5009
	SysBrk          = 5012
	SysClone        = 5055
	SysExitGroup    = 5205
	SysRead         = 5000
	SysWrite        = 5001
	SysFcntl        = 5070
	SysExit         = 5058
	SysSchedYield   = 5023
	SysGetTID       = 5178
	SysFutex        = 5194
	SysOpen         = 5002
	SysNanosleep    = 5034
	SysClockGetTime = 5222
	SysGetpid       = 5038
)

// Noop Syscall numbers
const (
	// UndefinedSysNr is the value used for 32-bit syscall numbers that aren't supported for 64-bits
	UndefinedSysNr = ^Word(0)

	SysMunmap        = 5011
	SysGetAffinity   = 5196
	SysMadvise       = 5027
	SysRtSigprocmask = 5014
	SysSigaltstack   = 5129
	SysRtSigaction   = 5013
	SysPrlimit64     = 5297
	SysClose         = 5003
	SysPread64       = 5016
	SysStat          = 5004
	SysFstat         = 5005
	SysFstat64       = UndefinedSysNr
	SysOpenAt        = 5247
	SysReadlink      = 5087
	SysReadlinkAt    = 5257
	SysIoctl         = 5015
	SysEpollCreate1  = 5285
	SysPipe2         = 5287
	SysEpollCtl      = 5208
	SysEpollPwait    = 5272
	SysGetRandom     = 5313
	SysUname         = 5061
	SysStat64        = UndefinedSysNr
	SysGetuid        = 5100
	SysGetgid        = 5102
	SysLlseek        = UndefinedSysNr
	SysMinCore       = 5026
	SysTgkill        = 5225
	SysGetRLimit     = 5095
	SysLseek         = 5008
	// Profiling-related syscalls
	SysSetITimer    = 5036
	SysTimerCreate  = 5216
	SysTimerSetTime = 5217
	SysTimerDelete  = 5220
)

var ByteOrderWord = byteOrder64{}

type byteOrder64 struct{}

func (bo byteOrder64) Word(b []byte) Word {
	return binary.BigEndian.Uint64(b)
}

func (bo byteOrder64) AppendWord(b []byte, v uint64) []byte {
	return binary.BigEndian.AppendUint64(b, v)
}

func (bo byteOrder64) PutWord(b []byte, v uint64) {
	binary.BigEndian.PutUint64(b, v)
}
