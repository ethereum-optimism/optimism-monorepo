// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { IERC721Bridge } from "src/universal/interfaces/IERC721Bridge.sol";
import { ICrossDomainMessenger } from "src/universal/interfaces/ICrossDomainMessenger.sol";

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
    function messenger() external pure returns (ICrossDomainMessenger);
    function version() external view returns (string memory);

    function __constructor__() external;
}
