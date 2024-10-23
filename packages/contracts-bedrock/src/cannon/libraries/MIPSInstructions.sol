// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { MIPSMemory } from "src/cannon/libraries/MIPSMemory.sol";
import { MIPSState as st } from "src/cannon/libraries/MIPSState.sol";

library MIPSInstructions {
    uint32 internal constant OP_LOAD_LINKED = 0x30;
    uint32 internal constant OP_STORE_CONDITIONAL = 0x38;
    uint32 internal constant REG_RA = 31;

    struct CoreStepLogicParams {
        /// @param opcode The opcode value parsed from insn_.
        st.CpuScalars cpu;
        /// @param registers The CPU registers.
        uint32[32] registers;
        /// @param memRoot The current merkle root of the memory.
        bytes32 memRoot;
        /// @param memProofOffset The offset in calldata specify where the memory merkle proof is located.
        uint256 memProofOffset;
        /// @param insn The current 32-bit instruction at the pc.
        uint32 insn;
        /// @param cpu The CPU scalar fields.
        uint32 opcode;
        /// @param fun The function value parsed from insn_.
        uint32 fun;
    }

    /// @param _pc The program counter.
    /// @param _memRoot The current memory root.
    /// @param _insnProofOffset The calldata offset of the memory proof for the current instruction.
    /// @return insn_ The current 32-bit instruction at the pc.
    /// @return opcode_ The opcode value parsed from insn_.
    /// @return fun_ The function value parsed from insn_.
    function getInstructionDetails(
        uint32 _pc,
        bytes32 _memRoot,
        uint256 _insnProofOffset
    )
        internal
        pure
        returns (uint32 insn_, uint32 opcode_, uint32 fun_)
    {
        unchecked {
            insn_ = MIPSMemory.readMem(_memRoot, _pc, _insnProofOffset);
            opcode_ = insn_ >> 26; // First 6-bits
            fun_ = insn_ & 0x3f; // Last 6-bits

            return (insn_, opcode_, fun_);
        }
    }

    /// @notice Execute core MIPS step logic.
    /// @return newMemRoot_ The updated merkle root of memory after any modifications, may be unchanged.
    /// @return memUpdated_ True if memory was modified.
    /// @return memAddr_ Holds the memory address that was updated if memUpdated_ is true.
    function execMipsCoreStepLogic(CoreStepLogicParams memory _args)
        internal
        pure
        returns (bytes32 newMemRoot_, bool memUpdated_, uint32 memAddr_)
    {
        unchecked {
            newMemRoot_ = _args.memRoot;
            memUpdated_ = false;
            memAddr_ = 0;

            // j-type j/jal
            if (_args.opcode == 2 || _args.opcode == 3) {
                // Take top 4 bits of the next PC (its 256 MB region), and concatenate with the 26-bit offset
                uint32 target = (_args.cpu.nextPC & 0xF0000000) | (_args.insn & 0x03FFFFFF) << 2;
                handleJump(_args.cpu, _args.registers, _args.opcode == 2 ? 0 : 31, target);
                return (newMemRoot_, memUpdated_, memAddr_);
            }

            // register fetch
            uint32 rs = 0; // source register 1 value
            uint32 rt = 0; // source register 2 / temp value
            uint32 rtReg = (_args.insn >> 16) & 0x1F;

            // R-type or I-type (stores rt)
            rs = _args.registers[(_args.insn >> 21) & 0x1F];
            uint32 rdReg = rtReg;

            if (_args.opcode == 0 || _args.opcode == 0x1c) {
                // R-type (stores rd)
                rt = _args.registers[rtReg];
                rdReg = (_args.insn >> 11) & 0x1F;
            } else if (_args.opcode < 0x20) {
                // rt is SignExtImm
                // don't sign extend for andi, ori, xori
                if (_args.opcode == 0xC || _args.opcode == 0xD || _args.opcode == 0xe) {
                    // ZeroExtImm
                    rt = _args.insn & 0xFFFF;
                } else {
                    // SignExtImm
                    rt = signExtend(_args.insn & 0xFFFF, 16);
                }
            } else if (_args.opcode >= 0x28 || _args.opcode == 0x22 || _args.opcode == 0x26) {
                // store rt value with store
                rt = _args.registers[rtReg];

                // store actual rt with lwl and lwr
                rdReg = rtReg;
            }

            if ((_args.opcode >= 4 && _args.opcode < 8) || _args.opcode == 1) {
                handleBranch({
                    _cpu: _args.cpu,
                    _registers: _args.registers,
                    _opcode: _args.opcode,
                    _insn: _args.insn,
                    _rtReg: rtReg,
                    _rs: rs
                });
                return (newMemRoot_, memUpdated_, memAddr_);
            }

            uint32 storeAddr = 0xFF_FF_FF_FF;
            // memory fetch (all I-type)
            // we do the load for stores also
            uint32 mem = 0;
            if (_args.opcode >= 0x20) {
                // M[R[rs]+SignExtImm]
                rs += signExtend(_args.insn & 0xFFFF, 16);
                uint32 addr = rs & 0xFFFFFFFC;
                mem = MIPSMemory.readMem(_args.memRoot, addr, _args.memProofOffset);
                if (_args.opcode >= 0x28) {
                    // store
                    storeAddr = addr;
                    // store opcodes don't write back to a register
                    rdReg = 0;
                }
            }

            // ALU
            // Note: swr outputs more than 4 bytes without the mask 0xffFFffFF
            uint32 val = executeMipsInstruction(_args.insn, _args.opcode, _args.fun, rs, rt, mem) & 0xffFFffFF;

            if (_args.opcode == 0 && _args.fun >= 8 && _args.fun < 0x1c) {
                if (_args.fun == 8 || _args.fun == 9) {
                    // jr/jalr
                    handleJump(_args.cpu, _args.registers, _args.fun == 8 ? 0 : rdReg, rs);
                    return (newMemRoot_, memUpdated_, memAddr_);
                }

                if (_args.fun == 0xa) {
                    // movz
                    handleRd(_args.cpu, _args.registers, rdReg, rs, rt == 0);
                    return (newMemRoot_, memUpdated_, memAddr_);
                }
                if (_args.fun == 0xb) {
                    // movn
                    handleRd(_args.cpu, _args.registers, rdReg, rs, rt != 0);
                    return (newMemRoot_, memUpdated_, memAddr_);
                }

                // lo and hi registers
                // can write back
                if (_args.fun >= 0x10 && _args.fun < 0x1c) {
                    handleHiLo({
                        _cpu: _args.cpu,
                        _registers: _args.registers,
                        _fun: _args.fun,
                        _rs: rs,
                        _rt: rt,
                        _storeReg: rdReg
                    });
                    return (newMemRoot_, memUpdated_, memAddr_);
                }
            }

            // write memory
            if (storeAddr != 0xFF_FF_FF_FF) {
                newMemRoot_ = MIPSMemory.writeMem(storeAddr, _args.memProofOffset, val);
                memUpdated_ = true;
                memAddr_ = storeAddr;
            }

            // write back the value to destination register
            handleRd(_args.cpu, _args.registers, rdReg, val, true);

            return (newMemRoot_, memUpdated_, memAddr_);
        }
    }

    function signExtendImmediate(uint32 _insn) internal pure returns (uint32 offset_) {
        unchecked {
            return signExtend(_insn & 0xFFFF, 16);
        }
    }

    /// @notice Execute an instruction.
    function executeMipsInstruction(
        uint32 _insn,
        uint32 _opcode,
        uint32 _fun,
        uint32 _rs,
        uint32 _rt,
        uint32 _mem
    )
        internal
        pure
        returns (uint32 out_)
    {
        unchecked {
            if (_opcode == 0 || (_opcode >= 8 && _opcode < 0xF)) {
                assembly {
                    // transform ArithLogI to SPECIAL
                    switch _opcode
                    // addi
                    case 0x8 { _fun := 0x20 }
                    // addiu
                    case 0x9 { _fun := 0x21 }
                    // stli
                    case 0xA { _fun := 0x2A }
                    // sltiu
                    case 0xB { _fun := 0x2B }
                    // andi
                    case 0xC { _fun := 0x24 }
                    // ori
                    case 0xD { _fun := 0x25 }
                    // xori
                    case 0xE { _fun := 0x26 }
                }

                // sll
                if (_fun == 0x00) {
                    return _rt << ((_insn >> 6) & 0x1F);
                }
                // srl
                else if (_fun == 0x02) {
                    return _rt >> ((_insn >> 6) & 0x1F);
                }
                // sra
                else if (_fun == 0x03) {
                    uint32 shamt = (_insn >> 6) & 0x1F;
                    return signExtend(_rt >> shamt, 32 - shamt);
                }
                // sllv
                else if (_fun == 0x04) {
                    return _rt << (_rs & 0x1F);
                }
                // srlv
                else if (_fun == 0x6) {
                    return _rt >> (_rs & 0x1F);
                }
                // srav
                else if (_fun == 0x07) {
                    // shamt here is different than the typical shamt which comes from the
                    // instruction itself, here it comes from the rs register
                    uint32 shamt = _rs & 0x1F;
                    return signExtend(_rt >> shamt, 32 - shamt);
                }
                // functs in range [0x8, 0x1b] are handled specially by other functions
                // Explicitly enumerate each funct in range to reduce code diff against Go Vm
                // jr
                else if (_fun == 0x08) {
                    return _rs;
                }
                // jalr
                else if (_fun == 0x09) {
                    return _rs;
                }
                // movz
                else if (_fun == 0x0a) {
                    return _rs;
                }
                // movn
                else if (_fun == 0x0b) {
                    return _rs;
                }
                // syscall
                else if (_fun == 0x0c) {
                    return _rs;
                }
                // 0x0d - break not supported
                // sync
                else if (_fun == 0x0f) {
                    return _rs;
                }
                // mfhi
                else if (_fun == 0x10) {
                    return _rs;
                }
                // mthi
                else if (_fun == 0x11) {
                    return _rs;
                }
                // mflo
                else if (_fun == 0x12) {
                    return _rs;
                }
                // mtlo
                else if (_fun == 0x13) {
                    return _rs;
                }
                // mult
                else if (_fun == 0x18) {
                    return _rs;
                }
                // multu
                else if (_fun == 0x19) {
                    return _rs;
                }
                // div
                else if (_fun == 0x1a) {
                    return _rs;
                }
                // divu
                else if (_fun == 0x1b) {
                    return _rs;
                }
                // The rest includes transformed R-type arith imm instructions
                // add
                else if (_fun == 0x20) {
                    return (_rs + _rt);
                }
                // addu
                else if (_fun == 0x21) {
                    return (_rs + _rt);
                }
                // sub
                else if (_fun == 0x22) {
                    return (_rs - _rt);
                }
                // subu
                else if (_fun == 0x23) {
                    return (_rs - _rt);
                }
                // and
                else if (_fun == 0x24) {
                    return (_rs & _rt);
                }
                // or
                else if (_fun == 0x25) {
                    return (_rs | _rt);
                }
                // xor
                else if (_fun == 0x26) {
                    return (_rs ^ _rt);
                }
                // nor
                else if (_fun == 0x27) {
                    return ~(_rs | _rt);
                }
                // slti
                else if (_fun == 0x2a) {
                    return int32(_rs) < int32(_rt) ? 1 : 0;
                }
                // sltiu
                else if (_fun == 0x2b) {
                    return _rs < _rt ? 1 : 0;
                } else {
                    revert("invalid instruction");
                }
            } else {
                // SPECIAL2
                if (_opcode == 0x1C) {
                    // mul
                    if (_fun == 0x2) {
                        return uint32(int32(_rs) * int32(_rt));
                    }
                    // clz, clo
                    else if (_fun == 0x20 || _fun == 0x21) {
                        if (_fun == 0x20) {
                            _rs = ~_rs;
                        }
                        uint32 i = 0;
                        while (_rs & 0x80000000 != 0) {
                            i++;
                            _rs <<= 1;
                        }
                        return i;
                    }
                }
                // lui
                else if (_opcode == 0x0F) {
                    return _rt << 16;
                }
                // lb
                else if (_opcode == 0x20) {
                    return selectSubWord(_rs, _mem, 1, true);
                }
                // lh
                else if (_opcode == 0x21) {
                    return selectSubWord(_rs, _mem, 2, true);
                }
                // lwl
                else if (_opcode == 0x22) {
                    uint32 val = _mem << ((_rs & 3) * 8);
                    uint32 mask = uint32(0xFFFFFFFF) << ((_rs & 3) * 8);
                    return (_rt & ~mask) | val;
                }
                // lw
                else if (_opcode == 0x23) {
                    return selectSubWord(_rs, _mem, 4, true);
                }
                // lbu
                else if (_opcode == 0x24) {
                    return selectSubWord(_rs, _mem, 1, false);
                }
                //  lhu
                else if (_opcode == 0x25) {
                    return selectSubWord(_rs, _mem, 2, false);
                }
                //  lwr
                else if (_opcode == 0x26) {
                    uint32 val = _mem >> (24 - (_rs & 3) * 8);
                    uint32 mask = uint32(0xFFFFFFFF) >> (24 - (_rs & 3) * 8);
                    return (_rt & ~mask) | val;
                }
                //  sb
                else if (_opcode == 0x28) {
                    return updateSubWord(_rs, _mem, 1, _rt);
                }
                //  sh
                else if (_opcode == 0x29) {
                    return updateSubWord(_rs, _mem, 2, _rt);
                }
                //  swl
                else if (_opcode == 0x2a) {
                    uint32 val = _rt >> ((_rs & 3) * 8);
                    uint32 mask = uint32(0xFFFFFFFF) >> ((_rs & 3) * 8);
                    return (_mem & ~mask) | val;
                }
                //  sw
                else if (_opcode == 0x2b) {
                    return updateSubWord(_rs, _mem, 4, _rt);
                }
                //  swr
                else if (_opcode == 0x2e) {
                    uint32 val = _rt << (24 - (_rs & 3) * 8);
                    uint32 mask = uint32(0xFFFFFFFF) << (24 - (_rs & 3) * 8);
                    return (_mem & ~mask) | val;
                } else {
                    revert("invalid instruction");
                }
            }
            revert("invalid instruction");
        }
    }

    /// @notice Extends the value leftwards with its most significant bit (sign extension).
    function signExtend(uint32 _dat, uint32 _idx) internal pure returns (uint32 out_) {
        unchecked {
            bool isSigned = (_dat >> (_idx - 1)) != 0;
            uint256 signed = ((1 << (32 - _idx)) - 1) << _idx;
            uint256 mask = (1 << _idx) - 1;
            return uint32(_dat & mask | (isSigned ? signed : 0));
        }
    }

    /// @notice Handles a branch instruction, updating the MIPS state PC where needed.
    /// @param _cpu Holds the state of cpu scalars pc, nextPC, hi, lo.
    /// @param _registers Holds the current state of the cpu registers.
    /// @param _opcode The opcode of the branch instruction.
    /// @param _insn The instruction to be executed.
    /// @param _rtReg The register to be used for the branch.
    /// @param _rs The register to be compared with the branch register.
    function handleBranch(
        st.CpuScalars memory _cpu,
        uint32[32] memory _registers,
        uint32 _opcode,
        uint32 _insn,
        uint32 _rtReg,
        uint32 _rs
    )
        internal
        pure
    {
        unchecked {
            bool shouldBranch = false;

            if (_cpu.nextPC != _cpu.pc + 4) {
                revert("branch in delay slot");
            }

            // beq/bne: Branch on equal / not equal
            if (_opcode == 4 || _opcode == 5) {
                uint32 rt = _registers[_rtReg];
                shouldBranch = (_rs == rt && _opcode == 4) || (_rs != rt && _opcode == 5);
            }
            // blez: Branches if instruction is less than or equal to zero
            else if (_opcode == 6) {
                shouldBranch = int32(_rs) <= 0;
            }
            // bgtz: Branches if instruction is greater than zero
            else if (_opcode == 7) {
                shouldBranch = int32(_rs) > 0;
            }
            // bltz/bgez: Branch on less than zero / greater than or equal to zero
            else if (_opcode == 1) {
                // regimm
                uint32 rtv = ((_insn >> 16) & 0x1F);
                if (rtv == 0) {
                    shouldBranch = int32(_rs) < 0;
                }
                // bltzal
                if (rtv == 0x10) {
                    shouldBranch = int32(_rs) < 0;
                    _registers[31] = _cpu.pc + 8; // always set regardless of branch taken
                }
                if (rtv == 1) {
                    shouldBranch = int32(_rs) >= 0;
                }
                // bgezal (i.e. bal mnemonic)
                if (rtv == 0x11) {
                    shouldBranch = int32(_rs) >= 0;
                    _registers[REG_RA] = _cpu.pc + 8; // always set regardless of branch taken
                }
            }

            // Update the state's previous PC
            uint32 prevPC = _cpu.pc;

            // Execute the delay slot first
            _cpu.pc = _cpu.nextPC;

            // If we should branch, update the PC to the branch target
            // Otherwise, proceed to the next instruction
            if (shouldBranch) {
                _cpu.nextPC = prevPC + 4 + (signExtend(_insn & 0xFFFF, 16) << 2);
            } else {
                _cpu.nextPC = _cpu.nextPC + 4;
            }
        }
    }

    /// @notice Handles HI and LO register instructions.
    /// @param _cpu Holds the state of cpu scalars pc, nextPC, hi, lo.
    /// @param _registers Holds the current state of the cpu registers.
    /// @param _fun The function code of the instruction.
    /// @param _rs The value of the RS register.
    /// @param _rt The value of the RT register.
    /// @param _storeReg The register to store the result in.
    function handleHiLo(
        st.CpuScalars memory _cpu,
        uint32[32] memory _registers,
        uint32 _fun,
        uint32 _rs,
        uint32 _rt,
        uint32 _storeReg
    )
        internal
        pure
    {
        unchecked {
            uint32 val = 0;

            // mfhi: Move the contents of the HI register into the destination
            if (_fun == 0x10) {
                val = _cpu.hi;
            }
            // mthi: Move the contents of the source into the HI register
            else if (_fun == 0x11) {
                _cpu.hi = _rs;
            }
            // mflo: Move the contents of the LO register into the destination
            else if (_fun == 0x12) {
                val = _cpu.lo;
            }
            // mtlo: Move the contents of the source into the LO register
            else if (_fun == 0x13) {
                _cpu.lo = _rs;
            }
            // mult: Multiplies `rs` by `rt` and stores the result in HI and LO registers
            else if (_fun == 0x18) {
                uint64 acc = uint64(int64(int32(_rs)) * int64(int32(_rt)));
                _cpu.hi = uint32(acc >> 32);
                _cpu.lo = uint32(acc);
            }
            // multu: Unsigned multiplies `rs` by `rt` and stores the result in HI and LO registers
            else if (_fun == 0x19) {
                uint64 acc = uint64(uint64(_rs) * uint64(_rt));
                _cpu.hi = uint32(acc >> 32);
                _cpu.lo = uint32(acc);
            }
            // div: Divides `rs` by `rt`.
            // Stores the quotient in LO
            // And the remainder in HI
            else if (_fun == 0x1a) {
                if (int32(_rt) == 0) {
                    revert("MIPS: division by zero");
                }
                _cpu.hi = uint32(int32(_rs) % int32(_rt));
                _cpu.lo = uint32(int32(_rs) / int32(_rt));
            }
            // divu: Unsigned divides `rs` by `rt`.
            // Stores the quotient in LO
            // And the remainder in HI
            else if (_fun == 0x1b) {
                if (_rt == 0) {
                    revert("MIPS: division by zero");
                }
                _cpu.hi = _rs % _rt;
                _cpu.lo = _rs / _rt;
            }

            // Store the result in the destination register, if applicable
            if (_storeReg != 0) {
                _registers[_storeReg] = val;
            }

            // Update the PC
            _cpu.pc = _cpu.nextPC;
            _cpu.nextPC = _cpu.nextPC + 4;
        }
    }

    /// @notice Handles a jump instruction, updating the MIPS state PC where needed.
    /// @param _cpu Holds the state of cpu scalars pc, nextPC, hi, lo.
    /// @param _registers Holds the current state of the cpu registers.
    /// @param _linkReg The register to store the link to the instruction after the delay slot instruction.
    /// @param _dest The destination to jump to.
    function handleJump(
        st.CpuScalars memory _cpu,
        uint32[32] memory _registers,
        uint32 _linkReg,
        uint32 _dest
    )
        internal
        pure
    {
        unchecked {
            if (_cpu.nextPC != _cpu.pc + 4) {
                revert("jump in delay slot");
            }

            // Update the next PC to the jump destination.
            uint32 prevPC = _cpu.pc;
            _cpu.pc = _cpu.nextPC;
            _cpu.nextPC = _dest;

            // Update the link-register to the instruction after the delay slot instruction.
            if (_linkReg != 0) {
                _registers[_linkReg] = prevPC + 8;
            }
        }
    }

    /// @notice Handles a storing a value into a register.
    /// @param _cpu Holds the state of cpu scalars pc, nextPC, hi, lo.
    /// @param _registers Holds the current state of the cpu registers.
    /// @param _storeReg The register to store the value into.
    /// @param _val The value to store.
    /// @param _conditional Whether or not the store is conditional.
    function handleRd(
        st.CpuScalars memory _cpu,
        uint32[32] memory _registers,
        uint32 _storeReg,
        uint32 _val,
        bool _conditional
    )
        internal
        pure
    {
        unchecked {
            // The destination register must be valid.
            require(_storeReg < 32, "valid register");

            // Never write to reg 0, and it can be conditional (movz, movn).
            if (_storeReg != 0 && _conditional) {
                _registers[_storeReg] = _val;
            }

            // Update the PC.
            _cpu.pc = _cpu.nextPC;
            _cpu.nextPC = _cpu.nextPC + 4;
        }
    }

    /// @notice Selects a subword of byteLength size contained in memWord based on the low-order bits of vaddr
    /// @param _vaddr The virtual address of the the subword.
    /// @param _memWord The full word to select a subword from.
    /// @param _byteLength The size of the subword.
    /// @param _signExtend Whether to sign extend the selected subwrod.
    function selectSubWord(
        uint32 _vaddr,
        uint32 _memWord,
        uint32 _byteLength,
        bool _signExtend
    )
        internal
        pure
        returns (uint32 retval_)
    {
        (uint32 dataMask, uint32 bitOffset, uint32 bitLength) = calculateSubWordMaskAndOffset(_vaddr, _byteLength);
        retval_ = (_memWord >> bitOffset) & dataMask;
        if (_signExtend) {
            retval_ = signExtend(retval_, bitLength);
        }
        return retval_;
    }

    /// @notice Returns a word that has been updated by the specified subword at bit positions determined by the virtual
    /// address
    /// @param _vaddr The virtual address of the subword.
    /// @param _memWord The full word to update.
    /// @param _byteLength The size of the subword.
    /// @param _value The subword that updates _memWord.
    function updateSubWord(
        uint32 _vaddr,
        uint32 _memWord,
        uint32 _byteLength,
        uint32 _value
    )
        internal
        pure
        returns (uint32 word_)
    {
        (uint32 dataMask, uint32 bitOffset,) = calculateSubWordMaskAndOffset(_vaddr, _byteLength);
        uint32 subWordValue = dataMask & _value;
        uint32 memUpdateMask = dataMask << bitOffset;
        return subWordValue << bitOffset | (~memUpdateMask) & _memWord;
    }

    function calculateSubWordMaskAndOffset(
        uint32 _vaddr,
        uint32 _byteLength
    )
        internal
        pure
        returns (uint32 dataMask_, uint32 bitOffset_, uint32 bitLength_)
    {
        uint32 bitLength = _byteLength << 3;
        uint32 dataMask = ~uint32(0) >> (32 - bitLength);

        // Figure out sub-word index based on the low-order bits in vaddr
        uint32 byteIndexMask = _vaddr & 0x3 & ~(_byteLength - 1);
        uint32 maxByteShift = 4 - _byteLength;
        uint32 byteIndex = _vaddr & byteIndexMask;
        uint32 bitOffset = (maxByteShift - byteIndex) << 3;

        return (dataMask, bitOffset, bitLength);
    }
}
