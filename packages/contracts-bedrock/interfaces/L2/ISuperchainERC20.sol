// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

// Interfaces
import { IERC7802 } from "interfaces/L2/IERC7802.sol";
import { IERC20Solady as IERC20 } from "interfaces/vendor/IERC20Solady.sol";

/// @title ISuperchainERC20
/// @notice This interface is available on the SuperchainERC20 contract.
/// @dev This interface is needed for the abstract SuperchainERC20 implementation but is not part of the standard
interface ISuperchainERC20 is IERC7802, IERC20 {
    error Unauthorized();

    function supportsInterface(bytes4 _interfaceId) external view returns (bool);

    function version() external pure returns (string memory);

    function reinitializerValue() external pure returns (uint64);

    function __constructor__() external;
}
