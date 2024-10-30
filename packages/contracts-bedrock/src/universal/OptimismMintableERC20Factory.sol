// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { OptimismMintableERC20 } from "src/universal/OptimismMintableERC20.sol";
import { ISemver } from "src/universal/interfaces/ISemver.sol";
import { IOptimismERC20Factory } from "src/L2/interfaces/IOptimismERC20Factory.sol";

/// @title OptimismMintableERC20Factory
/// @notice OptimismMintableERC20Factory is an abstract factory contract that generates OptimismMintableERC20
///         contracts on the network it's deployed to. It should be inherited by a child contract that
///         implements the `bridge` getter function.
///         Any changes to this contract MUST result in a semver bump for contracts that inherit it.
abstract contract OptimismMintableERC20Factory is ISemver, IOptimismERC20Factory {
    /// @custom:spacer OptimismMintableERC20Factory's initializer slot spacing
    /// @notice Spacer to avoid packing into the initializer slot
    bytes32 private spacer_0_0_32;

    /// @custom:spacer bridge
    /// @notice Spacer to avoid packing into the initializer slot
    bytes32 private spacer_1_0_32;

    /// @notice Mapping of local token address to remote token address.
    ///         This is used to keep track of the token deployments.
    mapping(address => address) public deployments;

    /// @notice Reserve extra slots in the storage layout for future upgrades.
    ///         A gap size of 48 was chosen here, so that the first slot used in a child contract
    ///         would be a multiple of 50.
    uint256[48] private __gap;

    /// @custom:legacy
    /// @notice Emitted whenever a new OptimismMintableERC20 is created. Legacy version of the newer
    ///         OptimismMintableERC20Created event. We recommend relying on that event instead.
    /// @param remoteToken Address of the token on the remote chain.
    /// @param localToken  Address of the created token on the local chain.
    event StandardL2TokenCreated(address indexed remoteToken, address indexed localToken);

    /// @notice Emitted whenever a new OptimismMintableERC20 is created.
    /// @param localToken  Address of the created token on the local chain.
    /// @param remoteToken Address of the corresponding token on the remote chain.
    /// @param deployer    Address of the account that deployed the token.
    event OptimismMintableERC20Created(address indexed localToken, address indexed remoteToken, address deployer);

    function bridge() public view virtual returns (address);

    /// @notice Getter function for the address of the StandardBridge on this chain.
    ///         Public getter is legacy and will be removed in the future. Use `bridge` instead.
    /// @return Address of the StandardBridge on this chain.
    /// @custom:legacy
    function BRIDGE() external view returns (address) {
        return bridge();
    }

    /// @custom:legacy
    /// @notice Creates an instance of the OptimismMintableERC20 contract. Legacy version of the
    ///         newer createOptimismMintableERC20 function, which has a more intuitive name.
    /// @param _remoteToken Address of the token on the remote chain.
    /// @param _name        ERC20 name.
    /// @param _symbol      ERC20 symbol.
    /// @return Address of the newly created token.
    function createStandardL2Token(
        address _remoteToken,
        string memory _name,
        string memory _symbol
    )
        external
        returns (address)
    {
        return createOptimismMintableERC20(_remoteToken, _name, _symbol);
    }

    /// @notice Creates an instance of the OptimismMintableERC20 contract.
    /// @param _remoteToken Address of the token on the remote chain.
    /// @param _name        ERC20 name.
    /// @param _symbol      ERC20 symbol.
    /// @return Address of the newly created token.
    function createOptimismMintableERC20(
        address _remoteToken,
        string memory _name,
        string memory _symbol
    )
        public
        returns (address)
    {
        return createOptimismMintableERC20WithDecimals(_remoteToken, _name, _symbol, 18);
    }

    /// @notice Creates an instance of the OptimismMintableERC20 contract, with specified decimals.
    /// @param _remoteToken Address of the token on the remote chain.
    /// @param _name        ERC20 name.
    /// @param _symbol      ERC20 symbol.
    /// @param _decimals    ERC20 decimals
    /// @return Address of the newly created token.
    function createOptimismMintableERC20WithDecimals(
        address _remoteToken,
        string memory _name,
        string memory _symbol,
        uint8 _decimals
    )
        public
        returns (address)
    {
        require(_remoteToken != address(0), "OptimismMintableERC20Factory: must provide remote token address");

        bytes32 salt = keccak256(abi.encode(_remoteToken, _name, _symbol, _decimals));

        address localToken =
            address(new OptimismMintableERC20{ salt: salt }(bridge(), _remoteToken, _name, _symbol, _decimals));

        deployments[localToken] = _remoteToken;

        // Emit the old event too for legacy support.
        emit StandardL2TokenCreated(_remoteToken, localToken);

        // Emit the updated event. The arguments here differ from the legacy event, but
        // are consistent with the ordering used in StandardBridge events.
        emit OptimismMintableERC20Created(localToken, _remoteToken, msg.sender);

        return localToken;
    }
}
