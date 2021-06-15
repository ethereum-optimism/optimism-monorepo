// SPDX-License-Identifier: MIT
pragma solidity >0.5.0 <0.8.0;

/**
 * @title iOVM_SafetyCache
 */
interface iOVM_SafetyCache {

    /*********************
    * External Functions *
    **********************/
    function checkAndRegisterSafeBytecode(bytes memory _code) external returns (bool);

    function isRegisteredSafeBytecode(bytes32 _codehash) external view returns (bool);
}
