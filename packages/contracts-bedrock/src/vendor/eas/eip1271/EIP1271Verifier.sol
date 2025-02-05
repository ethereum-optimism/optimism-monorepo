// SPDX-License-Identifier: MIT
pragma solidity 0.8.19;

import { EIP712 } from "@openzeppelin/contracts/utils/cryptography/draft-EIP712.sol";
import { SignatureChecker } from "@openzeppelin/contracts/utils/cryptography/SignatureChecker.sol";
import { Address } from "@openzeppelin/contracts/utils/Address.sol";

import {
    AttestationRequestData,
    DelegatedAttestationRequest,
    DelegatedRevocationRequest,
    RevocationRequestData
} from "src/vendor/eas/IEAS.sol";

import {
    DeadlineExpired,
    NO_EXPIRATION_TIME,
    Signature,
    InvalidSignature,
    MAX_GAP,
    stringToBytes32,
    bytes32ToString
} from "src/vendor/eas/Common.sol";

/// @title EIP1271Verifier
/// @notice EIP1271Verifier typed signatures verifier for EAS delegated attestations.
abstract contract EIP1271Verifier is EIP712 {
    using Address for address;

    error InvalidNonce();

    // The hash of the data type used to relay calls to the attest function. It's the value of
    // keccak256("Attest(address attester,bytes32 schema,address recipient,uint64 expirationTime,bool revocable,bytes32
    // refUID,bytes data,uint256 value,uint256 nonce,uint64 deadline)").
    bytes32 private constant ATTEST_TYPEHASH = 0xfeb2925a02bae3dae48d424a0437a2b6ac939aa9230ddc55a1a76f065d988076;

    // The hash of the data type used to relay calls to the revoke function. It's the value of
    // keccak256("Revoke(address revoker,bytes32 schema,bytes32 uid,uint256 value,uint256 nonce,uint64 deadline)").
    bytes32 private constant REVOKE_TYPEHASH = 0xb5d556f07587ec0f08cf386545cc4362c702a001650c2058002615ee5c9d1e75;

    // The user readable name of the signing domain.
    bytes32 private immutable _name;

    // Replay protection nonces.
    mapping(address => uint256) private _nonces;

    // Upgrade forward-compatibility storage gap
    uint256[MAX_GAP - 1] private __gap;

    /// @dev Emitted when users invalidate nonces by increasing their nonces to (higher) new values.
    /// @param oldNonce The previous nonce.
    /// @param newNonce The new value.
    event NonceIncreased(uint256 oldNonce, uint256 newNonce);

    /// @dev Creates a new EIP1271Verifier instance.
    /// @param version The current major version of the signing domain
    constructor(string memory name, string memory version) EIP712(name, version) {
        _name = stringToBytes32(name);
    }

    /// @notice Returns the domain separator used in the encoding of the signatures for attest, and revoke.
    /// @return The domain separator used in the encoding of the signatures for attest, and revoke.
    function getDomainSeparator() external view returns (bytes32) {
        return _domainSeparatorV4();
    }

    /// @notice Returns the current nonce per-account.
    /// @param account The requested account.
    /// @return The current nonce.
    function getNonce(address account) external view returns (uint256) {
        return _nonces[account];
    }

    /// @notice Returns the EIP712 type hash for the attest function.
    /// @return The EIP712 type hash for the attest function.
    function getAttestTypeHash() external pure returns (bytes32) {
        return ATTEST_TYPEHASH;
    }

    /// @notice Returns the EIP712 type hash for the revoke function.
    /// @return The EIP712 type hash for the revoke function.
    function getRevokeTypeHash() external pure returns (bytes32) {
        return REVOKE_TYPEHASH;
    }

    /// @notice Returns the EIP712 name.
    /// @return The EIP712 name.
    function getName() external view returns (string memory) {
        return bytes32ToString(_name);
    }

    /// @notice Provides users an option to invalidate nonces by increasing their nonces to (higher) new values.
    /// @param newNonce The (higher) new value.
    function increaseNonce(uint256 newNonce) external {
        uint256 oldNonce = _nonces[msg.sender];
        if (newNonce <= oldNonce) {
            revert InvalidNonce();
        }

        _nonces[msg.sender] = newNonce;

        emit NonceIncreased({ oldNonce: oldNonce, newNonce: newNonce });
    }

    /// @notice Verifies delegated attestation request.
    /// @param request The arguments of the delegated attestation request.
    function _verifyAttest(DelegatedAttestationRequest memory request) internal {
        if (request.deadline != NO_EXPIRATION_TIME && request.deadline < _time()) {
            revert DeadlineExpired();
        }

        AttestationRequestData memory data = request.data;
        Signature memory signature = request.signature;

        bytes32 hash = _hashTypedDataV4(
            keccak256(
                abi.encode(
                    ATTEST_TYPEHASH,
                    request.attester,
                    request.schema,
                    data.recipient,
                    data.expirationTime,
                    data.revocable,
                    data.refUID,
                    keccak256(data.data),
                    data.value,
                    _nonces[request.attester]++,
                    request.deadline
                )
            )
        );
        if (
            !SignatureChecker.isValidSignatureNow(
                request.attester, hash, abi.encodePacked(signature.r, signature.s, signature.v)
            )
        ) {
            revert InvalidSignature();
        }
    }

    /// @notice Verifies delegated revocation request.
    /// @param request The arguments of the delegated revocation request.
    function _verifyRevoke(DelegatedRevocationRequest memory request) internal {
        if (request.deadline != NO_EXPIRATION_TIME && request.deadline < _time()) {
            revert DeadlineExpired();
        }

        RevocationRequestData memory data = request.data;
        Signature memory signature = request.signature;

        bytes32 hash = _hashTypedDataV4(
            keccak256(
                abi.encode(
                    REVOKE_TYPEHASH,
                    request.revoker,
                    request.schema,
                    data.uid,
                    data.value,
                    _nonces[request.revoker]++,
                    request.deadline
                )
            )
        );
        if (
            !SignatureChecker.isValidSignatureNow(
                request.revoker, hash, abi.encodePacked(signature.r, signature.s, signature.v)
            )
        ) {
            revert InvalidSignature();
        }
    }

    /// @dev Returns the current's block timestamp. This method is overridden during tests and used to simulate the
    ///     current block time.
    function _time() internal view virtual returns (uint64) {
        return uint64(block.timestamp);
    }
}
