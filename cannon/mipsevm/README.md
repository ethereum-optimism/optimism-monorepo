# `mipsevm`

Supported 63 instructions:
| Category             | Instruction   | Description                                  |
|----------------------|---------------|----------------------------------------------|
| `Arithmetic`         | `add`         | Add.                                         |
| `Arithmetic`         | `addi`        | Add immediate (with sign-extension).         |
| `Arithmetic`         | `addiu`       | Add immediate unsigned.                      |
| `Arithmetic`         | `addu`        | Add unsigned.                                |
| `Logical`            | `and`         | Bitwise AND.                                 |
| `Logical`            | `andi`        | Bitwise AND immediate.                       |
| `Conditional Branch` | `beq`         | Branch on equal.                             |
| `Conditional Branch` | `bgez`        | Branch on greater than or equal to zero.     |
| `Conditional Branch` | `bgtz`        | Branch on greater than zero.                 |
| `Conditional Branch` | `bgezal`      | Branch and link on greater than or equal to zero.     |
| `Conditional Branch` | `blez`        | Branch on less than or equal to zero.        |
| `Conditional Branch` | `bltz`        | Branch on less than zero.                    |
| `Conditional Branch` | `bltzal`      | Branch and link on less than zero.           |
| `Conditional Branch` | `bne`         | Branch on not equal.                         |
| `Logical`            | `clo`         | Count leading ones.                          |
| `Logical`            | `clz`         | Count leading zeros.                         |
| `Logical`            | `dclz`        | Count Leading Zeros in Doubleword.           |
| `Bit Manipulation`   | `dext`        | Doubleword Extract Bit Field.                |
| `Bit Manipulation`   | `dextm`       | Doubleword Extract Bit Field Middle.         |
| `Bit Manipulation`   | `dextu`       | Doubleword Extract Bit Field Upper.          |
| `Bit Manipulation`   | `dins`        | Doubleword Insert Bit Field.                 |
| `Bit Manipulation`   | `dinsm`       | Doubleword Insert Bit Field Middle.          |
| `Bit Manipulation`   | `dinsu`       | Doubleword Insert Bit Field Upper.           |
| `Arithmetic`         | `div`         | Divide.                                      |
| `Arithmetic`         | `divu`        | Divide unsigned.                             |
| `Logical`            | `drotr`       | Doubleword Rotate Right.                     |
| `Logical`            | `drotr32`     | Doubleword Rotate Right Plus 32.             |
| `Logical`            | `drotrv`      | Doubleword Rotate Right Variable.            |
| `Bit Manipulation`   | `dsbh`        | Doubleword Swap Bytes Within Halfwords.      |
| `Bit Manipulation`   | `dshd`        | Doubleword Swap Halfwords Within Doublewords.|
| `Bit Manipulation`   | `ext`         | Extract Bit Field.                           |
| `Bit Manipulation`   | `ins`         | Insert Bit Field.                            |
| `Unconditional Jump` | `j`           | Jump.                                        |
| `Unconditional Jump` | `jal`         | Jump and link.                               |
| `Unconditional Jump` | `jalr`        | Jump and link register.                      |
| `Unconditional Jump` | `jr`          | Jump register.                               |
| `Data Transfer`      | `lb`          | Load byte.                                   |
| `Data Transfer`      | `lbu`         | Load byte unsigned.                          |
| `Data Transfer`      | `lh`          | Load halfword.                               |
| `Data Transfer`      | `lhu`         | Load halfword unsigned.                      |
| `Data Transfer`      | `ll`          | Load linked word.                            |
| `Data Transfer`      | `lui`         | Load upper immediate.                        |
| `Data Transfer`      | `lw`          | Load word.                                   |
| `Data Transfer`      | `lwl`         | Load word left.                              |
| `Data Transfer`      | `lwr`         | Load word right.                             |
| `Data Transfer`      | `mfhi`        | Move from HI register.                       |
| `Data Transfer`      | `mflo`        | Move from LO register.                       |
| `Data Transfer`      | `movn`        | Move conditional on not zero.                |
| `Data Transfer`      | `movz`        | Move conditional on zero.                    |
| `Data Transfer`      | `mthi`        | Move to HI register.                         |
| `Data Transfer`      | `mtlo`        | Move to LO register.                         |
| `Arithmetic`         | `mul`         | Multiply (to produce a word result).         |
| `Arithmetic`         | `mult`        | Multiply.                                    |
| `Arithmetic`         | `multu`       | Multiply unsigned.                           |
| `Logical`            | `nor`         | Bitwise NOR.                                 |
| `Logical`            | `or`          | Bitwise OR.                                  |
| `Logical`            | `ori`         | Bitwise OR immediate.                        |
| `Logical`            | `rotr`        | Rotate Word Right.                           |
| `Logical`            | `rotrv`       | Rotate Word Right Variable.                  |
| `Data Transfer`      | `sb`          | Store byte.                                  |
| `Data Transfer`      | `sc`          | Store conditional.                           |
| `Arithmetic`         | `seb`         | Sign-Extend Byte.                            |
| `Arithmetic`         | `seh`         | Sign-Extend Halfword.                        |
| `Data Transfer`      | `sh`          | Store halfword.                              |
| `Logical`            | `sll`         | Shift left logical.                          |
| `Logical`            | `sllv`        | Shift left logical variable.                 |
| `Comparison`         | `slt`         | Set on less than (signed).                   |
| `Comparison`         | `slti`        | Set on less than immediate.                  |
| `Comparison`         | `sltiu`       | Set on less than immediate unsigned.         |
| `Comparison`         | `sltu`        | Set on less than unsigned.                   |
| `Logical`            | `sra`         | Shift right arithmetic.                      |
| `Logical`            | `srav`        | Shift right arithmetic variable.             |
| `Logical`            | `srl`         | Shift right logical.                         |
| `Logical`            | `srlv`        | Shift right logical variable.                |
| `Arithmetic`         | `sub`         | Subtract.                                    |
| `Arithmetic`         | `subu`        | Subtract unsigned.                           |
| `Data Transfer`      | `sw`          | Store word.                                  |
| `Data Transfer`      | `swl`         | Store word left.                             |
| `Data Transfer`      | `swr`         | Store word right.                            |
| `Serialization`      | `sync`        | Synchronize shared memory.                   |
| `System Calls`       | `syscall`     | System call.                                 |
| `Bit Manipulation`   | `wsbh`        | Word Swap Bytes Within Halfwords.            |
| `Logical`            | `xor`         | Bitwise XOR.                                 |
| `Logical`            | `xori`        | Bitwise XOR immediate.                       |

To run:
1. Load a program into a state, e.g. using `LoadELF`.
2. Patch the program if necessary: e.g. using `PatchGo` for Go programs, `PatchStack` for empty initial stack, etc.
4. Implement the `PreimageOracle` interface
5. Instrument the emulator with the state, and pre-image oracle, using `NewInstrumentedState`
6. Step through the instrumented state with `Step(proof)`,
   where `proof==true` if witness data should be generated. Steps are faster with `proof==false`.
7. Optionally repeat the step on-chain by calling `MIPS.sol` and `PreimageOracle.sol`, using the above witness data.
