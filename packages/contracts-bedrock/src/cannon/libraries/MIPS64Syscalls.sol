// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { MIPS64Memory } from "src/cannon/libraries/MIPS64Memory.sol";
import { MIPS64State as st } from "src/cannon/libraries/MIPS64State.sol";
import { IPreimageOracle } from "src/cannon/interfaces/IPreimageOracle.sol";
import { PreimageKeyLib } from "src/cannon/PreimageKeyLib.sol";
import { MIPS64Arch as arch } from "src/cannon/libraries/MIPS64Arch.sol";

library MIPS64Syscalls {
    struct SysReadParams {
        /// @param _a0 The file descriptor.
        uint64 a0;
        /// @param _a1 The memory location where data should be read to.
        uint64 a1;
        /// @param _a2 The number of bytes to read from the file
        uint64 a2;
        /// @param _preimageKey The key of the preimage to read.
        bytes32 preimageKey;
        /// @param _preimageOffset The offset of the preimage to read.
        uint64 preimageOffset;
        /// @param _localContext The local context for the preimage key.
        bytes32 localContext;
        /// @param _oracle The address of the preimage oracle.
        IPreimageOracle oracle;
        /// @param _proofOffset The offset of the memory proof in calldata.
        uint256 proofOffset;
        /// @param _memRoot The current memory root.
        bytes32 memRoot;
    }

    uint64 internal constant U64_MASK = 0xFFffFFffFFffFFff;
    uint64 internal constant PAGE_ADDR_MASK = 4095;
    uint64 internal constant PAGE_SIZE = 4096;

    uint32 internal constant SYS_MMAP = 5009;
    uint32 internal constant SYS_BRK = 5012;
    uint32 internal constant SYS_CLONE = 5055;
    uint32 internal constant SYS_EXIT_GROUP = 5205;
    uint32 internal constant SYS_READ = 5000;
    uint32 internal constant SYS_WRITE = 5001;
    uint32 internal constant SYS_FCNTL = 5070;
    uint32 internal constant SYS_EXIT = 5058;
    uint32 internal constant SYS_SCHED_YIELD = 5023;
    uint32 internal constant SYS_GETTID = 5178;
    uint32 internal constant SYS_FUTEX = 5194;
    uint32 internal constant SYS_OPEN = 5002;
    uint32 internal constant SYS_NANOSLEEP = 5034;
    uint32 internal constant SYS_CLOCKGETTIME = 5222;
    uint32 internal constant SYS_GETPID = 5038;
    // no-op syscalls
    uint32 internal constant SYS_MUNMAP = 5011;
    uint32 internal constant SYS_GETAFFINITY = 5196;
    uint32 internal constant SYS_MADVISE = 5027;
    uint32 internal constant SYS_RTSIGPROCMASK = 5014;
    uint32 internal constant SYS_SIGALTSTACK = 5129;
    uint32 internal constant SYS_RTSIGACTION = 5013;
    uint32 internal constant SYS_PRLIMIT64 = 5297;
    uint32 internal constant SYS_CLOSE = 5003;
    uint32 internal constant SYS_PREAD64 = 5016;
    uint32 internal constant SYS_STAT = 5004;
    uint32 internal constant SYS_FSTAT = 5005;
    //uint32 internal constant SYS_FSTAT64 = 0xFFFFFFFF;  // UndefinedSysNr - not supported by MIPS64
    uint32 internal constant SYS_OPENAT = 5247;
    uint32 internal constant SYS_READLINK = 5087;
    uint32 internal constant SYS_READLINKAT = 5257;
    uint32 internal constant SYS_IOCTL = 5015;
    uint32 internal constant SYS_EPOLLCREATE1 = 5285;
    uint32 internal constant SYS_PIPE2 = 5287;
    uint32 internal constant SYS_EPOLLCTL = 5208;
    uint32 internal constant SYS_EPOLLPWAIT = 5272;
    uint32 internal constant SYS_GETRANDOM = 5313;
    uint32 internal constant SYS_UNAME = 5061;
    //uint32 internal constant SYS_STAT64 = 0xFFFFFFFF;  // UndefinedSysNr - not supported by MIPS64
    uint32 internal constant SYS_GETUID = 5100;
    uint32 internal constant SYS_GETGID = 5102;
    //uint32 internal constant SYS_LLSEEK = 0xFFFFFFFF;  // UndefinedSysNr - not supported by MIPS64
    uint32 internal constant SYS_MINCORE = 5026;
    uint32 internal constant SYS_TGKILL = 5225;
    uint32 internal constant SYS_GETRLIMIT = 5095;
    uint32 internal constant SYS_LSEEK = 5008;
    // profiling-related syscalls - ignored
    uint32 internal constant SYS_SETITIMER = 5036;
    uint32 internal constant SYS_TIMERCREATE = 5216;
    uint32 internal constant SYS_TIMERSETTIME = 5217;
    uint32 internal constant SYS_TIMERDELETE = 5220;

    uint32 internal constant FD_STDIN = 0;
    uint32 internal constant FD_STDOUT = 1;
    uint32 internal constant FD_STDERR = 2;
    uint32 internal constant FD_HINT_READ = 3;
    uint32 internal constant FD_HINT_WRITE = 4;
    uint32 internal constant FD_PREIMAGE_READ = 5;
    uint32 internal constant FD_PREIMAGE_WRITE = 6;

    uint64 internal constant SYS_ERROR_SIGNAL = U64_MASK;
    uint64 internal constant EBADF = 0x9;
    uint64 internal constant EINVAL = 0x16;
    uint64 internal constant EAGAIN = 0xb;
    uint64 internal constant ETIMEDOUT = 0x91;

    uint64 internal constant FUTEX_WAIT_PRIVATE = 128;
    uint64 internal constant FUTEX_WAKE_PRIVATE = 129;
    uint64 internal constant FUTEX_TIMEOUT_STEPS = 10000;
    uint64 internal constant FUTEX_NO_TIMEOUT = type(uint64).max;
    uint64 internal constant FUTEX_EMPTY_ADDR = U64_MASK;

    uint64 internal constant SCHED_QUANTUM = 100_000;
    uint64 internal constant HZ = 10_000_000;
    uint64 internal constant CLOCK_GETTIME_REALTIME_FLAG = 0;
    uint64 internal constant CLOCK_GETTIME_MONOTONIC_FLAG = 1;
    /// @notice Start of the data segment.
    uint64 internal constant PROGRAM_BREAK = 0x00_00_40_00_00_00_00_00;
    uint64 internal constant HEAP_END = 0x00_00_60_00_00_00_00_00;

    // SYS_CLONE flags
    uint64 internal constant CLONE_VM = 0x100;
    uint64 internal constant CLONE_FS = 0x200;
    uint64 internal constant CLONE_FILES = 0x400;
    uint64 internal constant CLONE_SIGHAND = 0x800;
    uint64 internal constant CLONE_PTRACE = 0x2000;
    uint64 internal constant CLONE_VFORK = 0x4000;
    uint64 internal constant CLONE_PARENT = 0x8000;
    uint64 internal constant CLONE_THREAD = 0x10000;
    uint64 internal constant CLONE_NEWNS = 0x20000;
    uint64 internal constant CLONE_SYSVSEM = 0x40000;
    uint64 internal constant CLONE_SETTLS = 0x80000;
    uint64 internal constant CLONE_PARENTSETTID = 0x100000;
    uint64 internal constant CLONE_CHILDCLEARTID = 0x200000;
    uint64 internal constant CLONE_UNTRACED = 0x800000;
    uint64 internal constant CLONE_CHILDSETTID = 0x1000000;
    uint64 internal constant CLONE_STOPPED = 0x2000000;
    uint64 internal constant CLONE_NEWUTS = 0x4000000;
    uint64 internal constant CLONE_NEWIPC = 0x8000000;
    uint64 internal constant VALID_SYS_CLONE_FLAGS =
        CLONE_VM | CLONE_FS | CLONE_FILES | CLONE_SIGHAND | CLONE_SYSVSEM | CLONE_THREAD;

    // FYI: https://en.wikibooks.org/wiki/MIPS_Assembly/Register_File
    //      https://refspecs.linuxfoundation.org/elf/mipsabi.pdf
    uint32 internal constant REG_V0 = 2;
    uint32 internal constant REG_A0 = 4;
    uint32 internal constant REG_A1 = 5;
    uint32 internal constant REG_A2 = 6;
    uint32 internal constant REG_A3 = 7;

    // FYI: https://web.archive.org/web/20231223163047/https://www.linux-mips.org/wiki/Syscall
    uint32 internal constant REG_SYSCALL_NUM = REG_V0;
    uint32 internal constant REG_SYSCALL_ERRNO = REG_A3;
    uint32 internal constant REG_SYSCALL_RET1 = REG_V0;
    uint32 internal constant REG_SYSCALL_PARAM1 = REG_A0;
    uint32 internal constant REG_SYSCALL_PARAM2 = REG_A1;
    uint32 internal constant REG_SYSCALL_PARAM3 = REG_A2;
    uint32 internal constant REG_SYSCALL_PARAM4 = REG_A3;

    // Constants copied from MIPS64Arch for use in Yul
    uint64 internal constant WORD_SIZE_BYTES = 8;
    uint64 internal constant EXT_MASK = 0x7;

    /// @notice Extract syscall num and arguments from registers.
    /// @param _registers The cpu registers.
    /// @return sysCallNum_ The syscall number.
    /// @return a0_ The first argument available to the syscall operation.
    /// @return a1_ The second argument available to the syscall operation.
    /// @return a2_ The third argument available to the syscall operation.
    /// @return a3_ The fourth argument available to the syscall operation.
    function getSyscallArgs(uint64[32] memory _registers)
        internal
        pure
        returns (uint64 sysCallNum_, uint64 a0_, uint64 a1_, uint64 a2_, uint64 a3_)
    {
        unchecked {
            sysCallNum_ = _registers[REG_SYSCALL_NUM];

            a0_ = _registers[REG_SYSCALL_PARAM1];
            a1_ = _registers[REG_SYSCALL_PARAM2];
            a2_ = _registers[REG_SYSCALL_PARAM3];
            a3_ = _registers[REG_SYSCALL_PARAM4];

            return (sysCallNum_, a0_, a1_, a2_, a3_);
        }
    }

    /// @notice Like a Linux mmap syscall. Allocates a page from the heap.
    /// @param _a0 The address for the new mapping
    /// @param _a1 The size of the new mapping
    /// @param _heap The current value of the heap pointer
    /// @return v0_ The address of the new mapping
    /// @return v1_ Unused error code (0)
    /// @return newHeap_ The new value for the heap, may be unchanged
    function handleSysMmap(
        uint64 _a0,
        uint64 _a1,
        uint64 _heap
    )
        internal
        pure
        returns (uint64 v0_, uint64 v1_, uint64 newHeap_)
    {
        unchecked {
            v1_ = uint64(0);
            newHeap_ = _heap;

            uint64 sz = _a1;
            if (sz & PAGE_ADDR_MASK != 0) {
                // adjust size to align with page size
                sz += PAGE_SIZE - (sz & PAGE_ADDR_MASK);
            }
            if (_a0 == 0) {
                v0_ = _heap;
                newHeap_ += sz;
                // Fail if new heap exceeds memory limit, newHeap overflows to low memory, or sz overflows
                if (newHeap_ > HEAP_END || newHeap_ < _heap || sz < _a1) {
                    v0_ = SYS_ERROR_SIGNAL;
                    v1_ = EINVAL;
                    return (v0_, v1_, _heap);
                }
            } else {
                v0_ = _a0;
            }

            return (v0_, v1_, newHeap_);
        }
    }

    /// @notice Like a Linux read syscall. Splits unaligned reads into aligned reads.
    ///         Args are provided as a struct to reduce stack pressure.
    /// @return v0_ The number of bytes read, -1 on error.
    /// @return v1_ The error code, 0 if there is no error.
    /// @return newPreimageOffset_ The new value for the preimage offset.
    /// @return newMemRoot_ The new memory root.
    function handleSysRead(SysReadParams memory _args)
        internal
        view
        returns (
            uint64 v0_,
            uint64 v1_,
            uint64 newPreimageOffset_,
            bytes32 newMemRoot_,
            bool memUpdated_,
            uint64 memAddr_
        )
    {
        unchecked {
            v0_ = uint64(0);
            v1_ = uint64(0);
            newMemRoot_ = _args.memRoot;
            newPreimageOffset_ = _args.preimageOffset;
            memUpdated_ = false;
            memAddr_ = 0;

            // args: _a0 = fd, _a1 = addr, _a2 = count
            // returns: v0_ = read, v1_ = err code
            if (_args.a0 == FD_STDIN) {
                // Leave v0_ and v1_ zero: read nothing, no error
            }
            // pre-image oracle read
            else if (_args.a0 == FD_PREIMAGE_READ) {
                uint64 effAddr = _args.a1 & arch.ADDRESS_MASK;
                // verify proof is correct, and get the existing memory.
                // mask the addr to align it to 4 bytes
                uint64 mem = MIPS64Memory.readMem(_args.memRoot, effAddr, _args.proofOffset);
                // If the preimage key is a local key, localize it in the context of the caller.
                if (uint8(_args.preimageKey[0]) == 1) {
                    _args.preimageKey = PreimageKeyLib.localize(_args.preimageKey, _args.localContext);
                }
                (bytes32 dat, uint256 datLen) = _args.oracle.readPreimage(_args.preimageKey, _args.preimageOffset);

                // Transform data for writing to memory
                // We use assembly for more precise ops, and no var count limit
                uint64 a1 = _args.a1;
                uint64 a2 = _args.a2;
                assembly {
                    let alignment := and(a1, EXT_MASK) // the read might not start at an aligned address
                    let space := sub(WORD_SIZE_BYTES, alignment) // remaining space in memory word
                    if lt(space, datLen) { datLen := space } // if less space than data, shorten data
                    if lt(a2, datLen) { datLen := a2 } // if requested to read less, read less
                    dat := shr(sub(256, mul(datLen, 8)), dat) // right-align data
                    // position data to insert into memory word
                    dat := shl(mul(sub(sub(WORD_SIZE_BYTES, datLen), alignment), 8), dat)
                    // mask all bytes after start
                    let mask := sub(shl(mul(sub(WORD_SIZE_BYTES, alignment), 8), 1), 1)
                    // mask of all bytes
                    let suffixMask := sub(shl(mul(sub(sub(WORD_SIZE_BYTES, alignment), datLen), 8), 1), 1)
                    // starting from end, maybe none
                    mask := and(mask, not(suffixMask)) // reduce mask to just cover the data we insert
                    mem := or(and(mem, not(mask)), dat) // clear masked part of original memory, and insert data
                }

                // Write memory back
                newMemRoot_ = MIPS64Memory.writeMem(effAddr, _args.proofOffset, mem);
                memUpdated_ = true;
                memAddr_ = effAddr;
                newPreimageOffset_ += uint64(datLen);
                v0_ = uint64(datLen);
            }
            // hint response
            else if (_args.a0 == FD_HINT_READ) {
                // Don't read into memory, just say we read it all
                // The result is ignored anyway
                v0_ = _args.a2;
            } else {
                v0_ = U64_MASK;
                v1_ = EBADF;
            }

            return (v0_, v1_, newPreimageOffset_, newMemRoot_, memUpdated_, memAddr_);
        }
    }

    /// @notice Like a Linux write syscall. Splits unaligned writes into aligned writes.
    /// @param _a0 The file descriptor.
    /// @param _a1 The memory address to read from.
    /// @param _a2 The number of bytes to read.
    /// @param _preimageKey The current preimaageKey.
    /// @param _preimageOffset The current preimageOffset.
    /// @param _proofOffset The offset of the memory proof in calldata.
    /// @param _memRoot The current memory root.
    /// @return v0_ The number of bytes written, or -1 on error.
    /// @return v1_ The error code, or 0 if empty.
    /// @return newPreimageKey_ The new preimageKey.
    /// @return newPreimageOffset_ The new preimageOffset.
    function handleSysWrite(
        uint64 _a0,
        uint64 _a1,
        uint64 _a2,
        bytes32 _preimageKey,
        uint64 _preimageOffset,
        uint256 _proofOffset,
        bytes32 _memRoot
    )
        internal
        pure
        returns (uint64 v0_, uint64 v1_, bytes32 newPreimageKey_, uint64 newPreimageOffset_)
    {
        unchecked {
            // args: _a0 = fd, _a1 = addr, _a2 = count
            // returns: v0_ = written, v1_ = err code
            v0_ = uint64(0);
            v1_ = uint64(0);
            newPreimageKey_ = _preimageKey;
            newPreimageOffset_ = _preimageOffset;

            if (_a0 == FD_STDOUT || _a0 == FD_STDERR || _a0 == FD_HINT_WRITE) {
                v0_ = _a2; // tell program we have written everything
            }
            // pre-image oracle
            else if (_a0 == FD_PREIMAGE_WRITE) {
                // mask the addr to align it to 4 bytes
                uint64 mem = MIPS64Memory.readMem(_memRoot, _a1 & arch.ADDRESS_MASK, _proofOffset);
                bytes32 key = _preimageKey;

                // Construct pre-image key from memory
                // We use assembly for more precise ops, and no var count limit
                assembly {
                    let alignment := and(_a1, EXT_MASK) // the read might not start at an aligned address
                    let space := sub(WORD_SIZE_BYTES, alignment) // remaining space in memory word
                    if lt(space, _a2) { _a2 := space } // if less space than data, shorten data
                    key := shl(mul(_a2, 8), key) // shift key, make space for new info
                    let mask := sub(shl(mul(_a2, 8), 1), 1) // mask for extracting value from memory
                    mem := and(shr(mul(sub(space, _a2), 8), mem), mask) // align value to right, mask it
                    key := or(key, mem) // insert into key
                }

                // Write pre-image key to oracle
                newPreimageKey_ = key;
                newPreimageOffset_ = 0; // reset offset, to read new pre-image data from the start
                v0_ = _a2;
            } else {
                v0_ = U64_MASK;
                v1_ = EBADF;
            }

            return (v0_, v1_, newPreimageKey_, newPreimageOffset_);
        }
    }

    /// @notice Like Linux fcntl (file control) syscall, but only supports minimal file-descriptor control commands, to
    /// retrieve the file-descriptor R/W flags.
    /// @param _a0 The file descriptor.
    /// @param _a1 The control command.
    /// @param v0_ The file status flag (only supported commands are F_GETFD and F_GETFL), or -1 on error.
    /// @param v1_ An error number, or 0 if there is no error.
    function handleSysFcntl(uint64 _a0, uint64 _a1) internal pure returns (uint64 v0_, uint64 v1_) {
        unchecked {
            v0_ = uint64(0);
            v1_ = uint64(0);

            // args: _a0 = fd, _a1 = cmd
            if (_a1 == 1) {
                // F_GETFD: get file descriptor flags
                if (
                    _a0 == FD_STDIN || _a0 == FD_STDOUT || _a0 == FD_STDERR || _a0 == FD_PREIMAGE_READ
                        || _a0 == FD_HINT_READ || _a0 == FD_PREIMAGE_WRITE || _a0 == FD_HINT_WRITE
                ) {
                    v0_ = 0; // No flags set
                } else {
                    v0_ = U64_MASK;
                    v1_ = EBADF;
                }
            } else if (_a1 == 3) {
                // F_GETFL: get file status flags
                if (_a0 == FD_STDIN || _a0 == FD_PREIMAGE_READ || _a0 == FD_HINT_READ) {
                    v0_ = 0; // O_RDONLY
                } else if (_a0 == FD_STDOUT || _a0 == FD_STDERR || _a0 == FD_PREIMAGE_WRITE || _a0 == FD_HINT_WRITE) {
                    v0_ = 1; // O_WRONLY
                } else {
                    v0_ = U64_MASK;
                    v1_ = EBADF;
                }
            } else {
                v0_ = U64_MASK;
                v1_ = EINVAL; // cmd not recognized by this kernel
            }

            return (v0_, v1_);
        }
    }

    function handleSyscallUpdates(
        st.CpuScalars memory _cpu,
        uint64[32] memory _registers,
        uint64 _v0,
        uint64 _v1
    )
        internal
        pure
    {
        unchecked {
            // Write the results back to the state registers
            _registers[REG_SYSCALL_RET1] = _v0;
            _registers[REG_SYSCALL_ERRNO] = _v1;

            // Update the PC and nextPC
            _cpu.pc = _cpu.nextPC;
            _cpu.nextPC = _cpu.nextPC + 4;
        }
    }
}
