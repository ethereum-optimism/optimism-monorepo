// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { OptimismMintableERC721 } from "src/universal/OptimismMintableERC721.sol";
import { IL2ERC721Bridge } from "src/L2/interfaces/IL2ERC721Bridge.sol";
import { ISemver } from "src/universal/interfaces/ISemver.sol";
import { IL1Block } from "src/L2/interfaces/IL1Block.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";
import { Types } from "src/libraries/Types.sol";

/// @title OptimismMintableERC721Factory
/// @notice Factory contract for creating OptimismMintableERC721 contracts.
///         This contract could in theory live on both L1 and L2 but it is not widely
///         used and is therefore set up to work on L2. This could be abstracted in the
///         future to be deployable on L1 as well.
contract OptimismMintableERC721Factory is ISemver {
    /// @notice Tracks addresses created by this factory.
    mapping(address => bool) public isOptimismMintableERC721;

    /// @notice Emitted whenever a new OptimismMintableERC721 contract is created.
    /// @param localToken  Address of the token on the this domain.
    /// @param remoteToken Address of the token on the remote domain.
    /// @param deployer    Address of the initiator of the deployment
    event OptimismMintableERC721Created(address indexed localToken, address indexed remoteToken, address deployer);

    /// @notice Semantic version.
    ///     The semver MUST be bumped any time that there is a change in
    ///     the OptimismMintableERC721 token contract since this contract
    ///     is responsible for deploying OptimismMintableERC721 contracts.
    /// @custom:semver 1.4.1-beta.4
    string public constant version = "1.4.1-beta.4";

    /// @notice Returns the remote chain id
    function REMOTE_CHAIN_ID() external view returns (uint256) {
        return remoteChainId();
    }

    /// @notice Getter function for the remote chain id.
    function remoteChainId() public view returns (uint256) {
        bytes memory data = IL1Block(Predeploys.L1_BLOCK_ATTRIBUTES).getConfig(Types.ConfigType.REMOTE_CHAIN_ID);
        return abi.decode(data, (uint256));
    }

    /// @notice Getter function for the bridge contract.
    ///         Public getter is legacy and will be removed in the future. Use `bridge()` instead.
    /// @return Bridge contract on this domain.
    /// @custom:legacy
    function BRIDGE() external pure returns (IL2ERC721Bridge) {
        return bridge();
    }

    /// @notice Returns the ERC721 bridge contract.
    function bridge() public pure returns (IL2ERC721Bridge) {
        return IL2ERC721Bridge(Predeploys.L2_ERC721_BRIDGE);
    }

    /// @notice Creates an instance of the standard ERC721.
    /// @param _remoteToken Address of the corresponding token on the other domain.
    /// @param _name        ERC721 name.
    /// @param _symbol      ERC721 symbol.
    function createOptimismMintableERC721(
        address _remoteToken,
        string memory _name,
        string memory _symbol
    )
        external
        returns (address)
    {
        require(_remoteToken != address(0), "OptimismMintableERC721Factory: L1 token address cannot be address(0)");

        bytes32 salt = keccak256(abi.encode(_remoteToken, _name, _symbol));
        address localToken =
            address(new OptimismMintableERC721{ salt: salt }(bridge(), remoteChainId(), _remoteToken, _name, _symbol));

        isOptimismMintableERC721[localToken] = true;
        emit OptimismMintableERC721Created(localToken, _remoteToken, msg.sender);

        return localToken;
    }
}
