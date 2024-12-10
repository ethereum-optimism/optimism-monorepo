// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Interfaces
import { IDripCheck } from "src/periphery/drippie/IDripCheck.sol";

/// @title CheckBalanceLow
/// @notice DripCheck for checking if an account's balance is below a given threshold.
contract CheckBalanceLow is IDripCheck {
    struct Params {
        address target;
        uint256 threshold;
    }

    /// @notice External event used to help client-side tooling encode parameters.
    /// @param params Parameters to encode.
    event _EventToExposeStructInABI__Params(Params params);

    /// @inheritdoc IDripCheck
    string public name = "CheckBalanceLow";

    /// @inheritdoc IDripCheck
    function check(bytes memory _params) external view returns (bool execute_) {
        Params memory params = abi.decode(_params, (Params));

        // Check target ETH balance is below threshold.
        execute_ = params.target.balance < params.threshold;
    }
}
