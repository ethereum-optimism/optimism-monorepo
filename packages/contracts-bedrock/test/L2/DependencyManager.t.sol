// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { CommonTest } from "test/setup/CommonTest.sol";
import { IDependencyManager } from "interfaces/L2/IDependencyManager.sol";
import { Constants } from "src/libraries/Constants.sol";
import { Unauthorized } from "src/libraries/errors/CommonErrors.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";
import { IL2ToL1MessagePasser } from "interfaces/L2/IL2ToL1MessagePasser.sol";
import { ISuperchainConfigInterop } from "interfaces/L1/ISuperchainConfigInterop.sol";

contract DependencyManager_Base_Test is CommonTest {
    event DependencyAdded(uint256 indexed chainId, address indexed systemConfig, address indexed superchainConfig);

    function setUp() public virtual override {
        super.enableInterop();
        super.setUp();
    }

    function _dependencyManager() internal pure returns (IDependencyManager) {
        return IDependencyManager(Predeploys.DEPENDENCY_MANAGER);
    }
}

contract DependencyManager_AddDependency_Test is DependencyManager_Base_Test {
    /// @dev Tests that addDependency succeeds when called by the depositor
    function testFuzz_addDependency_succeeds(uint256 _chainId) public {
        vm.assume(_chainId != block.chainid);

        // Expect call to L2ToL1MessagePasser.initiateWithdrawal
        vm.expectCall(
            address(payable(Predeploys.L2_TO_L1_MESSAGE_PASSER)),
            abi.encodeCall(
                IL2ToL1MessagePasser.initiateWithdrawal,
                (
                    address(superchainConfig),
                    400_000,
                    abi.encodeCall(ISuperchainConfigInterop.addDependency, (_chainId, address(systemConfig)))
                )
            )
        );

        // Expect DependencyAdded to be emitted
        vm.expectEmit(address(_dependencyManager()));
        emit DependencyAdded(_chainId, address(systemConfig), address(superchainConfig));

        // Call addDependency
        vm.prank(Constants.DEPOSITOR_ACCOUNT);
        _dependencyManager().addDependency(address(superchainConfig), _chainId, address(systemConfig));

        assertTrue(_dependencyManager().isInDependencySet(_chainId));
        assertEq(_dependencyManager().dependencySetSize(), 1);
    }

    /// @dev Tests that addDependency reverts when called by non-depositor
    function testFuzz_addDependency_unauthorized_reverts(address _caller, uint256 _chainId) public {
        vm.assume(_caller != Constants.DEPOSITOR_ACCOUNT);

        vm.prank(_caller);
        vm.expectRevert(Unauthorized.selector);
        _dependencyManager().addDependency(address(superchainConfig), _chainId, address(systemConfig));
    }

    /// @dev Tests that addDependency reverts when adding duplicate dependency
    function testFuzz_addDependency_duplicateDependency_reverts(uint256 _chainId) public {
        vm.assume(_chainId != block.chainid);

        vm.startPrank(Constants.DEPOSITOR_ACCOUNT);

        _dependencyManager().addDependency(address(superchainConfig), _chainId, address(systemConfig));

        vm.expectRevert(IDependencyManager.AlreadyDependency.selector);
        _dependencyManager().addDependency(address(superchainConfig), _chainId, address(systemConfig));

        vm.stopPrank();
    }

    /// @dev Tests that addDependency reverts when adding current chain ID
    function test_addDependency_currentChainId_reverts() public {
        vm.prank(Constants.DEPOSITOR_ACCOUNT);

        vm.expectRevert(IDependencyManager.AlreadyDependency.selector);
        _dependencyManager().addDependency(address(superchainConfig), block.chainid, address(systemConfig));
    }

    /// @dev Tests that addDependency reverts when dependency set is too large
    function test_addDependency_setTooLarge_reverts() public {
        vm.startPrank(Constants.DEPOSITOR_ACCOUNT);

        // Add maximum number of dependencies (255)
        uint256 i;
        for (i = 1; i <= type(uint8).max; i++) {
            _dependencyManager().addDependency(address(superchainConfig), i, address(systemConfig));
        }

        vm.expectRevert(IDependencyManager.DependencySetSizeTooLarge.selector);
        _dependencyManager().addDependency(address(superchainConfig), i + 1, address(systemConfig));

        vm.stopPrank();
    }
}

contract DependencyManager_IsInDependencySet_Test is DependencyManager_Base_Test {
    /// @dev Tests that current chain is always in dependency set
    function testFuzz_isInDependencySet_currentChain_succeeds(uint256 _chainId) public {
        vm.assume(_chainId <= type(uint8).max);

        vm.chainId(_chainId);
        assertTrue(_dependencyManager().isInDependencySet(_chainId));
    }

    /// @dev Tests that added chains are in dependency set
    function testFuzz_isInDependencySet_addedChain_succeeds(uint256 _chainId) public {
        vm.assume(_chainId != block.chainid);

        vm.prank(Constants.DEPOSITOR_ACCOUNT);
        _dependencyManager().addDependency(address(superchainConfig), _chainId, address(systemConfig));

        assertTrue(_dependencyManager().isInDependencySet(_chainId));
    }

    /// @dev Tests that non-added chains are not in dependency set
    function testFuzz_isInDependencySet_nonAddedChain_succeeds(uint256 _chainId) public view {
        vm.assume(_chainId != block.chainid);
        assertFalse(_dependencyManager().isInDependencySet(_chainId));
    }
}

contract DependencyManager_DependencySet_Test is DependencyManager_Base_Test {
    /// @dev Tests that dependencySet returns correct values
    function testFuzz_dependencySet_succeeds(uint256[] memory _chainIds) public {
        vm.assume(_chainIds.length <= type(uint8).max);

        // Ensure chain IDs are unique and valid
        for (uint256 i = 0; i < _chainIds.length; i++) {
            _chainIds[i] = bound(_chainIds[i], 1, type(uint8).max);
            vm.assume(_chainIds[i] != block.chainid);
            for (uint256 j = 0; j < i; j++) {
                vm.assume(_chainIds[i] != _chainIds[j]);
            }
        }

        vm.startPrank(Constants.DEPOSITOR_ACCOUNT);

        // Add dependencies
        for (uint256 i = 0; i < _chainIds.length; i++) {
            _dependencyManager().addDependency(address(superchainConfig), _chainIds[i], address(systemConfig));
        }

        uint256[] memory deps = _dependencyManager().dependencySet();
        assertEq(deps.length, _chainIds.length);

        // Verify each chain ID is in the dependency set
        for (uint256 i = 0; i < _chainIds.length; i++) {
            assertTrue(_dependencyManager().isInDependencySet(_chainIds[i]));
        }

        vm.stopPrank();
    }

    /// @dev Tests that dependencySetSize returns correct value
    function testFuzz_dependencySetSize_succeeds(uint256[] memory _chainIds) public {
        vm.assume(_chainIds.length <= type(uint8).max);

        // Ensure chain IDs are unique and valid
        for (uint256 i = 0; i < _chainIds.length; i++) {
            _chainIds[i] = bound(_chainIds[i], 1, type(uint8).max);
            vm.assume(_chainIds[i] != block.chainid);
            for (uint256 j = 0; j < i; j++) {
                vm.assume(_chainIds[i] != _chainIds[j]);
            }
        }

        vm.startPrank(Constants.DEPOSITOR_ACCOUNT);

        for (uint256 i = 0; i < _chainIds.length; i++) {
            _dependencyManager().addDependency(address(superchainConfig), _chainIds[i], address(systemConfig));
            assertEq(_dependencyManager().dependencySetSize(), i + 1);
        }

        vm.stopPrank();
    }
}
