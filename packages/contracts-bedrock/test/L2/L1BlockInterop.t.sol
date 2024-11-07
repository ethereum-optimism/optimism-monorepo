// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing utilities
import { CommonTest } from "test/setup/CommonTest.sol";

// Libraries
import { StaticConfig } from "src/libraries/StaticConfig.sol";

// Target contract dependencies
import { L1BlockInterop, ConfigType } from "src/L2/L1BlockInterop.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";
import "src/libraries/L1BlockErrors.sol";

contract L1BlockInteropTest is CommonTest {
    event GasPayingTokenSet(address indexed token, uint8 indexed decimals, bytes32 name, bytes32 symbol);
    event DependencyAdded(uint256 indexed chainId);
    event DependencyRemoved(uint256 indexed chainId);

    modifier prankDepositor() {
        vm.startPrank(_l1BlockInterop().DEPOSITOR_ACCOUNT());
        _;
        vm.stopPrank();
    }

    /// @notice Marked virtual to be overridden in
    ///         test/kontrol/deployment/DeploymentSummary.t.sol
    function setUp() public virtual override {
        super.enableInterop();
        super.setUp();
    }

    /// @dev Tests that an arbitrary chain ID can be added to the dependency set.
    function testFuzz_isInDependencySet_succeeds(uint256 _chainId) public prankDepositor {
        vm.assume(_chainId != block.chainid);

        _l1BlockInterop().setConfig(ConfigType.ADD_DEPENDENCY, StaticConfig.encodeAddDependency(_chainId));

        assertTrue(_l1BlockInterop().isInDependencySet(_chainId));
    }

    /// @dev Tests that `isInDependencySet` returns true when the chain's chain ID is passed as the input.
    function test_isInDependencySet_chainChainId_succeeds() public view {
        assertTrue(_l1BlockInterop().isInDependencySet(block.chainid));
    }

    /// @dev Tests that `isInDependencySet` reverts when the input chain ID is not in the dependency set
    ///      and is not the chain's chain ID.
    function testFuzz_isInDependencySet_notDependency_reverts(uint256 _chainId) public view {
        vm.assume(_chainId != block.chainid);

        // Check that the chain ID is not in the dependency set
        assertFalse(_l1BlockInterop().isInDependencySet(_chainId));
    }

    /// @dev Tests that `isInDependencySet` returns false when the dependency set is empty.
    function testFuzz_isInDependencySet_dependencySetEmpty_succeeds(uint256 _chainId) public view {
        vm.assume(_chainId != block.chainid);

        assertEq(_l1BlockInterop().dependencySetSize(), 0);

        assertFalse(_l1BlockInterop().isInDependencySet(_chainId));
    }

    /// @dev Tests that the dependency set size is correct when adding an arbitrary number of chain IDs.
    function testFuzz_dependencySetSize_succeeds(uint8 _dependencySetSize) public prankDepositor {
        vm.assume(_dependencySetSize <= type(uint8).max);

        uint256 uniqueCount = 0;

        for (uint256 i = 0; i < _dependencySetSize; i++) {
            if (i == block.chainid) continue;
            _l1BlockInterop().setConfig(ConfigType.ADD_DEPENDENCY, StaticConfig.encodeAddDependency(i));
            uniqueCount++;
        }

        assertEq(_l1BlockInterop().dependencySetSize(), uniqueCount);
    }

    /// @dev Tests that the dependency set size is correct when the dependency set is empty.
    function test_dependencySetSize_dependencySetEmpty_succeeds() public view {
        assertEq(_l1BlockInterop().dependencySetSize(), 0);
    }

    /// @dev Tests that the config for setting the gas paying token succeeds.
    function testFuzz_setConfig_gasPayingToken_succeeds(
        address _token,
        uint8 _decimals,
        bytes32 _name,
        bytes32 _symbol
    )
        public
        prankDepositor
    {
        vm.assume(_token != address(vm));

        vm.expectEmit(address(l1Block));
        emit GasPayingTokenSet({ token: _token, decimals: _decimals, name: _name, symbol: _symbol });

        _l1BlockInterop().setConfig(
            ConfigType.SET_GAS_PAYING_TOKEN,
            StaticConfig.encodeSetGasPayingToken({ _token: _token, _decimals: _decimals, _name: _name, _symbol: _symbol })
        );
    }

    /// @dev Tests that setting the gas paying token config as not the depositor reverts.
    function testFuzz_setConfig_gasPayingTokenButNotDepositor_reverts(
        address _token,
        uint8 _decimals,
        bytes32 _name,
        bytes32 _symbol
    )
        public
    {
        vm.assume(_token != address(vm));

        vm.expectRevert(NotDepositor.selector);
        _l1BlockInterop().setConfig(
            ConfigType.SET_GAS_PAYING_TOKEN,
            StaticConfig.encodeSetGasPayingToken({ _token: _token, _decimals: _decimals, _name: _name, _symbol: _symbol })
        );
    }

    /// @dev Tests that the config for adding a dependency can be set.
    function testFuzz_setConfig_addDependency_succeeds(uint256 _chainId) public prankDepositor {
        vm.assume(_chainId != block.chainid);

        vm.expectEmit(address(l1Block));
        emit DependencyAdded(_chainId);

        _l1BlockInterop().setConfig(ConfigType.ADD_DEPENDENCY, StaticConfig.encodeAddDependency(_chainId));
    }

    /// @dev Tests that adding a dependency reverts if it's the chain's chain id
    function test_setConfig_addDependencyButChainChainId_reverts() public prankDepositor {
        vm.expectRevert(AlreadyDependency.selector);
        _l1BlockInterop().setConfig(ConfigType.ADD_DEPENDENCY, StaticConfig.encodeAddDependency(block.chainid));
    }

    /// @dev Tests that adding a dependency already in the set reverts
    function test_setConfig_addDependencyButAlreadyDependency_reverts(uint256 _chainId) public prankDepositor {
        vm.assume(_chainId != block.chainid);

        _l1BlockInterop().setConfig(ConfigType.ADD_DEPENDENCY, StaticConfig.encodeAddDependency(_chainId));

        vm.expectRevert(AlreadyDependency.selector);
        _l1BlockInterop().setConfig(ConfigType.ADD_DEPENDENCY, StaticConfig.encodeAddDependency(_chainId));
    }

    /// @dev Tests that setting the add dependency config as not the depositor reverts.
    function testFuzz_setConfig_addDependencyButNotDepositor_reverts(uint256 _chainId) public {
        vm.expectRevert(NotDepositor.selector);
        _l1BlockInterop().setConfig(ConfigType.ADD_DEPENDENCY, StaticConfig.encodeAddDependency(_chainId));
    }

    /// @dev Tests that setting the add dependency config when the dependency set size is too large reverts.
    function test_setConfig_addDependencyButDependencySetSizeTooLarge_reverts() public prankDepositor {
        for (uint256 i = 0; i < type(uint8).max; i++) {
            _l1BlockInterop().setConfig(ConfigType.ADD_DEPENDENCY, StaticConfig.encodeAddDependency(i));
        }

        assertEq(_l1BlockInterop().dependencySetSize(), type(uint8).max);

        vm.expectRevert(DependencySetSizeTooLarge.selector);
        _l1BlockInterop().setConfig(ConfigType.ADD_DEPENDENCY, StaticConfig.encodeAddDependency(1));
    }

    /// @dev Tests that the config for removing a dependency can be set.
    function testFuzz_setConfig_removeDependency_succeeds(uint256 _chainId) public prankDepositor {
        vm.assume(_chainId != block.chainid);

        // Add the chain ID to the dependency set before removing it
        _l1BlockInterop().setConfig(ConfigType.ADD_DEPENDENCY, StaticConfig.encodeAddDependency(_chainId));

        vm.expectEmit(address(l1Block));
        emit DependencyRemoved(_chainId);

        _l1BlockInterop().setConfig(ConfigType.REMOVE_DEPENDENCY, StaticConfig.encodeRemoveDependency(_chainId));
    }

    /// @dev Tests that setting the remove dependency config as not the depositor reverts.
    function testFuzz_setConfig_removeDependencyButNotDepositor_reverts(uint256 _chainId) public {
        vm.expectRevert(NotDepositor.selector);
        _l1BlockInterop().setConfig(ConfigType.REMOVE_DEPENDENCY, StaticConfig.encodeRemoveDependency(_chainId));
    }

    /// @dev Tests that setting the remove dependency config for the chain's chain ID reverts.
    function test_setConfig_removeDependencyButChainChainId_reverts() public prankDepositor {
        vm.expectRevert(CantRemovedDependency.selector);
        _l1BlockInterop().setConfig(ConfigType.REMOVE_DEPENDENCY, StaticConfig.encodeRemoveDependency(block.chainid));
    }

    /// @dev Tests that setting the remove dependency config for a chain ID that is not in the dependency set reverts.
    function testFuzz_setConfig_removeDependencyButNotDependency_reverts(uint256 _chainId) public prankDepositor {
        vm.assume(_chainId != block.chainid);

        vm.expectRevert(NotDependency.selector);
        _l1BlockInterop().setConfig(ConfigType.REMOVE_DEPENDENCY, StaticConfig.encodeRemoveDependency(_chainId));
    }

    /// @dev Returns the L1BlockInterop instance.
    function _l1BlockInterop() internal view returns (L1BlockInterop) {
        return L1BlockInterop(address(l1Block));
    }
}

contract L1BlockInteropIsDeposit_Test is L1BlockInteropTest {
    /// @dev Tests that `isDeposit` reverts if the caller is not the cross L2 inbox.
    function test_isDeposit_notCrossL2Inbox_reverts(address _caller) external {
        vm.assume(_caller != Predeploys.CROSS_L2_INBOX);
        vm.expectRevert(NotCrossL2Inbox.selector);
        _l1BlockInterop().isDeposit();
    }

    /// @dev Tests that `isDeposit` always returns the correct value.
    function test_isDeposit_succeeds() external {
        // Assert is false if the value is not updated
        vm.prank(Predeploys.CROSS_L2_INBOX);
        assertEq(_l1BlockInterop().isDeposit(), false);

        /// @dev Assuming that `setL1BlockValuesInterop` will set the proper value. That function is tested as well
        vm.prank(_l1BlockInterop().DEPOSITOR_ACCOUNT());
        _l1BlockInterop().setL1BlockValuesInterop();

        // Assert is true if the value is updated
        vm.prank(Predeploys.CROSS_L2_INBOX);
        assertEq(_l1BlockInterop().isDeposit(), true);
    }
}

contract L1BlockInteropSetL1BlockValuesInterop_Test is L1BlockInteropTest {
    /// @dev Tests that `setL1BlockValuesInterop` reverts if sender address is not the depositor
    function test_setL1BlockValuesInterop_notDepositor_reverts(address _caller) external {
        vm.assume(_caller != _l1BlockInterop().DEPOSITOR_ACCOUNT());
        vm.prank(_caller);
        vm.expectRevert(NotDepositor.selector);
        _l1BlockInterop().setL1BlockValuesInterop();
    }

    /// @dev Tests that `setL1BlockValuesInterop` succeeds if sender address is the depositor
    function test_setL1BlockValuesInterop_succeeds(
        uint32 baseFeeScalar,
        uint32 blobBaseFeeScalar,
        uint64 sequenceNumber,
        uint64 timestamp,
        uint64 number,
        uint256 baseFee,
        uint256 blobBaseFee,
        bytes32 hash,
        bytes32 batcherHash
    )
        external
    {
        // Ensure the `isDepositTransaction` flag is false before calling `setL1BlockValuesInterop`
        vm.prank(Predeploys.CROSS_L2_INBOX);
        assertEq(_l1BlockInterop().isDeposit(), false);

        bytes memory setValuesEcotoneCalldata = abi.encodePacked(
            baseFeeScalar, blobBaseFeeScalar, sequenceNumber, timestamp, number, baseFee, blobBaseFee, hash, batcherHash
        );

        vm.prank(_l1BlockInterop().DEPOSITOR_ACCOUNT());
        (bool success,) = address(l1Block).call(
            abi.encodePacked(L1BlockInterop.setL1BlockValuesInterop.selector, setValuesEcotoneCalldata)
        );
        assertTrue(success, "function call failed");

        // Assert that the `isDepositTransaction` flag was properly set to true
        vm.prank(Predeploys.CROSS_L2_INBOX);
        assertEq(_l1BlockInterop().isDeposit(), true);

        // Assert `setL1BlockValuesEcotone` was properly called, forwarding the calldata to it
        assertEq(_l1BlockInterop().baseFeeScalar(), baseFeeScalar, "base fee scalar not properly set");
        assertEq(_l1BlockInterop().blobBaseFeeScalar(), blobBaseFeeScalar, "blob base fee scalar not properly set");
        assertEq(_l1BlockInterop().sequenceNumber(), sequenceNumber, "sequence number not properly set");
        assertEq(_l1BlockInterop().timestamp(), timestamp, "timestamp not properly set");
        assertEq(_l1BlockInterop().number(), number, "number not properly set");
        assertEq(_l1BlockInterop().basefee(), baseFee, "base fee not properly set");
        assertEq(_l1BlockInterop().blobBaseFee(), blobBaseFee, "blob base fee not properly set");
        assertEq(_l1BlockInterop().hash(), hash, "hash not properly set");
        assertEq(_l1BlockInterop().batcherHash(), batcherHash, "batcher hash not properly set");
    }
}

contract L1BlockDepositsComplete_Test is L1BlockInteropTest {
    // @dev Tests that `depositsComplete` reverts if the caller is not the depositor.
    function test_depositsComplete_notDepositor_reverts(address _caller) external {
        vm.assume(_caller != _l1BlockInterop().DEPOSITOR_ACCOUNT());
        vm.expectRevert(NotDepositor.selector);
        _l1BlockInterop().depositsComplete();
    }

    // @dev Tests that `depositsComplete` succeeds if the caller is the depositor.
    function test_depositsComplete_succeeds() external {
        // Set the `isDeposit` flag to true
        vm.prank(_l1BlockInterop().DEPOSITOR_ACCOUNT());
        _l1BlockInterop().setL1BlockValuesInterop();

        // Assert that the `isDeposit` flag was properly set to true
        vm.prank(Predeploys.CROSS_L2_INBOX);
        assertTrue(_l1BlockInterop().isDeposit());

        // Call `depositsComplete`
        vm.prank(_l1BlockInterop().DEPOSITOR_ACCOUNT());
        _l1BlockInterop().depositsComplete();

        // Assert that the `isDeposit` flag was properly set to false
        /// @dev Assuming that `isDeposit()` wil return the proper value. That function is tested as well
        vm.prank(Predeploys.CROSS_L2_INBOX);
        assertEq(_l1BlockInterop().isDeposit(), false);
    }
}
