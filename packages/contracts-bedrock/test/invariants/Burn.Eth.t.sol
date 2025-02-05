// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { StdUtils } from "forge-std/StdUtils.sol";
import { Vm } from "forge-std/Vm.sol";

import { StdInvariant } from "forge-std/StdInvariant.sol";
import { Burn } from "src/libraries/Burn.sol";
import { InvariantTest } from "test/invariants/InvariantTest.sol";

contract Burn_EthBurner is StdUtils {
    Vm internal vm;
    bool public failedEthBurn;

    constructor(Vm _vm) {
        vm = _vm;
    }

    /// @notice Takes an integer amount of eth to burn through the Burn library and
    ///         updates the contract state if an incorrect amount of eth moved from the contract
    function burnEth(uint256 _value) external {
        uint256 preBurnvalue = bound(_value, 0, type(uint128).max);

        // Give the burner some ether for gas being used
        vm.deal(address(this), preBurnvalue);

        // cache the contract's eth balance
        uint256 preBurnBalance = address(this).balance;

        uint256 value = bound(preBurnvalue, 0, preBurnBalance);

        // execute a burn of _value eth
        Burn.eth(value);

        // check that exactly value eth was transferred from the contract
        unchecked {
            if (address(this).balance != preBurnBalance - value) {
                failedEthBurn = true;
            }
        }
    }
}

contract Burn_BurnEth_Invariant is StdInvariant, InvariantTest {
    Burn_EthBurner internal actor;

    function setUp() public override {
        super.setUp();
        // Create a Eth burner actor.

        actor = new Burn_EthBurner(vm);

        targetContract(address(actor));

        bytes4[] memory selectors = new bytes4[](1);
        selectors[0] = actor.burnEth.selector;
        FuzzSelector memory selector = FuzzSelector({ addr: address(actor), selectors: selectors });
        targetSelector(selector);
    }

    /// @custom:invariant `eth(uint256)` always burns the exact amount of eth passed.
    ///
    ///                   Asserts that when `Burn.eth(uint256)` is called, it always
    ///                   burns the exact amount of ETH passed to the function.
    function invariant_burn_eth() external view {
        // ASSERTION: The amount burned should always match the amount passed exactly
        assertEq(actor.failedEthBurn(), false);
    }
}
