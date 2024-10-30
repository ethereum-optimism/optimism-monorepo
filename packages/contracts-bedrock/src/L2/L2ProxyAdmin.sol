// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { ProxyAdmin } from "src/universal/ProxyAdmin.sol";
import { Constants } from "src/libraries/Constants.sol";

/// @custom:proxied true
/// @custom:predeploy
/// @title L2ProxyAdmin
contract L2ProxyAdmin is ProxyAdmin {
    constructor() ProxyAdmin(Constants.DEPOSITOR_ACCOUNT) { }

    /// @notice The owner of the L2ProxyAdmin is the `DEPOSITOR_ACCOUNT`.
    function owner() public pure override returns (address) {
        return Constants.DEPOSITOR_ACCOUNT;
    }
}
