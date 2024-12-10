// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// @title IOptimismMintableERC721
/// @notice Interface for contracts that are compatible with the OptimismMintableERC721 standard.
///         Tokens that follow this standard can be easily transferred across the ERC721 bridge.
interface IOptimismMintableERC721 {
    function __constructor__(
        address _bridge,
        uint256 _remoteChainId,
        address _remoteToken,
        string memory _name,
        string memory _symbol
    )
        external;

    event ApprovalForAll(address indexed owner, address indexed operator, bool approved);
    event Approval(address indexed owner, address indexed approved, uint256 indexed tokenId);
    event Burn(address indexed account, uint256 tokenId);
    event Mint(address indexed account, uint256 tokenId);
    event Transfer(address indexed from, address indexed to, uint256 indexed tokenId);

    function totalSupply() external view returns (uint256);

    function approve(address to, uint256 tokenId) external;

    function isApprovedForAll(address owner, address operator) external view returns (bool);

    function symbol() external view returns (string memory);

    function tokenByIndex(uint256 index) external view returns (uint256);

    function tokenOfOwnerByIndex(address owner, uint256 index) external view returns (uint256);

    function transferFrom(address from, address to, uint256 tokenId) external;

    function balanceOf(address owner) external view returns (uint256);

    function baseTokenURI() external view returns (string memory);

    function getApproved(uint256 tokenId) external view returns (address);

    function name() external view returns (string memory);

    function ownerOf(uint256 tokenId) external view returns (address);

    function safeTransferFrom(address from, address to, uint256 tokenId) external;

    function safeTransferFrom(address from, address to, uint256 tokenId, bytes memory data) external;

    function setApprovalForAll(address operator, bool approved) external;

    function supportsInterface(bytes4 _interfaceId) external view returns (bool);

    function tokenURI(uint256 tokenId) external view returns (string memory);

    function version() external view returns (string memory);

    function safeMint(address _to, uint256 _tokenId) external;

    function burn(address _from, uint256 _tokenId) external;

    function REMOTE_CHAIN_ID() external view returns (uint256);

    function REMOTE_TOKEN() external view returns (address);

    function BRIDGE() external view returns (address);

    function remoteChainId() external view returns (uint256);

    function remoteToken() external view returns (address);

    function bridge() external view returns (address);
}
