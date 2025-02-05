package tests

import (
	"bytes"
	"math"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/exec"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/memory"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/program"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/testutil"
	preimage "github.com/ethereum-optimism/optimism/op-preimage"
)

const syscallInsn = uint32(0x00_00_00_0c)

func FuzzStateSyscallBrk(f *testing.F) {
	versions := GetMipsVersionTestCases(f)
	f.Fuzz(func(t *testing.T, seed int64) {
		for _, v := range versions {
			t.Run(v.Name, func(t *testing.T) {
				goVm := v.VMFactory(nil, os.Stdout, os.Stderr, testutil.CreateLogger(), testutil.WithRandomization(seed))
				state := goVm.GetState()
				state.GetRegistersRef()[2] = arch.SysBrk
				testutil.StoreInstruction(state.GetMemory(), state.GetPC(), syscallInsn)
				step := state.GetStep()

				expected := testutil.NewExpectedState(state)
				expected.Step += 1
				expected.PC = state.GetCpu().NextPC
				expected.NextPC = state.GetCpu().NextPC + 4
				expected.Registers[2] = program.PROGRAM_BREAK // Return fixed BRK value
				expected.Registers[7] = 0                     // No error

				stepWitness, err := goVm.Step(true)
				require.NoError(t, err)
				require.False(t, stepWitness.HasPreimage())

				expected.Validate(t, state)
				testutil.ValidateEVM(t, stepWitness, step, goVm, v.StateHashFn, v.Contracts)
			})
		}
	})
}

func FuzzStateSyscallMmap(f *testing.F) {
	// Add special cases for large memory allocation
	f.Add(Word(0), Word(0x1000), Word(program.HEAP_END), int64(1))
	f.Add(Word(0), Word(1<<31), Word(program.HEAP_START), int64(2))
	// Check edge case - just within bounds
	f.Add(Word(0), Word(0x1000), Word(program.HEAP_END-4096), int64(3))

	versions := GetMipsVersionTestCases(f)
	f.Fuzz(func(t *testing.T, addr Word, siz Word, heap Word, seed int64) {
		for _, v := range versions {
			t.Run(v.Name, func(t *testing.T) {
				goVm := v.VMFactory(nil, os.Stdout, os.Stderr, testutil.CreateLogger(),
					testutil.WithRandomization(seed), testutil.WithHeap(heap))
				state := goVm.GetState()
				step := state.GetStep()

				state.GetRegistersRef()[2] = arch.SysMmap
				state.GetRegistersRef()[4] = addr
				state.GetRegistersRef()[5] = siz
				testutil.StoreInstruction(state.GetMemory(), state.GetPC(), syscallInsn)

				expected := testutil.NewExpectedState(state)
				expected.Step += 1
				expected.PC = state.GetCpu().NextPC
				expected.NextPC = state.GetCpu().NextPC + 4
				if addr == 0 {
					sizAlign := siz
					if sizAlign&memory.PageAddrMask != 0 { // adjust size to align with page size
						sizAlign = siz + memory.PageSize - (siz & memory.PageAddrMask)
					}
					newHeap := heap + sizAlign
					if newHeap > program.HEAP_END || newHeap < heap || sizAlign < siz {
						expected.Registers[2] = exec.SysErrorSignal
						expected.Registers[7] = exec.MipsEINVAL
					} else {
						expected.Heap = heap + sizAlign
						expected.Registers[2] = heap
						expected.Registers[7] = 0 // no error
					}
				} else {
					expected.Registers[2] = addr
					expected.Registers[7] = 0 // no error
				}

				stepWitness, err := goVm.Step(true)
				require.NoError(t, err)
				require.False(t, stepWitness.HasPreimage())

				expected.Validate(t, state)
				testutil.ValidateEVM(t, stepWitness, step, goVm, v.StateHashFn, v.Contracts)
			})
		}
	})
}

func FuzzStateSyscallExitGroup(f *testing.F) {
	versions := GetMipsVersionTestCases(f)
	f.Fuzz(func(t *testing.T, exitCode uint8, seed int64) {
		for _, v := range versions {
			t.Run(v.Name, func(t *testing.T) {
				goVm := v.VMFactory(nil, os.Stdout, os.Stderr, testutil.CreateLogger(),
					testutil.WithRandomization(seed))
				state := goVm.GetState()
				state.GetRegistersRef()[2] = arch.SysExitGroup
				state.GetRegistersRef()[4] = Word(exitCode)
				testutil.StoreInstruction(state.GetMemory(), state.GetPC(), syscallInsn)
				step := state.GetStep()

				expected := testutil.NewExpectedState(state)
				expected.Step += 1
				expected.Exited = true
				expected.ExitCode = exitCode

				stepWitness, err := goVm.Step(true)
				require.NoError(t, err)
				require.False(t, stepWitness.HasPreimage())

				expected.Validate(t, state)
				testutil.ValidateEVM(t, stepWitness, step, goVm, v.StateHashFn, v.Contracts)
			})
		}
	})
}

func FuzzStateSyscallFcntl(f *testing.F) {
	versions := GetMipsVersionTestCases(f)
	f.Fuzz(func(t *testing.T, fd Word, cmd Word, seed int64) {
		for _, v := range versions {
			t.Run(v.Name, func(t *testing.T) {
				goVm := v.VMFactory(nil, os.Stdout, os.Stderr, testutil.CreateLogger(),
					testutil.WithRandomization(seed))
				state := goVm.GetState()
				state.GetRegistersRef()[2] = arch.SysFcntl
				state.GetRegistersRef()[4] = fd
				state.GetRegistersRef()[5] = cmd
				testutil.StoreInstruction(state.GetMemory(), state.GetPC(), syscallInsn)
				step := state.GetStep()

				expected := testutil.NewExpectedState(state)
				expected.Step += 1
				expected.PC = state.GetCpu().NextPC
				expected.NextPC = state.GetCpu().NextPC + 4
				if cmd == 1 {
					switch fd {
					case exec.FdStdin, exec.FdStdout, exec.FdStderr,
						exec.FdPreimageRead, exec.FdHintRead, exec.FdPreimageWrite, exec.FdHintWrite:
						expected.Registers[2] = 0
						expected.Registers[7] = 0
					default:
						expected.Registers[2] = ^Word(0)
						expected.Registers[7] = exec.MipsEBADF
					}
				} else if cmd == 3 {
					switch fd {
					case exec.FdStdin, exec.FdPreimageRead, exec.FdHintRead:
						expected.Registers[2] = 0
						expected.Registers[7] = 0
					case exec.FdStdout, exec.FdStderr, exec.FdPreimageWrite, exec.FdHintWrite:
						expected.Registers[2] = 1
						expected.Registers[7] = 0
					default:
						expected.Registers[2] = ^Word(0)
						expected.Registers[7] = exec.MipsEBADF
					}
				} else {
					expected.Registers[2] = ^Word(0)
					expected.Registers[7] = exec.MipsEINVAL
				}

				stepWitness, err := goVm.Step(true)
				require.NoError(t, err)
				require.False(t, stepWitness.HasPreimage())

				expected.Validate(t, state)
				testutil.ValidateEVM(t, stepWitness, step, goVm, v.StateHashFn, v.Contracts)
			})
		}
	})
}

func FuzzStateHintRead(f *testing.F) {
	versions := GetMipsVersionTestCases(f)
	f.Fuzz(func(t *testing.T, addr Word, count Word, seed int64) {
		for _, v := range versions {
			t.Run(v.Name, func(t *testing.T) {
				preimageData := []byte("hello world")
				preimageKey := preimage.Keccak256Key(crypto.Keccak256Hash(preimageData)).PreimageKey()
				oracle := testutil.StaticOracle(t, preimageData) // only used for hinting

				goVm := v.VMFactory(oracle, os.Stdout, os.Stderr, testutil.CreateLogger(),
					testutil.WithRandomization(seed), testutil.WithPreimageKey(preimageKey))
				state := goVm.GetState()
				state.GetRegistersRef()[2] = arch.SysRead
				state.GetRegistersRef()[4] = exec.FdHintRead
				state.GetRegistersRef()[5] = addr
				state.GetRegistersRef()[6] = count
				testutil.StoreInstruction(state.GetMemory(), state.GetPC(), syscallInsn)
				step := state.GetStep()

				expected := testutil.NewExpectedState(state)
				expected.Step += 1
				expected.PC = state.GetCpu().NextPC
				expected.NextPC = state.GetCpu().NextPC + 4
				expected.Registers[2] = count
				expected.Registers[7] = 0 // no error

				stepWitness, err := goVm.Step(true)
				require.NoError(t, err)
				require.False(t, stepWitness.HasPreimage())

				expected.Validate(t, state)
				testutil.ValidateEVM(t, stepWitness, step, goVm, v.StateHashFn, v.Contracts)
			})
		}
	})
}

func FuzzStatePreimageRead(f *testing.F) {
	versions := GetMipsVersionTestCases(f)
	f.Fuzz(func(t *testing.T, addr arch.Word, pc arch.Word, count arch.Word, preimageOffset arch.Word, seed int64) {
		for _, v := range versions {
			t.Run(v.Name, func(t *testing.T) {
				effAddr := addr & arch.AddressMask
				pc = pc & arch.AddressMask
				preexistingMemoryVal := ^arch.Word(0)
				preimageValue := []byte("hello world")
				preimageData := testutil.AddPreimageLengthPrefix(preimageValue)
				if preimageOffset >= Word(len(preimageData)) || pc == effAddr {
					t.SkipNow()
				}
				preimageKey := preimage.Keccak256Key(crypto.Keccak256Hash(preimageValue)).PreimageKey()
				oracle := testutil.StaticOracle(t, preimageValue)

				goVm := v.VMFactory(oracle, os.Stdout, os.Stderr, testutil.CreateLogger(),
					testutil.WithRandomization(seed), testutil.WithPreimageKey(preimageKey), testutil.WithPreimageOffset(preimageOffset), testutil.WithPCAndNextPC(pc))
				state := goVm.GetState()
				state.GetRegistersRef()[2] = arch.SysRead
				state.GetRegistersRef()[4] = exec.FdPreimageRead
				state.GetRegistersRef()[5] = addr
				state.GetRegistersRef()[6] = count
				testutil.StoreInstruction(state.GetMemory(), state.GetPC(), syscallInsn)
				state.GetMemory().SetWord(effAddr, preexistingMemoryVal)
				step := state.GetStep()

				alignment := addr & arch.ExtMask
				writeLen := arch.WordSizeBytes - alignment
				if count < writeLen {
					writeLen = count
				}
				// Cap write length to remaining bytes of the preimage
				preimageDataLen := Word(len(preimageData))
				if preimageOffset+writeLen > preimageDataLen {
					writeLen = preimageDataLen - preimageOffset
				}

				expected := testutil.NewExpectedState(state)
				expected.Step += 1
				expected.PC = state.GetCpu().NextPC
				expected.NextPC = state.GetCpu().NextPC + 4
				expected.Registers[2] = writeLen
				expected.Registers[7] = 0 // no error
				expected.PreimageOffset += writeLen
				if writeLen > 0 {
					// Expect a memory write
					var expectedMemory []byte
					expectedMemory = arch.ByteOrderWord.AppendWord(expectedMemory, preexistingMemoryVal)
					copy(expectedMemory[alignment:], preimageData[preimageOffset:preimageOffset+writeLen])
					expected.ExpectMemoryWriteWord(effAddr, arch.ByteOrderWord.Word(expectedMemory[:]))
				}

				stepWitness, err := goVm.Step(true)
				require.NoError(t, err)
				require.True(t, stepWitness.HasPreimage())

				expected.Validate(t, state)
				testutil.ValidateEVM(t, stepWitness, step, goVm, v.StateHashFn, v.Contracts)
			})
		}
	})
}

func FuzzStateHintWrite(f *testing.F) {
	versions := GetMipsVersionTestCases(f)
	f.Fuzz(func(t *testing.T, addr Word, count Word, hint1, hint2, hint3 []byte, randSeed int64) {
		for _, v := range versions {
			t.Run(v.Name, func(t *testing.T) {
				// Make sure pc does not overlap with hint data in memory
				pc := Word(0)
				if addr <= 8 {
					addr += 8
				}

				// Set up hint data
				r := testutil.NewRandHelper(randSeed)
				hints := [][]byte{hint1, hint2, hint3}
				hintData := make([]byte, 0)
				for _, hint := range hints {
					prefixedHint := testutil.AddHintLengthPrefix(hint)
					hintData = append(hintData, prefixedHint...)
				}
				lastHintLen := math.Round(r.Fraction() * float64(len(hintData)))
				lastHint := hintData[:int(lastHintLen)]
				expectedBytesToProcess := int(count) + int(lastHintLen)
				if expectedBytesToProcess > len(hintData) {
					// Add an extra hint to span the rest of the hint data
					randomHint := r.RandomBytes(t, expectedBytesToProcess)
					prefixedHint := testutil.AddHintLengthPrefix(randomHint)
					hintData = append(hintData, prefixedHint...)
					hints = append(hints, randomHint)
				}

				// Set up state
				oracle := &testutil.HintTrackingOracle{}
				goVm := v.VMFactory(oracle, os.Stdout, os.Stderr, testutil.CreateLogger(),
					testutil.WithRandomization(randSeed), testutil.WithLastHint(lastHint), testutil.WithPCAndNextPC(pc))
				state := goVm.GetState()
				state.GetRegistersRef()[2] = arch.SysWrite
				state.GetRegistersRef()[4] = exec.FdHintWrite
				state.GetRegistersRef()[5] = addr
				state.GetRegistersRef()[6] = count
				step := state.GetStep()
				err := state.GetMemory().SetMemoryRange(addr, bytes.NewReader(hintData[int(lastHintLen):]))
				require.NoError(t, err)
				testutil.StoreInstruction(state.GetMemory(), state.GetPC(), syscallInsn)

				// Set up expectations
				expected := testutil.NewExpectedState(state)
				expected.Step += 1
				expected.PC = state.GetCpu().NextPC
				expected.NextPC = state.GetCpu().NextPC + 4
				expected.Registers[2] = count
				expected.Registers[7] = 0 // no error
				// Figure out hint expectations
				var expectedHints [][]byte
				expectedLastHint := make([]byte, 0)
				byteIndex := 0
				for _, hint := range hints {
					hintDataLength := len(hint) + 4 // Hint data + prefix
					hintLastByteIndex := hintDataLength + byteIndex - 1
					if hintLastByteIndex < expectedBytesToProcess {
						expectedHints = append(expectedHints, hint)
					} else {
						expectedLastHint = hintData[byteIndex:expectedBytesToProcess]
						break
					}
					byteIndex += hintDataLength
				}
				expected.LastHint = expectedLastHint

				// Run state transition
				stepWitness, err := goVm.Step(true)
				require.NoError(t, err)
				require.False(t, stepWitness.HasPreimage())

				// Validate
				require.Equal(t, expectedHints, oracle.Hints())
				expected.Validate(t, state)
				testutil.ValidateEVM(t, stepWitness, step, goVm, v.StateHashFn, v.Contracts)
			})
		}
	})
}

func FuzzStatePreimageWrite(f *testing.F) {
	versions := GetMipsVersionTestCases(f)
	f.Fuzz(func(t *testing.T, addr arch.Word, count arch.Word, seed int64) {
		for _, v := range versions {
			t.Run(v.Name, func(t *testing.T) {
				// Make sure pc does not overlap with preimage data in memory
				pc := Word(0)
				if addr <= 8 {
					addr += 8
				}
				effAddr := addr & arch.AddressMask
				preexistingMemoryVal := [8]byte{0x12, 0x34, 0x56, 0x78, 0x87, 0x65, 0x43, 0x21}
				preimageData := []byte("hello world")
				preimageKey := preimage.Keccak256Key(crypto.Keccak256Hash(preimageData)).PreimageKey()
				oracle := testutil.StaticOracle(t, preimageData)

				goVm := v.VMFactory(oracle, os.Stdout, os.Stderr, testutil.CreateLogger(),
					testutil.WithRandomization(seed), testutil.WithPreimageKey(preimageKey), testutil.WithPreimageOffset(128), testutil.WithPCAndNextPC(pc))
				state := goVm.GetState()
				state.GetRegistersRef()[2] = arch.SysWrite
				state.GetRegistersRef()[4] = exec.FdPreimageWrite
				state.GetRegistersRef()[5] = addr
				state.GetRegistersRef()[6] = count
				testutil.StoreInstruction(state.GetMemory(), state.GetPC(), syscallInsn)
				state.GetMemory().SetWord(effAddr, arch.ByteOrderWord.Word(preexistingMemoryVal[:]))
				step := state.GetStep()

				expectBytesWritten := count
				alignment := addr & arch.ExtMask
				sz := arch.WordSizeBytes - alignment
				if sz < expectBytesWritten {
					expectBytesWritten = sz
				}

				expected := testutil.NewExpectedState(state)
				expected.Step += 1
				expected.PC = state.GetCpu().NextPC
				expected.NextPC = state.GetCpu().NextPC + 4
				expected.PreimageOffset = 0
				expected.Registers[2] = expectBytesWritten
				expected.Registers[7] = 0 // No error
				expected.PreimageKey = preimageKey
				if expectBytesWritten > 0 {
					// Copy original preimage key, but shift it left by expectBytesWritten
					copy(expected.PreimageKey[:], preimageKey[expectBytesWritten:])
					// Copy memory data to rightmost expectedBytesWritten
					copy(expected.PreimageKey[32-expectBytesWritten:], preexistingMemoryVal[alignment:])
				}

				stepWitness, err := goVm.Step(true)
				require.NoError(t, err)
				require.False(t, stepWitness.HasPreimage())

				expected.Validate(t, state)
				testutil.ValidateEVM(t, stepWitness, step, goVm, v.StateHashFn, v.Contracts)
			})
		}
	})
}
