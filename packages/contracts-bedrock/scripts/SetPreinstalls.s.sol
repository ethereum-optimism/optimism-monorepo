// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Script } from "forge-std/Script.sol";
import { console2 as console } from "forge-std/console2.sol";

import { Preinstalls } from "src/libraries/Preinstalls.sol";

/// @title SetPreinstalls
/// @notice Sets all preinstalls in the VM state. There is no default "run()" entrypoint,
/// as this is used in L2Genesis.s.sol, and standalone in the Go test setup for L1 state.
contract SetPreinstalls is Script {
    /// @notice Sets all the preinstalls.
    ///         Warning: the creator-accounts of the preinstall contracts have 0 nonce values.
    ///         When performing a regular user-initiated contract-creation of a preinstall,
    ///         the creation will fail (but nonce will be bumped and not blocked).
    ///         The preinstalls themselves are all inserted with a nonce of 1, reflecting regular user execution.
    function setPreinstalls() public {
        _setPreinstallCode(Preinstalls.MultiCall3);
        _setPreinstallCode(Preinstalls.Create2Deployer);
        _setPreinstallCode(Preinstalls.Safe_v130);
        _setPreinstallCode(Preinstalls.SafeL2_v130);
        _setPreinstallCode(Preinstalls.MultiSendCallOnly_v130);
        _setPreinstallCode(Preinstalls.SafeSingletonFactory);
        _setPreinstallCode(Preinstalls.DeterministicDeploymentProxy);
        _setPreinstallCode(Preinstalls.MultiSend_v130);
        _setPreinstallCode(Preinstalls.Permit2);
        _setPreinstallCode(Preinstalls.SenderCreator_v060); // ERC 4337 v0.6.0
        _setPreinstallCode(Preinstalls.EntryPoint_v060); // ERC 4337 v0.6.0
        _setPreinstallCode(Preinstalls.SenderCreator_v070); // ERC 4337 v0.7.0
        _setPreinstallCode(Preinstalls.EntryPoint_v070); // ERC 4337 v0.7.0
        _setPreinstallCode(Preinstalls.BeaconBlockRoots);
        _setPreinstallCode(Preinstalls.HistoryStorage); // EIP-2935
        _setPreinstallCode(Preinstalls.CreateX);
        // 4788 sender nonce must be incremented, since it's part of later upgrade-transactions.
        // For the upgrade-tx to not create a contract that conflicts with an already-existing copy,
        // the nonce must be bumped.
        vm.setNonce(Preinstalls.BeaconBlockRootsSender, 1);
        vm.setNonce(Preinstalls.HistoryStorageSender, 1);
    }

    /// @notice Sets the bytecode in state
    function _setPreinstallCode(address _addr) internal {
        string memory cname = Preinstalls.getName(_addr);
        console.log("Setting %s preinstall code at: %s", cname, _addr);
        vm.etch(_addr, Preinstalls.getDeployedCode(_addr, block.chainid));
        // during testing in a shared L1/L2 account namespace some preinstalls may already have been inserted and used.
        if (vm.getNonce(_addr) == 0) {
            vm.setNonce(_addr, 1);
        }
    }
}
