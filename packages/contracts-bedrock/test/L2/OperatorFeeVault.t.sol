// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing utilities
import { CommonTest } from "test/setup/CommonTest.sol";

// Libraries
import { Types } from "src/libraries/Types.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";

// Test the implementations of the FeeVault
contract FeeVault_Test is CommonTest {
    /// @dev Tests that the constructor sets the correct values.
    function test_constructor_operatorFeeVault_succeeds() external view {
        assertEq(operatorFeeVault.RECIPIENT(), Predeploys.BASE_FEE_VAULT);
        assertEq(operatorFeeVault.recipient(), Predeploys.BASE_FEE_VAULT);
        assertEq(operatorFeeVault.MIN_WITHDRAWAL_AMOUNT(), 0);
        assertEq(operatorFeeVault.minWithdrawalAmount(), 0);
        assertEq(uint8(operatorFeeVault.WITHDRAWAL_NETWORK()), uint8(Types.WithdrawalNetwork.L2));
        assertEq(uint8(operatorFeeVault.withdrawalNetwork()), uint8(Types.WithdrawalNetwork.L2));
    }
}
