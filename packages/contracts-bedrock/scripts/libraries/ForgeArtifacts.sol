// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { Vm } from "forge-std/Vm.sol";
import { stdJson } from "forge-std/StdJson.sol";
import { LibString } from "@solady/utils/LibString.sol";
import { Process } from "scripts/libraries/Process.sol";

/// @notice Contains information about a storage slot. Mirrors the layout of the storage
///         slot object in Forge artifacts so that we can deserialize JSON into this struct.
struct StorageSlot {
    uint256 astId;
    string _contract;
    string label;
    uint256 offset;
    string slot;
    string _type;
}

struct AbiEntry {
    string fnName;
    bytes4 sel;
}

struct Abi {
    string contractName;
    AbiEntry[] entries;
}

/// @title ForgeArtifacts
/// @notice Library for interacting with the forge artifacts.
library ForgeArtifacts {
    /// @notice Foundry cheatcode VM.
    Vm private constant vm = Vm(address(uint160(uint256(keccak256("hevm cheat code")))));

    /// @notice Removes the semantic versioning from a contract name. The semver will exist if the contract is compiled
    /// more than once with different versions of the compiler.
    function _stripSemver(string memory _name) internal returns (string memory out_) {
        out_ = Process.bash(string.concat("echo ", _name, " | sed -E 's/[.][0-9]+\\.[0-9]+\\.[0-9]+//g'"));
    }

    /// @notice Builds the fully qualified name of a contract. Assumes that the
    ///         file name is the same as the contract name but strips semver for the file name.
    function _getFullyQualifiedName(string memory _name) internal returns (string memory out_) {
        string memory sanitized = _stripSemver(_name);
        out_ = string.concat(sanitized, ".sol:", _name);
    }

    /// @notice Returns the storage layout for a deployed contract.
    function getStorageLayout(string memory _name) internal returns (string memory layout_) {
        layout_ = Process.bash(string.concat("jq -r '.storageLayout' < ", _getForgeArtifactPath(_name)));
    }

    /// @notice Returns the abi from a the forge artifact
    function getAbi(string memory _name) internal returns (string memory abi_) {
        abi_ = Process.bash(string.concat("jq -r '.abi' < ", _getForgeArtifactPath(_name)));
    }

    /// @notice Returns the methodIdentifiers from the forge artifact
    function getMethodIdentifiers(string memory _name) internal returns (string[] memory ids_) {
        string memory res = Process.bash({
            _command: string.concat("jq '.methodIdentifiers // {} | keys ' < ", _getForgeArtifactPath(_name)),
            _allowEmpty: true
        });
        ids_ = stdJson.readStringArray(res, "");
    }

    /// @notice Returns the kind of contract (i.e. library, contract, or interface).
    /// @param _name The name of the contract to get the kind of.
    /// @return kind_ The kind of contract ("library", "contract", or "interface").
    function getContractKind(string memory _name) internal returns (string memory kind_) {
        kind_ = Process.bash(
            string.concat(
                "jq -r '.ast.nodes[] | select(.nodeType == \"ContractDefinition\") | .contractKind' < ",
                _getForgeArtifactPath(_name)
            )
        );
    }

    /// @notice Returns whether or not a contract is proxied.
    /// @param _name The name of the contract to check.
    /// @return out_ Whether or not the contract is proxied.
    function isProxiedContract(string memory _name) internal returns (bool out_) {
        // TODO: Using the `@custom:proxied` tag is to determine if a contract is meant to be
        // proxied is functional but developers can easily forget to add the tag when writing a new
        // contract. We should consider determining whether a contract is proxied based on the
        // deployment script since it's the source of truth for that. Current deployment script
        // does not make this easy but an updated script should likely make this possible.
        string memory res = Process.bash(
            string.concat(
                "jq -r '.rawMetadata' ",
                _getForgeArtifactPath(_name),
                " | jq -r '.output.devdoc' | jq -r 'has(\"custom:proxied\")'"
            )
        );
        out_ = stdJson.readBool(res, "");
    }

    /// @notice Returns whether or not a contract is predeployed.
    /// @param _name The name of the contract to check.
    /// @return out_ Whether or not the contract is predeployed.
    function isPredeployedContract(string memory _name) internal returns (bool out_) {
        // TODO: Similar to the above, using the `@custom:predeployed` tag is not reliable but
        // functional for now. Deployment script should make this easier to determine.
        string memory res = Process.bash(
            string.concat(
                "jq -r '.rawMetadata' ",
                _getForgeArtifactPath(_name),
                " | jq -r '.output.devdoc' | jq -r 'has(\"custom:predeploy\")'"
            )
        );
        out_ = stdJson.readBool(res, "");
    }

    function _getForgeArtifactDirectory(string memory _name) internal returns (string memory dir_) {
        string memory res = Process.bash("forge config --json | jq -r .out");
        string memory contractName = _stripSemver(_name);
        dir_ = string.concat(vm.projectRoot(), "/", string(res), "/", contractName, ".sol");
    }

    /// @notice Returns the filesystem path to the artifact path. If the contract was compiled
    ///         with multiple solidity versions then return the first one based on the result of `ls`.
    function _getForgeArtifactPath(string memory _name) internal returns (string memory out_) {
        string memory directory = _getForgeArtifactDirectory(_name);
        string memory path = string.concat(directory, "/", _name, ".json");
        if (vm.exists(path)) {
            return path;
        }

        string memory res = Process.bash(
            string.concat("ls -1 --color=never ", directory, " | jq -R -s -c 'split(\"\n\") | map(select(length > 0))'")
        );
        string[] memory files = stdJson.readStringArray(res, "");
        out_ = string.concat(directory, "/", files[0]);
    }

    /// @notice Returns the forge artifact given a contract name.
    function _getForgeArtifact(string memory _name) internal returns (string memory out_) {
        string memory forgeArtifactPath = _getForgeArtifactPath(_name);
        out_ = vm.readFile(forgeArtifactPath);
    }

    /// @notice Pulls the `_initialized` storage slot information from the Forge artifacts for a given contract.
    function getInitializedSlot(string memory _contractName) internal returns (StorageSlot memory slot_) {
        string memory storageLayout = getStorageLayout(_contractName);

        // FaultDisputeGame and PermissionedDisputeGame use a different name for the initialized storage slot.
        string memory slotName = "_initialized";
        string memory slotType = "t_uint8";
        if (LibString.eq(_contractName, "FaultDisputeGame") || LibString.eq(_contractName, "PermissionedDisputeGame")) {
            slotName = "initialized";
            slotType = "t_bool";
        }

        bytes memory rawSlot = vm.parseJson(
            Process.bash(
                string.concat(
                    "echo '",
                    storageLayout,
                    "' | jq '.storage[] | select(.label == \"",
                    slotName,
                    "\" and .type == \"",
                    slotType,
                    "\")'"
                )
            )
        );
        slot_ = abi.decode(rawSlot, (StorageSlot));
    }

    /// @notice Returns whether or not a contract is initialized.
    ///         Needs the name to get the storage layout.
    function isInitialized(string memory _name, address _address) internal returns (bool initialized_) {
        StorageSlot memory slot = ForgeArtifacts.getInitializedSlot(_name);
        bytes32 slotVal = vm.load(_address, bytes32(vm.parseUint(slot.slot)));
        initialized_ = uint8((uint256(slotVal) >> (slot.offset * 8)) & 0xFF) != 0;
    }

    /// @notice Returns the names of all contracts in a given directory.
    /// @param _path The path to search for contracts.
    /// @param _pathExcludes An array of paths to exclude from the search.
    /// @return contractNames_ An array of contract names.
    function getContractNames(
        string memory _path,
        string[] memory _pathExcludes
    )
        internal
        returns (string[] memory contractNames_)
    {
        string memory pathExcludesPat;
        for (uint256 i = 0; i < _pathExcludes.length; i++) {
            pathExcludesPat = string.concat(pathExcludesPat, " -path \"", _pathExcludes[i], "\"");
            if (i != _pathExcludes.length - 1) {
                pathExcludesPat = string.concat(pathExcludesPat, " -o ");
            }
        }

        contractNames_ = abi.decode(
            vm.parseJson(
                Process.bash(
                    string.concat(
                        "find ",
                        _path,
                        bytes(pathExcludesPat).length > 0 ? string.concat(" ! \\( ", pathExcludesPat, " \\)") : "",
                        " -type f -exec basename {} \\; | sed 's/\\.[^.]*$//' | jq -R -s 'split(\"\n\")[:-1]'"
                    )
                )
            ),
            (string[])
        );
    }

    /// @notice Returns the function ABIs of all L1 contracts.
    function getContractFunctionAbis(
        string memory _path,
        string[] memory _pathExcludes
    )
        internal
        returns (Abi[] memory abis_)
    {
        string[] memory contractNames = getContractNames(_path, _pathExcludes);
        abis_ = new Abi[](contractNames.length);

        for (uint256 i; i < contractNames.length; i++) {
            string memory contractName = contractNames[i];
            string[] memory methodIdentifiers = getMethodIdentifiers(contractName);
            abis_[i].contractName = contractName;
            abis_[i].entries = new AbiEntry[](methodIdentifiers.length);
            for (uint256 j; j < methodIdentifiers.length; j++) {
                string memory fnName = methodIdentifiers[j];
                bytes4 sel = bytes4(keccak256(abi.encodePacked(fnName)));
                abis_[i].entries[j] = AbiEntry({ fnName: fnName, sel: sel });
            }
        }
    }

    /// @notice Accepts a filepath and then ensures that the directory
    ///         exists for the file to live in.
    function ensurePath(string memory _path) internal {
        string[] memory outputs = vm.split(_path, "/");
        string memory path = "";
        for (uint256 i = 0; i < outputs.length - 1; i++) {
            path = string.concat(path, outputs[i], "/");
        }
        vm.createDir(path, true);
    }
}
