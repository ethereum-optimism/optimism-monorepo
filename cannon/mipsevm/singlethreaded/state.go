package singlethreaded

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/memory"
	"github.com/ethereum-optimism/optimism/op-service/serialize"
)

// STATE_WITNESS_SIZE is the size of the state witness encoding in bytes.
// ignoring 64-bit STATE_WITNESS_SIZE as it's not supported for singlethreaded
const STATE_WITNESS_SIZE = 226

type State struct {
	Memory *memory.Memory `json:"memory"`

	PreimageKey    common.Hash `json:"preimageKey"`
	PreimageOffset Word        `json:"preimageOffset"` // note that the offset includes the 8-byte length prefix

	Cpu mipsevm.CpuScalars `json:"cpu"`

	Heap Word `json:"heap"` // to handle mmap growth

	ExitCode uint8 `json:"exit"`
	Exited   bool  `json:"exited"`

	Step uint64 `json:"step"`

	Registers [32]Word `json:"registers"`

	// LastHint is optional metadata, and not part of the VM state itself.
	LastHint hexutil.Bytes `json:"lastHint,omitempty"`
}

var _ mipsevm.FPVMState = (*State)(nil)

func CreateEmptyState() *State {
	return &State{
		Cpu: mipsevm.CpuScalars{
			PC:     0,
			NextPC: 4,
			LO:     0,
			HI:     0,
		},
		Heap:      0,
		Registers: [32]Word{},
		Memory:    memory.NewMemory(),
		ExitCode:  0,
		Exited:    false,
		Step:      0,
	}
}

func CreateInitialState(pc, heapStart Word) *State {
	state := CreateEmptyState()
	state.Cpu.PC = pc
	state.Cpu.NextPC = pc + 4
	state.Heap = heapStart

	return state
}

func (s *State) CreateVM(logger log.Logger, po mipsevm.PreimageOracle, stdOut, stdErr io.Writer, meta mipsevm.Metadata) mipsevm.FPVM {
	return NewInstrumentedState(s, po, stdOut, stdErr, meta)
}

type stateMarshaling struct {
	Memory         *memory.Memory `json:"memory"`
	PreimageKey    common.Hash    `json:"preimageKey"`
	PreimageOffset Word           `json:"preimageOffset"`
	PC             Word           `json:"pc"`
	NextPC         Word           `json:"nextPC"`
	LO             Word           `json:"lo"`
	HI             Word           `json:"hi"`
	Heap           Word           `json:"heap"`
	ExitCode       uint8          `json:"exit"`
	Exited         bool           `json:"exited"`
	Step           uint64         `json:"step"`
	Registers      [32]Word       `json:"registers"`
	LastHint       hexutil.Bytes  `json:"lastHint,omitempty"`
}

func (s *State) MarshalJSON() ([]byte, error) { // nosemgrep
	sm := &stateMarshaling{
		Memory:         s.Memory,
		PreimageKey:    s.PreimageKey,
		PreimageOffset: s.PreimageOffset,
		PC:             s.Cpu.PC,
		NextPC:         s.Cpu.NextPC,
		LO:             s.Cpu.LO,
		HI:             s.Cpu.HI,
		Heap:           s.Heap,
		ExitCode:       s.ExitCode,
		Exited:         s.Exited,
		Step:           s.Step,
		Registers:      s.Registers,
		LastHint:       s.LastHint,
	}
	return json.Marshal(sm)
}

func (s *State) UnmarshalJSON(data []byte) error {
	sm := new(stateMarshaling)
	if err := json.Unmarshal(data, sm); err != nil {
		return err
	}
	s.Memory = sm.Memory
	s.PreimageKey = sm.PreimageKey
	s.PreimageOffset = sm.PreimageOffset
	s.Cpu.PC = sm.PC
	s.Cpu.NextPC = sm.NextPC
	s.Cpu.LO = sm.LO
	s.Cpu.HI = sm.HI
	s.Heap = sm.Heap
	s.ExitCode = sm.ExitCode
	s.Exited = sm.Exited
	s.Step = sm.Step
	s.Registers = sm.Registers
	s.LastHint = sm.LastHint
	return nil
}

func (s *State) GetPC() Word { return s.Cpu.PC }

func (s *State) GetCpu() mipsevm.CpuScalars { return s.Cpu }

func (s *State) GetRegistersRef() *[32]Word { return &s.Registers }

func (s *State) GetExitCode() uint8 { return s.ExitCode }

func (s *State) GetExited() bool { return s.Exited }

func (s *State) GetStep() uint64 { return s.Step }

func (s *State) GetLastHint() hexutil.Bytes {
	return s.LastHint
}

func (s *State) VMStatus() uint8 {
	return mipsevm.VmStatus(s.Exited, s.ExitCode)
}

func (s *State) GetMemory() *memory.Memory {
	return s.Memory
}

func (s *State) GetHeap() Word {
	return s.Heap
}

func (s *State) GetPreimageKey() common.Hash {
	return s.PreimageKey
}

func (s *State) GetPreimageOffset() Word {
	return s.PreimageOffset
}

func (s *State) EncodeWitness() ([]byte, common.Hash) {
	out := make([]byte, 0, STATE_WITNESS_SIZE)
	memRoot := s.Memory.MerkleRoot()
	out = append(out, memRoot[:]...)
	out = append(out, s.PreimageKey[:]...)
	out = arch.ByteOrderWord.AppendWord(out, s.PreimageOffset)
	out = arch.ByteOrderWord.AppendWord(out, s.Cpu.PC)
	out = arch.ByteOrderWord.AppendWord(out, s.Cpu.NextPC)
	out = arch.ByteOrderWord.AppendWord(out, s.Cpu.LO)
	out = arch.ByteOrderWord.AppendWord(out, s.Cpu.HI)
	out = arch.ByteOrderWord.AppendWord(out, s.Heap)
	out = append(out, s.ExitCode)
	out = mipsevm.AppendBoolToWitness(out, s.Exited)
	out = binary.BigEndian.AppendUint64(out, s.Step)
	for _, r := range s.Registers {
		out = arch.ByteOrderWord.AppendWord(out, r)
	}
	return out, stateHashFromWitness(out)
}

// Serialize writes the state in a simple binary format which can be read again using Deserialize
// The format is a simple concatenation of fields, with prefixed item count for repeating items and using big endian
// encoding for numbers.
//
// StateVersion                uint8(0)
// Memory                      As per Memory.Serialize
// PreimageKey                 [32]byte
// PreimageOffset              Word
// Cpu.PC					   Word
// Cpu.NextPC 				   Word
// Cpu.LO 					   Word
// Cpu.HI					   Word
// Heap                        Word
// ExitCode                    uint8
// Exited                      uint8 - 0 for false, 1 for true
// Step                        uint64
// Registers                   [32]Word
// len(LastHint)			   Word (0 when LastHint is nil)
// LastHint 				   []byte
func (s *State) Serialize(out io.Writer) error {
	bout := serialize.NewBinaryWriter(out)

	if err := s.Memory.Serialize(out); err != nil {
		return err
	}
	if err := bout.WriteHash(s.PreimageKey); err != nil {
		return err
	}
	if err := bout.WriteUInt(s.PreimageOffset); err != nil {
		return err
	}
	if err := bout.WriteUInt(s.Cpu.PC); err != nil {
		return err
	}
	if err := bout.WriteUInt(s.Cpu.NextPC); err != nil {
		return err
	}
	if err := bout.WriteUInt(s.Cpu.LO); err != nil {
		return err
	}
	if err := bout.WriteUInt(s.Cpu.HI); err != nil {
		return err
	}
	if err := bout.WriteUInt(s.Heap); err != nil {
		return err
	}
	if err := bout.WriteUInt(s.ExitCode); err != nil {
		return err
	}
	if err := bout.WriteBool(s.Exited); err != nil {
		return err
	}
	if err := bout.WriteUInt(s.Step); err != nil {
		return err
	}
	for _, r := range s.Registers {
		if err := bout.WriteUInt(r); err != nil {
			return err
		}
	}
	if err := bout.WriteBytes(s.LastHint); err != nil {
		return err
	}
	return nil
}

func (s *State) Deserialize(in io.Reader) error {
	bin := serialize.NewBinaryReader(in)
	s.Memory = memory.NewMemory()
	if err := s.Memory.Deserialize(in); err != nil {
		return err
	}
	if err := bin.ReadHash(&s.PreimageKey); err != nil {
		return err
	}
	if err := bin.ReadUInt(&s.PreimageOffset); err != nil {
		return err
	}
	if err := bin.ReadUInt(&s.Cpu.PC); err != nil {
		return err
	}
	if err := bin.ReadUInt(&s.Cpu.NextPC); err != nil {
		return err
	}
	if err := bin.ReadUInt(&s.Cpu.LO); err != nil {
		return err
	}
	if err := bin.ReadUInt(&s.Cpu.HI); err != nil {
		return err
	}
	if err := bin.ReadUInt(&s.Heap); err != nil {
		return err
	}
	if err := bin.ReadUInt(&s.ExitCode); err != nil {
		return err
	}
	if err := bin.ReadBool(&s.Exited); err != nil {
		return err
	}
	if err := bin.ReadUInt(&s.Step); err != nil {
		return err
	}
	for i := range s.Registers {
		if err := bin.ReadUInt(&s.Registers[i]); err != nil {
			return err
		}
	}
	if err := bin.ReadBytes((*[]byte)(&s.LastHint)); err != nil {
		return err
	}
	return nil
}

type StateWitness []byte

func (sw StateWitness) StateHash() (common.Hash, error) {
	if len(sw) != STATE_WITNESS_SIZE {
		return common.Hash{}, fmt.Errorf("Invalid witness length. Got %d, expected %d", len(sw), STATE_WITNESS_SIZE)
	}
	return stateHashFromWitness(sw), nil
}

func GetStateHashFn() mipsevm.HashFn {
	return func(sw []byte) (common.Hash, error) {
		return StateWitness(sw).StateHash()
	}
}

func stateHashFromWitness(sw []byte) common.Hash {
	if len(sw) != STATE_WITNESS_SIZE {
		panic("Invalid witness length")
	}
	hash := crypto.Keccak256Hash(sw)
	offset := 32*2 + 4*6
	exitCode := sw[offset]
	exited := sw[offset+1]
	status := mipsevm.VmStatus(exited == 1, exitCode)
	hash[0] = status
	return hash
}
