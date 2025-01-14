// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { ICrossDomainMessenger } from "interfaces/universal/ICrossDomainMessenger.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { IOptimismPortal2 as IOptimismPortal } from "interfaces/L1/IOptimismPortal2.sol";
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";

interface IL1CrossDomainMessenger is ICrossDomainMessenger {
    function PORTAL() external view returns (IOptimismPortal);
    function initialize(
        ISuperchainConfig _superchainConfig,
        IOptimismPortal _portal,
        ISystemConfig _systemConfig
    )
        external;
    function portal() external view returns (IOptimismPortal);
    function superchainConfig() external view returns (ISuperchainConfig);
    function systemConfig() external view returns (ISystemConfig);
    function version() external pure returns (string memory);
    function reinitializerValue() external pure returns (uint64);

    function __constructor__() external;
}
