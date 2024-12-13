package testutil

import (
	"encoding/binary"
	"fmt"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm/exec"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/memory"
)

func Uint32ToBytes(val uint32) []byte {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, val)

	return data
}

func Uint64ToBytes(val uint64) []byte {
	data := make([]byte, 8)
	binary.BigEndian.PutUint64(data, val)

	return data
}

// StoreInstruction writes a 4-byte instruction to memory
func StoreInstruction(mem *memory.Memory, pc Word, insn uint32) {
	if pc&0x3 != 0 {
		panic(fmt.Errorf("unaligned memory access: %x", pc))
	}
	exec.StoreSubWord(mem, pc, 4, Word(insn), new(exec.NoopMemoryTracker))
}

func GetInstruction(mem *memory.Memory, pc Word) uint32 {
	if pc&0x3 != 0 {
		panic(fmt.Errorf("unaligned memory access: %x", pc))
	}
	return uint32(exec.LoadSubWord(mem, pc, 4, false, new(exec.NoopMemoryTracker)))
}
