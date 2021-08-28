// SPDX-License-Identifier: MIT
pragma solidity >=0.5.16 <0.8.0;

/* Contract Imports */
import { L2StandardERC721 } from "../optimistic-ethereum/libraries/standards/L2StandardERC721.sol";

/**
 * @dev L2StandardERC721 is difficult to mock because of  OZ UintSet and UintToAddressMap.
 * This contract provides a `mintTestToken` function to generate tokens for testing.
 */
contract TestL2StandardERC721 is L2StandardERC721 {
    constructor(
        address _l2Bridge,
        address _l1Token,
        string memory _name,
        string memory _symbol
    )
        L2StandardERC721(_l2Bridge, _l1Token, _name, _symbol)
    {}

    function mintTestToken(address _to, uint256 _tokenId) public virtual {
        _mint(_to, _tokenId);
        emit Mint(_to, _tokenId);
    }
}
