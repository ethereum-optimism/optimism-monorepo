// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { console2 as console } from "forge-std/console2.sol";
import { Script } from "forge-std/Script.sol";

import { LibString } from "@solady/utils/LibString.sol";

import { Config } from "scripts/libraries/Config.sol";
import { DrippieConfig } from "scripts/periphery/drippie/DrippieConfig.s.sol";

import { Drippie } from "src/periphery/drippie/Drippie.sol";
import { IDripCheck } from "src/periphery/drippie/IDripCheck.sol";

/// @title ManageDrippie
/// @notice Script for managing drips in the Drippie contract.
contract ManageDrippie is Script {
    /// @notice Drippie configuration.
    DrippieConfig public cfg;

    /// @notice Modifier that wraps a function in broadcasting.
    modifier broadcast() {
        vm.startBroadcast(msg.sender);
        _;
        vm.stopBroadcast();
    }

    /// @notice Sets up the deployment script.
    function setUp() public {
        cfg = new DrippieConfig(Config.deployConfigPath());
        console.log("Config path: %s", Config.deployConfigPath());
    }

    /// @notice Runs the management script.
    function run() public {
        pauseDrips();
        installDrips();
    }

    /// @notice Pauses drips that have been removed from config.
    function pauseDrips() public broadcast {
        console.log("ManageDrippie: pausing removed drips");
        for (uint256 i = 0; i < cfg.drippie().getDripCount(); i++) {
            // Skip drips that aren't prefixed for this config file.
            string memory name = cfg.drippie().created(i);
            if (!LibString.startsWith(name, cfg.prefix())) {
                continue;
            }

            // Pause drips that are no longer in the config if not already paused.
            if (!cfg.names(name)) {
                // Pause the drip if it's active.
                if (cfg.drippie().getDripStatus(name) == Drippie.DripStatus.ACTIVE) {
                    console.log("ManageDrippie: pausing drip for %s", name);
                    cfg.drippie().status(name, Drippie.DripStatus.PAUSED);
                }
            }
        }
    }

    /// @notice Installs drips in the drippie contract.
    function installDrips() public broadcast {
        console.log("ManageDrippie: installing Drippie config for %s drips", cfg.dripsLength());
        for (uint256 i = 0; i < cfg.dripsLength(); i++) {
            DrippieConfig.FullDripConfig memory drip = abi.decode(cfg.drip(i), (DrippieConfig.FullDripConfig));
            Drippie.DripAction[] memory actions = new Drippie.DripAction[](1);
            actions[0] = Drippie.DripAction({ target: payable(drip.recipient), data: drip.data, value: drip.value });
            _installDrip({
                _drippie: cfg.drippie(),
                _name: drip.name,
                _config: Drippie.DripConfig({
                    reentrant: false,
                    interval: drip.interval,
                    dripcheck: IDripCheck(cfg.mustGetDripCheck(drip.dripcheck)),
                    checkparams: drip.checkparams,
                    actions: actions
                })
            });
        }
    }

    /// @notice Installs a drip in the drippie contract.
    /// @param _drippie The drippie contract.
    /// @param _name The name of the drip.
    /// @param _config The configuration of the drip.
    function _installDrip(Drippie _drippie, string memory _name, Drippie.DripConfig memory _config) internal {
        if (_drippie.getDripStatus(_name) == Drippie.DripStatus.NONE) {
            console.log("installing %s", _name);
            _drippie.create(_name, _config);
            console.log("%s installed successfully", _name);
        } else {
            console.log("%s already installed", _name);
        }

        // Grab the status again now that we've installed the drip.
        Drippie.DripStatus status = _drippie.getDripStatus(_name);
        if (status == Drippie.DripStatus.PAUSED) {
            console.log("activating %s", _name);
            _drippie.status(_name, Drippie.DripStatus.ACTIVE);
            console.log("%s activated successfully", _name);
        } else if (status == Drippie.DripStatus.ACTIVE) {
            console.log("%s already active", _name);
        } else {
            // TODO: Better way to handle this?
            console.log("WARNING: % could not be activated", _name);
        }
    }
}
