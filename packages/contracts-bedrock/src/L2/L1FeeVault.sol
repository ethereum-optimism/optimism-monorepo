// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { FeeVault } from "src/L2/FeeVault.sol";
import { Semver } from "src/universal/Semver.sol";

// Libraries
import { Types } from "src/libraries/Types.sol";

/// @custom:proxied true
/// @custom:predeploy 0x420000000000000000000000000000000000001A
/// @title L1FeeVault
/// @notice The L1FeeVault accumulates the L1 portion of the transaction fees.
contract L1FeeVault is FeeVault, Semver {
    /// @notice Semantic version.
    /// @custom:semver 1.5.1
    function _version() internal pure override returns (Versions memory) {
        return Versions({ major: 1, minor: 5, patch: 1, suffix: "" });
    }

    /// @notice Constructs the L1FeeVault contract.
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
