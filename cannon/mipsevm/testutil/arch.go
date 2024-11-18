// This file contains utils for setting up forward-compatible tests for 32- and 64-bit MIPS VMs

package testutil

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/memory"
)

type Word = arch.Word

// SetMemoryUint64 sets 8 bytes of memory (1 or 2 Words depending on architecture) and enforces the use of addresses
// that are compatible across 32- and 64-bit architectures
func SetMemoryUint64(t require.TestingT, mem *memory.Memory, addr Word, value uint64) {
	// We are setting 8 bytes of data, so mask addr to align with 8-byte boundaries in memory
	addrMask := ^Word(0) & ^Word(7)
	targetAddr := addr & addrMask

	data := Uint64ToBytes(value)
	err := mem.SetMemoryRange(targetAddr, bytes.NewReader(data))
	require.NoError(t, err)

	// Sanity check
	if addr&0x04 != 0x04 {
		// In order to write tests that run seamlessly across both 32- and 64-bit architectures,
		// we need to use a memory address that is a multiple of 4, but not a multiple of 8.
		// This allows us to expect a consistent value when getting a 32-bit memory value at the given address.
		// For example, if memory contains [0x00: 0x1111_2222, 0x04: 0x3333_4444]:
		// - the 64-bit MIPS VM will get effAddr 0x00, pulling the rightmost (lower-order) 32-bit value
		// - the 32-bit MIPS VM will get effAddr 0x04, pulling the same 32-bit value
		t.Errorf("Invalid address used to set uint64 memory value: %016x", addr)
		t.FailNow()
	}
	// Give the above addr check, memory access should return the same value across architectures
	effAddr := addr & arch.AddressMask
	actual := mem.GetWord(effAddr)
	require.Equal(t, Word(value), actual)
}

// ToSignedInteger converts the unsigend Word to a SignedInteger.
// Useful for avoiding Go compiler warnings for literals that don't fit in a signed type
func ToSignedInteger(x Word) arch.SignedInteger {
	return arch.SignedInteger(x)
}

// Cannon32OnlyTest skips the test if it's not a cannon64 build
func Cannon32OnlyTest(t testing.TB, msg string, args ...any) {
	if !arch.IsMips32 {
		t.Skipf(msg, args...)
	}
}
