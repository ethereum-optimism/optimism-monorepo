// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { ICrossDomainMessenger } from "interfaces/universal/ICrossDomainMessenger.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { IOptimismPortal2 as IOptimismPortal } from "interfaces/L1/IOptimismPortal2.sol";

interface IL1CrossDomainMessenger is ICrossDomainMessenger {
    function PORTAL() external view returns (IOptimismPortal);
    function initialize(
        ISuperchainConfig _superchainConfig,
        IOptimismPortal _portal
    )
        external;
    function portal() external view returns (IOptimismPortal);
    function superchainConfig() external view returns (ISuperchainConfig);
    function version() external view returns (string memory);

    function __constructor__() external;
}
