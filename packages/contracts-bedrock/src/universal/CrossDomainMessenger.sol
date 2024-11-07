// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Initializable } from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import { SafeCall } from "src/libraries/SafeCall.sol";
import { Hashing } from "src/libraries/Hashing.sol";
import { Encoding } from "src/libraries/Encoding.sol";
import { Constants } from "src/libraries/Constants.sol";

/// @custom:legacy
/// @title CrossDomainMessengerLegacySpacer0
/// @notice Contract only exists to add a spacer to the CrossDomainMessenger where the
///         libAddressManager variable used to exist. Must be the first contract in the inheritance
///         tree of the CrossDomainMessenger.
contract CrossDomainMessengerLegacySpacer0 {
    /// @custom:legacy
    /// @custom:spacer libAddressManager
    /// @notice Spacer for backwards compatibility.
    address private spacer_0_0_20;
}

/// @custom:legacy
/// @title CrossDomainMessengerLegacySpacer1
/// @notice Contract only exists to add a spacer to the CrossDomainMessenger where the
///         PausableUpgradable and OwnableUpgradeable variables used to exist. Must be
///         the third contract in the inheritance tree of the CrossDomainMessenger.
contract CrossDomainMessengerLegacySpacer1 {
    /// @custom:legacy
    /// @custom:spacer ContextUpgradable's __gap
    /// @notice Spacer for backwards compatibility. Comes from OpenZeppelin
    ///         ContextUpgradable.
    uint256[50] private spacer_1_0_1600;

    /// @custom:legacy
    /// @custom:spacer OwnableUpgradeable's _owner
    /// @notice Spacer for backwards compatibility.
    ///         Come from OpenZeppelin OwnableUpgradeable.
    address private spacer_51_0_20;

    /// @custom:legacy
    /// @custom:spacer OwnableUpgradeable's __gap
    /// @notice Spacer for backwards compatibility. Comes from OpenZeppelin
    ///         OwnableUpgradeable.
    uint256[49] private spacer_52_0_1568;

    /// @custom:legacy
    /// @custom:spacer PausableUpgradable's _paused
    /// @notice Spacer for backwards compatibility. Comes from OpenZeppelin
    ///         PausableUpgradable.
    bool private spacer_101_0_1;

    /// @custom:legacy
    /// @custom:spacer PausableUpgradable's __gap
    /// @notice Spacer for backwards compatibility. Comes from OpenZeppelin
    ///         PausableUpgradable.
    uint256[49] private spacer_102_0_1568;

    /// @custom:legacy
    /// @custom:spacer ReentrancyGuardUpgradeable's `_status` field.
    /// @notice Spacer for backwards compatibility.
    uint256 private spacer_151_0_32;

    /// @custom:legacy
    /// @custom:spacer ReentrancyGuardUpgradeable's __gap
    /// @notice Spacer for backwards compatibility.
    uint256[49] private spacer_152_0_1568;

    /// @custom:legacy
    /// @custom:spacer blockedMessages
    /// @notice Spacer for backwards compatibility.
    mapping(bytes32 => bool) private spacer_201_0_32;

    /// @custom:legacy
    /// @custom:spacer relayedMessages
    /// @notice Spacer for backwards compatibility.
    mapping(bytes32 => bool) private spacer_202_0_32;
}

/// @custom:upgradeable
/// @title CrossDomainMessenger
/// @notice CrossDomainMessenger is a base contract that provides the core logic for the L1 and L2
///         cross-chain messenger contracts. It's designed to be a universal interface that only
///         needs to be extended slightly to provide low-level message passing functionality on each
///         chain it's deployed on. Currently only designed for message passing between two paired
///         chains and does not support one-to-many interactions.
///         Any changes to this contract MUST result in a semver bump for contracts that inherit it.
abstract contract CrossDomainMessenger is
    CrossDomainMessengerLegacySpacer0,
    Initializable,
    CrossDomainMessengerLegacySpacer1
{
    /// @notice Current message version identifier.
    uint16 public constant MESSAGE_VERSION = 1;

    /// @notice Constant overhead added to the base gas for a message.
    uint64 public constant RELAY_CONSTANT_OVERHEAD = 200_000;

    /// @notice Numerator for dynamic overhead added to the base gas for a message.
    uint64 public constant MIN_GAS_DYNAMIC_OVERHEAD_NUMERATOR = 64;

    /// @notice Denominator for dynamic overhead added to the base gas for a message.
    uint64 public constant MIN_GAS_DYNAMIC_OVERHEAD_DENOMINATOR = 63;

    /// @notice Extra gas added to base gas for each byte of calldata in a message.
    uint64 public constant MIN_GAS_CALLDATA_OVERHEAD = 16;

    /// @notice Gas reserved for performing the external call in `relayMessage`.
    uint64 public constant RELAY_CALL_OVERHEAD = 40_000;

    /// @notice Gas reserved for finalizing the execution of `relayMessage` after the safe call.
    uint64 public constant RELAY_RESERVED_GAS = 40_000;

    /// @notice Gas reserved for the execution between the `hasMinGas` check and the external
    ///         call in `relayMessage`.
    uint64 public constant RELAY_GAS_CHECK_BUFFER = 5_000;

    /// @notice Mapping of message hashes to boolean receipt values. Note that a message will only
    ///         be present in this mapping if it has successfully been relayed on this chain, and
    ///         can therefore not be relayed again.
    mapping(bytes32 => bool) public successfulMessages;

    /// @notice Address of the sender of the currently executing message on the other chain. If the
    ///         value of this variable is the default value (0x00000000...dead) then no message is
    ///         currently being executed. Use the xDomainMessageSender getter which will throw an
    ///         error if this is the case.
    address internal xDomainMsgSender;

    /// @notice Nonce for the next message to be sent, without the message version applied. Use the
    ///         messageNonce getter which will insert the message version into the nonce to give you
    ///         the actual nonce to be used for the message.
    uint240 internal msgNonce;

    /// @notice Mapping of message hashes to a boolean if and only if the message has failed to be
    ///         executed at least once. A message will not be present in this mapping if it
    ///         successfully executed on the first attempt.
    mapping(bytes32 => bool) public failedMessages;

    /// @notice CrossDomainMessenger contract on the other chain.
    /// @custom:network-specific
    CrossDomainMessenger public otherMessenger;

    /// @notice Reserve extra slots in the storage layout for future upgrades.
    ///         A gap size of 43 was chosen here, so that the first slot used in a child contract
    ///         would be 1 plus a multiple of 50.
    uint256[43] private __gap;

    /// @notice Emitted whenever a message is sent to the other chain.
    /// @param target       Address of the recipient of the message.
    /// @param sender       Address of the sender of the message.
    /// @param message      Message to trigger the recipient address with.
    /// @param messageNonce Unique nonce attached to the message.
    /// @param gasLimit     Minimum gas limit that the message can be executed with.
    event SentMessage(address indexed target, address sender, bytes message, uint256 messageNonce, uint256 gasLimit);

    /// @notice Additional event data to emit, required as of Bedrock. Cannot be merged with the
    ///         SentMessage event without breaking the ABI of this contract, this is good enough.
    /// @param sender Address of the sender of the message.
    /// @param value  ETH value sent along with the message to the recipient.
    event SentMessageExtension1(address indexed sender, uint256 value);

    /// @notice Emitted whenever a message is successfully relayed on this chain.
    /// @param msgHash Hash of the message that was relayed.
    event RelayedMessage(bytes32 indexed msgHash);

    /// @notice Emitted whenever a message fails to be relayed on this chain.
    /// @param msgHash Hash of the message that failed to be relayed.
    event FailedRelayedMessage(bytes32 indexed msgHash);

    /// @notice Sends a message to some target address on the other chain. Note that if the call
    ///         always reverts, then the message will be unrelayable, and any ETH sent will be
    ///         permanently locked. The same will occur if the target on the other chain is
    ///         considered unsafe (see the _isUnsafeTarget() function).
    /// @param _target      Target contract or wallet address.
    /// @param _message     Message to trigger the target address with.
    /// @param _minGasLimit Minimum gas limit that the message can be executed with.
    function sendMessage(address _target, bytes calldata _message, uint32 _minGasLimit) external payable {
        if (isCustomGasToken()) {
            require(msg.value == 0, "CrossDomainMessenger: cannot send value with custom gas token");
        }

        // Triggers a message to the other messenger. Note that the amount of gas provided to the
        // message is the amount of gas requested by the user PLUS the base gas value. We want to
        // guarantee the property that the call to the target contract will always have at least
        // the minimum gas limit specified by the user.
        _sendMessage({
            _to: address(otherMessenger),
            _gasLimit: baseGas(_message, _minGasLimit),
            _value: msg.value,
            _data: abi.encodeWithSelector(
                this.relayMessage.selector, messageNonce(), msg.sender, _target, msg.value, _minGasLimit, _message
            )
        });

        emit SentMessage(_target, msg.sender, _message, messageNonce(), _minGasLimit);
        emit SentMessageExtension1(msg.sender, msg.value);

        unchecked {
            ++msgNonce;
        }
    }

    /// @notice Relays a message that was sent by the other CrossDomainMessenger contract. Can only
    ///         be executed via cross-chain call from the other messenger OR if the message was
    ///         already received once and is currently being replayed.
    /// @param _nonce       Nonce of the message being relayed.
    /// @param _sender      Address of the user who sent the message.
    /// @param _target      Address that the message is targeted at.
    /// @param _value       ETH value to send with the message.
    /// @param _minGasLimit Minimum amount of gas that the message can be executed with.
    /// @param _message     Message to send to the target.
    function relayMessage(
        uint256 _nonce,
        address _sender,
        address _target,
        uint256 _value,
        uint256 _minGasLimit,
        bytes calldata _message
    )
        external
        payable
    {
        // On L1 this function will check the Portal for its paused status.
        // On L2 this function should be a no-op, because paused will always return false.
        require(paused() == false, "CrossDomainMessenger: paused");

        (, uint16 version) = Encoding.decodeVersionedNonce(_nonce);
        require(version < 2, "CrossDomainMessenger: only version 0 or 1 messages are supported at this time");

        // If the message is version 0, then it's a migrated legacy withdrawal. We therefore need
        // to check that the legacy version of the message has not already been relayed.
        if (version == 0) {
            bytes32 oldHash = Hashing.hashCrossDomainMessageV0(_target, _sender, _message, _nonce);
            require(successfulMessages[oldHash] == false, "CrossDomainMessenger: legacy withdrawal already relayed");
        }

        // We use the v1 message hash as the unique identifier for the message because it commits
        // to the value and minimum gas limit of the message.
        bytes32 versionedHash =
            Hashing.hashCrossDomainMessageV1(_nonce, _sender, _target, _value, _minGasLimit, _message);

        if (_isOtherMessenger()) {
            // These properties should always hold when the message is first submitted (as
            // opposed to being replayed).
            assert(msg.value == _value);
            assert(!failedMessages[versionedHash]);
        } else {
            require(msg.value == 0, "CrossDomainMessenger: value must be zero unless message is from a system address");

            require(failedMessages[versionedHash], "CrossDomainMessenger: message cannot be replayed");
        }

        require(
            _isUnsafeTarget(_target) == false, "CrossDomainMessenger: cannot send message to blocked system address"
        );

        require(successfulMessages[versionedHash] == false, "CrossDomainMessenger: message has already been relayed");

        // If there is not enough gas left to perform the external call and finish the execution,
        // return early and assign the message to the failedMessages mapping.
        // We are asserting that we have enough gas to:
        // 1. Call the target contract (_minGasLimit + RELAY_CALL_OVERHEAD + RELAY_GAS_CHECK_BUFFER)
        //   1.a. The RELAY_CALL_OVERHEAD is included in `hasMinGas`.
        // 2. Finish the execution after the external call (RELAY_RESERVED_GAS).
        //
        // If `xDomainMsgSender` is not the default L2 sender, this function
        // is being re-entered. This marks the message as failed to allow it to be replayed.
        if (
            !SafeCall.hasMinGas(_minGasLimit, RELAY_RESERVED_GAS + RELAY_GAS_CHECK_BUFFER)
                || xDomainMsgSender != Constants.DEFAULT_L2_SENDER
        ) {
            failedMessages[versionedHash] = true;
            emit FailedRelayedMessage(versionedHash);

            // Revert in this case if the transaction was triggered by the estimation address. This
            // should only be possible during gas estimation or we have bigger problems. Reverting
            // here will make the behavior of gas estimation change such that the gas limit
            // computed will be the amount required to relay the message, even if that amount is
            // greater than the minimum gas limit specified by the user.
            if (tx.origin == Constants.ESTIMATION_ADDRESS) {
                revert("CrossDomainMessenger: failed to relay message");
            }

            return;
        }

        xDomainMsgSender = _sender;
        bool success = SafeCall.call(_target, gasleft() - RELAY_RESERVED_GAS, _value, _message);
        xDomainMsgSender = Constants.DEFAULT_L2_SENDER;

        if (success) {
            // This check is identical to one above, but it ensures that the same message cannot be relayed
            // twice, and adds a layer of protection against rentrancy.
            assert(successfulMessages[versionedHash] == false);
            successfulMessages[versionedHash] = true;
            emit RelayedMessage(versionedHash);
        } else {
            failedMessages[versionedHash] = true;
            emit FailedRelayedMessage(versionedHash);

            // Revert in this case if the transaction was triggered by the estimation address. This
            // should only be possible during gas estimation or we have bigger problems. Reverting
            // here will make the behavior of gas estimation change such that the gas limit
            // computed will be the amount required to relay the message, even if that amount is
            // greater than the minimum gas limit specified by the user.
            if (tx.origin == Constants.ESTIMATION_ADDRESS) {
                revert("CrossDomainMessenger: failed to relay message");
            }
        }
    }

    /// @notice Retrieves the address of the contract or wallet that initiated the currently
    ///         executing message on the other chain. Will throw an error if there is no message
    ///         currently being executed. Allows the recipient of a call to see who triggered it.
    /// @return Address of the sender of the currently executing message on the other chain.
    function xDomainMessageSender() external view returns (address) {
        require(
            xDomainMsgSender != Constants.DEFAULT_L2_SENDER, "CrossDomainMessenger: xDomainMessageSender is not set"
        );

        return xDomainMsgSender;
    }

    /// @notice Retrieves the address of the paired CrossDomainMessenger contract on the other chain
    ///         Public getter is legacy and will be removed in the future. Use `otherMessenger()` instead.
    /// @return CrossDomainMessenger contract on the other chain.
    /// @custom:legacy
    function OTHER_MESSENGER() public view returns (CrossDomainMessenger) {
        return otherMessenger;
    }

    /// @notice Retrieves the next message nonce. Message version will be added to the upper two
    ///         bytes of the message nonce. Message version allows us to treat messages as having
    ///         different structures.
    /// @return Nonce of the next message to be sent, with added message version.
    function messageNonce() public view returns (uint256) {
        return Encoding.encodeVersionedNonce(msgNonce, MESSAGE_VERSION);
    }

    /// @notice Computes the amount of gas required to guarantee that a given message will be
    ///         received on the other chain without running out of gas. Guaranteeing that a message
    ///         will not run out of gas is important because this ensures that a message can always
    ///         be replayed on the other chain if it fails to execute completely.
    /// @param _message     Message to compute the amount of required gas for.
    /// @param _minGasLimit Minimum desired gas limit when message goes to target.
    /// @return Amount of gas required to guarantee message receipt.
    function baseGas(bytes calldata _message, uint32 _minGasLimit) public pure returns (uint64) {
        return
        // Constant overhead
        RELAY_CONSTANT_OVERHEAD
        // Calldata overhead
        + (uint64(_message.length) * MIN_GAS_CALLDATA_OVERHEAD)
        // Dynamic overhead (EIP-150)
        + ((_minGasLimit * MIN_GAS_DYNAMIC_OVERHEAD_NUMERATOR) / MIN_GAS_DYNAMIC_OVERHEAD_DENOMINATOR)
        // Gas reserved for the worst-case cost of 3/5 of the `CALL` opcode's dynamic gas
        // factors. (Conservative)
        + RELAY_CALL_OVERHEAD
        // Relay reserved gas (to ensure execution of `relayMessage` completes after the
        // subcontext finishes executing) (Conservative)
        + RELAY_RESERVED_GAS
        // Gas reserved for the execution between the `hasMinGas` check and the `CALL`
        // opcode. (Conservative)
        + RELAY_GAS_CHECK_BUFFER;
    }

    /// @notice Returns the address of the gas token and the token's decimals.
    function gasPayingToken() internal view virtual returns (address, uint8);

    /// @notice Returns whether the chain uses a custom gas token or not.
    function isCustomGasToken() internal view returns (bool) {
        (address token,) = gasPayingToken();
        return token != Constants.ETHER;
    }

    /// @notice Initializer.
    /// @param _otherMessenger CrossDomainMessenger contract on the other chain.
    function __CrossDomainMessenger_init(CrossDomainMessenger _otherMessenger) internal onlyInitializing {
        // We only want to set the xDomainMsgSender to the default value if it hasn't been initialized yet,
        // meaning that this is a fresh contract deployment.
        // This prevents resetting the xDomainMsgSender to the default value during an upgrade, which would enable
        // a reentrant withdrawal to sandwhich the upgrade replay a withdrawal twice.
        if (xDomainMsgSender == address(0)) {
            xDomainMsgSender = Constants.DEFAULT_L2_SENDER;
        }
        otherMessenger = _otherMessenger;
    }

    /// @notice Sends a low-level message to the other messenger. Needs to be implemented by child
    ///         contracts because the logic for this depends on the network where the messenger is
    ///         being deployed.
    /// @param _to       Recipient of the message on the other chain.
    /// @param _gasLimit Minimum gas limit the message can be executed with.
    /// @param _value    Amount of ETH to send with the message.
    /// @param _data     Message data.
    function _sendMessage(address _to, uint64 _gasLimit, uint256 _value, bytes memory _data) internal virtual;

    /// @notice Checks whether the message is coming from the other messenger. Implemented by child
    ///         contracts because the logic for this depends on the network where the messenger is
    ///         being deployed.
    /// @return Whether the message is coming from the other messenger.
    function _isOtherMessenger() internal view virtual returns (bool);

    /// @notice Checks whether a given call target is a system address that could cause the
    ///         messenger to peform an unsafe action. This is NOT a mechanism for blocking user
    ///         addresses. This is ONLY used to prevent the execution of messages to specific
    ///         system addresses that could cause security issues, e.g., having the
    ///         CrossDomainMessenger send messages to itself.
    /// @param _target Address of the contract to check.
    /// @return Whether or not the address is an unsafe system address.
    function _isUnsafeTarget(address _target) internal view virtual returns (bool);

    /// @notice This function should return true if the contract is paused.
    ///         On L1 this function will check the SuperchainConfig for its paused status.
    ///         On L2 this function should be a no-op.
    /// @return Whether or not the contract is paused.
    function paused() public view virtual returns (bool) {
        return false;
    }
}
