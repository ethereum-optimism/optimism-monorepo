// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

import { IProxyAdmin } from "interfaces/universal/IProxyAdmin.sol";
import { Storage } from "src/libraries/Storage.sol";
import { Constants } from "src/libraries/Constants.sol";

/// @notice Base contract for ProxyAdmin-owned contracts. It's main goal is to expose the ProxyAdmin owner address on
///         a function and also to check if the current contract and a given proxy have the same PAO.
abstract contract PAOBase {
    /// @notice Getter for the owner of the ProxyAdmin.
    ///         The ProxyAdmin is the owner of the current proxy contract.
    function PAO() public view returns (address) {
        // Get the proxy admin address reading for the reserved slot it has on the Proxy contract.
        IProxyAdmin proxyAdmin = IProxyAdmin(Storage.getAddress(Constants.PROXY_OWNER_ADDRESS));
        // Return the owner of the proxy admin.
        return proxyAdmin.owner();
    }

    /// @notice Checks if the ProxyAdmin owner of the current contract is the same as the ProxyAdmin owner of the given
    ///         proxy.
    /// @param _proxy The address of the proxy to check.
    function _samePAO(address _proxy) internal view returns (bool) {
        return PAO() == PAOBase(_proxy).PAO();
    }
}
