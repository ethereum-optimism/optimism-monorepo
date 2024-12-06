// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

// Contracts
import { Faucet } from "src/periphery/faucet/Faucet.sol";

/// @title  IFaucetAuthModule
/// @notice Interface for faucet authentication modules.
interface IFaucetAuthModule {
    /// @notice Verifies that the given drip parameters are valid.
    /// @param _params Drip parameters to verify.
    /// @param _id     Authentication ID to verify.
    /// @param _proof  Authentication proof to verify.
    /// @return valid_ True if the drip parameters are valid.
    function verify(
        Faucet.DripParameters memory _params,
        bytes32 _id,
        bytes memory _proof
    )
        external
        view
        returns (bool);
}
