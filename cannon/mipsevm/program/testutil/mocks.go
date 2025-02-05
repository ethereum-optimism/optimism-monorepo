package testutil

import (
	"debug/elf"
	"io"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/memory"
)

// MockELFFile create a mock ELF file with custom program segments
func MockELFFile(progs []*elf.Prog) *elf.File {
	return &elf.File{Progs: progs}
}

// MockProg sets up a elf.Prog structure for testing
func MockProg(progType elf.ProgType, filesz, memsz, vaddr uint64) *elf.Prog {
	return &elf.Prog{
		ProgHeader: elf.ProgHeader{
			Type:   progType,
			Filesz: filesz,
			Memsz:  memsz,
			Vaddr:  vaddr,
		},
	}
}

// MockProgWithReader creates an elf.Prog with a TrackableReaderAt to track reads
func MockProgWithReader(progType elf.ProgType, filesz, memsz, vaddr uint64, data []byte) (*elf.Prog, *TrackableReaderAt) {
	reader := &TrackableReaderAt{data: data}
	prog := MockProg(progType, filesz, memsz, vaddr)
	prog.ReaderAt = io.NewSectionReader(reader, 0, int64(filesz))
	return prog, reader
}

// TrackableReaderAt tracks the number of bytes read
type TrackableReaderAt struct {
	data      []byte
	BytesRead int
}

func (r *TrackableReaderAt) ReadAt(p []byte, offset int64) (int, error) {
	if offset >= int64(len(r.data)) {
		return 0, io.EOF
	}
	numBytesRead := copy(p, r.data[offset:])
	r.BytesRead += numBytesRead
	if numBytesRead < len(p) {
		return numBytesRead, io.EOF
	}
	return numBytesRead, nil
}

// MockCreateInitState returns a mock FPVMState for testing
func MockCreateInitState(pc, heapStart arch.Word) *MockFPVMState {
	return newMockFPVMState()
}

type MockFPVMState struct {
	memory *memory.Memory
}

var _ mipsevm.FPVMState = (*MockFPVMState)(nil)

func newMockFPVMState() *MockFPVMState {
	mem := memory.NewMemory()
	state := MockFPVMState{mem}
	return &state
}

func (m MockFPVMState) Serialize(out io.Writer) error {
	panic("not implemented")
}

func (m MockFPVMState) GetMemory() *memory.Memory {
	return m.memory
}

func (m MockFPVMState) GetHeap() arch.Word {
	panic("not implemented")
}

func (m MockFPVMState) GetPreimageKey() common.Hash {
	panic("not implemented")
}

func (m MockFPVMState) GetPreimageOffset() arch.Word {
	panic("not implemented")
}

func (m MockFPVMState) GetPC() arch.Word {
	panic("not implemented")
}

func (m MockFPVMState) GetCpu() mipsevm.CpuScalars {
	panic("not implemented")
}

func (m MockFPVMState) GetRegistersRef() *[32]arch.Word {
	panic("not implemented")
}

func (m MockFPVMState) GetStep() uint64 {
	panic("not implemented")
}

func (m MockFPVMState) GetExited() bool {
	panic("not implemented")
}

func (m MockFPVMState) GetExitCode() uint8 {
	panic("not implemented")
}

func (m MockFPVMState) GetLastHint() hexutil.Bytes {
	panic("not implemented")
}

func (m MockFPVMState) EncodeWitness() (witness []byte, hash common.Hash) {
	panic("not implemented")
}

func (m MockFPVMState) CreateVM(logger log.Logger, po mipsevm.PreimageOracle, stdOut, stdErr io.Writer, meta mipsevm.Metadata) mipsevm.FPVM {
	panic("not implemented")
}
