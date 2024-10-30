// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { ISemver } from "src/universal/interfaces/ISemver.sol";
import { FeeVault } from "src/L2/FeeVault.sol";
import { Types } from "src/libraries/Types.sol";
import { Encoding } from "src/libraries/Encoding.sol";

/// @custom:proxied true
/// @custom:predeploy 0x4200000000000000000000000000000000000019
/// @title BaseFeeVault
/// @notice The BaseFeeVault accumulates the base fee that is paid by transactions.
contract BaseFeeVault is FeeVault, ISemver {
    /// @notice Semantic version.
    /// @custom:semver 1.5.0-beta.4
    string public constant version = "1.5.0-beta.4";

    /// @notice Returns the FeeVault config
    /// @return recipient_           Wallet that will receive the fees.
    /// @return amount_              Minimum balance for withdrawals.
    /// @return withdrawalNetwork_   Network which the recipient will receive fees on.
    function config()
        public
        view
        override
        returns (address recipient_, uint256 amount_, Types.WithdrawalNetwork withdrawalNetwork_)
    {
        bytes memory data = L1_BLOCK().getConfig(Types.ConfigType.BASE_FEE_VAULT_CONFIG);
        (recipient_, amount_, withdrawalNetwork_) = Encoding.decodeFeeVaultConfig(abi.decode(data, (bytes32)));
    }
}
