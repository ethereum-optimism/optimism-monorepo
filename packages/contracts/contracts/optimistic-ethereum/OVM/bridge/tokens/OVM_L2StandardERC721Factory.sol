// SPDX-License-Identifier: MIT
pragma solidity >0.5.0 <0.8.0;
pragma experimental ABIEncoderV2;

/* Contract Imports */
import { L2StandardERC721 } from "../../../libraries/standards/L2StandardERC721.sol";
import { Lib_PredeployAddresses } from "../../../libraries/constants/Lib_PredeployAddresses.sol";

/**
 * @title OVM_L2StandardERC721Factory
 * @dev Factory contract for creating standard L2 token representations of L1 ERC721s
 * compatible with and working on the standard bridge.
 * Compiler used: optimistic-solc
 * Runtime target: OVM
 */
contract OVM_L2StandardERC721Factory {

    event StandardL2ERC721Created(address indexed _l1Token, address indexed _l2Token);

    /**
     * @dev Creates an instance of the standard ERC721 token on L2.
     * @param _l1Token Address of the corresponding L1 token.
     * @param _name ERC721 name.
     * @param _symbol ERC721 symbol.
     */
    function createStandardL2ERC721(
        address _l1Token,
        string memory _name,
        string memory _symbol
    )
        external
    {
        require (_l1Token != address(0), "Must provide L1 token address");

        L2StandardERC721 l2Token = new L2StandardERC721(
            Lib_PredeployAddresses.L2_STANDARD_BRIDGE,
            _l1Token,
            _name,
            _symbol
        );

        emit StandardL2ERC721Created(_l1Token, address(l2Token));
    }
}
