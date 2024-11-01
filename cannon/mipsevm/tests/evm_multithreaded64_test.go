//go:build cannon64
// +build cannon64

// These tests target architectures that are 64-bit or larger
package tests

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/multithreaded"
	mttestutil "github.com/ethereum-optimism/optimism/cannon/mipsevm/multithreaded/testutil"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/testutil"
)

func TestEVM_MT64_LL(t *testing.T) {
	memVal := Word(0x11223344_55667788)
	memValNeg := Word(0xF1223344_F5667788)
	cases := []struct {
		name   string
		base   Word
		offset int
		addr   Word
		memVal Word
		retReg int
		retVal Word
	}{
		{name: "8-byte-aligned addr", base: 0x01, offset: 0x0107, addr: 0x0108, memVal: memVal, retVal: 0x11223344, retReg: 5},
		{name: "8-byte-aligned addr, neg value", base: 0x01, offset: 0x0107, addr: 0x0108, memVal: memValNeg, retVal: 0xFFFFFFFF_F1223344, retReg: 5},
		{name: "8-byte-aligned addr, extra bits", base: 0x01, offset: 0x0109, addr: 0x010A, memVal: memVal, retVal: 0x11223344, retReg: 5},
		{name: "8-byte-aligned addr, addr signed extended", base: 0x01, offset: 0xFF37, addr: 0xFFFF_FFFF_FFFF_FF38, memVal: memVal, retVal: 0x11223344, retReg: 5},
		{name: "8-byte-aligned addr, addr signed extended w overflow", base: 0x1000_0001, offset: 0xFF07, addr: 0x0000_0000_0FFF_FF08, memVal: memVal, retVal: 0x11223344, retReg: 5},
		{name: "4-byte-aligned addr", base: 0x01, offset: 0x0103, addr: 0x0104, memVal: memVal, retVal: 0x55667788, retReg: 5},
		{name: "4-byte-aligned addr, neg value", base: 0x01, offset: 0x0104, addr: 0x0105, memVal: memValNeg, retVal: 0xFFFFFFFF_F5667788, retReg: 5},
		{name: "4-byte-aligned addr, extra bits", base: 0x01, offset: 0x0105, addr: 0x0106, memVal: memVal, retVal: 0x55667788, retReg: 5},
		{name: "4-byte-aligned addr, addr signed extended", base: 0x01, offset: 0xFF33, addr: 0xFFFF_FFFF_FFFF_FF34, memVal: memVal, retVal: 0x55667788, retReg: 5},
		{name: "4-byte-aligned addr, addr signed extended w overflow", base: 0x1000_0001, offset: 0xFF03, addr: 0x0000_0000_0FFF_FF04, memVal: memVal, retVal: 0x55667788, retReg: 5},
		{name: "Return register set to 0", base: 0x01, offset: 0x0107, addr: 0x0108, memVal: memVal, retVal: 0x11223344, retReg: 0},
	}
	for i, c := range cases {
		for _, withExistingReservation := range []bool{true, false} {
			tName := fmt.Sprintf("%v (withExistingReservation = %v)", c.name, withExistingReservation)
			t.Run(tName, func(t *testing.T) {
				effAddr := arch.AddressMask & c.addr

				retReg := c.retReg
				baseReg := 6
				insn := uint32((0b11_0000 << 26) | (baseReg & 0x1F << 21) | (retReg & 0x1F << 16) | (0xFFFF & c.offset))
				goVm, state, contracts := setup(t, i, nil, testutil.WithPCAndNextPC(0x40))
				step := state.GetStep()

				// Set up state
				testutil.StoreInstruction(state.GetMemory(), state.GetPC(), insn)
				state.GetMemory().SetWord(effAddr, c.memVal)
				state.GetRegistersRef()[baseReg] = c.base
				if withExistingReservation {
					state.LLReservationStatus = multithreaded.LLStatusActive32bit
					state.LLAddress = c.addr + 1
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
				expected.LLAddress = c.addr
				expected.LLOwnerThread = state.GetCurrentThread().ThreadId
				if retReg != 0 {
					expected.ActiveThread().Registers[retReg] = c.retVal
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

func TestEVM_MT64_SC(t *testing.T) {
	llVariations := []struct {
		name                string
		llReservationStatus multithreaded.LLReservationStatus
		matchThreadId       bool
		matchAddr           bool
		shouldSucceed       bool
	}{
		{name: "should succeed", llReservationStatus: multithreaded.LLStatusActive32bit, matchThreadId: true, matchAddr: true, shouldSucceed: true},
		{name: "mismatch addr", llReservationStatus: multithreaded.LLStatusActive32bit, matchThreadId: false, matchAddr: true, shouldSucceed: false},
		{name: "mismatched thread", llReservationStatus: multithreaded.LLStatusActive32bit, matchThreadId: true, matchAddr: false, shouldSucceed: false},
		{name: "mismatched addr & thread", llReservationStatus: multithreaded.LLStatusActive32bit, matchThreadId: false, matchAddr: false, shouldSucceed: false},
		{name: "mismatched reservation status", llReservationStatus: multithreaded.LLStatusActive64bit, matchThreadId: true, matchAddr: true, shouldSucceed: false},
		{name: "no active reservation", llReservationStatus: multithreaded.LLStatusNone, matchThreadId: true, matchAddr: true, shouldSucceed: false},
	}

	cases := []struct {
		name           string
		base           Word
		offset         int
		addr           Word
		value          Word
		expectedMemVal Word
		rtReg          int
		threadId       Word
	}{
		{name: "8-byte-aligned addr", base: 0x01, offset: 0x0137, addr: 0x0138, value: 0xABCD, expectedMemVal: 0xABCD_0000_0000, rtReg: 5, threadId: 4},
		{name: "8-byte-aligned addr, extra bits", base: 0x01, offset: 0x0138, addr: 0x0139, value: 0xABCD, expectedMemVal: 0xABCD_0000_0000, rtReg: 5, threadId: 4},
		{name: "8-byte-aligned addr, signed extended", base: 0x01, offset: 0xFF37, addr: 0xFFFF_FFFF_FFFF_FF38, value: 0xABCD, expectedMemVal: 0xABCD_0000_0000, rtReg: 5, threadId: 4},
		{name: "8-byte-aligned addr, signed extended w overflow", base: 0x1000_0001, offset: 0xFF37, addr: 0x0FFF_FF38, value: 0xABCD, expectedMemVal: 0xABCD_0000_0000, rtReg: 5, threadId: 4},
		{name: "4-byte-aligned addr", base: 0x01, offset: 0x0133, addr: 0x0134, value: 0xABCD, expectedMemVal: 0x_0000_0000_0000_ABCD, rtReg: 5, threadId: 4},
		{name: "4-byte-aligned addr, extra bits", base: 0x01, offset: 0x0134, addr: 0x0135, value: 0xABCD, expectedMemVal: 0x_0000_0000_0000_ABCD, rtReg: 5, threadId: 4},
		{name: "4-byte-aligned addr, signed extended", base: 0x01, offset: 0xFF33, addr: 0xFFFF_FFFF_FFFF_FF34, value: 0xABCD, expectedMemVal: 0x_0000_0000_0000_ABCD, rtReg: 5, threadId: 4},
		{name: "4-byte-aligned addr, signed extended w overflow", base: 0x1000_0001, offset: 0xFF33, addr: 0x0FFF_FF34, value: 0xABCD, expectedMemVal: 0x_0000_0000_0000_ABCD, rtReg: 5, threadId: 4},
		{name: "Return register set to 0", base: 0x01, offset: 0x0138, addr: 0x0139, value: 0xABCD, expectedMemVal: 0xABCD_0000_0000, rtReg: 0, threadId: 4},
		{name: "Zero valued ll args", base: 0x0, offset: 0x0, value: 0xABCD, expectedMemVal: 0xABCD_0000_0000, rtReg: 5, threadId: 0},
	}
	for i, c := range cases {
		for _, v := range llVariations {
			tName := fmt.Sprintf("%v (%v)", c.name, v.name)
			t.Run(tName, func(t *testing.T) {
				effAddr := arch.AddressMask & c.addr

				// Setup
				rtReg := c.rtReg
				baseReg := 6
				insn := uint32((0b11_1000 << 26) | (baseReg & 0x1F << 21) | (rtReg & 0x1F << 16) | (0xFFFF & c.offset))
				goVm, state, contracts := setup(t, i, nil)
				mttestutil.InitializeSingleThread(i*23456, state, i%2 == 1, testutil.WithPCAndNextPC(0x40))
				step := state.GetStep()

				// Define LL-related params
				var llAddress, llOwnerThread Word
				if v.matchAddr {
					llAddress = c.addr
				} else {
					llAddress = c.addr + 1
				}
				if v.matchThreadId {
					llOwnerThread = c.threadId
				} else {
					llOwnerThread = c.threadId + 1
				}

				// Setup state
				state.GetCurrentThread().ThreadId = c.threadId
				testutil.StoreInstruction(state.GetMemory(), state.GetPC(), insn)
				state.GetRegistersRef()[baseReg] = c.base
				state.GetRegistersRef()[rtReg] = c.value
				state.LLReservationStatus = v.llReservationStatus
				state.LLAddress = llAddress
				state.LLOwnerThread = llOwnerThread

				// Setup expectations
				expected := mttestutil.NewExpectedMTState(state)
				expected.ExpectStep()
				var retVal Word
				if v.shouldSucceed {
					retVal = 1
					expected.ExpectMemoryWordWrite(effAddr, c.expectedMemVal)
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

func TestEVM_MT64_LLD(t *testing.T) {
	memVal := Word(0x11223344_55667788)
	memValNeg := Word(0xF1223344_F5667788)
	cases := []struct {
		name   string
		base   Word
		offset int
		addr   Word
		memVal Word
		retReg int
	}{
		{name: "Aligned addr", base: 0x01, offset: 0x0107, addr: 0x0108, memVal: memVal, retReg: 5},
		{name: "Aligned addr, neg value", base: 0x01, offset: 0x0107, addr: 0x0108, memVal: memValNeg, retReg: 5},
		{name: "Unaligned addr, offset=1", base: 0x01, offset: 0x0100, addr: 0x0101, memVal: memVal, retReg: 5},
		{name: "Unaligned addr, offset=2", base: 0x02, offset: 0x0100, addr: 0x0102, memVal: memVal, retReg: 5},
		{name: "Unaligned addr, offset=3", base: 0x03, offset: 0x0100, addr: 0x0103, memVal: memVal, retReg: 5},
		{name: "Unaligned addr, offset=4", base: 0x04, offset: 0x0100, addr: 0x0104, memVal: memVal, retReg: 5},
		{name: "Unaligned addr, offset=5", base: 0x05, offset: 0x0100, addr: 0x0105, memVal: memVal, retReg: 5},
		{name: "Unaligned addr, offset=6", base: 0x06, offset: 0x0100, addr: 0x0106, memVal: memVal, retReg: 5},
		{name: "Unaligned addr, offset=7", base: 0x07, offset: 0x0100, addr: 0x0107, memVal: memVal, retReg: 5},
		{name: "Aligned addr, signed extended", base: 0x01, offset: 0xFF37, addr: 0xFFFF_FFFF_FFFF_FF38, memVal: memVal, retReg: 5},
		{name: "Aligned addr, signed extended w overflow", base: 0x1000_0001, offset: 0xFF07, addr: 0x0000_0000_0FFF_FF08, memVal: memVal, retReg: 5},
		{name: "Return register set to 0", base: 0x01, offset: 0x0107, addr: 0x0108, memVal: memVal, retReg: 0},
	}
	for i, c := range cases {
		for _, withExistingReservation := range []bool{true, false} {
			tName := fmt.Sprintf("%v (withExistingReservation = %v)", c.name, withExistingReservation)
			t.Run(tName, func(t *testing.T) {
				effAddr := arch.AddressMask & c.addr

				retReg := c.retReg
				baseReg := 6
				insn := uint32((0b11_0100 << 26) | (baseReg & 0x1F << 21) | (retReg & 0x1F << 16) | (0xFFFF & c.offset))
				goVm, state, contracts := setup(t, i, nil, testutil.WithPCAndNextPC(0x40))
				step := state.GetStep()

				// Set up state
				testutil.StoreInstruction(state.GetMemory(), state.GetPC(), insn)
				state.GetMemory().SetWord(effAddr, c.memVal)
				state.GetRegistersRef()[baseReg] = c.base
				if withExistingReservation {
					state.LLReservationStatus = multithreaded.LLStatusActive64bit
					state.LLAddress = c.addr + 1
					state.LLOwnerThread = 123
				} else {
					state.LLReservationStatus = multithreaded.LLStatusNone
					state.LLAddress = 0
					state.LLOwnerThread = 0
				}

				// Set up expectations
				expected := mttestutil.NewExpectedMTState(state)
				expected.ExpectStep()
				expected.LLReservationStatus = multithreaded.LLStatusActive64bit
				expected.LLAddress = c.addr
				expected.LLOwnerThread = state.GetCurrentThread().ThreadId
				if retReg != 0 {
					expected.ActiveThread().Registers[retReg] = c.memVal
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

func TestEVM_MT64_SCD(t *testing.T) {
	value := Word(0x11223344_55667788)
	llVariations := []struct {
		name                string
		llReservationStatus multithreaded.LLReservationStatus
		matchThreadId       bool
		matchAddr           bool
		shouldSucceed       bool
	}{
		{name: "should succeed", llReservationStatus: multithreaded.LLStatusActive64bit, matchThreadId: true, matchAddr: true, shouldSucceed: true},
		{name: "mismatch addr", llReservationStatus: multithreaded.LLStatusActive64bit, matchThreadId: false, matchAddr: true, shouldSucceed: false},
		{name: "mismatched thread", llReservationStatus: multithreaded.LLStatusActive64bit, matchThreadId: true, matchAddr: false, shouldSucceed: false},
		{name: "mismatched addr & thread", llReservationStatus: multithreaded.LLStatusActive64bit, matchThreadId: false, matchAddr: false, shouldSucceed: false},
		{name: "mismatched status", llReservationStatus: multithreaded.LLStatusActive32bit, matchThreadId: true, matchAddr: true, shouldSucceed: false},
		{name: "no active reservation", llReservationStatus: multithreaded.LLStatusNone, matchThreadId: true, matchAddr: true, shouldSucceed: false},
	}

	cases := []struct {
		name     string
		base     Word
		offset   int
		addr     Word
		rtReg    int
		threadId Word
	}{
		{name: "Aligned addr", base: 0x01, offset: 0x0137, addr: 0x0138, rtReg: 5, threadId: 4},
		{name: "Unaligned addr, offset=1", base: 0x01, offset: 0x0100, addr: 0x0101, rtReg: 5, threadId: 4},
		{name: "Unaligned addr, offset=2", base: 0x02, offset: 0x0100, addr: 0x0102, rtReg: 5, threadId: 4},
		{name: "Unaligned addr, offset=3", base: 0x03, offset: 0x0100, addr: 0x0103, rtReg: 5, threadId: 4},
		{name: "Unaligned addr, offset=4", base: 0x04, offset: 0x0100, addr: 0x0104, rtReg: 5, threadId: 4},
		{name: "Unaligned addr, offset=5", base: 0x05, offset: 0x0100, addr: 0x0105, rtReg: 5, threadId: 4},
		{name: "Unaligned addr, offset=6", base: 0x06, offset: 0x0100, addr: 0x0106, rtReg: 5, threadId: 4},
		{name: "Unaligned addr, offset=7", base: 0x07, offset: 0x0100, addr: 0x0107, rtReg: 5, threadId: 4},
		{name: "Aligned addr, signed extended", base: 0x01, offset: 0xFF37, addr: 0xFFFF_FFFF_FFFF_FF38, rtReg: 5, threadId: 4},
		{name: "Aligned addr, signed extended w overflow", base: 0x1000_0001, offset: 0xFF37, addr: 0x0FFF_FF38, rtReg: 5, threadId: 4},
		{name: "Return register set to 0", base: 0x01, offset: 0x0138, addr: 0x0139, rtReg: 0, threadId: 4},
		{name: "Zero valued ll args", base: 0x0, offset: 0x0, rtReg: 5, threadId: 0},
	}
	for i, c := range cases {
		for _, v := range llVariations {
			tName := fmt.Sprintf("%v (%v)", c.name, v.name)
			t.Run(tName, func(t *testing.T) {
				effAddr := arch.AddressMask & c.addr

				// Setup
				rtReg := c.rtReg
				baseReg := 6
				insn := uint32((0b11_1100 << 26) | (baseReg & 0x1F << 21) | (rtReg & 0x1F << 16) | (0xFFFF & c.offset))
				goVm, state, contracts := setup(t, i, nil)
				mttestutil.InitializeSingleThread(i*23456, state, i%2 == 1, testutil.WithPCAndNextPC(0x40))
				step := state.GetStep()

				// Define LL-related params
				var llAddress, llOwnerThread Word
				if v.matchAddr {
					llAddress = c.addr
				} else {
					llAddress = c.addr + 1
				}
				if v.matchThreadId {
					llOwnerThread = c.threadId
				} else {
					llOwnerThread = c.threadId + 1
				}

				// Setup state
				state.GetCurrentThread().ThreadId = c.threadId
				testutil.StoreInstruction(state.GetMemory(), state.GetPC(), insn)
				state.GetRegistersRef()[baseReg] = c.base
				state.GetRegistersRef()[rtReg] = value
				state.LLReservationStatus = v.llReservationStatus
				state.LLAddress = llAddress
				state.LLOwnerThread = llOwnerThread

				// Setup expectations
				expected := mttestutil.NewExpectedMTState(state)
				expected.ExpectStep()
				var retVal Word
				if v.shouldSucceed {
					retVal = 1
					expected.ExpectMemoryWordWrite(effAddr, value)
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
