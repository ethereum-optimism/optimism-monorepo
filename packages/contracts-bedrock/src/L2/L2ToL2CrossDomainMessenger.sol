// SPDX-License-Identifier: MIT
pragma solidity 0.8.25;

import { Encoding } from "src/libraries/Encoding.sol";
import { Hashing } from "src/libraries/Hashing.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";
import { CrossL2Inbox, Identifier } from "src/L2/CrossL2Inbox.sol";
import { ISemver } from "src/universal/interfaces/ISemver.sol";
import { SafeCall } from "src/libraries/SafeCall.sol";
import { TransientReentrancyAware } from "src/libraries/TransientContext.sol";

/// @notice Thrown when a non-written slot in transient storage is attempted to be read from.
error NotEntered();

/// @notice Thrown when attempting to relay a message where payload origin is not L2ToL2CrossDomainMessenger.
error IdOriginNotL2ToL2CrossDomainMessenger();

/// @notice Thrown when the payload provided to the relay is not a SentMessage event.
error EventPayloadNotSentMessage();

/// @notice Thrown when attempting to send a message to the chain that the message is being sent from.
error MessageDestinationSameChain();

/// @notice Thrown when attempting to relay a message whose destination chain is not the chain relaying it.
error MessageDestinationNotRelayChain();

/// @notice Thrown when attempting to relay a message whose target is CrossL2Inbox.
error MessageTargetCrossL2Inbox();

/// @notice Thrown when attempting to relay a message whose target is L2ToL2CrossDomainMessenger.
error MessageTargetL2ToL2CrossDomainMessenger();

/// @notice Thrown when attempting to relay a message that has already been relayed.
error MessageAlreadyRelayed();

/// @notice Thrown when a reentrant call is detected.
error ReentrantCall();

/// @notice Thrown when a call to the target contract during message relay fails.
error TargetCallFailed();

/// @custom:proxied true
/// @custom:predeploy 0x4200000000000000000000000000000000000023
/// @title L2ToL2CrossDomainMessenger
/// @notice The L2ToL2CrossDomainMessenger is a higher level abstraction on top of the CrossL2Inbox that provides
///         features necessary for secure transfers ERC20 tokens between L2 chains. Messages sent through the
///         L2ToL2CrossDomainMessenger on the source chain receive both replay protection as well as domain binding.
contract L2ToL2CrossDomainMessenger is ISemver, TransientReentrancyAware {
    /// @notice Storage slot for the sender of the current cross domain message.
    ///         Equal to bytes32(uint256(keccak256("l2tol2crossdomainmessenger.sender")) - 1)
    bytes32 internal constant CROSS_DOMAIN_MESSAGE_SENDER_SLOT =
        0xb83444d07072b122e2e72a669ce32857d892345c19856f4e7142d06a167ab3f3;

    /// @notice Storage slot for the source of the current cross domain message.
    ///         Equal to bytes32(uint256(keccak256("l2tol2crossdomainmessenger.source")) - 1)
    bytes32 internal constant CROSS_DOMAIN_MESSAGE_SOURCE_SLOT =
        0x711dfa3259c842fffc17d6e1f1e0fc5927756133a2345ca56b4cb8178589fee7;

    /// @notice Event selector for the SentMessage event. Will be removed in favor of reading
    //          the `selector` property directly once crytic/slithe/#2566 is fixed.
    bytes32 internal constant SENT_MESSAGE_EVENT_SELECTOR =
        0x382409ac69001e11931a28435afef442cbfd20d9891907e8fa373ba7d351f320;

    /// @notice Current message version identifier.
    uint16 public constant messageVersion = uint16(0);

    /// @notice Semantic version.
    /// @custom:semver 1.0.0-beta.10
    string public constant version = "1.0.0-beta.10";

    /// @notice Mapping of message hashes to boolean receipt values. Note that a message will only be present in this
    ///         mapping if it has successfully been relayed on this chain, and can therefore not be relayed again.
    mapping(bytes32 => bool) public successfulMessages;

    /// @notice Nonce for the next message to be sent, without the message version applied. Use the messageNonce getter,
    ///         which will insert the message version into the nonce to give you the actual nonce to be used for the
    ///         message.
    uint240 internal msgNonce;

    /// @notice Emitted whenever a message is sent to a destination
    /// @param destination  Chain ID of the destination chain.
    /// @param target       Target contract or wallet address.
    /// @param messageNonce Nonce associated with the messsage sent
    /// @param sender       Address initiating this message call
    /// @param message      Message payload to call target with.
    event SentMessage(
        uint256 indexed destination, address indexed target, uint256 indexed messageNonce, address sender, bytes message
    );

    /// @notice Emitted whenever a message is successfully relayed on this chain.
    /// @param source       Chain ID of the source chain.
    /// @param messageNonce Nonce associated with the messsage sent
    /// @param messageHash  Hash of the message that was relayed.
    event RelayedMessage(uint256 indexed source, uint256 indexed messageNonce, bytes32 indexed messageHash);

    /// @notice Retrieves the sender of the current cross domain message. If not entered, reverts.
    /// @return sender_ Address of the sender of the current cross domain message.
    function crossDomainMessageSender() external view onlyEntered returns (address sender_) {
        assembly {
            sender_ := tload(CROSS_DOMAIN_MESSAGE_SENDER_SLOT)
        }
    }

    /// @notice Retrieves the source of the current cross domain message. If not entered, reverts.
    /// @return source_ Chain ID of the source of the current cross domain message.
    function crossDomainMessageSource() external view onlyEntered returns (uint256 source_) {
        assembly {
            source_ := tload(CROSS_DOMAIN_MESSAGE_SOURCE_SLOT)
        }
    }

    /// @notice Retrieves the context of the current cross domain message. If not entered, reverts.
    /// @return sender_ Address of the sender of the current cross domain message.
    /// @return source_ Chain ID of the source of the current cross domain message.
    function crossDomainMessageContext() external view onlyEntered returns (address sender_, uint256 source_) {
        assembly {
            sender_ := tload(CROSS_DOMAIN_MESSAGE_SENDER_SLOT)
            source_ := tload(CROSS_DOMAIN_MESSAGE_SOURCE_SLOT)
        }
    }

    /// @notice Sends a message to some target address on a destination chain. Note that if the call always reverts,
    ///         then the message will be unrelayable and any ETH sent will be permanently locked. The same will occur
    ///         if the target on the other chain is considered unsafe (see the _isUnsafeTarget() function).
    /// @param _destination Chain ID of the destination chain.
    /// @param _target      Target contract or wallet address.
    /// @param _message     Message payload to call target with.
    /// @return The hash of the message being sent, used to track whether the message has successfully been relayed.
    function sendMessage(uint256 _destination, address _target, bytes calldata _message) external returns (bytes32) {
        if (_destination == block.chainid) revert MessageDestinationSameChain();
        if (_target == Predeploys.CROSS_L2_INBOX) revert MessageTargetCrossL2Inbox();
        if (_target == Predeploys.L2_TO_L2_CROSS_DOMAIN_MESSENGER) revert MessageTargetL2ToL2CrossDomainMessenger();

        uint256 nonce = messageNonce();
        emit SentMessage(_destination, _target, nonce, msg.sender, _message);

        msgNonce++;

        return Hashing.hashL2toL2CrossDomainMessage({
            _destination: _destination,
            _source: block.chainid,
            _nonce: nonce,
            _sender: msg.sender,
            _target: _target,
            _message: _message
        });
    }

    /// @notice Relays a message that was sent by the other L2ToL2CrossDomainMessenger contract. Can only be executed
    ///         via cross chain call from the other messenger OR if the message was already received once and is
    ///         currently being replayed.
    /// @param _id          Identifier of the SentMessage event to be relayed
    /// @param _sentMessage Message payload of the `SentMessage` event
    function relayMessage(Identifier calldata _id, bytes calldata _sentMessage) external payable nonReentrant {
        // Ensure the log came from the messenger. Since the log origin is the CDM, there isn't a scenario where
        // this can be invoked from the CrossL2Inbox as the SentMessage log is not calldata for this function
        if (_id.origin != Predeploys.L2_TO_L2_CROSS_DOMAIN_MESSENGER) {
            revert IdOriginNotL2ToL2CrossDomainMessenger();
        }

        // Signal that this is a cross chain call that needs to have the identifier validated
        CrossL2Inbox(Predeploys.CROSS_L2_INBOX).validateMessage(_id, keccak256(_sentMessage));

        // Decode the payload
        (uint256 destination, address target, uint256 nonce, address sender, bytes memory message) =
            _decodeSentMessagePayload(_sentMessage);

        // Assert invariants on the message
        if (destination != block.chainid) revert MessageDestinationNotRelayChain();
        if (target == Predeploys.CROSS_L2_INBOX) revert MessageTargetCrossL2Inbox();
        if (target == Predeploys.L2_TO_L2_CROSS_DOMAIN_MESSENGER) revert MessageTargetL2ToL2CrossDomainMessenger();

        uint256 source = _id.chainId;
        bytes32 messageHash = Hashing.hashL2toL2CrossDomainMessage({
            _destination: destination,
            _source: source,
            _nonce: nonce,
            _sender: sender,
            _target: target,
            _message: message
        });

        if (successfulMessages[messageHash]) {
            revert MessageAlreadyRelayed();
        }

        _storeMessageMetadata(source, sender);

        bool success = SafeCall.call(target, msg.value, message);

        if (!success) {
            revert TargetCallFailed();
        }

        successfulMessages[messageHash] = true;
        emit RelayedMessage(source, nonce, messageHash);

        _storeMessageMetadata(0, address(0));
    }

    /// @notice Retrieves the next message nonce. Message version will be added to the upper two bytes of the message
    ///         nonce. Message version allows us to treat messages as having different structures.
    /// @return Nonce of the next message to be sent, with added message version.
    function messageNonce() public view returns (uint256) {
        return Encoding.encodeVersionedNonce(msgNonce, messageVersion);
    }

    /// @notice Stores message data such as sender and source in transient storage.
    /// @param _source Chain ID of the source chain.
    /// @param _sender Address of the sender of the message.
    function _storeMessageMetadata(uint256 _source, address _sender) internal {
        assembly {
            tstore(CROSS_DOMAIN_MESSAGE_SENDER_SLOT, _sender)
            tstore(CROSS_DOMAIN_MESSAGE_SOURCE_SLOT, _source)
        }
    }

    function _decodeSentMessagePayload(bytes calldata _payload)
        internal
        pure
        returns (uint256 destination_, address target_, uint256 nonce_, address sender_, bytes memory message_)
    {
        // Validate Selector (also reverts if LOG0 with no topics)
        bytes32 selector = abi.decode(_payload[:32], (bytes32));
        if (selector != SENT_MESSAGE_EVENT_SELECTOR) revert EventPayloadNotSentMessage();

        // Topics
        (destination_, target_, nonce_) = abi.decode(_payload[32:128], (uint256, address, uint256));

        // Data
        (sender_, message_) = abi.decode(_payload[128:], (address, bytes));
    }
}
