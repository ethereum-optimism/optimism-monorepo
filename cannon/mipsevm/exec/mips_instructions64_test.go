//go:build cannon64
// +build cannon64

// These tests target architectures that are 64-bit or larger
package exec

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/memory"
)

// TestLoadSubWord_64bits extends TestLoadSubWord_32bits by testing up to 64-bits (7 byte) offsets
func TestLoadSubWord_64bits(t *testing.T) {
	memVal := uint64(0x1234_5678_9876_5432)
	cases := []struct {
		name          string
		byteLength    Word
		addr          uint64
		memVal        uint64
		signExtend    bool
		expectedValue uint64
	}{
		{name: "64-bit", byteLength: 8, addr: 0xFF00_0000, memVal: 0x8234_5678_9876_5432, expectedValue: 0x8234_5678_9876_5432},
		{name: "64-bit w sign extension", byteLength: 8, addr: 0xFF00_0000, memVal: 0x8234_5678_9876_5432, expectedValue: 0x8234_5678_9876_5432, signExtend: true},
		{name: "32-bit, offset=0", byteLength: 4, addr: 0xFF00_0000, memVal: memVal, expectedValue: 0x1234_5678},
		{name: "32-bit, offset=0, extra bits", byteLength: 4, addr: 0xFF00_0001, memVal: memVal, expectedValue: 0x1234_5678},
		{name: "32-bit, offset=0, extra bits", byteLength: 4, addr: 0xFF00_0002, memVal: memVal, expectedValue: 0x1234_5678},
		{name: "32-bit, offset=0, extra bits", byteLength: 4, addr: 0xFF00_0003, memVal: memVal, expectedValue: 0x1234_5678},
		{name: "32-bit, offset=4", byteLength: 4, addr: 0xFF00_0004, memVal: memVal, expectedValue: 0x9876_5432},
		{name: "32-bit, offset=4, extra bits", byteLength: 4, addr: 0xFF00_0005, memVal: memVal, expectedValue: 0x9876_5432},
		{name: "32-bit, offset=4, extra bits", byteLength: 4, addr: 0xFF00_0006, memVal: memVal, expectedValue: 0x9876_5432},
		{name: "32-bit, offset=4, extra bits", byteLength: 4, addr: 0xFF00_0007, memVal: memVal, expectedValue: 0x9876_5432},
		{name: "32-bit, sign extend negative", byteLength: 4, addr: 0xFF00_0006, memVal: 0x1234_5678_F1E2_A1B1, expectedValue: 0xFFFF_FFFF_F1E2_A1B1, signExtend: true},
		{name: "32-bit, sign extend positive", byteLength: 4, addr: 0xFF00_0007, memVal: 0x1234_5678_7876_5432, expectedValue: 0x7876_5432, signExtend: true},
		{name: "16-bit, subword offset=4", byteLength: 2, addr: 0x04, memVal: memVal, expectedValue: 0x9876},
		{name: "16-bit, subword offset=4, extra bit set", byteLength: 2, addr: 0x05, memVal: memVal, expectedValue: 0x9876},
		{name: "16-bit, subword offset=6", byteLength: 2, addr: 0x06, memVal: memVal, expectedValue: 0x5432},
		{name: "16-bit, subword offset=6, extra bit set", byteLength: 2, addr: 0x07, memVal: memVal, expectedValue: 0x5432},
		{name: "16-bit, sign extend negative val", byteLength: 2, addr: 0x04, memVal: 0x1234_5678_8BEE_CCDD, expectedValue: 0xFFFF_FFFF_FFFF_8BEE, signExtend: true},
		{name: "16-bit, sign extend positive val", byteLength: 2, addr: 0x04, memVal: 0x1234_5678_7876_5432, expectedValue: 0x7876, signExtend: true},
		{name: "8-bit, offset=4", byteLength: 1, addr: 0x1234, memVal: memVal, expectedValue: 0x98},
		{name: "8-bit, offset=5", byteLength: 1, addr: 0x1235, memVal: memVal, expectedValue: 0x76},
		{name: "8-bit, offset=6", byteLength: 1, addr: 0x1236, memVal: memVal, expectedValue: 0x54},
		{name: "8-bit, offset=7", byteLength: 1, addr: 0x1237, memVal: memVal, expectedValue: 0x32},
		{name: "8-bit, sign extend positive", byteLength: 1, addr: 0x1237, memVal: memVal, expectedValue: 0x32, signExtend: true},
		{name: "8-bit, sign extend negative", byteLength: 1, addr: 0x1237, memVal: 0x1234_5678_8764_4381, expectedValue: 0xFFFF_FFFF_FFFF_FF81, signExtend: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mem := memory.NewMemory()
			memTracker := NewMemoryTracker(mem)

			effAddr := Word(c.addr) & arch.AddressMask
			mem.SetWord(effAddr, c.memVal)

			retVal := LoadSubWord(mem, Word(c.addr), c.byteLength, c.signExtend, memTracker)
			require.Equal(t, c.expectedValue, retVal)
		})
	}
}

// TestStoreSubWord_64bits extends TestStoreSubWord_32bits by testing up to 64-bits (7 byte) offsets
func TestStoreSubWord_64bits(t *testing.T) {
	memVal := uint64(0xFFFF_FFFF_FFFF_FFFF)
	value := uint64(0x1234_5678_9876_5432)

	cases := []struct {
		name          string
		byteLength    Word
		addr          uint64
		expectedValue uint64
	}{
		{name: "64-bit", byteLength: 8, addr: 0xFF00_0000, expectedValue: value},
		{name: "32-bit, offset 0", byteLength: 4, addr: 0xFF00_0000, expectedValue: 0x9876_5432_FFFF_FFFF},
		{name: "32-bit, offset 0, extra addr bits", byteLength: 4, addr: 0xFF00_0001, expectedValue: 0x9876_5432_FFFF_FFFF},
		{name: "32-bit, offset 0, extra addr bits", byteLength: 4, addr: 0xFF00_0002, expectedValue: 0x9876_5432_FFFF_FFFF},
		{name: "32-bit, offset 0, extra addr bits", byteLength: 4, addr: 0xFF00_0003, expectedValue: 0x9876_5432_FFFF_FFFF},
		{name: "32-bit, offset 4", byteLength: 4, addr: 0xFF00_0004, expectedValue: 0xFFFF_FFFF_9876_5432},
		{name: "32-bit, offset 4, extra addr bits", byteLength: 4, addr: 0xFF00_0005, expectedValue: 0xFFFF_FFFF_9876_5432},
		{name: "32-bit, offset 4, extra addr bits", byteLength: 4, addr: 0xFF00_0006, expectedValue: 0xFFFF_FFFF_9876_5432},
		{name: "32-bit, offset 4, extra addr bits", byteLength: 4, addr: 0xFF00_0007, expectedValue: 0xFFFF_FFFF_9876_5432},
		{name: "16-bit, offset=4", byteLength: 2, addr: 0x04, expectedValue: 0xFFFF_FFFF_5432_FFFF},
		{name: "16-bit, offset=4, extra bit set", byteLength: 2, addr: 0x05, expectedValue: 0xFFFF_FFFF_5432_FFFF},
		{name: "16-bit, offset=6", byteLength: 2, addr: 0x06, expectedValue: 0xFFFF_FFFF_FFFF_5432},
		{name: "16-bit, offset=6, extra bit set", byteLength: 2, addr: 0x07, expectedValue: 0xFFFF_FFFF_FFFF_5432},
		{name: "8-bit, offset=4", byteLength: 1, addr: 0x1234, expectedValue: 0xFFFF_FFFF_32FF_FFFF},
		{name: "8-bit, offset=5", byteLength: 1, addr: 0x1235, expectedValue: 0xFFFF_FFFF_FF32_FFFF},
		{name: "8-bit, offset=6", byteLength: 1, addr: 0x1236, expectedValue: 0xFFFF_FFFF_FFFF_32FF},
		{name: "8-bit, offset=7", byteLength: 1, addr: 0x1237, expectedValue: 0xFFFF_FFFF_FFFF_FF32},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mem := memory.NewMemory()
			memTracker := NewMemoryTracker(mem)

			effAddr := Word(c.addr) & arch.AddressMask
			mem.SetWord(effAddr, memVal)

			StoreSubWord(mem, Word(c.addr), c.byteLength, Word(value), memTracker)
			newMemVal := mem.GetWord(effAddr)

			require.Equal(t, c.expectedValue, newMemVal)
		})
	}
}

func TestSignExtend_64bit(t *testing.T) {
	cases := []struct {
		name     string
		data     Word
		index    Word
		expected Word
	}{
		{name: "idx 32, signed", data: 0x0000_0000_8123_4567, index: 32, expected: 0xFFFF_FFFF_8123_4567},
		{name: "idx 32, unsigned", data: 0x0000_0000_7123_4567, index: 32, expected: 0x0000_0000_7123_4567},
		{name: "idx 32, signed, non-zero upper bits", data: 0x1234_4321_8123_4567, index: 32, expected: 0xFFFF_FFFF_8123_4567},
		{name: "idx 32, unsigned, non-zero upper bits", data: 0xFFFF_FFFF_7123_4567, index: 32, expected: 0x0000_0000_7123_4567},
		{name: "idx 33, signed", data: 0x0000_0001_2345_4321, index: 33, expected: 0xFFFF_FFFF_2345_4321},
		{name: "idx 33, unsigned", data: 0x0000_0000_2345_4321, index: 33, expected: 0x0000_0000_2345_4321},
		{name: "idx 33, signed, non-zero upper bits", data: 0xABCD_4321_2345_4321, index: 33, expected: 0xFFFF_FFFF_2345_4321},
		{name: "idx 33, unsigned, non-zero upper bits", data: 0xFFFF_FFFE_2345_4321, index: 33, expected: 0x0000_0000_2345_4321},
		{name: "idx 48, signed", data: 0x0000_A123_0123_4567, index: 48, expected: 0xFFFF_A123_0123_4567},
		{name: "idx 48, unsigned", data: 0x0000_0123_0123_4567, index: 48, expected: 0x0000_0123_0123_4567},
		{name: "idx 48, signed, non-zero upper bits", data: 0xABCD_A123_0123_4567, index: 48, expected: 0xFFFF_A123_0123_4567},
		{name: "idx 48, unsigned, non-zero upper bits", data: 0xABCD_0123_0123_4567, index: 48, expected: 0x0000_0123_0123_4567},
		{name: "idx 50, signed", data: 0x0002_A123_0123_4567, index: 50, expected: 0xFFFE_A123_0123_4567},
		{name: "idx 50, unsigned", data: 0x0001_0123_0123_4567, index: 50, expected: 0x0001_0123_0123_4567},
		{name: "idx 50, signed, non-zero upper bits", data: 0x1AB2_A123_0123_4567, index: 50, expected: 0xFFFE_A123_0123_4567},
		{name: "idx 50, unsigned, non-zero upper bits", data: 0xFED1_0123_0123_4567, index: 50, expected: 0x0001_0123_0123_4567},
		{name: "idx 64, signed", data: 0xABCD_0101_8123_4567, index: 64, expected: 0xABCD_0101_8123_4567},
		{name: "idx 64, unsigned", data: 0x789A_0101_8123_4567, index: 64, expected: 0x789A_0101_8123_4567},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual := SignExtend(c.data, c.index)
			require.Equal(t, c.expected, actual)
		})
	}
}
