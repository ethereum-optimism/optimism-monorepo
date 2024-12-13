Kontrol Lemmas
==============

Lemmas are K rewrite rules that enhance the reasoning power of Kontrol. For more information on lemmas, please consult [this section](https://docs.runtimeverification.com/kontrol/guides/advancing-proofs) of the Kontrol documentation.

This file contains the lemmas required to run the proofs included in the [proofs](./proofs) folder. Some of these lemmas are general enough to likely be incorporated into future versions of Kontrol, while others are specific to the challenges presented by the proofs.

Similarly to other files such as [`cheatcodes.md`](https://github.com/runtimeverification/kontrol/blob/master/src/kontrol/kdist/cheatcodes.md), we use the idiomatic way of programming in Kontrol, which is [literate programming](https://en.wikipedia.org/wiki/Literate_programming), allowing for better documentation of the code.

## Imports

For writing the lemmas, we use the [`foundry.md`](https://github.com/runtimeverification/kontrol/blob/master/src/kontrol/kdist/foundry.md) file. This file contains and imports all of the definitions from KEVM and Kontrol on top of which we write the lemmas.

```k
requires "foundry.md"

module PAUSABILITY-LEMMAS
    imports BOOL
    imports FOUNDRY
    imports INT-SYMBOLIC
```

## Arithmetic

Lemmas on arithmetic reasoning. Specifically, on: cancellativity, inequalities in which the two sides are of different signs; and the rounding-up mechanism of the Solidity compiler (expressed through `notMaxUInt5 &Int ( X +Int 31 )`, which rounds up `X` to the nearest multiple of 32).

```k
    // Cancellativity #1
    rule A +Int ( (B -Int A) +Int C ) => B +Int C [simplification]

    // Cancellativity #2
    rule (A -Int B) -Int (C -Int B) => A -Int C [simplification]

    // Cancellativity #3
    rule A -Int (A +Int B) => 0 -Int B [simplification]

    // Various inequalities
    rule X  <Int A &Int B => true requires X <Int 0 andBool 0 <=Int A andBool 0 <=Int B [concrete(X), simplification]
    rule X  <Int A +Int B => true requires X <Int 0 andBool 0 <=Int A andBool 0 <=Int B [concrete(X), simplification]
    rule X <=Int A +Int B => true requires X <Int 0 andBool 0 <=Int A andBool 0 <=Int B [concrete(X), simplification]

    // Upper bound on (pow256 - 32) &Int lengthBytes(X)
    rule notMaxUInt5 &Int Y <=Int Y => true
      requires 0 <=Int Y
      [simplification]

    // Bounds on notMaxUInt5 &Int ( X +Int 31 )
    rule X <=Int   notMaxUInt5 &Int ( X +Int 31 )          => true requires 0 <=Int X                   [simplification]
    rule X <=Int   notMaxUInt5 &Int ( Y +Int 31 )          => true requires X <=Int 0 andBool 0 <=Int Y [simplification, concrete(X)]
    rule X <=Int ( notMaxUInt5 &Int ( X +Int 31 ) ) +Int Y => true requires 0 <=Int X andBool 0 <=Int Y [simplification, concrete(Y)]

    rule notMaxUInt5 &Int X +Int 31 <Int Y => true requires 0 <=Int X andBool X +Int 32 <=Int Y [simplification, concrete(Y)]

    rule notMaxUInt5 &Int X +Int 31 <Int X +Int 32 => true requires 0 <=Int X [simplification]
```

## `#asWord`

Lemmas about [`#asWord`](https://github.com/runtimeverification/evm-semantics/blob/master/kevm-pyk/src/kevm_pyk/kproj/evm-semantics/evm-types.md#bytes-helper-functions). `#asWord(B)` interprets the byte array `B` as a single word (with MSB first).

```k
    // Move to function parameters
    rule { #asWord ( BA1 ) #Equals #asWord ( BA2 ) } => #Top
      requires BA1 ==K BA2
      [simplification]

    // #asWord ignores leading zeros
    rule #asWord ( BA1 +Bytes BA2 ) => #asWord ( BA2 )
      requires #asInteger(BA1) ==Int 0
      [simplification, concrete(BA1)]

    // `#asWord` of a byte array cannot equal a number that cannot fit within the byte array
    rule #asWord ( BA ) ==Int Y => false
        requires lengthBytes(BA) <=Int 32
         andBool (2 ^Int (8 *Int lengthBytes(BA))) <=Int Y
        [concrete(Y), simplification]
```

## `#asInteger`

Lemmas about [`#asInteger`](https://github.com/runtimeverification/evm-semantics/blob/master/kevm-pyk/src/kevm_pyk/kproj/evm-semantics/evm-types.md#bytes-helper-functions). `#asInteger(X)` interprets the byte array `X` as a single arbitrary-precision integer (with MSB first).

```k
    // Conversion from bytes always yields a non-negative integer
    rule 0 <=Int #asInteger ( _ ) => true [simplification]
```

## `#padRightToWidth`

Lemmas about [`#padRightToWidth`](https://github.com/runtimeverification/evm-semantics/blob/master/kevm-pyk/src/kevm_pyk/kproj/evm-semantics/evm-types.md#bytes-helper-functions). `#padRightToWidth(W, BA)` right-pads the byte array `BA` with zeros so that the resulting byte array has length `W`.

```k
    // Definitional expansion
    rule #padRightToWidth (W, BA) => BA +Bytes #buf(W -Int lengthBytes(BA), 0)
      [concrete(W), simplification]
```

## `#range(BA, START, WIDTH)`

Lemmas about [`#range(BA, START, WIDTH)`](https://github.com/runtimeverification/evm-semantics/blob/master/kevm-pyk/src/kevm_pyk/kproj/evm-semantics/evm-types.md#bytes-helper-functions). `#range(BA, START, WIDTH)` returns the range of `BA` from index `START` of width `WIDTH`.

```k
    // Parameter equality
    rule { #range (BA, S, W1) #Equals #range (BA, S, W2) } => #Top
      requires W1 ==Int W2
      [simplification]
```

## Byte array indexing and update

Lemmas about [`BA [ I ]` and `BA1 [ S := BA2 ]`](https://github.com/runtimeverification/evm-semantics/blob/master/kevm-pyk/src/kevm_pyk/kproj/evm-semantics/evm-types.md#element-access). `BA [ I ]` returns the integer representation of the `I`-th byte of byte array `BA`. `BA1 [ S := BA2 ]` updates the byte array `BA1` with byte array `BA2` from index `S`.


```k
    // Byte indexing in terms of #asWord
    rule BA [ X ] => #asWord ( #range (BA, X, 1) )
      requires X <=Int lengthBytes(BA)
      [simplification(40)]

    // Empty update has no effect
    rule BA [ START := b"" ] => BA
      requires 0 <=Int START andBool START <=Int lengthBytes(BA)
      [simplification]

    // Update passes to right operand of concat if start position is beyond the left operand
    rule ( BA1 +Bytes BA2 ) [ S := BA ] => BA1 +Bytes ( BA2 [ S -Int lengthBytes(BA1) := BA ] )
      requires lengthBytes(BA1) <=Int S
      [simplification]

    // Consecutive quasi-contiguous byte-array update
    rule BA [ S1 := BA1 ] [ S2 := BA2 ] => BA [ S1 := #range(BA1, 0, S2 -Int S1) +Bytes BA2 ]
      requires 0 <=Int S1 andBool S1 <=Int S2 andBool S2 <=Int S1 +Int lengthBytes(BA1)
      [simplification]

    // Parameter equality: byte-array update
    rule { BA1:Bytes [ S1 := BA2 ] #Equals BA3:Bytes [ S2 := BA4 ] } => #Top
      requires BA1 ==K BA3 andBool S1 ==Int S2 andBool BA2 ==K BA4
      [simplification]
```

Summaries
---------

Functions summaries are rewrite rules that capture (summarize) the effects of executing a function. Such rules allow Kontrol to, instead of executing the function itself, just apply the summary rule.

## `copy_memory_to_memory` summary

The following rule summarises the behavior of the `copy_memory_to_memory` function. This function is automatically generated by the Solidity compiler. In its Yul form, it is as follows:

```solidity
function copy_memory_to_memory(src, dst, length) {
  let i := 0
  for { } lt(i, length) { i := add(i, 32) }
  {
    mstore(add(dst, i), mload(add(src, i)))
  }
  if gt(i, length)
  {
    // clear end
    mstore(add(dst, length), 0)
  }
}
```

It is used to copy `length` bytes of memory from index `src` to index `dest`, doing so in steps of 32 bytes, and right-padding with zeros to a multiple of 32.

Following the compiler constraints, we enforce a limit on the length of byte arrays and indices into byte arrays.

```k
    syntax Int ::= "maxBytesLength" [alias]
    rule maxBytesLength => 9223372036854775808
```

The summary lemma is as follows, with commentary inlined:

```k
    rule [copy-memory-to-memory-summary]:
      <k> #execute ... </k>
      <useGas> false </useGas>
      <schedule> SHANGHAI </schedule>
      <jumpDests> JUMPDESTS </jumpDests>
      // The program and program counter are symbolic, focusing on the part we will be executing (CP)
      <program> PROGRAM </program>
      <pc> PCOUNT => PCOUNT +Int 53 </pc>
      // The word stack has the appropriate form, as per the compiled code
      <wordStack> LENGTH : _ : SRC : DEST : WS </wordStack>
      // The program copies LENGTH bytes of memory from SRC +Int 32 to DEST +Int OFFSET,
      // padded with 32 zeros in case LENGTH is not divisible by 32
      <localMem>
        LM => LM [ DEST +Int 32 := #range ( LM, SRC +Int 32, LENGTH ) +Bytes
                                   #buf ( ( ( notMaxUInt5 &Int ( LENGTH +Int maxUInt5 ) ) -Int LENGTH ) , 0 ) +Bytes
                                   #buf ( ( ( ( 32 -Int ( ( notMaxUInt5 &Int ( LENGTH +Int maxUInt5 ) ) -Int LENGTH ) ) ) modInt 32 ), 0 ) ]
      </localMem>
      requires
       // The current program we are executing differs from the original one only in the hardcoded jump addresses,
       // which are now relative to PCOUNT, and the hardcoded offset, which is now symbolic.
               #range(PROGRAM, PCOUNT, 53) ==K b"`\x00[\x81\x81\x10\x15b\x00\x81`W` \x81\x85\x01\x81\x01Q\x86\x83\x01\x82\x01R\x01b\x00\x81BV[\x81\x81\x11\x15b\x00\x81sW`\x00` \x83\x87\x01\x01R[P"
                                               [ 08 := #buf(3, PCOUNT +Int 32) ]
                                               [ 28 := #buf(3, PCOUNT +Int  2) ]
                                               [ 38 := #buf(3, PCOUNT +Int 51) ]

       // Various well-formedness constraints. In particular, the maxBytesLength-related ones are present to
       // remove various chops that would otherwise creep into the execution, and are reasonable since byte
       // arrays in actual programs would never reach that size.
       andBool 0 <=Int PCOUNT
       andBool 0 <=Int LENGTH andBool LENGTH <Int maxBytesLength
       andBool 0 <=Int SRC    andBool SRC    <Int maxBytesLength
       andBool 0 <=Int DEST   andBool DEST   <Int maxBytesLength
       andBool #sizeWordStack(WS) <=Int 1015
       andBool SRC +Int LENGTH <=Int DEST // No overlap between source and destination
       andBool DEST <=Int lengthBytes(LM) // Destination starts within current memory
       andBool PCOUNT +Int 51 <Int lengthBytes(JUMPDESTS) // We are not looking outside of the JUMPDESTs bytearray
       // All JUMPDESTs in the program are valid
       andBool JUMPDESTS[PCOUNT +Int 2] ==Int 1 andBool JUMPDESTS[PCOUNT +Int 32] ==Int 1 andBool JUMPDESTS[PCOUNT +Int 51] ==Int 1
       andBool PCOUNT +Int 51 <Int 2 ^Int 24  // and fit into three bytes
      [priority(30), concrete(JUMPDESTS, PROGRAM, PCOUNT), preserves-definedness]

endmodule
```

This summary is required to enable reasoning about byte arrays or arbitrary (symbolic) length. Otherwise, we would have to deal with a loop as the Solidity compiler copies memory to memory in chunks of 32 bytes at a time, and as this loop would have a symbolic bound, the symbolic execution would either have to be bounded or would not terminate.

Unfortunately, the Solidity compiler optimizes the compiled bytecode in unpredictable ways, meaning that changes in the test suite can affect the compilation of `copy_memory_to_memory`. In light of this, and in order to be able to use our summary, we opt against using the `Test` contract of `forge-std`.

The looping issue has been recognized as such by the Solidity developers, and starting from version [0.8.24](https://soliditylang.org/blog/2024/01/26/solidity-0.8.24-release-announcement/) EVM comes with an `MCOPY` instruction ([EIP-5656](https://eips.ethereum.org/EIPS/eip-5656)), which copies a part of memory to another part of memory as an atomic action. If the development were to move to this (or higher) version of the compiler, there would be no need for this summary.
