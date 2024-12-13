package multithreaded

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/exec"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/memory"
	"github.com/ethereum-optimism/optimism/op-service/serialize"
)

// STATE_WITNESS_SIZE is the size of the state witness encoding in bytes.
const (
	MEMROOT_WITNESS_OFFSET                    = 0
	PREIMAGE_KEY_WITNESS_OFFSET               = MEMROOT_WITNESS_OFFSET + 32
	PREIMAGE_OFFSET_WITNESS_OFFSET            = PREIMAGE_KEY_WITNESS_OFFSET + 32
	HEAP_WITNESS_OFFSET                       = PREIMAGE_OFFSET_WITNESS_OFFSET + arch.WordSizeBytes
	LL_RESERVATION_ACTIVE_OFFSET              = HEAP_WITNESS_OFFSET + arch.WordSizeBytes
	LL_ADDRESS_OFFSET                         = LL_RESERVATION_ACTIVE_OFFSET + 1
	LL_OWNER_THREAD_OFFSET                    = LL_ADDRESS_OFFSET + arch.WordSizeBytes
	EXITCODE_WITNESS_OFFSET                   = LL_OWNER_THREAD_OFFSET + arch.WordSizeBytes
	EXITED_WITNESS_OFFSET                     = EXITCODE_WITNESS_OFFSET + 1
	STEP_WITNESS_OFFSET                       = EXITED_WITNESS_OFFSET + 1
	STEPS_SINCE_CONTEXT_SWITCH_WITNESS_OFFSET = STEP_WITNESS_OFFSET + 8
	WAKEUP_WITNESS_OFFSET                     = STEPS_SINCE_CONTEXT_SWITCH_WITNESS_OFFSET + 8
	TRAVERSE_RIGHT_WITNESS_OFFSET             = WAKEUP_WITNESS_OFFSET + arch.WordSizeBytes
	LEFT_THREADS_ROOT_WITNESS_OFFSET          = TRAVERSE_RIGHT_WITNESS_OFFSET + 1
	RIGHT_THREADS_ROOT_WITNESS_OFFSET         = LEFT_THREADS_ROOT_WITNESS_OFFSET + 32
	THREAD_ID_WITNESS_OFFSET                  = RIGHT_THREADS_ROOT_WITNESS_OFFSET + 32

	// 172 and 196 bytes for 32 and 64-bit respectively
	STATE_WITNESS_SIZE = THREAD_ID_WITNESS_OFFSET + arch.WordSizeBytes
)

type LLReservationStatus uint8

const (
	LLStatusNone        LLReservationStatus = 0x0
	LLStatusActive32bit LLReservationStatus = 0x1
	LLStatusActive64bit LLReservationStatus = 0x2
)

type State struct {
	Memory *memory.Memory

	PreimageKey    common.Hash
	PreimageOffset Word // note that the offset includes the 8-byte length prefix

	Heap                Word                // to handle mmap growth
	LLReservationStatus LLReservationStatus // Determines whether there is an active memory reservation, and what type
	LLAddress           Word                // The "linked" memory address reserved via the LL (load linked) op
	LLOwnerThread       Word                // The id of the thread that holds the reservation on LLAddress

	ExitCode uint8
	Exited   bool

	Step                        uint64
	StepsSinceLastContextSwitch uint64
	Wakeup                      Word

	TraverseRight    bool
	LeftThreadStack  []*ThreadState
	RightThreadStack []*ThreadState
	NextThreadId     Word

	// LastHint is optional metadata, and not part of the VM state itself.
	LastHint hexutil.Bytes
}

var _ mipsevm.FPVMState = (*State)(nil)

func CreateEmptyState() *State {
	initThread := CreateEmptyThread()

	return &State{
		Memory:              memory.NewMemory(),
		Heap:                0,
		LLReservationStatus: LLStatusNone,
		LLAddress:           0,
		LLOwnerThread:       0,
		ExitCode:            0,
		Exited:              false,
		Step:                0,
		Wakeup:              exec.FutexEmptyAddr,
		TraverseRight:       false,
		LeftThreadStack:     []*ThreadState{initThread},
		RightThreadStack:    []*ThreadState{},
		NextThreadId:        initThread.ThreadId + 1,
	}
}

func CreateInitialState(pc, heapStart Word) *State {
	state := CreateEmptyState()
	currentThread := state.GetCurrentThread()
	currentThread.Cpu.PC = pc
	currentThread.Cpu.NextPC = pc + 4
	state.Heap = heapStart

	return state
}

func (s *State) CreateVM(logger log.Logger, po mipsevm.PreimageOracle, stdOut, stdErr io.Writer, meta mipsevm.Metadata) mipsevm.FPVM {
	logger.Info("Using cannon multithreaded VM", "is32", arch.IsMips32)
	return NewInstrumentedState(s, po, stdOut, stdErr, logger, meta)
}

func (s *State) GetCurrentThread() *ThreadState {
	activeStack := s.getActiveThreadStack()

	activeStackSize := len(activeStack)
	if activeStackSize == 0 {
		panic("Active thread stack is empty")
	}

	return activeStack[activeStackSize-1]
}

func (s *State) getActiveThreadStack() []*ThreadState {
	var activeStack []*ThreadState
	if s.TraverseRight {
		activeStack = s.RightThreadStack
	} else {
		activeStack = s.LeftThreadStack
	}

	return activeStack
}

func (s *State) getRightThreadStackRoot() common.Hash {
	return s.calculateThreadStackRoot(s.RightThreadStack)
}

func (s *State) getLeftThreadStackRoot() common.Hash {
	return s.calculateThreadStackRoot(s.LeftThreadStack)
}

func (s *State) calculateThreadStackRoot(stack []*ThreadState) common.Hash {
	curRoot := EmptyThreadsRoot
	for _, thread := range stack {
		curRoot = computeThreadRoot(curRoot, thread)
	}

	return curRoot
}

func (s *State) GetPC() Word {
	activeThread := s.GetCurrentThread()
	return activeThread.Cpu.PC
}

func (s *State) GetCpu() mipsevm.CpuScalars {
	activeThread := s.GetCurrentThread()
	return activeThread.Cpu
}

func (s *State) getCpuRef() *mipsevm.CpuScalars {
	return &s.GetCurrentThread().Cpu
}

func (s *State) GetRegistersRef() *[32]Word {
	activeThread := s.GetCurrentThread()
	return &activeThread.Registers
}

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
	out = arch.ByteOrderWord.AppendWord(out, s.Heap)
	out = append(out, byte(s.LLReservationStatus))
	out = arch.ByteOrderWord.AppendWord(out, s.LLAddress)
	out = arch.ByteOrderWord.AppendWord(out, s.LLOwnerThread)
	out = append(out, s.ExitCode)
	out = mipsevm.AppendBoolToWitness(out, s.Exited)

	out = binary.BigEndian.AppendUint64(out, s.Step)
	out = binary.BigEndian.AppendUint64(out, s.StepsSinceLastContextSwitch)
	out = arch.ByteOrderWord.AppendWord(out, s.Wakeup)

	leftStackRoot := s.getLeftThreadStackRoot()
	rightStackRoot := s.getRightThreadStackRoot()
	out = mipsevm.AppendBoolToWitness(out, s.TraverseRight)
	out = append(out, (leftStackRoot)[:]...)
	out = append(out, (rightStackRoot)[:]...)
	out = arch.ByteOrderWord.AppendWord(out, s.NextThreadId)

	return out, stateHashFromWitness(out)
}

func (s *State) EncodeThreadProof() []byte {
	activeStack := s.getActiveThreadStack()
	threadCount := len(activeStack)
	if threadCount == 0 {
		panic("Invalid empty thread stack")
	}

	activeThread := activeStack[threadCount-1]
	otherThreads := activeStack[:threadCount-1]
	threadBytes := activeThread.serializeThread()
	otherThreadsWitness := s.calculateThreadStackRoot(otherThreads)

	out := make([]byte, 0, THREAD_WITNESS_SIZE)
	out = append(out, threadBytes[:]...)
	out = append(out, otherThreadsWitness[:]...)
	return out
}

func (s *State) ThreadCount() int {
	return len(s.LeftThreadStack) + len(s.RightThreadStack)
}

// Serialize writes the state in a simple binary format which can be read again using Deserialize
// The format is a simple concatenation of fields, with prefixed item count for repeating items and using big endian
// encoding for numbers.
//
// StateVersion                uint8(1)
// Memory                      As per Memory.Serialize
// PreimageKey                 [32]byte
// PreimageOffset              Word
// Heap                        Word
// ExitCode                    uint8
// Exited                      uint8 - 0 for false, 1 for true
// Step                        uint64
// StepsSinceLastContextSwitch uint64
// Wakeup                      Word
// TraverseRight               uint8 - 0 for false, 1 for true
// NextThreadId                Word
// len(LeftThreadStack)        Word
// LeftThreadStack entries     as per ThreadState.Serialize
// len(RightThreadStack)       Word
// RightThreadStack entries    as per ThreadState.Serialize
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
	if err := bout.WriteUInt(s.Heap); err != nil {
		return err
	}
	if err := bout.WriteUInt(s.LLReservationStatus); err != nil {
		return err
	}
	if err := bout.WriteUInt(s.LLAddress); err != nil {
		return err
	}
	if err := bout.WriteUInt(s.LLOwnerThread); err != nil {
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
	if err := bout.WriteUInt(s.StepsSinceLastContextSwitch); err != nil {
		return err
	}
	if err := bout.WriteUInt(s.Wakeup); err != nil {
		return err
	}
	if err := bout.WriteBool(s.TraverseRight); err != nil {
		return err
	}
	if err := bout.WriteUInt(s.NextThreadId); err != nil {
		return err
	}

	if err := bout.WriteUInt(Word(len(s.LeftThreadStack))); err != nil {
		return err
	}
	for _, stack := range s.LeftThreadStack {
		if err := stack.Serialize(out); err != nil {
			return err
		}
	}
	if err := bout.WriteUInt(Word(len(s.RightThreadStack))); err != nil {
		return err
	}
	for _, stack := range s.RightThreadStack {
		if err := stack.Serialize(out); err != nil {
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
	if err := bin.ReadUInt(&s.Heap); err != nil {
		return err
	}
	if err := bin.ReadUInt(&s.LLReservationStatus); err != nil {
		return err
	}
	if err := bin.ReadUInt(&s.LLAddress); err != nil {
		return err
	}
	if err := bin.ReadUInt(&s.LLOwnerThread); err != nil {
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
	if err := bin.ReadUInt(&s.StepsSinceLastContextSwitch); err != nil {
		return err
	}
	if err := bin.ReadUInt(&s.Wakeup); err != nil {
		return err
	}
	if err := bin.ReadBool(&s.TraverseRight); err != nil {
		return err
	}

	if err := bin.ReadUInt(&s.NextThreadId); err != nil {
		return err
	}

	var leftThreadStackSize Word
	if err := bin.ReadUInt(&leftThreadStackSize); err != nil {
		return err
	}
	s.LeftThreadStack = make([]*ThreadState, leftThreadStackSize)
	for i := range s.LeftThreadStack {
		s.LeftThreadStack[i] = &ThreadState{}
		if err := s.LeftThreadStack[i].Deserialize(in); err != nil {
			return err
		}
	}

	var rightThreadStackSize Word
	if err := bin.ReadUInt(&rightThreadStackSize); err != nil {
		return err
	}
	s.RightThreadStack = make([]*ThreadState, rightThreadStackSize)
	for i := range s.RightThreadStack {
		s.RightThreadStack[i] = &ThreadState{}
		if err := s.RightThreadStack[i].Deserialize(in); err != nil {
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
		panic(fmt.Sprintf("Invalid witness length. Got %d, expected %d", len(sw), STATE_WITNESS_SIZE))
	}
	hash := crypto.Keccak256Hash(sw)
	exitCode := sw[EXITCODE_WITNESS_OFFSET]
	exited := sw[EXITED_WITNESS_OFFSET]
	status := mipsevm.VmStatus(exited == 1, exitCode)
	hash[0] = status
	return hash
}
