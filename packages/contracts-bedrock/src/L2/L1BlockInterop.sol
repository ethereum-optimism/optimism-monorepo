// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { L1Block } from "src/L2/L1Block.sol";

// Libraries
import { Predeploys } from "src/libraries/Predeploys.sol";
import { NotDepositor, NotCrossL2Inbox } from "src/libraries/L1BlockErrors.sol";

/// @custom:proxied true
/// @custom:predeploy 0x4200000000000000000000000000000000000015
/// @title L1BlockInterop
/// @notice Interop extensions of L1Block.
contract L1BlockInterop is L1Block {
    /// @notice Storage slot that the isDeposit is stored at.
    ///         This is a custom slot that is not part of the standard storage layout.
    /// keccak256(abi.encode(uint256(keccak256("l1Block.identifier.isDeposit")) - 1)) & ~bytes32(uint256(0xff))
    uint256 internal constant IS_DEPOSIT_SLOT = 0x921bd3a089295c6e5540e8fba8195448d253efd6f2e3e495b499b627dc36a300;

    /// @custom:semver +interop-beta.5
    function version() public pure override returns (string memory) {
        return string.concat(super.version(), "+interop-beta.5");
    }

    /// @notice Returns whether the call was triggered from a a deposit or not.
    /// @notice This function is only callable by the CrossL2Inbox contract.
    function isDeposit() external view returns (bool isDeposit_) {
        if (msg.sender != Predeploys.CROSS_L2_INBOX) revert NotCrossL2Inbox();
        assembly {
            isDeposit_ := sload(IS_DEPOSIT_SLOT)
        }
    }

    /// @notice Updates the `isDeposit` flag and sets the L1 block values for an Interop upgraded chain.
    ///         It updates the L1 block values through the `setL1BlockValuesEcotone` function.
    ///         It forwards the calldata to the internally-used `setL1BlockValuesEcotone` function.
    function setL1BlockValuesInterop() external {
        // Set the isDeposit flag to true.
        assembly {
            sstore(IS_DEPOSIT_SLOT, 1)
        }

        _setL1BlockValuesEcotone();
    }

    /// @notice Resets the isDeposit flag.
    ///         Should only be called by the depositor account after the deposits are complete.
    function depositsComplete() external {
        if (msg.sender != DEPOSITOR_ACCOUNT()) revert NotDepositor();

        // Set the isDeposit flag to false.
        assembly {
            sstore(IS_DEPOSIT_SLOT, 0)
        }
    }
}
