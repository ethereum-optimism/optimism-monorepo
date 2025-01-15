// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing utilities
import { CommonTest } from "test/setup/CommonTest.sol";
import { Reverter } from "test/mocks/Callers.sol";
import { stdError } from "forge-std/StdError.sol";

// Libraries
import { AddressAliasHelper } from "src/vendor/AddressAliasHelper.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";
import { Hashing } from "src/libraries/Hashing.sol";
import { Encoding } from "src/libraries/Encoding.sol";
import { ForgeArtifacts } from "scripts/libraries/ForgeArtifacts.sol";

// Target contract dependencies
import { IL1CrossDomainMessenger } from "interfaces/L1/IL1CrossDomainMessenger.sol";
import { IOptimismPortal2 } from "interfaces/L1/IOptimismPortal2.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";

contract L1CrossDomainMessenger_Test is CommonTest {
    /// @dev The receiver address
    address recipient = address(0xabbaacdc);

    /// @dev The storage slot of the l2Sender
    uint256 senderSlotIndex;

    function setUp() public override {
        super.setUp();
        senderSlotIndex = ForgeArtifacts.getSlot("OptimismPortal2", "l2Sender").slot;
    }

    /// @dev Tests that the implementation is initialized correctly.
    /// @notice Marked virtual to be overridden in
    ///         test/kontrol/deployment/DeploymentSummary.t.sol
    function test_constructor_succeeds() external virtual {
        IL1CrossDomainMessenger impl = IL1CrossDomainMessenger(addressManager.getAddress("OVM_L1CrossDomainMessenger"));
        assertEq(address(impl.superchainConfig()), address(0));
        assertEq(address(impl.PORTAL()), address(0));
        assertEq(address(impl.portal()), address(0));

        // The constructor now uses _disableInitializers, whereas OP Mainnet has the other messenger in storage
        returnIfForkTest("L1CrossDomainMessenger_Test: impl storage differs on forked network");
        assertEq(address(impl.OTHER_MESSENGER()), address(0));
        assertEq(address(impl.otherMessenger()), address(0));
    }

    /// @dev Tests that the proxy is initialized correctly.
    function test_initialize_succeeds() external view {
        assertEq(address(l1CrossDomainMessenger.superchainConfig()), address(superchainConfig));
        assertEq(address(l1CrossDomainMessenger.PORTAL()), address(optimismPortal2));
        assertEq(address(l1CrossDomainMessenger.portal()), address(optimismPortal2));
        assertEq(address(l1CrossDomainMessenger.OTHER_MESSENGER()), Predeploys.L2_CROSS_DOMAIN_MESSENGER);
        assertEq(address(l1CrossDomainMessenger.otherMessenger()), Predeploys.L2_CROSS_DOMAIN_MESSENGER);
    }

    /// @dev Tests that the version can be decoded from the message nonce.
    function test_messageVersion_succeeds() external view {
        (, uint16 version) = Encoding.decodeVersionedNonce(l1CrossDomainMessenger.messageNonce());
        assertEq(version, l1CrossDomainMessenger.MESSAGE_VERSION());
    }

    /// @dev Tests that the sendMessage function is able to send a single message.
    /// TODO: this same test needs to be done with the legacy message type
    ///       by setting the message version to 0
    function test_sendMessage_succeeds() external {
        // deposit transaction on the optimism portal should be called
        vm.expectCall(
            address(optimismPortal2),
            abi.encodeCall(
                IOptimismPortal2.depositTransaction,
                (
                    Predeploys.L2_CROSS_DOMAIN_MESSENGER,
                    0,
                    l1CrossDomainMessenger.baseGas(hex"ff", 100),
                    false,
                    Encoding.encodeCrossDomainMessage(
                        l1CrossDomainMessenger.messageNonce(), alice, recipient, 0, 100, hex"ff"
                    )
                )
            )
        );

        // TransactionDeposited event
        vm.expectEmit(address(optimismPortal2));
        emitTransactionDeposited(
            AddressAliasHelper.applyL1ToL2Alias(address(l1CrossDomainMessenger)),
            Predeploys.L2_CROSS_DOMAIN_MESSENGER,
            0,
            0,
            l1CrossDomainMessenger.baseGas(hex"ff", 100),
            false,
            Encoding.encodeCrossDomainMessage(l1CrossDomainMessenger.messageNonce(), alice, recipient, 0, 100, hex"ff")
        );

        // SentMessage event
        vm.expectEmit(address(l1CrossDomainMessenger));
        emit SentMessage(recipient, alice, hex"ff", l1CrossDomainMessenger.messageNonce(), 100);

        // SentMessageExtension1 event
        vm.expectEmit(address(l1CrossDomainMessenger));
        emit SentMessageExtension1(alice, 0);

        vm.prank(alice);
        l1CrossDomainMessenger.sendMessage(recipient, hex"ff", uint32(100));
    }

    /// @dev Tests that the sendMessage function is able to send
    ///      the same message twice.
    function test_sendMessage_twice_succeeds() external {
        uint256 nonce = l1CrossDomainMessenger.messageNonce();
        l1CrossDomainMessenger.sendMessage(recipient, hex"aa", uint32(500_000));
        l1CrossDomainMessenger.sendMessage(recipient, hex"aa", uint32(500_000));
        // the nonce increments for each message sent
        assertEq(nonce + 2, l1CrossDomainMessenger.messageNonce());
    }

    /// @dev Tests that the xDomainMessageSender reverts when not set.
    function test_xDomainSender_notSet_reverts() external {
        vm.expectRevert("CrossDomainMessenger: xDomainMessageSender is not set");
        l1CrossDomainMessenger.xDomainMessageSender();
    }

    /// @dev Tests that the relayMessage function reverts when
    ///      the message version is not 0 or 1.
    /// @notice Marked virtual to be overridden in
    ///         test/kontrol/deployment/DeploymentSummary.t.sol
    function test_relayMessage_v2_reverts() external virtual {
        address target = address(0xabcd);
        address sender = Predeploys.L2_CROSS_DOMAIN_MESSENGER;

        // Set the value of op.l2Sender() to be the L2 Cross Domain Messenger.
        vm.store(address(optimismPortal2), bytes32(senderSlotIndex), bytes32(abi.encode(sender)));

        // Expect a revert.
        vm.expectRevert("CrossDomainMessenger: only version 0 or 1 messages are supported at this time");

        // Try to relay a v2 message.
        vm.prank(address(optimismPortal2));
        l1CrossDomainMessenger.relayMessage(
            Encoding.encodeVersionedNonce({ _nonce: 0, _version: 2 }), // nonce
            sender,
            target,
            0, // value
            0,
            hex"1111"
        );
    }

    /// @dev Tests that the relayMessage function is able to relay a message
    ///      successfully by calling the target contract.
    function test_relayMessage_succeeds() external {
        address target = address(0xabcd);
        address sender = Predeploys.L2_CROSS_DOMAIN_MESSENGER;

        vm.expectCall(target, hex"1111");

        // set the value of op.l2Sender() to be the L2 Cross Domain Messenger.
        vm.store(address(optimismPortal2), bytes32(senderSlotIndex), bytes32(abi.encode(sender)));
        vm.prank(address(optimismPortal2));

        vm.expectEmit(address(l1CrossDomainMessenger));

        bytes32 hash = Hashing.hashCrossDomainMessage(
            Encoding.encodeVersionedNonce({ _nonce: 0, _version: 1 }), sender, target, 0, 0, hex"1111"
        );

        emit RelayedMessage(hash);

        l1CrossDomainMessenger.relayMessage(
            Encoding.encodeVersionedNonce({ _nonce: 0, _version: 1 }), // nonce
            sender,
            target,
            0, // value
            0,
            hex"1111"
        );

        // the message hash is in the successfulMessages mapping
        assert(l1CrossDomainMessenger.successfulMessages(hash));
        // it is not in the received messages mapping
        assertEq(l1CrossDomainMessenger.failedMessages(hash), false);
    }

    /// @dev Tests that relayMessage reverts if caller is optimismPortal2 and the value sent does not match the amount
    function test_relayMessage_fromOtherMessengerValueMismatch_reverts() external {
        address target = alice;
        address sender = Predeploys.L2_CROSS_DOMAIN_MESSENGER;
        bytes memory message = hex"1111";

        // set the value of op.l2Sender() to be the L2CrossDomainMessenger.
        vm.store(address(optimismPortal2), bytes32(senderSlotIndex), bytes32(abi.encode(sender)));

        // correctly sending as OptimismPortal but amount does not match msg.value
        vm.deal(address(optimismPortal2), 10 ether);
        vm.prank(address(optimismPortal2));
        vm.expectRevert(stdError.assertionError);
        l1CrossDomainMessenger.relayMessage{ value: 10 ether }(
            Encoding.encodeVersionedNonce({ _nonce: 0, _version: 1 }), sender, target, 9 ether, 0, message
        );
    }

    /// @dev Tests that relayMessage reverts if a failed message is attempted to be replayed via the optimismPortal2
    function test_relayMessage_fromOtherMessengerFailedMessageReplay_reverts() external {
        address target = alice;
        address sender = Predeploys.L2_CROSS_DOMAIN_MESSENGER;
        bytes memory message = hex"1111";

        // set the value of op.l2Sender() to be the L2 Cross Domain Messenger.
        vm.store(address(optimismPortal2), bytes32(senderSlotIndex), bytes32(abi.encode(sender)));

        // make a failed message
        vm.etch(target, hex"fe");
        vm.prank(address(optimismPortal2));
        l1CrossDomainMessenger.relayMessage(
            Encoding.encodeVersionedNonce({ _nonce: 0, _version: 1 }), sender, target, 0, 0, message
        );

        // cannot replay messages when optimism portal is msg.sender
        vm.prank(address(optimismPortal2));
        vm.expectRevert(stdError.assertionError);
        l1CrossDomainMessenger.relayMessage(
            Encoding.encodeVersionedNonce({ _nonce: 0, _version: 1 }), sender, target, 0, 0, message
        );
    }

    /// @dev Tests that relayMessage reverts if attempting to relay a message
    ///      with l1CrossDomainMessenger as the target
    function test_relayMessage_toSelf_reverts() external {
        address sender = Predeploys.L2_CROSS_DOMAIN_MESSENGER;
        bytes memory message = hex"1111";

        vm.store(address(optimismPortal2), bytes32(senderSlotIndex), bytes32(abi.encode(sender)));

        vm.prank(address(optimismPortal2));
        vm.expectRevert("CrossDomainMessenger: cannot send message to blocked system address");
        l1CrossDomainMessenger.relayMessage(
            Encoding.encodeVersionedNonce({ _nonce: 0, _version: 1 }),
            sender,
            address(l1CrossDomainMessenger),
            0,
            0,
            message
        );
    }

    /// @dev Tests that relayMessage reverts if attempting to relay a message
    ///      with optimismPortal as the target
    function test_relayMessage_toOptimismPortal_reverts() external {
        address sender = Predeploys.L2_CROSS_DOMAIN_MESSENGER;
        bytes memory message = hex"1111";

        vm.store(address(optimismPortal2), bytes32(senderSlotIndex), bytes32(abi.encode(sender)));

        vm.prank(address(optimismPortal2));
        vm.expectRevert("CrossDomainMessenger: cannot send message to blocked system address");
        l1CrossDomainMessenger.relayMessage(
            Encoding.encodeVersionedNonce({ _nonce: 0, _version: 1 }), sender, address(optimismPortal2), 0, 0, message
        );
    }

    /// @dev Tests that the relayMessage function reverts if the message called by non-optimismPortal2 but not a failed
    /// message
    function test_relayMessage_relayingNewMessageByExternalUser_reverts() external {
        address target = address(alice);
        address sender = Predeploys.L2_CROSS_DOMAIN_MESSENGER;
        bytes memory message = hex"1111";

        vm.store(address(optimismPortal2), bytes32(senderSlotIndex), bytes32(abi.encode(sender)));

        vm.prank(bob);
        vm.expectRevert("CrossDomainMessenger: message cannot be replayed");
        l1CrossDomainMessenger.relayMessage(
            Encoding.encodeVersionedNonce({ _nonce: 0, _version: 1 }), sender, target, 0, 0, message
        );
    }

    /// @dev Tests that the relayMessage function reverts if eth is
    ///      sent from a contract other than the standard bridge.
    function test_replayMessage_withValue_reverts() external {
        address target = address(0xabcd);
        address sender = Predeploys.L2_CROSS_DOMAIN_MESSENGER;
        bytes memory message = hex"1111";

        vm.expectRevert("CrossDomainMessenger: value must be zero unless message is from a system address");
        l1CrossDomainMessenger.relayMessage{ value: 100 }(
            Encoding.encodeVersionedNonce({ _nonce: 0, _version: 1 }), sender, target, 0, 0, message
        );
    }

    /// @dev Tests that the xDomainMessageSender is reset to the original value
    ///      after a message is relayed.
    function test_xDomainMessageSender_reset_succeeds() external {
        vm.expectRevert("CrossDomainMessenger: xDomainMessageSender is not set");
        l1CrossDomainMessenger.xDomainMessageSender();

        address sender = Predeploys.L2_CROSS_DOMAIN_MESSENGER;

        vm.store(address(optimismPortal2), bytes32(senderSlotIndex), bytes32(abi.encode(sender)));
        vm.prank(address(optimismPortal2));
        l1CrossDomainMessenger.relayMessage(
            Encoding.encodeVersionedNonce({ _nonce: 0, _version: 1 }), address(0), address(0), 0, 0, hex""
        );

        vm.expectRevert("CrossDomainMessenger: xDomainMessageSender is not set");
        l1CrossDomainMessenger.xDomainMessageSender();
    }

    /// @dev Tests that relayMessage should successfully call the target contract after
    ///      the first message fails and ETH is stuck, but the second message succeeds
    ///      with a version 1 message.
    function test_relayMessage_retryAfterFailure_succeeds() external {
        address target = address(0xabcd);
        address sender = Predeploys.L2_CROSS_DOMAIN_MESSENGER;
        uint256 value = 100;

        vm.expectCall(target, hex"1111");

        bytes32 hash = Hashing.hashCrossDomainMessage(
            Encoding.encodeVersionedNonce({ _nonce: 0, _version: 1 }), sender, target, value, 0, hex"1111"
        );

        vm.store(address(optimismPortal2), bytes32(senderSlotIndex), bytes32(abi.encode(sender)));
        vm.etch(target, address(new Reverter()).code);
        vm.deal(address(optimismPortal2), value);
        vm.prank(address(optimismPortal2));
        l1CrossDomainMessenger.relayMessage{ value: value }(
            Encoding.encodeVersionedNonce({ _nonce: 0, _version: 1 }), // nonce
            sender,
            target,
            value,
            0,
            hex"1111"
        );

        assertEq(address(l1CrossDomainMessenger).balance, value);
        assertEq(address(target).balance, 0);
        assertEq(l1CrossDomainMessenger.successfulMessages(hash), false);
        assertEq(l1CrossDomainMessenger.failedMessages(hash), true);

        vm.expectEmit(address(l1CrossDomainMessenger));

        emit RelayedMessage(hash);

        vm.etch(target, address(0).code);
        vm.prank(address(sender));
        l1CrossDomainMessenger.relayMessage(
            Encoding.encodeVersionedNonce({ _nonce: 0, _version: 1 }), // nonce
            sender,
            target,
            value,
            0,
            hex"1111"
        );

        assertEq(address(l1CrossDomainMessenger).balance, 0);
        assertEq(address(target).balance, value);
        assertEq(l1CrossDomainMessenger.successfulMessages(hash), true);
        assertEq(l1CrossDomainMessenger.failedMessages(hash), true);
    }

    /// @dev Tests that relayMessage should successfully call the target contract after
    ///      the first message fails and ETH is stuck, but the second message succeeds
    ///      with a legacy message.
    function test_relayMessage_legacy_succeeds() external {
        address target = address(0xabcd);
        address sender = Predeploys.L2_CROSS_DOMAIN_MESSENGER;

        // Compute the message hash.
        bytes32 hash = Hashing.hashCrossDomainMessageV1(
            // Using a legacy nonce with version 0.
            Encoding.encodeVersionedNonce({ _nonce: 0, _version: 0 }),
            sender,
            target,
            0,
            0,
            hex"1111"
        );

        // Set the value of op.l2Sender() to be the L2 Cross Domain Messenger.
        vm.store(address(optimismPortal2), bytes32(senderSlotIndex), bytes32(abi.encode(sender)));

        // Target should be called with expected data.
        vm.expectCall(target, hex"1111");

        // Expect RelayedMessage event to be emitted.
        vm.expectEmit(address(l1CrossDomainMessenger));
        emit RelayedMessage(hash);

        // Relay the message.
        vm.prank(address(optimismPortal2));
        l1CrossDomainMessenger.relayMessage(
            Encoding.encodeVersionedNonce({ _nonce: 0, _version: 0 }), // nonce
            sender,
            target,
            0, // value
            0,
            hex"1111"
        );

        // Message was successfully relayed.
        assertEq(l1CrossDomainMessenger.successfulMessages(hash), true);
        assertEq(l1CrossDomainMessenger.failedMessages(hash), false);
    }

    /// @dev Tests that relayMessage should revert if the message is already replayed.
    function test_relayMessage_legacyOldReplay_reverts() external {
        address target = address(0xabcd);
        address sender = Predeploys.L2_CROSS_DOMAIN_MESSENGER;

        // Compute the message hash.
        bytes32 hash = Hashing.hashCrossDomainMessageV1(
            // Using a legacy nonce with version 0.
            Encoding.encodeVersionedNonce({ _nonce: 0, _version: 0 }),
            sender,
            target,
            0,
            0,
            hex"1111"
        );

        // Set the value of op.l2Sender() to be the L2 Cross Domain Messenger.
        vm.store(address(optimismPortal2), bytes32(senderSlotIndex), bytes32(abi.encode(sender)));
        // Mark legacy message as already relayed.
        uint256 successfulMessagesSlot = 203;
        bytes32 oldHash = Hashing.hashCrossDomainMessageV0(target, sender, hex"1111", 0);
        bytes32 slot = keccak256(abi.encode(oldHash, successfulMessagesSlot));
        vm.store(address(l1CrossDomainMessenger), slot, bytes32(uint256(1)));

        // Expect revert.
        vm.expectRevert("CrossDomainMessenger: legacy withdrawal already relayed");

        // Relay the message.
        vm.prank(address(optimismPortal2));
        l1CrossDomainMessenger.relayMessage(
            Encoding.encodeVersionedNonce({ _nonce: 0, _version: 0 }), // nonce
            sender,
            target,
            0, // value
            0,
            hex"1111"
        );

        // Message was not relayed.
        assertEq(l1CrossDomainMessenger.successfulMessages(hash), false);
        assertEq(l1CrossDomainMessenger.failedMessages(hash), false);
    }

    /// @dev Tests that relayMessage can be retried after a failure with a legacy message.
    function test_relayMessage_legacyRetryAfterFailure_succeeds() external {
        address target = address(0xabcd);
        address sender = Predeploys.L2_CROSS_DOMAIN_MESSENGER;
        uint256 value = 100;

        // Compute the message hash.
        bytes32 hash = Hashing.hashCrossDomainMessageV1(
            // Using a legacy nonce with version 0.
            Encoding.encodeVersionedNonce({ _nonce: 0, _version: 0 }),
            sender,
            target,
            value,
            0,
            hex"1111"
        );

        // Set the value of op.l2Sender() to be the L2 Cross Domain Messenger.
        vm.store(address(optimismPortal2), bytes32(senderSlotIndex), bytes32(abi.encode(sender)));

        // Turn the target into a Reverter.
        vm.etch(target, address(new Reverter()).code);

        // Target should be called with expected data.
        vm.expectCall(target, hex"1111");

        // Expect FailedRelayedMessage event to be emitted.
        vm.expectEmit(address(l1CrossDomainMessenger));
        emit FailedRelayedMessage(hash);

        // Relay the message.
        vm.deal(address(optimismPortal2), value);
        vm.prank(address(optimismPortal2));
        l1CrossDomainMessenger.relayMessage{ value: value }(
            Encoding.encodeVersionedNonce({ _nonce: 0, _version: 0 }), // nonce
            sender,
            target,
            value,
            0,
            hex"1111"
        );

        // Message failed.
        assertEq(address(l1CrossDomainMessenger).balance, value);
        assertEq(address(target).balance, 0);
        assertEq(l1CrossDomainMessenger.successfulMessages(hash), false);
        assertEq(l1CrossDomainMessenger.failedMessages(hash), true);

        // Make the target not revert anymore.
        vm.etch(target, address(0).code);

        // Target should be called with expected data.
        vm.expectCall(target, hex"1111");

        // Expect RelayedMessage event to be emitted.
        vm.expectEmit(address(l1CrossDomainMessenger));
        emit RelayedMessage(hash);

        // Retry the message.
        vm.prank(address(sender));
        l1CrossDomainMessenger.relayMessage(
            Encoding.encodeVersionedNonce({ _nonce: 0, _version: 0 }), // nonce
            sender,
            target,
            value,
            0,
            hex"1111"
        );

        // Message was successfully relayed.
        assertEq(address(l1CrossDomainMessenger).balance, 0);
        assertEq(address(target).balance, value);
        assertEq(l1CrossDomainMessenger.successfulMessages(hash), true);
        assertEq(l1CrossDomainMessenger.failedMessages(hash), true);
    }

    /// @dev Tests that relayMessage cannot be retried after success with a legacy message.
    function test_relayMessage_legacyRetryAfterSuccess_reverts() external {
        address target = address(0xabcd);
        address sender = Predeploys.L2_CROSS_DOMAIN_MESSENGER;
        uint256 value = 100;

        // Compute the message hash.
        bytes32 hash = Hashing.hashCrossDomainMessageV1(
            // Using a legacy nonce with version 0.
            Encoding.encodeVersionedNonce({ _nonce: 0, _version: 0 }),
            sender,
            target,
            value,
            0,
            hex"1111"
        );

        // Set the value of op.l2Sender() to be the L2 Cross Domain Messenger.
        vm.store(address(optimismPortal2), bytes32(senderSlotIndex), bytes32(abi.encode(sender)));

        // Target should be called with expected data.
        vm.expectCall(target, hex"1111");

        // Expect RelayedMessage event to be emitted.
        vm.expectEmit(address(l1CrossDomainMessenger));
        emit RelayedMessage(hash);

        // Relay the message.
        vm.deal(address(optimismPortal2), value);
        vm.prank(address(optimismPortal2));
        l1CrossDomainMessenger.relayMessage{ value: value }(
            Encoding.encodeVersionedNonce({ _nonce: 0, _version: 0 }), // nonce
            sender,
            target,
            value,
            0,
            hex"1111"
        );

        // Message was successfully relayed.
        assertEq(address(l1CrossDomainMessenger).balance, 0);
        assertEq(address(target).balance, value);
        assertEq(l1CrossDomainMessenger.successfulMessages(hash), true);
        assertEq(l1CrossDomainMessenger.failedMessages(hash), false);

        // Expect a revert.
        vm.expectRevert("CrossDomainMessenger: message cannot be replayed");

        // Retry the message.
        vm.prank(address(sender));
        l1CrossDomainMessenger.relayMessage(
            Encoding.encodeVersionedNonce({ _nonce: 0, _version: 0 }), // nonce
            sender,
            target,
            value,
            0,
            hex"1111"
        );
    }

    /// @dev Tests that relayMessage cannot be called after a failure and a successful replay.
    function test_relayMessage_legacyRetryAfterFailureThenSuccess_reverts() external {
        address target = address(0xabcd);
        address sender = Predeploys.L2_CROSS_DOMAIN_MESSENGER;
        uint256 value = 100;

        // Compute the message hash.
        bytes32 hash = Hashing.hashCrossDomainMessageV1(
            // Using a legacy nonce with version 0.
            Encoding.encodeVersionedNonce({ _nonce: 0, _version: 0 }),
            sender,
            target,
            value,
            0,
            hex"1111"
        );

        // Set the value of op.l2Sender() to be the L2 Cross Domain Messenger.
        vm.store(address(optimismPortal2), bytes32(senderSlotIndex), bytes32(abi.encode(sender)));

        // Turn the target into a Reverter.
        vm.etch(target, address(new Reverter()).code);

        // Target should be called with expected data.
        vm.expectCall(target, hex"1111");

        // Relay the message.
        vm.deal(address(optimismPortal2), value);
        vm.prank(address(optimismPortal2));
        l1CrossDomainMessenger.relayMessage{ value: value }(
            Encoding.encodeVersionedNonce({ _nonce: 0, _version: 0 }), // nonce
            sender,
            target,
            value,
            0,
            hex"1111"
        );

        // Message failed.
        assertEq(address(l1CrossDomainMessenger).balance, value);
        assertEq(address(target).balance, 0);
        assertEq(l1CrossDomainMessenger.successfulMessages(hash), false);
        assertEq(l1CrossDomainMessenger.failedMessages(hash), true);

        // Make the target not revert anymore.
        vm.etch(target, address(0).code);

        // Target should be called with expected data.
        vm.expectCall(target, hex"1111");

        // Expect RelayedMessage event to be emitted.
        vm.expectEmit(address(l1CrossDomainMessenger));
        emit RelayedMessage(hash);

        // Retry the message
        vm.prank(address(sender));
        l1CrossDomainMessenger.relayMessage(
            Encoding.encodeVersionedNonce({ _nonce: 0, _version: 0 }), // nonce
            sender,
            target,
            value,
            0,
            hex"1111"
        );

        // Message was successfully relayed.
        assertEq(address(l1CrossDomainMessenger).balance, 0);
        assertEq(address(target).balance, value);
        assertEq(l1CrossDomainMessenger.successfulMessages(hash), true);
        assertEq(l1CrossDomainMessenger.failedMessages(hash), true);

        // Expect a revert.
        vm.expectRevert("CrossDomainMessenger: message has already been relayed");

        // Retry the message again.
        vm.prank(address(sender));
        l1CrossDomainMessenger.relayMessage(
            Encoding.encodeVersionedNonce({ _nonce: 0, _version: 0 }), // nonce
            sender,
            target,
            value,
            0,
            hex"1111"
        );
    }

    /// @dev Tests that the relayMessage function is able to relay a message
    ///      successfully by calling the target contract.
    function test_relayMessage_paused_reverts() external {
        vm.prank(superchainConfig.guardian());
        superchainConfig.pause("identifier");
        vm.expectRevert("CrossDomainMessenger: paused");

        l1CrossDomainMessenger.relayMessage(
            Encoding.encodeVersionedNonce({ _nonce: 0, _version: 1 }), // nonce
            address(0),
            address(0),
            0, // value
            0,
            hex"1111"
        );
    }

    /// @dev Tests that the superchain config is called by the messengers paused function
    function test_pause_callsSuperchainConfig_succeeds() external {
        vm.expectCall(address(superchainConfig), abi.encodeCall(ISuperchainConfig.paused, ()));
        l1CrossDomainMessenger.paused();
    }

    /// @dev Tests that changing the superchain config paused status changes the return value of the messenger
    function test_pause_matchesSuperchainConfig_succeeds() external {
        assertFalse(l1CrossDomainMessenger.paused());
        assertEq(l1CrossDomainMessenger.paused(), superchainConfig.paused());

        vm.prank(superchainConfig.guardian());
        superchainConfig.pause("identifier");

        assertTrue(l1CrossDomainMessenger.paused());
        assertEq(l1CrossDomainMessenger.paused(), superchainConfig.paused());
    }
}

/// @dev A regression test against a reentrancy vulnerability in the CrossDomainMessenger contract, which
///      was possible by intercepting and sandwhiching a signed Safe Transaction to upgrade it.
contract L1CrossDomainMessenger_ReinitReentryTest is CommonTest {
    bool attacked;

    // Common values used across functions
    uint256 constant messageValue = 50;
    bytes selector = abi.encodeCall(this.reinitAndReenter, ());
    address sender;
    bytes32 hash;
    address target;

    function setUp() public override {
        super.setUp();
        target = address(this);
        sender = Predeploys.L2_CROSS_DOMAIN_MESSENGER;
        hash = Hashing.hashCrossDomainMessage(
            Encoding.encodeVersionedNonce({ _nonce: 0, _version: 1 }), sender, target, messageValue, 0, selector
        );
        vm.deal(address(l1CrossDomainMessenger), messageValue * 2);
    }

    /// @dev This method will be called by the relayed message, and will attempt to reenter the relayMessage function
    ///      exactly once.
    function reinitAndReenter() external payable {
        // only attempt the attack once
        if (!attacked) {
            attacked = true;
            // set initialized to false
            vm.store(address(l1CrossDomainMessenger), 0, bytes32(uint256(0)));

            // call the initializer function
            l1CrossDomainMessenger.initialize(ISuperchainConfig(superchainConfig), IOptimismPortal2(optimismPortal2));

            // attempt to re-replay the withdrawal
            vm.expectEmit(address(l1CrossDomainMessenger));
            emit FailedRelayedMessage(hash);
            l1CrossDomainMessenger.relayMessage(
                Encoding.encodeVersionedNonce({ _nonce: 0, _version: 1 }), // nonce
                sender,
                target,
                messageValue,
                0,
                selector
            );
        }
    }

    /// @dev Tests that the relayMessage function cannot be reentered by calling the `initialize()` function within the
    ///      relayed message.
    function test_relayMessage_replayStraddlingReinit_reverts() external {
        uint256 balanceBeforeThis = address(this).balance;
        uint256 balanceBeforeMessenger = address(l1CrossDomainMessenger).balance;

        // A requisite for the attack is that the message has already been attempted and written to the failedMessages
        // mapping, so that it can be replayed.
        vm.store(address(l1CrossDomainMessenger), keccak256(abi.encode(hash, 206)), bytes32(uint256(1)));
        assertTrue(l1CrossDomainMessenger.failedMessages(hash));

        vm.expectEmit(address(l1CrossDomainMessenger));
        emit FailedRelayedMessage(hash);
        l1CrossDomainMessenger.relayMessage(
            Encoding.encodeVersionedNonce({ _nonce: 0, _version: 1 }), // nonce
            sender,
            target,
            messageValue,
            0,
            selector
        );

        // The message hash is not in the successfulMessages mapping.
        assertFalse(l1CrossDomainMessenger.successfulMessages(hash));
        // The balance of this contract is unchanged.
        assertEq(address(this).balance, balanceBeforeThis);
        // The balance of the messenger contract is unchanged.
        assertEq(address(l1CrossDomainMessenger).balance, balanceBeforeMessenger);
    }
}
