// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { Vm } from "forge-std/Vm.sol";

library Process {
    /// @notice Error for when an ffi command fails.
    error FfiFailed(string);

    /// @notice Foundry cheatcode VM.
    Vm private constant vm = Vm(address(uint160(uint256(keccak256("hevm cheat code")))));

    /// @notice Executes a bash command in a subprocess and returns its output as a string. Will revert if the command
    /// returns no output.
    /// @param _command The bash command to execute
    function bash(string memory _command) internal returns (string memory stdout_) {
        stdout_ = bash({ _command: _command, _allowEmpty: false });
    }

    /// @notice Executes a bash command in a subprocess and returns its output as a string. Will 'optionally' revert if
    /// the command returns no output.
    /// @param _command The bash command to execute
    /// @param _allowEmpty Allow empty output.
    function bash(string memory _command, bool _allowEmpty) internal returns (string memory stdout_) {
        string[] memory command = new string[](3);
        command[0] = "bash";
        command[1] = "-c";
        command[2] = _command;
        stdout_ = string(run({ _command: command, _allowEmpty: _allowEmpty }));
    }

    /// @notice Run a command in a subprocess. Fails if no output is returned.
    /// @param _command Command to run.
    function run(string[] memory _command) internal returns (bytes memory stdout_) {
        stdout_ = run({ _command: _command, _allowEmpty: false });
    }

    /// @notice Run a command in a subprocess.
    /// @param _command Command to run.
    /// @param _allowEmpty Allow empty output.
    function run(string[] memory _command, bool _allowEmpty) internal returns (bytes memory stdout_) {
        Vm.FfiResult memory result = vm.tryFfi(_command);
        string memory command;
        for (uint256 i = 0; i < _command.length; i++) {
            command = string.concat(command, _command[i], " ");
        }
        if (result.exitCode != 0) {
            revert FfiFailed(string.concat("Command: ", command, "\nError: ", string(result.stderr)));
        }
        // If the output is empty, result.stdout is "[]".
        if (!_allowEmpty && keccak256(result.stdout) == keccak256(bytes("[]"))) {
            revert FfiFailed(string.concat("No output from Command: ", command));
        }
        stdout_ = result.stdout;
    }
}
