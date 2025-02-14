//go:build cannon64
// +build cannon64

package tests

import (
	"fmt"
	"os"
	"testing"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm/testutil"
	"github.com/stretchr/testify/require"
)

func TestEVM_SingleStep_Operators64(t *testing.T) {
	cases := []operatorTestCase{
		{name: "dadd. both unsigned 32", funct: 0x2c, isImm: false, rs: Word(0x12), rt: Word(0x20), expectRes: Word(0x32)},                                                                  // dadd t0, s1, s2
		{name: "dadd. unsigned 32 and signed", funct: 0x2c, isImm: false, rs: Word(0x12), rt: Word(^uint32(0)), expectRes: Word(0x1_00_00_00_11)},                                           // dadd t0, s1, s2
		{name: "dadd. signed and unsigned 32", funct: 0x2c, isImm: false, rs: Word(^uint32(0)), rt: Word(0x12), expectRes: Word(0x1_00_00_00_11)},                                           // dadd t0, s1, s2
		{name: "dadd. unsigned 64 and unsigned 32", funct: 0x2c, isImm: false, rs: Word(0x0FFFFFFF_00000012), rt: Word(0x20), expectRes: Word(0x0FFFFFFF_00000032)},                         // dadd t0, s1, s2
		{name: "dadd. unsigned 32 and signed", funct: 0x2c, isImm: false, rs: Word(12), rt: ^Word(0), expectRes: Word(11)},                                                                  // dadd t0, s1, s2
		{name: "dadd. signed and unsigned 32", funct: 0x2c, isImm: false, rs: ^Word(0), rt: Word(12), expectRes: Word(11)},                                                                  // dadd t0, s1, s2
		{name: "dadd. signed and unsigned 32. expect signed", funct: 0x2c, isImm: false, rs: ^Word(20), rt: Word(4), expectRes: ^Word(16)},                                                  // dadd t0, s1, s2
		{name: "dadd. unsigned 32 and signed. expect signed", funct: 0x2c, isImm: false, rs: Word(4), rt: ^Word(20), expectRes: ^Word(16)},                                                  // dadd t0, s1, s2
		{name: "dadd. both signed", funct: 0x2c, isImm: false, rs: ^Word(10), rt: ^Word(4), expectRes: ^Word(15)},                                                                           // dadd t0, s1, s2
		{name: "dadd. signed and unsigned 64. expect unsigned", funct: 0x2c, isImm: false, rs: ^Word(0), rt: Word(0x000000FF_00000000), expectRes: Word(0x000000FE_FFFFFFFF)},               // dadd t0, s1, s2
		{name: "dadd. signed and unsigned 64. expect signed", funct: 0x2c, isImm: false, rs: Word(0x80000000_00000000), rt: Word(0x40000000_00000000), expectRes: Word(0xC000000000000000)}, // dadd t0, s1, s2

		{name: "daddu. both 32", funct: 0x2d, isImm: false, rs: Word(0x12), rt: Word(0x20), expectRes: Word(0x32)},                                                    // daddu t0, s1, s2
		{name: "daddu. 32-bit. expect doubleword-sized", funct: 0x2d, isImm: false, rs: Word(0x12), rt: Word(^uint32(0)), expectRes: Word(0x1_00_00_00_11)},           // daddu t0, s1, s2
		{name: "daddu. 32-bit. expect double-word sized x", funct: 0x2d, isImm: false, rs: Word(^uint32(0)), rt: Word(0x12), expectRes: Word(0x1_00_00_00_11)},        // dadu t0, s1, s2
		{name: "daddu. doubleword-sized, word-sized", funct: 0x2d, isImm: false, rs: Word(0x0FFFFFFF_00000012), rt: Word(0x20), expectRes: Word(0x0FFFFFFF_00000032)}, // dadu t0, s1, s2
		{name: "daddu. overflow. rt sign bit set", funct: 0x2d, isImm: false, rs: Word(12), rt: ^Word(0), expectRes: Word(11)},                                        // dadu t0, s1, s2
		{name: "daddu. overflow. rs sign bit set", funct: 0x2d, isImm: false, rs: ^Word(0), rt: Word(12), expectRes: Word(11)},                                        // dadu t0, s1, s2
		{name: "daddu. doubleword-sized and word-sized", funct: 0x2d, isImm: false, rs: ^Word(20), rt: Word(4), expectRes: ^Word(16)},                                 // dadu t0, s1, s2
		{name: "daddu. word-sized and doubleword-sized", funct: 0x2d, isImm: false, rs: Word(4), rt: ^Word(20), expectRes: ^Word(16)},                                 // dadu t0, s1, s2
		{name: "daddu. both doubleword-sized. expect overflow", funct: 0x2d, isImm: false, rs: ^Word(10), rt: ^Word(4), expectRes: ^Word(15)},                         // dadu t0, s1, s2

		{name: "daddi word-sized", opcode: 0x18, isImm: true, rs: Word(12), rt: ^Word(0), imm: uint16(20), expectRes: Word(32)},                                           // daddi t0, s1, s2
		{name: "daddi doubleword-sized", opcode: 0x18, isImm: true, rs: Word(0x00000010_00000000), rt: ^Word(0), imm: uint16(0x20), expectRes: Word(0x00000010_00000020)}, // daddi t0, s1, s2
		{name: "daddi 32-bit sign", opcode: 0x18, isImm: true, rs: Word(0xFF_FF_FF_FF), rt: ^Word(0), imm: uint16(0x20), expectRes: Word(0x01_00_00_00_1F)},               // daddi t0, s1, s2
		{name: "daddi double-word signed", opcode: 0x18, isImm: true, rs: ^Word(0), rt: ^Word(0), imm: uint16(0x20), expectRes: Word(0x1F)},                               // daddi t0, s1, s2
		{name: "daddi double-word signed. expect signed", opcode: 0x18, isImm: true, rs: ^Word(0x10), rt: ^Word(0), imm: uint16(0x1), expectRes: ^Word(0xF)},              // daddi t0, s1, s2

		{name: "daddiu word-sized", opcode: 0x19, isImm: true, rs: Word(4), rt: ^Word(0), imm: uint16(40), expectRes: Word(44)},                                            // daddiu t0, s1, 40
		{name: "daddiu doubleword-sized", opcode: 0x19, isImm: true, rs: Word(0x00000010_00000000), rt: ^Word(0), imm: uint16(0x20), expectRes: Word(0x00000010_00000020)}, // daddiu t0, s1, 40
		{name: "daddiu 32-bit sign", opcode: 0x19, isImm: true, rs: Word(0xFF_FF_FF_FF), rt: ^Word(0), imm: uint16(0x20), expectRes: Word(0x01_00_00_00_1F)},               // daddiu t0, s1, 40
		{name: "daddiu overflow", opcode: 0x19, isImm: true, rs: ^Word(0), rt: ^Word(0), imm: uint16(0x20), expectRes: Word(0x1F)},                                         // daddiu t0, s1, s2

		{name: "dsub. both unsigned 32", funct: 0x2e, isImm: false, rs: Word(0x12), rt: Word(0x1), expectRes: Word(0x11)},                                     // dsub t0, s1, s2
		{name: "dsub. signed and unsigned 32", funct: 0x2e, isImm: false, rs: ^Word(1), rt: Word(0x1), expectRes: Word(^uint64(2))},                           // dsub t0, s1, s2
		{name: "dsub. signed and unsigned 64", funct: 0x2e, isImm: false, rs: ^Word(1), rt: Word(0x00AABBCC_00000000), expectRes: ^Word(0x00AABBCC_00000001)}, // dsub t0, s1, s2
		{name: "dsub. both signed. unsigned result", funct: 0x2e, isImm: false, rs: ^Word(1), rt: ^Word(2), expectRes: Word(1)},                               // dsub t0, s1, s2
		{name: "dsub. both signed. signed result", funct: 0x2e, isImm: false, rs: ^Word(2), rt: ^Word(1), expectRes: ^Word(0)},                                // dsub t0, s1, s2
		{name: "dsub. signed and zero", funct: 0x2e, isImm: false, rs: ^Word(0), rt: Word(0), expectRes: ^Word(0)},                                            // dsub t0, s1, s2

		{name: "dsubu. both unsigned 32", funct: 0x2f, isImm: false, rs: Word(0x12), rt: Word(0x1), expectRes: Word(0x11)},                                       // dsubu t0, s1, s2
		{name: "dsubu. signed and unsigned 32", funct: 0x2f, isImm: false, rs: ^Word(1), rt: Word(0x1), expectRes: Word(^uint64(2))},                             // dsubu t0, s1, s2
		{name: "dsubu. signed and unsigned 64", funct: 0x2f, isImm: false, rs: ^Word(1), rt: Word(0x00AABBCC_00000000), expectRes: ^Word(0x00AABBCC_00000001)},   // dsubu t0, s1, s2
		{name: "dsubu. both signed. unsigned result", funct: 0x2f, isImm: false, rs: ^Word(1), rt: ^Word(2), expectRes: Word(1)},                                 // dsubu t0, s1, s2
		{name: "dsubu. both signed. signed result", funct: 0x2f, isImm: false, rs: ^Word(2), rt: ^Word(1), expectRes: ^Word(0)},                                  // dsubu t0, s1, s2
		{name: "dsubu. signed and zero", funct: 0x2f, isImm: false, rs: ^Word(0), rt: Word(0), expectRes: ^Word(0)},                                              // dsubu t0, s1, s2
		{name: "dsubu. overflow", funct: 0x2f, isImm: false, rs: Word(0x80000000_00000000), rt: Word(0x7FFFFFFF_FFFFFFFF), expectRes: Word(0x00000000_00000001)}, // dsubu t0, s1, s2

		// dsllv
		{name: "dsllv", funct: 0x14, rt: Word(0x20), rs: Word(0), expectRes: Word(0x20)},
		{name: "dsllv", funct: 0x14, rt: Word(0x20), rs: Word(1), expectRes: Word(0x40)},
		{name: "dsllv sign", funct: 0x14, rt: Word(0x80_00_00_00_00_00_00_20), rs: Word(1), expectRes: Word(0x00_00_00_00_00_00_00_40)},
		{name: "dsllv max", funct: 0x14, rt: Word(0xFF_FF_FF_FF_FF_FF_FF_Fe), rs: Word(0x3f), expectRes: Word(0x0)},
		{name: "dsllv max almost clear", funct: 0x14, rt: Word(0x1), rs: Word(0x3f), expectRes: Word(0x80_00_00_00_00_00_00_00)},

		// dsrlv t0, s1, s2
		{name: "dsrlv", funct: 0x16, rt: Word(0x20), rs: Word(0), expectRes: Word(0x20)},
		{name: "dsrlv", funct: 0x16, rt: Word(0x20), rs: Word(1), expectRes: Word(0x10)},
		{name: "dsrlv sign-extend", funct: 0x16, rt: Word(0x80_00_00_00_00_00_00_20), rs: Word(1), expectRes: Word(0x40_00_00_00_00_00_00_10)},
		{name: "dsrlv max", funct: 0x16, rt: Word(0x7F_FF_00_00_00_00_00_20), rs: Word(0x3f), expectRes: Word(0x0)},
		{name: "dsrlv max sign-extend", funct: 0x16, rt: Word(0x80_00_00_00_00_00_00_20), rs: Word(0x3f), expectRes: Word(0x1)},

		// dsrav t0, s1, s2
		{name: "dsrav", funct: 0x17, rt: Word(0x20), rs: Word(0), expectRes: Word(0x20)},
		{name: "dsrav", funct: 0x17, rt: Word(0x20), rs: Word(1), expectRes: Word(0x10)},
		{name: "dsrav sign-extend", funct: 0x17, rt: Word(0x80_00_00_00_00_00_00_20), rs: Word(1), expectRes: Word(0xc0_00_00_00_00_00_00_10)},
		{name: "dsrav max", funct: 0x17, rt: Word(0x7F_FF_00_00_00_00_00_20), rs: Word(0x3f), expectRes: Word(0x0)},
		{name: "dsrav max sign-extend", funct: 0x17, rt: Word(0x80_00_00_00_00_00_00_20), rs: Word(0x3f), expectRes: Word(0xFF_FF_FF_FF_FF_FF_FF_FF)},
	}
	testOperators(t, cases, false)
}

func TestEVM_SingleStep_Bitwise64(t *testing.T) {
	cases := []operatorTestCase{
		{name: "and", funct: 0x24, isImm: false, rs: Word(1200), rt: Word(490), expectRes: Word(160)},                          // and t0, s1, s2
		{name: "andi", opcode: 0xc, isImm: true, rs: Word(4), rt: Word(1), imm: uint16(40), expectRes: Word(0)},                // andi t0, s1, 40
		{name: "or", funct: 0x25, isImm: false, rs: Word(1200), rt: Word(490), expectRes: Word(1530)},                          // or t0, s1, s2
		{name: "ori", opcode: 0xd, isImm: true, rs: Word(4), rt: Word(1), imm: uint16(40), expectRes: Word(44)},                // ori t0, s1, 40
		{name: "xor", funct: 0x26, isImm: false, rs: Word(1200), rt: Word(490), expectRes: Word(1370)},                         // xor t0, s1, s2
		{name: "xori", opcode: 0xe, isImm: true, rs: Word(4), rt: Word(1), imm: uint16(40), expectRes: Word(44)},               // xori t0, s1, 40
		{name: "nor", funct: 0x27, isImm: false, rs: Word(0x4b0), rt: Word(0x1ea), expectRes: Word(0xFF_FF_FF_FF_FF_FF_FA_05)}, // nor t0, s1, s2
		{name: "slt", funct: 0x2a, isImm: false, rs: 0xFF_FF_FF_FE, rt: Word(5), expectRes: Word(0)},                           // slt t0, s1, s2
		{name: "slt", funct: 0x2a, isImm: false, rs: 0xFF_FF_FF_FF_FF_FF_FF_FE, rt: Word(5), expectRes: Word(1)},               // slt t0, s1, s2
		{name: "sltu", funct: 0x2b, isImm: false, rs: Word(1200), rt: Word(490), expectRes: Word(0)},                           // sltu t0, s1, s2
	}
	testOperators(t, cases, false)
}

func TestEVM_SingleStep_Shift64(t *testing.T) {
	cases := []struct {
		name      string
		rd        Word
		rt        Word
		sa        uint32
		funct     uint32
		expectRes Word
	}{
		{name: "dsll", funct: 0x38, rd: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0x1), sa: 0, expectRes: Word(0x1)},                                              // dsll t8, s2, 0
		{name: "dsll", funct: 0x38, rd: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0x1), sa: 1, expectRes: Word(0x2)},                                              // dsll t8, s2, 1
		{name: "dsll", funct: 0x38, rd: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0x1), sa: 31, expectRes: Word(0x80_00_00_00)},                                   // dsll t8, s2, 31
		{name: "dsll", funct: 0x38, rd: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0xFF_FF_FF_FF_00_00_00_00), sa: 1, expectRes: Word(0xFF_FF_FF_FE_00_00_00_00)},  // dsll t8, s2, 1
		{name: "dsll", funct: 0x38, rd: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0xFF_FF_FF_FF_00_00_00_00), sa: 31, expectRes: Word(0x80_00_00_00_00_00_00_00)}, // dsll t8, s2, 31

		{name: "dsrl", funct: 0x3a, rd: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0x1), sa: 0, expectRes: Word(0x1)},                                             // dsrl t8, s2, 0
		{name: "dsrl", funct: 0x3a, rd: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0x1), sa: 1, expectRes: Word(0x0)},                                             // dsrl t8, s2, 1
		{name: "dsrl", funct: 0x3a, rd: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0xFF_FF_FF_FF_00_00_00_00), sa: 1, expectRes: Word(0x7F_FF_FF_FF_80_00_00_00)}, // dsrl t8, s2, 1
		{name: "dsrl", funct: 0x3a, rd: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0xFF_FF_FF_FF_00_00_00_00), sa: 31, expectRes: Word(0x01_FF_FF_FF_FE)},         // dsrl t8, s2, 31

		{name: "dsra", funct: 0x3b, rd: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0x1), sa: 0, expectRes: Word(0x1)},                                              // dsra t8, s2, 0
		{name: "dsra", funct: 0x3b, rd: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0x1), sa: 1, expectRes: Word(0x0)},                                              // dsra t8, s2, 1
		{name: "dsra", funct: 0x3b, rd: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0xFF_FF_FF_FF_00_00_00_00), sa: 1, expectRes: Word(0xFF_FF_FF_FF_80_00_00_00)},  // dsra t8, s2, 1
		{name: "dsra", funct: 0x3b, rd: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0xFF_FF_FF_FF_00_00_00_00), sa: 31, expectRes: Word(0xFF_FF_FF_FF_FF_FF_FF_FE)}, // dsra t8, s2, 31

		{name: "dsll32", funct: 0x3c, rd: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0x1), sa: 0, expectRes: Word(0x1_00_00_00_00)},                                  // dsll32 t8, s2, 0
		{name: "dsll32", funct: 0x3c, rd: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0x1), sa: 1, expectRes: Word(0x2_00_00_00_00)},                                  // dsll32 t8, s2, 1
		{name: "dsll32", funct: 0x3c, rd: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0x1), sa: 31, expectRes: Word(0x80_00_00_00_00_00_00_00)},                       // dsll32 t8, s2, 31
		{name: "dsll32", funct: 0x3c, rd: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0xFF_FF_FF_FF_FF_FF_FF_FF), sa: 1, expectRes: Word(0xFF_FF_FF_FE_00_00_00_00)},  // dsll32 t8, s2, 1
		{name: "dsll32", funct: 0x3c, rd: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0xFF_FF_FF_FF_FF_FF_FF_FF), sa: 31, expectRes: Word(0x80_00_00_00_00_00_00_00)}, // dsll32 t8, s2, 31

		{name: "dsrl32", funct: 0x3e, rd: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0x1), sa: 0, expectRes: Word(0x0)},                                 // dsrl32 t8, s2, 0
		{name: "dsrl32", funct: 0x3e, rd: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0x1), sa: 31, expectRes: Word(0x0)},                                // dsrl32 t8, s2, 31
		{name: "dsrl32", funct: 0x3e, rd: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0xFF_FF_FF_FF_FF_FF_FF_FF), sa: 1, expectRes: Word(0x7F_FF_FF_FF)}, // dsrl32 t8, s2, 1
		{name: "dsrl32", funct: 0x3e, rd: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0xFF_FF_FF_FF_FF_FF_FF_FF), sa: 31, expectRes: Word(0x1)},          // dsrl32 t8, s2, 31
		{name: "dsrl32", funct: 0x3e, rd: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0x1_0000_0000), sa: 0, expectRes: Word(0x1)},                       // dsrl32 t8, s2, 0
		{name: "dsrl32", funct: 0x3e, rd: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0x1_0000_0000), sa: 31, expectRes: Word(0x0)},                      // dsrl32 t8, s2, 31

		{name: "dsra32", funct: 0x3f, rd: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0x1), sa: 0, expectRes: Word(0x0)},                                             // dsra32 t8, s2, 0
		{name: "dsra32", funct: 0x3f, rd: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0x1), sa: 1, expectRes: Word(0x0)},                                             // dsra32 t8, s2, 1
		{name: "dsra32", funct: 0x3f, rd: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0xFF_FF_FF_FF), sa: 0, expectRes: Word(0x0)},                                   // dsra32 t8, s2, 0
		{name: "dsra32", funct: 0x3f, rd: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0x01_FF_FF_FF_FF), sa: 0, expectRes: Word(0x1)},                                // dsra32 t8, s2, 0
		{name: "dsra32", funct: 0x3f, rd: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0xFF_FF_FF_FF_FF_FF_FF_FF), sa: 1, expectRes: Word(0xFF_FF_FF_FF_FF_FF_FF_FF)}, // dsra32 t8, s2, 1
		{name: "dsra32", funct: 0x3f, rd: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0xFF_FF_FF_00_00_00_00_00), sa: 1, expectRes: Word(0xFF_FF_FF_FF_FF_FF_FF_80)}, // dsra32 t8, s2, 1
		{name: "dsra32", funct: 0x3f, rd: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0x7F_FF_FF_FF_FF_FF_FF_FF), sa: 31, expectRes: Word(0x0)},                      // dsra32 t8, s2, 1
	}

	v := GetMultiThreadedTestCase(t)
	for i, tt := range cases {
		testName := fmt.Sprintf("%v %v", v.Name, tt.name)
		t.Run(testName, func(t *testing.T) {
			pc := Word(0x0)
			goVm := v.VMFactory(nil, os.Stdout, os.Stderr, testutil.CreateLogger(), testutil.WithRandomization(int64(i)), testutil.WithPCAndNextPC(pc))
			state := goVm.GetState()
			var insn uint32
			var rtReg uint32
			var rdReg uint32
			rtReg = 18
			rdReg = 8
			insn = rtReg<<16 | rdReg<<11 | tt.sa<<6 | tt.funct
			state.GetRegistersRef()[rdReg] = tt.rd
			state.GetRegistersRef()[rtReg] = tt.rt
			testutil.StoreInstruction(state.GetMemory(), pc, insn)
			step := state.GetStep()

			// Setup expectations
			expected := testutil.NewExpectedState(state)
			expected.ExpectStep()
			expected.Registers[rdReg] = tt.expectRes

			stepWitness, err := goVm.Step(true)
			require.NoError(t, err)

			// Check expectations
			expected.Validate(t, state)
			testutil.ValidateEVM(t, stepWitness, step, goVm, v.StateHashFn, v.Contracts)
		})
	}
}

func TestEVM_SingleStep_LoadStore64(t *testing.T) {
	t1 := Word(0xFF000000_00000108)

	cases := []loadStoreTestCase{
		{name: "lb 0", opcode: uint32(0x20), memVal: Word(0x71_72_73_74_75_76_77_78), expectRes: Word(0x71)},                                            // lb $t0, 0($t1)
		{name: "lb 1", opcode: uint32(0x20), imm: 1, memVal: Word(0x71_72_73_74_75_76_77_78), expectRes: Word(0x72)},                                    // lb $t0, 1($t1)
		{name: "lb 2", opcode: uint32(0x20), imm: 2, memVal: Word(0x71_72_73_74_75_76_77_78), expectRes: Word(0x73)},                                    // lb $t0, 2($t1)
		{name: "lb 3", opcode: uint32(0x20), imm: 3, memVal: Word(0x71_72_73_74_75_76_77_78), expectRes: Word(0x74)},                                    // lb $t0, 3($t1)
		{name: "lb 4", opcode: uint32(0x20), imm: 4, memVal: Word(0x71_72_73_74_75_76_77_78), expectRes: Word(0x75)},                                    // lb $t0, 4($t1)
		{name: "lb 5", opcode: uint32(0x20), imm: 5, memVal: Word(0x71_72_73_74_75_76_77_78), expectRes: Word(0x76)},                                    // lb $t0, 5($t1)
		{name: "lb 6", opcode: uint32(0x20), imm: 6, memVal: Word(0x71_72_73_74_75_76_77_78), expectRes: Word(0x77)},                                    // lb $t0, 6($t1)
		{name: "lb 7", opcode: uint32(0x20), imm: 7, memVal: Word(0x71_72_73_74_75_76_77_78), expectRes: Word(0x78)},                                    // lb $t0, 7($t1)
		{name: "lb sign-extended 0", opcode: uint32(0x20), memVal: Word(0x81_72_73_74_75_76_77_78), expectRes: Word(0xFF_FF_FF_FF_FF_FF_FF_81)},         // lb $t0, 0($t1)
		{name: "lb sign-extended 1", opcode: uint32(0x20), imm: 1, memVal: Word(0x71_82_73_74_75_76_77_78), expectRes: Word(0xFF_FF_FF_FF_FF_FF_FF_82)}, // lb $t0, 1($t1)
		{name: "lb sign-extended 2", opcode: uint32(0x20), imm: 2, memVal: Word(0x71_72_83_74_75_76_77_78), expectRes: Word(0xFF_FF_FF_FF_FF_FF_FF_83)}, // lb $t0, 2($t1)
		{name: "lb sign-extended 3", opcode: uint32(0x20), imm: 3, memVal: Word(0x71_72_73_84_75_76_77_78), expectRes: Word(0xFF_FF_FF_FF_FF_FF_FF_84)}, // lb $t0, 3($t1)
		{name: "lb sign-extended 4", opcode: uint32(0x20), imm: 4, memVal: Word(0x71_72_73_74_85_76_77_78), expectRes: Word(0xFF_FF_FF_FF_FF_FF_FF_85)}, // lb $t0, 4($t1)
		{name: "lb sign-extended 5", opcode: uint32(0x20), imm: 5, memVal: Word(0x71_72_73_74_75_86_77_78), expectRes: Word(0xFF_FF_FF_FF_FF_FF_FF_86)}, // lb $t0, 5($t1)
		{name: "lb sign-extended 6", opcode: uint32(0x20), imm: 6, memVal: Word(0x71_72_73_74_75_76_87_78), expectRes: Word(0xFF_FF_FF_FF_FF_FF_FF_87)}, // lb $t0, 6($t1)
		{name: "lb sign-extended 7", opcode: uint32(0x20), imm: 7, memVal: Word(0x71_72_73_74_75_76_77_88), expectRes: Word(0xFF_FF_FF_FF_FF_FF_FF_88)}, // lb $t0, 7($t1)

		{name: "lh offset=0", opcode: uint32(0x21), memVal: Word(0x11223344_55667788), expectRes: Word(0x11_22)},                                         // lhu $t0, 0($t1)
		{name: "lh offset=0 sign-extended", opcode: uint32(0x21), memVal: Word(0x81223344_55667788), expectRes: Word(0xFF_FF_FF_FF_FF_FF_81_22)},         // lhu $t0, 0($t1)
		{name: "lh offset=2", opcode: uint32(0x21), imm: 2, memVal: Word(0x11223344_55667788), expectRes: Word(0x33_44)},                                 // lhu $t0, 2($t1)
		{name: "lh offset=2 sign-extended", opcode: uint32(0x21), imm: 2, memVal: Word(0x11228344_55667788), expectRes: Word(0xFF_FF_FF_FF_FF_FF_83_44)}, // lhu $t0, 2($t1)
		{name: "lh offset=4", opcode: uint32(0x21), imm: 4, memVal: Word(0x11223344_55667788), expectRes: Word(0x55_66)},                                 // lhu $t0, 4($t1)
		{name: "lh offset=4 sign-extended", opcode: uint32(0x21), imm: 4, memVal: Word(0x11223344_85667788), expectRes: Word(0xFF_FF_FF_FF_FF_FF_85_66)}, // lhu $t0, 4($t1)
		{name: "lh offset=6", opcode: uint32(0x21), imm: 6, memVal: Word(0x11223344_55661788), expectRes: Word(0x17_88)},                                 // lhu $t0, 6($t1)
		{name: "lh offset=6 sign-extended", opcode: uint32(0x21), imm: 6, memVal: Word(0x11223344_55668788), expectRes: Word(0xFF_FF_FF_FF_FF_FF_87_88)}, // lhu $t0, 6($t1)

		{name: "lw upper", opcode: uint32(0x23), memVal: Word(0x11223344_55667788), expectRes: Word(0x11223344)},                                // lw $t0, 0($t1)
		{name: "lw upper sign-extended", opcode: uint32(0x23), memVal: Word(0x81223344_55667788), expectRes: Word(0xFFFFFFFF_81223344)},         // lw $t0, 0($t1)
		{name: "lw lower", opcode: uint32(0x23), imm: 4, memVal: Word(0x11223344_55667788), expectRes: Word(0x55667788)},                        // lw $t0, 4($t1)
		{name: "lw lower sign-extended", opcode: uint32(0x23), imm: 4, memVal: Word(0x11223344_85667788), expectRes: Word(0xFFFFFFFF_85667788)}, // lw $t0, 4($t1)

		{name: "lbu 0", opcode: uint32(0x24), memVal: Word(0x71_72_73_74_75_76_77_78), expectRes: Word(0x71)},                       // lbu $t0, 0($t1)
		{name: "lbu 1", opcode: uint32(0x24), imm: 1, memVal: Word(0x71_72_73_74_75_76_77_78), expectRes: Word(0x72)},               // lbu $t0, 1($t1)
		{name: "lbu 2", opcode: uint32(0x24), imm: 2, memVal: Word(0x71_72_73_74_75_76_77_78), expectRes: Word(0x73)},               // lbu $t0, 2($t1)
		{name: "lbu 3", opcode: uint32(0x24), imm: 3, memVal: Word(0x71_72_73_74_75_76_77_78), expectRes: Word(0x74)},               // lbu $t0, 3($t1)
		{name: "lbu 4", opcode: uint32(0x24), imm: 4, memVal: Word(0x71_72_73_74_75_76_77_78), expectRes: Word(0x75)},               // lbu $t0, 4($t1)
		{name: "lbu 5", opcode: uint32(0x24), imm: 5, memVal: Word(0x71_72_73_74_75_76_77_78), expectRes: Word(0x76)},               // lbu $t0, 5($t1)
		{name: "lbu 6", opcode: uint32(0x24), imm: 6, memVal: Word(0x71_72_73_74_75_76_77_78), expectRes: Word(0x77)},               // lbu $t0, 6($t1)
		{name: "lbu 7", opcode: uint32(0x24), imm: 7, memVal: Word(0x71_72_73_74_75_76_77_78), expectRes: Word(0x78)},               // lbu $t0, 7($t1)
		{name: "lbu sign-extended 0", opcode: uint32(0x24), memVal: Word(0x81_72_73_74_75_76_77_78), expectRes: Word(0x81)},         // lbu $t0, 0($t1)
		{name: "lbu sign-extended 1", opcode: uint32(0x24), imm: 1, memVal: Word(0x71_82_73_74_75_76_77_78), expectRes: Word(0x82)}, // lbu $t0, 1($t1)
		{name: "lbu sign-extended 2", opcode: uint32(0x24), imm: 2, memVal: Word(0x71_72_83_74_75_76_77_78), expectRes: Word(0x83)}, // lbu $t0, 2($t1)
		{name: "lbu sign-extended 3", opcode: uint32(0x24), imm: 3, memVal: Word(0x71_72_73_84_75_76_77_78), expectRes: Word(0x84)}, // lbu $t0, 3($t1)
		{name: "lbu sign-extended 4", opcode: uint32(0x24), imm: 4, memVal: Word(0x71_72_73_74_85_76_77_78), expectRes: Word(0x85)}, // lbu $t0, 4($t1)
		{name: "lbu sign-extended 5", opcode: uint32(0x24), imm: 5, memVal: Word(0x71_72_73_74_75_86_77_78), expectRes: Word(0x86)}, // lbu $t0, 5($t1)
		{name: "lbu sign-extended 6", opcode: uint32(0x24), imm: 6, memVal: Word(0x71_72_73_74_75_76_87_78), expectRes: Word(0x87)}, // lbu $t0, 6($t1)
		{name: "lbu sign-extended 7", opcode: uint32(0x24), imm: 7, memVal: Word(0x71_72_73_74_75_76_77_88), expectRes: Word(0x88)}, // lbu $t0, 7($t1)

		{name: "lhu offset=0", opcode: uint32(0x25), memVal: Word(0x11223344_55667788), expectRes: Word(0x11_22)},                       // lhu $t0, 0($t1)
		{name: "lhu offset=0 zero-extended", opcode: uint32(0x25), memVal: Word(0x81223344_55667788), expectRes: Word(0x81_22)},         // lhu $t0, 0($t1)
		{name: "lhu offset=2", opcode: uint32(0x25), imm: 2, memVal: Word(0x11223344_55667788), expectRes: Word(0x33_44)},               // lhu $t0, 2($t1)
		{name: "lhu offset=2 zero-extended", opcode: uint32(0x25), imm: 2, memVal: Word(0x11228344_55667788), expectRes: Word(0x83_44)}, // lhu $t0, 2($t1)
		{name: "lhu offset=4", opcode: uint32(0x25), imm: 4, memVal: Word(0x11223344_55667788), expectRes: Word(0x55_66)},               // lhu $t0, 4($t1)
		{name: "lhu offset=4 zero-extended", opcode: uint32(0x25), imm: 4, memVal: Word(0x11223344_85667788), expectRes: Word(0x85_66)}, // lhu $t0, 4($t1)
		{name: "lhu offset=6", opcode: uint32(0x25), imm: 6, memVal: Word(0x11223344_55661788), expectRes: Word(0x17_88)},               // lhu $t0, 6($t1)
		{name: "lhu offset=6 zero-extended", opcode: uint32(0x25), imm: 6, memVal: Word(0x11223344_55668788), expectRes: Word(0x87_88)}, // lhu $t0, 6($t1)

		{name: "lwl", opcode: uint32(0x22), rt: Word(0xaa_bb_cc_dd), imm: 4, memVal: Word(0x12_34_56_78), expectRes: Word(0x12_34_56_78)},                                                                // lwl $t0, 4($t1)
		{name: "lwl unaligned address", opcode: uint32(0x22), rt: Word(0xaa_bb_cc_dd), imm: 5, memVal: Word(0x12_34_56_78), expectRes: Word(0x34_56_78_dd)},                                              // lwl $t0, 5($t1)
		{name: "lwl offset 0 sign bit 31 set", opcode: uint32(0x22), rt: Word(0x11_22_33_44_55_66_77_88), imm: 0, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0xFF_FF_FF_FF_AA_BB_CC_DD)},   // lwl $t0, 0($t1)
		{name: "lwl offset 0 sign bit 31 clear", opcode: uint32(0x22), rt: Word(0x11_22_33_44_55_66_77_88), imm: 0, memVal: Word(0x7A_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0x00_00_00_00_7A_BB_CC_DD)}, // lwl $t0, 0($t1)
		{name: "lwl offset 1 sign bit 31 set", opcode: uint32(0x22), rt: Word(0x11_22_33_44_55_66_77_88), imm: 1, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0xFF_FF_FF_FF_BB_CC_DD_88)},   // lwl $t0, 1($t1)
		{name: "lwl offset 1 sign bit 31 clear", opcode: uint32(0x22), rt: Word(0x11_22_33_44_55_66_77_88), imm: 1, memVal: Word(0xAA_7B_CC_DD_A1_B1_C1_D1), expectRes: Word(0x00_00_00_00_7B_CC_DD_88)}, // lwl $t0, 1($t1)
		{name: "lwl offset 2 sign bit 31 set", opcode: uint32(0x22), rt: Word(0x11_22_33_44_55_66_77_88), imm: 2, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0xFF_FF_FF_FF_CC_DD_77_88)},   // lwl $t0, 2($t1)
		{name: "lwl offset 2 sign bit 31 clear", opcode: uint32(0x22), rt: Word(0x11_22_33_44_55_66_77_88), imm: 2, memVal: Word(0xAA_BB_7C_DD_A1_B1_C1_D1), expectRes: Word(0x00_00_00_00_7C_DD_77_88)}, // lwl $t0, 2($t1)
		{name: "lwl offset 3 sign bit 31 set", opcode: uint32(0x22), rt: Word(0x11_22_33_44_55_66_77_88), imm: 3, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0xFF_FF_FF_FF_DD_66_77_88)},   // lwl $t0, 3($t1)
		{name: "lwl offset 3 sign bit 31 clear", opcode: uint32(0x22), rt: Word(0x11_22_33_44_55_66_77_88), imm: 3, memVal: Word(0xAA_BB_CC_7D_A1_B1_C1_D1), expectRes: Word(0x00_00_00_00_7D_66_77_88)}, // lwl $t0, 3($t1)
		{name: "lwl offset 4 sign bit 31 set", opcode: uint32(0x22), rt: Word(0x11_22_33_44_55_66_77_88), imm: 4, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0xFF_FF_FF_FF_A1_B1_C1_D1)},   // lwl $t0, 4($t1)
		{name: "lwl offset 4 sign bit 31 clear", opcode: uint32(0x22), rt: Word(0x11_22_33_44_55_66_77_88), imm: 4, memVal: Word(0xAA_BB_CC_DD_71_B1_C1_D1), expectRes: Word(0x00_00_00_00_71_B1_C1_D1)}, // lwl $t0, 4($t1)
		{name: "lwl offset 5 sign bit 31 set", opcode: uint32(0x22), rt: Word(0x11_22_33_44_55_66_77_88), imm: 5, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0xFF_FF_FF_FF_B1_C1_D1_88)},   // lwl $t0, 5($t1)
		{name: "lwl offset 5 sign bit 31 clear", opcode: uint32(0x22), rt: Word(0x11_22_33_44_55_66_77_88), imm: 5, memVal: Word(0xAA_BB_CC_DD_A1_71_C1_D1), expectRes: Word(0x00_00_00_00_71_C1_D1_88)}, // lwl $t0, 5($t1)
		{name: "lwl offset 6 sign bit 31 set", opcode: uint32(0x22), rt: Word(0x11_22_33_44_55_66_77_88), imm: 6, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0xFF_FF_FF_FF_C1_D1_77_88)},   // lwl $t0, 6($t1)
		{name: "lwl offset 6 sign bit 31 clear", opcode: uint32(0x22), rt: Word(0x11_22_33_44_55_66_77_88), imm: 6, memVal: Word(0xAA_BB_CC_DD_A1_B1_71_D1), expectRes: Word(0x00_00_00_00_71_D1_77_88)}, // lwl $t0, 6($t1)
		{name: "lwl offset 7 sign bit 31 set", opcode: uint32(0x22), rt: Word(0x11_22_33_44_55_66_77_88), imm: 7, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0xFF_FF_FF_FF_D1_66_77_88)},   // lwl $t0, 7($t1)
		{name: "lwl offset 7 sign bit 31 clear", opcode: uint32(0x22), rt: Word(0x11_22_33_44_55_66_77_88), imm: 7, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_71), expectRes: Word(0x00_00_00_00_71_66_77_88)}, // lwl $t0, 7($t1)

		{name: "lwr zero-extended imm 0 sign bit 31 clear", opcode: uint32(0x26), rt: Word(0x11_22_33_44_55_66_77_88), imm: 0, memVal: Word(0x7A_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0x11_22_33_44_55_66_77_7A)}, // lwr $t0, 0($t1)
		{name: "lwr zero-extended imm 0 sign bit 31 set", opcode: uint32(0x26), rt: Word(0x11_22_33_44_55_66_77_88), imm: 0, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0x11_22_33_44_55_66_77_AA)},   // lwr $t0, 0($t1)
		{name: "lwr zero-extended imm 1 sign bit 31 clear", opcode: uint32(0x26), rt: Word(0x11_22_33_44_55_66_77_88), imm: 1, memVal: Word(0x7A_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0x11_22_33_44_55_66_7A_BB)}, // lwr $t0, 1($t1)
		{name: "lwr zero-extended imm 1 sign bit 31 set", opcode: uint32(0x26), rt: Word(0x11_22_33_44_55_66_77_88), imm: 1, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0x11_22_33_44_55_66_AA_BB)},   // lwr $t0, 1($t1)
		{name: "lwr zero-extended imm 2 sign bit 31 clear", opcode: uint32(0x26), rt: Word(0x11_22_33_44_55_66_77_88), imm: 2, memVal: Word(0x7A_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0x11_22_33_44_55_7A_BB_CC)}, // lwr $t0, 2($t1)
		{name: "lwr zero-extended imm 2 sign bit 31 set", opcode: uint32(0x26), rt: Word(0x11_22_33_44_55_66_77_88), imm: 2, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0x11_22_33_44_55_AA_BB_CC)},   // lwr $t0, 2($t1)
		{name: "lwr sign-extended imm 3 sign bit 31 clear", opcode: uint32(0x26), rt: Word(0x11_22_33_44_55_66_77_88), imm: 3, memVal: Word(0x7A_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0x00_00_00_00_7A_BB_CC_DD)}, // lwr $t0, 3($t1)
		{name: "lwr sign-extended imm 3 sign bit 31 set", opcode: uint32(0x26), rt: Word(0x11_22_33_44_55_66_77_88), imm: 3, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0xFF_FF_FF_FF_AA_BB_CC_DD)},   // lwr $t0, 3($t1)
		{name: "lwr zero-extended imm 4 sign bit 31 clear", opcode: uint32(0x26), rt: Word(0x11_22_33_44_55_66_77_88), imm: 4, memVal: Word(0xAA_BB_CC_DD_71_B1_C1_D1), expectRes: Word(0x11_22_33_44_55_66_77_71)}, // lwr $t0, 4($t1)
		{name: "lwr zero-extended imm 4 sign bit 31 set", opcode: uint32(0x26), rt: Word(0x11_22_33_44_85_66_77_88), imm: 4, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0x11_22_33_44_85_66_77_A1)},   // lwr $t0, 4($t1)
		{name: "lwr zero-extended imm 5 sign bit 31 clear", opcode: uint32(0x26), rt: Word(0x11_22_33_44_55_66_77_88), imm: 5, memVal: Word(0xAA_BB_CC_DD_71_B1_C1_D1), expectRes: Word(0x11_22_33_44_55_66_71_B1)}, // lwr $t0, 5($t1)
		{name: "lwr zero-extended imm 5 sign bit 31 set", opcode: uint32(0x26), rt: Word(0x11_22_33_44_85_66_77_88), imm: 5, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0x11_22_33_44_85_66_A1_B1)},   // lwr $t0, 5($t1)
		{name: "lwr zero-extended imm 6 sign bit 31 clear", opcode: uint32(0x26), rt: Word(0x11_22_33_44_55_66_77_88), imm: 6, memVal: Word(0xAA_BB_CC_DD_71_B1_C1_D1), expectRes: Word(0x11_22_33_44_55_71_B1_C1)}, // lwr $t0, 6($t1)
		{name: "lwr zero-extended imm 6 sign bit 31 set", opcode: uint32(0x26), rt: Word(0x11_22_33_44_85_66_77_88), imm: 6, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0x11_22_33_44_85_A1_B1_C1)},   // lwr $t0, 6($t1)
		{name: "lwr sign-extended imm 7 sign bit 31 clear", opcode: uint32(0x26), rt: Word(0x11_22_33_44_55_66_77_88), imm: 7, memVal: Word(0xAA_BB_CC_DD_71_B1_C1_D1), expectRes: Word(0x00_00_00_00_71_B1_C1_D1)}, // lwr $t0, 7($t1)
		{name: "lwr sign-extended imm 7 sign bit 31 set", opcode: uint32(0x26), rt: Word(0x11_22_33_44_55_66_77_88), imm: 7, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0xFF_FF_FF_FF_A1_B1_C1_D1)},   // lwr $t0, 7($t1)

		{name: "sb offset=0", opcode: uint32(0x28), memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0x11_22_33_44_55_66_77_88), imm: 0, expectMemVal: Word(0x88_BB_CC_DD_A1_B1_C1_D1)}, // sb $t0, 0($t1)
		{name: "sb offset=1", opcode: uint32(0x28), memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0x11_22_33_44_55_66_77_88), imm: 1, expectMemVal: Word(0xAA_88_CC_DD_A1_B1_C1_D1)}, // sb $t0, 1($t1)
		{name: "sb offset=2", opcode: uint32(0x28), memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0x11_22_33_44_55_66_77_88), imm: 2, expectMemVal: Word(0xAA_BB_88_DD_A1_B1_C1_D1)}, // sb $t0, 2($t1)
		{name: "sb offset=3", opcode: uint32(0x28), memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0x11_22_33_44_55_66_77_88), imm: 3, expectMemVal: Word(0xAA_BB_CC_88_A1_B1_C1_D1)}, // sb $t0, 3($t1)
		{name: "sb offset=4", opcode: uint32(0x28), memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0x11_22_33_44_55_66_77_88), imm: 4, expectMemVal: Word(0xAA_BB_CC_DD_88_B1_C1_D1)}, // sb $t0, 4($t1)
		{name: "sb offset=5", opcode: uint32(0x28), memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0x11_22_33_44_55_66_77_88), imm: 5, expectMemVal: Word(0xAA_BB_CC_DD_A1_88_C1_D1)}, // sb $t0, 5($t1)
		{name: "sb offset=6", opcode: uint32(0x28), memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0x11_22_33_44_55_66_77_88), imm: 6, expectMemVal: Word(0xAA_BB_CC_DD_A1_B1_88_D1)}, // sb $t0, 6($t1)
		{name: "sb offset=7", opcode: uint32(0x28), memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0x11_22_33_44_55_66_77_88), imm: 7, expectMemVal: Word(0xAA_BB_CC_DD_A1_B1_C1_88)}, // sb $t0, 7($t1)

		{name: "sh offset=0", opcode: uint32(0x29), memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0x11_22_33_44_55_66_77_88), imm: 0, expectMemVal: Word(0x77_88_CC_DD_A1_B1_C1_D1)}, // sh $t0, 0($t1)
		{name: "sh offset=2", opcode: uint32(0x29), memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0x11_22_33_44_55_66_77_88), imm: 2, expectMemVal: Word(0xAA_BB_77_88_A1_B1_C1_D1)}, // sh $t0, 2($t1)
		{name: "sh offset=4", opcode: uint32(0x29), memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0x11_22_33_44_55_66_77_88), imm: 4, expectMemVal: Word(0xAA_BB_CC_DD_77_88_C1_D1)}, // sh $t0, 4($t1)
		{name: "sh offset=6", opcode: uint32(0x29), memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), rt: Word(0x11_22_33_44_55_66_77_88), imm: 6, expectMemVal: Word(0xAA_BB_CC_DD_A1_B1_77_88)}, // sh $t0, 6($t1)

		{name: "swl offset=0", opcode: uint32(0x2a), rt: Word(0x11_22_33_44_55_66_77_88), memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), imm: 0, expectMemVal: Word(0x55_66_77_88_A1_B1_C1_D1)}, //  swl $t0, 0($t1)
		{name: "swl offset=1", opcode: uint32(0x2a), rt: Word(0x11_22_33_44_55_66_77_88), memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), imm: 1, expectMemVal: Word(0xAA_55_66_77_A1_B1_C1_D1)}, //  swl $t0, 1($t1)
		{name: "swl offset=2", opcode: uint32(0x2a), rt: Word(0x11_22_33_44_55_66_77_88), memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), imm: 2, expectMemVal: Word(0xAA_BB_55_66_A1_B1_C1_D1)}, //  swl $t0, 2($t1)
		{name: "swl offset=3", opcode: uint32(0x2a), rt: Word(0x11_22_33_44_55_66_77_88), memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), imm: 3, expectMemVal: Word(0xAA_BB_CC_55_A1_B1_C1_D1)}, //  swl $t0, 3($t1)
		{name: "swl offset=4", opcode: uint32(0x2a), rt: Word(0x11_22_33_44_55_66_77_88), memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), imm: 4, expectMemVal: Word(0xAA_BB_CC_DD_55_66_77_88)}, //  swl $t0, 4($t1)
		{name: "swl offset=5", opcode: uint32(0x2a), rt: Word(0x11_22_33_44_55_66_77_88), memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), imm: 5, expectMemVal: Word(0xAA_BB_CC_DD_A1_55_66_77)}, //  swl $t0, 5($t1)
		{name: "swl offset=6", opcode: uint32(0x2a), rt: Word(0x11_22_33_44_55_66_77_88), memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), imm: 6, expectMemVal: Word(0xAA_BB_CC_DD_A1_B1_55_66)}, //  swl $t0, 6($t1)
		{name: "swl offset=7", opcode: uint32(0x2a), rt: Word(0x11_22_33_44_55_66_77_88), memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), imm: 7, expectMemVal: Word(0xAA_BB_CC_DD_A1_B1_C1_55)}, //  swl $t0, 7($t1)

		{name: "sw offset=0", opcode: uint32(0x2b), rt: Word(0x11_22_33_44_55_66_77_88), memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), imm: 0, expectMemVal: Word(0x55_66_77_88_A1_B1_C1_D1)}, // sw $t0, 0($t1)
		{name: "sw offset=4", opcode: uint32(0x2b), rt: Word(0x11_22_33_44_55_66_77_88), memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), imm: 4, expectMemVal: Word(0xAA_BB_CC_DD_55_66_77_88)}, // sw $t0, 4($t1)

		{name: "swr offset=0", opcode: uint32(0x2e), rt: Word(0x11_22_33_44_55_66_77_88), imm: 0, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectMemVal: Word(0x88_BB_CC_DD_A1_B1_C1_D1)}, // swr $t0, 0($t1)
		{name: "swr offset=1", opcode: uint32(0x2e), rt: Word(0x11_22_33_44_55_66_77_88), imm: 1, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectMemVal: Word(0x77_88_CC_DD_A1_B1_C1_D1)}, // swr $t0, 1($t1)
		{name: "swr offset=2", opcode: uint32(0x2e), rt: Word(0x11_22_33_44_55_66_77_88), imm: 2, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectMemVal: Word(0x66_77_88_DD_A1_B1_C1_D1)}, // swr $t0, 2($t1)
		{name: "swr offset=3", opcode: uint32(0x2e), rt: Word(0x11_22_33_44_55_66_77_88), imm: 3, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectMemVal: Word(0x55_66_77_88_A1_B1_C1_D1)}, // swr $t0, 3($t1)
		{name: "swr offset=4", opcode: uint32(0x2e), rt: Word(0x11_22_33_44_55_66_77_88), imm: 4, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectMemVal: Word(0xAA_BB_CC_DD_88_B1_C1_D1)}, // swr $t0, 4($t1)
		{name: "swr offset=5", opcode: uint32(0x2e), rt: Word(0x11_22_33_44_55_66_77_88), imm: 5, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectMemVal: Word(0xAA_BB_CC_DD_77_88_C1_D1)}, // swr $t0, 5($t1)
		{name: "swr offset=6", opcode: uint32(0x2e), rt: Word(0x11_22_33_44_55_66_77_88), imm: 6, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectMemVal: Word(0xAA_BB_CC_DD_66_77_88_D1)}, // swr $t0, 6($t1)
		{name: "swr offset=7", opcode: uint32(0x2e), rt: Word(0x11_22_33_44_55_66_77_88), imm: 7, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectMemVal: Word(0xAA_BB_CC_DD_55_66_77_88)}, // swr $t0, 7($t1)

		// 64-bit instructions
		{name: "ldl offset 0 sign bit 31 set", opcode: uint32(0x1A), rt: Word(0x11_22_33_44_55_66_77_88), imm: 0, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0xAA_BB_CC_DD_A1_B1_C1_D1)},   // ldl $t0, 0($t1)
		{name: "ldl offset 1 sign bit 31 set", opcode: uint32(0x1A), rt: Word(0x11_22_33_44_55_66_77_88), imm: 1, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0xBB_CC_DD_A1_B1_C1_D1_88)},   // ldl $t0, 1($t1)
		{name: "ldl offset 2 sign bit 31 set", opcode: uint32(0x1A), rt: Word(0x11_22_33_44_55_66_77_88), imm: 2, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0xCC_DD_A1_B1_C1_D1_77_88)},   // ldl $t0, 2($t1)
		{name: "ldl offset 3 sign bit 31 set", opcode: uint32(0x1A), rt: Word(0x11_22_33_44_55_66_77_88), imm: 3, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0xDD_A1_B1_C1_D1_66_77_88)},   // ldl $t0, 3($t1)
		{name: "ldl offset 4 sign bit 31 set", opcode: uint32(0x1A), rt: Word(0x11_22_33_44_55_66_77_88), imm: 4, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0xA1_B1_C1_D1_55_66_77_88)},   // ldl $t0, 4($t1)
		{name: "ldl offset 5 sign bit 31 set", opcode: uint32(0x1A), rt: Word(0x11_22_33_44_55_66_77_88), imm: 5, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0xB1_C1_D1_44_55_66_77_88)},   // ldl $t0, 5($t1)
		{name: "ldl offset 6 sign bit 31 set", opcode: uint32(0x1A), rt: Word(0x11_22_33_44_55_66_77_88), imm: 6, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0xC1_D1_33_44_55_66_77_88)},   // ldl $t0, 6($t1)
		{name: "ldl offset 7 sign bit 31 set", opcode: uint32(0x1A), rt: Word(0x11_22_33_44_55_66_77_88), imm: 7, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0xD1_22_33_44_55_66_77_88)},   // ldl $t0, 7($t1)
		{name: "ldl offset 0 sign bit 31 clear", opcode: uint32(0x1A), rt: Word(0x11_22_33_44_55_66_77_88), imm: 0, memVal: Word(0x7A_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0x7A_BB_CC_DD_A1_B1_C1_D1)}, // ldl $t0, 0($t1)
		{name: "ldl offset 1 sign bit 31 clear", opcode: uint32(0x1A), rt: Word(0x11_22_33_44_55_66_77_88), imm: 1, memVal: Word(0xAA_7B_CC_DD_A1_B1_C1_D1), expectRes: Word(0x7B_CC_DD_A1_B1_C1_D1_88)}, // ldl $t0, 1($t1)
		{name: "ldl offset 2 sign bit 31 clear", opcode: uint32(0x1A), rt: Word(0x11_22_33_44_55_66_77_88), imm: 2, memVal: Word(0xAA_BB_7C_DD_A1_B1_C1_D1), expectRes: Word(0x7C_DD_A1_B1_C1_D1_77_88)}, // ldl $t0, 2($t1)
		{name: "ldl offset 3 sign bit 31 clear", opcode: uint32(0x1A), rt: Word(0x11_22_33_44_55_66_77_88), imm: 3, memVal: Word(0xAA_BB_CC_7D_A1_B1_C1_D1), expectRes: Word(0x7D_A1_B1_C1_D1_66_77_88)}, // ldl $t0, 3($t1)
		{name: "ldl offset 4 sign bit 31 clear", opcode: uint32(0x1A), rt: Word(0x11_22_33_44_55_66_77_88), imm: 4, memVal: Word(0xAA_BB_CC_DD_71_B1_C1_D1), expectRes: Word(0x71_B1_C1_D1_55_66_77_88)}, // ldl $t0, 4($t1)
		{name: "ldl offset 5 sign bit 31 clear", opcode: uint32(0x1A), rt: Word(0x11_22_33_44_55_66_77_88), imm: 5, memVal: Word(0xAA_BB_CC_DD_A1_71_C1_D1), expectRes: Word(0x71_C1_D1_44_55_66_77_88)}, // ldl $t0, 5($t1)
		{name: "ldl offset 6 sign bit 31 clear", opcode: uint32(0x1A), rt: Word(0x11_22_33_44_55_66_77_88), imm: 6, memVal: Word(0xAA_BB_CC_DD_A1_B1_71_D1), expectRes: Word(0x71_D1_33_44_55_66_77_88)}, // ldl $t0, 6($t1)
		{name: "ldl offset 7 sign bit 31 clear", opcode: uint32(0x1A), rt: Word(0x11_22_33_44_55_66_77_88), imm: 7, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_71), expectRes: Word(0x71_22_33_44_55_66_77_88)}, // ldl $t0, 7($t1)

		{name: "ldr offset 0 sign bit clear", opcode: uint32(0x1b), rt: Word(0x11_22_33_44_55_66_77_88), imm: 0, memVal: Word(0x3A_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0x11_22_33_44_55_66_77_3A)}, // ldr $t0, 0($t1)
		{name: "ldr offset 1 sign bit clear", opcode: uint32(0x1b), rt: Word(0x11_22_33_44_55_66_77_88), imm: 1, memVal: Word(0x3A_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0x11_22_33_44_55_66_3A_BB)}, // ldr $t0, 1($t1)
		{name: "ldr offset 2 sign bit clear", opcode: uint32(0x1b), rt: Word(0x11_22_33_44_55_66_77_88), imm: 2, memVal: Word(0x3A_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0x11_22_33_44_55_3A_BB_CC)}, // ldr $t0, 2($t1)
		{name: "ldr offset 3 sign bit clear", opcode: uint32(0x1b), rt: Word(0x11_22_33_44_55_66_77_88), imm: 3, memVal: Word(0x3A_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0x11_22_33_44_3A_BB_CC_DD)}, // ldr $t0, 3($t1)
		{name: "ldr offset 4 sign bit clear", opcode: uint32(0x1b), rt: Word(0x11_22_33_44_55_66_77_88), imm: 4, memVal: Word(0x3A_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0x11_22_33_3A_BB_CC_DD_A1)}, // ldr $t0, 4($t1)
		{name: "ldr offset 5 sign bit clear", opcode: uint32(0x1b), rt: Word(0x11_22_33_44_55_66_77_88), imm: 5, memVal: Word(0x3A_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0x11_22_3A_BB_CC_DD_A1_B1)}, // ldr $t0, 5($t1)
		{name: "ldr offset 6 sign bit clear", opcode: uint32(0x1b), rt: Word(0x11_22_33_44_55_66_77_88), imm: 6, memVal: Word(0x3A_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0x11_3A_BB_CC_DD_A1_B1_C1)}, // ldr $t0, 6($t1)
		{name: "ldr offset 7 sign bit clear", opcode: uint32(0x1b), rt: Word(0x11_22_33_44_55_66_77_88), imm: 7, memVal: Word(0x3A_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0x3A_BB_CC_DD_A1_B1_C1_D1)}, // ldr $t0, 7($t1)
		{name: "ldr offset 0 sign bit set", opcode: uint32(0x1b), rt: Word(0x11_22_33_44_55_66_77_88), imm: 0, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0x11_22_33_44_55_66_77_AA)},   // ldr $t0, 0($t1)
		{name: "ldr offset 1 sign bit set", opcode: uint32(0x1b), rt: Word(0x11_22_33_44_55_66_77_88), imm: 1, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0x11_22_33_44_55_66_AA_BB)},   // ldr $t0, 1($t1)
		{name: "ldr offset 2 sign bit set", opcode: uint32(0x1b), rt: Word(0x11_22_33_44_55_66_77_88), imm: 2, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0x11_22_33_44_55_AA_BB_CC)},   // ldr $t0, 2($t1)
		{name: "ldr offset 3 sign bit set", opcode: uint32(0x1b), rt: Word(0x11_22_33_44_55_66_77_88), imm: 3, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0x11_22_33_44_AA_BB_CC_DD)},   // ldr $t0, 3($t1)
		{name: "ldr offset 4 sign bit set", opcode: uint32(0x1b), rt: Word(0x11_22_33_44_55_66_77_88), imm: 4, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0x11_22_33_AA_BB_CC_DD_A1)},   // ldr $t0, 4($t1)
		{name: "ldr offset 5 sign bit set", opcode: uint32(0x1b), rt: Word(0x11_22_33_44_55_66_77_88), imm: 5, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0x11_22_AA_BB_CC_DD_A1_B1)},   // ldr $t0, 5($t1)
		{name: "ldr offset 6 sign bit set", opcode: uint32(0x1b), rt: Word(0x11_22_33_44_55_66_77_88), imm: 6, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0x11_AA_BB_CC_DD_A1_B1_C1)},   // ldr $t0, 6($t1)
		{name: "ldr offset 7 sign bit set", opcode: uint32(0x1b), rt: Word(0x11_22_33_44_55_66_77_88), imm: 7, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0xAA_BB_CC_DD_A1_B1_C1_D1)},   // ldr $t0, 7($t1)

		{name: "lwu upper", opcode: uint32(0x27), memVal: Word(0x11223344_55667788), expectRes: Word(0x11223344)},              // lw $t0, 0($t1)
		{name: "lwu upper sign", opcode: uint32(0x27), memVal: Word(0x81223344_55667788), expectRes: Word(0x81223344)},         // lw $t0, 0($t1)
		{name: "lwu lower", opcode: uint32(0x27), imm: 4, memVal: Word(0x11223344_55667788), expectRes: Word(0x55667788)},      // lw $t0, 4($t1)
		{name: "lwu lower sign", opcode: uint32(0x27), imm: 4, memVal: Word(0x11223344_85667788), expectRes: Word(0x85667788)}, // lw $t0, 4($t1)

		{name: "sdl offset=0", opcode: uint32(0x2c), rt: Word(0x11_22_33_44_55_66_77_88), memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), imm: 0, expectMemVal: Word(0x11_22_33_44_55_66_77_88)}, //  sdl $t0, 0($t1)
		{name: "sdl offset=1", opcode: uint32(0x2c), rt: Word(0x11_22_33_44_55_66_77_88), memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), imm: 1, expectMemVal: Word(0xAA_11_22_33_44_55_66_77)}, //  sdl $t0, 1($t1)
		{name: "sdl offset=2", opcode: uint32(0x2c), rt: Word(0x11_22_33_44_55_66_77_88), memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), imm: 2, expectMemVal: Word(0xAA_BB_11_22_33_44_55_66)}, //  sdl $t0, 2($t1)
		{name: "sdl offset=3", opcode: uint32(0x2c), rt: Word(0x11_22_33_44_55_66_77_88), memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), imm: 3, expectMemVal: Word(0xAA_BB_CC_11_22_33_44_55)}, //  sdl $t0, 3($t1)
		{name: "sdl offset=4", opcode: uint32(0x2c), rt: Word(0x11_22_33_44_55_66_77_88), memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), imm: 4, expectMemVal: Word(0xAA_BB_CC_DD_11_22_33_44)}, //  sdl $t0, 4($t1)
		{name: "sdl offset=5", opcode: uint32(0x2c), rt: Word(0x11_22_33_44_55_66_77_88), memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), imm: 5, expectMemVal: Word(0xAA_BB_CC_DD_A1_11_22_33)}, //  sdl $t0, 5($t1)
		{name: "sdl offset=6", opcode: uint32(0x2c), rt: Word(0x11_22_33_44_55_66_77_88), memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), imm: 6, expectMemVal: Word(0xAA_BB_CC_DD_A1_B1_11_22)}, //  sdl $t0, 6($t1)
		{name: "sdl offset=7", opcode: uint32(0x2c), rt: Word(0x11_22_33_44_55_66_77_88), memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), imm: 7, expectMemVal: Word(0xAA_BB_CC_DD_A1_B1_C1_11)}, //  sdl $t0, 7($t1)

		{name: "sdr offset=0", opcode: uint32(0x2d), rt: Word(0x11_22_33_44_55_66_77_88), imm: 0, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectMemVal: Word(0x88_BB_CC_DD_A1_B1_C1_D1)}, // sdr $t0, 0($t1)
		{name: "sdr offset=1", opcode: uint32(0x2d), rt: Word(0x11_22_33_44_55_66_77_88), imm: 1, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectMemVal: Word(0x77_88_CC_DD_A1_B1_C1_D1)}, // sdr $t0, 1($t1)
		{name: "sdr offset=2", opcode: uint32(0x2d), rt: Word(0x11_22_33_44_55_66_77_88), imm: 2, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectMemVal: Word(0x66_77_88_DD_A1_B1_C1_D1)}, // sdr $t0, 2($t1)
		{name: "sdr offset=3", opcode: uint32(0x2d), rt: Word(0x11_22_33_44_55_66_77_88), imm: 3, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectMemVal: Word(0x55_66_77_88_A1_B1_C1_D1)}, // sdr $t0, 3($t1)
		{name: "sdr offset=4", opcode: uint32(0x2d), rt: Word(0x11_22_33_44_55_66_77_88), imm: 4, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectMemVal: Word(0x44_55_66_77_88_B1_C1_D1)}, // sdr $t0, 4($t1)
		{name: "sdr offset=5", opcode: uint32(0x2d), rt: Word(0x11_22_33_44_55_66_77_88), imm: 5, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectMemVal: Word(0x33_44_55_66_77_88_C1_D1)}, // sdr $t0, 5($t1)
		{name: "sdr offset=6", opcode: uint32(0x2d), rt: Word(0x11_22_33_44_55_66_77_88), imm: 6, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectMemVal: Word(0x22_33_44_55_66_77_88_D1)}, // sdr $t0, 6($t1)
		{name: "sdr offset=7", opcode: uint32(0x2d), rt: Word(0x11_22_33_44_55_66_77_88), imm: 7, memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectMemVal: Word(0x11_22_33_44_55_66_77_88)}, // sdr $t0, 7($t1)

		{name: "ld", opcode: uint32(0x37), rt: Word(0x11_22_33_44_55_66_77_88), memVal: Word(0x7A_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0x7A_BB_CC_DD_A1_B1_C1_D1)},        // ld $t0, 0($t1)
		{name: "ld signed", opcode: uint32(0x37), rt: Word(0x11_22_33_44_55_66_77_88), memVal: Word(0x8A_BB_CC_DD_A1_B1_C1_D1), expectRes: Word(0x8A_BB_CC_DD_A1_B1_C1_D1)}, // ld $t0, 0($t1)

		{name: "sd", opcode: uint32(0x3f), rt: Word(0x11_22_33_44_55_66_77_88), memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectMemVal: Word(0x11_22_33_44_55_66_77_88)},        // sd $t0, 0($t1)
		{name: "sd signed", opcode: uint32(0x3f), rt: Word(0x81_22_33_44_55_66_77_88), memVal: Word(0xAA_BB_CC_DD_A1_B1_C1_D1), expectMemVal: Word(0x81_22_33_44_55_66_77_88)}, // sd $t0, 4($t1)
	}
	// use a fixed base for all tests
	for i := range cases {
		cases[i].base = t1
	}
	testLoadStore(t, cases)
}

func TestEVM_SingleStep_MulDiv64(t *testing.T) {
	cases := []mulDivTestCase{
		// dmult s1, s2
		// expected hi,lo were verified using qemu-mips
		{name: "dmult 0", funct: 0x1c, rs: 0, rt: 0, expectLo: 0, expectHi: 0},
		{name: "dmult 1", funct: 0x1c, rs: 1, rt: 1, expectLo: 1, expectHi: 0},
		{name: "dmult 2", funct: 0x1c, rs: 0x01_00_00_00_00, rt: 2, expectLo: 0x02_00_00_00_00, expectHi: 0},
		{name: "dmult 3", funct: 0x1c, rs: 0x01_00_00_00_00_00_00_00, rt: 2, expectLo: 0x02_00_00_00_00_00_00_00, expectHi: 0},
		{name: "dmult 4", funct: 0x1c, rs: 0x40_00_00_00_00_00_00_00, rt: 2, expectLo: 0x80_00_00_00_00_00_00_00, expectHi: 0x0},
		{name: "dmult 5", funct: 0x1c, rs: 0x40_00_00_00_00_00_00_00, rt: 0x1000, expectLo: 0x0, expectHi: 0x4_00},
		{name: "dmult 6", funct: 0x1c, rs: 0x80_00_00_00_00_00_00_00, rt: 0x1000, expectLo: 0x0, expectHi: 0xFF_FF_FF_FF_FF_FF_F8_00},
		{name: "dmult 7", funct: 0x1c, rs: 0x80_00_00_00_00_00_00_00, rt: 0x80_00_00_00_00_00_00_00, expectLo: 0x0, expectHi: 0x40_00_00_00_00_00_00_00},
		{name: "dmult 8", funct: 0x1c, rs: 0x40_00_00_00_00_00_00_01, rt: 0x1000, expectLo: 0x1000, expectHi: 0x4_00},
		{name: "dmult 9", funct: 0x1c, rs: 0x80_00_00_00_00_00_00_80, rt: 0x80_00_00_00_00_00_00_80, expectLo: 0x4000, expectHi: 0x3F_FF_FF_FF_FF_FF_FF_80},
		{name: "dmult 10", funct: 0x1c, rs: Word(0xFF_FF_FF_FF_FF_FF_FF_FF), rt: Word(0x1), expectLo: 0xFF_FF_FF_FF_FF_FF_FF_FF, expectHi: 0xFF_FF_FF_FF_FF_FF_FF_FF},
		{name: "dmult 11", funct: 0x1c, rs: Word(0xFF_FF_FF_FF_FF_FF_FF_FF), rt: Word(0xFF_FF_FF_FF_FF_FF_FF_FF), expectLo: 0x1, expectHi: Word(0)},
		{name: "dmult 12", funct: 0x1c, rs: Word(0xFF_FF_FF_FF_FF_FF_FF_D3), rt: Word(0xAA_BB_CC_DD_A1_D1_C1_E0), expectLo: 0xFC_FC_FD_0A_8E_20_EB_A0, expectHi: 0x00_00_00_00_00_00_00_0E},
		{name: "dmult 13", funct: 0x1c, rs: Word(0x7F_FF_FF_FF_FF_FF_FF_FF), rt: Word(0xAA_BB_CC_DD_A1_D1_C1_E1), expectLo: 0xD5_44_33_22_5E_2E_3E_1F, expectHi: 0xD5_5D_E6_6E_D0_E8_E0_F0},
		{name: "dmult 14", funct: 0x1c, rs: Word(0x7F_FF_FF_FF_FF_FF_FF_FF), rt: Word(0x8F_FF_FF_FF_FF_FF_FF_FF), expectLo: 0xF0_00_00_00_00_00_00_01, expectHi: 0xC7_FF_FF_FF_FF_FF_FF_FF},

		// dmultu s1, s2
		{name: "dmultu 0", funct: 0x1d, rs: 0, rt: 0, expectLo: 0, expectHi: 0},
		{name: "dmultu 1", funct: 0x1d, rs: 1, rt: 1, expectLo: 1, expectHi: 0},
		{name: "dmultu 2", funct: 0x1d, rs: 0x01_00_00_00_00, rt: 2, expectLo: 0x02_00_00_00_00, expectHi: 0},
		{name: "dmultu 3", funct: 0x1d, rs: 0x01_00_00_00_00_00_00_00, rt: 2, expectLo: 0x02_00_00_00_00_00_00_00, expectHi: 0},
		{name: "dmultu 4", funct: 0x1d, rs: 0x40_00_00_00_00_00_00_00, rt: 2, expectLo: 0x80_00_00_00_00_00_00_00, expectHi: 0x0},
		{name: "dmultu 5", funct: 0x1d, rs: 0x40_00_00_00_00_00_00_00, rt: 0x1000, expectLo: 0x0, expectHi: 0x4_00},
		{name: "dmultu 6", funct: 0x1d, rs: 0x80_00_00_00_00_00_00_00, rt: 0x1000, expectLo: 0x0, expectHi: 0x8_00},
		{name: "dmultu 7", funct: 0x1d, rs: 0x80_00_00_00_00_00_00_00, rt: 0x80_00_00_00_00_00_00_00, expectLo: 0x0, expectHi: 0x40_00_00_00_00_00_00_00},
		{name: "dmultu 8", funct: 0x1d, rs: 0x40_00_00_00_00_00_00_01, rt: 0x1000, expectLo: 0x1000, expectHi: 0x4_00},
		{name: "dmultu 9", funct: 0x1d, rs: 0x80_00_00_00_00_00_00_80, rt: 0x80_00_00_00_00_00_00_80, expectLo: 0x4000, expectHi: 0x40_00_00_00_00_00_00_80},
		{name: "dmultu 10", funct: 0x1d, rs: Word(0xFF_FF_FF_FF_FF_FF_FF_FF), rt: Word(0xFF_FF_FF_FF_FF_FF_FF_FF), expectLo: 0x1, expectHi: Word(0xFF_FF_FF_FF_FF_FF_FF_FE)},
		{name: "dmultu 11", funct: 0x1d, rs: Word(0xFF_FF_FF_FF_FF_FF_FF_FF), rt: Word(0xFF_FF_FF_FF_FF_FF_FF_FF), expectLo: 0x1, expectHi: 0xFF_FF_FF_FF_FF_FF_FF_FE},
		{name: "dmultu 12", funct: 0x1d, rs: Word(0xFF_FF_FF_FF_FF_FF_FF_D3), rt: Word(0xAA_BB_CC_DD_A1_D1_C1_E0), expectLo: 0xFC_FC_FD_0A_8E_20_EB_A0, expectHi: 0xAA_BB_CC_DD_A1_D1_C1_C1},
		{name: "dmultu 13", funct: 0x1d, rs: Word(0x7F_FF_FF_FF_FF_FF_FF_FF), rt: Word(0xAA_BB_CC_DD_A1_D1_C1_E1), expectLo: 0xD5_44_33_22_5E_2E_3E_1F, expectHi: 0x55_5D_E6_6E_D0_E8_E0_EF},
		{name: "dmultu 14", funct: 0x1d, rs: Word(0x7F_FF_FF_FF_FF_FF_FF_FF), rt: Word(0x8F_FF_FF_FF_FF_FF_FF_FF), expectLo: 0xF0_00_00_00_00_00_00_01, expectHi: 0x47_FF_FF_FF_FF_FF_FF_FE},

		// ddiv rs, rt
		{name: "ddiv", funct: 0x1e, rs: 0, rt: 0, panicMsg: "instruction divide by zero", revertMsg: "division by zero"},
		{name: "ddiv", funct: 0x1e, rs: 1, rt: 0, panicMsg: "instruction divide by zero", revertMsg: "division by zero"},
		{name: "ddiv", funct: 0x1e, rs: 0xFF_FF_FF_FF_FF_FF_FF_FF, rt: 0, panicMsg: "instruction divide by zero", revertMsg: "division by zero"},
		{name: "ddiv", funct: 0x1e, rs: 0, rt: 1, expectLo: 0, expectHi: 0},
		{name: "ddiv", funct: 0x1e, rs: 1, rt: 1, expectLo: 1, expectHi: 0},
		{name: "ddiv", funct: 0x1e, rs: 10, rt: 3, expectLo: 3, expectHi: 1},
		{name: "ddiv", funct: 0x1e, rs: 0x7F_FF_FF_FF_00_00_00_00, rt: 2, expectLo: 0x3F_FF_FF_FF_80_00_00_00, expectHi: 0},
		{name: "ddiv", funct: 0x1e, rs: 0xFF_FF_FF_FF_00_00_00_00, rt: 2, expectLo: 0xFF_FF_FF_FF_80_00_00_00, expectHi: 0},
		{name: "ddiv", funct: 0x1e, rs: ^Word(0), rt: ^Word(0), expectLo: 1, expectHi: 0},
		{name: "ddiv", funct: 0x1e, rs: ^Word(0), rt: 2, expectLo: 0, expectHi: ^Word(0)},
		{name: "ddiv", funct: 0x1e, rs: 0x7F_FF_FF_FF_00_00_00_00, rt: ^Word(0), expectLo: 0x80_00_00_01_00_00_00_00, expectHi: 0},

		// ddivu
		{name: "ddivu", funct: 0x1f, rs: 0, rt: 0, panicMsg: "instruction divide by zero", revertMsg: "division by zero"},
		{name: "ddivu", funct: 0x1f, rs: 1, rt: 0, panicMsg: "instruction divide by zero", revertMsg: "division by zero"},
		{name: "ddivu", funct: 0x1f, rs: 0xFF_FF_FF_FF_FF_FF_FF_FF, rt: 0, panicMsg: "instruction divide by zero", revertMsg: "division by zero"},
		{name: "ddivu", funct: 0x1f, rs: 0, rt: 1, expectLo: 0, expectHi: 0},
		{name: "ddivu", funct: 0x1f, rs: 1, rt: 1, expectLo: 1, expectHi: 0},
		{name: "ddivu", funct: 0x1f, rs: 10, rt: 3, expectLo: 3, expectHi: 1},
		{name: "ddivu", funct: 0x1f, rs: 0x7F_FF_FF_FF_00_00_00_00, rt: 2, expectLo: 0x3F_FF_FF_FF_80_00_00_00, expectHi: 0},
		{name: "ddivu", funct: 0x1f, rs: 0xFF_FF_FF_FF_00_00_00_00, rt: 2, expectLo: 0x7F_FF_FF_FF_80_00_00_00, expectHi: 0},
		{name: "ddivu", funct: 0x1f, rs: ^Word(0), rt: ^Word(0), expectLo: 1, expectHi: 0},
		{name: "ddivu", funct: 0x1f, rs: ^Word(0), rt: 2, expectLo: 0x7F_FF_FF_FF_FF_FF_FF_FF, expectHi: 1},
		{name: "ddivu", funct: 0x1f, rs: 0x7F_FF_FF_FF_00_00_00_00, rt: ^Word(0), expectLo: 0, expectHi: 0x7F_FF_FF_FF_00_00_00_00},

		// a couple div/divu 64-bit edge cases
		{name: "div lower word zero", funct: 0x1a, rs: 1, rt: 0xFF_FF_FF_FF_00_00_00_00, panicMsg: "instruction divide by zero", revertMsg: "division by zero"},
		{name: "divu lower word zero", funct: 0x1b, rs: 1, rt: 0xFF_FF_FF_FF_00_00_00_00, panicMsg: "instruction divide by zero", revertMsg: "division by zero"},
	}

	testMulDiv(t, cases, false)
}

func TestEVM_SingleStep_Branch64(t *testing.T) {
	t.Parallel()
	cases := []branchTestCase{
		// blez
		{name: "blez", pc: 0, opcode: 0x6, rs: 0x5, offset: 0x100, expectNextPC: 0x8},
		{name: "blez large rs", pc: 0x10, opcode: 0x6, rs: 0x7F_FF_FF_FF_FF_FF_FF_FF, offset: 0x100, expectNextPC: 0x18},
		{name: "blez zero rs", pc: 0x10, opcode: 0x6, rs: 0x0, offset: 0x100, expectNextPC: 0x414},
		{name: "blez sign rs", pc: 0x10, opcode: 0x6, rs: -1, offset: 0x100, expectNextPC: 0x414},
		{name: "blez rs only sign bit set", pc: 0x10, opcode: 0x6, rs: testutil.ToSignedInteger(0x80_00_00_00_00_00_00_00), offset: 0x100, expectNextPC: 0x414},
		{name: "blez sign-extended offset", pc: 0x10, opcode: 0x6, rs: -1, offset: 0x80_00, expectNextPC: 0xFF_FF_FF_FF_FF_FE_00_14},

		// bgtz
		{name: "bgtz", pc: 0, opcode: 0x7, rs: 0x5, offset: 0x100, expectNextPC: 0x404},
		{name: "bgtz sign-extended offset", pc: 0x10, opcode: 0x7, rs: 0x5, offset: 0x80_00, expectNextPC: 0xFF_FF_FF_FF_FF_FE_00_14},
		{name: "bgtz large rs", pc: 0x10, opcode: 0x7, rs: 0x7F_FF_FF_FF_FF_FF_FF_FF, offset: 0x100, expectNextPC: 0x414},
		{name: "bgtz zero rs", pc: 0x10, opcode: 0x7, rs: 0x0, offset: 0x100, expectNextPC: 0x18},
		{name: "bgtz sign rs", pc: 0x10, opcode: 0x7, rs: -1, offset: 0x100, expectNextPC: 0x18},
		{name: "bgtz rs only sign bit set", pc: 0x10, opcode: 0x7, rs: testutil.ToSignedInteger(0x80_00_00_00_00_00_00_00), offset: 0x100, expectNextPC: 0x18},

		// bltz t0, $x
		{name: "bltz", pc: 0, opcode: 0x1, regimm: 0x0, rs: 0x5, offset: 0x100, expectNextPC: 0x8},
		{name: "bltz large rs", pc: 0x10, opcode: 0x1, regimm: 0x0, rs: 0x7F_FF_FF_FF_FF_FF_FF_FF, offset: 0x100, expectNextPC: 0x18},
		{name: "bltz zero rs", pc: 0x10, opcode: 0x1, regimm: 0x0, rs: 0x0, offset: 0x100, expectNextPC: 0x18},
		{name: "bltz sign rs", pc: 0x10, opcode: 0x1, regimm: 0x0, rs: -1, offset: 0x100, expectNextPC: 0x414},
		{name: "bltz rs only sign bit set", pc: 0x10, opcode: 0x1, regimm: 0x0, rs: testutil.ToSignedInteger(0x80_00_00_00_00_00_00_00), offset: 0x100, expectNextPC: 0x414},
		{name: "bltz sign-extended offset", pc: 0x10, opcode: 0x1, regimm: 0x0, rs: -1, offset: 0x80_00, expectNextPC: 0xFF_FF_FF_FF_FF_FE_00_14},
		{name: "bltz large offset no-sign", pc: 0x10, opcode: 0x1, regimm: 0x0, rs: -1, offset: 0x7F_FF, expectNextPC: 0x2_00_10},

		// bltzal t0, $x
		{name: "bltzal", pc: 0, opcode: 0x1, regimm: 0x10, rs: 0x5, offset: 0x100, expectNextPC: 0x8, expectLink: true},
		{name: "bltzal large rs", pc: 0x10, opcode: 0x1, regimm: 0x10, rs: 0x7F_FF_FF_FF_FF_FF_FF_FF, offset: 0x100, expectNextPC: 0x18, expectLink: true},
		{name: "bltzal zero rs", pc: 0x10, opcode: 0x1, regimm: 0x10, rs: 0x0, offset: 0x100, expectNextPC: 0x18, expectLink: true},
		{name: "bltzal sign rs", pc: 0x10, opcode: 0x1, regimm: 0x10, rs: -1, offset: 0x100, expectNextPC: 0x414, expectLink: true},
		{name: "bltzal rs only sign bit set", pc: 0x10, opcode: 0x1, regimm: 0x10, rs: testutil.ToSignedInteger(0x80_00_00_00_00_00_00_00), offset: 0x100, expectNextPC: 0x414, expectLink: true},
		{name: "bltzal sign-extended offset", pc: 0x10, opcode: 0x1, regimm: 0x10, rs: -1, offset: 0x80_00, expectNextPC: 0xFF_FF_FF_FF_FF_FE_00_14, expectLink: true},
		{name: "bltzal large offset no-sign", pc: 0x10, opcode: 0x1, regimm: 0x10, rs: -1, offset: 0x7F_FF, expectNextPC: 0x2_00_10, expectLink: true},

		// bgez t0, $x
		{name: "bgez", pc: 0, opcode: 0x1, regimm: 0x1, rs: 0x5, offset: 0x100, expectNextPC: 0x404},
		{name: "bgez large rs", pc: 0x10, opcode: 0x1, regimm: 0x1, rs: 0x7F_FF_FF_FF_FF_FF_FF_FF, offset: 0x100, expectNextPC: 0x414},
		{name: "bgez zero rs", pc: 0x10, opcode: 0x1, regimm: 0x1, rs: 0x0, offset: 0x100, expectNextPC: 0x414},
		{name: "bgez branch not taken", pc: 0x10, opcode: 0x1, regimm: 0x1, rs: -1, offset: 0x100, expectNextPC: 0x18},
		{name: "bgez sign-extended offset", pc: 0x10, opcode: 0x1, regimm: 0x1, rs: 1, offset: 0x80_00, expectNextPC: 0xFF_FF_FF_FF_FF_FE_00_14},
		{name: "bgez large offset no-sign", pc: 0x10, opcode: 0x1, regimm: 0x1, rs: 1, offset: 0x70_00, expectNextPC: 0x1_C0_14},
		{name: "bgez fill bit offset except sign", pc: 0x10, opcode: 0x1, regimm: 0x1, rs: 1, offset: 0x7F_FF, expectNextPC: 0x2_00_10},

		// bgezal t0, $x
		{name: "bgezal", pc: 0, opcode: 0x1, regimm: 0x11, rs: 0x5, offset: 0x100, expectNextPC: 0x404, expectLink: true},
		{name: "bgezal large rs", pc: 0x10, opcode: 0x1, regimm: 0x11, rs: 0x7F_FF_FF_FF_FF_FF_FF_FF, offset: 0x100, expectNextPC: 0x414, expectLink: true},
		{name: "bgezal zero rs", pc: 0x10, opcode: 0x1, regimm: 0x11, rs: 0x0, offset: 0x100, expectNextPC: 0x414, expectLink: true},
		{name: "bgezal branch not taken", pc: 0x10, opcode: 0x1, regimm: 0x11, rs: -1, offset: 0x100, expectNextPC: 0x18, expectLink: true},
		{name: "bgezal sign-extended offset", pc: 0x10, opcode: 0x1, regimm: 0x11, rs: 1, offset: 0x80_00, expectNextPC: 0xFF_FF_FF_FF_FF_FE_00_14, expectLink: true},
		{name: "bgezal large offset no-sign", pc: 0x10, opcode: 0x1, regimm: 0x11, rs: 1, offset: 0x70_00, expectNextPC: 0x1_C0_14, expectLink: true},
		{name: "bgezal fill bit offset except sign", pc: 0x10, opcode: 0x1, regimm: 0x11, rs: 1, offset: 0x7F_FF, expectNextPC: 0x2_00_10, expectLink: true},
	}

	testBranch(t, cases)
}

func TestEVM_SingleStep_Clz64(t *testing.T) {
	t.Parallel()
	rsReg := uint32(7)
	rdReg := uint32(8)
	cases := []struct {
		name           string
		rs             Word
		funct          uint32
		expectedResult Word
	}{
		// dclz
		{name: "dclz", rs: Word(0x0), expectedResult: Word(64), funct: 0b10_0100},
		{name: "dclz", rs: Word(0x1), expectedResult: Word(63), funct: 0b10_0100},
		{name: "dclz", rs: Word(0x10_00_00_00), expectedResult: Word(35), funct: 0b10_0100},
		{name: "dclz", rs: Word(0x80_00_00_00), expectedResult: Word(32), funct: 0b10_0100},
		{name: "dclz", rs: Word(0x80_00_00_00_00_00_00_00), expectedResult: Word(0), funct: 0b10_0100},
		{name: "dclz, sign-extended", rs: Word(0x80_00_00_00_00_00_00_00), expectedResult: Word(0), funct: 0b10_0100},
	}

	versions := GetMipsVersionTestCases(t)
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
				// step := state.GetStep()

				// Setup expectations
				expected := testutil.NewExpectedState(state)
				expected.ExpectStep()
				expected.Registers[rdReg] = tt.expectedResult
				// stepWitness, err := goVm.Step(true)
				_, err := goVm.Step(true)
				require.NoError(t, err)

				// Check expectations
				expected.Validate(t, state)

				// testutil.ValidateEVM(t, stepWitness, step, goVm, v.StateHashFn, v.Contracts)
			})
		}
	}
}

func TestEVM_SingleStep_Rot64(t *testing.T) {
	t.Parallel()
	rsReg := uint32(7)
	rdReg := uint32(8)
	rtReg := uint32(9)

	cases := []struct {
		name           string
		rs             Word
		rt             Word
		sa             uint32
		funct          uint32
		expectedResult Word
	}{
		// drotr
		// 0x2 rotated right by 1 -> 0x1
		{name: "drotr", sa: 1, rt: Word(0x2), expectedResult: Word(0x1), funct: 0b11_1010},
		// 0x1 rotated right by 1 -> MSB set
		{name: "drotr MSB set", sa: 1, rt: Word(0x1), expectedResult: Word(0x80_00_00_00_00_00_00_00), funct: 0b11_1010},
		// Rotate right by 8 (byte-level rotation)
		{name: "drotr byte shift", sa: 8, rt: Word(0x123456789ABCDEF0), expectedResult: Word(0xF0123456789ABCDE), funct: 0b11_1010},
		// Rotate right by 16 (halfword-level rotation)
		{name: "drotr halfword shift", sa: 16, rt: Word(0x123456789ABCDEF0), expectedResult: Word(0xDEF0123456789ABC), funct: 0b11_1010},
		// Edge case: rotating zero
		{name: "drotr zero", sa: 5, rt: Word(0x0), expectedResult: Word(0x0), funct: 0b11_1010},

		// drotr32
		// Rotate by exactly 32 bits (should swap halves)
		{name: "drotr32", sa: 32 + 0, rt: Word(0x123456789ABCDEF0), expectedResult: Word(0x9ABCDEF012345678), funct: 0b11_1110},
		// Rotate by 33 bits (should shift one more bit beyond simple case)
		{name: "drotr32 by 1", sa: 32 + 1, rt: Word(0x123456789ABCDEF0), expectedResult: Word(0x4d5e6f78091a2b3c), funct: 0b11_1110},
		// Rotate by 40 bits (byte-level rotation)
		{name: "drotr32 by 8 (byte shift)", sa: 32 + 8, rt: Word(0x123456789ABCDEF0), expectedResult: Word(0x789abcdef0123456), funct: 0b11_1110},
		// Rotate by 48 bits (halfword-level rotation)
		{name: "drotr32 by 16 (halfword shift)", sa: 32 + 16, rt: Word(0x123456789ABCDEF0), expectedResult: Word(0x56789abcdef01234), funct: 0b11_1110},
		// Rotate by 63 bits (one less than full 64-bit cycle)
		{name: "drotr32 by 31", sa: 32 + 31, rt: Word(0x123456789ABCDEF0), expectedResult: Word(0x2468acf13579bde0), funct: 0b11_1110},
		// Rotate with MSB set, shifting it down into the lower bits
		{name: "drotr32 with MSB set", sa: 32 + 4, rt: Word(0x8000000000000000), expectedResult: Word(0x0000000008000000), funct: 0b11_1110},
		// Rotate all ones (0xFFFFFFFFFFFFFFFF) should remain unchanged
		{name: "drotr32 all ones", sa: 32 + 8, rt: Word(0xFFFFFFFFFFFFFFFF), expectedResult: Word(0xFFFFFFFFFFFFFFFF), funct: 0b11_1110},
		// Rotate zero (should remain zero)
		{name: "drotr32 zero", sa: 32 + 5, rt: Word(0x0000000000000000), expectedResult: Word(0x0000000000000000), funct: 0b11_1110},

		// drotrv
		// 0x2 rotated right by 1 -> 0x1
		{name: "drotrv", rs: 1, rt: Word(0x2), expectedResult: Word(0x1), funct: 0b01_0110},
		// 0x1 rotated right by 1 -> MSB set
		{name: "drotrv MSB set", rs: 1, rt: Word(0x1), expectedResult: Word(0x80_00_00_00_00_00_00_00), funct: 0b01_0110},
		// Rotate right by 8 (byte-level rotation)
		{name: "drotrv byte shift", rs: 8, rt: Word(0x123456789ABCDEF0), expectedResult: Word(0xF0123456789ABCDE), funct: 0b01_0110},
		// Rotate right by 16 (halfword-level rotation)
		{name: "drotrv halfword shift", rs: 16, rt: Word(0x123456789ABCDEF0), expectedResult: Word(0xDEF0123456789ABC), funct: 0b01_0110},
		// Rotate by exactly 32 bits (should swap halves)
		{name: "drotrv by 32", rs: 32, rt: Word(0x123456789ABCDEF0), expectedResult: Word(0x9ABCDEF012345678), funct: 0b01_0110},
		// Rotate by 33 bits (should shift one more bit beyond simple case)
		{name: "drotrv by 33", rs: 33, rt: Word(0x123456789ABCDEF0), expectedResult: Word(0x4d5e6f78091a2b3c), funct: 0b01_0110},
		// Rotate by 40 bits (byte-level rotation)
		{name: "drotrv by 40", rs: 40, rt: Word(0x123456789ABCDEF0), expectedResult: Word(0x789abcdef0123456), funct: 0b01_0110},
		// Rotate by 48 bits (halfword-level rotation)
		{name: "drotrv by 48", rs: 48, rt: Word(0x123456789ABCDEF0), expectedResult: Word(0x56789abcdef01234), funct: 0b01_0110},
		// Rotate by 63 bits (one less than full 64-bit cycle)
		{name: "drotrv by 63", rs: 63, rt: Word(0x123456789ABCDEF0), expectedResult: Word(0x2468acf13579bde0), funct: 0b01_0110},
		// Rotate with MSB set, shifting it down into the lower bits
		{name: "drotrv with MSB set", rs: 36, rt: Word(0x8000000000000000), expectedResult: Word(0x0000000008000000), funct: 0b01_0110},
		// Rotate all ones (0xFFFFFFFFFFFFFFFF) should remain unchanged
		{name: "drotrv all ones", rs: 40, rt: Word(0xFFFFFFFFFFFFFFFF), expectedResult: Word(0xFFFFFFFFFFFFFFFF), funct: 0b01_0110},
		// Rotate zero (should remain zero)
		{name: "drotrv zero", rs: 5, rt: Word(0x0), expectedResult: Word(0x0), funct: 0b01_0110},
	}

	versions := GetMipsVersionTestCases(t)
	for _, v := range versions {
		for i, tt := range cases {
			testName := fmt.Sprintf("%v (%v)", tt.name, v.Name)
			t.Run(testName, func(t *testing.T) {
				// Set up state
				goVm := v.VMFactory(nil, os.Stdout, os.Stderr, testutil.CreateLogger(), testutil.WithRandomization(int64(i)))
				state := goVm.GetState()

				var insn uint32
				if tt.funct == 0b11_1010 { // drotr
					insn = 1<<21 | rtReg<<16 | rdReg<<11 | tt.sa<<6 | tt.funct
				} else if tt.funct == 0b11_1110 { // drotr32
					require.GreaterOrEqual(t, tt.sa, uint32(32), "sa should be >= 32 for drotr32")
					insn = 1<<21 | rtReg<<16 | rdReg<<11 | (tt.sa-32)<<6 | tt.funct
				} else if tt.funct == 0b01_0110 { // drotrv
					insn = rsReg<<21 | rtReg<<16 | rdReg<<11 | 1<<6 | tt.funct
				}
				testutil.StoreInstruction(state.GetMemory(), state.GetPC(), insn)
				state.GetRegistersRef()[rtReg] = tt.rt
				if tt.funct == 0b01_0110 { // drotrv
					state.GetRegistersRef()[rsReg] = tt.rs
				}
				// step := state.GetStep()

				// Setup expectations
				expected := testutil.NewExpectedState(state)
				expected.ExpectStep()
				expected.Registers[rdReg] = tt.expectedResult
				// stepWitness, err := goVm.Step(true)
				_, err := goVm.Step(true)
				require.NoError(t, err)

				// Check expectations
				expected.Validate(t, state)

				// testutil.ValidateEVM(t, stepWitness, step, goVm, v.StateHashFn, v.Contracts)
			})
		}
	}
}

func TestEVM_SingleStep_Ext64(t *testing.T) {
	t.Parallel()
	rsReg := uint32(7)
	rtReg := uint32(9)

	cases := []struct {
		name           string
		rs             Word
		msbd           uint32
		lsb            uint32
		funct          uint32
		expectedResult Word
	}{
		// dext
		// Extract 8 bits starting at bit 0 (byte 0)
		{name: "dext byte 0", rs: Word(0x123456789ABCDEF0), msbd: 8 - 1, lsb: 0, funct: 0b000011, expectedResult: Word(0xF0)},
		// Extract 8 bits starting at bit 8 (byte 1)
		{name: "dext byte 1", rs: Word(0x123456789ABCDEF0), msbd: 8 - 1, lsb: 8, funct: 0b000011, expectedResult: Word(0xDE)},
		// Extract 8 bits starting at bit 16 (byte 2)
		{name: "dext byte 2", rs: Word(0x123456789ABCDEF0), msbd: 8 - 1, lsb: 16, funct: 0b000011, expectedResult: Word(0xBC)},
		// Extract 8 bits starting at bit 24 (byte 3)
		{name: "dext byte 3", rs: Word(0x123456789ABCDEF0), msbd: 8 - 1, lsb: 24, funct: 0b000011, expectedResult: Word(0x9A)},
		// Extract 4 bits starting at bit 0 (low nibble)
		{name: "dext nibble low", rs: Word(0x123456789ABCDEF0), msbd: 4 - 1, lsb: 0, funct: 0b000011, expectedResult: Word(0x0)},
		// Extract 4 bits starting at bit 12 (high nibble)
		{name: "dext nibble high", rs: Word(0x123456789ABCDEF0), msbd: 4 - 1, lsb: 12, funct: 0b000011, expectedResult: Word(0xD)},
		// Extract full 16-bit halfword [15:0]
		{name: "dext half word", rs: Word(0x123456789ABCDEF0), msbd: 16 - 1, lsb: 0, funct: 0b000011, expectedResult: Word(0xDEF0)},
		// Extract full 32-bit word [31:0]
		{name: "dext full word", rs: Word(0x123456789ABCDEF0), msbd: 32 - 1, lsb: 0, funct: 0b000011, expectedResult: Word(0x9ABCDEF0)},

		// dextm
		// Extract 32 + 8 bits starting at bit 20 [60:20]
		{name: "dextm by 40", rs: Word(0x123456789ABCDEF0), msbd: 32 + (8 - 1), lsb: 20, funct: 0b000001, expectedResult: Word(0x23456789AB)},
		// Extract 32 + 16 bits starting at bit 24 [48:0]
		{name: "dextm by 48", rs: Word(0x123456789ABCDEF0), msbd: 32 + (16 - 1), lsb: 0, funct: 0b000001, expectedResult: Word(0x56789ABCDEF0)},
		// Extract 32 + 28 word from bit 4 [63:4]
		{name: "dextm by 60", rs: Word(0x123456789ABCDEF0), msbd: 32 + (28 - 1), lsb: 4, funct: 0b000001, expectedResult: Word(0x123456789ABCDEF)},

		// dextu
		// Extract 4 bits from bit 40
		{name: "dextu byte 5", rs: Word(0x123456789ABCDEF0), msbd: 4 - 1, lsb: 40, funct: 0b000010, expectedResult: Word(0x6)},
		// Extract 12 bits from bit 44
		{name: "dextu 12-bit unaligned", rs: Word(0x123456789ABCDEF0), msbd: 12 - 1, lsb: 44, funct: 0b000010, expectedResult: Word(0x345)},
		// Extract 16 bits from bit 48 (halfword in upper half)
		{name: "dextu halfword", rs: Word(0x123456789ABCDEF0), msbd: 16 - 1, lsb: 48, funct: 0b000010, expectedResult: Word(0x1234)},
		// Extract 24 bits from bit 36
		{name: "dextu 24-bit field", rs: Word(0x123456789ABCDEF0), msbd: 24 - 1, lsb: 36, funct: 0b000010, expectedResult: Word(0x234567)},
		// Extract full 32-bit word from bit 32
		{name: "dextu full word", rs: Word(0x123456789ABCDEF0), msbd: 32 - 1, lsb: 32, funct: 0b000010, expectedResult: Word(0x12345678)},

		// ext
		// Extract lower 8 bits (byte 0)
		{name: "ext byte 0", rs: Word(0x12345678), msbd: 8 - 1, lsb: 0, funct: 0b000000, expectedResult: Word(0x78)},
		// Extract bits 8-15 (byte 1)
		{name: "ext byte 1", rs: Word(0x12345678), msbd: 8 - 1, lsb: 8, funct: 0b000000, expectedResult: Word(0x56)},
		// Extract bits 16-23 (byte 2)
		{name: "ext byte 2", rs: Word(0x12345678), msbd: 8 - 1, lsb: 16, funct: 0b000000, expectedResult: Word(0x34)},
		// Extract bits 24-31 (byte 3)
		{name: "ext byte 3", rs: Word(0x12345678), msbd: 8 - 1, lsb: 24, funct: 0b000000, expectedResult: Word(0x12)},
		// Extract 16-bit halfword from bits 8-23
		{name: "ext halfword", rs: Word(0x12345678), msbd: 16 - 1, lsb: 8, funct: 0b000000, expectedResult: Word(0x3456)},
		// Extract full 32-bit word (should return the same value)
		{name: "ext full word", rs: Word(0x12345678), msbd: 32 - 1, lsb: 0, funct: 0b000000, expectedResult: Word(0x12345678)},
		// Extract full 32-bit word sign extended
		{name: "ext full word", rs: Word(0xFFFFFFFF), msbd: 32 - 1, lsb: 0, funct: 0b000000, expectedResult: Word(0xFFFFFFFFFFFFFFFF)},
	}

	versions := GetMipsVersionTestCases(t)
	for _, v := range versions {
		for i, tt := range cases {
			testName := fmt.Sprintf("%v (%v)", tt.name, v.Name)
			t.Run(testName, func(t *testing.T) {
				// Set up state
				goVm := v.VMFactory(nil, os.Stdout, os.Stderr, testutil.CreateLogger(), testutil.WithRandomization(int64(i)))
				state := goVm.GetState()

				var insn uint32
				if tt.funct == 0b00_0011 || tt.funct == 0b00_0000 { // dext, ext
					insn = 0b011111<<26 | rsReg<<21 | rtReg<<16 | tt.msbd<<11 | tt.lsb<<6 | tt.funct
				} else if tt.funct == 0b00_0001 { // dextm
					require.GreaterOrEqual(t, tt.msbd, uint32(32), "msbd should be >= 32 for dextm")
					insn = 0b011111<<26 | rsReg<<21 | rtReg<<16 | (tt.msbd-32)<<11 | tt.lsb<<6 | tt.funct
				} else if tt.funct == 0b00_0010 { // dextu
					require.GreaterOrEqual(t, tt.lsb, uint32(32), "lsb should be >= 32 for dextu")
					insn = 0b011111<<26 | rsReg<<21 | rtReg<<16 | tt.msbd<<11 | (tt.lsb-32)<<6 | tt.funct
				}

				testutil.StoreInstruction(state.GetMemory(), state.GetPC(), insn)
				state.GetRegistersRef()[rsReg] = tt.rs
				// step := state.GetStep()

				// Setup expectations
				expected := testutil.NewExpectedState(state)
				expected.ExpectStep()
				expected.Registers[rtReg] = tt.expectedResult
				// stepWitness, err := goVm.Step(true)
				_, err := goVm.Step(true)
				require.NoError(t, err)

				// Check expectations
				expected.Validate(t, state)

				// testutil.ValidateEVM(t, stepWitness, step, goVm, v.StateHashFn, v.Contracts)
			})
		}
	}
}

func TestEVM_SingleStep_Ins64(t *testing.T) {
	t.Parallel()
	rsReg := uint32(7)
	rtReg := uint32(9)

	cases := []struct {
		name           string
		rs             Word
		rt             Word
		msb            uint32
		lsb            uint32
		funct          uint32
		expectedResult Word
	}{
		// dins
		// Insert 8 bits from rs into rt at bit 16
		{name: "dins byte insert", rs: Word(0x12345678), rt: Word(0xFFFFFFFFFFFFFFFF), msb: 16 + (8 - 1), lsb: 16, funct: 0b000111, expectedResult: Word(0xFFFFFFFFFF_78_FFFF)},
		// Insert 12 bits from rs into rt at bit 20
		{name: "dins 12-bit insert", rs: Word(0x12345678), rt: Word(0xFFFFFFFFFFFFFFFF), msb: 20 + (12 - 1), lsb: 20, funct: 0b000111, expectedResult: Word(0xFFFFFFFF_678_FFFFF)},
		// Insert 16 bits from rs into rt at bit 8
		{name: "dins halfword insert", rs: Word(0x12345678), rt: Word(0xFFFFFFFFFFFFFFFF), msb: 8 + (16 - 1), lsb: 8, funct: 0b000111, expectedResult: Word(0xFFFFFFFFFF_5678_FF)},
		// Insert 24 bits from rs into rt at bit 4
		{name: "dins 24-bit insert", rs: Word(0x12345678), rt: Word(0xFFFFFFFFFFFFFFFF), msb: 4 + (24 - 1), lsb: 4, funct: 0b000111, expectedResult: Word(0xFFFFFFFFF_345678_F)},
		// Insert 32 bits from rs into rt at bit 0
		{name: "dins full word insert", rs: Word(0x12345678), rt: Word(0xFFFFFFFFFFFFFFFF), msb: 0 + (32 - 1), lsb: 0, funct: 0b000111, expectedResult: Word(0xFFFFFFFF_12345678)},

		// dinsm
		// Insert 32 + 8 bits from rs into rt at bit 24
		{name: "dinsm byte insert", rs: Word(0x123456789ABCDEF0), rt: Word(0xFFFFFFFFFFFFFFFF), msb: 32 + (8 - 1), lsb: 24, funct: 0b000101, expectedResult: Word(0xFFFFFF_DEF0_FFFFFF)},
		// Insert 32 + 12 bits from rs into rt at bit 28
		{name: "dinsm 12-bit insert", rs: Word(0x123456789ABCDEF0), rt: Word(0xFFFFFFFFFFFFFFFF), msb: 32 + (12 - 1), lsb: 28, funct: 0b000101, expectedResult: Word(0xFFFFF_DEF0_FFFFFFF)},
		// Insert 32 + 16 bits from rs into rt at bit 20
		{name: "dinsm halfword insert", rs: Word(0x123456789ABCDEF0), rt: Word(0xFFFFFFFFFFFFFFFF), msb: 32 + (16 - 1), lsb: 20, funct: 0b000101, expectedResult: Word(0xFFFF_ABCDEF0_FFFFF)},
		// Insert 32 + 24 bits from rs into rt at bit 12
		{name: "dinsm 24-bit insert", rs: Word(0x123456789ABCDEF0), rt: Word(0xFFFFFFFFFFFFFFFF), msb: 32 + (24 - 1), lsb: 12, funct: 0b000101, expectedResult: Word(0xFF_6789ABCDEF0_FFF)},
		// Insert 32 + 32 bits from rs into rt at bit 0
		{name: "dinsm full dword insert", rs: Word(0x123456789ABCDEF0), rt: Word(0xFFFFFFFFFFFFFFFF), msb: 32 + (32 - 1), lsb: 0, funct: 0b000101, expectedResult: Word(0x123456789ABCDEF0)},

		// dinsu
		// Insert 8 bits from rs into rt at bit 40
		{name: "dinsu byte insert", rs: Word(0x123456789ABCDEF0), rt: Word(0xFFFFFFFFFFFFFFFF), msb: 40 + (8 - 1), lsb: 40, funct: 0b000110, expectedResult: Word(0xFFFF_F0_FFFFFFFFFF)},
		// Insert 12 bits from rs into rt at bit 44
		{name: "dinsu 12-bit insert", rs: Word(0x123456789ABCDEF0), rt: Word(0xFFFFFFFFFFFFFFFF), msb: 44 + (12 - 1), lsb: 44, funct: 0b000110, expectedResult: Word(0xFF_EF0_FFFFFFFFFFF)},
		// Insert 16 bits from rs into rt at bit 48
		{name: "dinsu halfword insert", rs: Word(0x123456789ABCDEF0), rt: Word(0xFFFFFFFFFFFFFFFF), msb: 48 + (16 - 1), lsb: 48, funct: 0b000110, expectedResult: Word(0xDEF0_FFFFFFFFFFFF)},
		// Insert 24 bits from rs into rt at bit 36
		{name: "dinsu 24-bit insert", rs: Word(0x123456789ABCDEF0), rt: Word(0xFFFFFFFFFFFFFFFF), msb: 36 + (24 - 1), lsb: 36, funct: 0b000110, expectedResult: Word(0xF_BCDEF0_FFFFFFFFF)},
		// Insert 32 bits from rs into rt at bit 32
		{name: "dinsu full word insert", rs: Word(0x123456789ABCDEF0), rt: Word(0xFFFFFFFFFFFFFFFF), msb: 32 + (32 - 1), lsb: 32, funct: 0b000110, expectedResult: Word(0x9ABCDEF0_FFFFFFFF)},

		// ins
		// Insert 8-bit value from rs into rt at bit 0
		{name: "ins byte 0", rs: Word(0x000000AA), rt: Word(0x0FFF0000), msb: 0 + (8 - 1), lsb: 0, funct: 0b000100, expectedResult: Word(0x0FFF00AA)},
		// Insert 8-bit value from rs into rt at bit 8
		{name: "ins byte 1", rs: Word(0x000000AA), rt: Word(0x0FFF0000), msb: 8 + (8 - 1), lsb: 8, funct: 0b000100, expectedResult: Word(0x0FFFAA00)},
		// Insert 16-bit value from rs into rt at bit 0
		{name: "ins halfword 0", rs: Word(0x0000AAAA), rt: Word(0x0FFF0000), msb: 0 + (16 - 1), lsb: 0, funct: 0b000100, expectedResult: Word(0x0FFFAAAA)},
		// Insert 16-bit value from rs into rt at bit 8 (sign extend)
		{name: "ins halfword 1", rs: Word(0x0000AAAA), rt: Word(0xFFFF0000), msb: 8 + (16 - 1), lsb: 8, funct: 0b000100, expectedResult: Word(0xFFFFFFFFFFAAAA00)},
		// Insert 24-bit value from rs into rt at bit 4 (sign extend)
		{name: "ins 24-bit", rs: Word(0x00AAAAAA), rt: Word(0xFFFF0000), msb: 4 + (24 - 1), lsb: 4, funct: 0b000100, expectedResult: Word(0xFFFFFFFFFAAAAAA0)},
		// Insert full 32-bit value from rs into rt (sign extend)
		{name: "ins full word", rs: Word(0xAAAAAAAA), rt: Word(0xFFFF0000), msb: 0 + (32 - 1), lsb: 0, funct: 0b000100, expectedResult: Word(0xFFFFFFFFAAAAAAAA)},
	}

	versions := GetMipsVersionTestCases(t)
	for _, v := range versions {
		for i, tt := range cases {
			testName := fmt.Sprintf("%v (%v)", tt.name, v.Name)
			t.Run(testName, func(t *testing.T) {
				// Set up state
				goVm := v.VMFactory(nil, os.Stdout, os.Stderr, testutil.CreateLogger(), testutil.WithRandomization(int64(i)))
				state := goVm.GetState()

				var insn uint32
				if tt.funct == 0b00_0111 || tt.funct == 0b00_0100 { // dins, ins
					insn = 0b011111<<26 | rsReg<<21 | rtReg<<16 | tt.msb<<11 | tt.lsb<<6 | tt.funct
				} else if tt.funct == 0b00_0101 { // dinsm
					require.GreaterOrEqual(t, tt.msb, uint32(32), "msb should be >= 32 for dextm")
					insn = 0b011111<<26 | rsReg<<21 | rtReg<<16 | (tt.msb-32)<<11 | tt.lsb<<6 | tt.funct
				} else if tt.funct == 0b00_0110 { // dinsu
					require.GreaterOrEqual(t, tt.msb, uint32(32), "msb should be >= 32 for dextm")
					require.GreaterOrEqual(t, tt.lsb, uint32(32), "lsb should be >= 32 for dextu")
					insn = 0b011111<<26 | rsReg<<21 | rtReg<<16 | (tt.msb-32)<<11 | (tt.lsb-32)<<6 | tt.funct
				}
				testutil.StoreInstruction(state.GetMemory(), state.GetPC(), insn)
				state.GetRegistersRef()[rtReg] = tt.rt
				state.GetRegistersRef()[rsReg] = tt.rs

				// step := state.GetStep()

				// Setup expectations
				expected := testutil.NewExpectedState(state)
				expected.ExpectStep()
				expected.Registers[rtReg] = tt.expectedResult
				// stepWitness, err := goVm.Step(true)
				_, err := goVm.Step(true)
				require.NoError(t, err)

				// Check expectations
				expected.Validate(t, state)

				// testutil.ValidateEVM(t, stepWitness, step, goVm, v.StateHashFn, v.Contracts)
			})
		}
	}
}

func TestEVM_SingleStep_Swap64(t *testing.T) {
	t.Parallel()
	rdReg := uint32(8)
	rtReg := uint32(9)

	cases := []struct {
		name           string
		rt             Word
		funct          uint32
		special        uint32
		expectedResult Word
	}{
		// dsbh
		// Swap bytes within halfwords for a 64-bit value
		{name: "dsbh", rt: Word(0x1122334455667788), funct: 0b100100, special: 0b00010, expectedResult: Word(0x2211443366558877)},
		// Swap bytes within halfwords when all bytes are the same
		{name: "dsbh all same", rt: Word(0xFFFFFFFFFFFFFFFF), funct: 0b100100, special: 0b00010, expectedResult: Word(0xFFFFFFFFFFFFFFFF)},
		// Swap bytes within halfwords with zeros
		{name: "dsbh with zero", rt: Word(0x0000FFFF0000FFFF), funct: 0b100100, special: 0b00010, expectedResult: Word(0x0000FFFF0000FFFF)},
		// Swap bytes within halfwords when all bytes are different
		{name: "dsbh all different", rt: Word(0x123456789ABCDEF0), funct: 0b100100, special: 0b00010, expectedResult: Word(0x34127856BC9AF0DE)},

		// dshd
		// Swap halfwords within doubleword
		{name: "dshd", rt: Word(0x1122334455667788), funct: 0b100100, special: 0b00101, expectedResult: Word(0x7788556633441122)},
		// Swap halfwords within doubleword with alternating bit patterns
		{name: "dshd pattern", rt: Word(0xAABBCCDDEEFF0011), funct: 0b100100, special: 0b00101, expectedResult: Word(0x0011EEFFCCDDAABB)},
		// Swap halfwords within doubleword when all bytes are the same
		{name: "dshd all same", rt: Word(0xFFFFFFFFFFFFFFFF), funct: 0b100100, special: 0b00101, expectedResult: Word(0xFFFFFFFFFFFFFFFF)},
		// Swap halfwords within doubleword with zeros
		{name: "dshd with zero", rt: Word(0x0000FFFF0000FFFF), funct: 0b100100, special: 0b00101, expectedResult: Word(0xFFFF0000FFFF0000)},
		// Swap halfwords within doubleword when all bytes are different
		{name: "dshd half reversed", rt: Word(0x123456789ABCDEF0), funct: 0b100100, special: 0b00101, expectedResult: Word(0xDEF09ABC56781234)},

		// wsbh
		// Swap bytes within halfwords (lower 32 bits)
		{name: "wsbh", rt: Word(0x11223344), funct: 0b100000, special: 0b00010, expectedResult: Word(0x0000000022114433)},
		// Swap bytes within halfwords (sign extend)
		{name: "wsbh sign extend", rt: Word(0xEEFF0011), funct: 0b100000, special: 0b00010, expectedResult: Word(0xFFFFFFFFFFEE1100)},
		// Swap bytes within halfwords (all bits set)
		{name: "wsbh all ones 64-bit", rt: Word(0xFFFFFFFF), funct: 0b100000, special: 0b00010, expectedResult: Word(0xFFFFFFFFFFFFFFFF)},
		// Swap bytes within halfwords (all bits zero)
		{name: "wsbh all zero 64-bit", rt: Word(0x00000000), funct: 0b100000, special: 0b00010, expectedResult: Word(0x0000000000000000)},
	}

	versions := GetMipsVersionTestCases(t)
	for _, v := range versions {
		for i, tt := range cases {
			testName := fmt.Sprintf("%v (%v)", tt.name, v.Name)
			t.Run(testName, func(t *testing.T) {
				// Set up state
				goVm := v.VMFactory(nil, os.Stdout, os.Stderr, testutil.CreateLogger(), testutil.WithRandomization(int64(i)))
				state := goVm.GetState()

				var insn uint32
				insn = 0b011111<<26 | rtReg<<16 | rdReg<<11 | tt.special<<6 | tt.funct

				testutil.StoreInstruction(state.GetMemory(), state.GetPC(), insn)
				state.GetRegistersRef()[rtReg] = tt.rt
				// step := state.GetStep()

				// Setup expectations
				expected := testutil.NewExpectedState(state)
				expected.ExpectStep()
				expected.Registers[rdReg] = tt.expectedResult
				// stepWitness, err := goVm.Step(true)
				_, err := goVm.Step(true)
				require.NoError(t, err)

				// Check expectations
				expected.Validate(t, state)

				// testutil.ValidateEVM(t, stepWitness, step, goVm, v.StateHashFn, v.Contracts)
			})
		}
	}
}

func TestEVM_SingleStep_SignExtend64(t *testing.T) {
	t.Parallel()
	rdReg := uint32(8)
	rtReg := uint32(9)
	cases := []struct {
		name           string
		rt             Word
		funct          uint32
		special        uint32
		expectedResult Word
	}{
		// seb
		// Sign-extend byte (positive value)
		{name: "seb positive", rt: Word(0x0000007F), funct: 0b100000, special: 0b10000, expectedResult: Word(0x000000000000007F)},
		// Sign-extend byte (negative value)
		{name: "seb negative", rt: Word(0x00000080), funct: 0b100000, special: 0b10000, expectedResult: Word(0xFFFFFFFFFFFFFF80)},
		// Sign-extend byte (mid-range)
		{name: "seb mid-range", rt: Word(0x00000055), funct: 0b100000, special: 0b10000, expectedResult: Word(0x0000000000000055)},
		// Sign-extend byte (full 8-bit set)
		{name: "seb full-byte", rt: Word(0x000000FF), funct: 0b100000, special: 0b10000, expectedResult: Word(0xFFFFFFFFFFFFFFFF)},
		// Sign-extend byte with upper bits set
		{name: "seb upper bits", rt: Word(0x123456FF), funct: 0b100000, special: 0b10000, expectedResult: Word(0xFFFFFFFFFFFFFFFF)},

		// seh
		{name: "seh positive", rt: Word(0x00007FFF), funct: 0b100000, special: 0b11000, expectedResult: Word(0x0000000000007FFF)},
		// Sign-extend halfword (negative value)
		{name: "seh negative", rt: Word(0x00008000), funct: 0b100000, special: 0b11000, expectedResult: Word(0xFFFFFFFFFFFF8000)},
		// Sign-extend halfword (mid-range)
		{name: "seh mid-range", rt: Word(0x00005555), funct: 0b100000, special: 0b11000, expectedResult: Word(0x0000000000005555)},
		// Sign-extend halfword (full 16-bit set)
		{name: "seh full-halfword", rt: Word(0x0000FFFF), funct: 0b100000, special: 0b11000, expectedResult: Word(0xFFFFFFFFFFFFFFFF)},
		// Sign-extend halfword with upper bits set
		{name: "seh upper bits", rt: Word(0x1234FFFF), funct: 0b100000, special: 0b11000, expectedResult: Word(0xFFFFFFFFFFFFFFFF)},
	}

	versions := GetMipsVersionTestCases(t)
	for _, v := range versions {
		for i, tt := range cases {
			testName := fmt.Sprintf("%v (%v)", tt.name, v.Name)
			t.Run(testName, func(t *testing.T) {
				// Set up state
				goVm := v.VMFactory(nil, os.Stdout, os.Stderr, testutil.CreateLogger(), testutil.WithRandomization(int64(i)))
				state := goVm.GetState()

				var insn uint32
				insn = 0b011111<<26 | rtReg<<16 | rdReg<<11 | tt.special<<6 | tt.funct

				testutil.StoreInstruction(state.GetMemory(), state.GetPC(), insn)
				state.GetRegistersRef()[rtReg] = tt.rt
				// step := state.GetStep()

				// Setup expectations
				expected := testutil.NewExpectedState(state)
				expected.ExpectStep()
				expected.Registers[rdReg] = tt.expectedResult
				// stepWitness, err := goVm.Step(true)
				_, err := goVm.Step(true)
				require.NoError(t, err)

				// Check expectations
				expected.Validate(t, state)

				// testutil.ValidateEVM(t, stepWitness, step, goVm, v.StateHashFn, v.Contracts)
			})
		}
	}
}
