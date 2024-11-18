// These tests target architectures that are 64-bit or larger
package tests

import (
	"encoding/binary"
	"fmt"
	"os"
	"slices"
	"testing"

	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/exec"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/multithreaded"
	mttestutil "github.com/ethereum-optimism/optimism/cannon/mipsevm/multithreaded/testutil"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/testutil"
)

type Word = arch.Word

func TestEVM_MT_LL(t *testing.T) {
	// Set up some test values that will be reused
	posValue := uint64(0xAAAA_BBBB_1122_3344)
	posValueRet := uint64(0x1122_3344)
	negValue := uint64(0x1111_1111_8877_6655)
	negRetValue := uint64(0xFFFF_FFFF_8877_6655) // Sign extended version of negValue

	// Note: parameters are written as 64-bit values. For 32-bit architectures, these values are downcast to 32-bit
	cases := []struct {
		name         string
		base         uint64
		offset       int
		expectedAddr uint64
		memValue     uint64
		retVal       uint64
		rtReg        int
	}{
		{name: "Aligned addr", base: 0x01, offset: 0x0133, expectedAddr: 0x0134, memValue: posValue, retVal: posValueRet, rtReg: 5},
		{name: "Aligned addr, negative value", base: 0x01, offset: 0x0133, expectedAddr: 0x0134, memValue: negValue, retVal: negRetValue, rtReg: 5},
		{name: "Aligned addr, addr signed extended", base: 0x01, offset: 0xFF33, expectedAddr: 0xFFFF_FFFF_FFFF_FF34, memValue: posValue, retVal: posValueRet, rtReg: 5},
		{name: "Unaligned addr", base: 0xFF12_0001, offset: 0x3405, expectedAddr: 0xFF12_3406, memValue: posValue, retVal: posValueRet, rtReg: 5},
		{name: "Unaligned addr, addr sign extended w overflow", base: 0xFF12_0001, offset: 0x8405, expectedAddr: 0xFF11_8406, memValue: posValue, retVal: posValueRet, rtReg: 5},
		{name: "Return register set to 0", base: 0xFF12_0001, offset: 0x7404, expectedAddr: 0xFF12_7405, memValue: posValue, retVal: 0, rtReg: 0},
	}
	for i, c := range cases {
		for _, withExistingReservation := range []bool{true, false} {
			tName := fmt.Sprintf("%v (withExistingReservation = %v)", c.name, withExistingReservation)
			t.Run(tName, func(t *testing.T) {
				rtReg := c.rtReg
				baseReg := 6
				insn := uint32((0b11_0000 << 26) | (baseReg & 0x1F << 21) | (rtReg & 0x1F << 16) | (0xFFFF & c.offset))
				goVm, state, contracts := setup(t, i, nil, testutil.WithPCAndNextPC(0x40))
				step := state.GetStep()

				// Set up state
				testutil.SetMemoryUint64(t, state.GetMemory(), Word(c.expectedAddr), c.memValue)
				testutil.StoreInstruction(state.GetMemory(), state.GetPC(), insn)
				state.GetRegistersRef()[baseReg] = Word(c.base)
				if withExistingReservation {
					state.LLReservationStatus = multithreaded.LLStatusActive32bit
					state.LLAddress = Word(c.expectedAddr + 1)
					state.LLOwnerThread = 123
				} else {
					state.LLReservationStatus = multithreaded.LLStatusNone
					state.LLAddress = 0
					state.LLOwnerThread = 0
				}

				// Set up expectations
				expected := mttestutil.NewExpectedMTState(state)
				expected.ExpectStep()
				expected.LLReservationStatus = multithreaded.LLStatusActive32bit
				expected.LLAddress = Word(c.expectedAddr)
				expected.LLOwnerThread = state.GetCurrentThread().ThreadId
				if rtReg != 0 {
					expected.ActiveThread().Registers[rtReg] = Word(c.retVal)
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

func TestEVM_MT_SC(t *testing.T) {
	// Set up some test values that will be reused
	memValue := uint64(0x1122_3344_5566_7788)

	llVariations := []struct {
		name                string
		llReservationStatus multithreaded.LLReservationStatus
		matchThreadId       bool
		matchAddr           bool
		shouldSucceed       bool
	}{
		{name: "should succeed", llReservationStatus: multithreaded.LLStatusActive32bit, matchThreadId: true, matchAddr: true, shouldSucceed: true},
		{name: "mismatch thread", llReservationStatus: multithreaded.LLStatusActive32bit, matchThreadId: false, matchAddr: true, shouldSucceed: false},
		{name: "mismatched addr", llReservationStatus: multithreaded.LLStatusActive32bit, matchThreadId: true, matchAddr: false, shouldSucceed: false},
		{name: "mismatched addr & thread", llReservationStatus: multithreaded.LLStatusActive32bit, matchThreadId: false, matchAddr: false, shouldSucceed: false},
		{name: "mismatched status", llReservationStatus: multithreaded.LLStatusActive64bit, matchThreadId: true, matchAddr: true, shouldSucceed: false},
		{name: "no active reservation", llReservationStatus: multithreaded.LLStatusNone, matchThreadId: true, matchAddr: true, shouldSucceed: false},
	}

	// Note: Some parameters are written as 64-bit values. For 32-bit architectures, these values are downcast to 32-bit
	cases := []struct {
		name         string
		base         Word
		offset       int
		expectedAddr uint64
		storeValue   uint32
		rtReg        int
		threadId     Word
	}{
		{name: "Aligned addr", base: 0x01, offset: 0x0133, expectedAddr: 0x0134, storeValue: 0xAABB_CCDD, rtReg: 5, threadId: 4},
		{name: "Aligned addr, signed extended", base: 0x01, offset: 0xFF33, expectedAddr: 0xFFFF_FFFF_FFFF_FF34, storeValue: 0xAABB_CCDD, rtReg: 5, threadId: 4},
		{name: "Unaligned addr", base: 0xFF12_0001, offset: 0x3404, expectedAddr: 0xFF12_3405, storeValue: 0xAABB_CCDD, rtReg: 5, threadId: 4},
		{name: "Unaligned addr, sign extended w overflow", base: 0xFF12_0001, offset: 0x8404, expectedAddr: 0xFF_11_8405, storeValue: 0xAABB_CCDD, rtReg: 5, threadId: 4},
		{name: "Return register set to 0", base: 0xFF12_0001, offset: 0x7403, expectedAddr: 0xFF12_7404, storeValue: 0xAABB_CCDD, rtReg: 0, threadId: 4},
	}
	for i, c := range cases {
		for _, v := range llVariations {
			tName := fmt.Sprintf("%v (%v)", c.name, v.name)
			t.Run(tName, func(t *testing.T) {
				rtReg := c.rtReg
				baseReg := 6
				insn := uint32((0b11_1000 << 26) | (baseReg & 0x1F << 21) | (rtReg & 0x1F << 16) | (0xFFFF & c.offset))
				goVm, state, contracts := setup(t, i, nil)
				mttestutil.InitializeSingleThread(i*23456, state, i%2 == 1, testutil.WithPCAndNextPC(0x40))
				step := state.GetStep()

				// Define LL-related params
				var llAddress, llOwnerThread Word
				if v.matchAddr {
					llAddress = Word(c.expectedAddr)
				} else {
					llAddress = Word(c.expectedAddr) + 1
				}
				if v.matchThreadId {
					llOwnerThread = c.threadId
				} else {
					llOwnerThread = c.threadId + 1
				}

				// Setup state
				testutil.SetMemoryUint64(t, state.GetMemory(), Word(c.expectedAddr), memValue)
				state.GetCurrentThread().ThreadId = c.threadId
				testutil.StoreInstruction(state.GetMemory(), state.GetPC(), insn)
				state.GetRegistersRef()[baseReg] = c.base
				state.GetRegistersRef()[rtReg] = Word(c.storeValue)
				state.LLReservationStatus = v.llReservationStatus
				state.LLAddress = llAddress
				state.LLOwnerThread = llOwnerThread

				// Setup expectations
				expected := mttestutil.NewExpectedMTState(state)
				expected.ExpectStep()
				var retVal Word
				if v.shouldSucceed {
					retVal = 1
					expected.ExpectMemoryWriteUint32(t, Word(c.expectedAddr), c.storeValue)
					expected.LLReservationStatus = multithreaded.LLStatusNone
					expected.LLAddress = 0
					expected.LLOwnerThread = 0
				} else {
					retVal = 0
				}
				if rtReg != 0 {
					expected.ActiveThread().Registers[rtReg] = retVal
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

func TestEVM_MT_SysRead_Preimage32(t *testing.T) {
	testutil.Cannon32OnlyTest(t, "These tests are fully covered for 64-bits in TestEVM_MT_SysRead_Preimage64")

	t.Parallel()
	preimageValue := make([]byte, 0, 8)
	preimageValue = binary.BigEndian.AppendUint32(preimageValue, 0x12_34_56_78)
	preimageValue = binary.BigEndian.AppendUint32(preimageValue, 0x98_76_54_32)
	cases := []testMTSysReadPreimageTestCase{
		{name: "Aligned addr, write 1 byte", addr: 0x00_00_FF_00, count: 1, writeLen: 1, preimageOffset: 8, prestateMem: 0xFF_FF_FF_FF, postateMem: 0x12_FF_FF_FF},
		{name: "Aligned addr, write 2 byte", addr: 0x00_00_FF_00, count: 2, writeLen: 2, preimageOffset: 8, prestateMem: 0xFF_FF_FF_FF, postateMem: 0x12_34_FF_FF},
		{name: "Aligned addr, write 3 byte", addr: 0x00_00_FF_00, count: 3, writeLen: 3, preimageOffset: 8, prestateMem: 0xFF_FF_FF_FF, postateMem: 0x12_34_56_FF},
		{name: "Aligned addr, write 4 byte", addr: 0x00_00_FF_00, count: 4, writeLen: 4, preimageOffset: 8, prestateMem: 0xFF_FF_FF_FF, postateMem: 0x12_34_56_78},
		{name: "1-byte misaligned addr, write 1 byte", addr: 0x00_00_FF_01, count: 1, writeLen: 1, preimageOffset: 8, prestateMem: 0xFF_FF_FF_FF, postateMem: 0xFF_12_FF_FF},
		{name: "1-byte misaligned addr, write 2 byte", addr: 0x00_00_FF_01, count: 2, writeLen: 2, preimageOffset: 9, prestateMem: 0xFF_FF_FF_FF, postateMem: 0xFF_34_56_FF},
		{name: "1-byte misaligned addr, write 3 byte", addr: 0x00_00_FF_01, count: 3, writeLen: 3, preimageOffset: 8, prestateMem: 0xFF_FF_FF_FF, postateMem: 0xFF_12_34_56},
		{name: "2-byte misaligned addr, write 1 byte", addr: 0x00_00_FF_02, count: 1, writeLen: 1, preimageOffset: 8, prestateMem: 0xFF_FF_FF_FF, postateMem: 0xFF_FF_12_FF},
		{name: "2-byte misaligned addr, write 2 byte", addr: 0x00_00_FF_02, count: 2, writeLen: 2, preimageOffset: 12, prestateMem: 0xFF_FF_FF_FF, postateMem: 0xFF_FF_98_76},
		{name: "3-byte misaligned addr, write 1 byte", addr: 0x00_00_FF_03, count: 1, writeLen: 1, preimageOffset: 8, prestateMem: 0xFF_FF_FF_FF, postateMem: 0xFF_FF_FF_12},
		{name: "Count of 0", addr: 0x00_00_FF_03, count: 0, writeLen: 0, preimageOffset: 8, prestateMem: 0xFF_FF_FF_FF, postateMem: 0xFF_FF_FF_FF},
		{name: "Count greater than 4", addr: 0x00_00_FF_00, count: 15, writeLen: 4, preimageOffset: 8, prestateMem: 0xFF_FF_FF_FF, postateMem: 0x12_34_56_78},
		{name: "Count greater than 4, unaligned", addr: 0x00_00_FF_01, count: 15, writeLen: 3, preimageOffset: 8, prestateMem: 0xFF_FF_FF_FF, postateMem: 0xFF_12_34_56},
		{name: "Offset at last byte", addr: 0x00_00_FF_00, count: 4, writeLen: 1, preimageOffset: 15, prestateMem: 0xFF_FF_FF_FF, postateMem: 0x32_FF_FF_FF},
		{name: "Offset just out of bounds", addr: 0x00_00_FF_00, count: 4, writeLen: 0, preimageOffset: 16, prestateMem: 0xFF_FF_FF_FF, postateMem: 0xFF_FF_FF_FF, shouldPanic: true},
		{name: "Offset out of bounds", addr: 0x00_00_FF_00, count: 4, writeLen: 0, preimageOffset: 17, prestateMem: 0xFF_FF_FF_FF, postateMem: 0xFF_FF_FF_FF, shouldPanic: true},
	}

	testMTSysReadPreimage(t, preimageValue, cases)
}

func TestEVM_MT_StoreOpsClearMemReservation32(t *testing.T) {
	t.Parallel()
	testutil.Cannon32OnlyTest(t, "These tests are fully covered for 64-bits in TestEVM_MT_StoreOpsClearMemReservation64")

	cases := []testMTStoreOpsClearMemReservationTestCase{
		{name: "Store byte", opcode: 0b10_1000, base: 0xFF_00_00_04, offset: 0x08, effAddr: 0xFF_00_00_0C, preMem: 0xFF_FF_FF_FF, postMem: 0x78_FF_FF_FF},
		{name: "Store halfword", opcode: 0b10_1001, base: 0xFF_00_00_04, offset: 0x08, effAddr: 0xFF_00_00_0C, preMem: 0xFF_FF_FF_FF, postMem: 0x56_78_FF_FF},
		{name: "Store word left", opcode: 0b10_1010, base: 0xFF_00_00_04, offset: 0x08, effAddr: 0xFF_00_00_0C, preMem: 0xFF_FF_FF_FF, postMem: 0x12_34_56_78},
		{name: "Store word", opcode: 0b10_1011, base: 0xFF_00_00_04, offset: 0x08, effAddr: 0xFF_00_00_0C, preMem: 0xFF_FF_FF_FF, postMem: 0x12_34_56_78},
		{name: "Store word right", opcode: 0b10_1110, base: 0xFF_00_00_04, offset: 0x08, effAddr: 0xFF_00_00_0C, preMem: 0xFF_FF_FF_FF, postMem: 0x78_FF_FF_FF},
	}
	testMTStoreOpsClearMemReservation(t, cases)
}

func TestEVM_SysClone_FlagHandling(t *testing.T) {
	contracts := testutil.TestContractsSetup(t, testutil.MipsMultithreaded)

	cases := []struct {
		name  string
		flags Word
		valid bool
	}{
		{"the supported flags bitmask", exec.ValidCloneFlags, true},
		{"no flags", 0, false},
		{"all flags", ^Word(0), false},
		{"all unsupported flags", ^Word(exec.ValidCloneFlags), false},
		{"a few supported flags", exec.CloneFs | exec.CloneSysvsem, false},
		{"one supported flag", exec.CloneFs, false},
		{"mixed supported and unsupported flags", exec.CloneFs | exec.CloneParentSettid, false},
		{"a single unsupported flag", exec.CloneUntraced, false},
		{"multiple unsupported flags", exec.CloneUntraced | exec.CloneParentSettid, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			state := multithreaded.CreateEmptyState()
			testutil.StoreInstruction(state.Memory, state.GetPC(), syscallInsn)
			state.GetRegistersRef()[2] = arch.SysClone // Set syscall number
			state.GetRegistersRef()[4] = c.flags       // Set first argument
			curStep := state.Step

			var err error
			var stepWitness *mipsevm.StepWitness
			goVm := multithreaded.NewInstrumentedState(state, nil, os.Stdout, os.Stderr, nil, nil)
			if !c.valid {
				// The VM should exit
				stepWitness, err = goVm.Step(true)
				require.NoError(t, err)
				require.Equal(t, curStep+1, state.GetStep())
				require.Equal(t, true, goVm.GetState().GetExited())
				require.Equal(t, uint8(mipsevm.VMStatusPanic), goVm.GetState().GetExitCode())
				require.Equal(t, 1, state.ThreadCount())
			} else {
				stepWitness, err = goVm.Step(true)
				require.NoError(t, err)
				require.Equal(t, curStep+1, state.GetStep())
				require.Equal(t, false, goVm.GetState().GetExited())
				require.Equal(t, uint8(0), goVm.GetState().GetExitCode())
				require.Equal(t, 2, state.ThreadCount())
			}

			testutil.ValidateEVM(t, stepWitness, curStep, goVm, multithreaded.GetStateHashFn(), contracts)
		})
	}
}

func TestEVM_SysClone_Successful(t *testing.T) {
	cases := []struct {
		name          string
		traverseRight bool
	}{
		{"traverse left", false},
		{"traverse right", true},
	}

	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stackPtr := Word(100)

			goVm, state, contracts := setup(t, i, nil)
			mttestutil.InitializeSingleThread(i*333, state, c.traverseRight)
			testutil.StoreInstruction(state.Memory, state.GetPC(), syscallInsn)
			state.GetRegistersRef()[2] = arch.SysClone        // the syscall number
			state.GetRegistersRef()[4] = exec.ValidCloneFlags // a0 - first argument, clone flags
			state.GetRegistersRef()[5] = stackPtr             // a1 - the stack pointer
			step := state.GetStep()

			// Sanity-check assumptions
			require.Equal(t, Word(1), state.NextThreadId)

			// Setup expectations
			expected := mttestutil.NewExpectedMTState(state)
			expected.Step += 1
			expectedNewThread := expected.ExpectNewThread()
			expected.ActiveThreadId = expectedNewThread.ThreadId
			expected.StepsSinceLastContextSwitch = 0
			if c.traverseRight {
				expected.RightStackSize += 1
			} else {
				expected.LeftStackSize += 1
			}
			// Original thread expectations
			expected.PrestateActiveThread().PC = state.GetCpu().NextPC
			expected.PrestateActiveThread().NextPC = state.GetCpu().NextPC + 4
			expected.PrestateActiveThread().Registers[2] = 1
			expected.PrestateActiveThread().Registers[7] = 0
			// New thread expectations
			expectedNewThread.PC = state.GetCpu().NextPC
			expectedNewThread.NextPC = state.GetCpu().NextPC + 4
			expectedNewThread.ThreadId = 1
			expectedNewThread.Registers[2] = 0
			expectedNewThread.Registers[7] = 0
			expectedNewThread.Registers[29] = stackPtr

			var err error
			var stepWitness *mipsevm.StepWitness
			stepWitness, err = goVm.Step(true)
			require.NoError(t, err)

			expected.Validate(t, state)
			activeStack, inactiveStack := mttestutil.GetThreadStacks(state)
			require.Equal(t, 2, len(activeStack))
			require.Equal(t, 0, len(inactiveStack))
			testutil.ValidateEVM(t, stepWitness, step, goVm, multithreaded.GetStateHashFn(), contracts)
		})
	}
}

func TestEVM_SysGetTID(t *testing.T) {
	cases := []struct {
		name     string
		threadId Word
	}{
		{"zero", 0},
		{"non-zero", 11},
	}

	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			goVm, state, contracts := setup(t, i*789, nil)
			mttestutil.InitializeSingleThread(i*789, state, false)

			state.GetCurrentThread().ThreadId = c.threadId
			testutil.StoreInstruction(state.Memory, state.GetPC(), syscallInsn)
			state.GetRegistersRef()[2] = arch.SysGetTID // Set syscall number
			step := state.Step

			// Set up post-state expectations
			expected := mttestutil.NewExpectedMTState(state)
			expected.ExpectStep()
			expected.ActiveThread().Registers[2] = c.threadId
			expected.ActiveThread().Registers[7] = 0

			// State transition
			var err error
			var stepWitness *mipsevm.StepWitness
			stepWitness, err = goVm.Step(true)
			require.NoError(t, err)

			// Validate post-state
			expected.Validate(t, state)
			testutil.ValidateEVM(t, stepWitness, step, goVm, multithreaded.GetStateHashFn(), contracts)
		})
	}
}

func TestEVM_SysExit(t *testing.T) {
	cases := []struct {
		name               string
		threadCount        int
		shouldExitGlobally bool
	}{
		// If we exit the last thread, the whole process should exit
		{name: "one thread", threadCount: 1, shouldExitGlobally: true},
		{name: "two threads ", threadCount: 2},
		{name: "three threads ", threadCount: 3},
	}

	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			exitCode := uint8(3)

			goVm, state, contracts := setup(t, i*133, nil)
			mttestutil.SetupThreads(int64(i*1111), state, i%2 == 0, c.threadCount, 0)

			testutil.StoreInstruction(state.Memory, state.GetPC(), syscallInsn)
			state.GetRegistersRef()[2] = arch.SysExit   // Set syscall number
			state.GetRegistersRef()[4] = Word(exitCode) // The first argument (exit code)
			step := state.Step

			// Set up expectations
			expected := mttestutil.NewExpectedMTState(state)
			expected.Step += 1
			expected.StepsSinceLastContextSwitch += 1
			expected.ActiveThread().Exited = true
			expected.ActiveThread().ExitCode = exitCode
			if c.shouldExitGlobally {
				expected.Exited = true
				expected.ExitCode = exitCode
			}

			// State transition
			var err error
			var stepWitness *mipsevm.StepWitness
			stepWitness, err = goVm.Step(true)
			require.NoError(t, err)

			// Validate post-state
			expected.Validate(t, state)
			testutil.ValidateEVM(t, stepWitness, step, goVm, multithreaded.GetStateHashFn(), contracts)
		})
	}
}

func TestEVM_PopExitedThread(t *testing.T) {
	cases := []struct {
		name                         string
		traverseRight                bool
		activeStackThreadCount       int
		expectTraverseRightPostState bool
	}{
		{name: "traverse right", traverseRight: true, activeStackThreadCount: 2, expectTraverseRightPostState: true},
		{name: "traverse right, switch directions", traverseRight: true, activeStackThreadCount: 1, expectTraverseRightPostState: false},
		{name: "traverse left", traverseRight: false, activeStackThreadCount: 2, expectTraverseRightPostState: false},
		{name: "traverse left, switch directions", traverseRight: false, activeStackThreadCount: 1, expectTraverseRightPostState: true},
	}

	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			goVm, state, contracts := setup(t, i*133, nil)
			mttestutil.SetupThreads(int64(i*222), state, c.traverseRight, c.activeStackThreadCount, 1)
			step := state.Step

			// Setup thread to be dropped
			threadToPop := state.GetCurrentThread()
			threadToPop.Exited = true
			threadToPop.ExitCode = 1

			// Set up expectations
			expected := mttestutil.NewExpectedMTState(state)
			expected.Step += 1
			expected.ActiveThreadId = mttestutil.FindNextThreadExcluding(state, threadToPop.ThreadId).ThreadId
			expected.StepsSinceLastContextSwitch = 0
			expected.ThreadCount -= 1
			expected.TraverseRight = c.expectTraverseRightPostState
			expected.Thread(threadToPop.ThreadId).Dropped = true
			if c.traverseRight {
				expected.RightStackSize -= 1
			} else {
				expected.LeftStackSize -= 1
			}

			// State transition
			var err error
			var stepWitness *mipsevm.StepWitness
			stepWitness, err = goVm.Step(true)
			require.NoError(t, err)

			// Validate post-state
			expected.Validate(t, state)
			testutil.ValidateEVM(t, stepWitness, step, goVm, multithreaded.GetStateHashFn(), contracts)
		})
	}
}

func TestEVM_SysFutex_WaitPrivate(t *testing.T) {
	// Note: parameters are written as 64-bit values. For 32-bit architectures, these values are downcast to 32-bit
	cases := []struct {
		name             string
		addressParam     uint64
		effAddr          uint64
		targetValue      uint64
		actualValue      uint64
		timeout          uint64
		shouldFail       bool
		shouldSetTimeout bool
	}{
		{name: "successful wait, no timeout", addressParam: 0xFF_FF_FF_FF_FF_FF_12_38, effAddr: 0xFF_FF_FF_FF_FF_FF_12_38, targetValue: 0xFF_FF_FF_FF_FF_FF_FF_01, actualValue: 0xFF_FF_FF_FF_FF_FF_FF_01},
		{name: "successful wait, no timeout, unaligned addr", addressParam: 0xFF_FF_FF_FF_FF_FF_12_39, effAddr: 0xFF_FF_FF_FF_FF_FF_12_38, targetValue: 0xFF_FF_FF_FF_FF_FF_FF_01, actualValue: 0xFF_FF_FF_FF_FF_FF_FF_01},
		{name: "memory mismatch, no timeout", addressParam: 0xFF_FF_FF_FF_FF_FF_12_00, effAddr: 0xFF_FF_FF_FF_FF_FF_12_00, targetValue: 0xFF_FF_FF_FF_FF_FF_FF_01, actualValue: 0xFF_FF_FF_FF_FF_FF_FF_02, shouldFail: true},
		{name: "memory mismatch, no timeout, unaligned", addressParam: 0xFF_FF_FF_FF_FF_FF_12_03, effAddr: 0xFF_FF_FF_FF_FF_FF_12_00, targetValue: 0xFF_FF_FF_FF_FF_FF_FF_01, actualValue: 0xFF_FF_FF_FF_FF_FF_FF_02, shouldFail: true},
		{name: "successful wait w timeout", addressParam: 0xFF_FF_FF_FF_FF_FF_12_38, effAddr: 0xFF_FF_FF_FF_FF_FF_12_38, targetValue: 0xFF_FF_FF_FF_FF_FF_FF_01, actualValue: 0xFF_FF_FF_FF_FF_FF_FF_01, timeout: 1000000, shouldSetTimeout: true},
		{name: "successful wait w timeout, unaligned", addressParam: 0xFF_FF_FF_FF_FF_FF_12_32, effAddr: 0xFF_FF_FF_FF_FF_FF_12_30, targetValue: 0xFF_FF_FF_FF_FF_FF_FF_01, actualValue: 0xFF_FF_FF_FF_FF_FF_FF_01, timeout: 1000000, shouldSetTimeout: true},
		{name: "memory mismatch w timeout", addressParam: 0xFF_FF_FF_FF_FF_FF_12_00, effAddr: 0xFF_FF_FF_FF_FF_FF_12_00, targetValue: 0xFF_FF_FF_FF_FF_FF_FF_01, actualValue: 0xFF_FF_FF_FF_FF_FF_FF_02, timeout: 2000000, shouldFail: true},
		{name: "memory mismatch w timeout, unaligned", addressParam: 0xFF_FF_FF_FF_FF_FF_12_0F, effAddr: 0xFF_FF_FF_FF_FF_FF_12_10, targetValue: 0xFF_FF_FF_FF_FF_FF_FF_01, actualValue: 0xFF_FF_FF_FF_FF_FF_FF_02, timeout: 2000000, shouldFail: true},
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			goVm, state, contracts := setup(t, i*1234, nil)
			step := state.GetStep()

			testutil.StoreInstruction(state.Memory, state.GetPC(), syscallInsn)
			state.Memory.SetWord(Word(c.effAddr), Word(c.actualValue))
			state.GetRegistersRef()[2] = arch.SysFutex // Set syscall number
			state.GetRegistersRef()[4] = Word(c.addressParam)
			state.GetRegistersRef()[5] = exec.FutexWaitPrivate
			state.GetRegistersRef()[6] = Word(c.targetValue)
			state.GetRegistersRef()[7] = Word(c.timeout)

			// Setup expectations
			expected := mttestutil.NewExpectedMTState(state)
			expected.Step += 1
			expected.StepsSinceLastContextSwitch += 1
			if c.shouldFail {
				expected.ActiveThread().PC = state.GetCpu().NextPC
				expected.ActiveThread().NextPC = state.GetCpu().NextPC + 4
				expected.ActiveThread().Registers[2] = exec.SysErrorSignal
				expected.ActiveThread().Registers[7] = exec.MipsEAGAIN
			} else {
				// PC and return registers should not update on success, updates happen when wait completes
				expected.ActiveThread().FutexAddr = Word(c.effAddr)
				expected.ActiveThread().FutexVal = Word(c.targetValue)
				expected.ActiveThread().FutexTimeoutStep = exec.FutexNoTimeout
				if c.shouldSetTimeout {
					expected.ActiveThread().FutexTimeoutStep = step + exec.FutexTimeoutSteps + 1
				}
			}

			// State transition
			stepWitness, err := goVm.Step(true)
			require.NoError(t, err)

			// Validate post-state
			expected.Validate(t, state)
			testutil.ValidateEVM(t, stepWitness, step, goVm, multithreaded.GetStateHashFn(), contracts)
		})
	}
}

func TestEVM_SysFutex_WakePrivate(t *testing.T) {
	// Note: parameters are written as 64-bit values. For 32-bit architectures, these values are downcast to 32-bit
	cases := []struct {
		name                string
		addressParam        uint64
		effAddr             uint64
		activeThreadCount   int
		inactiveThreadCount int
		traverseRight       bool
		expectTraverseRight bool
	}{
		{name: "Traverse right", addressParam: 0xFF_FF_FF_FF_FF_FF_67_00, effAddr: 0xFF_FF_FF_FF_FF_FF_67_00, activeThreadCount: 2, inactiveThreadCount: 1, traverseRight: true},
		{name: "Traverse right, unaligned addr", addressParam: 0xFF_FF_FF_FF_FF_FF_67_89, effAddr: 0xFF_FF_FF_FF_FF_FF_67_88, activeThreadCount: 2, inactiveThreadCount: 1, traverseRight: true},
		{name: "Traverse right, no left threads", addressParam: 0xFF_FF_FF_FF_FF_FF_67_84, effAddr: 0xFF_FF_FF_FF_FF_FF_67_84, activeThreadCount: 2, inactiveThreadCount: 0, traverseRight: true},
		{name: "Traverse right, no left threads, unaligned addr", addressParam: 0xFF_FF_FF_FF_FF_FF_67_8E, effAddr: 0xFF_FF_FF_FF_FF_FF_67_8C, activeThreadCount: 2, inactiveThreadCount: 0, traverseRight: true},
		{name: "Traverse right, single thread", addressParam: 0xFF_FF_FF_FF_FF_FF_67_88, effAddr: 0xFF_FF_FF_FF_FF_FF_67_88, activeThreadCount: 1, inactiveThreadCount: 0, traverseRight: true},
		{name: "Traverse right, single thread, unaligned", addressParam: 0xFF_FF_FF_FF_FF_FF_67_89, effAddr: 0xFF_FF_FF_FF_FF_FF_67_88, activeThreadCount: 1, inactiveThreadCount: 0, traverseRight: true},
		{name: "Traverse left", addressParam: 0xFF_FF_FF_FF_FF_FF_67_88, effAddr: 0xFF_FF_FF_FF_FF_FF_67_88, activeThreadCount: 2, inactiveThreadCount: 1, traverseRight: false},
		{name: "Traverse left, unaliagned", addressParam: 0xFF_FF_FF_FF_FF_FF_67_89, effAddr: 0xFF_FF_FF_FF_FF_FF_67_88, activeThreadCount: 2, inactiveThreadCount: 1, traverseRight: false},
		{name: "Traverse left, switch directions", addressParam: 0xFF_FF_FF_FF_FF_FF_67_88, effAddr: 0xFF_FF_FF_FF_FF_FF_67_88, activeThreadCount: 1, inactiveThreadCount: 1, traverseRight: false, expectTraverseRight: true},
		{name: "Traverse left, switch directions, unaligned", addressParam: 0xFF_FF_FF_FF_FF_FF_67_89, effAddr: 0xFF_FF_FF_FF_FF_FF_67_88, activeThreadCount: 1, inactiveThreadCount: 1, traverseRight: false, expectTraverseRight: true},
		{name: "Traverse left, single thread", addressParam: 0xFF_FF_FF_FF_FF_FF_67_88, effAddr: 0xFF_FF_FF_FF_FF_FF_67_88, activeThreadCount: 1, inactiveThreadCount: 0, traverseRight: false, expectTraverseRight: true},
		{name: "Traverse left, single thread, unaligned", addressParam: 0xFF_FF_FF_FF_FF_FF_67_89, effAddr: 0xFF_FF_FF_FF_FF_FF_67_88, activeThreadCount: 1, inactiveThreadCount: 0, traverseRight: false, expectTraverseRight: true},
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			goVm, state, contracts := setup(t, i*1122, nil)
			mttestutil.SetupThreads(int64(i*2244), state, c.traverseRight, c.activeThreadCount, c.inactiveThreadCount)
			step := state.Step

			testutil.StoreInstruction(state.Memory, state.GetPC(), syscallInsn)
			state.GetRegistersRef()[2] = arch.SysFutex // Set syscall number
			state.GetRegistersRef()[4] = Word(c.addressParam)
			state.GetRegistersRef()[5] = exec.FutexWakePrivate

			// Set up post-state expectations
			expected := mttestutil.NewExpectedMTState(state)
			expected.ExpectStep()
			expected.ActiveThread().Registers[2] = 0
			expected.ActiveThread().Registers[7] = 0
			expected.Wakeup = Word(c.effAddr) & arch.AddressMask // aligned for 32 and 64-bit compatibility
			expected.ExpectPreemption(state)
			expected.TraverseRight = c.expectTraverseRight
			if c.traverseRight != c.expectTraverseRight {
				// If we preempt the current thread and then switch directions, the same
				// thread will remain active
				expected.ActiveThreadId = state.GetCurrentThread().ThreadId
			}

			// State transition
			stepWitness, err := goVm.Step(true)
			require.NoError(t, err)

			// Validate post-state
			expected.Validate(t, state)
			testutil.ValidateEVM(t, stepWitness, step, goVm, multithreaded.GetStateHashFn(), contracts)
		})
	}

}

func TestEVM_SysFutex_UnsupportedOp(t *testing.T) {
	// From: https://github.com/torvalds/linux/blob/5be63fc19fcaa4c236b307420483578a56986a37/include/uapi/linux/futex.h
	const FUTEX_PRIVATE_FLAG = 128
	const FUTEX_WAIT = 0
	const FUTEX_WAKE = 1
	const FUTEX_FD = 2
	const FUTEX_REQUEUE = 3
	const FUTEX_CMP_REQUEUE = 4
	const FUTEX_WAKE_OP = 5
	const FUTEX_LOCK_PI = 6
	const FUTEX_UNLOCK_PI = 7
	const FUTEX_TRYLOCK_PI = 8
	const FUTEX_WAIT_BITSET = 9
	const FUTEX_WAKE_BITSET = 10
	const FUTEX_WAIT_REQUEUE_PI = 11
	const FUTEX_CMP_REQUEUE_PI = 12
	const FUTEX_LOCK_PI2 = 13

	unsupportedFutexOps := map[string]Word{
		"FUTEX_WAIT":                    FUTEX_WAIT,
		"FUTEX_WAKE":                    FUTEX_WAKE,
		"FUTEX_FD":                      FUTEX_FD,
		"FUTEX_REQUEUE":                 FUTEX_REQUEUE,
		"FUTEX_CMP_REQUEUE":             FUTEX_CMP_REQUEUE,
		"FUTEX_WAKE_OP":                 FUTEX_WAKE_OP,
		"FUTEX_LOCK_PI":                 FUTEX_LOCK_PI,
		"FUTEX_UNLOCK_PI":               FUTEX_UNLOCK_PI,
		"FUTEX_TRYLOCK_PI":              FUTEX_TRYLOCK_PI,
		"FUTEX_WAIT_BITSET":             FUTEX_WAIT_BITSET,
		"FUTEX_WAKE_BITSET":             FUTEX_WAKE_BITSET,
		"FUTEX_WAIT_REQUEUE_PI":         FUTEX_WAIT_REQUEUE_PI,
		"FUTEX_CMP_REQUEUE_PI":          FUTEX_CMP_REQUEUE_PI,
		"FUTEX_LOCK_PI2":                FUTEX_LOCK_PI2,
		"FUTEX_REQUEUE_PRIVATE":         (FUTEX_REQUEUE | FUTEX_PRIVATE_FLAG),
		"FUTEX_CMP_REQUEUE_PRIVATE":     (FUTEX_CMP_REQUEUE | FUTEX_PRIVATE_FLAG),
		"FUTEX_WAKE_OP_PRIVATE":         (FUTEX_WAKE_OP | FUTEX_PRIVATE_FLAG),
		"FUTEX_LOCK_PI_PRIVATE":         (FUTEX_LOCK_PI | FUTEX_PRIVATE_FLAG),
		"FUTEX_LOCK_PI2_PRIVATE":        (FUTEX_LOCK_PI2 | FUTEX_PRIVATE_FLAG),
		"FUTEX_UNLOCK_PI_PRIVATE":       (FUTEX_UNLOCK_PI | FUTEX_PRIVATE_FLAG),
		"FUTEX_TRYLOCK_PI_PRIVATE":      (FUTEX_TRYLOCK_PI | FUTEX_PRIVATE_FLAG),
		"FUTEX_WAIT_BITSET_PRIVATE":     (FUTEX_WAIT_BITSET | FUTEX_PRIVATE_FLAG),
		"FUTEX_WAKE_BITSET_PRIVATE":     (FUTEX_WAKE_BITSET | FUTEX_PRIVATE_FLAG),
		"FUTEX_WAIT_REQUEUE_PI_PRIVATE": (FUTEX_WAIT_REQUEUE_PI | FUTEX_PRIVATE_FLAG),
		"FUTEX_CMP_REQUEUE_PI_PRIVATE":  (FUTEX_CMP_REQUEUE_PI | FUTEX_PRIVATE_FLAG),
	}

	for name, op := range unsupportedFutexOps {
		t.Run(name, func(t *testing.T) {
			goVm, state, contracts := setup(t, int(op), nil)
			step := state.GetStep()

			testutil.StoreInstruction(state.Memory, state.GetPC(), syscallInsn)
			state.GetRegistersRef()[2] = arch.SysFutex // Set syscall number
			state.GetRegistersRef()[5] = op

			// Setup expectations
			expected := mttestutil.NewExpectedMTState(state)
			expected.Step += 1
			expected.StepsSinceLastContextSwitch += 1
			expected.ActiveThread().PC = state.GetCpu().NextPC
			expected.ActiveThread().NextPC = state.GetCpu().NextPC + 4
			expected.ActiveThread().Registers[2] = exec.SysErrorSignal
			expected.ActiveThread().Registers[7] = exec.MipsEINVAL

			// State transition
			var err error
			var stepWitness *mipsevm.StepWitness
			stepWitness, err = goVm.Step(true)
			require.NoError(t, err)

			// Validate post-state
			expected.Validate(t, state)
			testutil.ValidateEVM(t, stepWitness, step, goVm, multithreaded.GetStateHashFn(), contracts)
		})
	}
}

func TestEVM_SysYield(t *testing.T) {
	runPreemptSyscall(t, "SysSchedYield", arch.SysSchedYield)
}

func TestEVM_SysNanosleep(t *testing.T) {
	runPreemptSyscall(t, "SysNanosleep", arch.SysNanosleep)
}

func runPreemptSyscall(t *testing.T, syscallName string, syscallNum uint32) {
	cases := []struct {
		name            string
		traverseRight   bool
		activeThreads   int
		inactiveThreads int
	}{
		{name: "Last active thread", activeThreads: 1, inactiveThreads: 2},
		{name: "Only thread", activeThreads: 1, inactiveThreads: 0},
		{name: "Do not change directions", activeThreads: 2, inactiveThreads: 2},
		{name: "Do not change directions", activeThreads: 3, inactiveThreads: 0},
	}

	for i, c := range cases {
		for _, traverseRight := range []bool{true, false} {
			testName := fmt.Sprintf("%v: %v (traverseRight = %v)", syscallName, c.name, traverseRight)
			t.Run(testName, func(t *testing.T) {
				goVm, state, contracts := setup(t, i*789, nil)
				mttestutil.SetupThreads(int64(i*3259), state, traverseRight, c.activeThreads, c.inactiveThreads)

				testutil.StoreInstruction(state.Memory, state.GetPC(), syscallInsn)
				state.GetRegistersRef()[2] = Word(syscallNum) // Set syscall number
				step := state.Step

				// Set up post-state expectations
				expected := mttestutil.NewExpectedMTState(state)
				expected.ExpectStep()
				expected.ExpectPreemption(state)
				expected.PrestateActiveThread().Registers[2] = 0
				expected.PrestateActiveThread().Registers[7] = 0

				// State transition
				var err error
				var stepWitness *mipsevm.StepWitness
				stepWitness, err = goVm.Step(true)
				require.NoError(t, err)

				// Validate post-state
				expected.Validate(t, state)
				testutil.ValidateEVM(t, stepWitness, step, goVm, multithreaded.GetStateHashFn(), contracts)
			})
		}
	}
}

func TestEVM_SysOpen(t *testing.T) {
	goVm, state, contracts := setup(t, 5512, nil)

	testutil.StoreInstruction(state.Memory, state.GetPC(), syscallInsn)
	state.GetRegistersRef()[2] = arch.SysOpen // Set syscall number
	step := state.Step

	// Set up post-state expectations
	expected := mttestutil.NewExpectedMTState(state)
	expected.ExpectStep()
	expected.ActiveThread().Registers[2] = exec.SysErrorSignal
	expected.ActiveThread().Registers[7] = exec.MipsEBADF

	// State transition
	var err error
	var stepWitness *mipsevm.StepWitness
	stepWitness, err = goVm.Step(true)
	require.NoError(t, err)

	// Validate post-state
	expected.Validate(t, state)
	testutil.ValidateEVM(t, stepWitness, step, goVm, multithreaded.GetStateHashFn(), contracts)
}

func TestEVM_SysGetPID(t *testing.T) {
	goVm, state, contracts := setup(t, 1929, nil)

	testutil.StoreInstruction(state.Memory, state.GetPC(), syscallInsn)
	state.GetRegistersRef()[2] = arch.SysGetpid // Set syscall number
	step := state.Step

	// Set up post-state expectations
	expected := mttestutil.NewExpectedMTState(state)
	expected.ExpectStep()
	expected.ActiveThread().Registers[2] = 0
	expected.ActiveThread().Registers[7] = 0

	// State transition
	var err error
	var stepWitness *mipsevm.StepWitness
	stepWitness, err = goVm.Step(true)
	require.NoError(t, err)

	// Validate post-state
	expected.Validate(t, state)
	testutil.ValidateEVM(t, stepWitness, step, goVm, multithreaded.GetStateHashFn(), contracts)
}

func TestEVM_SysClockGettimeMonotonic(t *testing.T) {
	testEVM_SysClockGettime(t, exec.ClockGettimeMonotonicFlag)
}

func TestEVM_SysClockGettimeRealtime(t *testing.T) {
	testEVM_SysClockGettime(t, exec.ClockGettimeRealtimeFlag)
}

func testEVM_SysClockGettime(t *testing.T, clkid Word) {
	llVariations := []struct {
		name                   string
		llReservationStatus    multithreaded.LLReservationStatus
		matchThreadId          bool
		matchEffAddr           bool
		matchEffAddr2          bool
		shouldClearReservation bool
	}{
		{name: "matching reservation", llReservationStatus: multithreaded.LLStatusActive32bit, matchThreadId: true, matchEffAddr: true, shouldClearReservation: true},
		{name: "matching reservation, 64-bit", llReservationStatus: multithreaded.LLStatusActive64bit, matchThreadId: true, matchEffAddr: true, shouldClearReservation: true},
		{name: "matching reservation, 2nd word", llReservationStatus: multithreaded.LLStatusActive32bit, matchThreadId: true, matchEffAddr2: true, shouldClearReservation: true},
		{name: "matching reservation, 2nd word, 64-bit", llReservationStatus: multithreaded.LLStatusActive64bit, matchThreadId: true, matchEffAddr2: true, shouldClearReservation: true},
		{name: "matching reservation, diff thread", llReservationStatus: multithreaded.LLStatusActive32bit, matchThreadId: false, matchEffAddr: true, shouldClearReservation: true},
		{name: "matching reservation, diff thread, 2nd word", llReservationStatus: multithreaded.LLStatusActive32bit, matchThreadId: false, matchEffAddr2: true, shouldClearReservation: true},
		{name: "mismatched reservation", llReservationStatus: multithreaded.LLStatusActive32bit, matchThreadId: true, matchEffAddr: false, shouldClearReservation: false},
		{name: "mismatched reservation, diff thread", llReservationStatus: multithreaded.LLStatusActive32bit, matchThreadId: false, matchEffAddr: false, shouldClearReservation: false},
		{name: "no reservation, matching addr", llReservationStatus: multithreaded.LLStatusNone, matchThreadId: true, matchEffAddr: true, shouldClearReservation: true},
		{name: "no reservation, matching addr2", llReservationStatus: multithreaded.LLStatusNone, matchThreadId: true, matchEffAddr2: true, shouldClearReservation: true},
		{name: "no reservation, mismatched addr", llReservationStatus: multithreaded.LLStatusNone, matchThreadId: true, matchEffAddr: false, shouldClearReservation: false},
	}

	cases := []struct {
		name         string
		timespecAddr Word
	}{
		{"aligned timespec address", 0x1000},
		{"unaligned timespec address", 0x1003},
	}
	for i, c := range cases {
		for _, v := range llVariations {
			tName := fmt.Sprintf("%v (%v)", c.name, v.name)
			t.Run(tName, func(t *testing.T) {
				goVm, state, contracts := setup(t, 2101, nil)
				mttestutil.InitializeSingleThread(2101+i, state, i%2 == 1)
				effAddr := c.timespecAddr & arch.AddressMask
				effAddr2 := effAddr + arch.WordSizeBytes
				step := state.Step

				// Define LL-related params
				var llAddress, llOwnerThread Word
				if v.matchEffAddr {
					llAddress = effAddr
				} else if v.matchEffAddr2 {
					llAddress = effAddr2
				} else {
					llAddress = effAddr2 + 8
				}
				if v.matchThreadId {
					llOwnerThread = state.GetCurrentThread().ThreadId
				} else {
					llOwnerThread = state.GetCurrentThread().ThreadId + 1
				}

				testutil.StoreInstruction(state.Memory, state.GetPC(), syscallInsn)
				state.GetRegistersRef()[2] = arch.SysClockGetTime // Set syscall number
				state.GetRegistersRef()[4] = clkid                // a0
				state.GetRegistersRef()[5] = c.timespecAddr       // a1
				state.LLReservationStatus = v.llReservationStatus
				state.LLAddress = llAddress
				state.LLOwnerThread = llOwnerThread

				expected := mttestutil.NewExpectedMTState(state)
				expected.ExpectStep()
				expected.ActiveThread().Registers[2] = 0
				expected.ActiveThread().Registers[7] = 0
				next := state.Step + 1
				var secs, nsecs Word
				if clkid == exec.ClockGettimeMonotonicFlag {
					secs = Word(next / exec.HZ)
					nsecs = Word((next % exec.HZ) * (1_000_000_000 / exec.HZ))
				}
				expected.ExpectMemoryWordWrite(effAddr, secs)
				expected.ExpectMemoryWordWrite(effAddr2, nsecs)
				if v.shouldClearReservation {
					expected.LLReservationStatus = multithreaded.LLStatusNone
					expected.LLAddress = 0
					expected.LLOwnerThread = 0
				}

				var err error
				var stepWitness *mipsevm.StepWitness
				stepWitness, err = goVm.Step(true)
				require.NoError(t, err)

				// Validate post-state
				expected.Validate(t, state)
				testutil.ValidateEVM(t, stepWitness, step, goVm, multithreaded.GetStateHashFn(), contracts)
			})
		}
	}
}

func TestEVM_SysClockGettimeNonMonotonic(t *testing.T) {
	goVm, state, contracts := setup(t, 2101, nil)

	timespecAddr := Word(0x1000)
	testutil.StoreInstruction(state.Memory, state.GetPC(), syscallInsn)
	state.GetRegistersRef()[2] = arch.SysClockGetTime // Set syscall number
	state.GetRegistersRef()[4] = 0xDEAD               // a0 - invalid clockid
	state.GetRegistersRef()[5] = timespecAddr         // a1
	step := state.Step

	expected := mttestutil.NewExpectedMTState(state)
	expected.ExpectStep()
	expected.ActiveThread().Registers[2] = exec.SysErrorSignal
	expected.ActiveThread().Registers[7] = exec.MipsEINVAL

	var err error
	var stepWitness *mipsevm.StepWitness
	stepWitness, err = goVm.Step(true)
	require.NoError(t, err)

	// Validate post-state
	expected.Validate(t, state)
	testutil.ValidateEVM(t, stepWitness, step, goVm, multithreaded.GetStateHashFn(), contracts)
}

var NoopSyscalls = map[string]uint32{
	"SysGetAffinity":   4240,
	"SysMadvise":       4218,
	"SysRtSigprocmask": 4195,
	"SysSigaltstack":   4206,
	"SysRtSigaction":   4194,
	"SysPrlimit64":     4338,
	"SysClose":         4006,
	"SysPread64":       4200,
	"SysStat":          4106,
	"SysFstat":         4108,
	"SysFstat64":       4215,
	"SysOpenAt":        4288,
	"SysReadlink":      4085,
	"SysReadlinkAt":    4298,
	"SysIoctl":         4054,
	"SysEpollCreate1":  4326,
	"SysPipe2":         4328,
	"SysEpollCtl":      4249,
	"SysEpollPwait":    4313,
	"SysGetRandom":     4353,
	"SysUname":         4122,
	"SysStat64":        4213,
	"SysGetuid":        4024,
	"SysGetgid":        4047,
	"SysLlseek":        4140,
	"SysMinCore":       4217,
	"SysTgkill":        4266,
	"SysGetRLimit":     4076,
	"SysLseek":         4019,
	"SysMunmap":        4091,
	"SysSetITimer":     4104,
	"SysTimerCreate":   4257,
	"SysTimerSetTime":  4258,
	"SysTimerDelete":   4261,
}

func TestEVM_NoopSyscall32(t *testing.T) {
	testutil.Cannon32OnlyTest(t, "These tests are fully covered for 64-bits in TestEVM_NoopSyscall64")
	testNoopSyscall(t, NoopSyscalls)
}

func TestEVM_UnsupportedSyscall32(t *testing.T) {
	testutil.Cannon32OnlyTest(t, "These tests are fully covered for 64-bits in TestEVM_UnsupportedSyscall64")
	t.Parallel()

	var noopSyscallNums = maps.Values(NoopSyscalls)
	var supportedSyscalls = []uint32{arch.SysMmap, arch.SysBrk, arch.SysClone, arch.SysExitGroup, arch.SysRead, arch.SysWrite, arch.SysFcntl, arch.SysExit, arch.SysSchedYield, arch.SysGetTID, arch.SysFutex, arch.SysOpen, arch.SysNanosleep, arch.SysClockGetTime, arch.SysGetpid}
	unsupportedSyscalls := make([]uint32, 0, 400)
	for i := 4000; i < 4400; i++ {
		candidate := uint32(i)
		if slices.Contains(supportedSyscalls, candidate) || slices.Contains(noopSyscallNums, candidate) {
			continue
		}
		unsupportedSyscalls = append(unsupportedSyscalls, candidate)
	}

	testUnsupportedSyscall(t, unsupportedSyscalls)
}

func TestEVM_EmptyThreadStacks(t *testing.T) {
	t.Parallel()
	var tracer *tracing.Hooks

	cases := []struct {
		name           string
		otherStackSize int
		traverseRight  bool
	}{
		{name: "Traverse right with empty stacks", otherStackSize: 0, traverseRight: true},
		{name: "Traverse left with empty stacks", otherStackSize: 0, traverseRight: false},
		{name: "Traverse right with one non-empty stack on the other side", otherStackSize: 1, traverseRight: true},
		{name: "Traverse left with one non-empty stack on the other side", otherStackSize: 1, traverseRight: false},
	}
	// Generate proof variations
	proofVariations := GenerateEmptyThreadProofVariations(t)

	for i, c := range cases {
		for _, proofCase := range proofVariations {
			testName := fmt.Sprintf("%v (proofCase=%v)", c.name, proofCase.Name)
			t.Run(testName, func(t *testing.T) {
				goVm, state, contracts := setup(t, i*123, nil)
				mttestutil.SetupThreads(int64(i*123), state, c.traverseRight, 0, c.otherStackSize)

				require.PanicsWithValue(t, "Active thread stack is empty", func() { _, _ = goVm.Step(false) })

				errorMessage := "active thread stack is empty"
				testutil.AssertEVMReverts(t, state, contracts, tracer, proofCase.Proof, testutil.CreateErrorStringMatcher(errorMessage))
			})
		}
	}
}

func TestEVM_NormalTraversalStep_HandleWaitingThread(t *testing.T) {
	cases := []struct {
		name            string
		step            uint64
		activeStackSize int
		otherStackSize  int
		futexAddr       Word
		targetValue     Word
		actualValue     Word
		timeoutStep     uint64
		shouldWakeup    bool
		shouldTimeout   bool
	}{
		{name: "Preempt, no timeout #1", step: 100, activeStackSize: 1, otherStackSize: 0, futexAddr: 0x100, targetValue: 0x01, actualValue: 0x01, timeoutStep: exec.FutexNoTimeout},
		{name: "Preempt, no timeout #2", step: 100, activeStackSize: 1, otherStackSize: 1, futexAddr: 0x100, targetValue: 0x01, actualValue: 0x01, timeoutStep: exec.FutexNoTimeout},
		{name: "Preempt, no timeout #3", step: 100, activeStackSize: 2, otherStackSize: 1, futexAddr: 0x100, targetValue: 0x01, actualValue: 0x01, timeoutStep: exec.FutexNoTimeout},
		{name: "Preempt, no timeout, unaligned", step: 100, activeStackSize: 2, otherStackSize: 1, futexAddr: 0x101, targetValue: 0x01, actualValue: 0x01, timeoutStep: exec.FutexNoTimeout},
		{name: "Preempt, with timeout #1", step: 100, activeStackSize: 2, otherStackSize: 1, futexAddr: 0x100, targetValue: 0x01, actualValue: 0x01, timeoutStep: 101},
		{name: "Preempt, with timeout #2", step: 100, activeStackSize: 1, otherStackSize: 1, futexAddr: 0x100, targetValue: 0x01, actualValue: 0x01, timeoutStep: 150},
		{name: "Preempt, with timeout, unaligned", step: 100, activeStackSize: 1, otherStackSize: 1, futexAddr: 0x101, targetValue: 0x01, actualValue: 0x01, timeoutStep: 150},
		{name: "Wakeup, no timeout #1", step: 100, activeStackSize: 1, otherStackSize: 0, futexAddr: 0x100, targetValue: 0x01, actualValue: 0x02, timeoutStep: exec.FutexNoTimeout, shouldWakeup: true},
		{name: "Wakeup, no timeout #2", step: 100, activeStackSize: 2, otherStackSize: 1, futexAddr: 0x100, targetValue: 0x01, actualValue: 0x02, timeoutStep: exec.FutexNoTimeout, shouldWakeup: true},
		{name: "Wakeup, no timeout, unaligned", step: 100, activeStackSize: 2, otherStackSize: 1, futexAddr: 0x102, targetValue: 0x01, actualValue: 0x02, timeoutStep: exec.FutexNoTimeout, shouldWakeup: true},
		{name: "Wakeup with timeout #1", step: 100, activeStackSize: 2, otherStackSize: 1, futexAddr: 0x100, targetValue: 0x01, actualValue: 0x02, timeoutStep: 100, shouldWakeup: true, shouldTimeout: true},
		{name: "Wakeup with timeout #2", step: 100, activeStackSize: 2, otherStackSize: 1, futexAddr: 0x100, targetValue: 0x02, actualValue: 0x02, timeoutStep: 100, shouldWakeup: true, shouldTimeout: true},
		{name: "Wakeup with timeout #3", step: 100, activeStackSize: 2, otherStackSize: 1, futexAddr: 0x100, targetValue: 0x02, actualValue: 0x02, timeoutStep: 50, shouldWakeup: true, shouldTimeout: true},
		{name: "Wakeup with timeout, unaligned", step: 100, activeStackSize: 2, otherStackSize: 1, futexAddr: 0x103, targetValue: 0x02, actualValue: 0x02, timeoutStep: 50, shouldWakeup: true, shouldTimeout: true},
	}

	for _, c := range cases {
		for i, traverseRight := range []bool{true, false} {
			testName := fmt.Sprintf("%v (traverseRight=%v)", c.name, traverseRight)
			t.Run(testName, func(t *testing.T) {
				// Sanity check
				if !c.shouldWakeup && c.shouldTimeout {
					require.Fail(t, "Invalid test case - cannot expect a timeout with no wakeup")
				}
				effAddr := c.futexAddr & arch.AddressMask
				goVm, state, contracts := setup(t, i, nil)
				mttestutil.SetupThreads(int64(i*101), state, traverseRight, c.activeStackSize, c.otherStackSize)
				state.Step = c.step

				activeThread := state.GetCurrentThread()
				activeThread.FutexAddr = c.futexAddr
				activeThread.FutexVal = c.targetValue
				activeThread.FutexTimeoutStep = c.timeoutStep
				state.GetMemory().SetWord(effAddr, c.actualValue)

				// Set up post-state expectations
				expected := mttestutil.NewExpectedMTState(state)
				expected.Step += 1
				if c.shouldWakeup {
					expected.ActiveThread().FutexAddr = exec.FutexEmptyAddr
					expected.ActiveThread().FutexVal = 0
					expected.ActiveThread().FutexTimeoutStep = 0
					// PC and return registers are updated onWaitComplete
					expected.ActiveThread().PC = state.GetCpu().NextPC
					expected.ActiveThread().NextPC = state.GetCpu().NextPC + 4
					if c.shouldTimeout {
						expected.ActiveThread().Registers[2] = exec.SysErrorSignal
						expected.ActiveThread().Registers[7] = exec.MipsETIMEDOUT
					} else {
						expected.ActiveThread().Registers[2] = 0
						expected.ActiveThread().Registers[7] = 0
					}
				} else {
					expected.ExpectPreemption(state)
				}

				// State transition
				var err error
				var stepWitness *mipsevm.StepWitness
				stepWitness, err = goVm.Step(true)
				require.NoError(t, err)

				// Validate post-state
				expected.Validate(t, state)
				testutil.ValidateEVM(t, stepWitness, c.step, goVm, multithreaded.GetStateHashFn(), contracts)
			})

		}
	}
}

func TestEVM_NormalTraversal_Full(t *testing.T) {
	cases := []struct {
		name        string
		threadCount int
	}{
		{"1 thread", 1},
		{"2 threads", 2},
		{"3 threads", 3},
	}

	for i, c := range cases {
		for _, traverseRight := range []bool{true, false} {
			testName := fmt.Sprintf("%v (traverseRight = %v)", c.name, traverseRight)
			t.Run(testName, func(t *testing.T) {
				// Setup
				goVm, state, contracts := setup(t, i*789, nil)
				mttestutil.SetupThreads(int64(i*2947), state, traverseRight, c.threadCount, 0)
				// Put threads into a waiting state so that we just traverse through them
				for _, thread := range mttestutil.GetAllThreads(state) {
					thread.FutexAddr = 0x04
					thread.FutexTimeoutStep = exec.FutexNoTimeout
				}
				step := state.Step

				initialState := mttestutil.NewExpectedMTState(state)

				// Loop through all the threads to get back to the starting state
				iterations := c.threadCount * 2
				for i := 0; i < iterations; i++ {
					// Set up post-state expectations
					expected := mttestutil.NewExpectedMTState(state)
					expected.Step += 1
					expected.ExpectPreemption(state)

					// State transition
					var err error
					var stepWitness *mipsevm.StepWitness
					stepWitness, err = goVm.Step(true)
					require.NoError(t, err)

					// Validate post-state
					expected.Validate(t, state)
					testutil.ValidateEVM(t, stepWitness, step, goVm, multithreaded.GetStateHashFn(), contracts)
				}

				// We should be back to the original state with only a few modifications
				initialState.Step += uint64(iterations)
				initialState.StepsSinceLastContextSwitch = 0
				initialState.Validate(t, state)
			})
		}
	}
}

func TestEVM_WakeupTraversalStep(t *testing.T) {
	addr := Word(0x1234)
	wakeupVal := Word(0x999)
	cases := []struct {
		name              string
		wakeupAddr        Word
		futexAddr         Word
		targetVal         Word
		traverseRight     bool
		activeStackSize   int
		otherStackSize    int
		shouldClearWakeup bool
		shouldPreempt     bool
	}{
		{name: "Matching addr, not wakeable, first thread", wakeupAddr: addr, futexAddr: addr, targetVal: wakeupVal, traverseRight: false, activeStackSize: 3, otherStackSize: 0, shouldClearWakeup: true},
		{name: "Matching addr, wakeable, first thread", wakeupAddr: addr, futexAddr: addr, targetVal: wakeupVal + 1, traverseRight: false, activeStackSize: 3, otherStackSize: 0, shouldClearWakeup: true},
		{name: "Matching addr, not wakeable, last thread", wakeupAddr: addr, futexAddr: addr, targetVal: wakeupVal, traverseRight: true, activeStackSize: 1, otherStackSize: 2, shouldClearWakeup: true},
		{name: "Matching addr, wakeable, last thread", wakeupAddr: addr, futexAddr: addr, targetVal: wakeupVal + 1, traverseRight: true, activeStackSize: 1, otherStackSize: 2, shouldClearWakeup: true},
		{name: "Matching addr, not wakeable, intermediate thread", wakeupAddr: addr, futexAddr: addr, targetVal: wakeupVal, traverseRight: false, activeStackSize: 2, otherStackSize: 2, shouldClearWakeup: true},
		{name: "Matching addr, wakeable, intermediate thread", wakeupAddr: addr, futexAddr: addr, targetVal: wakeupVal + 1, traverseRight: true, activeStackSize: 2, otherStackSize: 2, shouldClearWakeup: true},
		{name: "Mismatched addr, last thread", wakeupAddr: addr, futexAddr: addr + 4, traverseRight: true, activeStackSize: 1, otherStackSize: 2, shouldPreempt: true, shouldClearWakeup: true},
		{name: "Mismatched addr", wakeupAddr: addr, futexAddr: addr + 4, traverseRight: true, activeStackSize: 2, otherStackSize: 2, shouldPreempt: true},
		{name: "Mismatched addr", wakeupAddr: addr, futexAddr: addr + 4, traverseRight: false, activeStackSize: 2, otherStackSize: 0, shouldPreempt: true},
		{name: "Mismatched addr", wakeupAddr: addr, futexAddr: addr + 4, traverseRight: false, activeStackSize: 1, otherStackSize: 0, shouldPreempt: true},
		{name: "Non-waiting thread", wakeupAddr: addr, futexAddr: exec.FutexEmptyAddr, traverseRight: false, activeStackSize: 1, otherStackSize: 0, shouldPreempt: true},
		{name: "Non-waiting thread", wakeupAddr: addr, futexAddr: exec.FutexEmptyAddr, traverseRight: true, activeStackSize: 2, otherStackSize: 1, shouldPreempt: true},
		{name: "Non-waiting thread, last thread", wakeupAddr: addr, futexAddr: exec.FutexEmptyAddr, traverseRight: true, activeStackSize: 1, otherStackSize: 1, shouldPreempt: true, shouldClearWakeup: true},
		// Check behavior of unaligned addresses - should be the same as aligned addresses (no memory access)
		{name: "Matching addr, unaligned", wakeupAddr: addr + 1, futexAddr: addr + 1, targetVal: wakeupVal, traverseRight: false, activeStackSize: 3, otherStackSize: 0, shouldClearWakeup: true},
		{name: "Mismatched addr, last thread, wakeup unaligned", wakeupAddr: addr + 1, futexAddr: addr + 4, traverseRight: true, activeStackSize: 1, otherStackSize: 2, shouldPreempt: true, shouldClearWakeup: true},
		{name: "Mismatched addr, last thread, futex unaligned", wakeupAddr: addr, futexAddr: addr + 5, traverseRight: true, activeStackSize: 1, otherStackSize: 2, shouldPreempt: true, shouldClearWakeup: true},
		{name: "Mismatched addr, last thread, wake & futex unaligned", wakeupAddr: addr + 1, futexAddr: addr + 5, traverseRight: true, activeStackSize: 1, otherStackSize: 2, shouldPreempt: true, shouldClearWakeup: true},
		{name: "Mismatched addr, wakeup unaligned", wakeupAddr: addr + 3, futexAddr: addr + 4, traverseRight: true, activeStackSize: 2, otherStackSize: 2, shouldPreempt: true},
		{name: "Mismatched addr, futex unaligned", wakeupAddr: addr, futexAddr: addr + 6, traverseRight: true, activeStackSize: 2, otherStackSize: 2, shouldPreempt: true},
		{name: "Mismatched addr, wakeup & futex unaligned", wakeupAddr: addr + 2, futexAddr: addr + 6, traverseRight: true, activeStackSize: 2, otherStackSize: 2, shouldPreempt: true},
		{name: "Non-waiting thread, last thread, unaligned wakeup", wakeupAddr: addr + 3, futexAddr: exec.FutexEmptyAddr, traverseRight: true, activeStackSize: 1, otherStackSize: 1, shouldPreempt: true, shouldClearWakeup: true},
	}

	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			goVm, state, contracts := setup(t, i*2000, nil)
			mttestutil.SetupThreads(int64(i*101), state, c.traverseRight, c.activeStackSize, c.otherStackSize)
			step := state.Step

			state.Wakeup = c.wakeupAddr
			state.GetMemory().SetWord(c.wakeupAddr&arch.AddressMask, wakeupVal)
			activeThread := state.GetCurrentThread()
			activeThread.FutexAddr = c.futexAddr
			activeThread.FutexVal = c.targetVal
			activeThread.FutexTimeoutStep = exec.FutexNoTimeout

			// Set up post-state expectations
			expected := mttestutil.NewExpectedMTState(state)
			expected.Step += 1
			if c.shouldClearWakeup {
				expected.Wakeup = exec.FutexEmptyAddr
			}
			if c.shouldPreempt {
				// Just preempt the current thread
				expected.ExpectPreemption(state)
			}

			// State transition
			var err error
			var stepWitness *mipsevm.StepWitness
			stepWitness, err = goVm.Step(true)
			require.NoError(t, err)

			// Validate post-state
			expected.Validate(t, state)
			testutil.ValidateEVM(t, stepWitness, step, goVm, multithreaded.GetStateHashFn(), contracts)
		})
	}
}

func TestEVM_WakeupTraversal_Full(t *testing.T) {
	cases := []struct {
		name        string
		threadCount int
	}{
		{"1 thread", 1},
		{"2 threads", 2},
		{"3 threads", 3},
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Setup
			goVm, state, contracts := setup(t, i*789, nil)
			mttestutil.SetupThreads(int64(i*2947), state, false, c.threadCount, 0)
			state.Wakeup = 0x08
			step := state.Step

			initialState := mttestutil.NewExpectedMTState(state)

			// Loop through all the threads to get back to the starting state
			iterations := c.threadCount * 2
			for i := 0; i < iterations; i++ {
				// Set up post-state expectations
				expected := mttestutil.NewExpectedMTState(state)
				expected.Step += 1
				expected.ExpectPreemption(state)

				// State transition
				var err error
				var stepWitness *mipsevm.StepWitness
				stepWitness, err = goVm.Step(true)
				require.NoError(t, err)

				// We should clear the wakeup on the last step
				if i == iterations-1 {
					expected.Wakeup = exec.FutexEmptyAddr
				}

				// Validate post-state
				expected.Validate(t, state)
				testutil.ValidateEVM(t, stepWitness, step, goVm, multithreaded.GetStateHashFn(), contracts)
			}

			// We should be back to the original state with only a few modifications
			initialState.Step += uint64(iterations)
			initialState.StepsSinceLastContextSwitch = 0
			initialState.Wakeup = exec.FutexEmptyAddr
			initialState.Validate(t, state)
		})
	}
}

func TestEVM_WakeupTraversal_WithExitedThreads(t *testing.T) {
	addr := Word(0x1234)
	wakeupVal := Word(0x999)
	cases := []struct {
		name                  string
		wakeupAddr            Word
		futexAddr             Word
		targetVal             Word
		traverseRight         bool
		activeStackSize       int
		otherStackSize        int
		exitedThreadIdx       []int
		shouldClearWakeup     bool
		shouldPreempt         bool
		activeThreadFutexAddr Word
		activeThreadFutexVal  Word
	}{
		{name: "Wakeable thread exists among exited threads", wakeupAddr: addr, futexAddr: addr, targetVal: wakeupVal + 1, traverseRight: false, activeStackSize: 3, otherStackSize: 1, exitedThreadIdx: []int{2}, shouldClearWakeup: false, shouldPreempt: true, activeThreadFutexAddr: addr + 8, activeThreadFutexVal: wakeupVal + 2},
		{name: "All threads exited", wakeupAddr: addr, futexAddr: addr, targetVal: wakeupVal, traverseRight: false, activeStackSize: 3, otherStackSize: 0, exitedThreadIdx: []int{1, 2}, shouldClearWakeup: false, shouldPreempt: true, activeThreadFutexAddr: addr + 16, activeThreadFutexVal: wakeupVal + 3},
		{name: "Exited threads, no matching futex", wakeupAddr: addr, futexAddr: addr + 4, targetVal: wakeupVal, traverseRight: false, activeStackSize: 2, otherStackSize: 1, exitedThreadIdx: []int{}, shouldClearWakeup: false, shouldPreempt: true, activeThreadFutexAddr: addr + 24, activeThreadFutexVal: wakeupVal + 4},
		{name: "Matching addr, not wakeable, with exited threads", wakeupAddr: addr, futexAddr: addr, targetVal: wakeupVal, traverseRight: true, activeStackSize: 3, otherStackSize: 0, exitedThreadIdx: []int{1}, shouldClearWakeup: false, shouldPreempt: true, activeThreadFutexAddr: addr + 32, activeThreadFutexVal: wakeupVal + 5},
		{name: "Non-waiting threads with exited threads", wakeupAddr: addr, futexAddr: exec.FutexEmptyAddr, targetVal: 0, traverseRight: false, activeStackSize: 2, otherStackSize: 1, exitedThreadIdx: []int{}, shouldClearWakeup: false, shouldPreempt: true, activeThreadFutexAddr: addr + 40, activeThreadFutexVal: wakeupVal + 6},
	}

	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			goVm, state, contracts := setup(t, i*1000, nil)
			mttestutil.SetupThreads(int64(i*5000), state, c.traverseRight, c.activeStackSize, c.otherStackSize)
			step := state.Step

			state.Wakeup = c.wakeupAddr
			state.GetMemory().SetWord(c.wakeupAddr&arch.AddressMask, wakeupVal)

			threads := mttestutil.GetAllThreads(state)
			for idx, thread := range threads {
				if slices.Contains(c.exitedThreadIdx, idx) {
					thread.Exited = true
				} else {
					thread.FutexAddr = c.futexAddr
					thread.FutexVal = c.targetVal
					thread.FutexTimeoutStep = exec.FutexNoTimeout
				}
			}

			activeThread := state.GetCurrentThread()
			activeThread.Exited = true

			activeThread.FutexAddr = c.activeThreadFutexAddr
			activeThread.FutexVal = c.activeThreadFutexVal
			activeThread.FutexTimeoutStep = exec.FutexNoTimeout

			expected := mttestutil.NewExpectedMTState(state)
			expected.Step += 1

			if c.shouldClearWakeup {
				expected.Wakeup = exec.FutexEmptyAddr
			}
			if c.shouldPreempt {
				// Just preempt the current thread
				expected.ExpectPreemption(state)
			}

			// State transition
			var err error
			var stepWitness *mipsevm.StepWitness
			stepWitness, err = goVm.Step(true)
			require.NoError(t, err)
			expected.Validate(t, state)
			testutil.ValidateEVM(t, stepWitness, step, goVm, multithreaded.GetStateHashFn(), contracts)
		})
	}
}

func TestEVM_SchedQuantumThreshold(t *testing.T) {
	cases := []struct {
		name                        string
		stepsSinceLastContextSwitch uint64
		shouldPreempt               bool
	}{
		{name: "just under threshold", stepsSinceLastContextSwitch: exec.SchedQuantum - 1},
		{name: "at threshold", stepsSinceLastContextSwitch: exec.SchedQuantum, shouldPreempt: true},
		{name: "beyond threshold", stepsSinceLastContextSwitch: exec.SchedQuantum + 1, shouldPreempt: true},
	}

	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			goVm, state, contracts := setup(t, i*789, nil)
			// Setup basic getThreadId syscall instruction
			testutil.StoreInstruction(state.Memory, state.GetPC(), syscallInsn)
			state.GetRegistersRef()[2] = arch.SysGetTID // Set syscall number
			state.StepsSinceLastContextSwitch = c.stepsSinceLastContextSwitch
			step := state.Step

			// Set up post-state expectations
			expected := mttestutil.NewExpectedMTState(state)
			if c.shouldPreempt {
				expected.Step += 1
				expected.ExpectPreemption(state)
			} else {
				// Otherwise just expect a normal step
				expected.ExpectStep()
				expected.ActiveThread().Registers[2] = state.GetCurrentThread().ThreadId
				expected.ActiveThread().Registers[7] = 0
			}

			// State transition
			var err error
			var stepWitness *mipsevm.StepWitness
			stepWitness, err = goVm.Step(true)
			require.NoError(t, err)

			// Validate post-state
			expected.Validate(t, state)
			testutil.ValidateEVM(t, stepWitness, step, goVm, multithreaded.GetStateHashFn(), contracts)
		})
	}
}

func setup(t require.TestingT, randomSeed int, preimageOracle mipsevm.PreimageOracle, opts ...testutil.StateOption) (mipsevm.FPVM, *multithreaded.State, *testutil.ContractMetadata) {
	v := GetMultiThreadedTestCase(t)
	allOpts := append([]testutil.StateOption{testutil.WithRandomization(int64(randomSeed))}, opts...)
	vm := v.VMFactory(preimageOracle, os.Stdout, os.Stderr, testutil.CreateLogger(), allOpts...)
	state := mttestutil.GetMtState(t, vm)

	return vm, state, v.Contracts

}
