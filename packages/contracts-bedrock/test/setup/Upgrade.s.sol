// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

// Testing
import { VmSafe } from "forge-std/Vm.sol";
import { console2 as console } from "forge-std/console2.sol";
import { stdJson } from "forge-std/StdJson.sol";

// Scripts
import { Deploy } from "scripts/deploy/Deploy.s.sol";
import { DeploySuperchainInput, DeploySuperchain, DeploySuperchainOutput } from "scripts/deploy/DeploySuperchain.s.sol";

// Helpers
// import { FFIInterface } from "test/setup/FFIInterface";

// Libraries
import { Types } from "scripts/libraries/Types.sol";
import { Process } from "scripts/libraries/Process.sol";

/// @title Upgrade
/// @notice Script used to deploy a bedrock system. The entire system is deployed within the `run` function.
///         To add a new contract to the system, add a public function that deploys that individual contract.
///         Then add a call to that function inside of `run`. Be sure to call the `save` function after each
///         deployment so that hardhat-deploy style artifacts can be generated using a call to `sync()`.
///         The `CONTRACT_ADDRESSES_PATH` environment variable can be set to a path that contains a JSON file full of
///         contract name to address pairs. That enables this script to be much more flexible in the way it is used.
///         This contract must not have constructor logic because it is set into state using `etch`.
contract Upgrade is Deploy {
    // using stdJson for string;

    ////////////////////////////////////////////////////////////////
    //                    SetUp and Run                           //
    ////////////////////////////////////////////////////////////////

    /// @notice Deploy all of the L1 contracts necessary for a full Superchain with a single Op Chain.
    function run() public override {
        console.log("Deploying fresh Superchain Shared contracts");
        deploySuperchain();
    }
}
