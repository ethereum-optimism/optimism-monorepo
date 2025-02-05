// SPDX-License-Identifier: MIT
pragma solidity 0.8.25;

// Testing utilities
import { Test } from "forge-std/Test.sol";

// Libraries
import { Predeploys } from "src/libraries/Predeploys.sol";
import { TransientContext } from "src/libraries/TransientContext.sol";

// Target contracts
import {
    CrossL2Inbox,
    Identifier,
    NotEntered,
    NoExecutingDeposits,
    NotDepositor,
    InteropStartAlreadySet
} from "src/L2/CrossL2Inbox.sol";
import { IL1BlockInterop } from "interfaces/L2/IL1BlockInterop.sol";

/// @title CrossL2InboxWithModifiableTransientStorage
/// @dev CrossL2Inbox contract with methods to modify the transient storage.
///      This is used to test the transient storage of CrossL2Inbox.
contract CrossL2InboxWithModifiableTransientStorage is CrossL2Inbox {
    /// @dev Increments call depth in transient storage.
    function increment() external {
        TransientContext.increment();
    }

    /// @dev Sets origin in transient storage.
    /// @param _origin Origin to set.
    function setOrigin(address _origin) external {
        TransientContext.set(ORIGIN_SLOT, uint160(_origin));
    }

    /// @dev Sets block number in transient storage.
    /// @param _blockNumber Block number to set.
    function setBlockNumber(uint256 _blockNumber) external {
        TransientContext.set(BLOCK_NUMBER_SLOT, _blockNumber);
    }

    /// @dev Sets log index in transient storage.
    /// @param _logIndex Log index to set.
    function setLogIndex(uint256 _logIndex) external {
        TransientContext.set(LOG_INDEX_SLOT, _logIndex);
    }

    /// @dev Sets timestamp in transient storage.
    /// @param _timestamp Timestamp to set.
    function setTimestamp(uint256 _timestamp) external {
        TransientContext.set(TIMESTAMP_SLOT, _timestamp);
    }

    /// @dev Sets chain ID in transient storage.
    /// @param _chainId Chain ID to set.
    function setChainId(uint256 _chainId) external {
        TransientContext.set(CHAINID_SLOT, _chainId);
    }
}

/// @title CrossL2InboxTest
/// @dev Contract for testing the CrossL2Inbox contract.
contract CrossL2InboxTest is Test {
    /// @dev Selector for the `isInDependencySet` method of the L1Block contract.
    bytes4 constant L1BlockIsInDependencySetSelector = bytes4(keccak256("isInDependencySet(uint256)"));

    /// @dev Storage slot that the interop start timestamp is stored at.
    ///      Equal to bytes32(uint256(keccak256("crossl2inbox.interopstart")) - 1)
    bytes32 internal constant INTEROP_START_SLOT = bytes32(uint256(keccak256("crossl2inbox.interopstart")) - 1);

    /// @dev CrossL2Inbox contract instance.
    CrossL2Inbox crossL2Inbox;

    // interop start timestamp
    uint256 interopStartTime = 420;

    /// @dev The address that represents the system caller responsible for L1 attributes
    ///         transactions.
    address internal constant DEPOSITOR_ACCOUNT = 0xDeaDDEaDDeAdDeAdDEAdDEaddeAddEAdDEAd0001;

    /// @dev Sets up the test suite.
    function setUp() public {
        // Deploy the L2ToL2CrossDomainMessenger contract
        vm.etch(Predeploys.CROSS_L2_INBOX, address(new CrossL2InboxWithModifiableTransientStorage()).code);
        crossL2Inbox = CrossL2Inbox(Predeploys.CROSS_L2_INBOX);
    }

    modifier setInteropStart() {
        // Set interop start
        vm.store(address(crossL2Inbox), INTEROP_START_SLOT, bytes32(interopStartTime));

        // Set timestamp to be after interop start
        vm.warp(interopStartTime + 1 hours);

        _;
    }

    /// @dev Tests that the setInteropStart function updates the INTEROP_START_SLOT storage slot correctly
    function testFuzz_setInteropStart_succeeds(uint256 time) external {
        // Jump to time.
        vm.warp(time);

        // Impersonate the depositor account.
        vm.prank(DEPOSITOR_ACCOUNT);

        // Set interop start.
        crossL2Inbox.setInteropStart();

        // Check that the storage slot was set correctly and the public getter function returns the right value.
        assertEq(crossL2Inbox.interopStart(), time);
        assertEq(uint256(vm.load(address(crossL2Inbox), INTEROP_START_SLOT)), time);
    }

    /// @dev Tests that the setInteropStart function reverts when the caller is not the DEPOSITOR_ACCOUNT.
    function test_setInteropStart_notDepositorAccount_reverts() external {
        // Expect revert with OnlyDepositorAccount selector
        vm.expectRevert(NotDepositor.selector);

        // Call setInteropStart function
        crossL2Inbox.setInteropStart();
    }

    /// @dev Tests that the setInteropStart function reverts if called when already set
    function test_setInteropStart_interopStartAlreadySet_reverts() external {
        // Impersonate the depositor account.
        vm.startPrank(DEPOSITOR_ACCOUNT);

        // Call setInteropStart function
        crossL2Inbox.setInteropStart();

        // Expect revert with InteropStartAlreadySet selector if called a second time
        vm.expectRevert(InteropStartAlreadySet.selector);

        // Call setInteropStart function again
        crossL2Inbox.setInteropStart();
    }

    function testFuzz_validateMessage_succeeds(Identifier memory _id, bytes32 _messageHash) external setInteropStart {
        // Ensure that the id's timestamp is valid (less than or equal to the current block timestamp and greater than
        // interop start time)
        _id.timestamp = bound(_id.timestamp, interopStartTime + 1, block.timestamp);

        // Ensure that the chain ID is in the dependency set
        vm.mockCall({
            callee: Predeploys.L1_BLOCK_ATTRIBUTES,
            data: abi.encodeCall(IL1BlockInterop.isInDependencySet, (_id.chainId)),
            returnData: abi.encode(true)
        });

        // Ensure is not a deposit transaction
        vm.mockCall({
            callee: Predeploys.L1_BLOCK_ATTRIBUTES,
            data: abi.encodeCall(IL1BlockInterop.isDeposit, ()),
            returnData: abi.encode(false)
        });

        // Look for the emit ExecutingMessage event
        vm.expectEmit(Predeploys.CROSS_L2_INBOX);
        emit CrossL2Inbox.ExecutingMessage(_messageHash, _id);

        // Call the validateMessage function
        crossL2Inbox.validateMessage(_id, _messageHash);
    }

    function testFuzz_validateMessage_isDeposit_reverts(Identifier calldata _id, bytes32 _messageHash) external {
        // Ensure it is a deposit transaction
        vm.mockCall({
            callee: Predeploys.L1_BLOCK_ATTRIBUTES,
            data: abi.encodeCall(IL1BlockInterop.isDeposit, ()),
            returnData: abi.encode(true)
        });

        // Expect a revert with the NoExecutingDeposits selector
        vm.expectRevert(NoExecutingDeposits.selector);

        // Call the validateMessage function
        crossL2Inbox.validateMessage(_id, _messageHash);
    }

    /// @dev Tests that the `origin` function returns the correct value.
    function testFuzz_origin_succeeds(address _origin) external {
        // Increment the call depth to prevent NotEntered revert
        CrossL2InboxWithModifiableTransientStorage(Predeploys.CROSS_L2_INBOX).increment();
        // Set origin in the transient storage
        CrossL2InboxWithModifiableTransientStorage(Predeploys.CROSS_L2_INBOX).setOrigin(_origin);
        // Check that the `origin` function returns the correct value
        assertEq(crossL2Inbox.origin(), _origin);
    }

    /// @dev Tests that the `origin` function reverts when not entered.
    function test_origin_notEntered_reverts() external {
        // Expect a revert with the NotEntered selector
        vm.expectRevert(NotEntered.selector);
        // Call the `origin` function
        crossL2Inbox.origin();
    }

    /// @dev Tests that the `blockNumber` function returns the correct value.
    function testFuzz_blockNumber_succeeds(uint256 _blockNumber) external {
        // Increment the call depth to prevent NotEntered revert
        CrossL2InboxWithModifiableTransientStorage(Predeploys.CROSS_L2_INBOX).increment();
        // Set blockNumber in the transient storage
        CrossL2InboxWithModifiableTransientStorage(Predeploys.CROSS_L2_INBOX).setBlockNumber(_blockNumber);
        // Check that the `blockNumber` function returns the correct value
        assertEq(crossL2Inbox.blockNumber(), _blockNumber);
    }

    /// @dev Tests that the `blockNumber` function reverts when not entered.
    function test_blockNumber_notEntered_reverts() external {
        // Expect a revert with the NotEntered selector
        vm.expectRevert(NotEntered.selector);
        // Call the `blockNumber` function
        crossL2Inbox.blockNumber();
    }

    /// @dev Tests that the `logIndex` function returns the correct value.
    function testFuzz_logIndex_succeeds(uint256 _logIndex) external {
        // Increment the call depth to prevent NotEntered revert
        CrossL2InboxWithModifiableTransientStorage(Predeploys.CROSS_L2_INBOX).increment();
        // Set logIndex in the transient storage
        CrossL2InboxWithModifiableTransientStorage(Predeploys.CROSS_L2_INBOX).setLogIndex(_logIndex);
        // Check that the `logIndex` function returns the correct value
        assertEq(crossL2Inbox.logIndex(), _logIndex);
    }

    /// @dev Tests that the `logIndex` function reverts when not entered.
    function test_logIndex_notEntered_reverts() external {
        // Expect a revert with the NotEntered selector
        vm.expectRevert(NotEntered.selector);
        // Call the `logIndex` function
        crossL2Inbox.logIndex();
    }

    /// @dev Tests that the `timestamp` function returns the correct value.
    function testFuzz_timestamp_succeeds(uint256 _timestamp) external {
        // Increment the call depth to prevent NotEntered revert
        CrossL2InboxWithModifiableTransientStorage(Predeploys.CROSS_L2_INBOX).increment();
        // Set timestamp in the transient storage
        CrossL2InboxWithModifiableTransientStorage(Predeploys.CROSS_L2_INBOX).setTimestamp(_timestamp);
        // Check that the `timestamp` function returns the correct value
        assertEq(crossL2Inbox.timestamp(), _timestamp);
    }

    /// @dev Tests that the `timestamp` function reverts when not entered.
    function test_timestamp_notEntered_reverts() external {
        // Expect a revert with the NotEntered selector
        vm.expectRevert(NotEntered.selector);
        // Call the `timestamp` function
        crossL2Inbox.timestamp();
    }

    /// @dev Tests that the `chainId` function returns the correct value.
    function testFuzz_chainId_succeeds(uint256 _chainId) external {
        // Increment the call depth to prevent NotEntered revert
        CrossL2InboxWithModifiableTransientStorage(Predeploys.CROSS_L2_INBOX).increment();
        // Set chainId in the transient storage
        CrossL2InboxWithModifiableTransientStorage(Predeploys.CROSS_L2_INBOX).setChainId(_chainId);
        // Check that the `chainId` function returns the correct value
        assertEq(crossL2Inbox.chainId(), _chainId);
    }

    /// @dev Tests that the `chainId` function reverts when not entered.
    function test_chainId_notEntered_reverts() external {
        // Expect a revert with the NotEntered selector
        vm.expectRevert(NotEntered.selector);
        // Call the `chainId` function
        crossL2Inbox.chainId();
    }
}
