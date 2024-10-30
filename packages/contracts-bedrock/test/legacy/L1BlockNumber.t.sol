// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { Test } from "forge-std/Test.sol";

// Contracts
import { IL1BlockNumber } from "src/legacy/interfaces/IL1BlockNumber.sol";
import { L1Block } from "src/L2/L1Block.sol";

// Libraries
import { Predeploys } from "src/libraries/Predeploys.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

contract L1BlockNumberTest is Test {
    L1Block lb;
    IL1BlockNumber bn;

    uint64 constant number = 99;

    /// @dev Sets up the test suite.
    function setUp() external {
        vm.etch(Predeploys.L1_BLOCK_ATTRIBUTES, vm.getDeployedCode("L1Block.sol:L1Block"));
        lb = L1Block(Predeploys.L1_BLOCK_ATTRIBUTES);
        bn = IL1BlockNumber(
            DeployUtils.create1({
                _name: "L1BlockNumber",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IL1BlockNumber.__constructor__, ()))
            })
        );
        vm.prank(lb.DEPOSITOR_ACCOUNT());

        lb.setL1BlockValues({
            _number: number,
            _timestamp: uint64(2),
            _basefee: 3,
            _hash: bytes32(uint256(10)),
            _sequenceNumber: uint64(4),
            _batcherHash: bytes32(uint256(0)),
            _l1FeeOverhead: 2,
            _l1FeeScalar: 3
        });
    }

    /// @dev Tests that `getL1BlockNumber` returns the set block number.
    function test_getL1BlockNumber_succeeds() external view {
        assertEq(bn.getL1BlockNumber(), number);
    }

    /// @dev Tests that `fallback` is correctly dispatched.
    function test_fallback_succeeds() external {
        (bool success, bytes memory ret) = address(bn).call(hex"");
        assertEq(success, true);
        assertEq(ret, abi.encode(number));
    }

    /// @dev Tests that `receive` is correctly dispatched.
    function test_receive_succeeds() external {
        (bool success, bytes memory ret) = address(bn).call{ value: 1 }(hex"");
        assertEq(success, true);
        assertEq(ret, abi.encode(number));
    }
}
