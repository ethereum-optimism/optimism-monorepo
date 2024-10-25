// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { IERC165 } from "@openzeppelin/contracts/utils/introspection/IERC165.sol";
import { IERC20 } from "@openzeppelin/contracts/token/ERC20/IERC20.sol";

/// @custom:legacy
/// @title ILegacyMintableERC20
/// @notice This interface was available on the legacy L2StandardERC20 contract.
///         It remains available on the OptimismMintableERC20 contract for
///         backwards compatibility.
interface ILegacyMintableERC20 is IERC165, IERC20 {
    function l1Token() external view returns (address);

    function mint(address _to, uint256 _amount) external;

    function burn(address _from, uint256 _amount) external;

    function __constructor__(
        address _l2Bridge,
        address _l1Token,
        string memory _name,
        string memory _symbol
    )
        external;

    event Mint(address indexed _account, uint256 _amount);
    event Burn(address indexed _account, uint256 _amount);

    function supportsInterface(bytes4 _interfaceId) external pure returns (bool);

    function symbol() external view returns (string memory);

    function name() external view returns (string memory);

    function decreaseAllowance(address spender, uint256 subtractedValue) external returns (bool);

    function increaseAllowance(address spender, uint256 addedValue) external returns (bool);

    function l2Bridge() external view returns (address);

    function decimals() external view returns (uint8);
}
