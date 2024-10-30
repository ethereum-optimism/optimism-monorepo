// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { CommonTest } from "test/setup/CommonTest.sol";
import { Reverter } from "test/mocks/Callers.sol";

// Libraries
import { Hashing } from "src/libraries/Hashing.sol";
import { Types } from "src/libraries/Types.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";
import { Encoding } from "src/libraries/Encoding.sol";
import { Constants } from "src/libraries/Constants.sol";

contract SequencerFeeVault_Test is CommonTest {
    function setUp() public override {
        super.setUp();
        assertEq(uint8(deploy.cfg().sequencerFeeVaultWithdrawalNetwork()), uint8(Types.WithdrawalNetwork.L1));
    }

    /// @dev Tests that the sequencer fee wallet is correct.
    function test_constructor_succeeds() external view {
        address recipient = deploy.cfg().sequencerFeeVaultRecipient();
        uint256 amount = deploy.cfg().sequencerFeeVaultMinimumWithdrawalAmount();
        assertEq(sequencerFeeVault.l1FeeWallet(), recipient);
        assertEq(sequencerFeeVault.RECIPIENT(), recipient);
        assertEq(sequencerFeeVault.recipient(), recipient);
        assertEq(sequencerFeeVault.MIN_WITHDRAWAL_AMOUNT(), amount);
        assertEq(sequencerFeeVault.minWithdrawalAmount(), amount);
        assertEq(uint8(sequencerFeeVault.WITHDRAWAL_NETWORK()), uint8(Types.WithdrawalNetwork.L1));
        assertEq(uint8(sequencerFeeVault.withdrawalNetwork()), uint8(Types.WithdrawalNetwork.L1));
    }

    /// @dev Tests that the fee vault is able to receive ETH.
    function test_receive_succeeds() external {
        uint256 balance = address(sequencerFeeVault).balance;

        vm.prank(alice);
        (bool success,) = address(sequencerFeeVault).call{ value: 100 }(hex"");

        assertEq(success, true);
        assertEq(address(sequencerFeeVault).balance, balance + 100);
    }

    /// @dev Tests that `withdraw` reverts if the balance is less than the minimum
    ///      withdrawal amount.
    function test_withdraw_notEnough_reverts() external {
        assert(address(sequencerFeeVault).balance < sequencerFeeVault.MIN_WITHDRAWAL_AMOUNT());

        vm.expectRevert("FeeVault: withdrawal amount must be greater than minimum withdrawal amount");
        sequencerFeeVault.withdraw();
    }

    /// @dev Tests that `withdraw` successfully initiates a withdrawal to L1.
    function test_withdraw_toL1_succeeds() external {
        uint256 amount = sequencerFeeVault.MIN_WITHDRAWAL_AMOUNT() + 1;
        vm.deal(address(sequencerFeeVault), amount);

        // No ether has been withdrawn yet
        assertEq(sequencerFeeVault.totalProcessed(), 0);

        address recipient = deploy.cfg().sequencerFeeVaultRecipient();

        vm.expectEmit(address(Predeploys.SEQUENCER_FEE_WALLET));
        emit Withdrawal(address(sequencerFeeVault).balance, recipient, address(this));
        vm.expectEmit(address(Predeploys.SEQUENCER_FEE_WALLET));
        emit Withdrawal(address(sequencerFeeVault).balance, recipient, address(this), Types.WithdrawalNetwork.L1);

        // The entire vault's balance is withdrawn
        vm.expectCall(Predeploys.L2_TO_L1_MESSAGE_PASSER, address(sequencerFeeVault).balance, hex"");

        // The message is passed to the correct recipient
        vm.expectEmit(Predeploys.L2_TO_L1_MESSAGE_PASSER);
        emit MessagePassed(
            l2ToL1MessagePasser.messageNonce(),
            address(sequencerFeeVault),
            recipient,
            amount,
            400_000,
            hex"",
            Hashing.hashWithdrawal(
                Types.WithdrawalTransaction({
                    nonce: l2ToL1MessagePasser.messageNonce(),
                    sender: address(sequencerFeeVault),
                    target: recipient,
                    value: amount,
                    gasLimit: 400_000,
                    data: hex""
                })
            )
        );

        sequencerFeeVault.withdraw();

        // The withdrawal was successful
        assertEq(sequencerFeeVault.totalProcessed(), amount);
        assertEq(address(sequencerFeeVault).balance, 0);
        assertEq(Predeploys.L2_TO_L1_MESSAGE_PASSER.balance, amount);
    }
}

contract SequencerFeeVault_L2Withdrawal_Test is CommonTest {
    /// @dev Sets up the test suite.
    function setUp() public override {
        super.setUp();

        // Explicitly use L2 withdrawal network
        bytes32 sequencerFeeVaultConfig = Encoding.encodeFeeVaultConfig({
            _recipient: deploy.cfg().sequencerFeeVaultRecipient(),
            _amount: deploy.cfg().sequencerFeeVaultMinimumWithdrawalAmount(),
            _network: Types.WithdrawalNetwork.L2
        });
        vm.prank(Constants.DEPOSITOR_ACCOUNT);
        l1Block.setConfig(Types.ConfigType.SEQUENCER_FEE_VAULT_CONFIG, abi.encode(sequencerFeeVaultConfig));
    }

    /// @dev Tests that the sequencer fee wallet is correct.
    function test_constructor_succeeds() external view {
        address recipient = deploy.cfg().sequencerFeeVaultRecipient();
        uint256 amount = deploy.cfg().sequencerFeeVaultMinimumWithdrawalAmount();
        assertEq(sequencerFeeVault.l1FeeWallet(), recipient);
        assertEq(sequencerFeeVault.RECIPIENT(), recipient);
        assertEq(sequencerFeeVault.recipient(), recipient);
        assertEq(sequencerFeeVault.MIN_WITHDRAWAL_AMOUNT(), amount);
        assertEq(sequencerFeeVault.minWithdrawalAmount(), amount);
        assertEq(uint8(sequencerFeeVault.WITHDRAWAL_NETWORK()), uint8(Types.WithdrawalNetwork.L2));
        assertEq(uint8(sequencerFeeVault.withdrawalNetwork()), uint8(Types.WithdrawalNetwork.L2));
    }

    /// @dev Tests that `withdraw` successfully initiates a withdrawal to L2.
    function test_withdraw_toL2_succeeds() external {
        uint256 amount = sequencerFeeVault.MIN_WITHDRAWAL_AMOUNT() + 1;
        vm.deal(address(sequencerFeeVault), amount);

        // No ether has been withdrawn yet
        assertEq(sequencerFeeVault.totalProcessed(), 0);

        vm.expectEmit(address(Predeploys.SEQUENCER_FEE_WALLET));
        emit Withdrawal(address(sequencerFeeVault).balance, sequencerFeeVault.RECIPIENT(), address(this));
        vm.expectEmit(address(Predeploys.SEQUENCER_FEE_WALLET));
        emit Withdrawal(
            address(sequencerFeeVault).balance, sequencerFeeVault.RECIPIENT(), address(this), Types.WithdrawalNetwork.L2
        );

        // The entire vault's balance is withdrawn
        vm.expectCall(sequencerFeeVault.RECIPIENT(), address(sequencerFeeVault).balance, bytes(""));

        sequencerFeeVault.withdraw();

        // The withdrawal was successful
        assertEq(sequencerFeeVault.totalProcessed(), amount);
        assertEq(address(sequencerFeeVault).balance, 0);
        assertEq(sequencerFeeVault.recipient().balance, amount);
    }

    /// @dev Tests that `withdraw` fails if the Recipient reverts. This also serves to simulate
    ///     a situation where insufficient gas is provided to the RECIPIENT.
    function test_withdraw_toL2recipientReverts_fails() external {
        uint256 amount = sequencerFeeVault.MIN_WITHDRAWAL_AMOUNT();

        vm.deal(address(sequencerFeeVault), amount);
        // No ether has been withdrawn yet
        assertEq(sequencerFeeVault.totalProcessed(), 0);

        // Ensure the RECIPIENT reverts
        vm.etch(sequencerFeeVault.RECIPIENT(), type(Reverter).runtimeCode);

        // The entire vault's balance is withdrawn
        vm.expectCall(sequencerFeeVault.recipient(), address(sequencerFeeVault).balance, bytes(""));
        vm.expectRevert("FeeVault: failed to send ETH to L2 fee recipient");
        sequencerFeeVault.withdraw();
        assertEq(sequencerFeeVault.totalProcessed(), 0);
    }
}
