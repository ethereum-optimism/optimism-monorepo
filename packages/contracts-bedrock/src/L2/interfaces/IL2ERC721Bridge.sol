// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { IERC721Bridge } from "src/universal/interfaces/IERC721Bridge.sol";

interface IL2ERC721Bridge is IERC721Bridge {
    function finalizeBridgeERC721(
        address _localToken,
        address _remoteToken,
        address _from,
        address _to,
        uint256 _tokenId,
        bytes memory _extraData
    )
        external;
    function initialize(address payable _l1ERC721Bridge) external;
    function version() external view returns (string memory);

    function __constructor__() external;
}
