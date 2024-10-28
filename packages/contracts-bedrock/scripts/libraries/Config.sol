// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { Vm, VmSafe } from "forge-std/Vm.sol";

/// @notice Enum representing different ways of outputting genesis allocs.
/// @custom:value NONE    No output, used in internal tests.
/// @custom:value LATEST  Output allocs only for latest fork.
/// @custom:value ALL     Output allocs for all intermediary forks.
enum OutputMode {
    NONE,
    LATEST,
    ALL
}

library OutputModeUtils {
    function toString(OutputMode _mode) internal pure returns (string memory) {
        if (_mode == OutputMode.NONE) {
            return "none";
        } else if (_mode == OutputMode.LATEST) {
            return "latest";
        } else if (_mode == OutputMode.ALL) {
            return "all";
        } else {
            return "unknown";
        }
    }
}

/// @notice Enum of forks available for selection when generating genesis allocs.
enum Fork {
    NONE,
    DELTA,
    ECOTONE,
    FJORD,
    GRANITE,
    HOLOCENE
}

Fork constant LATEST_FORK = Fork.HOLOCENE;

library ForkUtils {
    function toString(Fork _fork) internal pure returns (string memory) {
        if (_fork == Fork.NONE) {
            return "none";
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

    /// @notice Returns the value of the env var CONTRACT_ADDRESSES_PATH which is a JSON key/value
    ///         pair of contract names and their addresses. Each key/value pair is passed to `save`
    ///         which then backs the `getAddress` function.
    function contractAddressesPath() internal view returns (string memory env_) {
        env_ = vm.envOr("CONTRACT_ADDRESSES_PATH", string(""));
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

    /// @notice Returns the OutputMode for genesis allocs generation.
    ///         It reads the mode from the environment variable OUTPUT_MODE.
    ///         If it is unset, OutputMode.ALL is returned.
    function outputMode() internal view returns (OutputMode) {
        string memory modeStr = vm.envOr("OUTPUT_MODE", string("latest"));
        bytes32 modeHash = keccak256(bytes(modeStr));
        if (modeHash == keccak256(bytes("none"))) {
            return OutputMode.NONE;
        } else if (modeHash == keccak256(bytes("latest"))) {
            return OutputMode.LATEST;
        } else if (modeHash == keccak256(bytes("all"))) {
            return OutputMode.ALL;
        } else {
            revert(string.concat("Config: unknown output mode: ", modeStr));
        }
    }

    /// @notice Returns true if multithreaded Cannon is used for the deployment.
    function useMultithreadedCannon() internal view returns (bool enabled_) {
        enabled_ = vm.envOr("USE_MT_CANNON", false);
    }

    /// @notice Returns the latest fork to use for genesis allocs generation.
    ///         It reads the fork from the environment variable FORK. If it is
    ///         unset, NONE is returned.
    ///         If set to the special value "latest", the latest fork is returned.
    function fork() internal view returns (Fork) {
        string memory forkStr = vm.envOr("FORK", string(""));
        if (bytes(forkStr).length == 0) {
            return Fork.NONE;
        }
        bytes32 forkHash = keccak256(bytes(forkStr));
        if (forkHash == keccak256(bytes("latest"))) {
            return LATEST_FORK;
        } else if (forkHash == keccak256(bytes("delta"))) {
            return Fork.DELTA;
        } else if (forkHash == keccak256(bytes("ecotone"))) {
            return Fork.ECOTONE;
        } else if (forkHash == keccak256(bytes("fjord"))) {
            return Fork.FJORD;
        } else if (forkHash == keccak256(bytes("granite"))) {
            return Fork.GRANITE;
        } else if (forkHash == keccak256(bytes("holocene"))) {
            return Fork.HOLOCENE;
        } else {
            revert(string.concat("Config: unknown fork: ", forkStr));
        }
    }
}
