// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing utilities
import { Test } from "forge-std/Test.sol";
import { CallRecorder, Reverter } from "test/mocks/Callers.sol";
import { Transactor } from "src/periphery/Transactor.sol";

contract Transactor_Initializer is Test {
    address alice = address(128);
    address bob = address(256);

    Transactor transactor;
    Reverter reverter;
    CallRecorder callRecorded;

    function setUp() public {
        // Deploy Reverter and CallRecorder helper contracts
        reverter = new Reverter();
        callRecorded = new CallRecorder();

        // Deploy Transactor contract
        transactor = new Transactor(address(alice));
        vm.label(address(transactor), "Transactor");

        // Give alice and bob some ETH
        vm.deal(alice, 1 ether);
        vm.deal(bob, 1 ether);

        vm.label(alice, "alice");
        vm.label(bob, "bob");
    }
}

contract TransactorTest is Transactor_Initializer {
    /// @notice Tests if the owner was set correctly during deploy
    function test_constructor_succeeds() external view {
        assertEq(address(alice), transactor.owner());
    }

    /// @notice Tests CALL, should do a call to target
    function test_call_succeeds() external {
        // Initialize call data
        bytes memory data = abi.encodeCall(CallRecorder.record, ());
        // Run CALL
        vm.prank(alice);
        vm.expectCall(address(callRecorded), 200_000 wei, data);
        transactor.CALL(address(callRecorded), data, 200_000 wei);
    }

    /// @notice It should revert if called by non-owner
    function test_call_unauthorized_reverts() external {
        // Initialize call data
        bytes memory data = abi.encodeCall(CallRecorder.record, ());
        // Run CALL
        vm.prank(bob);
        vm.expectRevert("UNAUTHORIZED");
        transactor.CALL(address(callRecorded), data, 200_000 wei);
    }

    /// @notice Deletate call succeeds.
    function test_delegateCall_succeeds() external {
        // Initialize call data
        bytes memory data = abi.encodeCall(Reverter.doRevert, ());
        // Run CALL
        vm.prank(alice);
        vm.expectCall(address(reverter), data);
        transactor.DELEGATECALL(address(reverter), data);
    }

    /// @notice It should revert if called by non-owner
    function test_delegateCall_unauthorized_reverts() external {
        // Initialize call data
        bytes memory data = abi.encodeCall(Reverter.doRevert, ());
        // Run CALL
        vm.prank(bob);
        vm.expectRevert("UNAUTHORIZED");
        transactor.DELEGATECALL(address(reverter), data);
    }
}
