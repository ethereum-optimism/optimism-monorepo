// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { IERC165 } from "@openzeppelin/contracts/utils/introspection/IERC165.sol";
/// @custom:legacy
/// @title ILegacyMintableERC20
/// @notice This interface was available on the legacy L2StandardERC20 contract.
///         It remains available on the OptimismMintableERC20 contract for
///         backwards compatibility.

interface ILegacyMintableERC20 is IERC165 {
    function l1Token() external view returns (address);

    function mint(address _to, uint256 _amount) external;

    function burn(address _from, uint256 _amount) external;
}
