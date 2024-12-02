// SPDX-License-Identifier: MIT
pragma solidity 0.8.25;

// Testing utilities
import { CommonTest } from "test/setup/CommonTest.sol";

// Libraries
import { Types } from "src/libraries/Types.sol";
import { Hashing } from "src/libraries/Hashing.sol";

contract L2ToL1MessagePasserTest is CommonTest {
    /// @dev Tests that `initiateWithdrawal` succeeds and correctly sets the state
    ///      of the message passer for the withdrawal hash.
    function testFuzz_initiateWithdrawal_succeeds(
        address _sender,
        address _target,
        uint256 _value,
        uint256 _gasLimit,
        bytes memory _data
    )
        external
    {
        uint256 nonce = l2ToL1MessagePasser.messageNonce();

        bytes32 withdrawalHash = Hashing.hashWithdrawal(
            Types.WithdrawalTransaction({
                nonce: nonce,
                sender: _sender,
                target: _target,
                value: _value,
                gasLimit: _gasLimit,
                data: _data
            })
        );

        vm.expectEmit(address(l2ToL1MessagePasser));
        emit MessagePassed(nonce, _sender, _target, _value, _gasLimit, _data, withdrawalHash);

        vm.deal(_sender, _value);
        vm.prank(_sender);
        l2ToL1MessagePasser.initiateWithdrawal{ value: _value }(_target, _gasLimit, _data);

        assertEq(l2ToL1MessagePasser.sentMessages(withdrawalHash), true);

        bytes32 slot = keccak256(bytes.concat(withdrawalHash, bytes32(0)));

        assertEq(vm.load(address(l2ToL1MessagePasser), slot), bytes32(uint256(1)));
    }

    /// @dev Tests that `initiateWithdrawal` succeeds and emits the correct MessagePassed
    ///      log when called by a contract.
    function testFuzz_initiateWithdrawal_fromContract_succeeds(
        address _target,
        uint256 _gasLimit,
        uint256 _value,
        bytes memory _data
    )
        external
    {
        bytes32 withdrawalHash = Hashing.hashWithdrawal(
            Types.WithdrawalTransaction({
                nonce: l2ToL1MessagePasser.messageNonce(),
                sender: address(this),
                target: _target,
                value: _value,
                gasLimit: _gasLimit,
                data: _data
            })
        );

        vm.expectEmit(address(l2ToL1MessagePasser));
        emit MessagePassed(
            l2ToL1MessagePasser.messageNonce(), address(this), _target, _value, _gasLimit, _data, withdrawalHash
        );

        vm.deal(address(this), _value);
        l2ToL1MessagePasser.initiateWithdrawal{ value: _value }(_target, _gasLimit, _data);
    }

    /// @dev Tests that `initiateWithdrawal` succeeds and emits the correct MessagePassed
    ///      log when called by an EOA.
    function testFuzz_initiateWithdrawal_fromEOA_succeeds(
        uint256 _gasLimit,
        address _target,
        uint256 _value,
        bytes memory _data
    )
        external
    {
        uint256 nonce = l2ToL1MessagePasser.messageNonce();

        // EOA emulation
        vm.prank(alice, alice);
        vm.deal(alice, _value);
        bytes32 withdrawalHash =
            Hashing.hashWithdrawal(Types.WithdrawalTransaction(nonce, alice, _target, _value, _gasLimit, _data));

        vm.expectEmit(address(l2ToL1MessagePasser));
        emit MessagePassed(nonce, alice, _target, _value, _gasLimit, _data, withdrawalHash);

        l2ToL1MessagePasser.initiateWithdrawal{ value: _value }({ _target: _target, _gasLimit: _gasLimit, _data: _data });

        // the sent messages mapping is filled
        assertEq(l2ToL1MessagePasser.sentMessages(withdrawalHash), true);
        // the nonce increments
        assertEq(nonce + 1, l2ToL1MessagePasser.messageNonce());
    }

    /// @dev Tests that `burn` succeeds and destroys the ETH held in the contract.
    function testFuzz_burn_succeeds(uint256 _value, address _target, uint256 _gasLimit, bytes memory _data) external {
        vm.deal(address(this), _value);

        l2ToL1MessagePasser.initiateWithdrawal{ value: _value }({ _target: _target, _gasLimit: _gasLimit, _data: _data });

        assertEq(address(l2ToL1MessagePasser).balance, _value);
        emit WithdrawerBalanceBurnt(_value);
        l2ToL1MessagePasser.burn();

        // The Withdrawer should have no balance
        assertEq(address(l2ToL1MessagePasser).balance, 0);
    }
}
