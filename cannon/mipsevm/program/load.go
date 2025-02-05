package program

import (
	"bytes"
	"debug/elf"
	"fmt"
	"io"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
)

const (
	HEAP_START    = arch.HeapStart
	HEAP_END      = arch.HeapEnd
	PROGRAM_BREAK = arch.ProgramBreak
)

type Word = arch.Word

type CreateInitialFPVMState[T mipsevm.FPVMState] func(pc, heapStart Word) T

func LoadELF[T mipsevm.FPVMState](f *elf.File, initState CreateInitialFPVMState[T]) (T, error) {
	var empty T
	s := initState(Word(f.Entry), HEAP_START)

	for i, prog := range f.Progs {
		if prog.Type == elf.PT_MIPS_ABIFLAGS {
			continue
		}

		r := io.Reader(io.NewSectionReader(prog, 0, int64(prog.Filesz)))
		if prog.Filesz != prog.Memsz {
			if prog.Type == elf.PT_LOAD {
				if prog.Filesz < prog.Memsz {
					r = io.MultiReader(r, bytes.NewReader(make([]byte, prog.Memsz-prog.Filesz)))
				} else {
					return empty, fmt.Errorf("invalid PT_LOAD program segment %d, file size (%d) > mem size (%d)", i, prog.Filesz, prog.Memsz)
				}
			} else {
				return empty, fmt.Errorf("program segment %d has different file size (%d) than mem size (%d): filling for non PT_LOAD segments is not supported", i, prog.Filesz, prog.Memsz)
			}
		}

		if prog.Memsz == 0 {
			// Nothing to do
			continue
		}

		// Calculate the architecture-specific last valid memory address
		var lastMemoryAddr uint64
		if arch.IsMips32 {
			// 32-bit virtual address space
			lastMemoryAddr = (1 << 32) - 1
		} else {
			// 48-bit virtual address space
			lastMemoryAddr = (1 << 48) - 1
		}

		lastByteToWrite := prog.Vaddr + prog.Memsz - 1
		if lastByteToWrite > lastMemoryAddr || lastByteToWrite < prog.Vaddr {
			return empty, fmt.Errorf("program %d out of memory range: %x - %x (size: %x)", i, prog.Vaddr, lastByteToWrite, prog.Memsz)
		}
		if lastByteToWrite >= HEAP_START {
			return empty, fmt.Errorf("program %d overlaps with heap: %x - %x (size: %x). The heap start offset must be reconfigured", i, prog.Vaddr, lastByteToWrite, prog.Memsz)
		}
		if err := s.GetMemory().SetMemoryRange(Word(prog.Vaddr), r); err != nil {
			return empty, fmt.Errorf("failed to read program segment %d: %w", i, err)
		}
	}

	return s, nil
}
