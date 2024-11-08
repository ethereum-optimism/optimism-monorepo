// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

// Interfaces
import { IERC7802 } from "src/L2/interfaces/IERC7802.sol";
import { IERC20Solady as IERC20 } from "src/vendor/interfaces/IERC20Solady.sol";
import { ISemver } from "src/universal/interfaces/ISemver.sol";

/// @title ISuperchainERC20
/// @notice This interface is available on the SuperchainERC20 contract.
/// @dev This interface is needed for the abstract SuperchainERC20 implementation but is not part of the standard
interface ISuperchainERC20 is IERC7802, IERC20, ISemver {
    error Unauthorized();

    function supportsInterface(bytes4 _interfaceId) external view returns (bool);

    function __constructor__() external;
}
