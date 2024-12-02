// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { Test } from "forge-std/Test.sol";
import { Constants } from "src/libraries/Constants.sol";

contract Constants_Test is Test {
    /// @notice Check EIP1967 related constants.
    function test_eip1967Constants_succeeds() external pure {
        assertEq(
            bytes32(uint256(keccak256("eip1967.proxy.implementation")) - 1), Constants.PROXY_IMPLEMENTATION_ADDRESS
        );
        assertEq(bytes32(uint256(keccak256("eip1967.proxy.admin")) - 1), Constants.PROXY_OWNER_ADDRESS);
    }
}
