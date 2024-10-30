// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing utilities
import { Test } from "forge-std/Test.sol";

// Target contract
import { IDeployerWhitelist } from "src/legacy/interfaces/IDeployerWhitelist.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

contract DeployerWhitelist_Test is Test {
    IDeployerWhitelist list;

    /// @dev Sets up the test suite.
    function setUp() public {
        list = IDeployerWhitelist(
            DeployUtils.create1({
                _name: "DeployerWhitelist",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IDeployerWhitelist.__constructor__, ()))
            })
        );
    }

    /// @dev Tests that `owner` is initialized to the zero address.
    function test_owner_succeeds() external view {
        assertEq(list.owner(), address(0));
    }

    /// @dev Tests that `setOwner` correctly sets the contract owner.
    function test_storageSlots_succeeds() external {
        vm.prank(list.owner());
        list.setOwner(address(1));

        assertEq(bytes32(uint256(1)), vm.load(address(list), bytes32(uint256(0))));
    }
}
