// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

interface IETHLiquidity {
    error NotCustomGasToken();
    error Unauthorized();

    event LiquidityBurned(address indexed caller, uint256 value);
    event LiquidityMinted(address indexed caller, uint256 value);

    function burn() external payable;
    function mint(uint256 _amount) external;
    function version() external view returns (string memory);

    function __constructor__() external;
}
