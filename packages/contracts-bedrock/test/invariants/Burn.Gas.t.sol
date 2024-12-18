// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { StdUtils } from "forge-std/StdUtils.sol";
import { Vm } from "forge-std/Vm.sol";

import { StdInvariant } from "forge-std/StdInvariant.sol";
import { Burn } from "src/libraries/Burn.sol";
import { InvariantTest } from "test/invariants/InvariantTest.sol";

contract Burn_GasBurner is StdUtils {
    Vm internal vm;
    bool public failedGasBurn;

    constructor(Vm _vm) {
        vm = _vm;
    }

    /// @notice Takes an integer amount of gas to burn through the Burn library and
    ///         updates the contract state if at least that amount of gas was not burned
    ///         by the library
    function burnGas(uint256 _value) external {
        // cap the value to the max resource limit
        uint256 MAX_RESOURCE_LIMIT = 8_000_000;
        uint256 value = bound(_value, 0, MAX_RESOURCE_LIMIT);

        // cache the contract's current remaining gas
        uint256 preBurnGas = gasleft();

        // execute the gas burn
        Burn.gas(value);

        // cache the remaining gas post burn
        uint256 postBurnGas = gasleft();

        // check that at least value gas was burnt (and that there was no underflow)
        unchecked {
            if (postBurnGas - preBurnGas <= value && preBurnGas - value > preBurnGas) {
                failedGasBurn = true;
            }
        }
    }
}

contract Burn_BurnGas_Invariant is StdInvariant, InvariantTest {
    Burn_GasBurner internal actor;

    function setUp() public override {
        super.setUp();
        // Create a gas burner actor.
        actor = new Burn_GasBurner(vm);

        targetContract(address(actor));

        bytes4[] memory selectors = new bytes4[](1);
        selectors[0] = actor.burnGas.selector;
        FuzzSelector memory selector = FuzzSelector({ addr: address(actor), selectors: selectors });
        targetSelector(selector);
    }

    /// @custom:invariant `gas(uint256)` always burns at least the amount of gas passed.
    ///
    ///                   Asserts that when `Burn.gas(uint256)` is called, it always burns
    ///                   at least the amount of gas passed to the function.
    function invariant_burn_gas() external view {
        // ASSERTION: The amount burned should always match the amount passed exactly
        assertEq(actor.failedGasBurn(), false);
    }
}
