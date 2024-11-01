package multithreaded

import (
	"bytes"
	"debug/elf"
	"encoding/json"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/exec"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/memory"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/program"
)

func setWitnessField(witness StateWitness, fieldOffset int, fieldData []byte) {
	start := fieldOffset
	end := fieldOffset + len(fieldData)
	copy(witness[start:end], fieldData)
}

func setWitnessWord(witness StateWitness, fieldOffset int, value arch.Word) {
	arch.ByteOrderWord.PutWord(witness[fieldOffset:], value)
}

// Run through all permutations of `exited` / `exitCode` and ensure that the
// correct witness, state hash, and VM Status is produced.
func TestState_EncodeWitness(t *testing.T) {
	cases := []struct {
		exited   bool
		exitCode uint8
	}{
		{exited: false, exitCode: 0},
		{exited: false, exitCode: 1},
		{exited: false, exitCode: 2},
		{exited: false, exitCode: 3},
		{exited: true, exitCode: 0},
		{exited: true, exitCode: 1},
		{exited: true, exitCode: 2},
		{exited: true, exitCode: 3},
	}

	heap := Word(12)
	llAddress := Word(55)
	llThreadOwner := Word(99)
	preimageKey := crypto.Keccak256Hash([]byte{1, 2, 3, 4})
	preimageOffset := Word(24)
	step := uint64(33)
	stepsSinceContextSwitch := uint64(123)
	for _, c := range cases {
		state := CreateEmptyState()
		state.Exited = c.exited
		state.ExitCode = c.exitCode
		state.PreimageKey = preimageKey
		state.PreimageOffset = preimageOffset
		state.Heap = heap
		state.LLReservationStatus = LLStatusActive32bit
		state.LLAddress = llAddress
		state.LLOwnerThread = llThreadOwner
		state.Step = step
		state.StepsSinceLastContextSwitch = stepsSinceContextSwitch

		memRoot := state.Memory.MerkleRoot()
		leftStackRoot := state.calculateThreadStackRoot(state.LeftThreadStack)
		rightStackRoot := EmptyThreadsRoot

		// Set up expected witness
		expectedWitness := make(StateWitness, STATE_WITNESS_SIZE)
		setWitnessField(expectedWitness, MEMROOT_WITNESS_OFFSET, memRoot[:])
		setWitnessField(expectedWitness, PREIMAGE_KEY_WITNESS_OFFSET, preimageKey[:])
		setWitnessWord(expectedWitness, PREIMAGE_OFFSET_WITNESS_OFFSET, preimageOffset)
		setWitnessWord(expectedWitness, HEAP_WITNESS_OFFSET, heap)
		setWitnessField(expectedWitness, LL_RESERVATION_ACTIVE_OFFSET, []byte{1})
		setWitnessWord(expectedWitness, LL_ADDRESS_OFFSET, llAddress)
		setWitnessWord(expectedWitness, LL_OWNER_THREAD_OFFSET, llThreadOwner)
		setWitnessField(expectedWitness, EXITCODE_WITNESS_OFFSET, []byte{c.exitCode})
		if c.exited {
			setWitnessField(expectedWitness, EXITED_WITNESS_OFFSET, []byte{1})
		}
		setWitnessField(expectedWitness, STEP_WITNESS_OFFSET, []byte{0, 0, 0, 0, 0, 0, 0, byte(step)})
		setWitnessField(expectedWitness, STEPS_SINCE_CONTEXT_SWITCH_WITNESS_OFFSET, []byte{0, 0, 0, 0, 0, 0, 0, byte(stepsSinceContextSwitch)})
		setWitnessWord(expectedWitness, WAKEUP_WITNESS_OFFSET, ^arch.Word(0))
		setWitnessField(expectedWitness, TRAVERSE_RIGHT_WITNESS_OFFSET, []byte{0})
		setWitnessField(expectedWitness, LEFT_THREADS_ROOT_WITNESS_OFFSET, leftStackRoot[:])
		setWitnessField(expectedWitness, RIGHT_THREADS_ROOT_WITNESS_OFFSET, rightStackRoot[:])
		setWitnessWord(expectedWitness, THREAD_ID_WITNESS_OFFSET, 1)

		// Validate witness
		actualWitness, actualStateHash := state.EncodeWitness()
		require.Equal(t, len(actualWitness), STATE_WITNESS_SIZE, "Incorrect witness size")
		require.EqualValues(t, expectedWitness[:], actualWitness[:], "Incorrect witness: \n\tactual = \t0x%x \n\texpected = \t0x%x", actualWitness, expectedWitness)
		// Validate witness hash
		expectedStateHash := crypto.Keccak256Hash(actualWitness)
		expectedStateHash[0] = mipsevm.VmStatus(c.exited, c.exitCode)
		require.Equal(t, expectedStateHash, actualStateHash, "Incorrect state hash")
	}
}

func TestState_JSONCodec(t *testing.T) {
	elfProgram, err := elf.Open("../../testdata/example/bin/hello.elf")
	require.NoError(t, err, "open ELF file")
	state, err := program.LoadELF(elfProgram, CreateInitialState)
	require.NoError(t, err, "load ELF into state")
	// Set a few additional fields
	state.PreimageKey = crypto.Keccak256Hash([]byte{1, 2, 3, 4})
	state.PreimageOffset = 4
	state.Heap = 555
	state.Step = 99_999
	state.StepsSinceLastContextSwitch = 123
	state.Exited = true
	state.ExitCode = 2
	state.LastHint = []byte{11, 12, 13}

	stateJSON, err := json.Marshal(state)
	require.NoError(t, err)

	var newState *State
	err = json.Unmarshal(stateJSON, &newState)
	require.NoError(t, err)

	require.Equal(t, state.PreimageKey, newState.PreimageKey)
	require.Equal(t, state.PreimageOffset, newState.PreimageOffset)
	require.Equal(t, state.Heap, newState.Heap)
	require.Equal(t, state.ExitCode, newState.ExitCode)
	require.Equal(t, state.Exited, newState.Exited)
	require.Equal(t, state.Memory.MerkleRoot(), newState.Memory.MerkleRoot())
	require.Equal(t, state.Step, newState.Step)
	require.Equal(t, state.StepsSinceLastContextSwitch, newState.StepsSinceLastContextSwitch)
	require.Equal(t, state.Wakeup, newState.Wakeup)
	require.Equal(t, state.TraverseRight, newState.TraverseRight)
	require.Equal(t, state.LeftThreadStack, newState.LeftThreadStack)
	require.Equal(t, state.RightThreadStack, newState.RightThreadStack)
	require.Equal(t, state.NextThreadId, newState.NextThreadId)
	require.Equal(t, state.LastHint, newState.LastHint)
}

func TestState_Binary(t *testing.T) {
	elfProgram, err := elf.Open("../../testdata/example/bin/hello.elf")
	require.NoError(t, err, "open ELF file")
	state, err := program.LoadELF(elfProgram, CreateInitialState)
	require.NoError(t, err, "load ELF into state")
	// Set a few additional fields
	state.PreimageKey = crypto.Keccak256Hash([]byte{1, 2, 3, 4})
	state.PreimageOffset = 4
	state.Heap = 555
	state.Step = 99_999
	state.StepsSinceLastContextSwitch = 123
	state.Exited = true
	state.ExitCode = 2
	state.LastHint = []byte{11, 12, 13}

	buf := new(bytes.Buffer)
	err = state.Serialize(buf)
	require.NoError(t, err)

	newState := new(State)
	require.NoError(t, newState.Deserialize(bytes.NewReader(buf.Bytes())))

	require.Equal(t, state.PreimageKey, newState.PreimageKey)
	require.Equal(t, state.PreimageOffset, newState.PreimageOffset)
	require.Equal(t, state.Heap, newState.Heap)
	require.Equal(t, state.ExitCode, newState.ExitCode)
	require.Equal(t, state.Exited, newState.Exited)
	require.Equal(t, state.Memory.MerkleRoot(), newState.Memory.MerkleRoot())
	require.Equal(t, state.Step, newState.Step)
	require.Equal(t, state.StepsSinceLastContextSwitch, newState.StepsSinceLastContextSwitch)
	require.Equal(t, state.Wakeup, newState.Wakeup)
	require.Equal(t, state.TraverseRight, newState.TraverseRight)
	require.Equal(t, state.LeftThreadStack, newState.LeftThreadStack)
	require.Equal(t, state.RightThreadStack, newState.RightThreadStack)
	require.Equal(t, state.NextThreadId, newState.NextThreadId)
	require.Equal(t, state.LastHint, newState.LastHint)
}

func TestSerializeStateRoundTrip(t *testing.T) {
	// Construct a test case with populated fields
	mem := memory.NewMemory()
	mem.AllocPage(5)
	p := mem.AllocPage(123)
	p.Data[2] = 0x01
	state := &State{
		Memory:                      mem,
		PreimageKey:                 common.Hash{0xFF},
		PreimageOffset:              5,
		Heap:                        0xc0ffee,
		LLReservationStatus:         LLStatusActive64bit,
		LLAddress:                   0x12345678,
		LLOwnerThread:               0x02,
		ExitCode:                    1,
		Exited:                      true,
		Step:                        0xdeadbeef,
		StepsSinceLastContextSwitch: 334,
		Wakeup:                      42,
		TraverseRight:               true,
		LeftThreadStack: []*ThreadState{
			{
				ThreadId:         45,
				ExitCode:         46,
				Exited:           true,
				FutexAddr:        47,
				FutexVal:         48,
				FutexTimeoutStep: 49,
				Cpu: mipsevm.CpuScalars{
					PC:     0xFF,
					NextPC: 0xFF + 4,
					LO:     0xbeef,
					HI:     0xbabe,
				},
				Registers: [32]Word{
					0xdeadbeef,
					0xdeadbeef,
					0xc0ffee,
					0xbeefbabe,
					0xdeadc0de,
					0xbadc0de,
					0xdeaddead,
				},
			},
			{
				ThreadId:         55,
				ExitCode:         56,
				Exited:           false,
				FutexAddr:        57,
				FutexVal:         58,
				FutexTimeoutStep: 59,
				Cpu: mipsevm.CpuScalars{
					PC:     0xEE,
					NextPC: 0xEE + 4,
					LO:     0xeeef,
					HI:     0xeabe,
				},
				Registers: [32]Word{
					0xabcdef,
					0x123456,
				},
			},
		},
		RightThreadStack: []*ThreadState{
			{
				ThreadId:         65,
				ExitCode:         66,
				Exited:           false,
				FutexAddr:        67,
				FutexVal:         68,
				FutexTimeoutStep: 69,
				Cpu: mipsevm.CpuScalars{
					PC:     0xdd,
					NextPC: 0xdd + 4,
					LO:     0xdeef,
					HI:     0xdabe,
				},
				Registers: [32]Word{
					0x654321,
				},
			},
			{
				ThreadId:         75,
				ExitCode:         76,
				Exited:           true,
				FutexAddr:        77,
				FutexVal:         78,
				FutexTimeoutStep: 79,
				Cpu: mipsevm.CpuScalars{
					PC:     0xcc,
					NextPC: 0xcc + 4,
					LO:     0xceef,
					HI:     0xcabe,
				},
				Registers: [32]Word{
					0x987653,
					0xfedbca,
				},
			},
		},
		NextThreadId: 489,
		LastHint:     hexutil.Bytes{1, 2, 3, 4, 5},
	}

	ser := new(bytes.Buffer)
	err := state.Serialize(ser)
	require.NoError(t, err, "must serialize state")
	state2 := &State{}
	err = state2.Deserialize(ser)
	require.NoError(t, err, "must deserialize state")
	require.Equal(t, state, state2, "must roundtrip state")
}

func TestState_EmptyThreadsRoot(t *testing.T) {
	data := [64]byte{}
	expectedEmptyRoot := crypto.Keccak256Hash(data[:])

	require.Equal(t, expectedEmptyRoot, EmptyThreadsRoot)
}

func TestState_EncodeThreadProof_SingleThread(t *testing.T) {
	state := CreateEmptyState()
	// Set some fields on the active thread
	activeThread := state.GetCurrentThread()
	activeThread.Cpu.PC = 4
	activeThread.Cpu.NextPC = 8
	activeThread.Cpu.HI = 11
	activeThread.Cpu.LO = 22
	for i := 0; i < 32; i++ {
		activeThread.Registers[i] = Word(i)
	}

	expectedProof := append([]byte{}, activeThread.serializeThread()[:]...)
	expectedProof = append(expectedProof, EmptyThreadsRoot[:]...)

	actualProof := state.EncodeThreadProof()
	require.Equal(t, THREAD_WITNESS_SIZE, len(actualProof))
	require.Equal(t, expectedProof, actualProof)
}

func TestState_EncodeThreadProof_MultipleThreads(t *testing.T) {
	state := CreateEmptyState()
	// Add some more threads
	require.Equal(t, state.TraverseRight, false, "sanity check")
	state.LeftThreadStack = append(state.LeftThreadStack, CreateEmptyThread())
	state.LeftThreadStack = append(state.LeftThreadStack, CreateEmptyThread())
	require.Equal(t, 3, len(state.LeftThreadStack), "sanity check")

	// Set some fields on our threads
	for i := 0; i < 3; i++ {
		curThread := state.LeftThreadStack[i]
		curThread.Cpu.PC = Word(4 * i)
		curThread.Cpu.NextPC = curThread.Cpu.PC + 4
		curThread.Cpu.HI = Word(11 + i)
		curThread.Cpu.LO = Word(22 + i)
		for j := 0; j < 32; j++ {
			curThread.Registers[j] = Word(j + i)
		}
	}

	expectedRoot := EmptyThreadsRoot
	for i := 0; i < 2; i++ {
		curThread := state.LeftThreadStack[i]
		hashedThread := crypto.Keccak256Hash(curThread.serializeThread())

		// root = prevRoot ++ hash(curRoot)
		hashData := append([]byte{}, expectedRoot[:]...)
		hashData = append(hashData, hashedThread[:]...)
		expectedRoot = crypto.Keccak256Hash(hashData)
	}

	expectedProof := append([]byte{}, state.GetCurrentThread().serializeThread()[:]...)
	expectedProof = append(expectedProof, expectedRoot[:]...)

	actualProof := state.EncodeThreadProof()
	require.Equal(t, THREAD_WITNESS_SIZE, len(actualProof))
	require.Equal(t, expectedProof, actualProof)
}

func TestState_EncodeThreadProof_EmptyThreadStackPanic(t *testing.T) {
	cases := []struct {
		name          string
		wakeupAddr    Word
		traverseRight bool
	}{
		{"traverse left during wakeup traversal", Word(99), false},
		{"traverse left during normal traversal", exec.FutexEmptyAddr, false},
		{"traverse right during wakeup traversal", Word(99), true},
		{"traverse right during normal traversal", exec.FutexEmptyAddr, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Set up invalid state where the active stack is empty
			state := CreateEmptyState()
			state.Wakeup = c.wakeupAddr
			state.TraverseRight = c.traverseRight
			if c.traverseRight {
				state.LeftThreadStack = []*ThreadState{CreateEmptyThread()}
				state.RightThreadStack = []*ThreadState{}
			} else {
				state.LeftThreadStack = []*ThreadState{}
				state.RightThreadStack = []*ThreadState{CreateEmptyThread()}
			}

			assert.PanicsWithValue(t, "Invalid empty thread stack", func() { state.EncodeThreadProof() })
		})
	}
}

func TestStateWitnessSize(t *testing.T) {
	expectedWitnessSize := 172
	if !arch.IsMips32 {
		expectedWitnessSize = 196
	}
	require.Equal(t, expectedWitnessSize, STATE_WITNESS_SIZE)
}

func TestThreadStateWitnessSize(t *testing.T) {
	expectedWitnessSize := 166
	if !arch.IsMips32 {
		expectedWitnessSize = 322
	}
	require.Equal(t, expectedWitnessSize, SERIALIZED_THREAD_SIZE)
}
