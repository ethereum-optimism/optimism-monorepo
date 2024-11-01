// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

library MIPS64Arch {
    uint64 internal constant WORD_SIZE = 64;
    uint64 internal constant WORD_SIZE_BYTES = 8;
    uint64 internal constant EXT_MASK = 0x7;
    uint64 internal constant ADDRESS_MASK = 0xFFFFFFFFFFFFFFF8;
}
