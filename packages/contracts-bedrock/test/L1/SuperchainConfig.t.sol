// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { CommonTest } from "test/setup/CommonTest.sol";

// Target contract dependencies
import { IProxy } from "src/universal/interfaces/IProxy.sol";
import { Unauthorized } from "src/libraries/errors/CommonErrors.sol";

// Target contract
import { ISuperchainConfig } from "src/L1/interfaces/ISuperchainConfig.sol";
import { SuperchainConfig, ISharedLockbox, ISystemConfigInterop } from "src/L1/SuperchainConfig.sol";

import { DeployUtils } from "scripts/libraries/DeployUtils.sol";
import { EnumerableSet } from "@openzeppelin/contracts/utils/structs/EnumerableSet.sol";

/// @notice For testing purposes contract, with setters to facilitate replicating complex scenarios when needed.
contract SuperchainConfigForTest is SuperchainConfig {
    using EnumerableSet for EnumerableSet.UintSet;

    constructor(address _sharedLockbox) SuperchainConfig(_sharedLockbox) { }

    function forTest_addChainOnDependencySet(uint256 _chainId) external {
        _dependencySet.add(_chainId);
    }

    function forTest_addChainAndSystemConfig(uint256 _chainId, address _systemConfig) external {
        _dependencySet.add(_chainId);
        systemConfigs[_chainId] = _systemConfig;
    }

    function forTest_setGuardian(address _guardian) external {
        bytes32 slot = GUARDIAN_SLOT;
        assembly {
            sstore(slot, _guardian)
        }
    }
}

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
                _args: DeployUtils.encodeConstructor(
                    abi.encodeCall(ISuperchainConfig.__constructor__, (address(sharedLockbox)))
                )
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

contract SuperchainConfig_AddChain_Test is CommonTest {
    event ChainAdded(uint256 indexed chainId, address indexed systemConfig, address indexed portal);

    function _mockAndExpect(address _target, bytes memory _calldata, bytes memory _returnData) internal {
        vm.mockCall(_target, _calldata, _returnData);
        vm.expectCall(_target, _calldata);
    }

    /// @notice Tests that `addChain` reverts when called by an unauthorized address.
    function test_addChain_unauthorized_reverts(address _caller, uint256 _chainId, address _systemConfig) external {
        vm.assume(_caller != superchainConfig.guardian());

        vm.expectRevert(Unauthorized.selector);
        vm.prank(_caller);
        superchainConfig.addChain(_chainId, _systemConfig);
    }

    /// @notice Tests that `addChain` reverts when the chain is already in the dependency set.
    function test_addChain_chainAlreadyExists_reverts(uint256 _chainId, address _systemConfig) external {
        SuperchainConfigForTest superchainConfig = new SuperchainConfigForTest(address(sharedLockbox));
        superchainConfig.forTest_addChainOnDependencySet(_chainId);

        vm.startPrank(superchainConfig.guardian());
        vm.expectRevert(SuperchainConfig.ChainAlreadyAdded.selector);
        superchainConfig.addChain(_chainId, _systemConfig);
    }

    /// @notice Tests that `addChain` successfully adds a chain to the dependency set when it is empty.
    function test_addChain_onEmptyDependencySet_succeeds(uint256 _chainId, address _portal) external {
        vm.assume(!superchainConfig.isInDependencySet(_chainId));

        // Store the PORTAL address we expect to be used in a call in the SystemConfig OptimsimPortal slot, and expect
        // it to be called
        vm.store(
            address(systemConfig),
            bytes32(uint256(keccak256("systemconfig.optimismportal")) - 1),
            bytes32(uint256(uint160(_portal)))
        );
        vm.expectCall(address(systemConfig), abi.encodeWithSelector(ISystemConfigInterop.optimismPortal.selector));

        // Mock and expect the call to authorize the portal on the SharedLockbox with the `_portal` address
        _mockAndExpect(
            address(sharedLockbox), abi.encodeWithSelector(ISharedLockbox.authorizePortal.selector, _portal), ""
        );

        // Expect the `addDependency` function call to not be called since the dependency set is empty
        uint64 zeroCalls = 0;
        vm.expectCall(
            address(systemConfig), abi.encodeWithSelector(ISystemConfigInterop.addDependency.selector), zeroCalls
        );

        // Expect the ChainAdded event to be emitted
        vm.expectEmit(address(superchainConfig));
        emit ChainAdded(_chainId, address(systemConfig), _portal);

        // Add the new chain to the dependency set
        vm.startPrank(superchainConfig.guardian());
        superchainConfig.addChain(_chainId, address(systemConfig));

        // Check that the new chain is in the dependency set
        assertTrue(superchainConfig.isInDependencySet(_chainId));
    }

    /// @notice Tests that `addChain` successfully adds a chain to the dependency set when it is not empty.
    ///         This tests deploys a new SuperchainConfigForTest contract and mocks several calls regarding SystemConfig
    ///         and SharedLockbox contracts of the added chains with the purpose of reducing test complexity and making
    ///         it more readable on the trade-off of getting a less realistic environment -- but finally checking the
    ///         logic that is being tested when having multiple dependencies.
    function test_addChain_withMultipleDependencies_succeeds(uint256 _chainId, address _portal) external {
        vm.assume(_chainId > 3);

        // Deploy a new SuperchainConfigForTest contract and set the address(sharedLockbox) and guardian addresses
        SuperchainConfigForTest superchainConfigForTest = new SuperchainConfigForTest(address(sharedLockbox));
        superchainConfigForTest.forTest_setGuardian(superchainConfig.guardian());

        // Define the chains to be added to the dependency set
        (uint256 chainIdOne, address systemConfigOne) = (1, makeAddr("SystemConfigOne"));
        (uint256 chainIdTwo, address systemConfigTwo) = (2, makeAddr("SystemConfigTwo"));
        (uint256 chainIdThree, address systemConfigThree) = (3, makeAddr("SystemConfigThree"));

        // Add the first three chains to the dependency set
        superchainConfigForTest.forTest_addChainAndSystemConfig(chainIdOne, systemConfigOne);
        superchainConfigForTest.forTest_addChainAndSystemConfig(chainIdTwo, systemConfigTwo);
        superchainConfigForTest.forTest_addChainAndSystemConfig(chainIdThree, systemConfigThree);

        // Mock and expect the calls when looping through the first chain of the dependency set
        _mockAndExpect(
            systemConfigOne, abi.encodeWithSelector(ISystemConfigInterop.addDependency.selector, _chainId), ""
        );
        _mockAndExpect(
            address(systemConfig), abi.encodeWithSelector(ISystemConfigInterop.addDependency.selector, chainIdOne), ""
        );

        // Mock and expect the calls when looping through the second chain of the dependency set
        _mockAndExpect(
            systemConfigTwo, abi.encodeWithSelector(ISystemConfigInterop.addDependency.selector, _chainId), ""
        );
        _mockAndExpect(
            address(systemConfig), abi.encodeWithSelector(ISystemConfigInterop.addDependency.selector, chainIdTwo), ""
        );

        // Mock and expect the calls when looping through the third chain of the dependency set
        _mockAndExpect(
            systemConfigThree, abi.encodeWithSelector(ISystemConfigInterop.addDependency.selector, _chainId), ""
        );
        _mockAndExpect(
            address(systemConfig), abi.encodeWithSelector(ISystemConfigInterop.addDependency.selector, chainIdThree), ""
        );

        // Store the PORTAL address we expect to be used in a call in the SystemConfig's OptimsimPortal slot, and expect
        // it to be called
        vm.store(
            address(systemConfig),
            bytes32(uint256(keccak256("systemconfig.optimismportal")) - 1),
            bytes32(uint256(uint160(_portal)))
        );
        vm.expectCall(address(systemConfig), abi.encodeWithSelector(ISystemConfigInterop.optimismPortal.selector));

        // Mock and expect the call to authorize the portal on the SharedLockbox with the `_portal` address
        _mockAndExpect(
            address(sharedLockbox), abi.encodeWithSelector(ISharedLockbox.authorizePortal.selector, _portal), ""
        );

        // Expect the ChainAdded event to be emitted
        vm.expectEmit(address(superchainConfigForTest));
        emit ChainAdded(_chainId, address(systemConfig), _portal);

        // Add the new chain to the dependency set
        vm.prank(superchainConfigForTest.guardian());
        superchainConfigForTest.addChain(_chainId, address(systemConfig));

        // Check that the new chain is in the dependency set
        assertTrue(superchainConfigForTest.isInDependencySet(_chainId));
    }
}

contract SuperchainConfig_IsInDependencySet_Test is CommonTest {
    /// @dev Tests that `isInDependencySet` returns false when the chain is not in the dependency set. Checking if empty
    ///      to ensure that should always be false.
    function test_isInDependencySet_false_succeeds(uint256 _chainId) external view {
        assert(superchainConfig.dependencySet().length == 0);
        assertFalse(superchainConfig.isInDependencySet(_chainId));
    }

    /// @dev Tests that `isInDependencySet` returns true when the chain is in the dependency set.
    function test_isInDependencySet_true_succeeds(uint256 _chainId) external {
        SuperchainConfigForTest superchainConfig = new SuperchainConfigForTest(address(sharedLockbox));
        superchainConfig.forTest_addChainOnDependencySet(_chainId);

        assertTrue(superchainConfig.isInDependencySet(_chainId));
    }
}

contract SuperchainConfig_DependencySet_Test is CommonTest {
    using EnumerableSet for EnumerableSet.UintSet;

    EnumerableSet.UintSet internal chainIds;

    /// @notice Tests that the dependency set returns properly the dependencies added.
    function test_dependencySet_succeeds(uint256[] calldata _chainIdsArray) public {
        SuperchainConfigForTest superchainConfigForTest = new SuperchainConfigForTest(address(sharedLockbox));

        // Ensure there are no repeated values on the input array
        for (uint256 i; i < _chainIdsArray.length; i++) {
            chainIds.add(_chainIdsArray[i]);
        }

        // Add the dependencies to the dependency set
        for (uint256 i; i < chainIds.length(); i++) {
            superchainConfigForTest.forTest_addChainOnDependencySet(chainIds.at(i));
        }

        // Check that the dependency set has the same length as the dependencies
        uint256[] memory dependencySet = superchainConfigForTest.dependencySet();
        assertEq(dependencySet.length, chainIds.length());

        // Check that the dependency set has the same chain IDs as the dependencies
        for (uint256 i; i < chainIds.length(); i++) {
            assertEq(dependencySet[i], chainIds.at(i));
        }
    }
}
