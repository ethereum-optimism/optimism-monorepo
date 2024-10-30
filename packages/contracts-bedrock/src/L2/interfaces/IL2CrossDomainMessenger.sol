// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { ICrossDomainMessenger } from "src/universal/interfaces/ICrossDomainMessenger.sol";

interface IL2CrossDomainMessenger is ICrossDomainMessenger {
    function MESSAGE_VERSION() external view returns (uint16);
    function l1CrossDomainMessenger() external view returns (ICrossDomainMessenger);
    function OTHER_MESSENGER() external view returns (ICrossDomainMessenger);
    function otherMessenger() external view returns (ICrossDomainMessenger);
    function version() external view returns (string memory);

    function __constructor__() external;
}
