// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

// Interfaces
import { Initializable } from "src/vendor/Initializable-v5.sol";

/// @title InitializablePublic
/// @notice Expands Initializable v5 vendored from OpenZeppelin to expose the version getter
contract InitializablePublic is Initializable {
    function getInitializedVersion() external view returns (uint64) {
        return _getInitializedVersion();
    }
}
