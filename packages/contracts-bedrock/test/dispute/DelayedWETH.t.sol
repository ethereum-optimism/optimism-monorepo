// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

// Testing
import { Test } from "forge-std/Test.sol";
import { CommonTest } from "test/setup/CommonTest.sol";

// Libraries
import { Burn } from "src/libraries/Burn.sol";
import "src/dispute/lib/Types.sol";
import "src/dispute/lib/Errors.sol";

contract DelayedWETH_Init is CommonTest {
    event Approval(address indexed src, address indexed guy, uint256 wad);
    event Transfer(address indexed src, address indexed dst, uint256 wad);
    event Deposit(address indexed dst, uint256 wad);
    event Withdrawal(address indexed src, uint256 wad);
    event Unwrap(address indexed src, uint256 wad);

    function setUp() public virtual override {
        super.setUp();

        // Transfer ownership of delayed WETH to the test contract.
        vm.prank(delayedWeth.owner());
        delayedWeth.transferOwnership(address(this));
    }
}

contract DelayedWETH_Initialize_Test is DelayedWETH_Init {
    /// @dev Tests that initialization is successful.
    function test_initialize_succeeds() public view {
        assertEq(delayedWeth.owner(), address(this));
        assertEq(address(delayedWeth.config()), address(superchainConfig));
    }
}

contract DelayedWETH_Unlock_Test is DelayedWETH_Init {
    /// @dev Tests that unlocking once is successful.
    function test_unlock_once_succeeds() public {
        delayedWeth.unlock(alice, 1 ether);
        (uint256 amount, uint256 timestamp) = delayedWeth.withdrawals(address(this), alice);
        assertEq(amount, 1 ether);
        assertEq(timestamp, block.timestamp);
    }

    /// @dev Tests that unlocking twice is successful and timestamp/amount is updated.
    function test_unlock_twice_succeeds() public {
        // Unlock once.
        uint256 ts = block.timestamp;
        delayedWeth.unlock(alice, 1 ether);
        (uint256 amount1, uint256 timestamp1) = delayedWeth.withdrawals(address(this), alice);
        assertEq(amount1, 1 ether);
        assertEq(timestamp1, ts);

        // Go forward in time.
        vm.warp(ts + 1);

        // Unlock again works.
        delayedWeth.unlock(alice, 1 ether);
        (uint256 amount2, uint256 timestamp2) = delayedWeth.withdrawals(address(this), alice);
        assertEq(amount2, 2 ether);
        assertEq(timestamp2, ts + 1);
    }
}

contract DelayedWETH_Withdraw_Test is DelayedWETH_Init {
    /// @dev Tests that withdrawing while unlocked and delay has passed is successful.
    function test_withdraw_whileUnlocked_succeeds() public {
        // Deposit some WETH.
        vm.prank(alice);
        delayedWeth.deposit{ value: 1 ether }();
        uint256 balance = address(alice).balance;

        // Unlock the withdrawal.
        vm.prank(alice);
        delayedWeth.unlock(alice, 1 ether);

        // Wait for the delay.
        vm.warp(block.timestamp + delayedWeth.delay() + 1);

        // Withdraw the WETH.
        vm.expectEmit(true, true, false, false);
        emit Withdrawal(address(alice), 1 ether);
        vm.prank(alice);
        delayedWeth.withdraw(alice, 1 ether);
        assertEq(address(alice).balance, balance + 1 ether);
    }

    /// @dev Tests that withdrawing when unlock was not called fails.
    function test_withdraw_whileLocked_fails() public {
        // Deposit some WETH.
        vm.prank(alice);
        delayedWeth.deposit{ value: 1 ether }();
        uint256 balance = address(alice).balance;

        // Withdraw fails when unlock not called.
        vm.expectRevert("DelayedWETH: withdrawal not unlocked");
        vm.prank(alice);
        delayedWeth.withdraw(alice, 0 ether);
        assertEq(address(alice).balance, balance);
    }

    /// @dev Tests that withdrawing while locked and delay has not passed fails.
    function test_withdraw_whileLockedNotLongEnough_fails() public {
        // Deposit some WETH.
        vm.prank(alice);
        delayedWeth.deposit{ value: 1 ether }();
        uint256 balance = address(alice).balance;

        // Call unlock.
        vm.prank(alice);
        delayedWeth.unlock(alice, 1 ether);

        // Wait for the delay, but not long enough.
        vm.warp(block.timestamp + delayedWeth.delay() - 1);

        // Withdraw fails when delay not met.
        vm.expectRevert("DelayedWETH: withdrawal delay not met");
        vm.prank(alice);
        delayedWeth.withdraw(alice, 1 ether);
        assertEq(address(alice).balance, balance);
    }

    /// @dev Tests that withdrawing more than unlocked amount fails.
    function test_withdraw_tooMuch_fails() public {
        // Deposit some WETH.
        vm.prank(alice);
        delayedWeth.deposit{ value: 1 ether }();
        uint256 balance = address(alice).balance;

        // Unlock the withdrawal.
        vm.prank(alice);
        delayedWeth.unlock(alice, 1 ether);

        // Wait for the delay.
        vm.warp(block.timestamp + delayedWeth.delay() + 1);

        // Withdraw too much fails.
        vm.expectRevert("DelayedWETH: insufficient unlocked withdrawal");
        vm.prank(alice);
        delayedWeth.withdraw(alice, 2 ether);
        assertEq(address(alice).balance, balance);
    }

    /// @dev Tests that withdrawing while paused fails.
    function test_withdraw_whenPaused_fails() public {
        // Deposit some WETH.
        vm.prank(alice);
        delayedWeth.deposit{ value: 1 ether }();

        // Unlock the withdrawal.
        vm.prank(alice);
        delayedWeth.unlock(alice, 1 ether);

        // Wait for the delay.
        vm.warp(block.timestamp + delayedWeth.delay() + 1);

        // Pause the contract.
        address guardian = optimismPortal.guardian();
        vm.prank(guardian);
        superchainConfig.pause("identifier");

        // Withdraw fails.
        vm.expectRevert("DelayedWETH: contract is paused");
        vm.prank(alice);
        delayedWeth.withdraw(alice, 1 ether);
    }
}

contract DelayedWETH_Recover_Test is DelayedWETH_Init {
    /// @dev Tests that recovering WETH succeeds. Makes sure that doing so succeeds with any amount
    ///      of ETH in the contract and any amount of gas used in the fallback function up to a
    ///      maximum of 20,000,000 gas. Owner contract should never be using that much gas but we
    ///      might as well set a very large upper bound for ourselves.
    /// @param _amount Amount of WETH to recover.
    /// @param _fallbackGasUsage Amount of gas to use in the fallback function.
    function testFuzz_recover_succeeds(uint256 _amount, uint256 _fallbackGasUsage) public {
        // Assume
        _fallbackGasUsage = bound(_fallbackGasUsage, 0, 20000000);

        // Set up the gas burner.
        FallbackGasUser gasUser = new FallbackGasUser(_fallbackGasUsage);

        // Transfer ownership to alice.
        delayedWeth.transferOwnership(address(gasUser));

        // Give the contract some WETH to recover.
        vm.deal(address(delayedWeth), _amount);

        // Record the initial balance.
        uint256 initialBalance = address(gasUser).balance;

        // Recover the WETH.
        vm.prank(address(gasUser));
        delayedWeth.recover(_amount);

        // Verify the WETH was recovered.
        assertEq(address(delayedWeth).balance, 0);
        assertEq(address(gasUser).balance, initialBalance + _amount);
    }

    /// @dev Tests that recovering WETH by non-owner fails.
    function test_recover_byNonOwner_fails() public {
        // Pretend to be a non-owner.
        vm.prank(alice);

        // Recover fails.
        vm.expectRevert("DelayedWETH: not owner");
        delayedWeth.recover(1 ether);
    }

    /// @dev Tests that recovering more than the balance recovers what it can.
    function test_recover_moreThanBalance_succeeds() public {
        // Transfer ownership to alice.
        delayedWeth.transferOwnership(alice);

        // Give the contract some WETH to recover.
        vm.deal(address(delayedWeth), 0.5 ether);

        // Record the initial balance.
        uint256 initialBalance = address(alice).balance;

        // Recover the WETH.
        vm.prank(alice);
        delayedWeth.recover(1 ether);

        // Verify the WETH was recovered.
        assertEq(address(delayedWeth).balance, 0);
        assertEq(address(alice).balance, initialBalance + 0.5 ether);
    }

    /// @dev Tests that recover reverts when recipient reverts.
    function test_recover_whenRecipientReverts_fails() public {
        // Set up the reverter.
        FallbackReverter reverter = new FallbackReverter();

        // Transfer ownership to the reverter.
        delayedWeth.transferOwnership(address(reverter));

        // Give the contract some WETH to recover.
        vm.deal(address(delayedWeth), 1 ether);

        // Recover fails.
        vm.expectRevert("DelayedWETH: recover failed");
        vm.prank(address(reverter));
        delayedWeth.recover(1 ether);
    }
}

contract DelayedWETH_Hold_Test is DelayedWETH_Init {
    /// @dev Tests that holding WETH succeeds.
    function test_hold_succeeds() public {
        uint256 amount = 1 ether;

        // Pretend to be alice and deposit some WETH.
        vm.prank(alice);
        delayedWeth.deposit{ value: amount }();

        // Hold some WETH.
        vm.expectEmit(true, true, true, false);
        emit Approval(alice, address(this), amount);
        delayedWeth.hold(alice, amount);

        // Verify the allowance.
        assertEq(delayedWeth.allowance(alice, address(this)), amount);

        // We can transfer.
        delayedWeth.transferFrom(alice, address(this), amount);

        // Verify the transfer.
        assertEq(delayedWeth.balanceOf(address(this)), amount);
    }

    /// @dev Tests that holding WETH by non-owner fails.
    function test_hold_byNonOwner_fails() public {
        // Pretend to be a non-owner.
        vm.prank(alice);

        // Hold fails.
        vm.expectRevert("DelayedWETH: not owner");
        delayedWeth.hold(bob, 1 ether);
    }
}

/// @title FallbackGasUser
/// @notice Contract that burns gas in the fallback function.
contract FallbackGasUser {
    /// @notice Amount of gas to use in the fallback function.
    uint256 public gas;

    /// @param _gas Amount of gas to use in the fallback function.
    constructor(uint256 _gas) {
        gas = _gas;
    }

    /// @notice Burn gas on fallback;
    fallback() external payable {
        Burn.gas(gas);
    }

    /// @notice Burn gas on receive.
    receive() external payable {
        Burn.gas(gas);
    }
}

/// @title FallbackReverter
/// @notice Contract that reverts in the fallback function.
contract FallbackReverter {
    /// @notice Revert on fallback.
    fallback() external payable {
        revert("FallbackReverter: revert");
    }

    /// @notice Revert on receive.
    receive() external payable {
        revert("FallbackReverter: revert");
    }
}
