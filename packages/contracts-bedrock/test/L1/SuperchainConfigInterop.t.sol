// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { CommonTest } from "test/setup/CommonTest.sol";

// Target contract dependencies
import { IProxy } from "interfaces/universal/IProxy.sol";
import { Unauthorized } from "src/libraries/errors/CommonErrors.sol";

// Target contract
import { ISuperchainConfigInterop } from "interfaces/L1/ISuperchainConfigInterop.sol";
import { SuperchainConfigInterop, ISystemConfig, IOptimismPortalInterop } from "src/L1/SuperchainConfigInterop.sol";

import { DeployUtils } from "scripts/libraries/DeployUtils.sol";
import { EnumerableSet } from "@openzeppelin/contracts/utils/structs/EnumerableSet.sol";

contract SuperchainConfigInterop_Base_Test is CommonTest {
    function setUp() public virtual override {
        super.enableInterop();
        super.setUp();
    }

    function _superchainConfigInterop() internal view returns (SuperchainConfigInterop) {
        return SuperchainConfigInterop(address(superchainConfig));
    }

    function _mockAndExpect(address _target, bytes memory _calldata, bytes memory _returnData) internal {
        vm.mockCall(_target, _calldata, _returnData);
        vm.expectCall(_target, _calldata);
    }

    function _setUpPortal(uint256 _chainId) internal returns (address portal_) {
        portal_ = address(bytes20(keccak256(abi.encodePacked(_chainId))));

        // Mock the portal to return the correct superchain config address
        _mockAndExpect(
            portal_, abi.encodeCall(IOptimismPortalInterop.superchainConfig, ()), abi.encode(address(superchainConfig))
        );
        _mockAndExpect(portal_, abi.encodeCall(IOptimismPortalInterop.migrateLiquidity, ()), abi.encode());

        // Store the PORTAL address we expect to be used in a call in the SystemConfig OptimsimPortal slot, and expect
        // it to be called
        vm.store(
            address(systemConfig),
            bytes32(uint256(keccak256("systemconfig.optimismportal")) - 1),
            bytes32(uint256(uint160(portal_)))
        );
        vm.expectCall(address(systemConfig), abi.encodeCall(ISystemConfig.optimismPortal, ()));
    }
}

contract SuperchainConfigInterop_Init_Test is SuperchainConfigInterop_Base_Test {
    function setUp() public virtual override {
        super.setUp();
        skipIfForkTest("SuperchainConfig_Init_Test: cannot test initialization on forked network");
    }

    /// @dev Tests that initialization sets the correct values. These are defined in CommonTest.sol.
    function test_initialize_succeeds() external view {
        assertFalse(_superchainConfigInterop().paused());
        assertEq(_superchainConfigInterop().guardian(), deploy.cfg().superchainConfigGuardian());
        assertEq(_superchainConfigInterop().clusterManager(), deploy.cfg().finalSystemOwner());
        assertEq(address(_superchainConfigInterop().sharedLockbox()), address(sharedLockbox));
    }

    /// @dev Tests that it can be intialized as paused.
    function test_initialize_paused_succeeds() external {
        IProxy newProxy = IProxy(
            DeployUtils.create1({
                _name: "Proxy",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IProxy.__constructor__, (alice)))
            })
        );
        ISuperchainConfigInterop newImpl = ISuperchainConfigInterop(
            DeployUtils.create1({
                _name: "SuperchainConfigInterop",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(ISuperchainConfigInterop.__constructor__, ()))
            })
        );

        vm.startPrank(alice);
        newProxy.upgradeToAndCall(
            address(newImpl),
            abi.encodeCall(
                ISuperchainConfigInterop.initialize,
                (deploy.cfg().superchainConfigGuardian(), true, deploy.cfg().finalSystemOwner(), address(sharedLockbox))
            )
        );

        assertTrue(ISuperchainConfigInterop(address(newProxy)).paused());
        assertEq(ISuperchainConfigInterop(address(newProxy)).guardian(), deploy.cfg().superchainConfigGuardian());
        assertEq(ISuperchainConfigInterop(address(newProxy)).clusterManager(), deploy.cfg().finalSystemOwner());
        assertEq(address(ISuperchainConfigInterop(address(newProxy)).sharedLockbox()), address(sharedLockbox));
    }
}

contract SuperchainConfigInterop_AddDependency_Test is SuperchainConfigInterop_Base_Test {
    event DependencyAdded(uint256 indexed chainId, address indexed systemConfig, address indexed portal);

    /// @notice Tests that `addDependency` reverts when called by an unauthorized address.
    function test_addDependency_unauthorized_reverts(
        address _caller,
        uint256 _chainId,
        address _systemConfig
    )
        external
    {
        vm.assume(_caller != _superchainConfigInterop().clusterManager());

        vm.expectRevert(Unauthorized.selector);
        vm.prank(_caller);
        _superchainConfigInterop().addDependency(_chainId, _systemConfig);
    }

    /// @notice Tests that `addDependency` reverts when the dependency set is too large.
    function test_addDependency_dependencySetTooLarge_reverts() external {
        vm.startPrank(_superchainConfigInterop().clusterManager());
        uint256 currentSize = _superchainConfigInterop().dependencySetSize();

        // Add the maximum number of dependencies to the dependency set
        uint256 i;
        for (i; i < type(uint8).max - currentSize; i++) {
            _setUpPortal(i);
            _superchainConfigInterop().addDependency(i, address(systemConfig));
        }

        // Check that the dependency set is full and that expect the next call to revert
        assertEq(_superchainConfigInterop().dependencySetSize(), type(uint8).max);
        vm.expectRevert(SuperchainConfigInterop.DependencySetTooLarge.selector);

        // Try to add another dependency to the dependency set
        uint256 chainId = i + 1;
        _superchainConfigInterop().addDependency(chainId, address(systemConfig));

        vm.stopPrank();
    }

    /// @notice Tests that `addDependency` reverts when the chain is already in the dependency set.
    function test_addDependency_chainAlreadyExists_reverts(uint256 _chainId) external {
        vm.assume(_chainId != block.chainid);

        // Mock the portal
        _setUpPortal(_chainId);

        vm.startPrank(_superchainConfigInterop().clusterManager());
        _superchainConfigInterop().addDependency(_chainId, address(systemConfig));

        vm.expectRevert(SuperchainConfigInterop.DependencyAlreadyAdded.selector);
        _superchainConfigInterop().addDependency(_chainId, address(systemConfig));
        vm.stopPrank();
    }

    /// @notice Tests that `addDependency` successfully adds a chain to the dependency set calling it from the cluster
    /// manager.
    function test_addDependencyFromClusterManager_succeeds(uint256 _chainId) external {
        vm.assume(!_superchainConfigInterop().isInDependencySet(_chainId));
        uint256 currentSize = _superchainConfigInterop().dependencySetSize();

        address portal = _setUpPortal(_chainId);

        // Expect the DependencyAdded event to be emitted
        vm.expectEmit(address(superchainConfig));
        emit DependencyAdded(_chainId, address(systemConfig), portal);

        // Add the new chain to the dependency set
        vm.prank(_superchainConfigInterop().clusterManager());
        _superchainConfigInterop().addDependency(_chainId, address(systemConfig));

        // Check that the new chain is in the dependency set
        assertTrue(_superchainConfigInterop().isInDependencySet(_chainId));
        assertEq(_superchainConfigInterop().dependencySetSize(), currentSize + 1);
        assertTrue(_superchainConfigInterop().authorizedPortals(portal));
    }

    /// @notice Tests that `addDependency` reverts when the caller is not the cluster manager or an authorized portal.
    function test_addDependency_notClusterManager_reverts(address _caller, uint256 _chainId) external {
        vm.assume(_caller != _superchainConfigInterop().clusterManager());

        vm.expectRevert(Unauthorized.selector);
        vm.prank(_caller);
        _superchainConfigInterop().addDependency(_chainId, address(systemConfig));
    }

    /// @notice Tests that `addDependency` reverts when the portal has an invalid superchain config address.
    function test_addDependency_portalInvalidSuperchainConfig_reverts(
        uint256 _chainId,
        address _superchainConfig
    )
        external
    {
        vm.assume(_chainId != block.chainid);
        vm.assume(_superchainConfig != address(superchainConfig));

        address portal = address(bytes20(keccak256(abi.encodePacked(_chainId))));

        // Store the PORTAL address we expect to be used in a call in the SystemConfig OptimsimPortal slot, and expect
        // it to be called
        vm.store(
            address(systemConfig),
            bytes32(uint256(keccak256("systemconfig.optimismportal")) - 1),
            bytes32(uint256(uint160(portal)))
        );

        // Mock the portal to return a different superchain config address
        _mockAndExpect(
            portal, abi.encodeCall(IOptimismPortalInterop.superchainConfig, ()), abi.encode(_superchainConfig)
        );

        vm.prank(_superchainConfigInterop().clusterManager());
        vm.expectRevert(SuperchainConfigInterop.InvalidSuperchainConfig.selector);
        _superchainConfigInterop().addDependency(_chainId, address(systemConfig));
    }

    /// @notice Tests that `addDependency` reverts when the portal is already authorized.
    function test_addDependency_portalAlreadyAuthorized_reverts(uint256 _chainId, uint256 _otherChainId) external {
        // Bound chainId to be within uint128 range but not equal to block.chainid
        _chainId = bound(_chainId, 1, type(uint128).max);
        _otherChainId = bound(_otherChainId, 1, type(uint128).max);
        vm.assume(_chainId != block.chainid);
        vm.assume(_otherChainId != block.chainid);
        vm.assume(_chainId != _otherChainId);

        _setUpPortal(_chainId);

        // Add first an authorized portal
        vm.prank(_superchainConfigInterop().clusterManager());
        _superchainConfigInterop().addDependency(_chainId, address(systemConfig));

        vm.prank(_superchainConfigInterop().clusterManager());
        vm.expectRevert(SuperchainConfigInterop.PortalAlreadyAuthorized.selector);
        _superchainConfigInterop().addDependency(_otherChainId, address(systemConfig));
    }

    /// @notice Tests that `addDependency` reverts when the superchain is paused.
    function test_addDependency_paused_reverts(uint256 _chainId) external {
        // Set up portal and pause the superchain
        vm.prank(_superchainConfigInterop().guardian());
        _superchainConfigInterop().pause("test pause");

        // Try to add dependency while paused
        vm.prank(_superchainConfigInterop().clusterManager());
        vm.expectRevert(SuperchainConfigInterop.SuperchainPaused.selector);
        _superchainConfigInterop().addDependency(_chainId, address(systemConfig));
    }
}

contract SuperchainConfigInterop_IsInDependencySet_Test is SuperchainConfigInterop_Base_Test {
    /// @dev Tests that `isInDependencySet` returns false when the chain is not in the dependency set. Checking if empty
    ///      to ensure that should always be false.
    function test_isInDependencySet_false_succeeds(uint256 _chainId) external view {
        vm.assume(_chainId != deploy.cfg().l2ChainID());
        assertFalse(_superchainConfigInterop().isInDependencySet(_chainId));
    }

    /// @dev Tests that `isInDependencySet` returns true when the chain is in the dependency set.
    function test_isInDependencySet_true_succeeds(uint256 _chainId) external {
        vm.assume(_chainId != block.chainid);
        _setUpPortal(_chainId);

        vm.prank(_superchainConfigInterop().clusterManager());
        _superchainConfigInterop().addDependency(_chainId, address(systemConfig));

        assertTrue(_superchainConfigInterop().isInDependencySet(_chainId));
    }
}

contract SuperchainConfigInterop_DependencySet_Test is SuperchainConfigInterop_Base_Test {
    using EnumerableSet for EnumerableSet.UintSet;

    EnumerableSet.UintSet internal chainIds;
    uint256 currentSize;

    function setUp() public virtual override {
        super.setUp();
        currentSize = _superchainConfigInterop().dependencySetSize();
    }

    function _addDependencies(uint256[] calldata _chainIdsArray) internal {
        vm.assume(_chainIdsArray.length <= type(uint8).max - currentSize);

        // Ensure there are no repeated values on the input array
        for (uint256 i; i < _chainIdsArray.length; i++) {
            if (_chainIdsArray[i] != block.chainid) chainIds.add(_chainIdsArray[i]);
        }

        vm.startPrank(_superchainConfigInterop().clusterManager());

        // Add the dependencies to the dependency set
        for (uint256 i; i < chainIds.length(); i++) {
            _setUpPortal(i);
            _superchainConfigInterop().addDependency(chainIds.at(i), address(systemConfig));
        }

        vm.stopPrank();
    }

    /// @notice Tests that the dependency set returns properly the dependencies added.
    function test_dependencySet_succeeds(uint256[] calldata _chainIdsArray) public {
        _addDependencies(_chainIdsArray);

        // Check that the dependency set has the same length as the dependencies
        uint256[] memory dependencySet = _superchainConfigInterop().dependencySet();
        assertEq(dependencySet.length, chainIds.length() + currentSize);

        // Check that the dependency set has the same chain IDs as the dependencies
        for (uint256 i; i < chainIds.length(); i++) {
            assertEq(dependencySet[i + currentSize], chainIds.at(i));
        }
    }

    /// @notice Tests that the dependency set size returns properly the number of dependencies added.
    function test_dependencySetSize_succeeds(uint256[] calldata _chainIdsArray) public {
        _addDependencies(_chainIdsArray);

        // Check that the dependency set has the same length as the dependencies
        assertEq(_superchainConfigInterop().dependencySetSize(), chainIds.length() + currentSize);
    }
}
