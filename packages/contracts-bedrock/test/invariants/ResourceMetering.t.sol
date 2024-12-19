// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { StdUtils } from "forge-std/StdUtils.sol";
import { StdInvariant } from "forge-std/StdInvariant.sol";

import { Arithmetic } from "src/libraries/Arithmetic.sol";
import { ResourceMetering } from "src/L1/ResourceMetering.sol";
import { IResourceMetering } from "interfaces/L1/IResourceMetering.sol";
import { Constants } from "src/libraries/Constants.sol";
import { InvariantTest } from "test/invariants/InvariantTest.sol";

contract ResourceMetering_User is StdUtils, ResourceMetering {
    bool public failedMaxGasPerBlock;
    bool public failedRaiseBaseFee;
    bool public failedLowerBaseFee;
    bool public failedNeverBelowMinBaseFee;
    bool public failedMaxRaiseBaseFeePerBlock;
    bool public failedMaxLowerBaseFeePerBlock;

    // Used as a special flag for the purpose of identifying unchecked math errors specifically
    // in the test contracts, not the target contracts themselves.
    bool public underflow;

    constructor() {
        initialize();
    }

    function initialize() internal initializer {
        __ResourceMetering_init();
    }

    function resourceConfig() public pure returns (ResourceMetering.ResourceConfig memory) {
        return _resourceConfig();
    }

    function _resourceConfig() internal pure override returns (ResourceMetering.ResourceConfig memory config_) {
        IResourceMetering.ResourceConfig memory rcfg = Constants.DEFAULT_RESOURCE_CONFIG();
        assembly ("memory-safe") {
            config_ := rcfg
        }
    }

    /// @notice Takes the necessary parameters to allow us to burn arbitrary amounts of gas to test
    ///         the underlying resource metering/gas market logic
    function burn(uint256 _gasToBurn, bool _raiseBaseFee) public {
        // Part 1: we cache the current param values and do some basic checks on them.
        uint256 cachedPrevBaseFee = uint256(params.prevBaseFee);
        uint256 cachedPrevBoughtGas = uint256(params.prevBoughtGas);
        uint256 cachedPrevBlockNum = uint256(params.prevBlockNum);

        ResourceMetering.ResourceConfig memory rcfg = resourceConfig();
        uint256 targetResourceLimit = uint256(rcfg.maxResourceLimit) / uint256(rcfg.elasticityMultiplier);

        // check that the last block's base fee hasn't dropped below the minimum
        if (cachedPrevBaseFee < uint256(rcfg.minimumBaseFee)) {
            failedNeverBelowMinBaseFee = true;
        }
        // check that the last block didn't consume more than the max amount of gas
        if (cachedPrevBoughtGas > uint256(rcfg.maxResourceLimit)) {
            failedMaxGasPerBlock = true;
        }

        // Part2: we perform the gas burn

        // force the gasToBurn into the correct range based on whether we intend to
        // raise or lower the baseFee after this block, respectively
        uint256 gasToBurn;
        if (_raiseBaseFee) {
            gasToBurn = bound(_gasToBurn, uint256(targetResourceLimit), uint256(rcfg.maxResourceLimit));
        } else {
            gasToBurn = bound(_gasToBurn, 0, targetResourceLimit);
        }

        _burnInternal(uint64(gasToBurn));

        // Part 3: we run checks and modify our invariant flags based on the updated params values

        // Calculate the maximum allowed baseFee change (per block)
        uint256 maxBaseFeeChange = cachedPrevBaseFee / uint256(rcfg.baseFeeMaxChangeDenominator);

        // If the last block used more than the target amount of gas (and there were no
        // empty blocks in between), ensure this block's baseFee increased, but not by
        // more than the max amount per block
        if (
            (cachedPrevBoughtGas > uint256(targetResourceLimit))
                && (uint256(params.prevBlockNum) - cachedPrevBlockNum == 1)
        ) {
            failedRaiseBaseFee = failedRaiseBaseFee || (params.prevBaseFee <= cachedPrevBaseFee);
            failedMaxRaiseBaseFeePerBlock =
                failedMaxRaiseBaseFeePerBlock || ((uint256(params.prevBaseFee) - cachedPrevBaseFee) < maxBaseFeeChange);
        }

        // If the last block used less than the target amount of gas, (or was empty),
        // ensure that: this block's baseFee was decreased, but not by more than the max amount
        if (
            (cachedPrevBoughtGas < uint256(targetResourceLimit))
                || (uint256(params.prevBlockNum) - cachedPrevBlockNum > 1)
        ) {
            // Invariant: baseFee should decrease
            failedLowerBaseFee = failedLowerBaseFee || (uint256(params.prevBaseFee) > cachedPrevBaseFee);

            if (params.prevBlockNum - cachedPrevBlockNum == 1) {
                // No empty blocks
                // Invariant: baseFee should not have decreased by more than the maximum amount
                failedMaxLowerBaseFeePerBlock = failedMaxLowerBaseFeePerBlock
                    || ((cachedPrevBaseFee - uint256(params.prevBaseFee)) <= maxBaseFeeChange);
            } else if (params.prevBlockNum - cachedPrevBlockNum > 1) {
                // We have at least one empty block
                // Update the maxBaseFeeChange to account for multiple blocks having passed
                unchecked {
                    maxBaseFeeChange = uint256(
                        int256(cachedPrevBaseFee)
                            - Arithmetic.clamp(
                                Arithmetic.cdexp(
                                    int256(cachedPrevBaseFee),
                                    int256(uint256(rcfg.baseFeeMaxChangeDenominator)),
                                    int256(uint256(params.prevBlockNum) - cachedPrevBlockNum)
                                ),
                                int256(uint256(rcfg.minimumBaseFee)),
                                int256(uint256(rcfg.maximumBaseFee))
                            )
                    );
                }

                // Detect an underflow in the previous calculation.
                // Without using unchecked above, and detecting the underflow here, fuzzer would
                // otherwise ignore the revert.
                underflow = underflow || maxBaseFeeChange > cachedPrevBaseFee;

                // Invariant: baseFee should not have decreased by more than the maximum amount
                failedMaxLowerBaseFeePerBlock = failedMaxLowerBaseFeePerBlock
                    || ((cachedPrevBaseFee - uint256(params.prevBaseFee)) <= maxBaseFeeChange);
            }
        }
    }

    function _burnInternal(uint64 _gasToBurn) private metered(_gasToBurn) { }
}

contract ResourceMetering_Invariant is StdInvariant, InvariantTest {
    ResourceMetering_User internal actor;

    function setUp() public override {
        super.setUp();
        // Create a actor.
        actor = new ResourceMetering_User();

        targetContract(address(actor));

        bytes4[] memory selectors = new bytes4[](1);
        selectors[0] = actor.burn.selector;
        FuzzSelector memory selector = FuzzSelector({ addr: address(actor), selectors: selectors });
        targetSelector(selector);
    }

    /// @custom:invariant The base fee should increase if the last block used more
    ///                   than the target amount of gas.
    ///
    ///                   If the last block used more than the target amount of gas
    ///                   (and there were no empty blocks in between), ensure this
    ///                   block's baseFee increased, but not by more than the max amount
    ///                   per block.
    function invariant_high_usage_raise_baseFee() external view {
        assertFalse(actor.failedRaiseBaseFee());
    }

    /// @custom:invariant The base fee should decrease if the last block used less
    ///                   than the target amount of gas.
    ///
    ///                   If the previous block used less than the target amount of gas,
    ///                   the base fee should decrease, but not more than the max amount.
    function invariant_low_usage_lower_baseFee() external view {
        assertFalse(actor.failedLowerBaseFee());
    }

    /// @custom:invariant A block's base fee should never be below `MINIMUM_BASE_FEE`.
    ///
    ///                   This test asserts that a block's base fee can never drop
    ///                   below the `MINIMUM_BASE_FEE` threshold.
    function invariant_never_below_min_baseFee() external view {
        assertFalse(actor.failedNeverBelowMinBaseFee());
    }

    /// @custom:invariant A block can never consume more than `MAX_RESOURCE_LIMIT` gas.
    ///
    ///                   This test asserts that a block can never consume more than
    ///                   the `MAX_RESOURCE_LIMIT` gas threshold.
    function invariant_never_above_max_gas_limit() external view {
        assertFalse(actor.failedMaxGasPerBlock());
    }

    /// @custom:invariant The base fee can never be raised more than the max base fee change.
    ///
    ///                   After a block consumes more gas than the target gas, the base fee
    ///                   cannot be raised more than the maximum amount allowed. The max base
    ///                   fee change (per-block) is derived as follows:
    ///                   `prevBaseFee / BASE_FEE_MAX_CHANGE_DENOMINATOR`
    function invariant_never_exceed_max_increase() external view {
        assertFalse(actor.failedMaxRaiseBaseFeePerBlock());
    }

    /// @custom:invariant The base fee can never be lowered more than the max base fee change.
    ///
    ///                   After a block consumes less than the target gas, the base fee cannot
    ///                   be lowered more than the maximum amount allowed. The max base fee
    ///                   change (per-block) is derived as follows:
    ///                   `prevBaseFee / BASE_FEE_MAX_CHANGE_DENOMINATOR`
    function invariant_never_exceed_max_decrease() external view {
        assertFalse(actor.failedMaxLowerBaseFeePerBlock());
    }

    /// @custom:invariant The `maxBaseFeeChange` calculation over multiple blocks can never
    ///                   underflow.
    ///
    ///                   When calculating the `maxBaseFeeChange` after multiple empty blocks,
    ///                   the calculation should never be allowed to underflow.
    function invariant_never_underflow() external view {
        assertFalse(actor.underflow());
    }
}
