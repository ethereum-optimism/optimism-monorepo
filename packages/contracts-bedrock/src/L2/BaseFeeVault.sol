// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { ISemver } from "src/universal/interfaces/ISemver.sol";
import { FeeVault } from "src/L2/FeeVault.sol";

import { Types } from "src/libraries/Types.sol";

/// @custom:proxied true
/// @custom:predeploy 0x4200000000000000000000000000000000000019
/// @title BaseFeeVault
/// @notice The BaseFeeVault accumulates the base fee that is paid by transactions.
contract BaseFeeVault is FeeVault, ISemver {
    /// @notice Semantic version.
    /// @custom:semver 1.5.0-beta.3
    string public constant version = "1.5.0-beta.3";

    /// @notice Constructs the BaseFeeVault contract.
    /// @param _recipient           Wallet that will receive the fees.
    /// @param _minWithdrawalAmount Minimum balance for withdrawals.
    /// @param _withdrawalNetwork   Network which the recipient will receive fees on.
    constructor(
        address _recipient,
        uint256 _minWithdrawalAmount,
        Types.WithdrawalNetwork _withdrawalNetwork
    )
        FeeVault(_recipient, _minWithdrawalAmount, _withdrawalNetwork)
    { }
}
