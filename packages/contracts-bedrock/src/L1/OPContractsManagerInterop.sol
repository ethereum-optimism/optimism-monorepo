// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { OPContractsManager } from "src/L1/OPContractsManager.sol";

// Interfaces
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { IProtocolVersions } from "interfaces/L1/IProtocolVersions.sol";
import { IResourceMetering } from "interfaces/L1/IResourceMetering.sol";
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";
import { ISystemConfigInterop } from "interfaces/L1/ISystemConfigInterop.sol";
import { IProxyAdmin } from "interfaces/universal/IProxyAdmin.sol";

contract OPContractsManagerInterop is OPContractsManager {
    /// @custom:semver +interop.4
    function version() public pure override returns (string memory) {
        return string.concat(super.version(), "+interop.4");
    }

    constructor(
        ISuperchainConfig _superchainConfig,
        IProtocolVersions _protocolVersions,
        IProxyAdmin _superchainProxyAdmin,
        string memory _l1ContractsRelease,
        Blueprints memory _blueprints,
        Implementations memory _implementations,
        address _upgradeController
    )
        OPContractsManager(
            _superchainConfig,
            _protocolVersions,
            _superchainProxyAdmin,
            _l1ContractsRelease,
            _blueprints,
            _implementations,
            _upgradeController
        )
    { }

    // The `SystemConfigInterop` contract has an extra `address _dependencyManager` argument
    // that we must account for.
    function encodeSystemConfigInitializer(
        DeployInput memory _input,
        DeployOutput memory _output
    )
        internal
        view
        virtual
        override
        returns (bytes memory)
    {
        (IResourceMetering.ResourceConfig memory referenceResourceConfig, ISystemConfig.Addresses memory opChainAddrs) =
            defaultSystemConfigParams(_input, _output);

        // TODO For now we assume that the dependency manager is the same as system config owner.
        // This is currently undefined since it's not part of the standard config, so we may need
        // to update where this value is pulled from in the future. To support a different dependency
        // manager in this contract without an invasive change of redefining the `Roles` struct,
        // we will make the change described in https://github.com/ethereum-optimism/optimism/issues/11783.
        address dependencyManager = address(_input.roles.systemConfigOwner);

        return abi.encodeCall(
            ISystemConfigInterop.initialize,
            (
                _input.roles.systemConfigOwner,
                _input.basefeeScalar,
                _input.blobBasefeeScalar,
                bytes32(uint256(uint160(_input.roles.batcher))), // batcherHash
                _input.gasLimit,
                _input.roles.unsafeBlockSigner,
                referenceResourceConfig,
                chainIdToBatchInboxAddress(_input.l2ChainId),
                opChainAddrs,
                dependencyManager
            )
        );
    }
}
