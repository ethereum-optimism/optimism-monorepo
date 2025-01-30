// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { Vm, VmSafe } from "forge-std/Vm.sol";

/// @notice Enum of forks available for selection when generating genesis allocs.
enum Fork {
    REGOLITH,
    CANYON,
    DELTA,
    ECOTONE,
    FJORD,
    GRANITE,
    HOLOCENE,
    ISTHMUS,
    INTEROP
}

Fork constant LATEST_FORK = Fork.ISTHMUS;

library ForkUtils {
    function toString(Fork _fork) internal pure returns (string memory) {
        if (_fork == Fork.REGOLITH) {
            return "regolith";
        } else if (_fork == Fork.CANYON) {
            return "canyon";
        } else if (_fork == Fork.DELTA) {
            return "delta";
        } else if (_fork == Fork.ECOTONE) {
            return "ecotone";
        } else if (_fork == Fork.FJORD) {
            return "fjord";
        } else if (_fork == Fork.GRANITE) {
            return "granite";
        } else if (_fork == Fork.HOLOCENE) {
            return "holocene";
        } else if (_fork == Fork.ISTHMUS) {
            return "isthmus";
        } else if (_fork == Fork.INTEROP) {
            return "interop";
        } else {
            return "unknown";
        }
    }
}

/// @title Config
/// @notice Contains all env var based config. Add any new env var parsing to this file
///         to ensure that all config is in a single place.
library Config {
    /// @notice Foundry cheatcode VM.
    Vm private constant vm = Vm(address(uint160(uint256(keccak256("hevm cheat code")))));

    /// @notice Returns the path on the local filesystem where the deployment artifact is
    ///         written to disk after doing a deployment.
    function deploymentOutfile() internal view returns (string memory env_) {
        env_ = vm.envOr(
            "DEPLOYMENT_OUTFILE",
            string.concat(vm.projectRoot(), "/deployments/", vm.toString(block.chainid), "-deploy.json")
        );
    }

    /// @notice Returns the path on the local filesystem where the deploy config is
    function deployConfigPath() internal view returns (string memory env_) {
        if (vm.isContext(VmSafe.ForgeContext.TestGroup)) {
            env_ = string.concat(vm.projectRoot(), "/deploy-config/hardhat.json");
        } else {
            env_ = vm.envOr("DEPLOY_CONFIG_PATH", string(""));
            require(bytes(env_).length > 0, "Config: must set DEPLOY_CONFIG_PATH to filesystem path of deploy config");
        }
    }

    /// @notice Returns the chainid from the EVM context or the value of the CHAIN_ID env var as
    ///         an override.
    function chainID() internal view returns (uint256 env_) {
        env_ = vm.envOr("CHAIN_ID", block.chainid);
    }

    /// @notice The CREATE2 salt to be used when deploying the implementations.
    function implSalt() internal view returns (string memory env_) {
        env_ = vm.envOr("IMPL_SALT", string("ethers phoenix"));
    }

    /// @notice Returns the path that the state dump file should be written to or read from
    ///         on the local filesystem.
    function stateDumpPath(string memory _suffix) internal view returns (string memory env_) {
        env_ = vm.envOr(
            "STATE_DUMP_PATH",
            string.concat(vm.projectRoot(), "/state-dump-", vm.toString(block.chainid), _suffix, ".json")
        );
    }

    /// @notice Returns the name of the file that the forge deployment artifact is written to on the local
    ///         filesystem. By default, it is the name of the deploy script with the suffix `-latest.json`.
    ///         This was useful for creating hardhat deploy style artifacts and will be removed in a future release.
    function deployFile(string memory _sig) internal view returns (string memory env_) {
        env_ = vm.envOr("DEPLOY_FILE", string.concat(_sig, "-latest.json"));
    }

    /// @notice Returns the private key that is used to configure drippie.
    function drippieOwnerPrivateKey() internal view returns (uint256 env_) {
        env_ = vm.envUint("DRIPPIE_OWNER_PRIVATE_KEY");
    }

    /// @notice Returns true if multithreaded Cannon is used for the deployment.
    function useMultithreadedCannon() internal view returns (bool enabled_) {
        enabled_ = vm.envOr("USE_MT_CANNON", false);
    }
}
