// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

// Testing
import { console } from "forge-std/console.sol";
import { Script } from "forge-std/Script.sol";

// Scripts
import { Artifacts } from "scripts/Artifacts.s.sol";
import { Config } from "scripts/libraries/Config.sol";
import { DeployConfig } from "scripts/deploy/DeployConfig.s.sol";
import { Process } from "scripts/libraries/Process.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

/// @title Deployer
/// @author tynes
/// @notice A contract that can make deploying and interacting with deployments easy.
abstract contract Deployer is Script {
    DeployConfig public constant cfg =
        DeployConfig(address(uint160(uint256(keccak256(abi.encode("optimism.deployconfig"))))));

    Artifacts public constant artifacts =
        Artifacts(address(uint160(uint256(keccak256(abi.encode("optimism.artifacts"))))));

    /// @notice Sets up the artifacts contract.
    function setUp() public virtual {
        DeployUtils.etchLabelAndAllowCheatcodes({ _etchTo: address(artifacts), _cname: "Artifacts" });
        artifacts.setUp();

        console.log("Commit hash: %s", gitCommitHash());

        DeployUtils.etchLabelAndAllowCheatcodes({ _etchTo: address(cfg), _cname: "DeployConfig" });
        cfg.read(Config.deployConfigPath());
    }

    /// @notice Returns the commit hash of HEAD. If no git repository is
    /// found, it will return the contents of the .gitcommit file. Otherwise,
    /// it will return an error. The .gitcommit file is used to store the
    /// git commit of the contracts when they are packaged into docker images
    /// in order to avoid the need to have a git repository in the image.
    function gitCommitHash() internal returns (string memory) {
        return Process.bash("cast abi-encode 'f(string)' $(git rev-parse HEAD || cat .gitcommit)");
    }
}
