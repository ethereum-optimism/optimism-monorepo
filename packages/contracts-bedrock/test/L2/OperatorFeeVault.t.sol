// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing utilities
import { CommonTest } from "test/setup/CommonTest.sol";

// Libraries
import { Types } from "src/libraries/Types.sol";

// Test the implementations of the FeeVault
contract FeeVault_Test is CommonTest {
    /// @dev Tests that the constructor sets the correct values.
    function test_constructor_operatorFeeVault_succeeds() external view {
        assertEq(operatorFeeVault.RECIPIENT(), deploy.cfg().operatorFeeVaultRecipient());
        assertEq(operatorFeeVault.recipient(), deploy.cfg().operatorFeeVaultRecipient());
        assertEq(operatorFeeVault.MIN_WITHDRAWAL_AMOUNT(), deploy.cfg().operatorFeeVaultMinimumWithdrawalAmount());
        assertEq(operatorFeeVault.minWithdrawalAmount(), deploy.cfg().operatorFeeVaultMinimumWithdrawalAmount());
        assertEq(uint8(operatorFeeVault.WITHDRAWAL_NETWORK()), uint8(Types.WithdrawalNetwork.L1));
        assertEq(uint8(operatorFeeVault.withdrawalNetwork()), uint8(Types.WithdrawalNetwork.L1));
    }
}
