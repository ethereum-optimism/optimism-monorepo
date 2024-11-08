// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Script } from "forge-std/Script.sol";
import { stdJson } from "forge-std/StdJson.sol";
import { console2 as console } from "forge-std/console2.sol";
import { Process } from "scripts/libraries/Process.sol";

contract SemverLock is Script {
    function run() public {
        // First, find all contracts with a Semver inheritance.
        string memory rawFiles =
            Process.bash("grep -rl '@custom:semver' src | jq -Rs 'split(\"\\n\") | map(select(length > 0))'");

        string[] memory files = vm.parseJsonStringArray(rawFiles, "");
        writeSemverLock(files);
    }

    /// @dev Writes a Semver lockfile
    function writeSemverLock(string[] memory _files) internal {
        string memory out;
        for (uint256 i; i < _files.length; i++) {
            // Use FFI to read the file to remove the need for FS permissions in the foundry.toml.
            string[] memory commands = new string[](2);
            commands[0] = "cat";
            commands[1] = _files[i];
            string memory fileContents = string(Process.run(commands));

            // Grab the contract name
            string memory contractName =
                Process.bash(string.concat("echo \"", _files[i], "\"| sed -E 's|src/.*/(.+)\\.sol|\\1|'"));

            string memory artifactsDir = Process.bash("forge config --json | jq -r .out");

            // Handle the case where there are multiple artifacts for a contract. This happens
            // when the same contract is compiled with multiple compiler versions.
            string memory contractArtifactDir = string.concat(artifactsDir, "/", contractName, ".sol");
            string memory artifactFiles = Process.bash(
                string.concat(
                    "ls -1 --color=never ",
                    contractArtifactDir,
                    " | jq -R -s -c 'split(\"\n\") | map(select(length > 0))'"
                )
            );

            string[] memory files = stdJson.readStringArray(artifactFiles, "");
            require(files.length > 0, string.concat("SemverLock: no artifacts found for ", contractName));
            string memory fileName = files[0];

            // Parse the artifact to get the contract's initcode hash.
            bytes memory initCode = vm.getCode(string.concat(artifactsDir, "/", contractName, ".sol/", fileName));

            // Serialize the initcode hash + sourcecode hash in JSON.
            vm.serializeBytes32(_files[i], "initCodeHash", keccak256(initCode));
            string memory obj = vm.serializeBytes32(_files[i], "sourceCodeHash", keccak256(bytes(fileContents)));

            // Serialize the map from the file name -> initcode hash + sourcecode hash in JSON.
            out = vm.serializeString("", _files[i], obj);
        }

        // Write the semver lockfile.
        vm.writeJson(out, "semver-lock.json");
        console.logString("Wrote semver lock file to \"semver-lock.json\".");
    }
}
