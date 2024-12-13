//go:build !cannon64
// +build !cannon64

package tests

import (
	"encoding/binary"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/exec"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/memory"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/testutil"
	preimage "github.com/ethereum-optimism/optimism/op-preimage"
)

func TestEVM_LL(t *testing.T) {
	cases := []struct {
		name    string
		base    Word
		offset  int
		value   Word
		effAddr Word
		rtReg   int
	}{
		{name: "Aligned effAddr", base: 0x00_00_00_01, offset: 0x0133, value: 0xABCD, effAddr: 0x00_00_01_34, rtReg: 5},
		{name: "Aligned effAddr, signed extended", base: 0x00_00_00_01, offset: 0xFF33, value: 0xABCD, effAddr: 0xFF_FF_FF_34, rtReg: 5},
		{name: "Unaligned effAddr", base: 0xFF_12_00_01, offset: 0x3401, value: 0xABCD, effAddr: 0xFF_12_34_00, rtReg: 5},
		{name: "Unaligned effAddr, sign extended w overflow", base: 0xFF_12_00_01, offset: 0x8401, value: 0xABCD, effAddr: 0xFF_11_84_00, rtReg: 5},
		{name: "Return register set to 0", base: 0x00_00_00_01, offset: 0x0133, value: 0xABCD, effAddr: 0x00_00_01_34, rtReg: 0},
	}
	v := GetSingleThreadedTestCase(t)
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rtReg := c.rtReg
			baseReg := 6
			pc := Word(0x44)
			insn := uint32((0b11_0000 << 26) | (baseReg & 0x1F << 21) | (rtReg & 0x1F << 16) | (0xFFFF & c.offset))
			goVm := v.VMFactory(nil, os.Stdout, os.Stderr, testutil.CreateLogger(), testutil.WithRandomization(int64(i)), testutil.WithPC(pc), testutil.WithNextPC(pc+4))
			state := goVm.GetState()
			testutil.StoreInstruction(state.GetMemory(), pc, insn)
			state.GetMemory().SetWord(c.effAddr, c.value)
			state.GetRegistersRef()[baseReg] = c.base
			step := state.GetStep()

			// Setup expectations
			expected := testutil.NewExpectedState(state)
			expected.Step += 1
			expected.PC = pc + 4
			expected.NextPC = pc + 8
			if rtReg != 0 {
				expected.Registers[rtReg] = c.value
			}

			stepWitness, err := goVm.Step(true)
			require.NoError(t, err)

			// Check expectations
			expected.Validate(t, state)
			testutil.ValidateEVM(t, stepWitness, step, goVm, v.StateHashFn, v.Contracts)
		})
	}
}

func TestEVM_SC(t *testing.T) {
	cases := []struct {
		name    string
		base    Word
		offset  int
		value   Word
		effAddr Word
		rtReg   int
	}{
		{name: "Aligned effAddr", base: 0x00_00_00_01, offset: 0x0133, value: 0xABCD, effAddr: 0x00_00_01_34, rtReg: 5},
		{name: "Aligned effAddr, signed extended", base: 0x00_00_00_01, offset: 0xFF33, value: 0xABCD, effAddr: 0xFF_FF_FF_34, rtReg: 5},
		{name: "Unaligned effAddr", base: 0xFF_12_00_01, offset: 0x3401, value: 0xABCD, effAddr: 0xFF_12_34_00, rtReg: 5},
		{name: "Unaligned effAddr, sign extended w overflow", base: 0xFF_12_00_01, offset: 0x8401, value: 0xABCD, effAddr: 0xFF_11_84_00, rtReg: 5},
		{name: "Return register set to 0", base: 0xFF_12_00_01, offset: 0x8401, value: 0xABCD, effAddr: 0xFF_11_84_00, rtReg: 0},
	}
	v := GetSingleThreadedTestCase(t)
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rtReg := c.rtReg
			baseReg := 6
			pc := Word(0x44)
			insn := uint32((0b11_1000 << 26) | (baseReg & 0x1F << 21) | (rtReg & 0x1F << 16) | (0xFFFF & c.offset))
			goVm := v.VMFactory(nil, os.Stdout, os.Stderr, testutil.CreateLogger(), testutil.WithRandomization(int64(i)), testutil.WithPC(pc), testutil.WithNextPC(pc+4))
			state := goVm.GetState()
			testutil.StoreInstruction(state.GetMemory(), pc, insn)
			state.GetRegistersRef()[baseReg] = c.base
			state.GetRegistersRef()[rtReg] = c.value
			step := state.GetStep()

			// Setup expectations
			expected := testutil.NewExpectedState(state)
			expected.Step += 1
			expected.PC = pc + 4
			expected.NextPC = pc + 8
			expectedMemory := memory.NewMemory()
			testutil.StoreInstruction(expectedMemory, pc, insn)
			expectedMemory.SetWord(c.effAddr, c.value)
			expected.MemoryRoot = expectedMemory.MerkleRoot()
			if rtReg != 0 {
				expected.Registers[rtReg] = 1 // 1 for success
			}

			stepWitness, err := goVm.Step(true)
			require.NoError(t, err)

			// Check expectations
			expected.Validate(t, state)
			testutil.ValidateEVM(t, stepWitness, step, goVm, v.StateHashFn, v.Contracts)
		})
	}
}

func TestEVM_SysRead_Preimage(t *testing.T) {
	preimageValue := make([]byte, 0, 8)
	preimageValue = binary.BigEndian.AppendUint32(preimageValue, 0x12_34_56_78)
	preimageValue = binary.BigEndian.AppendUint32(preimageValue, 0x98_76_54_32)

	v := GetSingleThreadedTestCase(t)

	cases := []struct {
		name           string
		addr           Word
		count          Word
		writeLen       Word
		preimageOffset Word
		prestateMem    Word
		postateMem     Word
		shouldPanic    bool
	}{
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
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			effAddr := arch.AddressMask & c.addr
			preimageKey := preimage.Keccak256Key(crypto.Keccak256Hash(preimageValue)).PreimageKey()
			oracle := testutil.StaticOracle(t, preimageValue)
			goVm := v.VMFactory(oracle, os.Stdout, os.Stderr, testutil.CreateLogger(), testutil.WithRandomization(int64(i)), testutil.WithPreimageKey(preimageKey), testutil.WithPreimageOffset(c.preimageOffset))
			state := goVm.GetState()
			step := state.GetStep()

			// Set up state
			state.GetRegistersRef()[2] = arch.SysRead
			state.GetRegistersRef()[4] = exec.FdPreimageRead
			state.GetRegistersRef()[5] = c.addr
			state.GetRegistersRef()[6] = c.count
			testutil.StoreInstruction(state.GetMemory(), state.GetPC(), syscallInsn)
			state.GetMemory().SetWord(effAddr, c.prestateMem)

			// Setup expectations
			expected := testutil.NewExpectedState(state)
			expected.ExpectStep()
			expected.Registers[2] = c.writeLen
			expected.Registers[7] = 0 // no error
			expected.PreimageOffset += c.writeLen
			expected.ExpectMemoryWriteWord(effAddr, c.postateMem)

			if c.shouldPanic {
				require.Panics(t, func() { _, _ = goVm.Step(true) })
				testutil.AssertPreimageOracleReverts(t, preimageKey, preimageValue, c.preimageOffset, v.Contracts)
			} else {
				stepWitness, err := goVm.Step(true)
				require.NoError(t, err)

				// Check expectations
				expected.Validate(t, state)
				testutil.ValidateEVM(t, stepWitness, step, goVm, v.StateHashFn, v.Contracts)
			}
		})
	}
}
