// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { CommonTest } from "test/setup/CommonTest.sol";

// Target contract dependencies
import { IProxy } from "interfaces/universal/IProxy.sol";
import { Unauthorized } from "src/libraries/errors/CommonErrors.sol";

// Target contract
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { SuperchainConfig, ISharedLockbox, ISystemConfig } from "src/L1/SuperchainConfig.sol";

import { DeployUtils } from "scripts/libraries/DeployUtils.sol";
import { EnumerableSet } from "@openzeppelin/contracts/utils/structs/EnumerableSet.sol";

contract SuperchainConfig_Init_Test is CommonTest {
    function setUp() public virtual override {
        super.setUp();
        skipIfForkTest("SuperchainConfig_Init_Test: cannot test initialization on forked network");
    }

    /// @dev Tests that initialization sets the correct values. These are defined in CommonTest.sol.
    function test_initialize_succeeds() external view {
        assertFalse(superchainConfig.paused());
        assertEq(superchainConfig.guardian(), deploy.cfg().superchainConfigGuardian());
        assertEq(superchainConfig.dependencyManager(), deploy.cfg().finalSystemOwner());
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
            abi.encodeCall(
                ISuperchainConfig.initialize,
                (deploy.cfg().superchainConfigGuardian(), deploy.cfg().finalSystemOwner(), true)
            )
        );

        assertTrue(ISuperchainConfig(address(newProxy)).paused());
        assertEq(ISuperchainConfig(address(newProxy)).guardian(), deploy.cfg().superchainConfigGuardian());
        assertEq(ISuperchainConfig(address(newProxy)).dependencyManager(), deploy.cfg().finalSystemOwner());
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

contract SuperchainConfig_AddDependency_Test is CommonTest {
    event DependencyAdded(uint256 indexed chainId, address indexed systemConfig, address indexed portal);

    function setUp() public virtual override {
        super.enableInterop();
        super.setUp();
    }

    function _mockAndExpect(address _target, bytes memory _calldata, bytes memory _returnData) internal {
        vm.mockCall(_target, _calldata, _returnData);
        vm.expectCall(_target, _calldata);
    }

    /// @notice Tests that `addDependency` reverts when called by an unauthorized address.
    function test_addDependency_unauthorized_reverts(
        address _caller,
        uint256 _chainId,
        address _systemConfig
    )
        external
    {
        vm.assume(_caller != superchainConfig.dependencyManager());

        vm.expectRevert(Unauthorized.selector);
        vm.prank(_caller);
        superchainConfig.addDependency(_chainId, _systemConfig);
    }

    /// @notice Tests that `addDependency` reverts when the dependency set is too large.
    function test_addDependency_dependencySetTooLarge_reverts() external {
        vm.startPrank(superchainConfig.dependencyManager());

        // Add the maximum number of dependencies to the dependency set
        uint256 i;
        for (i; i < type(uint8).max; i++) {
            superchainConfig.addDependency(i, address(systemConfig));
        }

        // Check that the dependency set is full and that expect the next call to revert
        assertEq(superchainConfig.dependencySetSize(), type(uint8).max);
        vm.expectRevert(SuperchainConfig.DependencySetTooLarge.selector);

        // Try to add another dependency to the dependency set
        uint256 chainId = i + 1;
        superchainConfig.addDependency(chainId, address(systemConfig));

        vm.stopPrank();
    }

    /// @notice Tests that `addDependency` reverts when the chain ID is the same as the current chain ID.
    function test_addDependency_sameChainID_reverts() external {
        vm.prank(superchainConfig.dependencyManager());
        vm.expectRevert(SuperchainConfig.InvalidChainID.selector);
        superchainConfig.addDependency(block.chainid, address(systemConfig));
    }

    /// @notice Tests that `addDependency` reverts when the chain is already in the dependency set.
    function test_addDependency_chainAlreadyExists_reverts(uint256 _chainId) external {
        vm.assume(_chainId != block.chainid);

        vm.startPrank(superchainConfig.dependencyManager());
        superchainConfig.addDependency(_chainId, address(systemConfig));

        vm.expectRevert(SuperchainConfig.DependencyAlreadyAdded.selector);
        superchainConfig.addDependency(_chainId, address(systemConfig));
        vm.stopPrank();
    }

    /// @notice Tests that `addDependency` successfully adds a chain to the dependency set when it is empty.
    function test_addDependency_onEmptyDependencySet_succeeds(uint256 _chainId, address _portal) external {
        vm.assume(!superchainConfig.isInDependencySet(_chainId));

        // Store the PORTAL address we expect to be used in a call in the SystemConfig OptimsimPortal slot, and expect
        // it to be called
        vm.store(
            address(systemConfig),
            bytes32(uint256(keccak256("systemconfig.optimismportal")) - 1),
            bytes32(uint256(uint160(_portal)))
        );
        vm.expectCall(address(systemConfig), abi.encodeCall(ISystemConfig.optimismPortal, ()));

        // Mock and expect the call to authorize the portal on the SharedLockbox with the `_portal` address
        vm.expectCall(address(sharedLockbox), abi.encodeCall(ISharedLockbox.authorizePortal, (_portal)));

        // Expect the DependencyAdded event to be emitted
        vm.expectEmit(address(superchainConfig));
        emit DependencyAdded(_chainId, address(systemConfig), _portal);

        // Add the new chain to the dependency set
        vm.prank(superchainConfig.dependencyManager());
        superchainConfig.addDependency(_chainId, address(systemConfig));

        // Check that the new chain is in the dependency set
        assertTrue(superchainConfig.isInDependencySet(_chainId));
        assertEq(superchainConfig.dependencySetSize(), 1);
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
        vm.assume(_chainId != block.chainid);
        vm.prank(superchainConfig.dependencyManager());
        superchainConfig.addDependency(_chainId, address(systemConfig));
        assertTrue(superchainConfig.isInDependencySet(_chainId));
    }
}

contract SuperchainConfig_DependencySet_Test is CommonTest {
    using EnumerableSet for EnumerableSet.UintSet;

    EnumerableSet.UintSet internal chainIds;

    function _addDependencies(uint256[] calldata _chainIdsArray) internal {
        vm.assume(_chainIdsArray.length <= type(uint8).max);

        // Ensure there are no repeated values on the input array
        for (uint256 i; i < _chainIdsArray.length; i++) {
            if (_chainIdsArray[i] != block.chainid) chainIds.add(_chainIdsArray[i]);
        }

        vm.startPrank(superchainConfig.dependencyManager());

        // Add the dependencies to the dependency set
        for (uint256 i; i < chainIds.length(); i++) {
            superchainConfig.addDependency(chainIds.at(i), address(systemConfig));
        }

        vm.stopPrank();
    }

    /// @notice Tests that the dependency set returns properly the dependencies added.
    function test_dependencySet_succeeds(uint256[] calldata _chainIdsArray) public {
        _addDependencies(_chainIdsArray);

        // Check that the dependency set has the same length as the dependencies
        uint256[] memory dependencySet = superchainConfig.dependencySet();
        assertEq(dependencySet.length, chainIds.length());

        // Check that the dependency set has the same chain IDs as the dependencies
        for (uint256 i; i < chainIds.length(); i++) {
            assertEq(dependencySet[i], chainIds.at(i));
        }
    }

    /// @notice Tests that the dependency set size returns properly the number of dependencies added.
    function test_dependencySetSize_succeeds(uint256[] calldata _chainIdsArray) public {
        _addDependencies(_chainIdsArray);

        // Check that the dependency set has the same length as the dependencies
        assertEq(superchainConfig.dependencySetSize(), chainIds.length());
    }
}
