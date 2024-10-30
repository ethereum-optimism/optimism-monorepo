// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { MIPSMemory } from "src/cannon/libraries/MIPSMemory.sol";
import { MIPSState as st } from "src/cannon/libraries/MIPSState.sol";
import { IPreimageOracle } from "src/cannon/interfaces/IPreimageOracle.sol";
import { PreimageKeyLib } from "src/cannon/PreimageKeyLib.sol";

library MIPSSyscalls {
    struct SysReadParams {
        /// @param _a0 The file descriptor.
        uint32 a0;
        /// @param _a1 The memory location where data should be read to.
        uint32 a1;
        /// @param _a2 The number of bytes to read from the file
        uint32 a2;
        /// @param _preimageKey The key of the preimage to read.
        bytes32 preimageKey;
        /// @param _preimageOffset The offset of the preimage to read.
        uint32 preimageOffset;
        /// @param _localContext The local context for the preimage key.
        bytes32 localContext;
        /// @param _oracle The address of the preimage oracle.
        IPreimageOracle oracle;
        /// @param _proofOffset The offset of the memory proof in calldata.
        uint256 proofOffset;
        /// @param _memRoot The current memory root.
        bytes32 memRoot;
    }

    uint32 internal constant SYS_MMAP = 4090;
    uint32 internal constant SYS_BRK = 4045;
    uint32 internal constant SYS_CLONE = 4120;
    uint32 internal constant SYS_EXIT_GROUP = 4246;
    uint32 internal constant SYS_READ = 4003;
    uint32 internal constant SYS_WRITE = 4004;
    uint32 internal constant SYS_FCNTL = 4055;
    uint32 internal constant SYS_EXIT = 4001;
    uint32 internal constant SYS_SCHED_YIELD = 4162;
    uint32 internal constant SYS_GETTID = 4222;
    uint32 internal constant SYS_FUTEX = 4238;
    uint32 internal constant SYS_OPEN = 4005;
    uint32 internal constant SYS_NANOSLEEP = 4166;
    uint32 internal constant SYS_CLOCKGETTIME = 4263;
    uint32 internal constant SYS_GETPID = 4020;
    // unused syscalls
    uint32 internal constant SYS_MUNMAP = 4091;
    uint32 internal constant SYS_GETAFFINITY = 4240;
    uint32 internal constant SYS_MADVISE = 4218;
    uint32 internal constant SYS_RTSIGPROCMASK = 4195;
    uint32 internal constant SYS_SIGALTSTACK = 4206;
    uint32 internal constant SYS_RTSIGACTION = 4194;
    uint32 internal constant SYS_PRLIMIT64 = 4338;
    uint32 internal constant SYS_CLOSE = 4006;
    uint32 internal constant SYS_PREAD64 = 4200;
    uint32 internal constant SYS_STAT = 4106;
    uint32 internal constant SYS_FSTAT = 4108;
    uint32 internal constant SYS_FSTAT64 = 4215;
    uint32 internal constant SYS_OPENAT = 4288;
    uint32 internal constant SYS_READLINK = 4085;
    uint32 internal constant SYS_READLINKAT = 4298;
    uint32 internal constant SYS_IOCTL = 4054;
    uint32 internal constant SYS_EPOLLCREATE1 = 4326;
    uint32 internal constant SYS_PIPE2 = 4328;
    uint32 internal constant SYS_EPOLLCTL = 4249;
    uint32 internal constant SYS_EPOLLPWAIT = 4313;
    uint32 internal constant SYS_GETRANDOM = 4353;
    uint32 internal constant SYS_UNAME = 4122;
    uint32 internal constant SYS_STAT64 = 4213;
    uint32 internal constant SYS_GETUID = 4024;
    uint32 internal constant SYS_GETGID = 4047;
    uint32 internal constant SYS_LLSEEK = 4140;
    uint32 internal constant SYS_MINCORE = 4217;
    uint32 internal constant SYS_TGKILL = 4266;
    uint32 internal constant SYS_GETRLIMIT = 4076;
    uint32 internal constant SYS_LSEEK = 4019;

    // profiling-related syscalls - ignored
    uint32 internal constant SYS_SETITIMER = 4104;
    uint32 internal constant SYS_TIMERCREATE = 4257;
    uint32 internal constant SYS_TIMERSETTIME = 4258;
    uint32 internal constant SYS_TIMERDELETE = 4261;

    uint32 internal constant FD_STDIN = 0;
    uint32 internal constant FD_STDOUT = 1;
    uint32 internal constant FD_STDERR = 2;
    uint32 internal constant FD_HINT_READ = 3;
    uint32 internal constant FD_HINT_WRITE = 4;
    uint32 internal constant FD_PREIMAGE_READ = 5;
    uint32 internal constant FD_PREIMAGE_WRITE = 6;

    uint32 internal constant SYS_ERROR_SIGNAL = 0xFF_FF_FF_FF;
    uint32 internal constant EBADF = 0x9;
    uint32 internal constant EINVAL = 0x16;
    uint32 internal constant EAGAIN = 0xb;
    uint32 internal constant ETIMEDOUT = 0x91;

    uint32 internal constant FUTEX_WAIT_PRIVATE = 128;
    uint32 internal constant FUTEX_WAKE_PRIVATE = 129;
    uint32 internal constant FUTEX_TIMEOUT_STEPS = 10000;
    uint64 internal constant FUTEX_NO_TIMEOUT = type(uint64).max;
    uint32 internal constant FUTEX_EMPTY_ADDR = 0xFF_FF_FF_FF;

    uint32 internal constant SCHED_QUANTUM = 100_000;
    uint32 internal constant HZ = 10_000_000;
    uint32 internal constant CLOCK_GETTIME_REALTIME_FLAG = 0;
    uint32 internal constant CLOCK_GETTIME_MONOTONIC_FLAG = 1;
    /// @notice Start of the data segment.
    uint32 internal constant PROGRAM_BREAK = 0x40000000;
    uint32 internal constant HEAP_END = 0x60000000;

    // SYS_CLONE flags
    uint32 internal constant CLONE_VM = 0x100;
    uint32 internal constant CLONE_FS = 0x200;
    uint32 internal constant CLONE_FILES = 0x400;
    uint32 internal constant CLONE_SIGHAND = 0x800;
    uint32 internal constant CLONE_PTRACE = 0x2000;
    uint32 internal constant CLONE_VFORK = 0x4000;
    uint32 internal constant CLONE_PARENT = 0x8000;
    uint32 internal constant CLONE_THREAD = 0x10000;
    uint32 internal constant CLONE_NEWNS = 0x20000;
    uint32 internal constant CLONE_SYSVSEM = 0x40000;
    uint32 internal constant CLONE_SETTLS = 0x80000;
    uint32 internal constant CLONE_PARENTSETTID = 0x100000;
    uint32 internal constant CLONE_CHILDCLEARTID = 0x200000;
    uint32 internal constant CLONE_UNTRACED = 0x800000;
    uint32 internal constant CLONE_CHILDSETTID = 0x1000000;
    uint32 internal constant CLONE_STOPPED = 0x2000000;
    uint32 internal constant CLONE_NEWUTS = 0x4000000;
    uint32 internal constant CLONE_NEWIPC = 0x8000000;
    uint32 internal constant VALID_SYS_CLONE_FLAGS =
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

    /// @notice Extract syscall num and arguments from registers.
    /// @param _registers The cpu registers.
    /// @return sysCallNum_ The syscall number.
    /// @return a0_ The first argument available to the syscall operation.
    /// @return a1_ The second argument available to the syscall operation.
    /// @return a2_ The third argument available to the syscall operation.
    /// @return a3_ The fourth argument available to the syscall operation.
    function getSyscallArgs(uint32[32] memory _registers)
        internal
        pure
        returns (uint32 sysCallNum_, uint32 a0_, uint32 a1_, uint32 a2_, uint32 a3_)
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
        uint32 _a0,
        uint32 _a1,
        uint32 _heap
    )
        internal
        pure
        returns (uint32 v0_, uint32 v1_, uint32 newHeap_)
    {
        unchecked {
            v1_ = uint32(0);
            newHeap_ = _heap;

            uint32 sz = _a1;
            if (sz & 4095 != 0) {
                // adjust size to align with page size
                sz += 4096 - (sz & 4095);
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
            uint32 v0_,
            uint32 v1_,
            uint32 newPreimageOffset_,
            bytes32 newMemRoot_,
            bool memUpdated_,
            uint32 memAddr_
        )
    {
        unchecked {
            v0_ = uint32(0);
            v1_ = uint32(0);
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
                uint32 effAddr = _args.a1 & 0xFFffFFfc;
                // verify proof is correct, and get the existing memory.
                // mask the addr to align it to 4 bytes
                uint32 mem = MIPSMemory.readMem(_args.memRoot, effAddr, _args.proofOffset);
                // If the preimage key is a local key, localize it in the context of the caller.
                if (uint8(_args.preimageKey[0]) == 1) {
                    _args.preimageKey = PreimageKeyLib.localize(_args.preimageKey, _args.localContext);
                }
                (bytes32 dat, uint256 datLen) = _args.oracle.readPreimage(_args.preimageKey, _args.preimageOffset);

                // Transform data for writing to memory
                // We use assembly for more precise ops, and no var count limit
                uint32 a1 = _args.a1;
                uint32 a2 = _args.a2;
                assembly {
                    let alignment := and(a1, 3) // the read might not start at an aligned address
                    let space := sub(4, alignment) // remaining space in memory word
                    if lt(space, datLen) { datLen := space } // if less space than data, shorten data
                    if lt(a2, datLen) { datLen := a2 } // if requested to read less, read less
                    dat := shr(sub(256, mul(datLen, 8)), dat) // right-align data
                    dat := shl(mul(sub(sub(4, datLen), alignment), 8), dat) // position data to insert into memory
                    // word
                    let mask := sub(shl(mul(sub(4, alignment), 8), 1), 1) // mask all bytes after start
                    let suffixMask := sub(shl(mul(sub(sub(4, alignment), datLen), 8), 1), 1) // mask of all bytes
                    // starting from end, maybe none
                    mask := and(mask, not(suffixMask)) // reduce mask to just cover the data we insert
                    mem := or(and(mem, not(mask)), dat) // clear masked part of original memory, and insert data
                }

                // Write memory back
                newMemRoot_ = MIPSMemory.writeMem(effAddr, _args.proofOffset, mem);
                memUpdated_ = true;
                memAddr_ = effAddr;
                newPreimageOffset_ += uint32(datLen);
                v0_ = uint32(datLen);
            }
            // hint response
            else if (_args.a0 == FD_HINT_READ) {
                // Don't read into memory, just say we read it all
                // The result is ignored anyway
                v0_ = _args.a2;
            } else {
                v0_ = 0xFFffFFff;
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
        uint32 _a0,
        uint32 _a1,
        uint32 _a2,
        bytes32 _preimageKey,
        uint32 _preimageOffset,
        uint256 _proofOffset,
        bytes32 _memRoot
    )
        internal
        pure
        returns (uint32 v0_, uint32 v1_, bytes32 newPreimageKey_, uint32 newPreimageOffset_)
    {
        unchecked {
            // args: _a0 = fd, _a1 = addr, _a2 = count
            // returns: v0_ = written, v1_ = err code
            v0_ = uint32(0);
            v1_ = uint32(0);
            newPreimageKey_ = _preimageKey;
            newPreimageOffset_ = _preimageOffset;

            if (_a0 == FD_STDOUT || _a0 == FD_STDERR || _a0 == FD_HINT_WRITE) {
                v0_ = _a2; // tell program we have written everything
            }
            // pre-image oracle
            else if (_a0 == FD_PREIMAGE_WRITE) {
                // mask the addr to align it to 4 bytes
                uint32 mem = MIPSMemory.readMem(_memRoot, _a1 & 0xFFffFFfc, _proofOffset);
                bytes32 key = _preimageKey;

                // Construct pre-image key from memory
                // We use assembly for more precise ops, and no var count limit
                assembly {
                    let alignment := and(_a1, 3) // the read might not start at an aligned address
                    let space := sub(4, alignment) // remaining space in memory word
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
                v0_ = 0xFFffFFff;
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
    function handleSysFcntl(uint32 _a0, uint32 _a1) internal pure returns (uint32 v0_, uint32 v1_) {
        unchecked {
            v0_ = uint32(0);
            v1_ = uint32(0);

            // args: _a0 = fd, _a1 = cmd
            if (_a1 == 1) {
                // F_GETFD: get file descriptor flags
                if (
                    _a0 == FD_STDIN || _a0 == FD_STDOUT || _a0 == FD_STDERR || _a0 == FD_PREIMAGE_READ
                        || _a0 == FD_HINT_READ || _a0 == FD_PREIMAGE_WRITE || _a0 == FD_HINT_WRITE
                ) {
                    v0_ = 0; // No flags set
                } else {
                    v0_ = 0xFFffFFff;
                    v1_ = EBADF;
                }
            } else if (_a1 == 3) {
                // F_GETFL: get file status flags
                if (_a0 == FD_STDIN || _a0 == FD_PREIMAGE_READ || _a0 == FD_HINT_READ) {
                    v0_ = 0; // O_RDONLY
                } else if (_a0 == FD_STDOUT || _a0 == FD_STDERR || _a0 == FD_PREIMAGE_WRITE || _a0 == FD_HINT_WRITE) {
                    v0_ = 1; // O_WRONLY
                } else {
                    v0_ = 0xFFffFFff;
                    v1_ = EBADF;
                }
            } else {
                v0_ = 0xFFffFFff;
                v1_ = EINVAL; // cmd not recognized by this kernel
            }

            return (v0_, v1_);
        }
    }

    function handleSyscallUpdates(
        st.CpuScalars memory _cpu,
        uint32[32] memory _registers,
        uint32 _v0,
        uint32 _v1
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
