// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import {
    DeployOwnership,
    SafeConfig,
    SecurityCouncilConfig,
    GuardianConfig,
    DeputyGuardianModuleConfig,
    LivenessModuleConfig
} from "scripts/deploy/DeployOwnership.s.sol";
import { Test } from "forge-std/Test.sol";

import { GnosisSafe as Safe } from "safe-contracts/GnosisSafe.sol";
import { ModuleManager } from "safe-contracts/base/ModuleManager.sol";

import { LivenessGuard } from "src/safe/LivenessGuard.sol";
import { LivenessModule } from "src/safe/LivenessModule.sol";
import { DeputyGuardianModule } from "src/safe/DeputyGuardianModule.sol";

contract DeployOwnershipTest is Test, DeployOwnership {
    address internal constant SENTINEL_MODULES = address(0x1);
    // keccak256("guard_manager.guard.address")
    bytes32 internal constant GUARD_STORAGE_SLOT = 0x4a204f620c8c5ccdca3fd54d003badd85ba500436a431f0cbda4f558c93c34c8;

    function setUp() public override {
        super.setUp();
        run();
    }

    /// @dev Helper function to make assertions on basic Safe config properties.
    function _checkSafeConfig(SafeConfig memory _safeConfig, Safe _safe) internal view {
        assertEq(_safe.getThreshold(), _safeConfig.threshold);

        address[] memory safeOwners = _safe.getOwners();
        assertEq(_safeConfig.owners.length, safeOwners.length);
        assertFalse(_safe.isOwner(msg.sender));
        for (uint256 i = 0; i < safeOwners.length; i++) {
            assertEq(safeOwners[i], _safeConfig.owners[i]);
        }
    }

    /// @dev Test the example Foundation Safe configurations, against the expected configuration, and
    ///     check that they both have the same configuration.
    function test_exampleFoundationSafes_configuration_succeeds() public {
        Safe upgradeSafe = Safe(payable(mustGetAddress("FoundationUpgradeSafe")));
        Safe operationsSafe = Safe(payable(mustGetAddress("FoundationOperationsSafe")));
        SafeConfig memory exampleFoundationConfig = _getExampleFoundationConfig();

        // Ensure the safes both match the example configuration
        _checkSafeConfig(exampleFoundationConfig, upgradeSafe);
        _checkSafeConfig(exampleFoundationConfig, operationsSafe);

        // Sanity check to ensure the safes match each other's configuration
        assertEq(upgradeSafe.getThreshold(), operationsSafe.getThreshold());
        assertEq(upgradeSafe.getOwners().length, operationsSafe.getOwners().length);
    }

    /// @dev Test the example Security Council Safe configuration.
    function test_exampleSecurityCouncilSafe_configuration_succeeds() public {
        Safe securityCouncilSafe = Safe(payable(mustGetAddress("SecurityCouncilSafe")));
        SecurityCouncilConfig memory exampleSecurityCouncilConfig = _getExampleCouncilConfig();

        _checkSafeConfig(exampleSecurityCouncilConfig.safeConfig, securityCouncilSafe);

        // Guard Checks
        address livenessGuard = mustGetAddress("LivenessGuard");

        // The Safe's getGuard method is internal, so we read directly from storage
        // https://github.com/safe-global/safe-contracts/blob/v1.4.0/contracts/base/GuardManager.sol#L66-L72
        assertEq(vm.load(address(securityCouncilSafe), GUARD_STORAGE_SLOT), bytes32(uint256(uint160(livenessGuard))));

        // check that all the owners have a lastLive time in the Guard
        address[] memory owners = exampleSecurityCouncilConfig.safeConfig.owners;
        for (uint256 i = 0; i < owners.length; i++) {
            assertEq(LivenessGuard(livenessGuard).lastLive(owners[i]), block.timestamp);
        }

        // Module Checks
        address livenessModule = mustGetAddress("LivenessModule");
        (address[] memory modules, address nextModule) =
            ModuleManager(securityCouncilSafe).getModulesPaginated(SENTINEL_MODULES, 2);
        assertEq(modules.length, 1);
        assertEq(modules[0], livenessModule);
        assertEq(nextModule, SENTINEL_MODULES); // ensures there are no more modules in the list

        // LivenessModule checks
        LivenessModuleConfig memory lmConfig = exampleSecurityCouncilConfig.livenessModuleConfig;
        assertEq(address(LivenessModule(livenessModule).livenessGuard()), livenessGuard);
        assertEq(LivenessModule(livenessModule).livenessInterval(), lmConfig.livenessInterval);
        assertEq(LivenessModule(livenessModule).thresholdPercentage(), lmConfig.thresholdPercentage);
        assertEq(LivenessModule(livenessModule).minOwners(), lmConfig.minOwners);

        // Ensure the threshold on the safe agrees with the LivenessModule's required threshold
        assertEq(securityCouncilSafe.getThreshold(), LivenessModule(livenessModule).getRequiredThreshold(owners.length));
    }

    /// @dev Test the example Guardian Safe configuration.
    function test_exampleGuardianSafe_configuration_succeeds() public view {
        Safe guardianSafe = Safe(payable(mustGetAddress("GuardianSafe")));
        address[] memory owners = new address[](1);
        owners[0] = mustGetAddress("SecurityCouncilSafe");
        GuardianConfig memory guardianConfig = _getExampleGuardianConfig();
        _checkSafeConfig(guardianConfig.safeConfig, guardianSafe);

        // DeputyGuardianModule checks
        address deputyGuardianModule = mustGetAddress("DeputyGuardianModule");
        (address[] memory modules, address nextModule) =
            ModuleManager(guardianSafe).getModulesPaginated(SENTINEL_MODULES, 2);
        assertEq(modules.length, 1);
        assertEq(modules[0], deputyGuardianModule);
        assertEq(nextModule, SENTINEL_MODULES); // ensures there are no more modules in the list

        assertEq(
            DeputyGuardianModule(deputyGuardianModule).deputyGuardian(),
            guardianConfig.deputyGuardianModuleConfig.deputyGuardian
        );
        assertEq(
            address(DeputyGuardianModule(deputyGuardianModule).superchainConfig()),
            address(guardianConfig.deputyGuardianModuleConfig.superchainConfig)
        );
    }
}
