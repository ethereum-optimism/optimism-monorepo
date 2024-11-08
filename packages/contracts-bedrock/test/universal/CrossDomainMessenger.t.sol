// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing utilities
import { Test } from "forge-std/Test.sol";
import { CommonTest } from "test/setup/CommonTest.sol";

// Libraries
import { Predeploys } from "src/libraries/Predeploys.sol";
import { Hashing } from "src/libraries/Hashing.sol";
import { Encoding } from "src/libraries/Encoding.sol";

import { IL1CrossDomainMessenger } from "src/L1/interfaces/IL1CrossDomainMessenger.sol";

// CrossDomainMessenger_Test is for testing functionality which is common to both the L1 and L2
// CrossDomainMessenger contracts. For simplicity, we use the L1 Messenger as the test contract.
contract CrossDomainMessenger_BaseGas_Test is CommonTest {
    /// @dev Ensure that baseGas passes for the max value of _minGasLimit,
    ///      this is about 4 Billion.
    function test_baseGas_succeeds() external view {
        l1CrossDomainMessenger.baseGas(hex"ff", type(uint32).max);
    }

    /// @dev Fuzz for other values which might cause a revert in baseGas.
    function testFuzz_baseGas_succeeds(uint32 _minGasLimit) external view {
        l1CrossDomainMessenger.baseGas(hex"ff", _minGasLimit);
    }

    /// @notice The baseGas function should always return a value greater than
    ///         or equal to the minimum gas limit value on the OptimismPortal.
    ///         This guarantees that the messengers will always pass sufficient
    ///         gas to the OptimismPortal.
    function testFuzz_baseGas_portalMinGasLimit_succeeds(bytes memory _data, uint32 _minGasLimit) external view {
        vm.assume(_data.length <= type(uint64).max);
        uint64 baseGas = l1CrossDomainMessenger.baseGas(_data, _minGasLimit);
        uint64 minGasLimit = optimismPortal.minimumGasLimit(uint64(_data.length));
        assertTrue(baseGas >= minGasLimit);
    }
}

/// @title ExternalRelay
/// @notice A mock external contract called via the SafeCall inside
///         the CrossDomainMessenger's `relayMessage` function.
contract ExternalRelay is Test {
    address internal op;
    address internal fuzzedSender;
    IL1CrossDomainMessenger internal l1CrossDomainMessenger;

    event FailedRelayedMessage(bytes32 indexed msgHash);

    constructor(IL1CrossDomainMessenger _l1Messenger, address _op) {
        l1CrossDomainMessenger = _l1Messenger;
        op = _op;
    }

    /// @notice Internal helper function to relay a message and perform assertions.
    function _internalRelay(address _innerSender) internal {
        address initialSender = l1CrossDomainMessenger.xDomainMessageSender();

        bytes memory callMessage = getCallData();

        bytes32 hash = Hashing.hashCrossDomainMessage({
            _nonce: Encoding.encodeVersionedNonce({ _nonce: 0, _version: 1 }),
            _sender: _innerSender,
            _target: address(this),
            _value: 0,
            _gasLimit: 0,
            _data: callMessage
        });

        vm.expectEmit(true, true, true, true);
        emit FailedRelayedMessage(hash);

        vm.prank(address(op));
        l1CrossDomainMessenger.relayMessage({
            _nonce: Encoding.encodeVersionedNonce({ _nonce: 0, _version: 1 }),
            _sender: _innerSender,
            _target: address(this),
            _value: 0,
            _minGasLimit: 0,
            _message: callMessage
        });

        assertTrue(l1CrossDomainMessenger.failedMessages(hash));
        assertFalse(l1CrossDomainMessenger.successfulMessages(hash));
        assertEq(initialSender, l1CrossDomainMessenger.xDomainMessageSender());
    }

    /// @notice externalCallWithMinGas is called by the CrossDomainMessenger.
    function externalCallWithMinGas() external payable {
        for (uint256 i = 0; i < 10; i++) {
            address _innerSender;
            unchecked {
                _innerSender = address(uint160(uint256(uint160(fuzzedSender)) + i));
            }
            _internalRelay(_innerSender);
        }
    }

    /// @notice Helper function to get the callData for an `externalCallWithMinGas
    function getCallData() public pure returns (bytes memory) {
        return abi.encodeCall(ExternalRelay.externalCallWithMinGas, ());
    }

    /// @notice Helper function to set the fuzzed sender
    function setFuzzedSender(address _fuzzedSender) public {
        fuzzedSender = _fuzzedSender;
    }
}

/// @title CrossDomainMessenger_RelayMessage_Test
/// @notice Fuzz tests re-entrancy into the CrossDomainMessenger relayMessage function.
contract CrossDomainMessenger_RelayMessage_Test is CommonTest {
    // Storage slot of the l2Sender
    uint256 constant senderSlotIndex = 50;

    ExternalRelay public er;

    function setUp() public override {
        super.setUp();
        er = new ExternalRelay(l1CrossDomainMessenger, address(optimismPortal));
    }

    /// @dev This test mocks an OptimismPortal call to the L1CrossDomainMessenger via
    ///      the relayMessage function. The relayMessage function will then use SafeCall's
    ///      callWithMinGas to call the target with call data packed in the callMessage.
    ///      For this test, the callWithMinGas will call the mock ExternalRelay test contract
    ///      defined above, executing the externalCallWithMinGas function which will try to
    ///      re-enter the CrossDomainMessenger's relayMessage function, resulting in that message
    ///      being recorded as failed.
    function testFuzz_relayMessageReenter_succeeds(address _sender, uint256 _gasLimit) external {
        vm.assume(_sender != Predeploys.L2_CROSS_DOMAIN_MESSENGER);
        address sender = Predeploys.L2_CROSS_DOMAIN_MESSENGER;

        er.setFuzzedSender(_sender);
        address target = address(er);
        bytes memory callMessage = er.getCallData();

        vm.expectCall(target, callMessage);

        uint64 gasLimit = uint64(bound(_gasLimit, 0, 30_000_000));

        bytes32 hash = Hashing.hashCrossDomainMessage({
            _nonce: Encoding.encodeVersionedNonce({ _nonce: 0, _version: 1 }),
            _sender: sender,
            _target: target,
            _value: 0,
            _gasLimit: gasLimit,
            _data: callMessage
        });

        // set the value of op.l2Sender() to be the L2 Cross Domain Messenger.
        vm.store(address(optimismPortal), bytes32(senderSlotIndex), bytes32(abi.encode(sender)));
        vm.prank(address(optimismPortal));
        l1CrossDomainMessenger.relayMessage({
            _nonce: Encoding.encodeVersionedNonce({ _nonce: 0, _version: 1 }),
            _sender: sender,
            _target: target,
            _value: 0,
            _minGasLimit: gasLimit,
            _message: callMessage
        });

        assertTrue(l1CrossDomainMessenger.successfulMessages(hash));
        assertEq(l1CrossDomainMessenger.failedMessages(hash), false);

        // Ensures that the `xDomainMsgSender` is set back to `Predeploys.L2_CROSS_DOMAIN_MESSENGER`
        vm.expectRevert("CrossDomainMessenger: xDomainMessageSender is not set");
        l1CrossDomainMessenger.xDomainMessageSender();
    }
}
