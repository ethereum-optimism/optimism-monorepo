//go:build cannon64
// +build cannon64

package tests

import (
	"os"
	"testing"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm/testutil"
	"github.com/stretchr/testify/require"
)

func FuzzStateConsistencyMulOp(f *testing.F) {
	f.Add(int64(0x80_00_00_00), int64(0x80_00_00_00), int64(1))
	f.Add(
		testutil.ToSignedInteger(uint64(0xFF_FF_FF_FF_11_22_33_44)),
		testutil.ToSignedInteger(uint64(0xFF_FF_FF_FF_11_22_33_44)),
		int64(1),
	)
	f.Add(
		testutil.ToSignedInteger(uint64(0xFF_FF_FF_FF_80_00_00_00)),
		testutil.ToSignedInteger(uint64(0xFF_FF_FF_FF_80_00_00_00)),
		int64(1),
	)
	f.Add(
		testutil.ToSignedInteger(uint64(0xFF_FF_FF_FF_FF_FF_FF_FF)),
		testutil.ToSignedInteger(uint64(0xFF_FF_FF_FF_FF_FF_FF_FF)),
		int64(1),
	)

	const opcode uint32 = 28
	const mulFunct uint32 = 0x2
	versions := GetMipsVersionTestCases(f)
	f.Fuzz(func(t *testing.T, rs int64, rt int64, seed int64) {
		for _, v := range versions {
			t.Run(v.Name, func(t *testing.T) {
				mulOpConsistencyCheck(t, versions, opcode, true, mulFunct, Word(rs), Word(rt), seed)
			})
		}
	})
}

func FuzzStateConsistencyMultOp(f *testing.F) {
	f.Add(int64(0x80_00_00_00), int64(0x80_00_00_00), int64(1))
	f.Add(
		testutil.ToSignedInteger(uint64(0xFF_FF_FF_FF_11_22_33_44)),
		testutil.ToSignedInteger(uint64(0xFF_FF_FF_FF_11_22_33_44)),
		int64(1),
	)
	f.Add(
		testutil.ToSignedInteger(uint64(0xFF_FF_FF_FF_80_00_00_00)),
		testutil.ToSignedInteger(uint64(0xFF_FF_FF_FF_80_00_00_00)),
		int64(1),
	)
	f.Add(
		testutil.ToSignedInteger(uint64(0xFF_FF_FF_FF_FF_FF_FF_FF)),
		testutil.ToSignedInteger(uint64(0xFF_FF_FF_FF_FF_FF_FF_FF)),
		int64(1),
	)

	const multFunct uint32 = 0x18
	versions := GetMipsVersionTestCases(f)
	f.Fuzz(func(t *testing.T, rs int64, rt int64, seed int64) {
		mulOpConsistencyCheck(t, versions, 0, false, multFunct, Word(rs), Word(rt), seed)
	})
}

func FuzzStateConsistencyMultuOp(f *testing.F) {
	f.Add(uint64(0x80_00_00_00), uint64(0x80_00_00_00), int64(1))
	f.Add(
		uint64(0xFF_FF_FF_FF_11_22_33_44),
		uint64(0xFF_FF_FF_FF_11_22_33_44),
		int64(1),
	)
	f.Add(
		uint64(0xFF_FF_FF_FF_80_00_00_00),
		uint64(0xFF_FF_FF_FF_80_00_00_00),
		int64(1),
	)
	f.Add(
		uint64(0xFF_FF_FF_FF_FF_FF_FF_FF),
		uint64(0xFF_FF_FF_FF_FF_FF_FF_FF),
		int64(1),
	)

	const multuFunct uint32 = 0x19
	versions := GetMipsVersionTestCases(f)
	f.Fuzz(func(t *testing.T, rs uint64, rt uint64, seed int64) {
		mulOpConsistencyCheck(t, versions, 0, false, multuFunct, rs, rt, seed)
	})
}

func mulOpConsistencyCheck(
	t *testing.T, versions []VersionedVMTestCase,
	opcode uint32, expectRdReg bool, funct uint32,
	rs Word, rt Word, seed int64) {
	for _, v := range versions {
		t.Run(v.Name, func(t *testing.T) {
			rsReg := uint32(17)
			rtReg := uint32(18)
			rdReg := uint32(0)
			if expectRdReg {
				rdReg = 19
			}

			insn := opcode<<26 | rsReg<<21 | rtReg<<16 | rdReg<<11 | funct
			goVm := v.VMFactory(nil, os.Stdout, os.Stderr, testutil.CreateLogger(), testutil.WithRandomization(seed), testutil.WithPCAndNextPC(0))
			state := goVm.GetState()
			state.GetRegistersRef()[rsReg] = rs
			state.GetRegistersRef()[rtReg] = rt
			testutil.StoreInstruction(state.GetMemory(), 0, insn)
			step := state.GetStep()

			// mere sanity checks
			expected := testutil.NewExpectedState(state)
			expected.ExpectStep()

			stepWitness, err := goVm.Step(true)
			require.NoError(t, err)

			// use the post-state rdReg or LO and HI just so we can run sanity checks
			if expectRdReg {
				expected.Registers[rdReg] = state.GetRegistersRef()[rdReg]
			} else {
				expected.LO = state.GetCpu().LO
				expected.HI = state.GetCpu().HI
			}
			expected.Validate(t, state)

			testutil.ValidateEVM(t, stepWitness, step, goVm, v.StateHashFn, v.Contracts)
		})
	}
}
