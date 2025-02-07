// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Libraries
import { MIPSMemory } from "src/cannon/libraries/MIPSMemory.sol";
import { MIPSSyscalls as sys } from "src/cannon/libraries/MIPSSyscalls.sol";
import { MIPSState as st } from "src/cannon/libraries/MIPSState.sol";
import { MIPSInstructions as ins } from "src/cannon/libraries/MIPSInstructions.sol";
import { VMStatuses } from "src/dispute/lib/Types.sol";
import {
    InvalidMemoryProof, InvalidRMWInstruction, InvalidSecondMemoryProof
} from "src/cannon/libraries/CannonErrors.sol";

// Interfaces
import { ISemver } from "interfaces/universal/ISemver.sol";
import { IPreimageOracle } from "interfaces/cannon/IPreimageOracle.sol";

/// @title MIPS2
/// @notice The MIPS2 contract emulates a single MIPS instruction.
///         It differs from MIPS.sol in that it supports multi-threading.
contract MIPS2 is ISemver {
    /// @notice The thread context.
    ///         Total state size: 4 + 1 + 1 + 4 + 4 + 4 + 4 + 32 * 4 = 150 bytes
    struct ThreadState {
        // metadata
        uint32 threadID;
        uint8 exitCode;
        bool exited;
        // state
        uint32 pc;
        uint32 nextPC;
        uint32 lo;
        uint32 hi;
        uint32[32] registers;
    }

    uint32 internal constant PACKED_THREAD_STATE_SIZE = 150;

    uint8 internal constant LL_STATUS_NONE = 0;
    uint8 internal constant LL_STATUS_ACTIVE = 1;

    /// @notice Stores the VM state.
    ///         Total state size: 32 + 32 + 4 + 4 + 1 + 4 + 4 + 1 + 1 + 8 + 8 + 1 + 32 + 32 + 4 = 168 bytes
    ///         If nextPC != pc + 4, then the VM is executing a branch/jump delay slot.
    struct State {
        bytes32 memRoot;
        bytes32 preimageKey;
        uint32 preimageOffset;
        uint32 heap;
        uint8 llReservationStatus;
        uint32 llAddress;
        uint32 llOwnerThread;
        uint8 exitCode;
        bool exited;
        uint64 step;
        uint64 stepsSinceLastContextSwitch;
        bool traverseRight;
        bytes32 leftThreadStack;
        bytes32 rightThreadStack;
        uint32 nextThreadID;
    }

    /// @notice The semantic version of the MIPS2 contract.
    /// @custom:semver 1.0.0-beta.29
    string public constant version = "1.0.0-beta.29";

    /// @notice The preimage oracle contract.
    IPreimageOracle internal immutable ORACLE;

    // The offset of the start of proof calldata (_threadWitness.offset) in the step() function
    uint256 internal constant THREAD_PROOF_OFFSET = 356;

    // The offset of the start of proof calldata (_memProof.offset) in the step() function
    uint256 internal constant MEM_PROOF_OFFSET = THREAD_PROOF_OFFSET + PACKED_THREAD_STATE_SIZE + 32;

    // The empty thread root - keccak256(bytes32(0) ++ bytes32(0))
    bytes32 internal constant EMPTY_THREAD_ROOT = hex"ad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb5";

    // State memory offset allocated during step
    uint256 internal constant STATE_MEM_OFFSET = 0x80;

    // ThreadState memory offset allocated during step
    uint256 internal constant TC_MEM_OFFSET = 0x260;

    /// @param _oracle The address of the preimage oracle contract.
    constructor(IPreimageOracle _oracle) {
        ORACLE = _oracle;
    }

    /// @notice Getter for the pre-image oracle contract.
    /// @return oracle_ The IPreimageOracle contract.
    function oracle() external view returns (IPreimageOracle oracle_) {
        oracle_ = ORACLE;
    }

    /// @notice Executes a single step of the multi-threaded vm.
    ///         Will revert if any required input state is missing.
    /// @param _stateData The encoded state witness data.
    /// @param _proof The encoded proof data: <<thread_context, inner_root>, <memory proof>.
    ///               Contains the thread context witness and the memory proof data for leaves within the MIPS VM's
    /// memory.
    ///               The thread context witness is a packed tuple of the thread context and the immediate inner root of
    /// the current thread stack.
    /// @param _localContext The local key context for the preimage oracle. Optional, can be set as a constant
    ///                      if the caller only requires one set of local keys.
    /// @return postState_ The hash of the post state witness after the state transition.
    function step(
        bytes calldata _stateData,
        bytes calldata _proof,
        bytes32 _localContext
    )
        public
        returns (bytes32 postState_)
    {
        postState_ = doStep(_stateData, _proof, _localContext);
        assertPostStateChecks();
    }

    function assertPostStateChecks() internal pure {
        State memory state;
        assembly {
            state := STATE_MEM_OFFSET
        }

        bytes32 activeStack = state.traverseRight ? state.rightThreadStack : state.leftThreadStack;
        if (activeStack == EMPTY_THREAD_ROOT) {
            revert("MIPS2: post-state active thread stack is empty");
        }
    }

    function doStep(
        bytes calldata _stateData,
        bytes calldata _proof,
        bytes32 _localContext
    )
        internal
        returns (bytes32)
    {
        unchecked {
            State memory state;
            ThreadState memory thread;
            uint32 exited;
            assembly {
                if iszero(eq(state, STATE_MEM_OFFSET)) {
                    // expected state mem offset check
                    revert(0, 0)
                }
                if iszero(eq(thread, TC_MEM_OFFSET)) {
                    // expected thread mem offset check
                    // STATE_MEM_OFFSET = 0x80 = 128
                    // 32 bytes per state field = 32 * 15 = 480
                    // TC_MEM_OFFSET = 480 + 128 = 608 = 0x260
                    revert(0, 0)
                }
                if iszero(eq(mload(0x40), shl(5, 59))) {
                    // 4 + 15 state slots + 40 thread slots = 59 expected memory check
                    revert(0, 0)
                }
                if iszero(eq(_stateData.offset, 132)) {
                    // 32*4+4=132 expected state data offset
                    revert(0, 0)
                }
                if iszero(eq(_proof.offset, THREAD_PROOF_OFFSET)) {
                    // _stateData.offset = 132
                    // stateData.length = ceil(stateSize / 32) * 32 = 6 * 32 = 192
                    // _proof size prefix = 32
                    // expected thread proof offset equals the sum of the above is 356
                    revert(0, 0)
                }

                function putField(callOffset, memOffset, size) -> callOffsetOut, memOffsetOut {
                    // calldata is packed, thus starting left-aligned, shift-right to pad and right-align
                    let w := shr(shl(3, sub(32, size)), calldataload(callOffset))
                    mstore(memOffset, w)
                    callOffsetOut := add(callOffset, size)
                    memOffsetOut := add(memOffset, 32)
                }

                // Unpack state from calldata into memory
                let c := _stateData.offset // calldata offset
                let m := STATE_MEM_OFFSET // mem offset
                c, m := putField(c, m, 32) // memRoot
                c, m := putField(c, m, 32) // preimageKey
                c, m := putField(c, m, 4) // preimageOffset
                c, m := putField(c, m, 4) // heap
                c, m := putField(c, m, 1) // llReservationStatus
                c, m := putField(c, m, 4) // llAddress
                c, m := putField(c, m, 4) // llOwnerThread
                c, m := putField(c, m, 1) // exitCode
                c, m := putField(c, m, 1) // exited
                exited := mload(sub(m, 32))
                c, m := putField(c, m, 8) // step
                c, m := putField(c, m, 8) // stepsSinceLastContextSwitch
                c, m := putField(c, m, 1) // traverseRight
                c, m := putField(c, m, 32) // leftThreadStack
                c, m := putField(c, m, 32) // rightThreadStack
                c, m := putField(c, m, 4) // nextThreadID
            }
            st.assertExitedIsValid(exited);

            if (state.exited) {
                // thread state is unchanged
                return outputState();
            }

            if (
                (state.leftThreadStack == EMPTY_THREAD_ROOT && !state.traverseRight)
                    || (state.rightThreadStack == EMPTY_THREAD_ROOT && state.traverseRight)
            ) {
                revert("MIPS2: active thread stack is empty");
            }

            state.step += 1;

            setThreadStateFromCalldata(thread);
            validateCalldataThreadWitness(state, thread);

            if (thread.exited) {
                popThread(state);
                return outputState();
            }

            if (state.stepsSinceLastContextSwitch >= sys.SCHED_QUANTUM) {
                preemptThread(state, thread);
                return outputState();
            }
            state.stepsSinceLastContextSwitch += 1;

            // instruction fetch
            uint256 insnProofOffset = MIPSMemory.memoryProofOffset(MEM_PROOF_OFFSET, 0);
            (uint32 insn, uint32 opcode, uint32 fun) =
                ins.getInstructionDetails(thread.pc, state.memRoot, insnProofOffset);

            // Handle syscall separately
            // syscall (can read and write)
            if (opcode == 0 && fun == 0xC) {
                return handleSyscall(_localContext);
            }

            // Handle RMW (read-modify-write) ops
            if (opcode == ins.OP_LOAD_LINKED || opcode == ins.OP_STORE_CONDITIONAL) {
                return handleRMWOps(state, thread, insn, opcode);
            }

            // Exec the rest of the step logic
            st.CpuScalars memory cpu = getCpuScalars(thread);
            ins.CoreStepLogicParams memory coreStepArgs = ins.CoreStepLogicParams({
                cpu: cpu,
                registers: thread.registers,
                memRoot: state.memRoot,
                memProofOffset: MIPSMemory.memoryProofOffset(MEM_PROOF_OFFSET, 1),
                insn: insn,
                opcode: opcode,
                fun: fun
            });
            bool memUpdated;
            uint32 memAddr;
            (state.memRoot, memUpdated, memAddr) = ins.execMipsCoreStepLogic(coreStepArgs);
            setStateCpuScalars(thread, cpu);
            updateCurrentThreadRoot();
            if (memUpdated) {
                handleMemoryUpdate(state, memAddr);
            }

            return outputState();
        }
    }

    function handleMemoryUpdate(State memory _state, uint32 _memAddr) internal pure {
        if (_memAddr == (0xFFFFFFFC & _state.llAddress)) {
            // Reserved address was modified, clear the reservation
            clearLLMemoryReservation(_state);
        }
    }

    function clearLLMemoryReservation(State memory _state) internal pure {
        _state.llReservationStatus = LL_STATUS_NONE;
        _state.llAddress = 0;
        _state.llOwnerThread = 0;
    }

    function handleRMWOps(
        State memory _state,
        ThreadState memory _thread,
        uint32 _insn,
        uint32 _opcode
    )
        internal
        returns (bytes32)
    {
        unchecked {
            uint32 baseReg = (_insn >> 21) & 0x1F;
            uint32 base = _thread.registers[baseReg];
            uint32 rtReg = (_insn >> 16) & 0x1F;
            uint32 offset = ins.signExtendImmediate(_insn);
            uint32 addr = base + offset;

            uint32 retVal = 0;
            uint32 threadId = _thread.threadID;
            if (_opcode == ins.OP_LOAD_LINKED) {
                retVal = loadWord(_state, addr);

                _state.llReservationStatus = LL_STATUS_ACTIVE;
                _state.llAddress = addr;
                _state.llOwnerThread = threadId;
            } else if (_opcode == ins.OP_STORE_CONDITIONAL) {
                // Check if our memory reservation is still intact
                if (
                    _state.llReservationStatus == LL_STATUS_ACTIVE && _state.llOwnerThread == threadId
                        && _state.llAddress == addr
                ) {
                    // Complete atomic update: set memory and return 1 for success
                    clearLLMemoryReservation(_state);

                    uint32 val = _thread.registers[rtReg];
                    storeWord(_state, addr, val);

                    retVal = 1;
                } else {
                    // Atomic update failed, return 0 for failure
                    retVal = 0;
                }
            } else {
                revert InvalidRMWInstruction();
            }

            st.CpuScalars memory cpu = getCpuScalars(_thread);
            ins.handleRd(cpu, _thread.registers, rtReg, retVal, true);
            setStateCpuScalars(_thread, cpu);
            updateCurrentThreadRoot();

            return outputState();
        }
    }

    function loadWord(State memory _state, uint32 _addr) internal pure returns (uint32 val_) {
        uint32 effAddr = _addr & 0xFFFFFFFC;
        uint256 memProofOffset = MIPSMemory.memoryProofOffset(MEM_PROOF_OFFSET, 1);
        val_ = MIPSMemory.readMem(_state.memRoot, effAddr, memProofOffset);
    }

    function storeWord(State memory _state, uint32 _addr, uint32 _val) internal pure {
        uint32 effAddr = _addr & 0xFFFFFFFC;
        uint256 memProofOffset = MIPSMemory.memoryProofOffset(MEM_PROOF_OFFSET, 1);
        _state.memRoot = MIPSMemory.writeMem(effAddr, memProofOffset, _val);
    }

    function handleSyscall(bytes32 _localContext) internal returns (bytes32 out_) {
        unchecked {
            // Load state from memory offsets to reduce stack pressure
            State memory state;
            ThreadState memory thread;
            assembly {
                state := STATE_MEM_OFFSET
                thread := TC_MEM_OFFSET
            }

            // Load the syscall numbers and args from the registers
            (uint32 syscall_no, uint32 a0, uint32 a1, uint32 a2) = sys.getSyscallArgs(thread.registers);
            // Syscalls that are unimplemented but known return with v0=0 and v1=0
            uint32 v0 = 0;
            uint32 v1 = 0;

            if (syscall_no == sys.SYS_MMAP) {
                (v0, v1, state.heap) = sys.handleSysMmap(a0, a1, state.heap);
            } else if (syscall_no == sys.SYS_BRK) {
                // brk: Returns a fixed address for the program break at 0x40000000
                v0 = sys.PROGRAM_BREAK;
            } else if (syscall_no == sys.SYS_CLONE) {
                if (sys.VALID_SYS_CLONE_FLAGS != a0) {
                    state.exited = true;
                    state.exitCode = VMStatuses.PANIC.raw();
                    return outputState();
                }
                v0 = state.nextThreadID;
                v1 = 0;
                ThreadState memory newThread;
                newThread.threadID = state.nextThreadID;
                newThread.exitCode = 0;
                newThread.exited = false;
                newThread.pc = thread.nextPC;
                newThread.nextPC = thread.nextPC + 4;
                newThread.lo = thread.lo;
                newThread.hi = thread.hi;
                for (uint256 i; i < 32; i++) {
                    newThread.registers[i] = thread.registers[i];
                }
                newThread.registers[sys.REG_SP] = a1; // set stack pointer
                // the child will perceive a 0 value as returned value instead, and no error
                newThread.registers[sys.REG_SYSCALL_RET1] = 0;
                newThread.registers[sys.REG_SYSCALL_ERRNO] = 0;
                state.nextThreadID++;

                // Preempt this thread for the new one. But not before updating PCs
                st.CpuScalars memory cpu0 = getCpuScalars(thread);
                sys.handleSyscallUpdates(cpu0, thread.registers, v0, v1);
                setStateCpuScalars(thread, cpu0);
                updateCurrentThreadRoot();
                pushThread(state, newThread);
                return outputState();
            } else if (syscall_no == sys.SYS_EXIT_GROUP) {
                // exit group: Sets the Exited and ExitCode states to true and argument 0.
                state.exited = true;
                state.exitCode = uint8(a0);
                updateCurrentThreadRoot();
                return outputState();
            } else if (syscall_no == sys.SYS_READ) {
                sys.SysReadParams memory args = sys.SysReadParams({
                    a0: a0,
                    a1: a1,
                    a2: a2,
                    preimageKey: state.preimageKey,
                    preimageOffset: state.preimageOffset,
                    localContext: _localContext,
                    oracle: ORACLE,
                    proofOffset: MIPSMemory.memoryProofOffset(MEM_PROOF_OFFSET, 1),
                    memRoot: state.memRoot
                });
                // Encapsulate execution to avoid stack-too-deep error
                (v0, v1) = execSysRead(state, args);
            } else if (syscall_no == sys.SYS_WRITE) {
                sys.SysWriteParams memory args = sys.SysWriteParams({
                    _a0: a0,
                    _a1: a1,
                    _a2: a2,
                    _preimageKey: state.preimageKey,
                    _preimageOffset: state.preimageOffset,
                    _proofOffset: MIPSMemory.memoryProofOffset(MEM_PROOF_OFFSET, 1),
                    _memRoot: state.memRoot
                });
                (v0, v1, state.preimageKey, state.preimageOffset) = sys.handleSysWrite(args);
            } else if (syscall_no == sys.SYS_FCNTL) {
                (v0, v1) = sys.handleSysFcntl(a0, a1);
            } else if (syscall_no == sys.SYS_GETTID) {
                v0 = thread.threadID;
                v1 = 0;
            } else if (syscall_no == sys.SYS_EXIT) {
                thread.exited = true;
                thread.exitCode = uint8(a0);
                if (lastThreadRemaining(state)) {
                    state.exited = true;
                    state.exitCode = uint8(a0);
                }
                updateCurrentThreadRoot();
                return outputState();
            } else if (syscall_no == sys.SYS_FUTEX) {
                // args: a0 = addr, a1 = op, a2 = val, a3 = timeout
                uint32 effFutexAddr = a0 & 0xFFffFFfc;
                if (a1 == sys.FUTEX_WAIT_PRIVATE) {
                    uint32 futexVal = getFutexValue(effFutexAddr);
                    uint32 targetValue = a2;
                    if (futexVal != targetValue) {
                        v0 = sys.SYS_ERROR_SIGNAL;
                        v1 = sys.EAGAIN;
                    } else {
                        return syscallYield(state, thread);
                    }
                } else if (a1 == sys.FUTEX_WAKE_PRIVATE) {
                    return syscallYield(state, thread);
                } else {
                    v0 = sys.SYS_ERROR_SIGNAL;
                    v1 = sys.EINVAL;
                }
            } else if (syscall_no == sys.SYS_SCHED_YIELD || syscall_no == sys.SYS_NANOSLEEP) {
                return syscallYield(state, thread);
            } else if (syscall_no == sys.SYS_OPEN) {
                v0 = sys.SYS_ERROR_SIGNAL;
                v1 = sys.EBADF;
            } else if (syscall_no == sys.SYS_CLOCKGETTIME) {
                if (a0 == sys.CLOCK_GETTIME_REALTIME_FLAG || a0 == sys.CLOCK_GETTIME_MONOTONIC_FLAG) {
                    v0 = 0;
                    v1 = 0;
                    uint32 secs = 0;
                    uint32 nsecs = 0;
                    if (a0 == sys.CLOCK_GETTIME_MONOTONIC_FLAG) {
                        secs = uint32(state.step / sys.HZ);
                        nsecs = uint32((state.step % sys.HZ) * (1_000_000_000 / sys.HZ));
                    }
                    uint32 effAddr = a1 & 0xFFffFFfc;
                    // First verify the effAddr path
                    if (
                        !MIPSMemory.isValidProof(
                            state.memRoot, effAddr, MIPSMemory.memoryProofOffset(MEM_PROOF_OFFSET, 1)
                        )
                    ) {
                        revert InvalidMemoryProof();
                    }
                    // Recompute the new root after updating effAddr
                    state.memRoot =
                        MIPSMemory.writeMem(effAddr, MIPSMemory.memoryProofOffset(MEM_PROOF_OFFSET, 1), secs);
                    handleMemoryUpdate(state, effAddr);
                    // Verify the second memory proof against the newly computed root
                    if (
                        !MIPSMemory.isValidProof(
                            state.memRoot, effAddr + 4, MIPSMemory.memoryProofOffset(MEM_PROOF_OFFSET, 2)
                        )
                    ) {
                        revert InvalidSecondMemoryProof();
                    }
                    state.memRoot =
                        MIPSMemory.writeMem(effAddr + 4, MIPSMemory.memoryProofOffset(MEM_PROOF_OFFSET, 2), nsecs);
                    handleMemoryUpdate(state, effAddr + 4);
                } else {
                    v0 = sys.SYS_ERROR_SIGNAL;
                    v1 = sys.EINVAL;
                }
            } else if (syscall_no == sys.SYS_GETPID) {
                v0 = 0;
                v1 = 0;
            } else if (syscall_no == sys.SYS_MUNMAP) {
                // ignored
            } else if (syscall_no == sys.SYS_GETAFFINITY) {
                // ignored
            } else if (syscall_no == sys.SYS_MADVISE) {
                // ignored
            } else if (syscall_no == sys.SYS_RTSIGPROCMASK) {
                // ignored
            } else if (syscall_no == sys.SYS_SIGALTSTACK) {
                // ignored
            } else if (syscall_no == sys.SYS_RTSIGACTION) {
                // ignored
            } else if (syscall_no == sys.SYS_PRLIMIT64) {
                // ignored
            } else if (syscall_no == sys.SYS_CLOSE) {
                // ignored
            } else if (syscall_no == sys.SYS_PREAD64) {
                // ignored
            } else if (syscall_no == sys.SYS_STAT) {
                // ignored
            } else if (syscall_no == sys.SYS_FSTAT) {
                // ignored
            } else if (syscall_no == sys.SYS_OPENAT) {
                // ignored
            } else if (syscall_no == sys.SYS_READLINK) {
                // ignored
            } else if (syscall_no == sys.SYS_READLINKAT) {
                // ignored
            } else if (syscall_no == sys.SYS_IOCTL) {
                // ignored
            } else if (syscall_no == sys.SYS_EPOLLCREATE1) {
                // ignored
            } else if (syscall_no == sys.SYS_PIPE2) {
                // ignored
            } else if (syscall_no == sys.SYS_EPOLLCTL) {
                // ignored
            } else if (syscall_no == sys.SYS_EPOLLPWAIT) {
                // ignored
            } else if (syscall_no == sys.SYS_GETRANDOM) {
                // ignored
            } else if (syscall_no == sys.SYS_UNAME) {
                // ignored
            } else if (syscall_no == sys.SYS_GETUID) {
                // ignored
            } else if (syscall_no == sys.SYS_GETGID) {
                // ignored
            } else if (syscall_no == sys.SYS_MINCORE) {
                // ignored
            } else if (syscall_no == sys.SYS_TGKILL) {
                // ignored
            } else if (syscall_no == sys.SYS_SETITIMER) {
                // ignored
            } else if (syscall_no == sys.SYS_TIMERCREATE) {
                // ignored
            } else if (syscall_no == sys.SYS_TIMERSETTIME) {
                // ignored
            } else if (syscall_no == sys.SYS_TIMERDELETE) {
                // ignored
            } else if (syscall_no == sys.SYS_GETRLIMIT) {
                // ignored
            } else if (syscall_no == sys.SYS_LSEEK) {
                // ignored
            } else {
                if (syscall_no == sys.SYS_FSTAT64 || syscall_no == sys.SYS_STAT64 || syscall_no == sys.SYS_LLSEEK) {
                    // noop
                } else {
                    revert("MIPS2: unimplemented syscall");
                }
            }

            st.CpuScalars memory cpu = getCpuScalars(thread);
            sys.handleSyscallUpdates(cpu, thread.registers, v0, v1);
            setStateCpuScalars(thread, cpu);

            updateCurrentThreadRoot();
            out_ = outputState();
        }
    }

    function syscallYield(State memory _state, ThreadState memory _thread) internal returns (bytes32 out_) {
        uint32 v0 = 0;
        uint32 v1 = 0;
        st.CpuScalars memory cpu = getCpuScalars(_thread);
        sys.handleSyscallUpdates(cpu, _thread.registers, v0, v1);
        setStateCpuScalars(_thread, cpu);
        preemptThread(_state, _thread);

        return outputState();
    }

    function execSysRead(
        State memory _state,
        sys.SysReadParams memory _args
    )
        internal
        view
        returns (uint32 v0_, uint32 v1_)
    {
        bool memUpdated;
        uint32 memAddr;
        (v0_, v1_, _state.preimageOffset, _state.memRoot, memUpdated, memAddr) = sys.handleSysRead(_args);
        if (memUpdated) {
            handleMemoryUpdate(_state, memAddr);
        }
    }

    /// @notice Computes the hash of the MIPS state.
    /// @return out_ The hashed MIPS state.
    function outputState() internal returns (bytes32 out_) {
        uint32 exited;
        assembly {
            // copies 'size' bytes, right-aligned in word at 'from', to 'to', incl. trailing data
            function copyMem(from, to, size) -> fromOut, toOut {
                mstore(to, mload(add(from, sub(32, size))))
                fromOut := add(from, 32)
                toOut := add(to, size)
            }

            // From points to the MIPS State
            let from := STATE_MEM_OFFSET

            // Copy to the free memory pointer
            let start := mload(0x40)
            let to := start

            // Copy state to free memory
            from, to := copyMem(from, to, 32) // memRoot
            from, to := copyMem(from, to, 32) // preimageKey
            from, to := copyMem(from, to, 4) // preimageOffset
            from, to := copyMem(from, to, 4) // heap
            from, to := copyMem(from, to, 1) // llReservationStatus
            from, to := copyMem(from, to, 4) // llAddress
            from, to := copyMem(from, to, 4) // llOwnerThread
            let exitCode := mload(from)
            from, to := copyMem(from, to, 1) // exitCode
            exited := mload(from)
            from, to := copyMem(from, to, 1) // exited
            from, to := copyMem(from, to, 8) // step
            from, to := copyMem(from, to, 8) // stepsSinceLastContextSwitch
            from, to := copyMem(from, to, 1) // traverseRight
            from, to := copyMem(from, to, 32) // leftThreadStack
            from, to := copyMem(from, to, 32) // rightThreadStack
            from, to := copyMem(from, to, 4) // nextThreadID

            // Clean up end of memory
            mstore(to, 0)

            // Log the resulting MIPS state, for debugging
            log0(start, sub(to, start))

            // Determine the VM status
            let status := 0
            switch exited
            case 1 {
                switch exitCode
                // VMStatusValid
                case 0 { status := 0 }
                // VMStatusInvalid
                case 1 { status := 1 }
                // VMStatusPanic
                default { status := 2 }
            }
            // VMStatusUnfinished
            default { status := 3 }

            // Compute the hash of the resulting MIPS state and set the status byte
            out_ := keccak256(start, sub(to, start))
            out_ := or(and(not(shl(248, 0xFF)), out_), shl(248, status))
        }

        st.assertExitedIsValid(exited);
    }

    /// @notice Updates the current thread stack root via inner thread root in calldata
    function updateCurrentThreadRoot() internal pure {
        State memory state;
        ThreadState memory thread;
        assembly {
            state := STATE_MEM_OFFSET
            thread := TC_MEM_OFFSET
        }
        bytes32 updatedRoot = computeThreadRoot(loadCalldataInnerThreadRoot(), thread);
        if (state.traverseRight) {
            state.rightThreadStack = updatedRoot;
        } else {
            state.leftThreadStack = updatedRoot;
        }
    }

    /// @notice Preempts the current thread for another and updates the VM state.
    ///         It reads the inner thread root from calldata to update the current thread stack root.
    function preemptThread(
        State memory _state,
        ThreadState memory _thread
    )
        internal
        pure
        returns (bool changedDirections_)
    {
        // pop thread from the current stack and push to the other stack
        if (_state.traverseRight) {
            require(_state.rightThreadStack != EMPTY_THREAD_ROOT, "empty right thread stack");
            _state.rightThreadStack = loadCalldataInnerThreadRoot();
            _state.leftThreadStack = computeThreadRoot(_state.leftThreadStack, _thread);
        } else {
            require(_state.leftThreadStack != EMPTY_THREAD_ROOT, "empty left thread stack");
            _state.leftThreadStack = loadCalldataInnerThreadRoot();
            _state.rightThreadStack = computeThreadRoot(_state.rightThreadStack, _thread);
        }
        bytes32 current = _state.traverseRight ? _state.rightThreadStack : _state.leftThreadStack;
        if (current == EMPTY_THREAD_ROOT) {
            _state.traverseRight = !_state.traverseRight;
            changedDirections_ = true;
        }
        _state.stepsSinceLastContextSwitch = 0;
    }

    /// @notice Pushes a thread to the current thread stack.
    function pushThread(State memory _state, ThreadState memory _thread) internal pure {
        if (_state.traverseRight) {
            _state.rightThreadStack = computeThreadRoot(_state.rightThreadStack, _thread);
        } else {
            _state.leftThreadStack = computeThreadRoot(_state.leftThreadStack, _thread);
        }
        _state.stepsSinceLastContextSwitch = 0;
    }

    /// @notice Removes the current thread from the stack.
    function popThread(State memory _state) internal pure {
        if (_state.traverseRight) {
            _state.rightThreadStack = loadCalldataInnerThreadRoot();
        } else {
            _state.leftThreadStack = loadCalldataInnerThreadRoot();
        }
        bytes32 current = _state.traverseRight ? _state.rightThreadStack : _state.leftThreadStack;
        if (current == EMPTY_THREAD_ROOT) {
            _state.traverseRight = !_state.traverseRight;
        }
        _state.stepsSinceLastContextSwitch = 0;
    }

    /// @notice Returns true if the number of threads is 1
    function lastThreadRemaining(State memory _state) internal pure returns (bool out_) {
        bytes32 inactiveStack = _state.traverseRight ? _state.leftThreadStack : _state.rightThreadStack;
        bool currentStackIsAlmostEmpty = loadCalldataInnerThreadRoot() == EMPTY_THREAD_ROOT;
        return inactiveStack == EMPTY_THREAD_ROOT && currentStackIsAlmostEmpty;
    }

    function computeThreadRoot(bytes32 _currentRoot, ThreadState memory _thread) internal pure returns (bytes32 out_) {
        // w_i = hash(w_0 ++ hash(thread))
        bytes32 threadRoot = outputThreadState(_thread);
        out_ = keccak256(abi.encodePacked(_currentRoot, threadRoot));
    }

    function outputThreadState(ThreadState memory _thread) internal pure returns (bytes32 out_) {
        assembly {
            // copies 'size' bytes, right-aligned in word at 'from', to 'to', incl. trailing data
            function copyMem(from, to, size) -> fromOut, toOut {
                mstore(to, mload(add(from, sub(32, size))))
                fromOut := add(from, 32)
                toOut := add(to, size)
            }

            // From points to the ThreadState
            let from := _thread

            // Copy to the free memory pointer
            let start := mload(0x40)
            let to := start

            // Copy state to free memory
            from, to := copyMem(from, to, 4) // threadID
            from, to := copyMem(from, to, 1) // exitCode
            from, to := copyMem(from, to, 1) // exited
            from, to := copyMem(from, to, 4) // pc
            from, to := copyMem(from, to, 4) // nextPC
            from, to := copyMem(from, to, 4) // lo
            from, to := copyMem(from, to, 4) // hi
            from := mload(from) // offset to registers
            // Copy registers
            for { let i := 0 } lt(i, 32) { i := add(i, 1) } { from, to := copyMem(from, to, 4) }

            // Clean up end of memory
            mstore(to, 0)

            // Compute the hash of the resulting ThreadState
            out_ := keccak256(start, sub(to, start))
        }
    }

    function getCpuScalars(ThreadState memory _tc) internal pure returns (st.CpuScalars memory cpu_) {
        cpu_ = st.CpuScalars({ pc: _tc.pc, nextPC: _tc.nextPC, lo: _tc.lo, hi: _tc.hi });
    }

    function setStateCpuScalars(ThreadState memory _tc, st.CpuScalars memory _cpu) internal pure {
        _tc.pc = _cpu.pc;
        _tc.nextPC = _cpu.nextPC;
        _tc.lo = _cpu.lo;
        _tc.hi = _cpu.hi;
    }

    /// @notice Validates the thread witness in calldata against the current thread.
    function validateCalldataThreadWitness(State memory _state, ThreadState memory _thread) internal pure {
        bytes32 witnessRoot = computeThreadRoot(loadCalldataInnerThreadRoot(), _thread);
        bytes32 expectedRoot = _state.traverseRight ? _state.rightThreadStack : _state.leftThreadStack;
        require(expectedRoot == witnessRoot, "invalid thread witness");
    }

    /// @notice Sets the thread context from calldata.
    function setThreadStateFromCalldata(ThreadState memory _thread) internal pure {
        uint256 s = 0;
        assembly {
            s := calldatasize()
        }
        // verify we have enough calldata
        require(s >= (THREAD_PROOF_OFFSET + PACKED_THREAD_STATE_SIZE), "insufficient calldata for thread witness");

        unchecked {
            assembly {
                function putField(callOffset, memOffset, size) -> callOffsetOut, memOffsetOut {
                    // calldata is packed, thus starting left-aligned, shift-right to pad and right-align
                    let w := shr(shl(3, sub(32, size)), calldataload(callOffset))
                    mstore(memOffset, w)
                    callOffsetOut := add(callOffset, size)
                    memOffsetOut := add(memOffset, 32)
                }

                let c := THREAD_PROOF_OFFSET
                let m := _thread
                c, m := putField(c, m, 4) // threadID
                c, m := putField(c, m, 1) // exitCode
                c, m := putField(c, m, 1) // exited
                c, m := putField(c, m, 4) // pc
                c, m := putField(c, m, 4) // nextPC
                c, m := putField(c, m, 4) // lo
                c, m := putField(c, m, 4) // hi
                m := mload(m) // offset to registers
                // Unpack register calldata into memory
                for { let i := 0 } lt(i, 32) { i := add(i, 1) } { c, m := putField(c, m, 4) }
            }
        }
    }

    /// @notice Loads the inner root for the current thread hash onion from calldata.
    function loadCalldataInnerThreadRoot() internal pure returns (bytes32 innerThreadRoot_) {
        uint256 s = 0;
        assembly {
            s := calldatasize()
            innerThreadRoot_ := calldataload(add(THREAD_PROOF_OFFSET, PACKED_THREAD_STATE_SIZE))
        }
        // verify we have enough calldata
        require(s >= (THREAD_PROOF_OFFSET + PACKED_THREAD_STATE_SIZE + 32), "insufficient calldata for thread witness");
    }

    /// @notice Loads a 32-bit futex value at _vAddr
    function getFutexValue(uint32 _vAddr) internal pure returns (uint32 out_) {
        State memory state;
        assembly {
            state := STATE_MEM_OFFSET
        }

        uint32 effAddr = _vAddr & 0xFFffFFfc;
        uint256 memProofOffset = MIPSMemory.memoryProofOffset(MEM_PROOF_OFFSET, 1);
        return MIPSMemory.readMem(state.memRoot, effAddr, memProofOffset);
    }
}
