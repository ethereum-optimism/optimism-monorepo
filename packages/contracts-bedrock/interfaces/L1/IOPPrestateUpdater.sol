// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { IProtocolVersions } from "interfaces/L1/IProtocolVersions.sol";
import { IProxyAdmin } from "interfaces/universal/IProxyAdmin.sol";
import { IOPContractsManager } from "interfaces/L1/IOPContractsManager.sol";

interface IOPPrestateUpdater is IOPContractsManager {
    /// @notice Thrown when a function from the parent (OPCM) is not implemented.
    error NotImplemented();

    /// @notice Thrown when the prestate of a permissioned disputed game is 0.
    error PDGPrestateRequired();

    /// @notice Thrown when the address off a fault dispute game is 0.
    error FDGNotFound();

    function __constructor__(
        ISuperchainConfig _superchainConfig,
        IProtocolVersions _protocolVersions,
        IProxyAdmin _superchainProxyAdmin,
        string memory _l1ContractsRelease,
        Blueprints memory _blueprints,
        Implementations memory _implementations,
        address _upgradeController
    )
    external;

    function updatePrestate(OpChainConfig[] memory _prestateUpdateInputs) external;
}
