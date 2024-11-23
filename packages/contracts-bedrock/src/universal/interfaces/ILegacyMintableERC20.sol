// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { IERC165 } from "@openzeppelin/contracts/utils/introspection/IERC165.sol";
import { IERC20Metadata } from "@openzeppelin/contracts/token/ERC20/extensions/IERC20Metadata.sol";
/// @custom:legacy
/// @title ILegacyMintableERC20
/// @notice This interface was available on the legacy L2StandardERC20 contract.
///         It remains available on the OptimismMintableERC20 contract for
///         backwards compatibility.

interface ILegacyMintableERC20 is IERC165, IERC20Metadata {
    function l1Token() external view returns (address);

    function mint(address _to, uint256 _amount) external;

    function burn(address _from, uint256 _amount) external;

    function l2Bridge() external view returns (address);

    function __constructor__(
        address _l2Bridge,
        address _l1Token,
        string memory _name,
        string memory _symbol
    )
        external;
}
