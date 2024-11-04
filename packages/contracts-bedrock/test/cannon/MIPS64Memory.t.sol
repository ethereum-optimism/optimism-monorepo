// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { CommonTest } from "test/setup/CommonTest.sol";
import { MIPS64Memory } from "src/cannon/libraries/MIPS64Memory.sol";
import { InvalidMemoryProof } from "src/cannon/libraries/CannonErrors.sol";

contract MIPS64Memory_Test is CommonTest {
    MIPS64MemoryWithCalldata mem;

    error InvalidAddress();

    function setUp() public virtual override {
        super.setUp();
        mem = new MIPS64MemoryWithCalldata();
    }

    /// @dev Static unit test for basic memory access
    function test_readMem_succeeds() external {
        uint64 addr = 0x100;
        uint64 word = 0x11_22_33_44_55_66_77_88;
        bytes32 root;
        bytes memory proof;
        (root, proof) = ffi.getCannonMemory64Proof(addr, word);
        uint64 readWord = mem.readMem(root, addr, 0, proof);
        assertEq(readWord, word);
    }

    /// @dev Static unit test asserting that reading from the zero address succeeds
    function test_readMemAtZero_succeeds() external {
        uint64 addr = 0x0;
        uint64 word = 0x11_22_33_44_55_66_77_88;
        bytes32 root;
        bytes memory proof;
        (root, proof) = ffi.getCannonMemory64Proof(addr, word);
        uint64 readWord = mem.readMem(root, addr, 0, proof);
        assertEq(readWord, word);
    }

    /// @dev Static unit test asserting that reading from high memory area succeeds
    function test_readMemHighMem_succeeds() external {
        uint64 addr = 0xFF_FF_FF_FF_00_00_00_88;
        uint64 word = 0x11_22_33_44_55_66_77_88;
        bytes32 root;
        bytes memory proof;
        (root, proof) = ffi.getCannonMemory64Proof(addr, word);
        uint64 readWord = mem.readMem(root, addr, 0, proof);
        assertEq(readWord, word);
    }

    /// @dev Static unit test asserting that reads revert when a misaligned memory address is provided
    function test_readMem_readInvalidAddress_reverts() external {
        uint64 addr = 0x100;
        uint64 word = 0x11_22_33_44_55_66_77_88;
        bytes32 root;
        bytes memory proof;
        (root, proof) = ffi.getCannonMemory64Proof(addr, word);
        vm.expectRevert(InvalidAddress.selector);
        mem.readMem(root, addr + 4, 0, proof);
    }

    /// @dev Static unit test asserting that reads revert when an invalid proof is provided
    function test_readMem_readInvalidProof_reverts() external {
        uint64 addr = 0x100;
        uint64 word = 0x11_22_33_44_55_66_77_88;
        bytes32 root;
        bytes memory proof;
        (root, proof) = ffi.getCannonMemory64Proof(addr, word);
        vm.assertTrue(proof[64] != 0x0); // make sure the proof is tampered
        proof[64] = 0x00;
        vm.expectRevert(InvalidMemoryProof.selector);
        mem.readMem(root, addr, 0, proof);
    }

    /// @dev Static unit test asserting that reads from a non-zero proof index succeeds
    function test_readMemNonZeroProofIndex_succeeds() external {
        uint64 addr = 0x100;
        uint64 word = 0x11_22_33_44_55_66_77_88;
        uint64 addr2 = 0xFF_FF_FF_FF_00_00_00_88;
        uint64 word2 = 0xF1_F2_F3_F4_F5_F6_F7_F8;
        bytes32 root;
        bytes memory proof;
        (root, proof) = ffi.getCannonMemory64Proof(addr, word, addr2, word2);

        uint64 readWord = mem.readMem(root, addr, 0, proof);
        assertEq(readWord, word);

        readWord = mem.readMem(root, addr2, 1, proof);
        assertEq(readWord, word2);
    }

    /// @dev Static unit test asserting basic memory write functionality
    function test_writeMem_succeeds() external {
        uint64 addr = 0x100;
        bytes memory zeroProof;
        (, zeroProof) = ffi.getCannonMemory64Proof(addr, 0);

        uint64 word = 0x11_22_33_44_55_66_77_88;
        (bytes32 expectedRoot,) = ffi.getCannonMemory64Proof(addr, word);

        bytes32 newRoot = mem.writeMem(addr, word, 0, zeroProof);
        assertEq(newRoot, expectedRoot);
    }

    // @dev Static unit test asserting that writes to high memory succeeds
    function test_writeMemHighMem_succeeds() external {
        uint64 addr = 0xFF_FF_FF_FF_00_00_00_88;
        bytes memory zeroProof;
        (, zeroProof) = ffi.getCannonMemory64Proof(addr, 0);

        uint64 word = 0x11_22_33_44_55_66_77_88;
        (bytes32 expectedRoot,) = ffi.getCannonMemory64Proof(addr, word);

        bytes32 newRoot = mem.writeMem(addr, word, 0, zeroProof);
        assertEq(newRoot, expectedRoot);
    }

    /// @dev Static unit test asserting that non-zero memory word is overwritten
    function test_writeMemNonZeroProofOffset_succeeds() external {
        uint64 addr = 0x100;
        uint64 word = 0x11_22_33_44_55_66_77_88;
        uint64 addr2 = 0x108;
        uint64 word2 = 0x55_55_55_55_77_77_77_77;
        bytes memory initProof;
        (, initProof) = ffi.getCannonMemory64Proof(addr, word, addr2, word2);

        uint64 word3 = 0x44_44_44_44_44_44_44_44;
        (bytes32 expectedRoot,) = ffi.getCannonMemory64Proof(addr, word, addr2, word2, addr2, word3);

        bytes32 newRoot = mem.writeMem(addr2, word3, 1, initProof);
        assertEq(newRoot, expectedRoot);
    }

    /// @dev Static unit test asserting that a zerod memory word is set for a non-zero memory proof
    function test_writeMemUniqueAccess_succeeds() external {
        uint64 addr = 0x100;
        uint64 word = 0x11_22_33_44_55_66_77_88;
        uint64 addr2 = 0x108;
        uint64 word2 = 0x55_55_55_55_77_77_77_77;
        bytes memory initProof;
        (, initProof) = ffi.getCannonMemory64Proof(addr, word, addr2, word2);

        uint64 addr3 = 0xAA_AA_AA_AA_00;
        uint64 word3 = 0x44_44_44_44_44_44_44_44;
        (, bytes memory addr3Proof) = ffi.getCannonMemory64Proof2(addr, word, addr2, word2, addr3);
        (bytes32 expectedRoot,) = ffi.getCannonMemory64Proof(addr, word, addr2, word2, addr3, word3);

        bytes32 newRoot = mem.writeMem(addr3, word3, 0, addr3Proof);
        assertEq(newRoot, expectedRoot);

        newRoot = mem.writeMem(addr3 + 8, word3, 0, addr3Proof);
        assertNotEq(newRoot, expectedRoot);

        newRoot = mem.writeMem(addr3, word3 + 1, 0, addr3Proof);
        assertNotEq(newRoot, expectedRoot);
    }

    /// @dev Static unit test asserting that writes succeeds in overwriting a non-zero memory word
    function test_writeMemNonZeroMem_succeeds() external {
        uint64 addr = 0x100;
        uint64 word = 0x11_22_33_44_55_66_77_88;
        bytes memory initProof;
        (, initProof) = ffi.getCannonMemory64Proof(addr, word);

        uint64 word2 = 0x55_55_55_55_77_77_77_77;
        (bytes32 expectedRoot,) = ffi.getCannonMemory64Proof(addr, word, addr + 8, word2);

        bytes32 newRoot = mem.writeMem(addr + 8, word2, 0, initProof);
        assertEq(newRoot, expectedRoot);
    }

    /// @dev Static unit test asserting that writes revert when a misaligned memory address is provided
    function test_writeMem_writeMemInvalidAddress_reverts() external {
        bytes memory zeroProof;
        (, zeroProof) = ffi.getCannonMemory64Proof(0x100, 0);
        vm.expectRevert(InvalidAddress.selector);
        mem.writeMem(0x104, 0x0, 0, zeroProof);
    }
}

contract MIPS64MemoryWithCalldata {
    function readMem(
        bytes32 _root,
        uint64 _addr,
        uint8 _proofIndex,
        bytes calldata /* _proof */
    )
        external
        pure
        returns (uint64 out_)
    {
        uint256 proofDataOffset = 4 + 32 + 32 + 32 + 32 + 32;
        uint256 proofOffset = MIPS64Memory.memoryProofOffset(proofDataOffset, _proofIndex);
        return MIPS64Memory.readMem(_root, _addr, proofOffset);
    }

    function writeMem(
        uint64 _addr,
        uint64 _value,
        uint8 _proofIndex,
        bytes calldata /* _proof */
    )
        external
        pure
        returns (bytes32 root_)
    {
        uint256 proofDataOffset = 4 + 32 + 32 + 32 + 32 + 32;
        uint256 proofOffset = MIPS64Memory.memoryProofOffset(proofDataOffset, _proofIndex);
        return MIPS64Memory.writeMem(_addr, proofOffset, _value);
    }
}
