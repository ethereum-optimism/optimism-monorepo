// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

// Contracts
import { Ownable } from "@openzeppelin/contracts/access/Ownable.sol";

// Libraries
import { Predeploys } from "src/libraries/Predeploys.sol";

// Interfaces
import { IL2CrossDomainMessenger } from "interfaces/L2/IL2CrossDomainMessenger.sol";

/// @title CrossDomainOwnable3
/// @notice This contract extends the OpenZeppelin `Ownable` contract for L2 contracts to be owned
///         by contracts on either L1 or L2. Note that this contract is meant to be used with
///         systems that use the CrossDomainMessenger system. It will not work if the
///         OptimismPortal is used directly.
abstract contract CrossDomainOwnable3 is Ownable {
    /// @notice If true, the contract uses the cross domain _checkOwner function override.
    ///         If false it uses the standard Ownable _checkOwner function.
    bool public isLocal = true;

    /// @notice Emits when ownership of the contract is transferred. Includes the
    ///         isLocal field in addition to the standard `Ownable` OwnershipTransferred event.
    /// @param previousOwner The previous owner of the contract.
    /// @param newOwner      The new owner of the contract.
    /// @param isLocal       Configures the `isLocal` contract variable.
    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner, bool isLocal);

    /// @notice Allows for ownership to be transferred with specifying the locality.
    /// @param _owner   The new owner of the contract.
    /// @param _isLocal Configures the locality of the ownership.
    function transferOwnership(address _owner, bool _isLocal) external onlyOwner {
        require(_owner != address(0), "CrossDomainOwnable3: new owner is the zero address");

        address oldOwner = owner();
        _transferOwnership(_owner);
        isLocal = _isLocal;

        emit OwnershipTransferred(oldOwner, _owner, _isLocal);
    }

    /// @notice Overrides the implementation of the `onlyOwner` modifier to check that the unaliased
    ///         `xDomainMessageSender` is the owner of the contract. This value is set to the caller
    ///         of the L1CrossDomainMessenger.
    function _checkOwner() internal view override {
        if (isLocal) {
            require(owner() == msg.sender, "CrossDomainOwnable3: caller is not the owner");
        } else {
            IL2CrossDomainMessenger messenger = IL2CrossDomainMessenger(Predeploys.L2_CROSS_DOMAIN_MESSENGER);

            require(msg.sender == address(messenger), "CrossDomainOwnable3: caller is not the messenger");

            require(owner() == messenger.xDomainMessageSender(), "CrossDomainOwnable3: caller is not the owner");
        }
    }
}
