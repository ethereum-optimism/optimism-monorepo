// SPDX-License-Identifier: MIT
pragma solidity 0.8.25;

import { FFIInterface } from "test/setup/FFIInterface.sol";
import { Deploy } from "scripts/deploy/Deploy.s.sol";
import { Test } from "forge-std/Test.sol";

/// @title InvariantTest
/// @dev An extension to `Test` that sets up excluded contracts for invariant testing.
contract InvariantTest is Test {
    FFIInterface constant ffi = FFIInterface(address(uint160(uint256(keccak256(abi.encode("optimism.ffi"))))));
    Deploy internal constant deploy = Deploy(address(uint160(uint256(keccak256(abi.encode("optimism.deploy"))))));

    function setUp() public virtual {
        excludeContract(address(ffi));
        excludeContract(address(deploy));
    }
}
