// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { CommonTest } from "test/setup/CommonTest.sol";

// Libraries
import { Predeploys } from "src/libraries/Predeploys.sol";
import "src/libraries/L1BlockErrors.sol";

// Interfaces
import { IL1BlockInterop } from "interfaces/L2/IL1BlockInterop.sol";

contract L1BlockInteropTest is CommonTest {
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

    /// @dev Returns the L1BlockInterop instance.
    function _l1BlockInterop() internal view returns (IL1BlockInterop) {
        return IL1BlockInterop(address(l1Block));
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
            abi.encodePacked(IL1BlockInterop.setL1BlockValuesInterop.selector, setValuesEcotoneCalldata)
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
