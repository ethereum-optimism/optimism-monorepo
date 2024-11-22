// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { ERC20 } from "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import { IOptimismPortalInterop as IOptimismPortal } from "src/L1/interfaces/IOptimismPortalInterop.sol";
import { SystemConfig } from "src/L1/SystemConfig.sol";
import { ConfigType } from "src/L2/L1BlockInterop.sol";

// Libraries
import { Constants } from "src/libraries/Constants.sol";
import { GasPayingToken } from "src/libraries/GasPayingToken.sol";
import { StaticConfig } from "src/libraries/StaticConfig.sol";
import { Storage } from "src/libraries/Storage.sol";

// Interfaces
import { IResourceMetering } from "src/L1/interfaces/IResourceMetering.sol";

/// @custom:proxied true
/// @title SystemConfigInterop
/// @notice The SystemConfig contract is used to manage configuration of an Optimism network.
///         All configuration is stored on L1 and picked up by L2 as part of the derviation of
///         the L2 chain.
contract SystemConfigInterop is SystemConfig {
    /// @notice Storage slot where the dependency manager address is stored
    /// @dev    Equal to bytes32(uint256(keccak256("systemconfig.dependencymanager")) - 1)
    bytes32 internal constant DEPENDENCY_MANAGER_SLOT =
        0x1708e077affb93e89be2665fb0fb72581be66f84dc00d25fed755ae911905b1c;

    /// @notice Initializer.
    /// @param _owner             Initial owner of the contract.
    /// @param _basefeeScalar     Initial basefee scalar value.
    /// @param _blobbasefeeScalar Initial blobbasefee scalar value.
    /// @param _batcherHash       Initial batcher hash.
    /// @param _gasLimit          Initial gas limit.
    /// @param _unsafeBlockSigner Initial unsafe block signer address.
    /// @param _config            Initial ResourceConfig.
    /// @param _batchInbox        Batch inbox address. An identifier for the op-node to find
    ///                           canonical data.
    /// @param _addresses         Set of L1 contract addresses. These should be the proxies.
    /// @param _dependencyManager The addressed allowed to add/remove from the dependency set
    function initialize(
        address _owner,
        uint32 _basefeeScalar,
        uint32 _blobbasefeeScalar,
        bytes32 _batcherHash,
        uint64 _gasLimit,
        address _unsafeBlockSigner,
        IResourceMetering.ResourceConfig memory _config,
        address _batchInbox,
        SystemConfig.Addresses memory _addresses,
        address _dependencyManager
    )
        external
    {
        // This method has an initializer modifier, and will revert if already initialized.
        initialize({
            _owner: _owner,
            _basefeeScalar: _basefeeScalar,
            _blobbasefeeScalar: _blobbasefeeScalar,
            _batcherHash: _batcherHash,
            _gasLimit: _gasLimit,
            _unsafeBlockSigner: _unsafeBlockSigner,
            _config: _config,
            _batchInbox: _batchInbox,
            _addresses: _addresses
        });
        Storage.setAddress(DEPENDENCY_MANAGER_SLOT, _dependencyManager);
    }

    /// @custom:semver +interop-beta.4
    function version() public pure override returns (string memory) {
        return string.concat(super.version(), "+interop-beta.4");
    }

    /// @notice Internal setter for the gas paying token address, includes validation.
    ///         The token must not already be set and must be non zero and not the ether address
    ///         to set the token address. This prevents the token address from being changed
    ///         and makes it explicitly opt-in to use custom gas token. Additionally,
    ///         OptimismPortal's address must be non zero, since otherwise the call to set the
    ///         config for the gas paying token to OptimismPortal will fail.
    /// @param _token Address of the gas paying token.
    function _setGasPayingToken(address _token) internal override {
        if (_token != address(0) && _token != Constants.ETHER && !isCustomGasToken()) {
            require(
                ERC20(_token).decimals() == GAS_PAYING_TOKEN_DECIMALS, "SystemConfig: bad decimals of gas paying token"
            );
            bytes32 name = GasPayingToken.sanitize(ERC20(_token).name());
            bytes32 symbol = GasPayingToken.sanitize(ERC20(_token).symbol());

            // Set the gas paying token in storage and in the OptimismPortal.
            GasPayingToken.set({ _token: _token, _decimals: GAS_PAYING_TOKEN_DECIMALS, _name: name, _symbol: symbol });
            IOptimismPortal(payable(optimismPortal())).setConfig(
                ConfigType.SET_GAS_PAYING_TOKEN,
                StaticConfig.encodeSetGasPayingToken({
                    _token: _token,
                    _decimals: GAS_PAYING_TOKEN_DECIMALS,
                    _name: name,
                    _symbol: symbol
                })
            );
        }
    }

    /// @notice Adds a chain to the interop dependency set. Can only be called by the dependency manager.
    /// @param _chainId Chain ID of chain to add.
    function addDependency(uint256 _chainId) external {
        require(msg.sender == dependencyManager(), "SystemConfig: caller is not the dependency manager");
        IOptimismPortal(payable(optimismPortal())).setConfig(
            ConfigType.ADD_DEPENDENCY, StaticConfig.encodeAddDependency(_chainId)
        );
    }

    /// @notice Removes a chain from the interop dependency set. Can only be called by the dependency manager
    /// @param _chainId Chain ID of the chain to remove.
    function removeDependency(uint256 _chainId) external {
        require(msg.sender == dependencyManager(), "SystemConfig: caller is not the dependency manager");
        IOptimismPortal(payable(optimismPortal())).setConfig(
            ConfigType.REMOVE_DEPENDENCY, StaticConfig.encodeRemoveDependency(_chainId)
        );
    }

    /// @notice getter for the dependency manager address
    function dependencyManager() public view returns (address) {
        return Storage.getAddress(DEPENDENCY_MANAGER_SLOT);
    }
}
