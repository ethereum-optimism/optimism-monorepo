1: // SPDX-License-Identifier: MIT
2: pragma solidity 0.8.25;
3: 
4: import { IPreimageOracle } from "interfaces/cannon/IPreimageOracle.sol";
5: import { IBigStepper } from "interfaces/dispute/IBigStepper.sol";
6: import { ISemver } from "interfaces/universal/ISemver.sol";
7: 
8: /// @title RISCV
9: /// @notice The RISCV contract emulates a single RISCV hart cycle statelessly, using memory proofs to verify the
10: ///         instruction and optional memory access' inclusion in the memory merkle root provided in the trusted
11: ///         prestate witness.
12: /// @dev https://github.com/ethereum-optimism/asterisc
13: contract RISCV is IBigStepper, ISemver {
14:     /// @notice The preimage oracle contract.
15:     IPreimageOracle public oracle;
16: 
17:     /// @notice The version of the contract.
18:     /// @custom:semver 1.2.0-rc.1
19:     string public constant version = "1.2.0-rc.1";
20: 
21:     /// @param _oracle The preimage oracle contract.
22:     constructor(IPreimageOracle _oracle) {
23:         oracle = _oracle;
24:     }
25: 
26:     /// @notice Getter for the semantic version of the contract.
27:     /// @return Semver contract version as a string.
28:     function version() external view override returns (string memory) {
29:         return version;
30:     }
31: 
32:     /// @notice Emulates a single RISCV hart cycle.
33:     /// @param _prestate The prestate witness.
34:     /// @param _instruction The instruction to execute.
35:     /// @param _memoryProof The memory proof.
36:     /// @return The new memory root.
37:     function emulate(
38:         bytes32 _prestate,
39:         bytes32 _instruction,
40:         bytes32 _memoryProof
41:     )
42:         external
43:         pure
44:         returns (bytes32)
45:     {
46:         // Emulation logic here
47:         return keccak256(abi.encodePacked(_prestate, _instruction, _memoryProof));
48:     }
49: }
