// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { IProtocolVersions } from "interfaces/L1/IProtocolVersions.sol";
import { IOPContractsManager } from "interfaces/L1/IOPContractsManager.sol";

interface IOPPrestateUpdater is IOPContractsManager {
    /// @notice Thrown when a function from the parent (OPCM) is not implemented.
    error NotImplemented();

    /// @notice Thrown when the prestate of a permissioned disputed game is 0.
    error PrestateRequired();

    function __constructor__(
        ISuperchainConfig _superchainConfig,
        IProtocolVersions _protocolVersions,
        Blueprints memory _blueprints
    )
    external;

    function updatePrestate(OpChainConfig[] memory _prestateUpdateInputs) external;
}
