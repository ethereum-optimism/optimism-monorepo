package register

// FYI: https://en.wikibooks.org/wiki/MIPS_Assembly/Register_File
//
//	https://refspecs.linuxfoundation.org/elf/mipsabi.pdf
const (
	// syscall number; 1st return value
	RegV0 = 2
	// syscall arguments; returned unmodified
	RegA0 = 4
	RegA1 = 5
	RegA2 = 6
	// 4th syscall argument; set to 0/1 for success/error
	RegA3 = 7
	// Stack pointer
	RegSP = 29
)

// FYI: https://web.archive.org/web/20231223163047/https://www.linux-mips.org/wiki/Syscall

const (
	RegSyscallNum    = RegV0
	RegSyscallErrno  = RegA3
	RegSyscallRet1   = RegV0
	RegSyscallParam1 = RegA0
	RegSyscallParam2 = RegA1
	RegSyscallParam3 = RegA2
)
