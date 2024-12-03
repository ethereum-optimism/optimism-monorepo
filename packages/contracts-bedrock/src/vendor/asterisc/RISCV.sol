// SPDX-License-Identifier: MIT
pragma solidity 0.8.25;

import { IPreimageOracle } from "interfaces/cannon/IPreimageOracle.sol";
import { IBigStepper } from "interfaces/dispute/IBigStepper.sol";

/// @title RISCV
/// @notice The RISCV contract emulates a single RISCV hart cycle statelessly, using memory proofs to verify the
///         instruction and optional memory access' inclusion in the memory merkle root provided in the trusted
///         prestate witness.
/// @dev https://github.com/ethereum-optimism/asterisc
contract RISCV is IBigStepper {
    /// @notice The preimage oracle contract.
    IPreimageOracle public oracle;

    /// @notice The version of the contract.
    /// @custom:semver 1.2.0
    string public constant version = "1.2.0";

    /// @param _oracle The preimage oracle contract.
    constructor(IPreimageOracle _oracle) {
        oracle = _oracle;
    }

    /// @inheritdoc IBigStepper
    function step(bytes calldata _stateData, bytes calldata _proof, bytes32 _localContext) public returns (bytes32) {
        assembly {
            function revertWithCode(code) {
                mstore(0, code)
                revert(0, 0x20)
            }

            function preimageOraclePos() -> out {
                // slot of preimageOraclePos field
                out := 0
            }

            //
            // Yul64 - functions to do 64 bit math - see yul64.go
            //
            function u64Mask() -> out {
                // max uint64
                out := shr(192, not(0)) // 256-64 = 192
            }

            function u32Mask() -> out {
                out := U64(shr(toU256(224), not(0))) // 256-32 = 224
            }

            function toU64(v) -> out {
                out := v
            }

            function shortToU64(v) -> out {
                out := v
            }

            function shortToU256(v) -> out {
                out := v
            }

            function longToU256(v) -> out {
                out := v
            }

            function u256ToU64(v) -> out {
                out := and(v, U256(u64Mask()))
            }

            function u64ToU256(v) -> out {
                out := v
            }

            function mask32Signed64(v) -> out {
                out := signExtend64(and64(v, u32Mask()), toU64(31))
            }

            function u64Mod() -> out {
                // 1 << 64
                out := shl(toU256(64), toU256(1))
            }

            function u64TopBit() -> out {
                // 1 << 63
                out := shl(toU256(63), toU256(1))
            }

            function signExtend64(v, bit) -> out {
                switch and(v, shl(bit, 1))
                case 0 {
                    // fill with zeroes, by masking
                    out := U64(and(U256(v), shr(sub(toU256(63), bit), U256(u64Mask()))))
                }
                default {
                    // fill with ones, by or-ing
                    out := U64(or(U256(v), shl(bit, shr(bit, U256(u64Mask())))))
                }
            }

            function signExtend64To256(v) -> out {
                switch and(U256(v), u64TopBit())
                case 0 { out := v }
                default { out := or(shl(toU256(64), not(0)), v) }
            }

            function add64(x, y) -> out {
                out := U64(mod(add(U256(x), U256(y)), u64Mod()))
            }

            function sub64(x, y) -> out {
                out := U64(mod(sub(U256(x), U256(y)), u64Mod()))
            }

            function mul64(x, y) -> out {
                out := u256ToU64(mul(U256(x), U256(y)))
            }

            function div64(x, y) -> out {
                out := u256ToU64(div(U256(x), U256(y)))
            }

            function sdiv64(x, y) -> out {
                // note: signed overflow semantics are the same between Go and EVM assembly
                out := u256ToU64(sdiv(signExtend64To256(x), signExtend64To256(y)))
            }

            function mod64(x, y) -> out {
                out := U64(mod(U256(x), U256(y)))
            }

            function smod64(x, y) -> out {
                out := u256ToU64(smod(signExtend64To256(x), signExtend64To256(y)))
            }

            function not64(x) -> out {
                out := u256ToU64(not(U256(x)))
            }

            function lt64(x, y) -> out {
                out := U64(lt(U256(x), U256(y)))
            }

            function gt64(x, y) -> out {
                out := U64(gt(U256(x), U256(y)))
            }

            function slt64(x, y) -> out {
                out := U64(slt(signExtend64To256(x), signExtend64To256(y)))
            }

            function sgt64(x, y) -> out {
                out := U64(sgt(signExtend64To256(x), signExtend64To256(y)))
            }

            function eq64(x, y) -> out {
                out := U64(eq(U256(x), U256(y)))
            }

            function iszero64(x) -> out {
                out := iszero(U256(x))
            }

            function and64(x, y) -> out {
                out := U64(and(U256(x), U256(y)))
            }

            function or64(x, y) -> out {
                out := U64(or(U256(x), U256(y)))
            }

            function xor64(x, y) -> out {
                out := U64(xor(U256(x), U256(y)))
            }

            function shl64(x, y) -> out {
                out := u256ToU64(shl(U256(x), U256(y)))
            }

            function shr64(x, y) -> out {
                out := U64(shr(U256(x), U256(y)))
            }

            function sar64(x, y) -> out {
                out := u256ToU64(sar(U256(x), signExtend64To256(y)))
            }

            // type casts, no-op in yul
            function b32asBEWord(v) -> out {
                out := v
            }
            function beWordAsB32(v) -> out {
                out := v
            }
            function U64(v) -> out {
                out := v
            }
            function U256(v) -> out {
                out := v
            }
            function toU256(v) -> out {
                out := v
            }

            //
            // Bit hacking util
            //
            function bitlen(x) -> n {
                if gt(x, sub(shl(128, 1), 1)) {
                    x := shr(128, x)
                    n := add(n, 128)
                }
                if gt(x, sub(shl(64, 1), 1)) {
                    x := shr(64, x)
                    n := add(n, 64)
                }
                if gt(x, sub(shl(32, 1), 1)) {
                    x := shr(32, x)
                    n := add(n, 32)
                }
                if gt(x, sub(shl(16, 1), 1)) {
                    x := shr(16, x)
                    n := add(n, 16)
                }
                if gt(x, sub(shl(8, 1), 1)) {
                    x := shr(8, x)
                    n := add(n, 8)
                }
                if gt(x, sub(shl(4, 1), 1)) {
                    x := shr(4, x)
                    n := add(n, 4)
                }
                if gt(x, sub(shl(2, 1), 1)) {
                    x := shr(2, x)
                    n := add(n, 2)
                }
                if gt(x, sub(shl(1, 1), 1)) {
                    x := shr(1, x)
                    n := add(n, 1)
                }
                if gt(x, 0) { n := add(n, 1) }
            }

            function endianSwap(x) -> out {
                for { let i := 0 } lt(i, 32) { i := add(i, 1) } {
                    out := or(shl(8, out), and(x, 0xff))
                    x := shr(8, x)
                }
            }

            //
            // State layout
            //
            function stateSizeMemRoot() -> out {
                out := 32
            }
            function stateSizePreimageKey() -> out {
                out := 32
            }
            function stateSizePreimageOffset() -> out {
                out := 8
            }
            function stateSizePC() -> out {
                out := 8
            }
            function stateSizeExitCode() -> out {
                out := 1
            }
            function stateSizeExited() -> out {
                out := 1
            }
            function stateSizeStep() -> out {
                out := 8
            }
            function stateSizeHeap() -> out {
                out := 8
            }
            function stateSizeLoadReservation() -> out {
                out := 8
            }
            function stateSizeRegisters() -> out {
                out := mul(8, 32)
            }

            function stateOffsetMemRoot() -> out {
                out := 0
            }
            function stateOffsetPreimageKey() -> out {
                out := add(stateOffsetMemRoot(), stateSizeMemRoot())
            }
            function stateOffsetPreimageOffset() -> out {
                out := add(stateOffsetPreimageKey(), stateSizePreimageKey())
            }
            function stateOffsetPC() -> out {
                out := add(stateOffsetPreimageOffset(), stateSizePreimageOffset())
            }
            function stateOffsetExitCode() -> out {
                out := add(stateOffsetPC(), stateSizePC())
            }
            function stateOffsetExited() -> out {
                out := add(stateOffsetExitCode(), stateSizeExitCode())
            }
            function stateOffsetStep() -> out {
                out := add(stateOffsetExited(), stateSizeExited())
            }
            function stateOffsetHeap() -> out {
                out := add(stateOffsetStep(), stateSizeStep())
            }
            function stateOffsetLoadReservation() -> out {
                out := add(stateOffsetHeap(), stateSizeHeap())
            }
            function stateOffsetRegisters() -> out {
                out := add(stateOffsetLoadReservation(), stateSizeLoadReservation())
            }
            function stateSize() -> out {
                out := add(stateOffsetRegisters(), stateSizeRegisters())
            }

            //
            // Initial EVM memory / calldata checks
            //
            if iszero(eq(mload(0x40), 0x80)) {
                // expected memory check: no allocated memory (start after scratch + free-mem-ptr + zero slot = 0x80)
                revert(0, 0)
            }
            if iszero(eq(_stateData.offset, 132)) {
                // 32*4+4 = 132 expected state data offset
                revert(0, 0)
            }
            if iszero(eq(calldataload(sub(_stateData.offset, 32)), stateSize())) {
                // user-provided state size must match expected state size
                revert(0, 0)
            }
            function paddedLen(v) -> out {
                // padded to multiple of 32 bytes
                let padding := mod(sub(32, mod(v, 32)), 32)
                out := add(v, padding)
            }
            if iszero(eq(_proof.offset, add(add(_stateData.offset, paddedLen(stateSize())), 32))) {
                // 132+stateSize+padding+32 = expected proof offset
                revert(0, 0)
            }
            function proofContentOffset() -> out {
                // since we can't reference proof.offset in functions, blame Yul
                // 132+362+(32-362%32)+32=548
                out := 548
            }
            if iszero(eq(_proof.offset, proofContentOffset())) { revert(0, 0) }

            //
            // State loading
            //
            function memStateOffset() -> out {
                out := 0x80
            }
            // copy the state calldata into memory, so we can mutate it
            mstore(0x40, add(memStateOffset(), stateSize())) // alloc, update free mem pointer
            calldatacopy(memStateOffset(), _stateData.offset, stateSize()) // same format in memory as in calldata

            //
            // State access
            //
            function readState(offset, length) -> out {
                out := mload(add(memStateOffset(), offset)) // note: the state variables are all big-endian encoded
                out := shr(shl(3, sub(32, length)), out) // shift-right to right-align data and reduce to desired length
            }
            function writeState(offset, length, data) {
                let memOffset := add(memStateOffset(), offset)
                // left-aligned mask of length bytes
                let mask := shl(shl(3, sub(32, length)), not(0))
                let prev := mload(memOffset)
                // align data to left
                data := shl(shl(3, sub(32, length)), data)
                // mask out data from previous word, and apply new data
                let result := or(and(prev, not(mask)), data)
                mstore(memOffset, result)
            }

            function getMemRoot() -> out {
                out := readState(stateOffsetMemRoot(), stateSizeMemRoot())
            }
            function setMemRoot(v) {
                writeState(stateOffsetMemRoot(), stateSizeMemRoot(), v)
            }

            function getPreimageKey() -> out {
                out := readState(stateOffsetPreimageKey(), stateSizePreimageKey())
            }
            function setPreimageKey(k) {
                writeState(stateOffsetPreimageKey(), stateSizePreimageKey(), k)
            }

            function getPreimageOffset() -> out {
                out := readState(stateOffsetPreimageOffset(), stateSizePreimageOffset())
            }
            function setPreimageOffset(v) {
                writeState(stateOffsetPreimageOffset(), stateSizePreimageOffset(), v)
            }

            function getPC() -> out {
                out := readState(stateOffsetPC(), stateSizePC())
            }
            function setPC(v) {
                writeState(stateOffsetPC(), stateSizePC(), v)
            }

            function getExited() -> out {
                out := readState(stateOffsetExited(), stateSizeExited())
            }
            function setExited() {
                writeState(stateOffsetExited(), stateSizeExited(), 1)
            }

            function getExitCode() -> out {
                out := readState(stateOffsetExitCode(), stateSizeExitCode())
            }
            function setExitCode(v) {
                writeState(stateOffsetExitCode(), stateSizeExitCode(), v)
            }

            function getStep() -> out {
                out := readState(stateOffsetStep(), stateSizeStep())
            }
            function setStep(v) {
                writeState(stateOffsetStep(), stateSizeStep(), v)
            }

            function getHeap() -> out {
                out := readState(stateOffsetHeap(), stateSizeHeap())
            }
            function setHeap(v) {
                writeState(stateOffsetHeap(), stateSizeHeap(), v)
            }

            function getLoadReservation() -> out {
                out := readState(stateOffsetLoadReservation(), stateSizeLoadReservation())
            }
            function setLoadReservation(addr) {
                writeState(stateOffsetLoadReservation(), stateSizeLoadReservation(), addr)
            }

            function getRegister(reg) -> out {
                if gt64(reg, toU64(31)) { revertWithCode(0xbad4e9) } // cannot load invalid register

                let offset := add64(toU64(stateOffsetRegisters()), mul64(reg, toU64(8)))
                out := readState(offset, 8)
            }
            function setRegister(reg, v) {
                if iszero64(reg) {
                    // reg 0 must stay 0
                    // v is a HINT, but no hints are specified by standard spec, or used by us.
                    leave
                }
                if gt64(reg, toU64(31)) { revertWithCode(0xbad4e9) } // unknown register

                let offset := add64(toU64(stateOffsetRegisters()), mul64(reg, toU64(8)))
                writeState(offset, 8, v)
            }

            //
            // State output
            //
            function vmStatus() -> status {
                switch getExited()
                case 1 {
                    switch getExitCode()
                    case 0 { status := 0 }
                    // VMStatusValid
                    case 1 { status := 1 }
                    // VMStatusInvalid
                    default { status := 2 } // VMStatusPanic
                }
                default { status := 3 } // VMStatusUnfinished
            }

            function computeStateHash() -> out {
                // Log the RISC-V state for debugging
                log0(memStateOffset(), stateSize())

                out := keccak256(memStateOffset(), stateSize())
                out := or(and(not(shl(248, 0xFF)), out), shl(248, vmStatus()))
            }

            //
            // Parse - functions to parse RISC-V instructions - see parse.go
            //
            function parseImmTypeI(instr) -> out {
                out := signExtend64(shr64(toU64(20), instr), toU64(11))
            }

            function parseImmTypeS(instr) -> out {
                out :=
                    signExtend64(
                        or64(shl64(toU64(5), shr64(toU64(25), instr)), and64(shr64(toU64(7), instr), toU64(0x1F))),
                        toU64(11)
                    )
            }

            function parseImmTypeB(instr) -> out {
                out :=
                    signExtend64(
                        or64(
                            or64(
                                shl64(toU64(1), and64(shr64(toU64(8), instr), toU64(0xF))),
                                shl64(toU64(5), and64(shr64(toU64(25), instr), toU64(0x3F)))
                            ),
                            or64(
                                shl64(toU64(11), and64(shr64(toU64(7), instr), toU64(1))),
                                shl64(toU64(12), shr64(toU64(31), instr))
                            )
                        ),
                        toU64(12)
                    )
            }

            function parseImmTypeU(instr) -> out {
                out := signExtend64(shr64(toU64(12), instr), toU64(19))
            }

            function parseImmTypeJ(instr) -> out {
                out :=
                    signExtend64(
                        or64(
                            or64(
                                and64(shr64(toU64(21), instr), shortToU64(0x3FF)), // 10 bits for index 0:9
                                shl64(toU64(10), and64(shr64(toU64(20), instr), toU64(1))) // 1 bit for index 10
                            ),
                            or64(
                                shl64(toU64(11), and64(shr64(toU64(12), instr), toU64(0xFF))), // 8 bits for index 11:18
                                shl64(toU64(19), shr64(toU64(31), instr)) // 1 bit for index 19
                            )
                        ),
                        toU64(19)
                    )
            }

            function parseOpcode(instr) -> out {
                out := and64(instr, toU64(0x7F))
            }

            function parseRd(instr) -> out {
                out := and64(shr64(toU64(7), instr), toU64(0x1F))
            }

            function parseFunct3(instr) -> out {
                out := and64(shr64(toU64(12), instr), toU64(0x7))
            }

            function parseRs1(instr) -> out {
                out := and64(shr64(toU64(15), instr), toU64(0x1F))
            }

            function parseRs2(instr) -> out {
                out := and64(shr64(toU64(20), instr), toU64(0x1F))
            }

            function parseFunct7(instr) -> out {
                out := shr64(toU64(25), instr)
            }

            //
            // Memory functions
            //
            function proofOffset(proofIndex) -> offset {
                // proof size: 64-5+1=60 (a 64-bit mem-address branch to 32 byte leaf, incl leaf itself), all 32 bytes
                offset := mul64(mul64(toU64(proofIndex), toU64(60)), toU64(32))
                offset := add64(offset, proofContentOffset())
            }

            function hashPair(a, b) -> h {
                mstore(0, a)
                mstore(0x20, b)
                h := keccak256(0, 0x40)
            }

            function getMemoryB32(addr, proofIndex) -> out {
                if and64(addr, toU64(31)) {
                    // quick addr alignment check
                    revertWithCode(0xbad10ad0) // addr not aligned with 32 bytes
                }
                let offset := proofOffset(proofIndex)
                let leaf := calldataload(offset)
                offset := add64(offset, toU64(32))

                let path := shr64(toU64(5), addr) // 32 bytes of memory per leaf
                let node := leaf // starting from the leaf node, work back up by combining with siblings, to reconstruct
                    // the root
                for { let i := 0 } lt(i, sub(64, 5)) { i := add(i, 1) } {
                    let sibling := calldataload(offset)
                    offset := add64(offset, toU64(32))
                    switch and64(shr64(toU64(i), path), toU64(1))
                    case 0 { node := hashPair(node, sibling) }
                    case 1 { node := hashPair(sibling, node) }
                }
                let memRoot := getMemRoot()
                if iszero(eq(b32asBEWord(node), b32asBEWord(memRoot))) {
                    // verify the root matches
                    revertWithCode(0xbadf00d1) // bad memory proof
                }
                out := leaf
            }

            // warning: setMemoryB32 does not verify the proof,
            // it assumes the same memory proof has been verified with getMemoryB32
            function setMemoryB32(addr, v, proofIndex) {
                if and64(addr, toU64(31)) { revertWithCode(0xbad10ad0) } // addr not aligned with 32 bytes

                let offset := proofOffset(proofIndex)
                let leaf := v
                offset := add64(offset, toU64(32))
                let path := shr64(toU64(5), addr) // 32 bytes of memory per leaf
                let node := leaf // starting from the leaf node, work back up by combining with siblings, to reconstruct
                    // the root
                for { let i := 0 } lt(i, sub(64, 5)) { i := add(i, 1) } {
                    let sibling := calldataload(offset)
                    offset := add64(offset, toU64(32))

                    switch and64(shr64(toU64(i), path), toU64(1))
                    case 0 { node := hashPair(node, sibling) }
                    case 1 { node := hashPair(sibling, node) }
                }
                setMemRoot(node) // store new memRoot
            }

            // load unaligned, optionally signed, little-endian, integer of 1 ... 8 bytes from memory
            function loadMem(addr, size, signed, proofIndexL, proofIndexR) -> out {
                if gt(size, 8) { revertWithCode(0xbad512e0) } // cannot load more than 8 bytes
                // load/verify left part
                let leftAddr := and64(addr, not64(toU64(31)))
                let left := b32asBEWord(getMemoryB32(leftAddr, proofIndexL))
                let alignment := sub64(addr, leftAddr)

                let right := 0
                let rightAddr := and64(add64(addr, sub64(size, toU64(1))), not64(toU64(31)))
                let leftShamt := sub64(sub64(toU64(32), alignment), size)
                let rightShamt := toU64(0)
                if iszero64(eq64(leftAddr, rightAddr)) {
                    // if unaligned, use second proof for the right part
                    if eq(proofIndexR, 0xff) { revertWithCode(0xbad22220) } // unexpected need for right-side proof in
                        // loadMem
                    // load/verify right part
                    right := b32asBEWord(getMemoryB32(rightAddr, proofIndexR))
                    // left content is aligned to right of 32 bytes
                    leftShamt := toU64(0)
                    rightShamt := sub64(sub64(toU64(64), alignment), size)
                }

                let addr_ := addr
                let size_ := size
                // left: prepare for byte-taking by right-aligning
                left := shr(u64ToU256(shl64(toU64(3), leftShamt)), left)
                // right: right-align for byte-taking by right-aligning
                right := shr(u64ToU256(shl64(toU64(3), rightShamt)), right)
                // loop:
                for { let i := 0 } lt(i, size_) { i := add(i, 1) } {
                    // translate to reverse byte lookup, since we are reading little-endian memory, and need the highest
                    // byte first.
                    // effAddr := (addr + size - 1 - i) &^ 31
                    let effAddr := and64(sub64(sub64(add64(addr_, size_), toU64(1)), toU64(i)), not64(toU64(31)))
                    // take a byte from either left or right, depending on the effective address
                    let b := toU256(0)
                    switch eq64(effAddr, leftAddr)
                    case 1 {
                        b := and(left, toU256(0xff))
                        left := shr(toU256(8), left)
                    }
                    case 0 {
                        b := and(right, toU256(0xff))
                        right := shr(toU256(8), right)
                    }
                    // append it to the output
                    out := or64(shl64(toU64(8), out), u256ToU64(b))
                }

                if signed {
                    let signBitShift := sub64(shl64(toU64(3), size_), toU64(1))
                    out := signExtend64(out, signBitShift)
                }
            }

            // Splits the value into a left and a right part, each with a mask (identify data) and a patch (diff
            // content).
            function leftAndRight(alignment, size, value) -> leftMask, rightMask, leftPatch, rightPatch {
                let start := alignment
                let end := add64(alignment, size)
                for { let i := 0 } lt(i, 64) { i := add(i, 1) } {
                    let index := toU64(i)
                    let leftSide := lt64(index, toU64(32))
                    switch leftSide
                    case 1 {
                        leftPatch := shl(8, leftPatch)
                        leftMask := shl(8, leftMask)
                    }
                    case 0 {
                        rightPatch := shl(8, rightPatch)
                        rightMask := shl(8, rightMask)
                    }
                    if and64(eq64(lt64(index, start), toU64(0)), lt64(index, end)) {
                        // if alignment <= i < alignment+size
                        let b := and(shr(u64ToU256(shl64(toU64(3), sub64(index, alignment))), value), toU256(0xff))
                        switch leftSide
                        case 1 {
                            leftPatch := or(leftPatch, b)
                            leftMask := or(leftMask, toU256(0xff))
                        }
                        case 0 {
                            rightPatch := or(rightPatch, b)
                            rightMask := or(rightMask, toU256(0xff))
                        }
                    }
                }
            }

            function storeMemUnaligned(addr, size, value, proofIndexL, proofIndexR) {
                if gt(size, 32) { revertWithCode(0xbad512e1) } // cannot store more than 32 bytes

                let leftAddr := and64(addr, not64(toU64(31)))
                let rightAddr := and64(add64(addr, sub64(size, toU64(1))), not64(toU64(31)))
                let alignment := sub64(addr, leftAddr)
                let leftMask, rightMask, leftPatch, rightPatch := leftAndRight(alignment, size, value)

                // load the left base
                let left := b32asBEWord(getMemoryB32(leftAddr, proofIndexL))
                // apply the left patch
                left := or(and(left, not(leftMask)), leftPatch)
                // write the left
                setMemoryB32(leftAddr, beWordAsB32(left), proofIndexL)

                // if aligned: nothing more to do here
                if eq64(leftAddr, rightAddr) { leave }
                if eq(proofIndexR, 0xff) { revertWithCode(0xbad22221) } // unexpected need for right-side proof in
                    // storeMem
                // load the right base (with updated mem root)
                let right := b32asBEWord(getMemoryB32(rightAddr, proofIndexR))
                // apply the right patch
                right := or(and(right, not(rightMask)), rightPatch)
                // write the right (with updated mem root)
                setMemoryB32(rightAddr, beWordAsB32(right), proofIndexR)
            }

            function storeMem(addr, size, value, proofIndexL, proofIndexR) {
                storeMemUnaligned(addr, size, u64ToU256(value), proofIndexL, proofIndexR)
            }

            //
            // Preimage oracle interactions
            //
            function writePreimageKey(addr, count) -> out {
                // adjust count down, so we only have to read a single 32 byte leaf of memory
                let alignment := and64(addr, toU64(31))
                let maxData := sub64(toU64(32), alignment)
                if gt64(count, maxData) { count := maxData }

                let dat := b32asBEWord(getMemoryB32(sub64(addr, alignment), 1))
                // shift out leading bits
                dat := shl(u64ToU256(shl64(toU64(3), alignment)), dat)
                // shift to right end, remove trailing bits
                dat := shr(u64ToU256(shl64(toU64(3), sub64(toU64(32), count))), dat)

                let bits := shl(toU256(3), u64ToU256(count))

                let preImageKey := getPreimageKey()

                // Append to key content by bit-shifting
                let key := b32asBEWord(preImageKey)
                key := shl(bits, key)
                key := or(key, dat)

                // We reset the pre-image value offset back to 0 (the right part of the merkle pair)
                setPreimageKey(beWordAsB32(key))
                setPreimageOffset(toU64(0))
                out := count
            }

            function readPreimagePart(key, offset) -> dat, datlen {
                let addr := sload(preimageOraclePos()) // calling Oracle.readPreimage(bytes32,uint256)
                let memPtr := mload(0x40) // get pointer to free memory for preimage interactions
                mstore(memPtr, shl(224, 0xe03110e1)) // (32-4)*8=224: right-pad the function selector, and then store it
                    // as prefix
                mstore(add(memPtr, 0x04), key)
                mstore(add(memPtr, 0x24), offset)
                let res := call(gas(), addr, 0, memPtr, 0x44, 0x00, 0x40) // output into scratch space
                if res {
                    // 1 on success
                    dat := mload(0x00)
                    datlen := mload(0x20)
                    leave
                }
                revertWithCode(0xbadf00d0)
            }

            // Original implementation is at src/cannon/PreimageKeyLib.sol
            // but it cannot be used because this is inside assembly block
            function localize(preImageKey, localContext_) -> localizedKey {
                // Grab the current free memory pointer to restore later.
                let ptr := mload(0x40)
                // Store the local data key and caller next to each other in memory for hashing.
                mstore(0, preImageKey)
                mstore(0x20, caller())
                mstore(0x40, localContext_)
                // Localize the key with the above `localize` operation.
                localizedKey := or(and(keccak256(0, 0x60), not(shl(248, 0xFF))), shl(248, 1))
                // Restore the free memory pointer.
                mstore(0x40, ptr)
            }

            function readPreimageValue(addr, count, localContext_) -> out {
                let preImageKey := getPreimageKey()
                let offset := getPreimageOffset()
                // If the preimage key is a local key, localize it in the context of the caller.
                let preImageKeyPrefix := shr(248, preImageKey) // 256-8=248
                if eq(preImageKeyPrefix, 1) { preImageKey := localize(preImageKey, localContext_) }
                // make call to pre-image oracle contract
                let pdatB32, pdatlen := readPreimagePart(preImageKey, offset)
                if iszero64(pdatlen) {
                    // EOF
                    out := toU64(0)
                    leave
                }
                let alignment := and64(addr, toU64(31)) // how many bytes addr is offset from being left-aligned
                let maxData := sub64(toU64(32), alignment) // higher alignment leaves less room for data this step
                if gt64(count, maxData) { count := maxData }
                if gt64(count, pdatlen) {
                    // cannot read more than pdatlen
                    count := pdatlen
                }

                let addr_ := addr
                let count_ := count
                let bits := shl64(toU64(3), sub64(toU64(32), count_)) // 32-count, in bits
                let mask := not(sub(shl(u64ToU256(bits), toU256(1)), toU256(1))) // left-aligned mask for count bytes
                let alignmentBits := u64ToU256(shl64(toU64(3), alignment))
                mask := shr(alignmentBits, mask) // mask of count bytes, shifted by alignment
                let pdat := shr(alignmentBits, b32asBEWord(pdatB32)) // pdat, shifted by alignment

                // update pre-image reader with updated offset
                let newOffset := add64(offset, count_)
                setPreimageOffset(newOffset)

                out := count_

                let node := getMemoryB32(sub64(addr_, alignment), 1)
                let dat := and(b32asBEWord(node), not(mask)) // keep old bytes outside of mask
                dat := or(dat, and(pdat, mask)) // fill with bytes from pdat
                setMemoryB32(sub64(addr_, alignment), beWordAsB32(dat), 1)
            }

            //
            // Syscall handling
            //
            function sysCall(localContext_) {
                let a7 := getRegister(toU64(17))
                switch a7
                case 93 {
                    // exit the calling thread. No multi-thread support yet, so just exit.
                    let a0 := getRegister(toU64(10))
                    setExitCode(and(a0, 0xff))
                    setExited()
                    // program stops here, no need to change registers.
                }
                case 94 {
                    // exit-group
                    let a0 := getRegister(toU64(10))
                    setExitCode(and(a0, 0xff))
                    setExited()
                }
                case 214 {
                    // brk
                    // Go sys_linux_riscv64 runtime will only ever call brk(NULL), i.e. first argument (register a0) set
                    // to 0.

                    // brk(0) changes nothing about the memory, and returns the current page break
                    let v := shl64(toU64(30), toU64(1)) // set program break at 1 GiB
                    setRegister(toU64(10), v)
                    setRegister(toU64(11), toU64(0)) // no error
                }
                case 222 {
                    // mmap
                    // A0 = addr (hint)
                    let addr := getRegister(toU64(10))
                    // A1 = n (length)
                    let length := getRegister(toU64(11))
                    // A2 = prot (memory protection type, can ignore)
                    // A3 = flags (shared with other process and or written back to file)
                    let flags := getRegister(toU64(13))
                    // A4 = fd (file descriptor, can ignore because we support anon memory only)
                    let fd := getRegister(toU64(14))
                    // A5 = offset (offset in file, we don't support any non-anon memory, so we can ignore this)

                    let errCode := 0
                    // ensure MAP_ANONYMOUS is set and fd == -1
                    switch or(iszero(and(flags, 0x20)), not(eq(fd, u64Mask())))
                    case 1 {
                        addr := u64Mask()
                        errCode := toU64(0x4d)
                    }
                    default {
                        switch addr
                        case 0 {
                            // No hint, allocate it ourselves, by as much as the requested length.
                            // Increase the length to align it with desired page size if necessary.
                            let align := and64(length, shortToU64(4095))
                            if align { length := add64(length, sub64(shortToU64(4096), align)) }
                            let prevHeap := getHeap()
                            addr := prevHeap
                            setHeap(add64(prevHeap, length)) // increment heap with length
                        }
                        default {
                            // allow hinted memory address (leave it in A0 as return argument)
                        }
                    }

                    setRegister(toU64(10), addr)
                    setRegister(toU64(11), errCode)
                }
                case 63 {
                    // read
                    let fd := getRegister(toU64(10)) // A0 = fd
                    let addr := getRegister(toU64(11)) // A1 = *buf addr
                    let count := getRegister(toU64(12)) // A2 = count
                    let n := 0
                    let errCode := 0
                    switch fd
                    case 0 {
                        // stdin
                        n := toU64(0) // never read anything from stdin
                        errCode := toU64(0)
                    }
                    case 3 {
                        // hint-read
                        // say we read it all, to continue execution after reading the hint-write ack response
                        n := count
                        errCode := toU64(0)
                    }
                    case 5 {
                        // preimage read
                        n := readPreimageValue(addr, count, localContext_)
                        errCode := toU64(0)
                    }
                    default {
                        n := u64Mask() //  -1 (reading error)
                        errCode := toU64(0x4d) // EBADF
                    }
                    setRegister(toU64(10), n)
                    setRegister(toU64(11), errCode)
                }
                case 64 {
                    // write
                    let fd := getRegister(toU64(10)) // A0 = fd
                    let addr := getRegister(toU64(11)) // A1 = *buf addr
                    let count := getRegister(toU64(12)) // A2 = count
                    let n := 0
                    let errCode := 0
                    switch fd
                    case 1 {
                        // stdout
                        n := count // write completes fully in single instruction step
                        errCode := toU64(0)
                    }
                    case 2 {
                        // stderr
                        n := count // write completes fully in single instruction step
                        errCode := toU64(0)
                    }
                    case 4 {
                        // hint-write
                        n := count
                        errCode := toU64(0)
                    }
                    case 6 {
                        // pre-image key-write
                        n := writePreimageKey(addr, count)
                        errCode := toU64(0) // no error
                    }
                    default {
                        // any other file, including (3) hint read (5) preimage read
                        n := u64Mask() //  -1 (writing error)
                        errCode := toU64(0x4d) // EBADF
                    }
                    setRegister(toU64(10), n)
                    setRegister(toU64(11), errCode)
                }
                case 25 {
                    // fcntl - file descriptor manipulation / info lookup
                    let fd := getRegister(toU64(10)) // A0 = fd
                    let cmd := getRegister(toU64(11)) // A1 = cmd
                    let out := 0
                    let errCode := 0
                    switch cmd
                    case 0x1 {
                        // F_GETFD: get file descriptor flags
                        switch fd
                        case 0 {
                            // stdin
                            out := toU64(0) // no flag set
                        }
                        case 1 {
                            // stdout
                            out := toU64(0) // no flag set
                        }
                        case 2 {
                            // stderr
                            out := toU64(0) // no flag set
                        }
                        case 3 {
                            // hint-read
                            out := toU64(0) // no flag set
                        }
                        case 4 {
                            // hint-write
                            out := toU64(0) // no flag set
                        }
                        case 5 {
                            // pre-image read
                            out := toU64(0) // no flag set
                        }
                        case 6 {
                            // pre-image write
                            out := toU64(0) // no flag set
                        }
                        default {
                            out := u64Mask()
                            errCode := toU64(0x4d) //EBADF
                        }
                    }
                    case 0x3 {
                        // F_GETFL: get file descriptor flags
                        switch fd
                        case 0 {
                            // stdin
                            out := toU64(0) // O_RDONLY
                        }
                        case 1 {
                            // stdout
                            out := toU64(1) // O_WRONLY
                        }
                        case 2 {
                            // stderr
                            out := toU64(1) // O_WRONLY
                        }
                        case 3 {
                            // hint-read
                            out := toU64(0) // O_RDONLY
                        }
                        case 4 {
                            // hint-write
                            out := toU64(1) // O_WRONLY
                        }
                        case 5 {
                            // pre-image read
                            out := toU64(0) // O_RDONLY
                        }
                        case 6 {
                            // pre-image write
                            out := toU64(1) // O_WRONLY
                        }
                        default {
                            out := u64Mask()
                            errCode := toU64(0x4d) // EBADF
                        }
                    }
                    default {
                        // no other commands: don't allow changing flags, duplicating FDs, etc.
                        out := u64Mask()
                        errCode := toU64(0x16) // EINVAL (cmd not recognized by this kernel)
                    }
                    setRegister(toU64(10), out)
                    setRegister(toU64(11), errCode) // EBADF
                }
                case 56 {
                    // openat - the Go linux runtime will try to open optional /sys/kernel files for performance hints
                    setRegister(toU64(10), u64Mask())
                    setRegister(toU64(11), toU64(0xd)) // EACCES - no access allowed
                }
                case 113 {
                    // clock_gettime
                    let addr := getRegister(toU64(11)) // addr of timespec struct
                    // write 1337s + 42ns as time
                    let value := or(shortToU256(1337), shl(shortToU256(64), toU256(42)))
                    storeMemUnaligned(addr, toU64(16), value, 1, 2)
                    setRegister(toU64(10), toU64(0))
                    setRegister(toU64(11), toU64(0))
                }
                case 220 {
                    // clone - not supported
                    setRegister(toU64(10), toU64(1))
                    setRegister(toU64(11), toU64(0))
                }
                case 163 {
                    // getrlimit
                    let res := getRegister(toU64(10))
                    let addr := getRegister(toU64(11))
                    switch res
                    case 0x7 {
                        // RLIMIT_NOFILE
                        // first 8 bytes: soft limit. 1024 file handles max open
                        // second 8 bytes: hard limit
                        storeMemUnaligned(
                            addr, toU64(16), or(shortToU256(1024), shl(toU256(64), shortToU256(1024))), 1, 2
                        )
                        setRegister(toU64(10), toU64(0))
                        setRegister(toU64(11), toU64(0))
                    }
                    default { revertWithCode(0xf0012) } // unrecognized resource limit lookup
                }
                case 261 {
                    // prlimit64 -- unsupported, we have getrlimit, is prlimit64 even called?
                    revertWithCode(0xf001ca11) // unsupported system call
                }
                case 422 {
                    // futex - not supported, for now
                    revertWithCode(0xf001ca11) // unsupported system call
                }
                case 101 {
                    // nanosleep - not supported, for now
                    revertWithCode(0xf001ca11) // unsupported system call
                }
                default {
                    // Ignore(no-op) unsupported system calls
                    setRegister(toU64(10), toU64(0))
                    setRegister(toU64(11), toU64(0))
                }
            }

            //
            // Instruction execution
            //
            if getExited() {
                // early exit if we can
                mstore(0, computeStateHash())
                return(0, 0x20)
            }
            setStep(add64(getStep(), toU64(1)))

            let _pc := getPC()
            let instr := loadMem(_pc, toU64(4), false, 0, 0xff) // raw instruction

            // these fields are ignored if not applicable to the instruction type / opcode
            let opcode := parseOpcode(instr)
            let rd := parseRd(instr) // destination register index
            let funct3 := parseFunct3(instr)
            let rs1 := parseRs1(instr) // source register 1 index
            let rs2 := parseRs2(instr) // source register 2 index
            let funct7 := parseFunct7(instr)

            switch opcode
            case 0x03 {
                let pc_ := _pc
                // 000_0011: memory loading
                // LB, LH, LW, LD, LBU, LHU, LWU
                let imm := parseImmTypeI(instr)
                let signed := iszero64(and64(funct3, toU64(4))) // 4 = 100 -> bitflag
                let size := shl64(and64(funct3, toU64(3)), toU64(1)) // 3 = 11 -> 1, 2, 4, 8 bytes size
                let rs1Value := getRegister(rs1)
                let memIndex := add64(rs1Value, signExtend64(imm, toU64(11)))
                let rdValue := loadMem(memIndex, size, signed, 1, 2)
                setRegister(rd, rdValue)
                setPC(add64(pc_, toU64(4)))
            }
            case 0x23 {
                let pc_ := _pc
                // 010_0011: memory storing
                // SB, SH, SW, SD
                let imm := parseImmTypeS(instr)
                let size := shl64(funct3, toU64(1))
                let value := getRegister(rs2)
                let rs1Value := getRegister(rs1)
                let memIndex := add64(rs1Value, signExtend64(imm, toU64(11)))
                storeMem(memIndex, size, value, 1, 2)
                setPC(add64(pc_, toU64(4)))
            }
            case 0x63 {
                // 110_0011: branching
                let rs1Value := getRegister(rs1)
                let rs2Value := getRegister(rs2)
                let branchHit := toU64(0)
                switch funct3
                case 0 {
                    // 000 = BEQ
                    branchHit := eq64(rs1Value, rs2Value)
                }
                case 1 {
                    // 001 = BNE
                    branchHit := and64(not64(eq64(rs1Value, rs2Value)), toU64(1))
                }
                case 4 {
                    // 100 = BLT
                    branchHit := slt64(rs1Value, rs2Value)
                }
                case 5 {
                    // 101 = BGE
                    branchHit := and64(not64(slt64(rs1Value, rs2Value)), toU64(1))
                }
                case 6 {
                    // 110 = BLTU
                    branchHit := lt64(rs1Value, rs2Value)
                }
                case 7 {
                    // 111 := BGEU
                    branchHit := and64(not64(lt64(rs1Value, rs2Value)), toU64(1))
                }
                switch branchHit
                case 0 { _pc := add64(_pc, toU64(4)) }
                default {
                    let imm := parseImmTypeB(instr)
                    // imm12 is a signed offset, in multiples of 2 bytes.
                    // So it's really 13 bits with a hardcoded 0 bit.
                    _pc := add64(_pc, imm)
                }
                // not like the other opcodes: nothing to write to rd register, and PC has already changed
                setPC(_pc)
            }
            case 0x13 {
                // 001_0011: immediate arithmetic and logic
                let rs1Value := getRegister(rs1)
                let imm := parseImmTypeI(instr)
                let rdValue := 0
                switch funct3
                case 0 {
                    // 000 = ADDI
                    rdValue := add64(rs1Value, imm)
                }
                case 1 {
                    // 001 = SLLI
                    rdValue := shl64(and64(imm, toU64(0x3F)), rs1Value) // lower 6 bits in 64 bit mode
                }
                case 2 {
                    // 010 = SLTI
                    rdValue := slt64(rs1Value, imm)
                }
                case 3 {
                    // 011 = SLTIU
                    rdValue := lt64(rs1Value, imm)
                }
                case 4 {
                    // 100 = XORI
                    rdValue := xor64(rs1Value, imm)
                }
                case 5 {
                    // 101 = SR~
                    switch shr64(toU64(6), imm)
                    // in rv64i the top 6 bits select the shift type
                    case 0x00 {
                        // 000000 = SRLI
                        rdValue := shr64(and64(imm, toU64(0x3F)), rs1Value) // lower 6 bits in 64 bit mode
                    }
                    case 0x10 {
                        // 010000 = SRAI
                        rdValue := sar64(and64(imm, toU64(0x3F)), rs1Value) // lower 6 bits in 64 bit mode
                    }
                }
                case 6 {
                    // 110 = ORI
                    rdValue := or64(rs1Value, imm)
                }
                case 7 {
                    // 111 = ANDI
                    rdValue := and64(rs1Value, imm)
                }
                setRegister(rd, rdValue)
                setPC(add64(_pc, toU64(4)))
            }
            case 0x1B {
                // 001_1011: immediate arithmetic and logic signed 32 bit
                let rs1Value := getRegister(rs1)
                let imm := parseImmTypeI(instr)
                let rdValue := 0
                switch funct3
                case 0 {
                    // 000 = ADDIW
                    rdValue := mask32Signed64(add64(rs1Value, imm))
                }
                case 1 {
                    // 001 = SLLIW
                    rdValue := mask32Signed64(shl64(and64(imm, toU64(0x1F)), rs1Value))
                }
                case 5 {
                    // 101 = SR~
                    let shamt := and64(imm, toU64(0x1F))
                    switch shr64(toU64(5), imm)
                    // top 7 bits select the shift type
                    case 0x00 {
                        // 0000000 = SRLIW
                        rdValue := signExtend64(shr64(shamt, and64(rs1Value, u32Mask())), toU64(31))
                    }
                    case 0x20 {
                        // 0100000 = SRAIW
                        rdValue := signExtend64(shr64(shamt, and64(rs1Value, u32Mask())), sub64(toU64(31), shamt))
                    }
                }
                setRegister(rd, rdValue)
                setPC(add64(_pc, toU64(4)))
            }
            case 0x33 {
                // 011_0011: register arithmetic and logic
                let rs1Value := getRegister(rs1)
                let rs2Value := getRegister(rs2)
                let rdValue := 0
                switch funct7
                case 1 {
                    // RV M extension
                    switch funct3
                    case 0 {
                        // 000 = MUL: signed x signed
                        rdValue := mul64(rs1Value, rs2Value)
                    }
                    case 1 {
                        // 001 = MULH: upper bits of signed x signed
                        rdValue :=
                            u256ToU64(shr(toU256(64), mul(signExtend64To256(rs1Value), signExtend64To256(rs2Value))))
                    }
                    case 2 {
                        // 010 = MULHSU: upper bits of signed x unsigned
                        rdValue := u256ToU64(shr(toU256(64), mul(signExtend64To256(rs1Value), u64ToU256(rs2Value))))
                    }
                    case 3 {
                        // 011 = MULHU: upper bits of unsigned x unsigned
                        rdValue := u256ToU64(shr(toU256(64), mul(u64ToU256(rs1Value), u64ToU256(rs2Value))))
                    }
                    case 4 {
                        // 100 = DIV
                        switch rs2Value
                        case 0 { rdValue := u64Mask() }
                        default { rdValue := sdiv64(rs1Value, rs2Value) }
                    }
                    case 5 {
                        // 101 = DIVU
                        switch rs2Value
                        case 0 { rdValue := u64Mask() }
                        default { rdValue := div64(rs1Value, rs2Value) }
                    }
                    case 6 {
                        // 110 = REM
                        switch rs2Value
                        case 0 { rdValue := rs1Value }
                        default { rdValue := smod64(rs1Value, rs2Value) }
                    }
                    case 7 {
                        // 111 = REMU
                        switch rs2Value
                        case 0 { rdValue := rs1Value }
                        default { rdValue := mod64(rs1Value, rs2Value) }
                    }
                }
                default {
                    switch funct3
                    case 0 {
                        // 000 = ADD/SUB
                        switch funct7
                        case 0x00 {
                            // 0000000 = ADD
                            rdValue := add64(rs1Value, rs2Value)
                        }
                        case 0x20 {
                            // 0100000 = SUB
                            rdValue := sub64(rs1Value, rs2Value)
                        }
                    }
                    case 1 {
                        // 001 = SLL
                        rdValue := shl64(and64(rs2Value, toU64(0x3F)), rs1Value) // only the low 6 bits are consider in
                            // RV6VI
                    }
                    case 2 {
                        // 010 = SLT
                        rdValue := slt64(rs1Value, rs2Value)
                    }
                    case 3 {
                        // 011 = SLTU
                        rdValue := lt64(rs1Value, rs2Value)
                    }
                    case 4 {
                        // 100 = XOR
                        rdValue := xor64(rs1Value, rs2Value)
                    }
                    case 5 {
                        // 101 = SR~
                        switch funct7
                        case 0x00 {
                            // 0000000 = SRL
                            rdValue := shr64(and64(rs2Value, toU64(0x3F)), rs1Value) // logical: fill with zeroes
                        }
                        case 0x20 {
                            // 0100000 = SRA
                            rdValue := sar64(and64(rs2Value, toU64(0x3F)), rs1Value) // arithmetic: sign bit is extended
                        }
                    }
                    case 6 {
                        // 110 = OR
                        rdValue := or64(rs1Value, rs2Value)
                    }
                    case 7 {
                        // 111 = AND
                        rdValue := and64(rs1Value, rs2Value)
                    }
                }
                setRegister(rd, rdValue)
                setPC(add64(_pc, toU64(4)))
            }
            case 0x3B {
                // 011_1011: register arithmetic and logic in 32 bits
                let rs1Value := getRegister(rs1)
                let rs2Value := getRegister(rs2)
                let rdValue := 0
                switch funct7
                case 1 {
                    // RV M extension
                    switch funct3
                    case 0 {
                        // 000 = MULW
                        rdValue := mask32Signed64(mul64(and64(rs1Value, u32Mask()), and64(rs2Value, u32Mask())))
                    }
                    case 4 {
                        // 100 = DIVW
                        switch rs2Value
                        case 0 { rdValue := u64Mask() }
                        default {
                            rdValue := mask32Signed64(sdiv64(mask32Signed64(rs1Value), mask32Signed64(rs2Value)))
                        }
                    }
                    case 5 {
                        // 101 = DIVUW
                        switch rs2Value
                        case 0 { rdValue := u64Mask() }
                        default {
                            rdValue := mask32Signed64(div64(and64(rs1Value, u32Mask()), and64(rs2Value, u32Mask())))
                        }
                    }
                    case 6 {
                        // 110 = REMW
                        switch rs2Value
                        case 0 { rdValue := mask32Signed64(rs1Value) }
                        default {
                            rdValue := mask32Signed64(smod64(mask32Signed64(rs1Value), mask32Signed64(rs2Value)))
                        }
                    }
                    case 7 {
                        // 111 = REMUW
                        switch rs2Value
                        case 0 { rdValue := mask32Signed64(rs1Value) }
                        default {
                            rdValue := mask32Signed64(mod64(and64(rs1Value, u32Mask()), and64(rs2Value, u32Mask())))
                        }
                    }
                }
                default {
                    switch funct3
                    case 0 {
                        // 000 = ADDW/SUBW
                        switch funct7
                        case 0x00 {
                            // 0000000 = ADDW
                            rdValue := mask32Signed64(add64(and64(rs1Value, u32Mask()), and64(rs2Value, u32Mask())))
                        }
                        case 0x20 {
                            // 0100000 = SUBW
                            rdValue := mask32Signed64(sub64(and64(rs1Value, u32Mask()), and64(rs2Value, u32Mask())))
                        }
                    }
                    case 1 {
                        // 001 = SLLW
                        rdValue := mask32Signed64(shl64(and64(rs2Value, toU64(0x1F)), rs1Value))
                    }
                    case 5 {
                        // 101 = SR~
                        let shamt := and64(rs2Value, toU64(0x1F))
                        switch funct7
                        case 0x00 {
                            // 0000000 = SRLW
                            rdValue := signExtend64(shr64(shamt, and64(rs1Value, u32Mask())), toU64(31))
                        }
                        case 0x20 {
                            // 0100000 = SRAW
                            rdValue := signExtend64(shr64(shamt, and64(rs1Value, u32Mask())), sub64(toU64(31), shamt))
                        }
                    }
                }
                setRegister(rd, rdValue)
                setPC(add64(_pc, toU64(4)))
            }
            case 0x37 {
                // 011_0111: LUI = Load upper immediate
                let imm := parseImmTypeU(instr)
                let rdValue := shl64(toU64(12), imm)
                setRegister(rd, rdValue)
                setPC(add64(_pc, toU64(4)))
            }
            case 0x17 {
                // 001_0111: AUIPC = Add upper immediate to PC
                let imm := parseImmTypeU(instr)
                let rdValue := add64(_pc, signExtend64(shl64(toU64(12), imm), toU64(31)))
                setRegister(rd, rdValue)
                setPC(add64(_pc, toU64(4)))
            }
            case 0x6F {
                // 110_1111: JAL = Jump and link
                let imm := parseImmTypeJ(instr)
                let rdValue := add64(_pc, toU64(4))
                setRegister(rd, rdValue)
                setPC(add64(_pc, signExtend64(shl64(toU64(1), imm), toU64(20)))) // signed offset in multiples of 2
                    // bytes (last bit is there, but ignored)
            }
            case 0x67 {
                // 110_0111: JALR = Jump and link register
                let rs1Value := getRegister(rs1)
                let imm := parseImmTypeI(instr)
                let rdValue := add64(_pc, toU64(4))
                setRegister(rd, rdValue)
                setPC(and64(add64(rs1Value, signExtend64(imm, toU64(11))), xor64(u64Mask(), toU64(1)))) // least
                    // significant bit is set to 0
            }
            case 0x73 {
                // 111_0011: environment things
                switch funct3
                case 0 {
                    // 000 = ECALL/EBREAK
                    switch shr64(toU64(20), instr)
                    // I-type, top 12 bits
                    case 0 {
                        // imm12 = 000000000000 ECALL
                        sysCall(_localContext)
                        setPC(add64(_pc, toU64(4)))
                    }
                    default {
                        // imm12 = 000000000001 EBREAK
                        setPC(add64(_pc, toU64(4))) // ignore breakpoint
                    }
                }
                default {
                    // CSR instructions
                    setRegister(rd, toU64(0)) // ignore CSR instructions
                    setPC(add64(_pc, toU64(4)))
                }
            }
            case 0x2F {
                // 010_1111: RV{32,64}A and RV{32,64}A atomic operations extension
                // acquire and release bits:
                //   aq := and64(shr64(toU64(1), funct7), toU64(1))
                //   rl := and64(funct7, toU64(1))
                // if none set: unordered
                // if aq is set: no following mem ops observed before acquire mem op
                // if rl is set: release mem op not observed before earlier mem ops
                // if both set: sequentially consistent
                // These are no-op here because there is no pipeline of mem ops to acquire/release.

                // 0b010 == RV32A W variants
                // 0b011 == RV64A D variants
                let size := shl64(funct3, toU64(1))
                if or(lt64(size, toU64(4)), gt64(size, toU64(8))) { revertWithCode(0xbada70) } // bad AMO size

                let addr := getRegister(rs1)
                if and64(addr, toU64(3)) {
                    // quick addr alignment check
                    revertWithCode(0xbad10ad0) // addr not aligned with 4 bytes
                }

                let op := shr64(toU64(2), funct7)
                switch op
                case 0x2 {
                    // 00010 = LR = Load Reserved
                    let v := loadMem(addr, size, true, 1, 2)
                    setRegister(rd, v)
                    setLoadReservation(addr)
                }
                case 0x3 {
                    // 00011 = SC = Store Conditional
                    let rdValue := toU64(1)
                    if eq64(addr, getLoadReservation()) {
                        let rs2Value := getRegister(rs2)
                        storeMem(addr, size, rs2Value, 1, 2)
                        rdValue := toU64(0)
                    }
                    setRegister(rd, rdValue)
                    setLoadReservation(toU64(0))
                }
                default {
                    // AMO: Atomic Memory Operation
                    let rs2Value := getRegister(rs2)
                    if eq64(size, toU64(4)) { rs2Value := mask32Signed64(rs2Value) }
                    let value := rs2Value
                    let v := loadMem(addr, size, true, 1, 2)
                    let rdValue := v
                    switch op
                    case 0x0 {
                        // 00000 = AMOADD = add
                        v := add64(v, value)
                    }
                    case 0x1 {
                        // 00001 = AMOSWAP
                        v := value
                    }
                    case 0x4 {
                        // 00100 = AMOXOR = xor
                        v := xor64(v, value)
                    }
                    case 0x8 {
                        // 01000 = AMOOR = or
                        v := or64(v, value)
                    }
                    case 0xc {
                        // 01100 = AMOAND = and
                        v := and64(v, value)
                    }
                    case 0x10 {
                        // 10000 = AMOMIN = min signed
                        if slt64(value, v) { v := value }
                    }
                    case 0x14 {
                        // 10100 = AMOMAX = max signed
                        if sgt64(value, v) { v := value }
                    }
                    case 0x18 {
                        // 11000 = AMOMINU = min unsigned
                        if lt64(value, v) { v := value }
                    }
                    case 0x1c {
                        // 11100 = AMOMAXU = max unsigned
                        if gt64(value, v) { v := value }
                    }
                    default { revertWithCode(0xf001a70) } // unknown atomic operation

                    storeMem(addr, size, v, 1, 3) // after overwriting 1, proof 2 is no longer valid
                    setRegister(rd, rdValue)
                }
                setPC(add64(_pc, toU64(4)))
            }
            case 0x0F {
                // 000_1111: fence
                // Used to impose additional ordering constraints; flushing the mem operation pipeline.
                // This VM doesn't have a pipeline, nor additional harts, so this is a no-op.
                // FENCE / FENCE.TSO / FENCE.I all no-op: there's nothing to synchronize.
                setPC(add64(_pc, toU64(4)))
            }
            case 0x07 {
                // FLW/FLD: floating point load word/double
                setPC(add64(_pc, toU64(4))) // no-op this.
            }
            case 0x27 {
                // FSW/FSD: floating point store word/double
                setPC(add64(_pc, toU64(4))) // no-op this.
            }
            case 0x53 {
                // FADD etc. no-op is enough to pass Go runtime check
                setPC(add64(_pc, toU64(4))) // no-op this.
            }
            default { revertWithCode(0xf001c0de) } // unknown instruction opcode

            mstore(0, computeStateHash())
            return(0, 0x20)
        }
    }
}
