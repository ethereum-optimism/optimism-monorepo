// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Interfaces
import { IDripCheck } from "src/periphery/drippie/IDripCheck.sol";

/// @title CheckTrue
/// @notice DripCheck that always returns true.
contract CheckTrue is IDripCheck {
    /// @inheritdoc IDripCheck
    string public name = "CheckTrue";

    /// @inheritdoc IDripCheck
    function check(bytes memory) external pure returns (bool execute_) {
        execute_ = true;
    }
}
