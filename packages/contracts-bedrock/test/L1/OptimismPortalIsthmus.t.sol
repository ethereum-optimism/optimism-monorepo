// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { CommonTest } from "test/setup/CommonTest.sol";

// Interfaces
import { IOptimismPortalIsthmus } from "interfaces/L1/IOptimismPortalIsthmus.sol";

contract OptimismPortalIsthmus_Test is CommonTest {
    /// @notice Marked virtual to be overridden in
    ///         test/kontrol/deployment/DeploymentSummary.t.sol
    function setUp() public virtual override {
        super.enableIsthmus();
        super.setUp();
        optimismPortal2.version();
    }

    /// @dev Tests that `receive` successfully deposits ETH.
    function testFuzz_receive_succeeds(uint256 _value) external {
        uint256 balanceBefore = address(optimismPortal2).balance;
        _value = bound(_value, 0, type(uint256).max - balanceBefore);

        vm.expectEmit(address(optimismPortal2));
        emitTransactionDepositedIsthmus(alice, alice, _value, _value, 100_000, false, hex"", 1);

        // give alice money and send as an eoa
        vm.deal(alice, _value);
        vm.prank(alice, alice);
        (bool s,) = address(optimismPortal2).call{ value: _value }(hex"");

        assertTrue(s);
        assertEq(address(optimismPortal2).balance, balanceBefore + _value);
    }

    function test_nonce_increment_works() external {
        vm.deal(alice, 100_000_000);

        uint256 value = 2;
        for (uint64 i = 1; i <= 10; i++) {
            vm.expectEmit(address(optimismPortal2));
            emitTransactionDepositedIsthmus(alice, alice, value, value, 100_000, false, hex"", i);
            vm.prank(alice, alice);
            (bool s,) = address(optimismPortal2).call{ value: value }(hex"");
            assertTrue(s);
        }
    }

    function test_depositNonce_works() external {
        vm.deal(alice, 100_000_000);

        uint64 nonce = IOptimismPortalIsthmus(payable(address(optimismPortal2))).depositNonce();
        assertEq(0, nonce);

        uint256 value = 2;
        for (uint64 i = 1; i <= 2; i++) {
            vm.prank(alice, alice);
            (bool s,) = address(optimismPortal2).call{ value: value }(hex"");
            assertTrue(s);
        }

        nonce = IOptimismPortalIsthmus(payable(address(optimismPortal2))).depositNonce();
        assertEq(2, nonce);
    }
}
