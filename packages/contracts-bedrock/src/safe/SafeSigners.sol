// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

library SafeSigners {
    /// @notice Splits signature bytes into `uint8 v, bytes32 r, bytes32 s`.
    ///         Copied directly from
    /// https://github.com/safe-global/safe-contracts/blob/e870f514ad34cd9654c72174d6d4a839e3c6639f/contracts/common/SignatureDecoder.sol
    /// @dev Make sure to perform a bounds check for @param pos, to avoid out of bounds access on @param signatures
    ///      The signature format is a compact form of {bytes32 r}{bytes32 s}{uint8 v}
    ///      Compact means uint8 is not padded to 32 bytes.
    /// @param _pos Which signature to read.
    ///            A prior bounds check of this parameter should be performed, to avoid out of bounds access.
    /// @param _signatures Concatenated {r, s, v} signatures.
    /// @return v_ Recovery ID or Safe signature type.
    /// @return r_ Output value r of the signature.
    /// @return s_ Output value s of the signature.
    function signatureSplit(
        bytes memory _signatures,
        uint256 _pos
    )
        internal
        pure
        returns (uint8 v_, bytes32 r_, bytes32 s_)
    {
        assembly {
            let signaturePos := mul(0x41, _pos)
            r_ := mload(add(_signatures, add(signaturePos, 0x20)))
            s_ := mload(add(_signatures, add(signaturePos, 0x40)))
            // Here we are loading the last 32 bytes, including 31 bytes
            // of 's'. There is no 'mload8' to do this.
            // 'byte' is not working due to the Solidity parser, so lets
            // use the second best option, 'and'
            v_ := and(mload(add(_signatures, add(signaturePos, 0x41))), 0xff)
        }
    }

    /// @notice Extract the signers from a set of signatures.
    ///         This method is based closely on the code in the Safe.checkNSignatures() method.
    ///         https://github.com/safe-global/safe-contracts/blob/e870f514ad34cd9654c72174d6d4a839e3c6639f/contracts/Safe.sol#L274
    ///         It has been modified by removing all signature _validation_ code. We trust the Safe to properly validate
    ///         the signatures.
    ///         This method therefore simply extracts the addresses from the signatures.
    function getNSigners(
        bytes32 _dataHash,
        bytes memory _signatures,
        uint256 _requiredSignatures
    )
        internal
        pure
        returns (address[] memory owners_)
    {
        owners_ = new address[](_requiredSignatures);

        address currentOwner;
        uint8 v;
        bytes32 r;
        bytes32 s;
        uint256 i;
        for (i = 0; i < _requiredSignatures; i++) {
            (v, r, s) = signatureSplit(_signatures, i);
            if (v == 0) {
                // If v is 0 then it is a contract signature
                // When handling contract signatures the address of the contract is encoded into r
                currentOwner = address(uint160(uint256(r)));
            } else if (v == 1) {
                // If v is 1 then it is an approved hash
                // When handling approved hashes the address of the approver is encoded into r
                currentOwner = address(uint160(uint256(r)));
            } else if (v > 30) {
                // If v > 30 then default va (27,28) has been adjusted for eth_sign flow
                // To support eth_sign and similar we adjust v and hash the messageHash with the Ethereum message prefix
                // before applying ecrecover
                currentOwner =
                    ecrecover(keccak256(abi.encodePacked("\x19Ethereum Signed Message:\n32", _dataHash)), v - 4, r, s);
            } else {
                // Default is the ecrecover flow with the provided data hash
                // Use ecrecover with the messageHash for EOA signatures
                currentOwner = ecrecover(_dataHash, v, r, s);
            }
            owners_[i] = currentOwner;
        }
    }
}
