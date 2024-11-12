package tests

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/exec"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/multithreaded"
	mttestutil "github.com/ethereum-optimism/optimism/cannon/mipsevm/multithreaded/testutil"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/testutil"
	preimage "github.com/ethereum-optimism/optimism/op-preimage"
	"github.com/ethereum/go-ethereum/crypto"
)

type operatorTestCase struct {
	name      string
	isImm     bool
	rs        Word
	rt        Word
	imm       uint16
	funct     uint32
	opcode    uint32
	expectRes Word
}

func testOperators(t *testing.T, cases []operatorTestCase, mips32Insn bool) {
	versions := GetMipsVersionTestCases(t)
	for _, v := range versions {
		for i, tt := range cases {
			// sign extend inputs for 64-bit compatibility
			if mips32Insn {
				tt.rs = randomizeUpperWord(signExtend64(tt.rs))
				tt.rt = randomizeUpperWord(signExtend64(tt.rt))
				tt.expectRes = signExtend64(tt.expectRes)
			}

			testName := fmt.Sprintf("%v (%v)", tt.name, v.Name)
			t.Run(testName, func(t *testing.T) {
				validator := testutil.NewEvmValidator(t, v.StateHashFn, v.Contracts)
				goVm := v.VMFactory(nil, os.Stdout, os.Stderr, testutil.CreateLogger(), testutil.WithRandomization(int64(i)), testutil.WithPC(0), testutil.WithNextPC(4))
				state := goVm.GetState()
				var insn uint32
				var baseReg uint32 = 17
				var rtReg uint32
				var rdReg uint32
				if tt.isImm {
					rtReg = 8
					insn = tt.opcode<<26 | baseReg<<21 | rtReg<<16 | uint32(tt.imm)
					state.GetRegistersRef()[rtReg] = tt.rt
					state.GetRegistersRef()[baseReg] = tt.rs
				} else {
					rtReg = 18
					rdReg = 8
					insn = baseReg<<21 | rtReg<<16 | rdReg<<11 | tt.funct
					state.GetRegistersRef()[baseReg] = tt.rs
					state.GetRegistersRef()[rtReg] = tt.rt
				}
				testutil.StoreInstruction(state.GetMemory(), 0, insn)
				step := state.GetStep()

				// Setup expectations
				expected := testutil.NewExpectedState(state)
				expected.Step += 1
				expected.PC = 4
				expected.NextPC = 8
				if tt.isImm {
					expected.Registers[rtReg] = tt.expectRes
				} else {
					expected.Registers[rdReg] = tt.expectRes
				}

				stepWitness, err := goVm.Step(true)
				require.NoError(t, err)

				// Check expectations
				expected.Validate(t, state)
				validator.ValidateEVM(t, stepWitness, step, goVm)
			})
		}
	}
}

type mulDivTestCase struct {
	name      string
	rs        Word
	rt        Word
	funct     uint32
	opcode    uint32
	expectHi  Word
	expectLo  Word
	expectRes Word
	rdReg     uint32
	panicMsg  string
	revertMsg string
}

func testMulDiv(t *testing.T, cases []mulDivTestCase, mips32Insn bool) {
	versions := GetMipsVersionTestCases(t)
	for _, v := range versions {
		for i, tt := range cases {
			if mips32Insn {
				tt.rs = randomizeUpperWord(signExtend64(tt.rs))
				tt.rt = randomizeUpperWord(signExtend64(tt.rt))
				tt.expectHi = signExtend64(tt.expectHi)
				tt.expectLo = signExtend64(tt.expectLo)
				tt.expectRes = signExtend64(tt.expectRes)
			}

			testName := fmt.Sprintf("%v (%v)", tt.name, v.Name)
			t.Run(testName, func(t *testing.T) {
				goVm := v.VMFactory(nil, os.Stdout, os.Stderr, testutil.CreateLogger(), testutil.WithRandomization(int64(i)), testutil.WithPC(0), testutil.WithNextPC(4))
				state := goVm.GetState()
				var insn uint32
				baseReg := uint32(0x9)
				rtReg := uint32(0xa)

				insn = tt.opcode<<26 | baseReg<<21 | rtReg<<16 | tt.rdReg<<11 | tt.funct
				state.GetRegistersRef()[rtReg] = tt.rt
				state.GetRegistersRef()[baseReg] = tt.rs
				testutil.StoreInstruction(state.GetMemory(), 0, insn)

				if tt.panicMsg != "" {
					proofData := v.ProofGenerator(t, goVm.GetState())
					require.PanicsWithValue(t, tt.panicMsg, func() {
						_, _ = goVm.Step(
							false)
					})
					testutil.AssertEVMReverts(t, state, v.Contracts, nil, proofData, testutil.CreateErrorStringMatcher(tt.revertMsg))
					return
				}

				step := state.GetStep()
				// Setup expectations
				expected := testutil.NewExpectedState(state)
				expected.ExpectStep()
				if tt.expectRes != 0 {
					expected.Registers[tt.rdReg] = tt.expectRes
				} else {
					expected.HI = tt.expectHi
					expected.LO = tt.expectLo
				}

				stepWitness, err := goVm.Step(true)
				require.NoError(t, err)

				// Check expectations
				expected.Validate(t, state)
				testutil.ValidateEVM(t, stepWitness, step, goVm, v.StateHashFn, v.Contracts)
			})
		}
	}
}

type loadStoreTestCase struct {
	name         string
	rt           Word
	base         Word
	imm          uint32
	opcode       uint32
	memVal       Word
	expectMemVal Word
	expectRes    Word
}

func testLoadStore(t *testing.T, cases []loadStoreTestCase) {
	baseReg := uint32(9)
	rtReg := uint32(8)

	v := GetMultiThreadedTestCase(t)
	for i, tt := range cases {
		testName := fmt.Sprintf("%v %v", v.Name, tt.name)
		t.Run(testName, func(t *testing.T) {
			addr := tt.base + Word(tt.imm)
			effAddr := arch.AddressMask & addr

			goVm := v.VMFactory(nil, os.Stdout, os.Stderr, testutil.CreateLogger(), testutil.WithRandomization(int64(i)), testutil.WithPCAndNextPC(0))
			state := goVm.GetState()

			insn := tt.opcode<<26 | baseReg<<21 | rtReg<<16 | uint32(tt.imm)
			state.GetRegistersRef()[rtReg] = tt.rt
			state.GetRegistersRef()[baseReg] = tt.base

			testutil.StoreInstruction(state.GetMemory(), 0, insn)
			state.GetMemory().SetWord(effAddr, tt.memVal)
			step := state.GetStep()

			// Setup expectations
			expected := testutil.NewExpectedState(state)
			expected.ExpectStep()
			if tt.expectMemVal != 0 {
				expected.ExpectMemoryWriteWord(effAddr, tt.expectMemVal)
			} else {
				expected.Registers[rtReg] = tt.expectRes
			}
			stepWitness, err := goVm.Step(true)
			require.NoError(t, err)

			// Check expectations
			expected.Validate(t, state)
			testutil.ValidateEVM(t, stepWitness, step, goVm, v.StateHashFn, v.Contracts)
		})
	}
}

type branchTestCase struct {
	name         string
	pc           Word
	expectNextPC Word
	opcode       uint32
	regimm       uint32
	expectLink   bool
	rs           arch.SignedInteger
	offset       uint16
}

func testBranch(t *testing.T, cases []branchTestCase) {
	versions := GetMipsVersionTestCases(t)
	for _, v := range versions {
		for i, tt := range cases {
			testName := fmt.Sprintf("%v (%v)", tt.name, v.Name)
			t.Run(testName, func(t *testing.T) {
				goVm := v.VMFactory(nil, os.Stdout, os.Stderr, testutil.CreateLogger(), testutil.WithRandomization(int64(i)), testutil.WithPCAndNextPC(tt.pc))
				state := goVm.GetState()
				const rsReg = 8 // t0
				insn := tt.opcode<<26 | rsReg<<21 | tt.regimm<<16 | uint32(tt.offset)
				testutil.StoreInstruction(state.GetMemory(), tt.pc, insn)
				state.GetRegistersRef()[rsReg] = Word(tt.rs)
				step := state.GetStep()

				// Setup expectations
				expected := testutil.NewExpectedState(state)
				expected.Step += 1
				expected.PC = state.GetCpu().NextPC
				expected.NextPC = tt.expectNextPC
				if tt.expectLink {
					expected.Registers[31] = state.GetPC() + 8
				}

				stepWitness, err := goVm.Step(true)
				require.NoError(t, err)

				// Check expectations
				expected.Validate(t, state)
				testutil.ValidateEVM(t, stepWitness, step, goVm, v.StateHashFn, v.Contracts)
			})
		}
	}
}

type testMTStoreOpsClearMemReservationTestCase struct {
	// name is the test name
	name string
	// opcode is the instruction opcode
	opcode uint32
	// offset is the immediate offset encoded in the instruction
	offset uint32
	// base is the base/rs register prestate
	base Word
	// effAddr is the address used to set the prestate preMem value. It is also used as the base LLAddress that can be adjusted reservation assertions
	effAddr Word
	// premem is the prestate value of the word located at effrAddr
	preMem Word
	// postMem is the expected post-state value of the word located at effAddr
	postMem Word
}

func testMTStoreOpsClearMemReservation(t *testing.T, cases []testMTStoreOpsClearMemReservationTestCase) {
	llVariations := []struct {
		name                   string
		llReservationStatus    multithreaded.LLReservationStatus
		matchThreadId          bool
		effAddrOffset          Word
		shouldClearReservation bool
	}{
		{name: "matching reservation", llReservationStatus: multithreaded.LLStatusActive32bit, matchThreadId: true, shouldClearReservation: true},
		{name: "matching reservation, unaligned", llReservationStatus: multithreaded.LLStatusActive32bit, effAddrOffset: 1, matchThreadId: true, shouldClearReservation: true},
		{name: "matching reservation, 64-bit", llReservationStatus: multithreaded.LLStatusActive64bit, matchThreadId: true, shouldClearReservation: true},
		{name: "matching reservation, diff thread", llReservationStatus: multithreaded.LLStatusActive32bit, matchThreadId: false, shouldClearReservation: true},
		{name: "matching reservation, diff thread, 64-bit", llReservationStatus: multithreaded.LLStatusActive64bit, matchThreadId: false, shouldClearReservation: true},
		{name: "mismatched reservation", llReservationStatus: multithreaded.LLStatusActive32bit, matchThreadId: true, effAddrOffset: 8, shouldClearReservation: false},
		{name: "mismatched reservation, diff thread", llReservationStatus: multithreaded.LLStatusActive32bit, matchThreadId: false, effAddrOffset: 8, shouldClearReservation: false},
		{name: "no reservation, matching addr", llReservationStatus: multithreaded.LLStatusNone, matchThreadId: true, shouldClearReservation: true},
		{name: "no reservation, mismatched addr", llReservationStatus: multithreaded.LLStatusNone, matchThreadId: true, effAddrOffset: 8, shouldClearReservation: false},
	}

	rt := Word(0x12_34_56_78)
	//rt := Word(0x12_34_56_78_12_34_56_78)
	baseReg := uint32(5)
	rtReg := uint32(6)
	for i, c := range cases {
		for _, v := range llVariations {
			tName := fmt.Sprintf("%v (%v)", c.name, v.name)
			t.Run(tName, func(t *testing.T) {
				t.Parallel()
				insn := uint32((c.opcode << 26) | (baseReg & 0x1F << 21) | (rtReg & 0x1F << 16) | (0xFFFF & c.offset))
				goVm, state, contracts := setup(t, i, nil, testutil.WithPCAndNextPC(0x08))
				step := state.GetStep()

				// Define LL-related params
				llAddress := c.effAddr + v.effAddrOffset
				llOwnerThread := state.GetCurrentThread().ThreadId
				if !v.matchThreadId {
					llOwnerThread += 1
				}

				// Setup state
				state.GetRegistersRef()[rtReg] = rt
				state.GetRegistersRef()[baseReg] = c.base
				testutil.StoreInstruction(state.GetMemory(), state.GetPC(), insn)
				state.GetMemory().SetWord(c.effAddr, c.preMem)
				state.LLReservationStatus = v.llReservationStatus
				state.LLAddress = llAddress
				state.LLOwnerThread = llOwnerThread

				// Setup expectations
				expected := mttestutil.NewExpectedMTState(state)
				expected.ExpectStep()
				expected.ExpectMemoryWordWrite(c.effAddr, c.postMem)
				if v.shouldClearReservation {
					expected.LLReservationStatus = multithreaded.LLStatusNone
					expected.LLAddress = 0
					expected.LLOwnerThread = 0
				}

				stepWitness, err := goVm.Step(true)
				require.NoError(t, err)

				// Check expectations
				expected.Validate(t, state)
				testutil.ValidateEVM(t, stepWitness, step, goVm, multithreaded.GetStateHashFn(), contracts)
			})
		}
	}
}

type testMTSysReadPreimageTestCase struct {
	name           string
	addr           Word
	count          Word
	writeLen       Word
	preimageOffset Word
	prestateMem    Word
	postateMem     Word
	shouldPanic    bool
}

func testMTSysReadPreimage(t *testing.T, preimageValue []byte, cases []testMTSysReadPreimageTestCase) {
	llVariations := []struct {
		name                   string
		llReservationStatus    multithreaded.LLReservationStatus
		matchThreadId          bool
		effAddrOffset          Word
		shouldClearReservation bool
	}{
		{name: "matching reservation", llReservationStatus: multithreaded.LLStatusActive32bit, matchThreadId: true, shouldClearReservation: true},
		{name: "matching reservation, unaligned", llReservationStatus: multithreaded.LLStatusActive32bit, effAddrOffset: 1, matchThreadId: true, shouldClearReservation: true},
		{name: "matching reservation, diff thread", llReservationStatus: multithreaded.LLStatusActive32bit, matchThreadId: false, shouldClearReservation: true},
		{name: "mismatched reservation", llReservationStatus: multithreaded.LLStatusActive32bit, matchThreadId: true, effAddrOffset: 8, shouldClearReservation: false},
		{name: "mismatched reservation", llReservationStatus: multithreaded.LLStatusActive64bit, matchThreadId: false, effAddrOffset: 8, shouldClearReservation: false},
		{name: "no reservation, matching addr", llReservationStatus: multithreaded.LLStatusNone, matchThreadId: true, shouldClearReservation: true},
		{name: "no reservation, mismatched addr", llReservationStatus: multithreaded.LLStatusNone, matchThreadId: true, effAddrOffset: 8, shouldClearReservation: false},
	}

	for i, c := range cases {
		for _, v := range llVariations {
			tName := fmt.Sprintf("%v (%v)", c.name, v.name)
			t.Run(tName, func(t *testing.T) {
				t.Parallel()
				effAddr := arch.AddressMask & c.addr
				preimageKey := preimage.Keccak256Key(crypto.Keccak256Hash(preimageValue)).PreimageKey()
				oracle := testutil.StaticOracle(t, preimageValue)
				goVm, state, contracts := setup(t, i, oracle)
				step := state.GetStep()

				// Define LL-related params
				llAddress := effAddr + v.effAddrOffset
				llOwnerThread := state.GetCurrentThread().ThreadId
				if !v.matchThreadId {
					llOwnerThread += 1
				}

				// Set up state
				state.PreimageKey = preimageKey
				state.PreimageOffset = c.preimageOffset
				state.GetRegistersRef()[2] = arch.SysRead
				state.GetRegistersRef()[4] = exec.FdPreimageRead
				state.GetRegistersRef()[5] = c.addr
				state.GetRegistersRef()[6] = c.count
				testutil.StoreInstruction(state.GetMemory(), state.GetPC(), syscallInsn)
				state.LLReservationStatus = v.llReservationStatus
				state.LLAddress = llAddress
				state.LLOwnerThread = llOwnerThread
				state.GetMemory().SetWord(effAddr, c.prestateMem)

				// Setup expectations
				expected := mttestutil.NewExpectedMTState(state)
				expected.ExpectStep()
				expected.ActiveThread().Registers[2] = c.writeLen
				expected.ActiveThread().Registers[7] = 0 // no error
				expected.PreimageOffset += c.writeLen
				expected.ExpectMemoryWordWrite(effAddr, c.postateMem)
				if v.shouldClearReservation {
					expected.LLReservationStatus = multithreaded.LLStatusNone
					expected.LLAddress = 0
					expected.LLOwnerThread = 0
				}

				if c.shouldPanic {
					require.Panics(t, func() { _, _ = goVm.Step(true) })
					testutil.AssertPreimageOracleReverts(t, preimageKey, preimageValue, c.preimageOffset, contracts)
				} else {
					stepWitness, err := goVm.Step(true)
					require.NoError(t, err)

					// Check expectations
					expected.Validate(t, state)
					testutil.ValidateEVM(t, stepWitness, step, goVm, multithreaded.GetStateHashFn(), contracts)
				}
			})
		}
	}
}

func testNoopSyscall(t *testing.T, syscalls map[string]uint32) {
	for noopName, noopVal := range syscalls {
		t.Run(noopName, func(t *testing.T) {
			t.Parallel()
			goVm, state, contracts := setup(t, int(noopVal), nil)

			testutil.StoreInstruction(state.Memory, state.GetPC(), syscallInsn)
			state.GetRegistersRef()[2] = Word(noopVal) // Set syscall number
			step := state.Step

			// Set up post-state expectations
			expected := mttestutil.NewExpectedMTState(state)
			expected.ExpectStep()
			expected.ActiveThread().Registers[2] = 0
			expected.ActiveThread().Registers[7] = 0

			// State transition
			stepWitness, err := goVm.Step(true)
			require.NoError(t, err)

			// Validate post-state
			expected.Validate(t, state)
			testutil.ValidateEVM(t, stepWitness, step, goVm, multithreaded.GetStateHashFn(), contracts)
		})
	}
}

func testUnsupportedSyscall(t *testing.T, unsupportedSyscalls []uint32) {
	for i, syscallNum := range unsupportedSyscalls {
		testName := fmt.Sprintf("Unsupported syscallNum %v", syscallNum)
		i := i
		syscallNum := syscallNum
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			goVm, state, contracts := setup(t, i*3434, nil)
			// Setup basic getThreadId syscall instruction
			testutil.StoreInstruction(state.Memory, state.GetPC(), syscallInsn)
			state.GetRegistersRef()[2] = Word(syscallNum)
			proofData := multiThreadedProofGenerator(t, state)
			// Set up post-state expectations
			require.Panics(t, func() { _, _ = goVm.Step(true) })

			errorMessage := "unimplemented syscall"
			testutil.AssertEVMReverts(t, state, contracts, nil, proofData, testutil.CreateErrorStringMatcher(errorMessage))
		})
	}
}

// signExtend64 is used to sign-extend 32-bit words for 64-bit compatibility
func signExtend64(w Word) Word {
	if arch.IsMips32 {
		return w
	} else {
		return exec.SignExtend(w, 32)
	}
}

const seed = 0xdead

var rand = testutil.NewRandHelper(seed)

// randomizeUpperWord is used to assert that 32-bit operations use the lower word only
func randomizeUpperWord(w Word) Word {
	if arch.IsMips32 {
		return w
	} else {
		if w>>32 == 0x0 { // nolint:staticcheck
			rnd := rand.Uint32()
			upper := uint64(rnd) << 32
			return Word(upper | uint64(uint32(w)))
		} else {
			return w
		}
	}
}
