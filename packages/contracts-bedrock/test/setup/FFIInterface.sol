// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Types } from "src/libraries/Types.sol";
import { Vm } from "forge-std/Vm.sol";
import { Strings } from "@openzeppelin/contracts/utils/Strings.sol";
import { Process } from "scripts/libraries/Process.sol";

/// @title FFIInterface
/// @notice This contract is set into state using `etch` and therefore must not have constructor logic.
///         It also MUST be compiled with `0.8.15` because `vm.getDeployedCode` will break if there
///         are multiple artifacts for different compiler versions.
contract FFIInterface {
    Vm internal constant vm = Vm(0x7109709ECfa91a80626fF3989D68f67F5b1DD12D);

    function getProveWithdrawalTransactionInputs(Types.WithdrawalTransaction memory _tx)
        external
        returns (bytes32, bytes32, bytes32, bytes32, bytes[] memory)
    {
        string[] memory cmds = new string[](9);
        cmds[0] = "scripts/go-ffi/go-ffi";
        cmds[1] = "diff";
        cmds[2] = "getProveWithdrawalTransactionInputs";
        cmds[3] = vm.toString(_tx.nonce);
        cmds[4] = vm.toString(_tx.sender);
        cmds[5] = vm.toString(_tx.target);
        cmds[6] = vm.toString(_tx.value);
        cmds[7] = vm.toString(_tx.gasLimit);
        cmds[8] = vm.toString(_tx.data);

        bytes memory result = Process.run(cmds);
        (
            bytes32 stateRoot,
            bytes32 storageRoot,
            bytes32 outputRoot,
            bytes32 withdrawalHash,
            bytes[] memory withdrawalProof
        ) = abi.decode(result, (bytes32, bytes32, bytes32, bytes32, bytes[]));

        return (stateRoot, storageRoot, outputRoot, withdrawalHash, withdrawalProof);
    }

    function hashCrossDomainMessage(
        uint256 _nonce,
        address _sender,
        address _target,
        uint256 _value,
        uint256 _gasLimit,
        bytes memory _data
    )
        external
        returns (bytes32)
    {
        string[] memory cmds = new string[](9);
        cmds[0] = "scripts/go-ffi/go-ffi";
        cmds[1] = "diff";
        cmds[2] = "hashCrossDomainMessage";
        cmds[3] = vm.toString(_nonce);
        cmds[4] = vm.toString(_sender);
        cmds[5] = vm.toString(_target);
        cmds[6] = vm.toString(_value);
        cmds[7] = vm.toString(_gasLimit);
        cmds[8] = vm.toString(_data);

        bytes memory result = Process.run(cmds);
        return abi.decode(result, (bytes32));
    }

    function hashWithdrawal(
        uint256 _nonce,
        address _sender,
        address _target,
        uint256 _value,
        uint256 _gasLimit,
        bytes memory _data
    )
        external
        returns (bytes32)
    {
        string[] memory cmds = new string[](9);
        cmds[0] = "scripts/go-ffi/go-ffi";
        cmds[1] = "diff";
        cmds[2] = "hashWithdrawal";
        cmds[3] = vm.toString(_nonce);
        cmds[4] = vm.toString(_sender);
        cmds[5] = vm.toString(_target);
        cmds[6] = vm.toString(_value);
        cmds[7] = vm.toString(_gasLimit);
        cmds[8] = vm.toString(_data);

        bytes memory result = Process.run(cmds);
        return abi.decode(result, (bytes32));
    }

    function hashOutputRootProof(
        bytes32 _version,
        bytes32 _stateRoot,
        bytes32 _messagePasserStorageRoot,
        bytes32 _latestBlockhash
    )
        external
        returns (bytes32)
    {
        string[] memory cmds = new string[](7);
        cmds[0] = "scripts/go-ffi/go-ffi";
        cmds[1] = "diff";
        cmds[2] = "hashOutputRootProof";
        cmds[3] = Strings.toHexString(uint256(_version));
        cmds[4] = Strings.toHexString(uint256(_stateRoot));
        cmds[5] = Strings.toHexString(uint256(_messagePasserStorageRoot));
        cmds[6] = Strings.toHexString(uint256(_latestBlockhash));

        bytes memory result = Process.run(cmds);
        return abi.decode(result, (bytes32));
    }

    function hashDepositTransaction(
        address _from,
        address _to,
        uint256 _mint,
        uint256 _value,
        uint64 _gas,
        bytes memory _data,
        uint64 _logIndex
    )
        external
        returns (bytes32)
    {
        string[] memory cmds = new string[](11);
        cmds[0] = "scripts/go-ffi/go-ffi";
        cmds[1] = "diff";
        cmds[2] = "hashDepositTransaction";
        cmds[3] = "0x0000000000000000000000000000000000000000000000000000000000000000";
        cmds[4] = vm.toString(_logIndex);
        cmds[5] = vm.toString(_from);
        cmds[6] = vm.toString(_to);
        cmds[7] = vm.toString(_mint);
        cmds[8] = vm.toString(_value);
        cmds[9] = vm.toString(_gas);
        cmds[10] = vm.toString(_data);

        bytes memory result = Process.run(cmds);
        return abi.decode(result, (bytes32));
    }

    function encodeDepositTransaction(Types.UserDepositTransaction calldata txn) external returns (bytes memory) {
        string[] memory cmds = new string[](12);
        cmds[0] = "scripts/go-ffi/go-ffi";
        cmds[1] = "diff";
        cmds[2] = "encodeDepositTransaction";
        cmds[3] = vm.toString(txn.from);
        cmds[4] = vm.toString(txn.to);
        cmds[5] = vm.toString(txn.value);
        cmds[6] = vm.toString(txn.mint);
        cmds[7] = vm.toString(txn.gasLimit);
        cmds[8] = vm.toString(txn.isCreation);
        cmds[9] = vm.toString(txn.data);
        cmds[10] = vm.toString(txn.l1BlockHash);
        cmds[11] = vm.toString(txn.logIndex);

        bytes memory result = Process.run(cmds);
        return abi.decode(result, (bytes));
    }

    function encodeCrossDomainMessage(
        uint256 _nonce,
        address _sender,
        address _target,
        uint256 _value,
        uint256 _gasLimit,
        bytes memory _data
    )
        external
        returns (bytes memory)
    {
        string[] memory cmds = new string[](9);
        cmds[0] = "scripts/go-ffi/go-ffi";
        cmds[1] = "diff";
        cmds[2] = "encodeCrossDomainMessage";
        cmds[3] = vm.toString(_nonce);
        cmds[4] = vm.toString(_sender);
        cmds[5] = vm.toString(_target);
        cmds[6] = vm.toString(_value);
        cmds[7] = vm.toString(_gasLimit);
        cmds[8] = vm.toString(_data);

        bytes memory result = Process.run(cmds);
        return abi.decode(result, (bytes));
    }

    function decodeVersionedNonce(uint256 nonce) external returns (uint256, uint256) {
        string[] memory cmds = new string[](4);
        cmds[0] = "scripts/go-ffi/go-ffi";
        cmds[1] = "diff";
        cmds[2] = "decodeVersionedNonce";
        cmds[3] = vm.toString(nonce);

        bytes memory result = Process.run(cmds);
        return abi.decode(result, (uint256, uint256));
    }

    function getMerkleTrieFuzzCase(string memory variant)
        external
        returns (bytes32, bytes memory, bytes memory, bytes[] memory)
    {
        string[] memory cmds = new string[](6);
        cmds[0] = "./scripts/go-ffi/go-ffi";
        cmds[1] = "trie";
        cmds[2] = variant;

        return abi.decode(Process.run(cmds), (bytes32, bytes, bytes, bytes[]));
    }

    function getCannonMemoryProof(uint32 pc, uint32 insn) external returns (bytes32, bytes memory) {
        string[] memory cmds = new string[](5);
        cmds[0] = "scripts/go-ffi/go-ffi";
        cmds[1] = "diff";
        cmds[2] = "cannonMemoryProof";
        cmds[3] = vm.toString(pc);
        cmds[4] = vm.toString(insn);
        bytes memory result = Process.run(cmds);
        (bytes32 memRoot, bytes memory proof) = abi.decode(result, (bytes32, bytes));
        return (memRoot, proof);
    }

    function getCannonMemoryProof(
        uint32 pc,
        uint32 insn,
        uint32 memAddr,
        uint32 memVal
    )
        external
        returns (bytes32, bytes memory)
    {
        string[] memory cmds = new string[](7);
        cmds[0] = "scripts/go-ffi/go-ffi";
        cmds[1] = "diff";
        cmds[2] = "cannonMemoryProof";
        cmds[3] = vm.toString(pc);
        cmds[4] = vm.toString(insn);
        cmds[5] = vm.toString(memAddr);
        cmds[6] = vm.toString(memVal);
        bytes memory result = Process.run(cmds);
        (bytes32 memRoot, bytes memory proof) = abi.decode(result, (bytes32, bytes));
        return (memRoot, proof);
    }

    function getCannonMemoryProof(
        uint32 pc,
        uint32 insn,
        uint32 memAddr,
        uint32 memVal,
        uint32 memAddr2,
        uint32 memVal2
    )
        external
        returns (bytes32, bytes memory)
    {
        string[] memory cmds = new string[](9);
        cmds[0] = "scripts/go-ffi/go-ffi";
        cmds[1] = "diff";
        cmds[2] = "cannonMemoryProof";
        cmds[3] = vm.toString(pc);
        cmds[4] = vm.toString(insn);
        cmds[5] = vm.toString(memAddr);
        cmds[6] = vm.toString(memVal);
        cmds[7] = vm.toString(memAddr2);
        cmds[8] = vm.toString(memVal2);
        bytes memory result = Process.run(cmds);
        (bytes32 memRoot, bytes memory proof) = abi.decode(result, (bytes32, bytes));
        return (memRoot, proof);
    }

    function getCannonMemoryProof2(
        uint32 pc,
        uint32 insn,
        uint32 memAddr,
        uint32 memVal,
        uint32 memAddrForProof
    )
        external
        returns (bytes32, bytes memory)
    {
        string[] memory cmds = new string[](8);
        cmds[0] = "scripts/go-ffi/go-ffi";
        cmds[1] = "diff";
        cmds[2] = "cannonMemoryProof2";
        cmds[3] = vm.toString(pc);
        cmds[4] = vm.toString(insn);
        cmds[5] = vm.toString(memAddr);
        cmds[6] = vm.toString(memVal);
        cmds[7] = vm.toString(memAddrForProof);
        bytes memory result = Process.run(cmds);
        (bytes32 memRoot, bytes memory proof) = abi.decode(result, (bytes32, bytes));
        return (memRoot, proof);
    }

    function getCannonMemoryProofWrongLeaf(
        uint32 pc,
        uint32 insn,
        uint32 memAddr,
        uint32 memVal
    )
        external
        returns (bytes32, bytes memory)
    {
        string[] memory cmds = new string[](7);
        cmds[0] = "scripts/go-ffi/go-ffi";
        cmds[1] = "diff";
        cmds[2] = "cannonMemoryProofWrongLeaf";
        cmds[3] = vm.toString(pc);
        cmds[4] = vm.toString(insn);
        cmds[5] = vm.toString(memAddr);
        cmds[6] = vm.toString(memVal);
        bytes memory result = Process.run(cmds);
        (bytes32 memRoot, bytes memory proof) = abi.decode(result, (bytes32, bytes));
        return (memRoot, proof);
    }

    function getCannonMemory64Proof(uint64 addr, uint64 value) external returns (bytes32, bytes memory) {
        string[] memory cmds = new string[](5);
        cmds[0] = "scripts/go-ffi/go-ffi-cannon64";
        cmds[1] = "diff";
        cmds[2] = "cannonMemoryProof";
        cmds[3] = vm.toString(addr);
        cmds[4] = vm.toString(value);
        bytes memory result = Process.run(cmds);
        (bytes32 memRoot, bytes memory proof) = abi.decode(result, (bytes32, bytes));
        return (memRoot, proof);
    }

    function getCannonMemory64Proof(
        uint64 addr0,
        uint64 value0,
        uint64 addr1,
        uint64 value1
    )
        external
        returns (bytes32, bytes memory)
    {
        string[] memory cmds = new string[](7);
        cmds[0] = "scripts/go-ffi/go-ffi-cannon64";
        cmds[1] = "diff";
        cmds[2] = "cannonMemoryProof";
        cmds[3] = vm.toString(addr0);
        cmds[4] = vm.toString(value0);
        cmds[5] = vm.toString(addr1);
        cmds[6] = vm.toString(value1);
        bytes memory result = Process.run(cmds);
        (bytes32 memRoot, bytes memory proof) = abi.decode(result, (bytes32, bytes));
        return (memRoot, proof);
    }

    function getCannonMemory64Proof(
        uint64 addr0,
        uint64 value0,
        uint64 addr1,
        uint64 value1,
        uint64 memAddr2,
        uint64 memVal2
    )
        external
        returns (bytes32, bytes memory)
    {
        string[] memory cmds = new string[](9);
        cmds[0] = "scripts/go-ffi/go-ffi-cannon64";
        cmds[1] = "diff";
        cmds[2] = "cannonMemoryProof";
        cmds[3] = vm.toString(addr0);
        cmds[4] = vm.toString(value0);
        cmds[5] = vm.toString(addr1);
        cmds[6] = vm.toString(value1);
        cmds[7] = vm.toString(memAddr2);
        cmds[8] = vm.toString(memVal2);
        bytes memory result = Process.run(cmds);
        (bytes32 memRoot, bytes memory proof) = abi.decode(result, (bytes32, bytes));
        return (memRoot, proof);
    }

    function getCannonMemory64Proof2(
        uint64 addr0,
        uint64 value0,
        uint64 addr1,
        uint64 value1,
        uint64 memAddrForProof
    )
        external
        returns (bytes32, bytes memory)
    {
        string[] memory cmds = new string[](8);
        cmds[0] = "scripts/go-ffi/go-ffi-cannon64";
        cmds[1] = "diff";
        cmds[2] = "cannonMemoryProof2";
        cmds[3] = vm.toString(addr0);
        cmds[4] = vm.toString(value0);
        cmds[5] = vm.toString(addr1);
        cmds[6] = vm.toString(value1);
        cmds[7] = vm.toString(memAddrForProof);
        bytes memory result = Process.run(cmds);
        (bytes32 memRoot, bytes memory proof) = abi.decode(result, (bytes32, bytes));
        return (memRoot, proof);
    }

    function encodeScalarEcotone(uint32 _basefeeScalar, uint32 _blobbasefeeScalar) external returns (bytes32) {
        string[] memory cmds = new string[](5);
        cmds[0] = "scripts/go-ffi/go-ffi";
        cmds[1] = "diff";
        cmds[2] = "encodeScalarEcotone";
        cmds[3] = vm.toString(_basefeeScalar);
        cmds[4] = vm.toString(_blobbasefeeScalar);
        bytes memory result = Process.run(cmds);
        return abi.decode(result, (bytes32));
    }

    function decodeScalarEcotone(bytes32 _scalar) external returns (uint32, uint32) {
        string[] memory cmds = new string[](4);
        cmds[0] = "scripts/go-ffi/go-ffi";
        cmds[1] = "diff";
        cmds[2] = "decodeScalarEcotone";
        cmds[3] = vm.toString(_scalar);
        bytes memory result = Process.run(cmds);
        return abi.decode(result, (uint32, uint32));
    }

    function encodeGasPayingToken(
        address _token,
        uint8 _decimals,
        bytes32 _name,
        bytes32 _symbol
    )
        external
        returns (bytes memory)
    {
        string[] memory cmds = new string[](7);
        cmds[0] = "scripts/go-ffi/go-ffi";
        cmds[1] = "diff";
        cmds[2] = "encodeGasPayingToken";
        cmds[3] = vm.toString(_token);
        cmds[4] = vm.toString(_decimals);
        cmds[5] = vm.toString(_name);
        cmds[6] = vm.toString(_symbol);

        bytes memory result = Process.run(cmds);
        return abi.decode(result, (bytes));
    }

    function encodeDependency(uint256 _chainId) external returns (bytes memory) {
        string[] memory cmds = new string[](4);
        cmds[0] = "scripts/go-ffi/go-ffi";
        cmds[1] = "diff";
        cmds[2] = "encodeDependency";
        cmds[3] = vm.toString(_chainId);

        bytes memory result = Process.run(cmds);
        return abi.decode(result, (bytes));
    }
}
