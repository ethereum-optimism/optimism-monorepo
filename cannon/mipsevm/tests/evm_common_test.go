package tests

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/exec"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/memory"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/program"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/testutil"
)

func TestEVM_OpenMIPS(t *testing.T) {
	testutil.Cannon32OnlyTest(t, "Skipping MIPS32 assembly test vectors for cannon64")

	testFiles, err := os.ReadDir("open_mips_tests/test/bin")
	require.NoError(t, err)

	cases := GetMipsVersionTestCases(t)
	skippedTests := map[string][]string{
		"multi-threaded":  {"clone.bin"},
		"single-threaded": {},
	}

	for _, c := range cases {
		skipped, exists := skippedTests[c.Name]
		require.True(t, exists)
		for _, f := range testFiles {
			testName := fmt.Sprintf("%v (%v)", f.Name(), c.Name)
			t.Run(testName, func(t *testing.T) {
				for _, skipped := range skipped {
					if f.Name() == skipped {
						t.Skipf("Skipping explicitly excluded open_mips testcase: %v", f.Name())
					}
				}

				oracle := testutil.SelectOracleFixture(t, f.Name())
				// Short-circuit early for exit_group.bin
				exitGroup := f.Name() == "exit_group.bin"
				expectPanic := strings.HasSuffix(f.Name(), "panic.bin")
				validator := testutil.NewEvmValidator(t, c.StateHashFn, c.Contracts, testutil.WithLocalOracle(oracle))

				fn := path.Join("open_mips_tests/test/bin", f.Name())
				programMem, err := os.ReadFile(fn)
				require.NoError(t, err)

				goVm := c.VMFactory(oracle, os.Stdout, os.Stderr, testutil.CreateLogger())
				state := goVm.GetState()
				err = state.GetMemory().SetMemoryRange(0, bytes.NewReader(programMem))
				require.NoError(t, err, "load program into state")

				// set the return address ($ra) to jump into when test completes
				state.GetRegistersRef()[31] = testutil.EndAddr

				// Catch panics and check if they are expected
				defer func() {
					if r := recover(); r != nil {
						if expectPanic {
							// Success
						} else {
							t.Errorf("unexpected panic: %v", r)
						}
					}
				}()

				for i := 0; i < 1000; i++ {
					curStep := goVm.GetState().GetStep()
					if goVm.GetState().GetPC() == testutil.EndAddr {
						break
					}
					if exitGroup && goVm.GetState().GetExited() {
						break
					}
					insn := testutil.GetInstruction(state.GetMemory(), state.GetPC())
					t.Logf("step: %4d pc: 0x%08x insn: 0x%08x", state.GetStep(), state.GetPC(), insn)

					stepWitness, err := goVm.Step(true)
					require.NoError(t, err)
					validator.ValidateEVM(t, stepWitness, curStep, goVm)
				}
				if exitGroup {
					require.NotEqual(t, arch.Word(testutil.EndAddr), goVm.GetState().GetPC(), "must not reach end")
					require.True(t, goVm.GetState().GetExited(), "must set exited state")
					require.Equal(t, uint8(1), goVm.GetState().GetExitCode(), "must exit with 1")
				} else if expectPanic {
					require.NotEqual(t, arch.Word(testutil.EndAddr), state.GetPC(), "must not reach end")
				} else {
					require.Equal(t, arch.Word(testutil.EndAddr), state.GetPC(), "must reach end")
					// inspect test result
					done, result := state.GetMemory().GetWord(testutil.BaseAddrEnd+4), state.GetMemory().GetWord(testutil.BaseAddrEnd+8)
					require.Equal(t, done, Word(1), "must be done")
					require.Equal(t, result, Word(1), "must have success result")
				}
			})
		}
	}
}

func TestEVM_SingleStep_Jump(t *testing.T) {
	versions := GetMipsVersionTestCases(t)
	cases := []struct {
		name         string
		pc           arch.Word
		nextPC       arch.Word
		insn         uint32
		expectNextPC arch.Word
		expectLink   bool
	}{
		{name: "j MSB set target", pc: 0, nextPC: 4, insn: 0x0A_00_00_02, expectNextPC: 0x08_00_00_08},                                           // j 0x02_00_00_02
		{name: "j non-zero PC region", pc: 0x10000000, nextPC: 0x10000004, insn: 0x08_00_00_02, expectNextPC: 0x10_00_00_08},                     // j 0x2
		{name: "jal MSB set target", pc: 0, nextPC: 4, insn: 0x0E_00_00_02, expectNextPC: 0x08_00_00_08, expectLink: true},                       // jal 0x02_00_00_02
		{name: "jal non-zero PC region", pc: 0x10000000, nextPC: 0x10000004, insn: 0x0C_00_00_02, expectNextPC: 0x10_00_00_08, expectLink: true}, // jal 0x2
	}

	for _, v := range versions {
		for i, tt := range cases {
			testName := fmt.Sprintf("%v (%v)", tt.name, v.Name)
			t.Run(testName, func(t *testing.T) {
				goVm := v.VMFactory(nil, os.Stdout, os.Stderr, testutil.CreateLogger(), testutil.WithRandomization(int64(i)), testutil.WithPC(tt.pc), testutil.WithNextPC(tt.nextPC))
				state := goVm.GetState()
				testutil.StoreInstruction(state.GetMemory(), tt.pc, tt.insn)
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

func TestEVM_SingleStep_Operators(t *testing.T) {
	cases := []operatorTestCase{
		{name: "add", funct: 0x20, isImm: false, rs: Word(12), rt: Word(20), expectRes: Word(32)},                                  // add t0, s1, s2
		{name: "add", funct: 0x20, isImm: false, rs: ^Word(0), rt: ^Word(0), expectRes: Word(0xFF_FF_FF_FE)},                       // add t0, s1, s2
		{name: "add", funct: 0x20, isImm: false, rs: Word(0x7F_FF_FF_FF), rt: Word(0x7F_FF_FF_FF), expectRes: Word(0xFF_FF_FF_FE)}, // add t0, s1, s2
		{name: "add", funct: 0x20, isImm: false, rs: ^Word(0), rt: Word(2), expectRes: Word(1)},                                    // add t0, s1, s2
		{name: "add", funct: 0x20, isImm: false, rs: Word(2), rt: ^Word(0), expectRes: Word(1)},                                    // add t0, s1, s2
		{name: "add", funct: 0x20, isImm: false, rs: Word(0x7F_FF_FF_FF), rt: Word(1), expectRes: Word(0x80_00_00_00)},             // add t0, s1, s2

		{name: "addu", funct: 0x21, isImm: false, rs: Word(12), rt: Word(20), expectRes: Word(32)},                                  // addu t0, s1, s2
		{name: "addu", funct: 0x21, isImm: false, rs: ^Word(0), rt: ^Word(0), expectRes: Word(0xFF_FF_FF_FE)},                       // addu t0, s1, s2
		{name: "addu", funct: 0x21, isImm: false, rs: Word(0x7F_FF_FF_FF), rt: Word(0x7F_FF_FF_FF), expectRes: Word(0xFF_FF_FF_FE)}, // addu t0, s1, s2
		{name: "addu", funct: 0x21, isImm: false, rs: ^Word(0), rt: Word(2), expectRes: Word(1)},                                    // addu t0, s1, s2
		{name: "addu", funct: 0x21, isImm: false, rs: Word(0x7F_FF_FF_FF), rt: Word(1), expectRes: Word(0x80_00_00_00)},             // addu t0, s1, s2

		{name: "addi", opcode: 0x8, isImm: true, rs: Word(4), rt: Word(1), imm: uint16(40), expectRes: Word(44)},                              // addi t0, s1, 40
		{name: "addi", opcode: 0x8, isImm: true, rs: ^Word(0), rt: Word(0xAA_BB_CC_DD), imm: uint16(1), expectRes: Word(0)},                   // addi t0, s1, 40
		{name: "addi", opcode: 0x8, isImm: true, rs: ^Word(0), rt: Word(0xAA_BB_CC_DD), imm: uint16(0xFF_FF), expectRes: Word(0xFF_FF_FF_FE)}, // addi t0, s1, 40
		{name: "addi sign", opcode: 0x8, isImm: true, rs: Word(2), rt: Word(1), imm: uint16(0xfffe), expectRes: Word(0)},                      // addi t0, s1, -2

		{name: "addiu", opcode: 0x9, isImm: true, rs: Word(4), rt: Word(1), imm: uint16(40), expectRes: Word(44)},                              // addiu t0, s1, 40
		{name: "addiu", opcode: 0x9, isImm: true, rs: ^Word(0), rt: Word(0xAA_BB_CC_DD), imm: uint16(1), expectRes: Word(0)},                   // addiu t0, s1, 40
		{name: "addiu", opcode: 0x9, isImm: true, rs: ^Word(0), rt: Word(0xAA_BB_CC_DD), imm: uint16(0xFF_FF), expectRes: Word(0xFF_FF_FF_FE)}, // addiu t0, s1, 40

		{name: "sub", funct: 0x22, isImm: false, rs: Word(20), rt: Word(12), expectRes: Word(8)},            // sub t0, s1, s2
		{name: "sub", funct: 0x22, isImm: false, rs: ^Word(0), rt: Word(1), expectRes: Word(0xFF_FF_FF_FE)}, // sub t0, s1, s2
		{name: "sub", funct: 0x22, isImm: false, rs: Word(1), rt: ^Word(0), expectRes: Word(0x2)},           // sub t0, s1, s2

		{name: "subu", funct: 0x23, isImm: false, rs: Word(20), rt: Word(12), expectRes: Word(8)},            // subu t0, s1, s2
		{name: "subu", funct: 0x23, isImm: false, rs: ^Word(0), rt: Word(1), expectRes: Word(0xFF_FF_FF_FE)}, // subu t0, s1, s2
		{name: "subu", funct: 0x23, isImm: false, rs: Word(1), rt: ^Word(0), expectRes: Word(0x2)},           // subu t0, s1, s2
	}
	testOperators(t, cases, true)
}

func TestEVM_SingleStep_Bitwise32(t *testing.T) {
	testutil.Cannon32OnlyTest(t, "These tests are fully covered for 64-bits in TestEVM_SingleStep_Bitwise64")
	// bitwise operations that use the full word size
	cases := []operatorTestCase{
		{name: "and", funct: 0x24, isImm: false, rs: Word(1200), rt: Word(490), expectRes: Word(160)},            // and t0, s1, s2
		{name: "andi", opcode: 0xc, isImm: true, rs: Word(4), rt: Word(1), imm: uint16(40), expectRes: Word(0)},  // andi t0, s1, 40
		{name: "or", funct: 0x25, isImm: false, rs: Word(1200), rt: Word(490), expectRes: Word(1530)},            // or t0, s1, s2
		{name: "ori", opcode: 0xd, isImm: true, rs: Word(4), rt: Word(1), imm: uint16(40), expectRes: Word(44)},  // ori t0, s1, 40
		{name: "xor", funct: 0x26, isImm: false, rs: Word(1200), rt: Word(490), expectRes: Word(1370)},           // xor t0, s1, s2
		{name: "xori", opcode: 0xe, isImm: true, rs: Word(4), rt: Word(1), imm: uint16(40), expectRes: Word(44)}, // xori t0, s1, 40
		{name: "nor", funct: 0x27, isImm: false, rs: Word(1200), rt: Word(490), expectRes: Word(4294965765)},     // nor t0, s1, s2
		{name: "slt", funct: 0x2a, isImm: false, rs: 0xFF_FF_FF_FE, rt: Word(5), expectRes: Word(1)},             // slt t0, s1, s2
		{name: "sltu", funct: 0x2b, isImm: false, rs: Word(1200), rt: Word(490), expectRes: Word(0)},             // sltu t0, s1, s2
	}
	testOperators(t, cases, false)
}

func TestEVM_SingleStep_LoadStore32(t *testing.T) {
	testutil.Cannon32OnlyTest(t, "These tests are fully covered for 64-bits in TestEVM_SingleStep_LoadStore64")
	loadMemVal := Word(0x11_22_33_44)
	loadMemValNeg := Word(0xF1_F2_F3_F4)
	rtVal := Word(0xaa_bb_cc_dd)
	cases := []loadStoreTestCase{
		{name: "lb, offset=0", opcode: uint32(0x20), base: 0x100, imm: 0x20, memVal: loadMemVal, expectRes: 0x11},
		{name: "lb, offset=1", opcode: uint32(0x20), base: 0x100, imm: 0x1, memVal: loadMemVal, expectRes: 0x22},
		{name: "lb, offset=2", opcode: uint32(0x20), base: 0x100, imm: 0x2, memVal: loadMemVal, expectRes: 0x33},
		{name: "lb, offset=2, variation", opcode: uint32(0x20), base: 0x102, imm: 0x20, memVal: loadMemVal, expectRes: 0x33},
		{name: "lb, offset=4", opcode: uint32(0x20), base: 0x103, imm: 0x0, memVal: loadMemVal, expectRes: 0x44},
		{name: "lb, negative, offset=0", opcode: uint32(0x20), base: 0x100, imm: 0x0, memVal: loadMemValNeg, expectRes: 0xFF_FF_FF_F1},
		{name: "lb, negative, offset=1", opcode: uint32(0x20), base: 0x101, imm: 0x0, memVal: loadMemValNeg, expectRes: 0xFF_FF_FF_F2},
		{name: "lb, negative, offset=2", opcode: uint32(0x20), base: 0x102, imm: 0x0, memVal: loadMemValNeg, expectRes: 0xFF_FF_FF_F3},
		{name: "lb, negative, offset=3", opcode: uint32(0x20), base: 0x103, imm: 0x0, memVal: loadMemValNeg, expectRes: 0xFF_FF_FF_F4},
		{name: "lh, offset=0", opcode: uint32(0x21), base: 0x100, imm: 0x20, memVal: loadMemVal, expectRes: 0x11_22},
		{name: "lh, offset=1", opcode: uint32(0x21), base: 0x101, imm: 0x20, memVal: loadMemVal, expectRes: 0x11_22},
		{name: "lh, offset=2", opcode: uint32(0x21), base: 0x102, imm: 0x20, memVal: loadMemVal, expectRes: 0x33_44},
		{name: "lh, offset=3", opcode: uint32(0x21), base: 0x102, imm: 0x1, memVal: loadMemVal, expectRes: 0x33_44},
		{name: "lh, negative, offset=0", opcode: uint32(0x21), base: 0x100, imm: 0x20, memVal: loadMemValNeg, expectRes: 0xFF_FF_F1_F2},
		{name: "lh, negative, offset=3", opcode: uint32(0x21), base: 0x102, imm: 0x1, memVal: loadMemValNeg, expectRes: 0xFF_FF_F3_F4},
		{name: "lw", opcode: uint32(0x23), base: 0x100, imm: 0x20, memVal: loadMemVal, expectRes: 0x11_22_33_44},
		{name: "lbu", opcode: uint32(0x24), base: 0x100, imm: 0x20, memVal: loadMemVal, expectRes: 0x11},
		{name: "lbu, negative", opcode: uint32(0x24), base: 0x100, imm: 0x20, memVal: loadMemValNeg, expectRes: 0xF1},
		{name: "lhu", opcode: uint32(0x25), base: 0x100, imm: 0x20, memVal: loadMemVal, expectRes: 0x11_22},
		{name: "lhu, negative", opcode: uint32(0x25), base: 0x100, imm: 0x20, memVal: loadMemValNeg, expectRes: 0xF1_F2},
		{name: "lwl", opcode: uint32(0x22), base: 0x100, imm: 0x20, rt: rtVal, memVal: loadMemVal, expectRes: loadMemVal},
		{name: "lwl unaligned", opcode: uint32(0x22), base: 0x100, imm: 0x1, rt: rtVal, memVal: loadMemVal, expectRes: 0x22_33_44_dd},
		{name: "lwr", opcode: uint32(0x26), base: 0x100, imm: 0x20, rt: rtVal, memVal: loadMemVal, expectRes: 0xaa_bb_cc_11},
		{name: "lwr unaligned", opcode: uint32(0x26), base: 0x100, imm: 0x1, rt: rtVal, memVal: loadMemVal, expectRes: 0xaa_bb_11_22},
		{name: "sb, offset=0", opcode: uint32(0x28), base: 0x100, imm: 0x20, rt: rtVal, expectMemVal: 0xdd_00_00_00},
		{name: "sb, offset=1", opcode: uint32(0x28), base: 0x100, imm: 0x21, rt: rtVal, expectMemVal: 0x00_dd_00_00},
		{name: "sb, offset=2", opcode: uint32(0x28), base: 0x102, imm: 0x20, rt: rtVal, expectMemVal: 0x00_00_dd_00},
		{name: "sb, offset=3", opcode: uint32(0x28), base: 0x103, imm: 0x20, rt: rtVal, expectMemVal: 0x00_00_00_dd},
		{name: "sh, offset=0", opcode: uint32(0x29), base: 0x100, imm: 0x20, rt: rtVal, expectMemVal: 0xcc_dd_00_00},
		{name: "sh, offset=1", opcode: uint32(0x29), base: 0x100, imm: 0x21, rt: rtVal, expectMemVal: 0xcc_dd_00_00},
		{name: "sh, offset=2", opcode: uint32(0x29), base: 0x102, imm: 0x20, rt: rtVal, expectMemVal: 0x00_00_cc_dd},
		{name: "sh, offset=3", opcode: uint32(0x29), base: 0x102, imm: 0x21, rt: rtVal, expectMemVal: 0x00_00_cc_dd},
		{name: "swl", opcode: uint32(0x2a), base: 0x100, imm: 0x20, rt: rtVal, expectMemVal: 0xaa_bb_cc_dd},
		{name: "sw", opcode: uint32(0x2b), base: 0x100, imm: 0x20, rt: rtVal, expectMemVal: 0xaa_bb_cc_dd},
		{name: "swr unaligned", opcode: uint32(0x2e), base: 0x100, imm: 0x1, rt: rtVal, expectMemVal: 0xcc_dd_00_00},
	}
	testLoadStore(t, cases)
}

func TestEVM_SingleStep_Lui(t *testing.T) {
	versions := GetMipsVersionTestCases(t)

	cases := []struct {
		name     string
		rtReg    uint32
		imm      uint32
		expectRt Word
	}{
		{name: "lui unsigned", rtReg: 5, imm: 0x1234, expectRt: 0x1234_0000},
		{name: "lui signed", rtReg: 7, imm: 0x8765, expectRt: signExtend64(0x8765_0000)},
	}

	for _, v := range versions {
		for i, tt := range cases {
			testName := fmt.Sprintf("%v (%v)", tt.name, v.Name)
			t.Run(testName, func(t *testing.T) {
				goVm := v.VMFactory(nil, os.Stdout, os.Stderr, testutil.CreateLogger(), testutil.WithRandomization(int64(i)))
				state := goVm.GetState()
				insn := 0b1111<<26 | uint32(tt.rtReg)<<16 | (tt.imm & 0xFFFF)
				testutil.StoreInstruction(state.GetMemory(), state.GetPC(), insn)
				step := state.GetStep()

				// Setup expectations
				expected := testutil.NewExpectedState(state)
				expected.ExpectStep()
				expected.Registers[tt.rtReg] = tt.expectRt
				stepWitness, err := goVm.Step(true)
				require.NoError(t, err)

				// Check expectations
				expected.Validate(t, state)
				testutil.ValidateEVM(t, stepWitness, step, goVm, v.StateHashFn, v.Contracts)
			})
		}
	}
}

func TestEVM_SingleStep_CloClz(t *testing.T) {
	versions := GetMipsVersionTestCases(t)

	rsReg := uint32(5)
	rdReg := uint32(6)
	cases := []struct {
		name           string
		rs             Word
		expectedResult Word
		funct          uint32
	}{
		{name: "clo", rs: 0xFFFF_FFFE, expectedResult: 31, funct: 0b10_0001},
		{name: "clo", rs: 0xE000_0000, expectedResult: 3, funct: 0b10_0001},
		{name: "clo", rs: 0x8000_0000, expectedResult: 1, funct: 0b10_0001},
		{name: "clo, sign-extended", rs: signExtend64(0x8000_0000), expectedResult: 1, funct: 0b10_0001},
		{name: "clo, sign-extended", rs: signExtend64(0xF800_0000), expectedResult: 5, funct: 0b10_0001},
		{name: "clz", rs: 0x1, expectedResult: 31, funct: 0b10_0000},
		{name: "clz", rs: 0x1000_0000, expectedResult: 3, funct: 0b10_0000},
		{name: "clz", rs: 0x8000_0000, expectedResult: 0, funct: 0b10_0000},
		{name: "clz, sign-extended", rs: signExtend64(0x8000_0000), expectedResult: 0, funct: 0b10_0000},
	}

	for _, v := range versions {
		for i, tt := range cases {
			testName := fmt.Sprintf("%v (%v)", tt.name, v.Name)
			t.Run(testName, func(t *testing.T) {
				// Set up state
				goVm := v.VMFactory(nil, os.Stdout, os.Stderr, testutil.CreateLogger(), testutil.WithRandomization(int64(i)))
				state := goVm.GetState()
				insn := 0b01_1100<<26 | rsReg<<21 | rdReg<<11 | tt.funct
				testutil.StoreInstruction(state.GetMemory(), state.GetPC(), insn)
				state.GetRegistersRef()[rsReg] = tt.rs
				step := state.GetStep()

				// Setup expectations
				expected := testutil.NewExpectedState(state)
				expected.ExpectStep()
				expected.Registers[rdReg] = tt.expectedResult
				stepWitness, err := goVm.Step(true)
				require.NoError(t, err)

				// Check expectations
				expected.Validate(t, state)
				testutil.ValidateEVM(t, stepWitness, step, goVm, v.StateHashFn, v.Contracts)
			})
		}
	}
}

func TestEVM_SingleStep_MovzMovn(t *testing.T) {
	versions := GetMipsVersionTestCases(t)
	cases := []struct {
		name  string
		funct uint32
	}{
		{name: "movz", funct: uint32(0xa)},
		{name: "movn", funct: uint32(0xb)},
	}
	for _, v := range versions {
		for i, tt := range cases {
			testName := fmt.Sprintf("%v (%v)", tt.name, v.Name)
			t.Run(testName, func(t *testing.T) {
				goVm := v.VMFactory(nil, os.Stdout, os.Stderr, testutil.CreateLogger(), testutil.WithRandomization(int64(i)), testutil.WithPC(0), testutil.WithNextPC(4))
				state := goVm.GetState()
				rsReg := uint32(9)
				rtReg := uint32(10)
				rdReg := uint32(8)
				insn := rsReg<<21 | rtReg<<16 | rdReg<<11 | tt.funct
				var t2 Word
				if tt.funct == 0xa {
					t2 = 0x0
				} else {
					t2 = 0x1
				}
				state.GetRegistersRef()[rtReg] = t2
				state.GetRegistersRef()[rsReg] = Word(0xb)
				state.GetRegistersRef()[rdReg] = Word(0xa)
				testutil.StoreInstruction(state.GetMemory(), 0, insn)
				step := state.GetStep()
				// Setup expectations
				expected := testutil.NewExpectedState(state)
				expected.ExpectStep()
				expected.Registers[rdReg] = state.GetRegistersRef()[rsReg]

				stepWitness, err := goVm.Step(true)
				require.NoError(t, err)
				// Check expectations
				expected.Validate(t, state)
				testutil.ValidateEVM(t, stepWitness, step, goVm, v.StateHashFn, v.Contracts)

				if tt.funct == 0xa {
					t2 = 0x1
				} else {
					t2 = 0x0
				}
				state.GetRegistersRef()[rtReg] = t2
				expected.ExpectStep()
				expected.Registers[rtReg] = t2
				expected.Registers[rdReg] = state.GetRegistersRef()[rdReg]

				stepWitness, err = goVm.Step(true)
				require.NoError(t, err)
				// Check expectations
				expected.Validate(t, state)
				testutil.ValidateEVM(t, stepWitness, step, goVm, v.StateHashFn, v.Contracts)
			})
		}
	}

}

func TestEVM_SingleStep_MfhiMflo(t *testing.T) {
	versions := GetMipsVersionTestCases(t)
	cases := []struct {
		name  string
		funct uint32
		hi    Word
		lo    Word
	}{
		{name: "mflo", funct: uint32(0x12), lo: Word(0xdeadbeef), hi: Word(0x0)},
		{name: "mfhi", funct: uint32(0x10), lo: Word(0x0), hi: Word(0xdeadbeef)},
	}
	expect := Word(0xdeadbeef)
	for _, v := range versions {
		for i, tt := range cases {
			testName := fmt.Sprintf("%v (%v)", tt.name, v.Name)
			t.Run(testName, func(t *testing.T) {
				goVm := v.VMFactory(nil, os.Stdout, os.Stderr, testutil.CreateLogger(), testutil.WithRandomization(int64(i)), testutil.WithLO(tt.lo), testutil.WithHI(tt.hi))
				state := goVm.GetState()
				rdReg := uint32(8)
				insn := rdReg<<11 | tt.funct
				testutil.StoreInstruction(state.GetMemory(), state.GetPC(), insn)
				step := state.GetStep()
				// Setup expectations
				expected := testutil.NewExpectedState(state)
				expected.ExpectStep()
				expected.Registers[rdReg] = expect
				stepWitness, err := goVm.Step(true)
				require.NoError(t, err)
				// Check expectations
				expected.Validate(t, state)
				testutil.ValidateEVM(t, stepWitness, step, goVm, v.StateHashFn, v.Contracts)
			})
		}
	}
}

func TestEVM_SingleStep_MulDiv(t *testing.T) {
	cases := []mulDivTestCase{
		{name: "mul", funct: uint32(0x2), opcode: uint32(28), rs: Word(5), rt: Word(2), rdReg: uint32(0x8), expectRes: Word(10)},                                    // mul t0, t1, t2
		{name: "mul", funct: uint32(0x2), opcode: uint32(28), rs: Word(0x1), rt: ^Word(0), rdReg: uint32(0x8), expectRes: ^Word(0)},                                 // mul t1, t2
		{name: "mul", funct: uint32(0x2), opcode: uint32(28), rs: Word(0xFF_FF_FF_FF), rt: Word(0xFF_FF_FF_FF), rdReg: uint32(0x8), expectRes: Word(0x1)},           // mul t1, t2
		{name: "mul", funct: uint32(0x2), opcode: uint32(28), rs: Word(0xFF_FF_FF_D3), rt: Word(0xAA_BB_CC_DD), rdReg: uint32(0x8), expectRes: Word(0xFC_FC_FD_27)}, // mul t1, t2

		{name: "mult", funct: uint32(0x18), rs: Word(0x0F_FF_00_00), rt: Word(100), rdReg: uint32(0x0), opcode: uint32(0), expectHi: Word(0x6), expectLo: Word(0x3F_9C_00_00)},           // mult t1, t2
		{name: "mult", funct: uint32(0x18), rs: Word(0x1), rt: Word(0xFF_FF_FF_FF), rdReg: uint32(0x0), opcode: uint32(0), expectHi: Word(0xFF_FF_FF_FF), expectLo: Word(0xFF_FF_FF_FF)}, // mult t1, t2
		{name: "mult", funct: uint32(0x18), rs: Word(0xFF_FF_FF_FF), rt: Word(0xFF_FF_FF_FF), rdReg: uint32(0x0), opcode: uint32(0), expectHi: Word(0), expectLo: Word(0x1)},             // mult t1, t2
		{name: "mult", funct: uint32(0x18), rs: Word(0xFF_FF_FF_D3), rt: Word(0xAA_BB_CC_DD), rdReg: uint32(0x0), opcode: uint32(0), expectHi: Word(0xE), expectLo: Word(0xFC_FC_FD_27)}, // mult t1, t2

		{name: "multu", funct: uint32(0x19), rs: Word(0x0F_FF_00_00), rt: Word(100), rdReg: uint32(0x0), opcode: uint32(0), expectHi: Word(0x6), expectLo: Word(0x3F_9C_00_00)},                     // multu t1, t2
		{name: "multu", funct: uint32(0x19), rs: Word(0x1), rt: Word(0xFF_FF_FF_FF), rdReg: uint32(0x0), opcode: uint32(0), expectHi: Word(0x0), expectLo: Word(0xFF_FF_FF_FF)},                     // multu t1, t2
		{name: "multu", funct: uint32(0x19), rs: Word(0xFF_FF_FF_FF), rt: Word(0xFF_FF_FF_FF), rdReg: uint32(0x0), opcode: uint32(0), expectHi: Word(0xFF_FF_FF_FE), expectLo: Word(0x1)},           // multu t1, t2
		{name: "multu", funct: uint32(0x19), rs: Word(0xFF_FF_FF_D3), rt: Word(0xAA_BB_CC_DD), rdReg: uint32(0x0), opcode: uint32(0), expectHi: Word(0xAA_BB_CC_BE), expectLo: Word(0xFC_FC_FD_27)}, // multu t1, t2
		{name: "multu", funct: uint32(0x19), rs: Word(0xFF_FF_FF_D3), rt: Word(0xAA_BB_CC_BE), rdReg: uint32(0x0), opcode: uint32(0), expectHi: Word(0xAA_BB_CC_9F), expectLo: Word(0xFC_FD_02_9A)}, // multu t1, t2

		{name: "div", funct: uint32(0x1a), rs: Word(5), rt: Word(2), rdReg: uint32(0x0), opcode: uint32(0), expectHi: Word(1), expectLo: Word(2)},                                           // div t1, t2
		{name: "div by zero", funct: uint32(0x1a), rs: Word(5), rt: Word(0), rdReg: uint32(0x0), opcode: uint32(0), panicMsg: "instruction divide by zero", revertMsg: "division by zero"},  // div t1, t2
		{name: "divu", funct: uint32(0x1b), rs: Word(5), rt: Word(2), rdReg: uint32(0x0), opcode: uint32(0), expectHi: Word(1), expectLo: Word(2)},                                          // divu t1, t2
		{name: "divu by zero", funct: uint32(0x1b), rs: Word(5), rt: Word(0), rdReg: uint32(0x0), opcode: uint32(0), panicMsg: "instruction divide by zero", revertMsg: "division by zero"}, // divu t1, t2
	}

	testMulDiv(t, cases, true)
}

func TestEVM_SingleStep_MthiMtlo(t *testing.T) {
	versions := GetMipsVersionTestCases(t)
	cases := []struct {
		name  string
		funct uint32
	}{
		{name: "mtlo", funct: uint32(0x13)},
		{name: "mthi", funct: uint32(0x11)},
	}
	val := Word(0xdeadbeef)
	for _, v := range versions {
		for i, tt := range cases {
			testName := fmt.Sprintf("%v (%v)", tt.name, v.Name)
			t.Run(testName, func(t *testing.T) {

				goVm := v.VMFactory(nil, os.Stdout, os.Stderr, testutil.CreateLogger(), testutil.WithRandomization(int64(i)))
				state := goVm.GetState()
				rsReg := uint32(8)
				insn := rsReg<<21 | tt.funct
				testutil.StoreInstruction(state.GetMemory(), state.GetPC(), insn)
				state.GetRegistersRef()[rsReg] = val
				step := state.GetStep()
				// Setup expectations
				expected := testutil.NewExpectedState(state)
				expected.ExpectStep()
				if tt.funct == 0x11 {
					expected.HI = state.GetRegistersRef()[rsReg]
				} else {
					expected.LO = state.GetRegistersRef()[rsReg]
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

func TestEVM_SingleStep_BeqBne(t *testing.T) {
	versions := GetMipsVersionTestCases(t)
	cases := []struct {
		name   string
		imm    uint32
		opcode uint32
		rs     Word
		rt     Word
	}{
		{name: "bne", opcode: uint32(0x5), imm: uint32(0x10), rs: Word(0xaa), rt: Word(0xdeadbeef)},       // bne $t0, $t1, 16
		{name: "beq", opcode: uint32(0x4), imm: uint32(0x10), rs: Word(0xdeadbeef), rt: Word(0xdeadbeef)}, // beq $t0, $t1, 16
	}
	for _, v := range versions {
		for i, tt := range cases {
			testName := fmt.Sprintf("%v (%v)", tt.name, v.Name)
			t.Run(testName, func(t *testing.T) {
				goVm := v.VMFactory(nil, os.Stdout, os.Stderr, testutil.CreateLogger(), testutil.WithRandomization(int64(i)), testutil.WithPC(0), testutil.WithNextPC(4))
				state := goVm.GetState()
				rsReg := uint32(9)
				rtReg := uint32(8)
				insn := tt.opcode<<26 | rsReg<<21 | rtReg<<16 | tt.imm
				state.GetRegistersRef()[rtReg] = tt.rt
				state.GetRegistersRef()[rsReg] = tt.rs
				testutil.StoreInstruction(state.GetMemory(), 0, insn)
				step := state.GetStep()
				// Setup expectations
				expected := testutil.NewExpectedState(state)
				expected.Step = state.GetStep() + 1
				expected.PC = state.GetCpu().NextPC
				expected.NextPC = state.GetCpu().NextPC + Word(tt.imm<<2)

				stepWitness, err := goVm.Step(true)
				require.NoError(t, err)
				// Check expectations
				expected.Validate(t, state)
				testutil.ValidateEVM(t, stepWitness, step, goVm, v.StateHashFn, v.Contracts)
			})
		}
	}

}

func TestEVM_SingleStep_SlSr(t *testing.T) {
	versions := GetMipsVersionTestCases(t)
	cases := []struct {
		name      string
		rs        Word
		rt        Word
		rsReg     uint32
		funct     uint16
		expectVal Word
	}{
		{name: "sll", funct: uint16(4) << 6, rt: Word(0x20), rsReg: uint32(0x0), expectVal: Word(0x20) << uint8(4)},                         // sll t0, t1, 3
		{name: "srl", funct: uint16(4)<<6 | 2, rt: Word(0x20), rsReg: uint32(0x0), expectVal: Word(0x20) >> uint8(4)},                       // srl t0, t1, 3
		{name: "sra", funct: uint16(4)<<6 | 3, rt: Word(0x80_00_00_20), rsReg: uint32(0x0), expectVal: signExtend64(0xF8_00_00_02)},         // sra t0, t1, 3
		{name: "sllv", funct: uint16(4), rt: Word(0x20), rs: Word(4), rsReg: uint32(0xa), expectVal: Word(0x20) << Word(4)},                 // sllv t0, t1, t2
		{name: "srlv", funct: uint16(6), rt: Word(0x20_00), rs: Word(4), rsReg: uint32(0xa), expectVal: Word(0x20_00) >> Word(4)},           // srlv t0, t1, t2
		{name: "srav", funct: uint16(7), rt: Word(0xdeafbeef), rs: Word(12), rsReg: uint32(0xa), expectVal: signExtend64(Word(0xfffdeafb))}, // srav t0, t1, t2
	}

	for _, v := range versions {
		for i, tt := range cases {
			testName := fmt.Sprintf("%v (%v)", tt.name, v.Name)
			t.Run(testName, func(t *testing.T) {
				goVm := v.VMFactory(nil, os.Stdout, os.Stderr, testutil.CreateLogger(), testutil.WithRandomization(int64(i)), testutil.WithPC(0), testutil.WithNextPC(4))
				state := goVm.GetState()
				var insn uint32
				rtReg := uint32(0x9)
				rdReg := uint32(0x8)
				insn = tt.rsReg<<21 | rtReg<<16 | rdReg<<11 | uint32(tt.funct)
				state.GetRegistersRef()[rtReg] = tt.rt
				state.GetRegistersRef()[tt.rsReg] = tt.rs
				testutil.StoreInstruction(state.GetMemory(), 0, insn)
				step := state.GetStep()

				// Setup expectations
				expected := testutil.NewExpectedState(state)
				expected.ExpectStep()

				expected.Registers[rdReg] = tt.expectVal

				stepWitness, err := goVm.Step(true)
				require.NoError(t, err)

				// Check expectations
				expected.Validate(t, state)
				testutil.ValidateEVM(t, stepWitness, step, goVm, v.StateHashFn, v.Contracts)
			})
		}
	}
}

func TestEVM_SingleStep_JrJalr(t *testing.T) {
	versions := GetMipsVersionTestCases(t)
	cases := []struct {
		name       string
		funct      uint16
		rdReg      uint32
		expectLink bool
	}{
		{name: "jr", funct: uint16(0x8), rdReg: uint32(0)},                       // jr t0
		{name: "jalr", funct: uint16(0x9), rdReg: uint32(0x9), expectLink: true}, // jalr t1, t0
	}
	for _, v := range versions {
		for i, tt := range cases {
			testName := fmt.Sprintf("%v (%v)", tt.name, v.Name)
			t.Run(testName, func(t *testing.T) {
				goVm := v.VMFactory(nil, os.Stdout, os.Stderr, testutil.CreateLogger(), testutil.WithRandomization(int64(i)), testutil.WithPC(0), testutil.WithNextPC(4))
				state := goVm.GetState()
				rsReg := uint32(8)
				insn := rsReg<<21 | tt.rdReg<<11 | uint32(tt.funct)
				state.GetRegistersRef()[rsReg] = Word(0x34)
				testutil.StoreInstruction(state.GetMemory(), 0, insn)
				step := state.GetStep()
				// Setup expectations
				expected := testutil.NewExpectedState(state)
				expected.Step = state.GetStep() + 1
				expected.PC = state.GetCpu().NextPC
				expected.NextPC = state.GetRegistersRef()[rsReg]
				if tt.expectLink {
					expected.Registers[tt.rdReg] = state.GetPC() + 8
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

func TestEVM_MMap(t *testing.T) {
	versions := GetMipsVersionTestCases(t)
	cases := []struct {
		name         string
		heap         arch.Word
		address      arch.Word
		size         arch.Word
		shouldFail   bool
		expectedHeap arch.Word
	}{
		{name: "Increment heap by max value", heap: program.HEAP_START, address: 0, size: ^arch.Word(0), shouldFail: true},
		{name: "Increment heap to 0", heap: program.HEAP_START, address: 0, size: ^arch.Word(0) - program.HEAP_START + 1, shouldFail: true},
		{name: "Increment heap to previous page", heap: program.HEAP_START, address: 0, size: ^arch.Word(0) - program.HEAP_START - memory.PageSize + 1, shouldFail: true},
		{name: "Increment max page size", heap: program.HEAP_START, address: 0, size: ^arch.Word(0) & ^arch.Word(memory.PageAddrMask), shouldFail: true},
		{name: "Increment max page size from 0", heap: 0, address: 0, size: ^arch.Word(0) & ^arch.Word(memory.PageAddrMask), shouldFail: true},
		{name: "Increment heap at limit", heap: program.HEAP_END, address: 0, size: 1, shouldFail: true},
		{name: "Increment heap to limit", heap: program.HEAP_END - memory.PageSize, address: 0, size: 1, shouldFail: false, expectedHeap: program.HEAP_END},
		{name: "Increment heap within limit", heap: program.HEAP_END - 2*memory.PageSize, address: 0, size: 1, shouldFail: false, expectedHeap: program.HEAP_END - memory.PageSize},
		{name: "Request specific address", heap: program.HEAP_START, address: 0x50_00_00_00, size: 0, shouldFail: false, expectedHeap: program.HEAP_START},
	}

	for _, v := range versions {
		for i, c := range cases {
			testName := fmt.Sprintf("%v (%v)", c.name, v.Name)
			t.Run(testName, func(t *testing.T) {
				goVm := v.VMFactory(nil, os.Stdout, os.Stderr, testutil.CreateLogger(), testutil.WithRandomization(int64(i)), testutil.WithHeap(c.heap))
				state := goVm.GetState()

				testutil.StoreInstruction(state.GetMemory(), state.GetPC(), syscallInsn)
				state.GetRegistersRef()[2] = arch.SysMmap
				state.GetRegistersRef()[4] = c.address
				state.GetRegistersRef()[5] = c.size
				step := state.GetStep()

				expected := testutil.NewExpectedState(state)
				expected.Step += 1
				expected.PC = state.GetCpu().NextPC
				expected.NextPC = state.GetCpu().NextPC + 4
				if c.shouldFail {
					expected.Registers[2] = exec.SysErrorSignal
					expected.Registers[7] = exec.MipsEINVAL
				} else {
					expected.Heap = c.expectedHeap
					if c.address == 0 {
						expected.Registers[2] = state.GetHeap()
						expected.Registers[7] = 0
					} else {
						expected.Registers[2] = c.address
						expected.Registers[7] = 0
					}
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

func TestEVM_SysWriteHint(t *testing.T) {
	versions := GetMipsVersionTestCases(t)
	cases := []struct {
		name             string
		memOffset        int      // Where the hint data is stored in memory
		hintData         []byte   // Hint data stored in memory at memOffset
		bytesToWrite     int      // How many bytes of hintData to write
		lastHint         []byte   // The buffer that stores lastHint in the state
		expectedHints    [][]byte // The hints we expect to be processed
		expectedLastHint []byte   // The lastHint we should expect for the post-state
	}{
		{
			name:      "write 1 full hint at beginning of page",
			memOffset: 4096,
			hintData: []byte{
				0, 0, 0, 6, // Length prefix
				0xAA, 0xAA, 0xAA, 0xAA, 0xBB, 0xBB, // Hint data
			},
			bytesToWrite: 10,
			lastHint:     []byte{},
			expectedHints: [][]byte{
				{0xAA, 0xAA, 0xAA, 0xAA, 0xBB, 0xBB},
			},
			expectedLastHint: []byte{},
		},
		{
			name:      "write 1 full hint across page boundary",
			memOffset: 4092,
			hintData: []byte{
				0, 0, 0, 8, // Length prefix
				0xAA, 0xAA, 0xAA, 0xAA, 0xBB, 0xBB, 0xBB, 0xBB, // Hint data
			},
			bytesToWrite: 12,
			lastHint:     []byte{},
			expectedHints: [][]byte{
				{0xAA, 0xAA, 0xAA, 0xAA, 0xBB, 0xBB, 0xBB, 0xBB},
			},
			expectedLastHint: []byte{},
		},
		{
			name:      "write 2 full hints",
			memOffset: 5012,
			hintData: []byte{
				0, 0, 0, 6, // Length prefix
				0xAA, 0xAA, 0xAA, 0xAA, 0xBB, 0xBB, // Hint data
				0, 0, 0, 8, // Length prefix
				0xAA, 0xAA, 0xAA, 0xAA, 0xBB, 0xBB, 0xBB, 0xBB, // Hint data
			},
			bytesToWrite: 22,
			lastHint:     []byte{},
			expectedHints: [][]byte{
				{0xAA, 0xAA, 0xAA, 0xAA, 0xBB, 0xBB},
				{0xAA, 0xAA, 0xAA, 0xAA, 0xBB, 0xBB, 0xBB, 0xBB},
			},
			expectedLastHint: []byte{},
		},
		{
			name:      "write a single partial hint",
			memOffset: 4092,
			hintData: []byte{
				0, 0, 0, 6, // Length prefix
				0xAA, 0xAA, 0xAA, 0xAA, 0xBB, 0xBB, // Hint data
			},
			bytesToWrite:     8,
			lastHint:         []byte{},
			expectedHints:    nil,
			expectedLastHint: []byte{0, 0, 0, 6, 0xAA, 0xAA, 0xAA, 0xAA},
		},
		{
			name:      "write 1 full, 1 partial hint",
			memOffset: 5012,
			hintData: []byte{
				0, 0, 0, 6, // Length prefix
				0xAA, 0xAA, 0xAA, 0xAA, 0xBB, 0xBB, // Hint data
				0, 0, 0, 8, // Length prefix
				0xAA, 0xAA, 0xAA, 0xAA, 0xBB, 0xBB, 0xBB, 0xBB, // Hint data
			},
			bytesToWrite: 16,
			lastHint:     []byte{},
			expectedHints: [][]byte{
				{0xAA, 0xAA, 0xAA, 0xAA, 0xBB, 0xBB},
			},
			expectedLastHint: []byte{0, 0, 0, 8, 0xAA, 0xAA},
		},
		{
			name:      "write a single partial hint to large capacity lastHint buffer",
			memOffset: 4092,
			hintData: []byte{
				0, 0, 0, 6, // Length prefix
				0xAA, 0xAA, 0xAA, 0xAA, 0xBB, 0xBB, // Hint data
			},
			bytesToWrite:     8,
			lastHint:         make([]byte, 0, 4096),
			expectedHints:    nil,
			expectedLastHint: []byte{0, 0, 0, 6, 0xAA, 0xAA, 0xAA, 0xAA},
		},
		{
			name:      "write full hint to large capacity lastHint buffer",
			memOffset: 5012,
			hintData: []byte{
				0, 0, 0, 6, // Length prefix
				0xAA, 0xAA, 0xAA, 0xAA, 0xBB, 0xBB, // Hint data
			},
			bytesToWrite: 10,
			lastHint:     make([]byte, 0, 4096),
			expectedHints: [][]byte{
				{0xAA, 0xAA, 0xAA, 0xAA, 0xBB, 0xBB},
			},
			expectedLastHint: []byte{},
		},
		{
			name:      "write multiple hints to large capacity lastHint buffer",
			memOffset: 4092,
			hintData: []byte{
				0, 0, 0, 8, // Length prefix
				0xAA, 0xAA, 0xAA, 0xAA, 0xBB, 0xBB, 0xCC, 0xCC, // Hint data
				0, 0, 0, 8, // Length prefix
				0xAA, 0xAA, 0xAA, 0xAA, 0xBB, 0xBB, 0xBB, 0xBB, // Hint data
			},
			bytesToWrite: 24,
			lastHint:     make([]byte, 0, 4096),
			expectedHints: [][]byte{
				{0xAA, 0xAA, 0xAA, 0xAA, 0xBB, 0xBB, 0xCC, 0xCC},
				{0xAA, 0xAA, 0xAA, 0xAA, 0xBB, 0xBB, 0xBB, 0xBB},
			},
			expectedLastHint: []byte{},
		},
		{
			name:      "write remaining hint data to non-empty lastHint buffer",
			memOffset: 4092,
			hintData: []byte{
				0xAA, 0xAA, 0xAA, 0xAA, 0xBB, 0xBB, 0xCC, 0xCC, // Hint data
			},
			bytesToWrite: 8,
			lastHint:     []byte{0, 0, 0, 8},
			expectedHints: [][]byte{
				{0xAA, 0xAA, 0xAA, 0xAA, 0xBB, 0xBB, 0xCC, 0xCC},
			},
			expectedLastHint: []byte{},
		},
		{
			name:      "write partial hint data to non-empty lastHint buffer",
			memOffset: 4092,
			hintData: []byte{
				0xAA, 0xAA, 0xAA, 0xAA, 0xBB, 0xBB, 0xCC, 0xCC, // Hint data
			},
			bytesToWrite:     4,
			lastHint:         []byte{0, 0, 0, 8},
			expectedHints:    nil,
			expectedLastHint: []byte{0, 0, 0, 8, 0xAA, 0xAA, 0xAA, 0xAA},
		},
	}

	const (
		insn = uint32(0x00_00_00_0C) // syscall instruction
	)

	for _, v := range versions {
		for i, tt := range cases {
			testName := fmt.Sprintf("%v (%v)", tt.name, v.Name)
			t.Run(testName, func(t *testing.T) {
				oracle := testutil.HintTrackingOracle{}
				goVm := v.VMFactory(&oracle, os.Stdout, os.Stderr, testutil.CreateLogger(), testutil.WithRandomization(int64(i)), testutil.WithLastHint(tt.lastHint))
				state := goVm.GetState()
				state.GetRegistersRef()[2] = arch.SysWrite
				state.GetRegistersRef()[4] = exec.FdHintWrite
				state.GetRegistersRef()[5] = arch.Word(tt.memOffset)
				state.GetRegistersRef()[6] = arch.Word(tt.bytesToWrite)

				err := state.GetMemory().SetMemoryRange(arch.Word(tt.memOffset), bytes.NewReader(tt.hintData))
				require.NoError(t, err)
				testutil.StoreInstruction(state.GetMemory(), state.GetPC(), insn)
				step := state.GetStep()

				expected := testutil.NewExpectedState(state)
				expected.Step += 1
				expected.PC = state.GetCpu().NextPC
				expected.NextPC = state.GetCpu().NextPC + 4
				expected.LastHint = tt.expectedLastHint
				expected.Registers[2] = arch.Word(tt.bytesToWrite) // Return count of bytes written
				expected.Registers[7] = 0                          // no Error

				stepWitness, err := goVm.Step(true)
				require.NoError(t, err)

				expected.Validate(t, state)
				require.Equal(t, tt.expectedHints, oracle.Hints())
				testutil.ValidateEVM(t, stepWitness, step, goVm, v.StateHashFn, v.Contracts)
			})
		}
	}
}

func TestEVM_Fault(t *testing.T) {
	var tracer *tracing.Hooks // no-tracer by default, but see test_util.MarkdownTracer

	versions := GetMipsVersionTestCases(t)

	misAlignedInstructionErr := func() testutil.ErrMatcher {
		if arch.IsMips32 {
			// matches revert(0,0)
			return testutil.CreateNoopErrorMatcher()
		} else {
			return testutil.CreateCustomErrorMatcher("InvalidPC()")
		}
	}

	cases := []struct {
		name                 string
		pc                   arch.Word
		nextPC               arch.Word
		insn                 uint32
		errMsg               testutil.ErrMatcher
		memoryProofAddresses []Word
	}{
		{name: "illegal instruction", nextPC: 0, insn: 0b111110 << 26, errMsg: testutil.CreateErrorStringMatcher("invalid instruction"), memoryProofAddresses: []Word{0x0}}, // memoryProof for the zero address at register 0 (+ imm)
		{name: "branch in delay-slot", nextPC: 8, insn: 0x11_02_00_03, errMsg: testutil.CreateErrorStringMatcher("branch in delay slot")},
		{name: "jump in delay-slot", nextPC: 8, insn: 0x0c_00_00_0c, errMsg: testutil.CreateErrorStringMatcher("jump in delay slot")},
		{name: "misaligned instruction", pc: 1, nextPC: 4, insn: 0b110111_00001_00001 << 16, errMsg: misAlignedInstructionErr()},
		{name: "misaligned instruction", pc: 2, nextPC: 4, insn: 0b110111_00001_00001 << 16, errMsg: misAlignedInstructionErr()},
		{name: "misaligned instruction", pc: 3, nextPC: 4, insn: 0b110111_00001_00001 << 16, errMsg: misAlignedInstructionErr()},
		{name: "misaligned instruction", pc: 5, nextPC: 4, insn: 0b110111_00001_00001 << 16, errMsg: misAlignedInstructionErr()},
	}

	for _, v := range versions {
		for _, tt := range cases {
			testName := fmt.Sprintf("%v (%v)", tt.name, v.Name)
			t.Run(testName, func(t *testing.T) {
				goVm := v.VMFactory(nil, os.Stdout, os.Stderr, testutil.CreateLogger(), testutil.WithPC(tt.pc), testutil.WithNextPC(tt.nextPC))
				state := goVm.GetState()
				testutil.StoreInstruction(state.GetMemory(), 0, tt.insn)
				// set the return address ($ra) to jump into when test completes
				state.GetRegistersRef()[31] = testutil.EndAddr

				proofData := v.ProofGenerator(t, goVm.GetState(), tt.memoryProofAddresses...)
				require.Panics(t, func() { _, _ = goVm.Step(false) })
				testutil.AssertEVMReverts(t, state, v.Contracts, tracer, proofData, tt.errMsg)
			})
		}
	}
}

func TestEVM_HelloProgram(t *testing.T) {
	if os.Getenv("SKIP_SLOW_TESTS") == "true" {
		t.Skip("Skipping slow test because SKIP_SLOW_TESTS is enabled")
	}

	t.Parallel()
	versions := GetMipsVersionTestCases(t)

	for _, v := range versions {
		v := v
		t.Run(v.Name, func(t *testing.T) {
			t.Parallel()
			validator := testutil.NewEvmValidator(t, v.StateHashFn, v.Contracts)

			var stdOutBuf, stdErrBuf bytes.Buffer
			elfFile := testutil.ProgramPath("hello")
			goVm := v.ElfVMFactory(t, elfFile, nil, io.MultiWriter(&stdOutBuf, os.Stdout), io.MultiWriter(&stdErrBuf, os.Stderr), testutil.CreateLogger())
			state := goVm.GetState()

			start := time.Now()
			for i := 0; i < 430_000; i++ {
				step := goVm.GetState().GetStep()
				if goVm.GetState().GetExited() {
					break
				}
				insn := testutil.GetInstruction(state.GetMemory(), state.GetPC())
				if i%10_000 == 0 { // avoid spamming test logs, we are executing many steps
					t.Logf("step: %4d pc: 0x%08x insn: 0x%08x", state.GetStep(), state.GetPC(), insn)
				}

				stepWitness, err := goVm.Step(true)
				require.NoError(t, err)
				validator.ValidateEVM(t, stepWitness, step, goVm)
			}
			end := time.Now()
			delta := end.Sub(start)
			t.Logf("test took %s, %d instructions, %s per instruction", delta, state.GetStep(), delta/time.Duration(state.GetStep()))

			require.True(t, state.GetExited(), "must complete program")
			require.Equal(t, uint8(0), state.GetExitCode(), "exit with 0")

			require.Equal(t, "hello world!\n", stdOutBuf.String(), "stdout says hello")
			require.Equal(t, "", stdErrBuf.String(), "stderr silent")
		})
	}
}

func TestEVM_ClaimProgram(t *testing.T) {
	if os.Getenv("SKIP_SLOW_TESTS") == "true" {
		t.Skip("Skipping slow test because SKIP_SLOW_TESTS is enabled")
	}

	t.Parallel()
	versions := GetMipsVersionTestCases(t)

	for _, v := range versions {
		v := v
		t.Run(v.Name, func(t *testing.T) {
			t.Parallel()
			validator := testutil.NewEvmValidator(t, v.StateHashFn, v.Contracts)
			oracle, expectedStdOut, expectedStdErr := testutil.ClaimTestOracle(t)

			var stdOutBuf, stdErrBuf bytes.Buffer
			elfFile := testutil.ProgramPath("claim")
			goVm := v.ElfVMFactory(t, elfFile, oracle, io.MultiWriter(&stdOutBuf, os.Stdout), io.MultiWriter(&stdErrBuf, os.Stderr), testutil.CreateLogger())
			state := goVm.GetState()

			for i := 0; i < 2000_000; i++ {
				curStep := goVm.GetState().GetStep()
				if goVm.GetState().GetExited() {
					break
				}

				insn := testutil.GetInstruction(state.GetMemory(), state.GetPC())
				if i%10_000 == 0 { // avoid spamming test logs, we are executing many steps
					t.Logf("step: %4d pc: 0x%08x insn: 0x%08x", state.GetStep(), state.GetPC(), insn)
				}

				stepWitness, err := goVm.Step(true)
				require.NoError(t, err)
				validator.ValidateEVM(t, stepWitness, curStep, goVm)
			}

			require.True(t, state.GetExited(), "must complete program")
			require.Equal(t, uint8(0), state.GetExitCode(), "exit with 0")

			require.Equal(t, expectedStdOut, stdOutBuf.String(), "stdout")
			require.Equal(t, expectedStdErr, stdErrBuf.String(), "stderr")
		})
	}
}

func TestEVM_EntryProgram(t *testing.T) {
	if os.Getenv("SKIP_SLOW_TESTS") == "true" {
		t.Skip("Skipping slow test because SKIP_SLOW_TESTS is enabled")
	}

	t.Parallel()
	versions := GetMipsVersionTestCases(t)

	for _, v := range versions {
		v := v
		t.Run(v.Name, func(t *testing.T) {
			t.Parallel()
			validator := testutil.NewEvmValidator(t, v.StateHashFn, v.Contracts)

			var stdOutBuf, stdErrBuf bytes.Buffer
			elfFile := testutil.ProgramPath("entry")
			goVm := v.ElfVMFactory(t, elfFile, nil, io.MultiWriter(&stdOutBuf, os.Stdout), io.MultiWriter(&stdErrBuf, os.Stderr), testutil.CreateLogger())
			state := goVm.GetState()

			start := time.Now()
			for i := 0; i < 500_000; i++ {
				curStep := goVm.GetState().GetStep()
				if goVm.GetState().GetExited() {
					break
				}
				insn := testutil.GetInstruction(state.GetMemory(), state.GetPC())
				if i%10_000 == 0 { // avoid spamming test logs, we are executing many steps
					t.Logf("step: %4d pc: 0x%08x insn: 0x%08x", state.GetStep(), state.GetPC(), insn)
				}

				stepWitness, err := goVm.Step(true)
				require.NoError(t, err)
				validator.ValidateEVM(t, stepWitness, curStep, goVm)
			}
			end := time.Now()
			delta := end.Sub(start)
			t.Logf("test took %s, %d instructions, %s per instruction", delta, state.GetStep(), delta/time.Duration(state.GetStep()))

			require.True(t, state.GetExited(), "must complete program")
			require.Equal(t, uint8(0), state.GetExitCode(), "exit with 0")
		})
	}
}

func TestEVM_SingleStep_Branch32(t *testing.T) {
	testutil.Cannon32OnlyTest(t, "These tests are fully covered for 64-bits in TestEVM_SingleStep_Branch64")
	t.Parallel()
	cases := []branchTestCase{
		// blez
		{name: "blez", pc: 0, opcode: 0x6, rs: 0x5, offset: 0x100, expectNextPC: 0x8},
		{name: "blez large rs", pc: 0x10, opcode: 0x6, rs: 0x7F_FF_FF_FF, offset: 0x100, expectNextPC: 0x18},
		{name: "blez zero rs", pc: 0x10, opcode: 0x6, rs: 0x0, offset: 0x100, expectNextPC: 0x414},
		{name: "blez sign rs", pc: 0x10, opcode: 0x6, rs: -1, offset: 0x100, expectNextPC: 0x414},
		{name: "blez rs only sign bit set", pc: 0x10, opcode: 0x6, rs: testutil.ToSignedInteger(0x80_00_00_00), offset: 0x100, expectNextPC: 0x414},
		{name: "blez sign-extended offset", pc: 0x10, opcode: 0x6, rs: -1, offset: 0x80_00, expectNextPC: 0xFF_FE_00_14},

		// bgtz
		{name: "bgtz", pc: 0, opcode: 0x7, rs: 0x5, offset: 0x100, expectNextPC: 0x404},
		{name: "bgtz sign-extended offset", pc: 0x10, opcode: 0x7, rs: 0x5, offset: 0x80_00, expectNextPC: 0xFF_FE_00_14},
		{name: "bgtz large rs", pc: 0x10, opcode: 0x7, rs: 0x7F_FF_FF_FF, offset: 0x100, expectNextPC: 0x414},
		{name: "bgtz zero rs", pc: 0x10, opcode: 0x7, rs: 0x0, offset: 0x100, expectNextPC: 0x18},
		{name: "bgtz sign rs", pc: 0x10, opcode: 0x7, rs: -1, offset: 0x100, expectNextPC: 0x18},
		{name: "bgtz rs only sign bit set", pc: 0x10, opcode: 0x7, rs: testutil.ToSignedInteger(0x80_00_00_00), offset: 0x100, expectNextPC: 0x18},

		// bltz t0, $x
		{name: "bltz", pc: 0, opcode: 0x1, regimm: 0x0, rs: 0x5, offset: 0x100, expectNextPC: 0x8},
		{name: "bltz large rs", pc: 0x10, opcode: 0x1, regimm: 0x0, rs: 0x7F_FF_FF_FF, offset: 0x100, expectNextPC: 0x18},
		{name: "bltz zero rs", pc: 0x10, opcode: 0x1, regimm: 0x0, rs: 0x0, offset: 0x100, expectNextPC: 0x18},
		{name: "bltz sign rs", pc: 0x10, opcode: 0x1, regimm: 0x0, rs: -1, offset: 0x100, expectNextPC: 0x414},
		{name: "bltz rs only sign bit set", pc: 0x10, opcode: 0x1, regimm: 0x0, rs: testutil.ToSignedInteger(0x80_00_00_00), offset: 0x100, expectNextPC: 0x414},
		{name: "bltz sign-extended offset", pc: 0x10, opcode: 0x1, regimm: 0x0, rs: -1, offset: 0x80_00, expectNextPC: 0xFF_FE_00_14},
		{name: "bltz large offset no-sign", pc: 0x10, opcode: 0x1, regimm: 0x0, rs: -1, offset: 0x7F_FF, expectNextPC: 0x2_00_10},

		// bltzal t0, $x
		{name: "bltzal", pc: 0, opcode: 0x1, regimm: 0x10, rs: 0x5, offset: 0x100, expectNextPC: 0x8, expectLink: true},
		{name: "bltzal large rs", pc: 0x10, opcode: 0x1, regimm: 0x10, rs: 0x7F_FF_FF_FF, offset: 0x100, expectNextPC: 0x18, expectLink: true},
		{name: "bltzal zero rs", pc: 0x10, opcode: 0x1, regimm: 0x10, rs: 0x0, offset: 0x100, expectNextPC: 0x18, expectLink: true},
		{name: "bltzal sign rs", pc: 0x10, opcode: 0x1, regimm: 0x10, rs: -1, offset: 0x100, expectNextPC: 0x414, expectLink: true},
		{name: "bltzal rs only sign bit set", pc: 0x10, opcode: 0x1, regimm: 0x10, rs: testutil.ToSignedInteger(0x80_00_00_00), offset: 0x100, expectNextPC: 0x414, expectLink: true},
		{name: "bltzal sign-extended offset", pc: 0x10, opcode: 0x1, regimm: 0x10, rs: -1, offset: 0x80_00, expectNextPC: 0xFF_FE_00_14, expectLink: true},
		{name: "bltzal large offset no-sign", pc: 0x10, opcode: 0x1, regimm: 0x10, rs: -1, offset: 0x7F_FF, expectNextPC: 0x2_00_10, expectLink: true},

		// bgez t0, $x
		{name: "bgez", pc: 0, opcode: 0x1, regimm: 0x1, rs: 0x5, offset: 0x100, expectNextPC: 0x404},
		{name: "bgez large rs", pc: 0x10, opcode: 0x1, regimm: 0x1, rs: 0x7F_FF_FF_FF, offset: 0x100, expectNextPC: 0x414},
		{name: "bgez zero rs", pc: 0x10, opcode: 0x1, regimm: 0x1, rs: 0x0, offset: 0x100, expectNextPC: 0x414},
		{name: "bgez branch not taken", pc: 0x10, opcode: 0x1, regimm: 0x1, rs: -1, offset: 0x100, expectNextPC: 0x18},
		{name: "bgez sign-extended offset", pc: 0x10, opcode: 0x1, regimm: 0x1, rs: 1, offset: 0x80_00, expectNextPC: 0xFF_FE_00_14},
		{name: "bgez large offset no-sign", pc: 0x10, opcode: 0x1, regimm: 0x1, rs: 1, offset: 0x70_00, expectNextPC: 0x1_C0_14},
		{name: "bgez fill bit offset except sign", pc: 0x10, opcode: 0x1, regimm: 0x1, rs: 1, offset: 0x7F_FF, expectNextPC: 0x2_00_10},

		// bgezal t0, $x
		{name: "bgezal", pc: 0, opcode: 0x1, regimm: 0x11, rs: 0x5, offset: 0x100, expectNextPC: 0x404, expectLink: true},
		{name: "bgezal large rs", pc: 0x10, opcode: 0x1, regimm: 0x11, rs: 0x7F_FF_FF_FF, offset: 0x100, expectNextPC: 0x414, expectLink: true},
		{name: "bgezal zero rs", pc: 0x10, opcode: 0x1, regimm: 0x11, rs: 0x0, offset: 0x100, expectNextPC: 0x414, expectLink: true},
		{name: "bgezal branch not taken", pc: 0x10, opcode: 0x1, regimm: 0x11, rs: -1, offset: 0x100, expectNextPC: 0x18, expectLink: true},
		{name: "bgezal sign-extended offset", pc: 0x10, opcode: 0x1, regimm: 0x11, rs: 1, offset: 0x80_00, expectNextPC: 0xFF_FE_00_14, expectLink: true},
		{name: "bgezal large offset no-sign", pc: 0x10, opcode: 0x1, regimm: 0x11, rs: 1, offset: 0x70_00, expectNextPC: 0x1_C0_14, expectLink: true},
		{name: "bgezal fill bit offset except sign", pc: 0x10, opcode: 0x1, regimm: 0x11, rs: 1, offset: 0x7F_FF, expectNextPC: 0x2_00_10, expectLink: true},
	}

	testBranch(t, cases)
}
