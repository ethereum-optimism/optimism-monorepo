// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing utilities
import { VmSafe } from "forge-std/Vm.sol";
import { Test } from "forge-std/Test.sol";
import { CommonTest } from "test/setup/CommonTest.sol";

// Libraries
import { Bytes32AddressLib } from "@rari-capital/solmate/src/utils/Bytes32AddressLib.sol";

// Target contract dependencies
import { AddressAliasHelper } from "src/vendor/AddressAliasHelper.sol";

// Target contract
import { CrossDomainOwnable } from "src/L2/CrossDomainOwnable.sol";

contract XDomainSetter is CrossDomainOwnable {
    uint256 public value;

    function set(uint256 _value) external onlyOwner {
        value = _value;
    }
}

contract CrossDomainOwnable_Test is Test {
    XDomainSetter setter;

    function setUp() public {
        setter = new XDomainSetter();
    }

    /// @dev Tests that the `onlyOwner` modifier reverts with the correct message.
    function test_onlyOwner_notOwner_reverts() external {
        vm.expectRevert("CrossDomainOwnable: caller is not the owner");
        setter.set(1);
    }

    /// @dev Tests that the `onlyOwner` modifier succeeds when called by the owner.
    function test_onlyOwner_succeeds() external {
        assertEq(setter.value(), 0);

        vm.prank(AddressAliasHelper.applyL1ToL2Alias(setter.owner()));
        setter.set(1);
        assertEq(setter.value(), 1);
    }
}

contract CrossDomainOwnableThroughPortal_Test is CommonTest {
    XDomainSetter setter;

    /// @dev Sets up the test suite.
    function setUp() public override {
        super.setUp();

        vm.prank(alice);
        setter = new XDomainSetter();
    }

    /// @dev Tests that `depositTransaction` succeeds when calling the `set` function on the
    ///      `XDomainSetter` contract.
    function test_depositTransaction_crossDomainOwner_succeeds() external {
        vm.recordLogs();

        vm.prank(alice);
        optimismPortal.depositTransaction({
            _to: address(setter),
            _value: 0,
            _gasLimit: 30_000,
            _isCreation: false,
            _data: abi.encodeCall(XDomainSetter.set, (1))
        });

        // Simulate the operation of the `op-node` by parsing data
        // from logs
        VmSafe.Log[] memory logs = vm.getRecordedLogs();
        // Only 1 log emitted
        assertEq(logs.length, 1);

        VmSafe.Log memory log = logs[0];

        // It is the expected topic
        bytes32 topic = log.topics[0];
        assertEq(topic, keccak256("TransactionDeposited(address,address,uint256,bytes)"));

        // from is indexed and the first argument to the event.
        bytes32 _from = log.topics[1];
        address from = Bytes32AddressLib.fromLast20Bytes(_from);

        assertEq(AddressAliasHelper.undoL1ToL2Alias(from), alice);

        // Make a call from the "from" value received from the log.
        // In theory the opaque data could be parsed from the log
        // and passed to a low level call to "to", but calling set
        // directly on the setter is good enough.
        vm.prank(from);
        setter.set(1);
        assertEq(setter.value(), 1);
    }
}
