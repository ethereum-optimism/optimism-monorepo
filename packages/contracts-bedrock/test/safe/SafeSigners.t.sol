// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Test } from "forge-std/Test.sol";
import { GnosisSafe as Safe } from "safe-contracts/GnosisSafe.sol";
import { SafeSigners } from "src/safe/SafeSigners.sol";
import "test/safe-tools/SafeTestTools.sol";

contract SafeSigners_Test is Test, SafeTestTools {
    bytes4 internal constant EIP1271_MAGIC_VALUE = 0x20c13b0b;

    enum SigTypes {
        Eoa,
        EthSign,
        ApprovedHash,
        Contract
    }

    /// @dev Maps every key to one of the 4 signatures types.
    ///      This is used in the tests below as a pseudorandom mechanism for determining which
    ///      signature type to use for each key.
    /// @param _key The key to map to a signature type.
    function sigType(uint256 _key) internal pure returns (SigTypes sigType_) {
        uint256 t = _key % 4;
        sigType_ = SigTypes(t);
    }

    /// @dev Test that for a given set of signatures:
    ///      1. safe.checkNSignatures() succeeds
    ///      2. the getSigners() method returns the expected signers
    ///      3. the expected signers are all owners of the safe.
    ///      Demonstrating these three properties is sufficient to prove that the getSigners() method
    ///      returns the same signatures as those recovered by safe.checkNSignatures().
    function testDiff_getSignaturesVsCheckSignatures_succeeds(bytes memory _data, uint256 _numSigs) external {
        bytes32 digest = keccak256(_data);

        // Limit the number of signatures to 25
        uint256 numSigs = bound(_numSigs, 1, 25);

        (, uint256[] memory keys) = SafeTestLib.makeAddrsAndKeys("getSigsTest", numSigs);
        for (uint256 i; i < keys.length; i++) {
            if (sigType(keys[i]) == SigTypes.Contract) {
                keys[i] =
                    SafeTestLib.encodeSmartContractWalletAsPK(SafeTestLib.decodeSmartContractWalletAsAddress(keys[i]));
            }
        }

        // Create a new safeInstance with M=N, so that it requires a signature from each key.
        SafeInstance memory safeInstance = SafeTestTools._setupSafe(keys, numSigs, 0);

        // Next we will generate signatures by iterating over the keys, and choosing the signature type
        // based on the key.
        uint8 v;
        bytes32 r;
        bytes32 s;
        uint256 contractSigs;
        bytes memory signatures;
        uint256[] memory pks = safeInstance.ownerPKs;
        for (uint256 i; i < pks.length; i++) {
            if (sigType(pks[i]) == SigTypes.Eoa) {
                (v, r, s) = vm.sign(pks[i], digest);
                signatures = bytes.concat(signatures, abi.encodePacked(r, s, v));
            } else if (sigType(pks[i]) == SigTypes.EthSign) {
                (v, r, s) = vm.sign(pks[i], keccak256(abi.encodePacked("\x19Ethereum Signed Message:\n32", digest)));
                v += 4;
                signatures = bytes.concat(signatures, abi.encodePacked(r, s, v));
            } else if (sigType(pks[i]) == SigTypes.ApprovedHash) {
                vm.prank(SafeTestLib.getAddr(pks[i]));
                safeInstance.safe.approveHash(digest);
                v = 1;
                // s is not checked on approved hash signatures, so we can leave it as zero.
                r = bytes32(uint256(uint160(SafeTestLib.getAddr(pks[i]))));
                signatures = bytes.concat(signatures, abi.encodePacked(r, s, v));
            } else if (sigType(pks[i]) == SigTypes.Contract) {
                contractSigs++;
                address addr = SafeTestLib.decodeSmartContractWalletAsAddress(pks[i]);
                r = bytes32(uint256(uint160(addr)));
                // nosemgrep: sol-style-use-abi-encodecall
                vm.mockCall(
                    addr, abi.encodeWithSignature("isValidSignature(bytes,bytes)"), abi.encode(EIP1271_MAGIC_VALUE)
                );
                v = 0;
                // s needs to point to data that comes after the signatures
                s = bytes32(numSigs * 65);
                signatures = bytes.concat(signatures, abi.encodePacked(r, s, v));
            }
        }

        // For each contract sig, add 64 bytes to the signature data. This is necessary to satisfy
        // the validation checks that the Safe contract performs on the value of s on contract
        // signatures. The Safe contract checks that s correctly points to additional data appended
        // after the signatures, and that the length of the data is within bounds.
        for (uint256 i; i < contractSigs; i++) {
            signatures = bytes.concat(signatures, abi.encode(32, 1));
        }

        // Signature checking on the Safe should succeed.
        safeInstance.safe.checkNSignatures(digest, _data, signatures, numSigs);

        // Recover the signatures using the _getNSigners() method.
        address[] memory gotSigners =
            SafeSigners.getNSigners({ _dataHash: digest, _signatures: signatures, _requiredSignatures: numSigs });

        // Compare the list of recovered signers to the expected signers.
        assertEq(gotSigners.length, numSigs);
        assertEq(gotSigners.length, safeInstance.owners.length);
        for (uint256 i; i < numSigs; i++) {
            assertEq(safeInstance.owners[i], gotSigners[i]);
        }
    }
}
