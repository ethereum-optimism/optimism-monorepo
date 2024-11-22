// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { OPContractsManager } from "src/L1/OPContractsManager.sol";
import { ISuperchainConfig } from "src/L1/interfaces/ISuperchainConfig.sol";
import { IProtocolVersions } from "src/L1/interfaces/IProtocolVersions.sol";
import { IResourceMetering } from "src/L1/interfaces/IResourceMetering.sol";
import { ISystemConfig } from "src/L1/interfaces/ISystemConfig.sol";
import { ISystemConfigInterop } from "src/L1/interfaces/ISystemConfigInterop.sol";

contract OPContractsManagerInterop is OPContractsManager {
    constructor(
        ISuperchainConfig _superchainConfig,
        IProtocolVersions _protocolVersions,
        string memory _l1ContractsRelease,
        Blueprints memory _blueprints,
        Implementations memory _implementations
    )
        OPContractsManager(_superchainConfig, _protocolVersions, _l1ContractsRelease, _blueprints, _implementations)
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
        bytes4 selector = ISystemConfigInterop.initialize.selector;
        (IResourceMetering.ResourceConfig memory referenceResourceConfig, ISystemConfig.Addresses memory opChainAddrs) =
            defaultSystemConfigParams(selector, _input, _output);

        // TODO For now we assume that the dependency manager is the same as system config owner.
        // This is currently undefined since it's not part of the standard config, so we may need
        // to update where this value is pulled from in the future. To support a different dependency
        // manager in this contract without an invasive change of redefining the `Roles` struct,
        // we will make the change described in https://github.com/ethereum-optimism/optimism/issues/11783.
        address dependencyManager = address(_input.roles.systemConfigOwner);

        return abi.encodeWithSelector(
            selector,
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
        );
    }
}
