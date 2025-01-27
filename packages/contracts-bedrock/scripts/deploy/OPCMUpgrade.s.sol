// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { Script } from "forge-std/Script.sol";
import { OPContractsManager } from "src/L1/OPContractsManager.sol";
import { console2 as console } from "forge-std/console2.sol";

contract OPCMUpgrade is Script {
    function run(
        address opcmAddress,
        OPContractsManager.OpChainConfig[] memory opChainConfigs
    ) external {
        // Basic validation
        require(opcmAddress != address(0), "invalid OPCM address");
        require(opChainConfigs.length > 0, "empty config array");

        vm.broadcast();

        // Cast the address to OPCM contract
        OPContractsManager opcm = OPContractsManager(opcmAddress);

        // Log the upgrade attempt
        console.log("calling upgrade on opcm at", opcmAddress);
        console.log("number of chains to upgrade:", opChainConfigs.length);

        // Call the upgrade function with the OpChainConfig array
        opcm.upgrade(opChainConfigs);

        console.log("opcm.upgrade completed");
    }
}
