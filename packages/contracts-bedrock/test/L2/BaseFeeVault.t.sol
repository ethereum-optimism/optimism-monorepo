// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing utilities
import { CommonTest } from "test/setup/CommonTest.sol";

// Libraries
import { Types } from "src/libraries/Types.sol";

// Test the implementations of the FeeVault
contract FeeVault_Test is CommonTest {
    /// @dev Tests that the constructor sets the correct values.
    function test_constructor_baseFeeVault_succeeds() external view {
        assertEq(baseFeeVault.RECIPIENT(), deploy.cfg().baseFeeVaultRecipient());
        assertEq(baseFeeVault.recipient(), deploy.cfg().baseFeeVaultRecipient());
        assertEq(baseFeeVault.MIN_WITHDRAWAL_AMOUNT(), deploy.cfg().baseFeeVaultMinimumWithdrawalAmount());
        assertEq(baseFeeVault.minWithdrawalAmount(), deploy.cfg().baseFeeVaultMinimumWithdrawalAmount());
        assertEq(uint8(baseFeeVault.WITHDRAWAL_NETWORK()), uint8(Types.WithdrawalNetwork.L1));
        assertEq(uint8(baseFeeVault.withdrawalNetwork()), uint8(Types.WithdrawalNetwork.L1));
    }
}
