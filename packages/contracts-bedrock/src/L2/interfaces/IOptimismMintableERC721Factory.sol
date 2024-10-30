// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { IL2ERC721Bridge } from "src/L2/interfaces/IL2ERC721Bridge.sol";

interface IOptimismMintableERC721Factory {
    event OptimismMintableERC721Created(address indexed localToken, address indexed remoteToken, address deployer);

    function BRIDGE() external pure returns (IL2ERC721Bridge);
    function bridge() external pure returns (IL2ERC721Bridge);
    function createOptimismMintableERC721(
        address _remoteToken,
        string memory _name,
        string memory _symbol
    )
        external
        returns (address);
    function isOptimismMintableERC721(address) external view returns (bool);
    function REMOTE_CHAIN_ID() external view returns (uint256);
    function remoteChainId() external view returns (uint256);
    function version() external view returns (string memory);

    function __constructor__() external;
}
