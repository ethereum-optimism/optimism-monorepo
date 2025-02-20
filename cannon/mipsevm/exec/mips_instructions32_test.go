// These tests target architectures that are 32-bit or larger
package exec

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/memory"
)

// TestLoadSubWord_32bits validates LoadSubWord with 32-bit offsets (up to 3 bytes)
func TestLoadSubWord_32bits(t *testing.T) {
	cases := []struct {
		name             string
		byteLength       Word
		addr             uint32
		memVal           uint32
		signExtend       bool
		shouldSignExtend bool
		expectedValue    uint32
	}{
		{name: "32-bit", byteLength: 4, addr: 0xFF00_0000, memVal: 0x1234_5678, expectedValue: 0x1234_5678},
		{name: "32-bit, extra bits", byteLength: 4, addr: 0xFF00_0001, memVal: 0x1234_5678, expectedValue: 0x1234_5678},
		{name: "32-bit, extra bits", byteLength: 4, addr: 0xFF00_0002, memVal: 0x1234_5678, expectedValue: 0x1234_5678},
		{name: "32-bit, extra bits", byteLength: 4, addr: 0xFF00_0003, memVal: 0x1234_5678, expectedValue: 0x1234_5678},
		{name: "16-bit, offset=0", byteLength: 2, addr: 0x00, memVal: 0x1234_5678, expectedValue: 0x1234},
		{name: "16-bit, offset=0, extra bit set", byteLength: 2, addr: 0x01, memVal: 0x1234_5678, expectedValue: 0x1234},
		{name: "16-bit, offset=2", byteLength: 2, addr: 0x02, memVal: 0x1234_5678, expectedValue: 0x5678},
		{name: "16-bit, offset=2, extra bit set", byteLength: 2, addr: 0x03, memVal: 0x1234_5678, expectedValue: 0x5678},
		{name: "16-bit, sign extend positive val", byteLength: 2, addr: 0x02, memVal: 0x1234_5678, expectedValue: 0x5678, signExtend: true, shouldSignExtend: false},
		{name: "16-bit, sign extend negative val", byteLength: 2, addr: 0x02, memVal: 0x1234_F678, expectedValue: 0xFFFF_F678, signExtend: true, shouldSignExtend: true},
		{name: "16-bit, do not sign extend negative val", byteLength: 2, addr: 0x02, memVal: 0x1234_F678, expectedValue: 0xF678, signExtend: false},
		{name: "8-bit, offset=0", byteLength: 1, addr: 0x1230, memVal: 0x1234_5678, expectedValue: 0x12},
		{name: "8-bit, offset=1", byteLength: 1, addr: 0x1231, memVal: 0x1234_5678, expectedValue: 0x34},
		{name: "8-bit, offset=2", byteLength: 1, addr: 0x1232, memVal: 0x1234_5678, expectedValue: 0x56},
		{name: "8-bit, offset=3", byteLength: 1, addr: 0x1233, memVal: 0x1234_5678, expectedValue: 0x78},
		{name: "8-bit, sign extend positive", byteLength: 1, addr: 0x1233, memVal: 0x1234_5678, expectedValue: 0x78, signExtend: true, shouldSignExtend: false},
		{name: "8-bit, sign extend negative", byteLength: 1, addr: 0x1233, memVal: 0x1234_5688, expectedValue: 0xFFFF_FF88, signExtend: true, shouldSignExtend: true},
		{name: "8-bit, do not sign extend neg value", byteLength: 1, addr: 0x1233, memVal: 0x1234_5688, expectedValue: 0x88, signExtend: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mem := memory.NewMemory()
			memTracker := NewMemoryTracker(mem)

			effAddr := Word(c.addr) & arch.AddressMask
			// Shift memval for consistency across architectures
			memVal := Word(c.memVal) << (arch.WordSize - 32)
			mem.SetWord(effAddr, memVal)

			retVal := LoadSubWord(mem, Word(c.addr), c.byteLength, c.signExtend, memTracker)

			// If sign extending, make sure retVal is consistent across architectures
			expected := Word(c.expectedValue)
			if c.shouldSignExtend {
				signedBits := ^Word(0xFFFF_FFFF)
				expected = expected | signedBits
			}
			require.Equal(t, expected, retVal)
		})
	}
}

// TestStoreSubWord_32bits validates LoadSubWord with 32-bit offsets (up to 3 bytes)
func TestStoreSubWord_32bits(t *testing.T) {
	memVal := 0xFFFF_FFFF
	value := 0x1234_5678

	cases := []struct {
		name          string
		byteLength    Word
		addr          uint32
		expectedValue uint32
	}{
		{name: "32-bit", byteLength: 4, addr: 0xFF00_0000, expectedValue: 0x1234_5678},
		{name: "32-bit, extra bits", byteLength: 4, addr: 0xFF00_0001, expectedValue: 0x1234_5678},
		{name: "32-bit, extra bits", byteLength: 4, addr: 0xFF00_0002, expectedValue: 0x1234_5678},
		{name: "32-bit, extra bits", byteLength: 4, addr: 0xFF00_0003, expectedValue: 0x1234_5678},
		{name: "16-bit, subword offset=0", byteLength: 2, addr: 0x00, expectedValue: 0x5678_FFFF},
		{name: "16-bit, subword offset=0, extra bit set", byteLength: 2, addr: 0x01, expectedValue: 0x5678_FFFF},
		{name: "16-bit, subword offset=2", byteLength: 2, addr: 0x02, expectedValue: 0xFFFF_5678},
		{name: "16-bit, subword offset=2, extra bit set", byteLength: 2, addr: 0x03, expectedValue: 0xFFFF_5678},
		{name: "8-bit, offset=0", byteLength: 1, addr: 0x1230, expectedValue: 0x78FF_FFFF},
		{name: "8-bit, offset=1", byteLength: 1, addr: 0x1231, expectedValue: 0xFF78_FFFF},
		{name: "8-bit, offset=2", byteLength: 1, addr: 0x1232, expectedValue: 0xFFFF_78FF},
		{name: "8-bit, offset=3", byteLength: 1, addr: 0x1233, expectedValue: 0xFFFF_FF78},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mem := memory.NewMemory()
			memTracker := NewMemoryTracker(mem)

			effAddr := Word(c.addr) & arch.AddressMask
			// Shift memval for consistency across architectures
			memVal := Word(memVal) << (arch.WordSize - 32)
			mem.SetWord(effAddr, memVal)

			StoreSubWord(mem, Word(c.addr), c.byteLength, Word(value), memTracker)
			newMemVal := mem.GetWord(effAddr)

			// Make sure expectation is consistent across architectures
			expected := Word(c.expectedValue) << (arch.WordSize - 32)
			require.Equal(t, expected, newMemVal)
		})
	}
}

func TestSignExtend_32bit(t *testing.T) {
	cases := []struct {
		name     string
		data     Word
		index    Word
		expected Word
	}{
		{name: "idx 1, signed", data: 0x0000_0001, index: 1, expected: 0xFFFF_FFFF},
		{name: "idx 1, unsigned", data: 0x0000_0000, index: 1, expected: 0x0000_0000},
		{name: "idx 2, signed", data: 0x0000_0002, index: 2, expected: 0xFFFF_FFFE},
		{name: "idx 2, unsigned", data: 0x0000_0001, index: 2, expected: 0x0000_0001},
		{name: "idx 4, signed", data: 0x0000_0008, index: 4, expected: 0xFFFF_FFF8},
		{name: "idx 4, unsigned", data: 0x0000_0005, index: 4, expected: 0x0000_0005},
		{name: "idx 8, signed", data: 0x0000_0092, index: 8, expected: 0xFFFF_FF92},
		{name: "idx 8, unsigned", data: 0x0000_0075, index: 8, expected: 0x0000_0075},
		{name: "idx 16, signed", data: 0x0000_A123, index: 16, expected: 0xFFFF_A123},
		{name: "idx 16, unsigned", data: 0x0000_7123, index: 16, expected: 0x0000_7123},
		{name: "idx 32, signed", data: 0x8123_4567, index: 32, expected: 0x8123_4567},
		{name: "idx 32, unsigned", data: 0x7123_4567, index: 32, expected: 0x7123_4567},
		{name: "idx 1, signed, nonzero upper bits", data: 0x1234_5671, index: 1, expected: 0xFFFF_FFFF},
		{name: "idx 1, unsigned, nonzero upper bits", data: 0x1234_567E, index: 1, expected: 0x0000_0000},
		{name: "idx 2, signed, nonzero upper bits", data: 0xABCD_EFE6, index: 2, expected: 0xFFFF_FFFE},
		{name: "idx 2, unsigned, nonzero upper bits", data: 0xABCD_EFED, index: 2, expected: 0x0000_0001},
		{name: "idx 4, signed, nonzero upper bits", data: 0x1230_0008, index: 4, expected: 0xFFFF_FFF8},
		{name: "idx 4, unsigned, nonzero upper bits", data: 0xFFF0_0005, index: 4, expected: 0x0000_0005},
		{name: "idx 8, signed, nonzero upper bits", data: 0x1111_1192, index: 8, expected: 0xFFFF_FF92},
		{name: "idx 8, unsigned, nonzero upper bits", data: 0xFFFF_FF75, index: 8, expected: 0x0000_0075},
		{name: "idx 16, signed, nonzero upper bits", data: 0x1234_A123, index: 16, expected: 0xFFFF_A123},
		{name: "idx 16, unsigned, nonzero upper bits", data: 0x1234_7123, index: 16, expected: 0x0000_7123},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual := SignExtend(c.data, c.index)
			expected := signExtend64(c.expected)
			require.Equal(t, expected, actual)
		})
	}
}

func signExtend64(w Word) Word {
	// If bit at index 31 == 1, then sign extend the higher bits on 64-bit architectures
	if !arch.IsMips32 && (w>>31)&1 == 1 {
		upperBits := uint64(0xFFFF_FFFF_0000_0000)
		return Word(upperBits) | w
	}
	return w
}
