// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { CommonTest } from "test/setup/CommonTest.sol";
import { Reverter } from "test/mocks/Callers.sol";
import { EIP1967Helper } from "test/mocks/EIP1967Helper.sol";

// Libraries
import { Hashing } from "src/libraries/Hashing.sol";
import { Encoding } from "src/libraries/Encoding.sol";
import { Types } from "src/libraries/Types.sol";
import { AddressAliasHelper } from "src/vendor/AddressAliasHelper.sol";

// Interfaces
import { IL2CrossDomainMessenger } from "src/L2/interfaces/IL2CrossDomainMessenger.sol";
import { IL2ToL1MessagePasser } from "src/L2/interfaces/IL2ToL1MessagePasser.sol";

contract L2CrossDomainMessenger_Test is CommonTest {
    /// @dev Receiver address for testing
    address recipient = address(0xabbaacdc);

    /// @dev Tests that the implementation is initialized correctly.
    function test_constructor_succeeds() external view {
        IL2CrossDomainMessenger impl =
            IL2CrossDomainMessenger(EIP1967Helper.getImplementation(deploy.mustGetAddress("L2CrossDomainMessenger")));
        assertEq(address(impl.OTHER_MESSENGER()), address(0));
        assertEq(address(impl.otherMessenger()), address(0));
        assertEq(address(impl.l1CrossDomainMessenger()), address(0));
    }

    /// @dev Tests that the proxy is initialized correctly.
    function test_initialize_succeeds() external view {
        assertEq(address(l2CrossDomainMessenger.OTHER_MESSENGER()), address(l1CrossDomainMessenger));
        assertEq(address(l2CrossDomainMessenger.otherMessenger()), address(l1CrossDomainMessenger));
        assertEq(address(l2CrossDomainMessenger.l1CrossDomainMessenger()), address(l1CrossDomainMessenger));
    }

    /// @dev Tests that `messageNonce` can be decoded correctly.
    function test_messageVersion_succeeds() external view {
        (, uint16 version) = Encoding.decodeVersionedNonce(l2CrossDomainMessenger.messageNonce());
        assertEq(version, l2CrossDomainMessenger.MESSAGE_VERSION());
    }

    /// @dev Tests that `sendMessage` executes successfully.
    function test_sendMessage_succeeds() external {
        bytes memory xDomainCallData =
            Encoding.encodeCrossDomainMessage(l2CrossDomainMessenger.messageNonce(), alice, recipient, 0, 100, hex"ff");
        vm.expectCall(
            address(l2ToL1MessagePasser),
            abi.encodeCall(
                IL2ToL1MessagePasser.initiateWithdrawal,
                (address(l1CrossDomainMessenger), l2CrossDomainMessenger.baseGas(hex"ff", 100), xDomainCallData)
            )
        );

        // MessagePassed event
        vm.expectEmit(true, true, true, true);
        emit MessagePassed(
            l2ToL1MessagePasser.messageNonce(),
            address(l2CrossDomainMessenger),
            address(l1CrossDomainMessenger),
            0,
            l2CrossDomainMessenger.baseGas(hex"ff", 100),
            xDomainCallData,
            Hashing.hashWithdrawal(
                Types.WithdrawalTransaction({
                    nonce: l2ToL1MessagePasser.messageNonce(),
                    sender: address(l2CrossDomainMessenger),
                    target: address(l1CrossDomainMessenger),
                    value: 0,
                    gasLimit: l2CrossDomainMessenger.baseGas(hex"ff", 100),
                    data: xDomainCallData
                })
            )
        );

        vm.prank(alice);
        l2CrossDomainMessenger.sendMessage(recipient, hex"ff", uint32(100));
    }

    /// @dev Tests that `sendMessage` can be called twice and that
    ///      the nonce increments correctly.
    function test_sendMessage_twice_succeeds() external {
        uint256 nonce = l2CrossDomainMessenger.messageNonce();
        l2CrossDomainMessenger.sendMessage(recipient, hex"aa", uint32(500_000));
        l2CrossDomainMessenger.sendMessage(recipient, hex"aa", uint32(500_000));
        // the nonce increments for each message sent
        assertEq(nonce + 2, l2CrossDomainMessenger.messageNonce());
    }

    /// @dev Tests that `sendMessage` reverts if the recipient is the zero address.
    function test_xDomainSender_senderNotSet_reverts() external {
        vm.expectRevert("CrossDomainMessenger: xDomainMessageSender is not set");
        l2CrossDomainMessenger.xDomainMessageSender();
    }

    /// @dev Tests that `sendMessage` reverts if the message version is not supported.
    function test_relayMessage_v2_reverts() external {
        address target = address(0xabcd);
        address sender = address(l1CrossDomainMessenger);
        address caller = AddressAliasHelper.applyL1ToL2Alias(address(l1CrossDomainMessenger));

        // Expect a revert.
        vm.expectRevert("CrossDomainMessenger: only version 0 or 1 messages are supported at this time");

        // Try to relay a v2 message.
        vm.prank(caller);
        l2CrossDomainMessenger.relayMessage(
            Encoding.encodeVersionedNonce(0, 2), // nonce
            sender,
            target,
            0, // value
            0,
            hex"1111"
        );
    }

    /// @dev Tests that `relayMessage` executes successfully.
    function test_relayMessage_succeeds() external {
        address target = address(0xabcd);
        address sender = address(l1CrossDomainMessenger);
        address caller = AddressAliasHelper.applyL1ToL2Alias(address(l1CrossDomainMessenger));

        vm.expectCall(target, hex"1111");

        vm.prank(caller);

        vm.expectEmit(true, true, true, true);

        bytes32 hash =
            Hashing.hashCrossDomainMessage(Encoding.encodeVersionedNonce(0, 1), sender, target, 0, 0, hex"1111");

        emit RelayedMessage(hash);

        l2CrossDomainMessenger.relayMessage(
            Encoding.encodeVersionedNonce(0, 1), // nonce
            sender,
            target,
            0, // value
            0,
            hex"1111"
        );

        // the message hash is in the successfulMessages mapping
        assert(l2CrossDomainMessenger.successfulMessages(hash));
        // it is not in the received messages mapping
        assertEq(l2CrossDomainMessenger.failedMessages(hash), false);
    }

    /// @dev Tests that `relayMessage` reverts if attempting to relay
    ///      a message sent to an L1 system contract.
    function test_relayMessage_toSystemContract_reverts() external {
        address target = address(l2ToL1MessagePasser);
        address sender = address(l1CrossDomainMessenger);
        address caller = AddressAliasHelper.applyL1ToL2Alias(address(l1CrossDomainMessenger));
        bytes memory message = hex"1111";

        vm.prank(caller);
        vm.expectRevert("CrossDomainMessenger: message cannot be replayed");
        l1CrossDomainMessenger.relayMessage(Encoding.encodeVersionedNonce(0, 1), sender, target, 0, 0, message);
    }

    /// @dev Tests that `relayMessage` correctly resets the `xDomainMessageSender`
    ///      to the original value after a message is relayed.
    function test_xDomainMessageSender_reset_succeeds() external {
        vm.expectRevert("CrossDomainMessenger: xDomainMessageSender is not set");
        l2CrossDomainMessenger.xDomainMessageSender();

        address caller = AddressAliasHelper.applyL1ToL2Alias(address(l1CrossDomainMessenger));
        vm.prank(caller);
        l2CrossDomainMessenger.relayMessage(Encoding.encodeVersionedNonce(0, 1), address(0), address(0), 0, 0, hex"");

        vm.expectRevert("CrossDomainMessenger: xDomainMessageSender is not set");
        l2CrossDomainMessenger.xDomainMessageSender();
    }

    /// @dev Tests that `relayMessage` is able to send a successful call
    ///      to the target contract after the first message fails and ETH
    ///      gets stuck, but the second message succeeds.
    function test_relayMessage_retry_succeeds() external {
        address target = address(0xabcd);
        address sender = address(l1CrossDomainMessenger);
        address caller = AddressAliasHelper.applyL1ToL2Alias(address(l1CrossDomainMessenger));
        uint256 value = 100;

        bytes32 hash =
            Hashing.hashCrossDomainMessage(Encoding.encodeVersionedNonce(0, 1), sender, target, value, 0, hex"1111");

        vm.etch(target, address(new Reverter()).code);
        vm.deal(address(caller), value);
        vm.prank(caller);
        l2CrossDomainMessenger.relayMessage{ value: value }(
            Encoding.encodeVersionedNonce(0, 1), // nonce
            sender,
            target,
            value,
            0,
            hex"1111"
        );

        assertEq(address(l2CrossDomainMessenger).balance, value);
        assertEq(address(target).balance, 0);
        assertEq(l2CrossDomainMessenger.successfulMessages(hash), false);
        assertEq(l2CrossDomainMessenger.failedMessages(hash), true);

        vm.expectEmit(true, true, true, true);

        emit RelayedMessage(hash);

        vm.etch(target, address(0).code);
        vm.prank(address(sender));
        l2CrossDomainMessenger.relayMessage(
            Encoding.encodeVersionedNonce(0, 1), // nonce
            sender,
            target,
            value,
            0,
            hex"1111"
        );

        assertEq(address(l2CrossDomainMessenger).balance, 0);
        assertEq(address(target).balance, value);
        assertEq(l2CrossDomainMessenger.successfulMessages(hash), true);
        assertEq(l2CrossDomainMessenger.failedMessages(hash), true);
    }

    /// @dev Tests that sendMessage succeeds with a custom gas token when the call value is zero.
    function test_sendMessage_customGasTokenButNoValue_succeeds() external {
        // Mock the gasPayingToken function to return a custom gas token
        vm.mockCall(address(l1Block), abi.encodeCall(systemConfig.gasPayingToken, ()), abi.encode(address(1), uint8(2)));

        bytes memory xDomainCallData =
            Encoding.encodeCrossDomainMessage(l2CrossDomainMessenger.messageNonce(), alice, recipient, 0, 100, hex"ff");
        vm.expectCall(
            address(l2ToL1MessagePasser),
            abi.encodeCall(
                IL2ToL1MessagePasser.initiateWithdrawal,
                (address(l1CrossDomainMessenger), l2CrossDomainMessenger.baseGas(hex"ff", 100), xDomainCallData)
            )
        );

        // MessagePassed event
        vm.expectEmit(true, true, true, true);
        emit MessagePassed(
            l2ToL1MessagePasser.messageNonce(),
            address(l2CrossDomainMessenger),
            address(l1CrossDomainMessenger),
            0,
            l2CrossDomainMessenger.baseGas(hex"ff", 100),
            xDomainCallData,
            Hashing.hashWithdrawal(
                Types.WithdrawalTransaction({
                    nonce: l2ToL1MessagePasser.messageNonce(),
                    sender: address(l2CrossDomainMessenger),
                    target: address(l1CrossDomainMessenger),
                    value: 0,
                    gasLimit: l2CrossDomainMessenger.baseGas(hex"ff", 100),
                    data: xDomainCallData
                })
            )
        );

        vm.prank(alice);
        l2CrossDomainMessenger.sendMessage(recipient, hex"ff", uint32(100));
    }

    /// @dev Tests that the sendMessage reverts when call value is non-zero with custom gas token.
    function test_sendMessage_customGasTokenWithValue_reverts() external {
        // Mock the gasPayingToken function to return a custom gas token
        vm.mockCall(address(l1Block), abi.encodeCall(systemConfig.gasPayingToken, ()), abi.encode(address(1), uint8(2)));

        vm.expectRevert("CrossDomainMessenger: cannot send value with custom gas token");
        l2CrossDomainMessenger.sendMessage{ value: 1 }(recipient, hex"ff", uint32(100));
    }

    /// @dev Tests that the relayMessage succeeds with a custom gas token when the call value is zero.
    function test_relayMessage_customGasTokenAndNoValue_succeeds() external {
        // Mock the gasPayingToken function to return a custom gas token
        vm.mockCall(address(l1Block), abi.encodeCall(systemConfig.gasPayingToken, ()), abi.encode(address(1), uint8(2)));

        address target = address(0xabcd);
        address sender = address(l1CrossDomainMessenger);
        address caller = AddressAliasHelper.applyL1ToL2Alias(address(l1CrossDomainMessenger));

        vm.expectCall(target, hex"1111");

        vm.prank(caller);

        vm.expectEmit(true, true, true, true);

        bytes32 hash =
            Hashing.hashCrossDomainMessage(Encoding.encodeVersionedNonce(0, 1), sender, target, 0, 0, hex"1111");

        emit RelayedMessage(hash);

        l2CrossDomainMessenger.relayMessage(
            Encoding.encodeVersionedNonce(0, 1), // nonce
            sender,
            target,
            0, // value
            0,
            hex"1111"
        );

        // the message hash is in the successfulMessages mapping
        assert(l2CrossDomainMessenger.successfulMessages(hash));
        // it is not in the received messages mapping
        assertEq(l2CrossDomainMessenger.failedMessages(hash), false);
    }

    /// @dev Tests that the relayMessage reverts when call value is non-zero with custom gas token.
    ///       The L1CrossDomainMessenger `sendMessage` function cannot send value with a custom gas token.
    function test_relayMessage_customGasTokenWithValue_reverts() external virtual {
        // Mock the gasPayingToken function to return a custom gas token
        vm.mockCall(address(l1Block), abi.encodeCall(systemConfig.gasPayingToken, ()), abi.encode(address(1), uint8(2)));
        vm.expectRevert("CrossDomainMessenger: value must be zero unless message is from a system address");

        l2CrossDomainMessenger.relayMessage{ value: 1 }(
            Encoding.encodeVersionedNonce({ _nonce: 0, _version: 1 }),
            address(0xabcd),
            address(0xabcd),
            1, // value
            0,
            hex"1111"
        );
    }
}
