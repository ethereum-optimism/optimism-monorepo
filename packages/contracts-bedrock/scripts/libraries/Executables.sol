// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { Vm } from "forge-std/Vm.sol";
import { Process } from "scripts/libraries/Process.sol";

/// @notice The executables used in ffi commands. These are set here
///         to have a single source of truth in case absolute paths
///         need to be used.
library Executables {
    /// @notice Foundry cheatcode VM.
    Vm private constant vm = Vm(address(uint160(uint256(keccak256("hevm cheat code")))));
    string internal constant bash = "bash";
    string internal constant jq = "jq";
    string internal constant forge = "forge";
    string internal constant echo = "echo";
    string internal constant sed = "sed";
    string internal constant find = "find";
    string internal constant ls = "ls";
    string internal constant git = "git";

    /// @notice Returns the commit hash of HEAD. If no git repository is
    /// found, it will return the contents of the .gitcommit file. Otherwise,
    /// it will return an error. The .gitcommit file is used to store the
    /// git commit of the contracts when they are packaged into docker images
    /// in order to avoid the need to have a git repository in the image.
    function gitCommitHash() internal returns (string memory) {
        return Process.bash("cast abi-encode 'f(string)' $(git rev-parse HEAD || cat .gitcommit)");
    }
}
