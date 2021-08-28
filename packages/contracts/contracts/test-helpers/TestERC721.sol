// SPDX-License-Identifier: MIT
pragma solidity >0.5.0 <0.8.0;

import { ERC721 } from "@openzeppelin/contracts/token/ERC721/ERC721.sol";

// a test ERC721 token with an open mint function
contract TestERC721 is ERC721 {
    constructor() ERC721('Test', 'TST') {}

    function mint(address to, uint256 tokenId) external {
        _mint(to, tokenId);
    }
}
