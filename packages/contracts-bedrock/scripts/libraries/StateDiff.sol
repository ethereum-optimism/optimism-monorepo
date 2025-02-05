// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { stdJson } from "forge-std/StdJson.sol";
import { VmSafe } from "forge-std/Vm.sol";

/// @title StateDiff
/// @author refcell
/// @notice Library to write StateDiff output to json.
library StateDiff {
    /// @notice Accepts an array of AccountAccess structs from the Vm and encodes them as a json string.
    /// @param _accountAccesses Array of AccountAccess structs.
    /// @return serialized_ string
    function encodeAccountAccesses(VmSafe.AccountAccess[] memory _accountAccesses)
        internal
        returns (string memory serialized_)
    {
        string[] memory accountAccesses = new string[](_accountAccesses.length);
        for (uint256 i = 0; i < _accountAccesses.length; i++) {
            accountAccesses[i] = serializeAccountAccess(_accountAccesses[i]);
        }
        serialized_ = stdJson.serialize("accountAccessElem", "accountAccesses", accountAccesses);
    }

    /// @notice Turns an AccountAccess into a json serialized string
    /// @param _accountAccess The AccountAccess to serialize
    /// @return serialized_ The json serialized string
    function serializeAccountAccess(VmSafe.AccountAccess memory _accountAccess)
        internal
        returns (string memory serialized_)
    {
        string memory json = "";
        json = stdJson.serialize("accountAccess", "chainInfo", serializeChainInfo(_accountAccess.chainInfo));
        json = stdJson.serialize("accountAccess", "kind", serializeAccountAccessKind(_accountAccess.kind));
        json = stdJson.serialize("accountAccess", "account", _accountAccess.account);
        json = stdJson.serialize("accountAccess", "accessor", _accountAccess.accessor);
        json = stdJson.serialize("accountAccess", "initialized", _accountAccess.initialized);
        json = stdJson.serialize("accountAccess", "oldBalance", _accountAccess.oldBalance);
        json = stdJson.serialize("accountAccess", "newBalance", _accountAccess.newBalance);
        json = stdJson.serialize("accountAccess", "deployedCode", _accountAccess.deployedCode);
        json = stdJson.serialize("accountAccess", "value", _accountAccess.value);
        json = stdJson.serialize("accountAccess", "data", _accountAccess.data);
        json = stdJson.serialize("accountAccess", "reverted", _accountAccess.reverted);
        json = stdJson.serialize(
            "accountAccess", "storageAccesses", serializeStorageAccesses(_accountAccess.storageAccesses)
        );
        serialized_ = json;
    }

    /// @notice Accepts a VmSafe.ChainInfo struct and encodes it as a json string.
    /// @param _chainInfo The ChainInfo struct to serialize
    /// @return serialized_ string
    function serializeChainInfo(VmSafe.ChainInfo memory _chainInfo) internal returns (string memory serialized_) {
        string memory json = "";
        json = stdJson.serialize("chainInfo", "forkId", _chainInfo.forkId);
        json = stdJson.serialize("chainInfo", "chainId", _chainInfo.chainId);
        serialized_ = json;
    }

    /// @notice Turns an AccountAccessKind into a string.
    /// @param _kind The AccountAccessKind to serialize
    /// @return serialized_ The string representation of the AccountAccessKind
    function serializeAccountAccessKind(VmSafe.AccountAccessKind _kind)
        internal
        pure
        returns (string memory serialized_)
    {
        if (_kind == VmSafe.AccountAccessKind.Call) {
            serialized_ = "Call";
        } else if (_kind == VmSafe.AccountAccessKind.DelegateCall) {
            serialized_ = "DelegateCall";
        } else if (_kind == VmSafe.AccountAccessKind.CallCode) {
            serialized_ = "CallCode";
        } else if (_kind == VmSafe.AccountAccessKind.StaticCall) {
            serialized_ = "StaticCall";
        } else if (_kind == VmSafe.AccountAccessKind.Create) {
            serialized_ = "Create";
        } else if (_kind == VmSafe.AccountAccessKind.SelfDestruct) {
            serialized_ = "SelfDestruct";
        } else {
            serialized_ = "Resume";
        }
    }

    /// @notice Accepts an array of StorageAccess structs from the Vm and encodes each as a json string.
    /// @param _storageAccesses Array of StorageAccess structs.
    /// @return serialized_ The list of json serialized StorageAccess structs.
    function serializeStorageAccesses(VmSafe.StorageAccess[] memory _storageAccesses)
        internal
        returns (string[] memory serialized_)
    {
        serialized_ = new string[](_storageAccesses.length);
        for (uint256 i = 0; i < _storageAccesses.length; i++) {
            serialized_[i] = serializeStorageAccess(_storageAccesses[i]);
        }
    }

    /// @notice Turns a StorageAccess into a json serialized string
    /// @param _storageAccess The StorageAccess to serialize
    /// @return serialized_ The json serialized string
    function serializeStorageAccess(VmSafe.StorageAccess memory _storageAccess)
        internal
        returns (string memory serialized_)
    {
        string memory json = "";
        json = stdJson.serialize("storageAccess", "account", _storageAccess.account);
        json = stdJson.serialize("storageAccess", "slot", _storageAccess.slot);
        json = stdJson.serialize("storageAccess", "isWrite", _storageAccess.isWrite);
        json = stdJson.serialize("storageAccess", "previousValue", _storageAccess.previousValue);
        json = stdJson.serialize("storageAccess", "newValue", _storageAccess.newValue);
        json = stdJson.serialize("storageAccess", "reverted", _storageAccess.reverted);
        serialized_ = json;
    }
}
