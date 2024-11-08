// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { StdUtils } from "forge-std/StdUtils.sol";
import { Vm } from "forge-std/Vm.sol";
import { IOptimismPortal } from "src/L1/interfaces/IOptimismPortal.sol";
import { IL1CrossDomainMessenger } from "src/L1/interfaces/IL1CrossDomainMessenger.sol";
import { CommonTest } from "test/setup/CommonTest.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";
import { Constants } from "src/libraries/Constants.sol";
import { Encoding } from "src/libraries/Encoding.sol";
import { Hashing } from "src/libraries/Hashing.sol";

contract RelayActor is StdUtils {
    // Storage slot of the l2Sender
    uint256 constant senderSlotIndex = 50;

    uint256 public numHashes;
    bytes32[] public hashes;
    bool public reverted = false;

    IOptimismPortal op;
    IL1CrossDomainMessenger xdm;
    Vm vm;
    bool doFail;

    constructor(IOptimismPortal _op, IL1CrossDomainMessenger _xdm, Vm _vm, bool _doFail) {
        op = _op;
        xdm = _xdm;
        vm = _vm;
        doFail = _doFail;
    }

    /// @notice Relays a message to the `L1CrossDomainMessenger` with a random `version`,
    ///         and `_message`.
    function relay(uint8 _version, uint8 _value, bytes memory _message) external {
        address target = address(0x04); // ID precompile
        address sender = Predeploys.L2_CROSS_DOMAIN_MESSENGER;

        // Set the minimum gas limit to the cost of the identity precompile's execution for
        // the given message.
        // ID Precompile cost can be determined by calculating: 15 + 3 * data_word_length
        uint32 minGasLimit = uint32(15 + 3 * ((_message.length + 31) / 32));

        // set the value of op.l2Sender() to be the L2 Cross Domain Messenger.
        vm.store(address(op), bytes32(senderSlotIndex), bytes32(abi.encode(sender)));

        // Restrict version to the range of [0, 1]
        _version = _version % 2;

        // Restrict the value to the range of [0, 1]
        // This is just so we get variance of calls with and without value. The ID precompile
        // will not reject value being sent to it.
        _value = _value % 2;

        // If the message should succeed, supply it `baseGas`. If not, supply it an amount of
        // gas that is too low to complete the call.
        uint256 gas = doFail ? bound(minGasLimit, 60_000, 80_000) : xdm.baseGas(_message, minGasLimit);

        // Compute the cross domain message hash and store it in `hashes`.
        // The `relayMessage` function will always encode the message as a version 1
        // message after checking that the V0 hash has not already been relayed.
        bytes32 _hash = Hashing.hashCrossDomainMessageV1(
            Encoding.encodeVersionedNonce(0, _version), sender, target, _value, minGasLimit, _message
        );
        hashes.push(_hash);
        numHashes += 1;

        // Make sure we've got a fresh message.
        vm.assume(xdm.successfulMessages(_hash) == false && xdm.failedMessages(_hash) == false);

        // Act as the optimism portal and call `relayMessage` on the `L1CrossDomainMessenger` with
        // the outer min gas limit.
        vm.startPrank(address(op));
        if (!doFail) {
            vm.expectCallMinGas(address(0x04), _value, minGasLimit, _message);
        }
        try xdm.relayMessage{ gas: gas, value: _value }(
            Encoding.encodeVersionedNonce(0, _version), sender, target, _value, minGasLimit, _message
        ) { } catch {
            // If any of these calls revert, set `reverted` to true to fail the invariant test.
            // NOTE: This is to get around forge's invariant fuzzer ignoring reverted calls
            // to this function.
            reverted = true;
        }
        vm.stopPrank();
    }
}

contract XDM_MinGasLimits is CommonTest {
    RelayActor actor;

    function init(bool doFail) public virtual {
        // Set up the `L1CrossDomainMessenger` and `OptimismPortal` contracts.
        super.setUp();

        // Deploy a relay actor
        actor = new RelayActor(optimismPortal, l1CrossDomainMessenger, vm, doFail);

        // Give the portal some ether to send to `relayMessage`
        vm.deal(address(optimismPortal), type(uint128).max);

        // Target the `RelayActor` contract
        targetContract(address(actor));

        // Don't allow the estimation address to be the sender
        excludeSender(Constants.ESTIMATION_ADDRESS);

        // Don't allow the predeploys to be the senders
        uint160 prefix = uint160(0x420) << 148;
        for (uint256 i = 0; i < 2048; i++) {
            address addr = address(prefix | uint160(i));
            excludeContract(addr);
        }

        // Target the actor's `relay` function
        bytes4[] memory selectors = new bytes4[](1);
        selectors[0] = actor.relay.selector;
        targetSelector(FuzzSelector({ addr: address(actor), selectors: selectors }));
    }
}

contract XDM_MinGasLimits_Succeeds is XDM_MinGasLimits {
    function setUp() public override {
        // Don't fail
        super.init(false);
    }

    /// @custom:invariant A call to `relayMessage` should succeed if at least the minimum gas limit
    ///                   can be supplied to the target context, there is enough gas to complete
    ///                   execution of `relayMessage` after the target context's execution is
    ///                   finished, and the target context did not revert.
    ///
    ///                   There are two minimum gas limits here:
    ///
    ///                   - The outer min gas limit is for the call from the `OptimismPortal` to the
    ///                     `L1CrossDomainMessenger`,  and it can be retrieved by calling the xdm's
    ///                     `baseGas` function with the `message` and inner limit.
    ///
    ///                   - The inner min gas limit is for the call from the
    ///                     `L1CrossDomainMessenger` to the target contract.
    function invariant_minGasLimits() external view {
        uint256 length = actor.numHashes();
        for (uint256 i = 0; i < length; ++i) {
            bytes32 hash = actor.hashes(i);
            // The message hash is set in the successfulMessages mapping
            assertTrue(l1CrossDomainMessenger.successfulMessages(hash));
            // The message hash is not set in the failedMessages mapping
            assertFalse(l1CrossDomainMessenger.failedMessages(hash));
        }
        assertFalse(actor.reverted());
    }
}

contract XDM_MinGasLimits_Reverts is XDM_MinGasLimits {
    function setUp() public override {
        // Do fail
        super.init(true);
    }

    /// @custom:invariant A call to `relayMessage` should assign the message hash to the
    ///                   `failedMessages` mapping if not enough gas is supplied to forward
    ///                   `minGasLimit` to the target context or if there is not enough gas to
    ///                   complete execution of `relayMessage` after the target context's execution
    ///                   is finished.
    ///
    ///                   There are two minimum gas limits here:
    ///
    ///                   - The outer min gas limit is for the call from the `OptimismPortal` to the
    ///                     `L1CrossDomainMessenger`,  and it can be retrieved by calling the xdm's
    ///                     `baseGas` function with the `message` and inner limit.
    ///
    ///                   - The inner min gas limit is for the call from the
    ///                     `L1CrossDomainMessenger` to the target contract.
    function invariant_minGasLimits() external view {
        uint256 length = actor.numHashes();
        for (uint256 i = 0; i < length; ++i) {
            bytes32 hash = actor.hashes(i);
            // The message hash is not set in the successfulMessages mapping
            assertFalse(l1CrossDomainMessenger.successfulMessages(hash));
            // The message hash is set in the failedMessages mapping
            assertTrue(l1CrossDomainMessenger.failedMessages(hash));
        }
        assertFalse(actor.reverted());
    }
}
