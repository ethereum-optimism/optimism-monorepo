// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { ISemver } from "src/universal/interfaces/ISemver.sol";

/// @title IL1ERC721Bridge
/// @notice Interface for the L1ERC721Bridge contract.
interface IL1ERC721Bridge is ISemver {
    function finalizeBridgeERC721(
        address _localToken,
        address _remoteToken,
        address _from,
        address _to,
        uint256 _tokenId,
        bytes calldata _extraData
    )
        external;
}
