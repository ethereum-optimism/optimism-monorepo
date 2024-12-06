// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

// Contracts
import { Ownable } from "@openzeppelin/contracts/access/Ownable.sol";

// Libraries
import { AddressAliasHelper } from "src/vendor/AddressAliasHelper.sol";

/// @title CrossDomainOwnable
/// @notice This contract extends the OpenZeppelin `Ownable` contract for L2 contracts to be owned
///         by contracts on L1. Note that this contract is only safe to be used if the
///         CrossDomainMessenger system is bypassed and the caller on L1 is calling the
///         OptimismPortal directly.
abstract contract CrossDomainOwnable is Ownable {
    /// @notice Overrides the implementation of the `onlyOwner` modifier to check that the unaliased
    ///         `msg.sender` is the owner of the contract.
    function _checkOwner() internal view override {
        require(
            owner() == AddressAliasHelper.undoL1ToL2Alias(msg.sender), "CrossDomainOwnable: caller is not the owner"
        );
    }
}
