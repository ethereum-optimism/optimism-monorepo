//go:build !cannon64
// +build !cannon64

package arch

import "encoding/binary"

type (
	// Word differs from the tradditional meaning in MIPS. The type represents the *maximum* architecture specific access length and value sizes.
	Word = uint32
	// SignedInteger specifies the maximum signed integer type used for arithmetic.
	SignedInteger = int32
)

const (
	IsMips32      = true
	WordSize      = 32
	WordSizeBytes = WordSize >> 3
	PageAddrSize  = 12
	PageKeySize   = WordSize - PageAddrSize

	MemProofLeafCount = 28
	MemProofSize      = MemProofLeafCount * 32

	AddressMask = 0xFFffFFfc
	ExtMask     = 0x3

	HeapStart       = 0x05_00_00_00
	HeapEnd         = 0x60_00_00_00
	ProgramBreak    = 0x40_00_00_00
	HighMemoryStart = 0x7f_ff_d0_00
)

// 32-bit Syscall codes
const (
	SysMmap         = 4090
	SysBrk          = 4045
	SysClone        = 4120
	SysExitGroup    = 4246
	SysRead         = 4003
	SysWrite        = 4004
	SysFcntl        = 4055
	SysExit         = 4001
	SysSchedYield   = 4162
	SysGetTID       = 4222
	SysFutex        = 4238
	SysOpen         = 4005
	SysNanosleep    = 4166
	SysClockGetTime = 4263
	SysGetpid       = 4020
)

// Noop Syscall codes
const (
	SysMunmap        = 4091
	SysGetAffinity   = 4240
	SysMadvise       = 4218
	SysRtSigprocmask = 4195
	SysSigaltstack   = 4206
	SysRtSigaction   = 4194
	SysPrlimit64     = 4338
	SysClose         = 4006
	SysPread64       = 4200
	SysStat          = 4106
	SysFstat         = 4108
	SysFstat64       = 4215
	SysOpenAt        = 4288
	SysReadlink      = 4085
	SysReadlinkAt    = 4298
	SysIoctl         = 4054
	SysEpollCreate1  = 4326
	SysPipe2         = 4328
	SysEpollCtl      = 4249
	SysEpollPwait    = 4313
	SysGetRandom     = 4353
	SysUname         = 4122
	SysStat64        = 4213
	SysGetuid        = 4024
	SysGetgid        = 4047
	SysLlseek        = 4140
	SysMinCore       = 4217
	SysTgkill        = 4266
	SysGetRLimit     = 4076
	SysLseek         = 4019
	// Profiling-related syscalls
	SysSetITimer    = 4104
	SysTimerCreate  = 4257
	SysTimerSetTime = 4258
	SysTimerDelete  = 4261
)

var ByteOrderWord = byteOrder32{}

type byteOrder32 struct{}

func (bo byteOrder32) Word(b []byte) Word {
	return binary.BigEndian.Uint32(b)
}

func (bo byteOrder32) AppendWord(b []byte, v uint32) []byte {
	return binary.BigEndian.AppendUint32(b, v)
}

func (bo byteOrder32) PutWord(b []byte, v uint32) {
	binary.BigEndian.PutUint32(b, v)
}
