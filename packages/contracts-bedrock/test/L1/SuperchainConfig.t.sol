// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { CommonTest } from "test/setup/CommonTest.sol";

// Target contract dependencies
import { IProxy } from "src/universal/interfaces/IProxy.sol";

// Target contract
import { ISuperchainConfig } from "src/L1/interfaces/ISuperchainConfig.sol";

import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

contract SuperchainConfig_Init_Test is CommonTest {
    /// @dev Tests that initialization sets the correct values. These are defined in CommonTest.sol.
    function test_initialize_unpaused_succeeds() external view {
        assertFalse(superchainConfig.paused());
        assertEq(superchainConfig.guardian(), deploy.cfg().superchainConfigGuardian());
    }

    /// @dev Tests that it can be intialized as paused.
    function test_initialize_paused_succeeds() external {
        IProxy newProxy = IProxy(
            DeployUtils.create1({
                _name: "Proxy",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IProxy.__constructor__, (alice)))
            })
        );
        ISuperchainConfig newImpl = ISuperchainConfig(
            DeployUtils.create1({
                _name: "SuperchainConfig",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(ISuperchainConfig.__constructor__, ()))
            })
        );

        vm.startPrank(alice);
        newProxy.upgradeToAndCall(
            address(newImpl),
            abi.encodeCall(ISuperchainConfig.initialize, (deploy.cfg().superchainConfigGuardian(), true))
        );

        assertTrue(ISuperchainConfig(address(newProxy)).paused());
        assertEq(ISuperchainConfig(address(newProxy)).guardian(), deploy.cfg().superchainConfigGuardian());
    }
}

contract SuperchainConfig_Pause_TestFail is CommonTest {
    /// @dev Tests that `pause` reverts when called by a non-guardian.
    function test_pause_notGuardian_reverts() external {
        assertFalse(superchainConfig.paused());

        assertTrue(superchainConfig.guardian() != alice);
        vm.expectRevert("SuperchainConfig: only guardian can pause");
        vm.prank(alice);
        superchainConfig.pause("identifier");

        assertFalse(superchainConfig.paused());
    }
}

contract SuperchainConfig_Pause_Test is CommonTest {
    /// @dev Tests that `pause` successfully pauses
    ///      when called by the guardian.
    function test_pause_succeeds() external {
        assertFalse(superchainConfig.paused());

        vm.expectEmit(address(superchainConfig));
        emit Paused("identifier");

        vm.prank(superchainConfig.guardian());
        superchainConfig.pause("identifier");

        assertTrue(superchainConfig.paused());
    }
}

contract SuperchainConfig_Unpause_TestFail is CommonTest {
    /// @dev Tests that `unpause` reverts when called by a non-guardian.
    function test_unpause_notGuardian_reverts() external {
        vm.prank(superchainConfig.guardian());
        superchainConfig.pause("identifier");
        assertEq(superchainConfig.paused(), true);

        assertTrue(superchainConfig.guardian() != alice);
        vm.expectRevert("SuperchainConfig: only guardian can unpause");
        vm.prank(alice);
        superchainConfig.unpause();

        assertTrue(superchainConfig.paused());
    }
}

contract SuperchainConfig_Unpause_Test is CommonTest {
    /// @dev Tests that `unpause` successfully unpauses
    ///      when called by the guardian.
    function test_unpause_succeeds() external {
        vm.startPrank(superchainConfig.guardian());
        superchainConfig.pause("identifier");
        assertEq(superchainConfig.paused(), true);

        vm.expectEmit(address(superchainConfig));
        emit Unpaused();
        superchainConfig.unpause();

        assertFalse(superchainConfig.paused());
    }
}
