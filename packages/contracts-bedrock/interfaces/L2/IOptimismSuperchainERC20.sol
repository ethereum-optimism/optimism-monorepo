// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

// Interfaces
import { ISuperchainERC20 } from "interfaces/L2/ISuperchainERC20.sol";

/// @title IOptimismSuperchainERC20
/// @notice This interface is available on the OptimismSuperchainERC20 contract.
interface IOptimismSuperchainERC20 is ISuperchainERC20 {
    error ZeroAddress();
    error InvalidInitialization();
    error NotInitializing();

    event Initialized(uint64 version);

    event Mint(address indexed to, uint256 amount);

    event Burn(address indexed from, uint256 amount);

    function initialize(address _remoteToken, string memory _name, string memory _symbol, uint8 _decimals) external;

    function mint(address _to, uint256 _amount) external;

    function burn(address _from, uint256 _amount) external;

    function remoteToken() external view returns (address);

    function supportsInterface(bytes4 _interfaceId) external view returns (bool);

    function __constructor__() external;
}
