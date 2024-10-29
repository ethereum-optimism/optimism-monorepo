// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { Test } from "forge-std/Test.sol";

// Contracts
import { IWETH98 } from "src/universal/interfaces/IWETH98.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

contract WETH98_Test is Test {
    event Approval(address indexed src, address indexed guy, uint256 wad);
    event Transfer(address indexed src, address indexed dst, uint256 wad);
    event Deposit(address indexed dst, uint256 wad);
    event Withdrawal(address indexed src, uint256 wad);

    IWETH98 public weth;
    address alice;
    address bob;

    function setUp() public {
        weth = IWETH98(
            DeployUtils.create1({
                _name: "WETH98",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IWETH98.__constructor__, ()))
            })
        );
        alice = makeAddr("alice");
        bob = makeAddr("bob");
        deal(alice, 1 ether);
    }

    function test_getName_succeeds() public view {
        assertEq(weth.name(), "Wrapped Ether");
        assertEq(weth.symbol(), "WETH");
        assertEq(weth.decimals(), 18);
    }

    function test_receive_succeeds() public {
        vm.expectEmit(address(weth));
        emit Deposit(alice, 1 ether);
        vm.prank(alice);
        (bool success,) = address(weth).call{ value: 1 ether }("");
        assertTrue(success);
        assertEq(weth.balanceOf(alice), 1 ether);
    }

    function test_fallback_succeeds() public {
        vm.expectEmit(address(weth));
        emit Deposit(alice, 1 ether);
        vm.prank(alice);
        (bool success,) = address(weth).call{ value: 1 ether }(hex"1234");
        assertTrue(success);
        assertEq(weth.balanceOf(alice), 1 ether);
    }

    function test_deposit_succeeds() public {
        vm.expectEmit(address(weth));
        emit Deposit(alice, 1 ether);
        vm.prank(alice);
        weth.deposit{ value: 1 ether }();
        assertEq(weth.balanceOf(alice), 1 ether);
    }

    function test_withdraw_succeeds() public {
        vm.prank(alice);
        weth.deposit{ value: 1 ether }();
        vm.expectEmit(address(weth));
        emit Withdrawal(alice, 1 ether);
        vm.prank(alice);
        weth.withdraw(1 ether);
        assertEq(weth.balanceOf(alice), 0);
    }

    function test_withdraw_partialWithdrawal_succeeds() public {
        vm.prank(alice);
        weth.deposit{ value: 1 ether }();
        vm.expectEmit(address(weth));
        emit Withdrawal(alice, 1 ether / 2);
        vm.prank(alice);
        weth.withdraw(1 ether / 2);
        assertEq(weth.balanceOf(alice), 1 ether / 2);
    }

    function test_withdraw_tooLargeWithdrawal_fails() public {
        vm.prank(alice);
        weth.deposit{ value: 1 ether }();
        vm.expectRevert();
        vm.prank(alice);
        weth.withdraw(1 ether + 1);
    }

    function test_transfer_succeeds() public {
        vm.prank(alice);
        weth.deposit{ value: 1 ether }();
        vm.expectEmit(address(weth));
        emit Transfer(alice, bob, 1 ether);
        vm.prank(alice);
        weth.transfer(bob, 1 ether);
        assertEq(weth.balanceOf(alice), 0);
        assertEq(weth.balanceOf(bob), 1 ether);
    }

    function test_transfer_tooLarge_fails() public {
        vm.prank(alice);
        weth.deposit{ value: 1 ether }();
        vm.expectRevert();
        vm.prank(alice);
        weth.transfer(bob, 1 ether + 1);
    }

    function test_approve_succeeds() public {
        vm.prank(alice);
        vm.expectEmit(address(weth));
        emit Approval(alice, bob, 1 ether);
        weth.approve(bob, 1 ether);
        assertEq(weth.allowance(alice, bob), 1 ether);
    }

    function test_transferFrom_succeeds() public {
        vm.prank(alice);
        weth.deposit{ value: 1 ether }();
        vm.prank(alice);
        weth.approve(bob, 1 ether);
        vm.expectEmit(address(weth));
        emit Transfer(alice, bob, 1 ether);
        vm.prank(bob);
        weth.transferFrom(alice, bob, 1 ether);
        assertEq(weth.balanceOf(alice), 0);
        assertEq(weth.balanceOf(bob), 1 ether);
    }

    function test_transferFrom_tooLittleApproval_fails() public {
        vm.prank(alice);
        weth.deposit{ value: 1 ether }();
        vm.prank(alice);
        weth.approve(bob, 1 ether);
        vm.expectRevert();
        vm.prank(bob);
        weth.transferFrom(alice, bob, 1 ether + 1);
    }

    function test_transferFrom_tooLittleBalance_fails() public {
        vm.prank(alice);
        weth.deposit{ value: 1 ether }();
        vm.prank(alice);
        weth.approve(bob, 2 ether);
        vm.expectRevert();
        vm.prank(bob);
        weth.transferFrom(alice, bob, 1 ether + 1);
    }
}
