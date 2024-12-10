// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Interfaces
import { IGelatoTreasury } from "interfaces/vendor/IGelatoTreasury.sol";
import { IDripCheck } from "src/periphery/drippie/IDripCheck.sol";

/// @title CheckGelatoLow
/// @notice DripCheck for checking if an account's Gelato ETH balance is below some threshold.
contract CheckGelatoLow is IDripCheck {
    struct Params {
        address treasury;
        uint256 threshold;
        address recipient;
    }

    /// @notice External event used to help client-side tooling encode parameters.
    /// @param params Parameters to encode.
    event _EventToExposeStructInABI__Params(Params params);

    /// @inheritdoc IDripCheck
    string public name = "CheckGelatoLow";

    /// @inheritdoc IDripCheck
    function check(bytes memory _params) external view returns (bool execute_) {
        Params memory params = abi.decode(_params, (Params));

        // Gelato represents ETH as 0xeeeee....eeeee.
        address eth = 0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE;

        // Get the total deposited amount.
        uint256 deposited = IGelatoTreasury(params.treasury).totalDepositedAmount(params.recipient, eth);

        // Get the total withdrawn amount.
        uint256 withdrawn = IGelatoTreasury(params.treasury).totalWithdrawnAmount(params.recipient, eth);

        // Figure out the current balance.
        uint256 balance = deposited - withdrawn;

        // Check if the balance is below the threshold.
        execute_ = balance < params.threshold;
    }
}
