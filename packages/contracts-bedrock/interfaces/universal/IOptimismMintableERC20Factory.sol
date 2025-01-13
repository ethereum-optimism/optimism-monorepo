// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

interface IOptimismMintableERC20Factory {
    error InvalidInitialization();
    error NotInitializing();

    event Initialized(uint64 version);
    event OptimismMintableERC20Created(address indexed localToken, address indexed remoteToken, address deployer);
    event StandardL2TokenCreated(address indexed remoteToken, address indexed localToken);

    function getInitializedVersion() external view returns (uint64);
    function BRIDGE() external view returns (address);
    function bridge() external view returns (address);
    function createOptimismMintableERC20(
        address _remoteToken,
        string memory _name,
        string memory _symbol
    )
        external
        returns (address);
    function createOptimismMintableERC20WithDecimals(
        address _remoteToken,
        string memory _name,
        string memory _symbol,
        uint8 _decimals
    )
        external
        returns (address);
    function createStandardL2Token(
        address _remoteToken,
        string memory _name,
        string memory _symbol
    )
        external
        returns (address);
    function deployments(address) external view returns (address);
    function initialize(address _bridge) external;
    function version() external view returns (string memory);

    function __constructor__() external;
}
