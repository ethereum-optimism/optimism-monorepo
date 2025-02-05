package program

import (
	"debug/elf"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/program/testutil"
)

func TestLoadELF(t *testing.T) {
	data := []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88}
	dataSize := uint64(len(data))
	lastValidAddr := uint64(HEAP_START - 1)
	lastAddr := uint64(^uint32(0))
	if !arch.IsMips32 {
		lastAddr = (1 << 48) - 1
	}

	tests := []struct {
		name         string
		progType     elf.ProgType
		memSize      uint64
		fileSize     uint64
		vAddr        uint64
		expectedErr  string
		shouldIgnore bool
	}{
		{name: "Zero length segment", progType: elf.PT_LOAD, fileSize: 0, memSize: 0, vAddr: 0},
		{name: "Zero length segment, non-zero fileSize", progType: elf.PT_LOAD, fileSize: 2, memSize: 0, vAddr: 0, expectedErr: "file size (2) > mem size (0)"},
		{name: "Loadable segment, fileSize > memSize", progType: elf.PT_LOAD, fileSize: dataSize * 2, memSize: dataSize, vAddr: 0x4000, expectedErr: "file size (16) > mem size (8)"},
		{name: "Loadable segment, fileSize < memSize", progType: elf.PT_LOAD, fileSize: dataSize, memSize: dataSize * 2, vAddr: 0x4000},
		{name: "Loadable segment, fileSize == memSize", progType: elf.PT_LOAD, fileSize: dataSize, memSize: dataSize, vAddr: 0x4000},
		{name: "Loadable segment, segment out-of-range", progType: elf.PT_LOAD, fileSize: dataSize, memSize: dataSize, vAddr: lastAddr - 1, expectedErr: "out of memory range"},
		{name: "Loadable segment, segment just out-of-range", progType: elf.PT_LOAD, fileSize: dataSize, memSize: dataSize, vAddr: lastAddr - dataSize + 2, expectedErr: "out of memory range"},
		{name: "Loadable segment, segment just in-range", progType: elf.PT_LOAD, fileSize: dataSize, memSize: dataSize, vAddr: lastAddr - dataSize + 1, expectedErr: "overlaps with heap"},
		{name: "Loadable segment, segment overlaps heap", progType: elf.PT_LOAD, fileSize: dataSize, memSize: dataSize, vAddr: lastValidAddr - 1, expectedErr: "overlaps with heap"},
		{name: "Loadable segment, segment just overlaps heap", progType: elf.PT_LOAD, fileSize: dataSize, memSize: dataSize, vAddr: lastValidAddr - dataSize + 2, expectedErr: "overlaps with heap"},
		{name: "Loadable segment, segment ends just before heap", progType: elf.PT_LOAD, fileSize: dataSize, memSize: dataSize, vAddr: lastValidAddr - dataSize + 1},
		{name: "MIPS Flags segment, invalid file size", progType: elf.PT_MIPS_ABIFLAGS, fileSize: dataSize * 2, memSize: dataSize, vAddr: 0x4000, shouldIgnore: true},
		{name: "MIPS Flags segment, out-of-range", progType: elf.PT_MIPS_ABIFLAGS, fileSize: dataSize, memSize: dataSize, vAddr: lastAddr, shouldIgnore: true},
		{name: "MIPS Flags segment, overlaps heap", progType: elf.PT_MIPS_ABIFLAGS, fileSize: dataSize, memSize: dataSize, vAddr: lastValidAddr, shouldIgnore: true},
		{name: "Other segment, fileSize > memSize", progType: elf.PT_DYNAMIC, fileSize: dataSize * 2, memSize: dataSize, vAddr: 0x4000, expectedErr: "filling for non PT_LOAD segments is not supported"},
		{name: "Other segment, memSize > fileSize", progType: elf.PT_DYNAMIC, fileSize: dataSize, memSize: dataSize * 2, vAddr: 0x4000, expectedErr: "filling for non PT_LOAD segments is not supported"},
		{name: "Other segment, out-of-range", progType: elf.PT_DYNAMIC, fileSize: dataSize, memSize: dataSize, vAddr: lastAddr, expectedErr: "out of memory range"},
		{name: "Other segment, overlaps heap", progType: elf.PT_DYNAMIC, fileSize: dataSize, memSize: dataSize, vAddr: lastValidAddr, expectedErr: "overlaps with heap"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prog, reader := testutil.MockProgWithReader(tt.progType, tt.fileSize, tt.memSize, tt.vAddr, data)
			progs := []*elf.Prog{prog}
			mockFile := testutil.MockELFFile(progs)
			state, err := LoadELF(mockFile, testutil.MockCreateInitState)

			if tt.expectedErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedErr)
			} else {
				require.NoError(t, err)

				if tt.shouldIgnore {
					// No data should be read
					require.Equal(t, reader.BytesRead, 0)
				} else {
					// Set up memory validation data
					expectedData := make([]byte, tt.memSize)
					copy(expectedData, data[:])
					memReader := state.GetMemory().ReadMemoryRange(arch.Word(tt.vAddr), arch.Word(tt.memSize))
					actualData, err := io.ReadAll(memReader)
					require.NoError(t, err)

					// Validate data was read into memory
					require.Equal(t, reader.BytesRead, int(tt.fileSize))
					require.Equal(t, actualData, expectedData)
				}
			}
		})
	}
}
