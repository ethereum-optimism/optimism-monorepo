// SPDX-License-Identifier: MIT

// This file exists to provide the `internal constant vm` on top of `KontrolCheats`.
// The reason for explicitly defining `vm` here instead of inheriting Forge's `Test`
// contract is the K summary of the `copy_memory_to_memory` function.
// This summary dependent on the bytecode of the test contract, which means that if `Test`
// was inherited, updating the version of `Test` could potentially imply having to adjust
// said summary for the latest version, introducing a flakiness source.
// Note that starting with version 0.8.24, the opcode `MCOPY` is introduced, removing the
// need for the `copy_memory_to_memory` function and its summary, and thus this workaround.
// For more information refer to the `copy_memory_to_memory` summary section of `pausability-lemmas.md`.

pragma solidity 0.8.25;

import { Vm } from "forge-std/Vm.sol";
import { KontrolCheats } from "kontrol-cheatcodes/KontrolCheats.sol";

/// @notice Tests inheriting this contract cannot be run with forge
abstract contract KontrolUtils is KontrolCheats {
    Vm internal constant vm = Vm(address(uint160(uint256(keccak256("hevm cheat code")))));
}
