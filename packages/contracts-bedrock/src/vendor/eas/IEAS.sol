// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { ISchemaRegistry } from "src/vendor/eas/ISchemaRegistry.sol";
import { Attestation, Signature } from "src/vendor/eas/Common.sol";

/// @dev A struct representing the arguments of the attestation request.
struct AttestationRequestData {
    address recipient; // The recipient of the attestation.
    uint64 expirationTime; // The time when the attestation expires (Unix timestamp).
    bool revocable; // Whether the attestation is revocable.
    bytes32 refUID; // The UID of the related attestation.
    bytes data; // Custom attestation data.
    uint256 value; // An explicit ETH amount to send to the resolver. This is important to prevent accidental user
        // errors.
}

/// @dev A struct representing the full arguments of the attestation request.
struct AttestationRequest {
    bytes32 schema; // The unique identifier of the schema.
    AttestationRequestData data; // The arguments of the attestation request.
}

/// @dev A struct representing the full arguments of the full delegated attestation request.
struct DelegatedAttestationRequest {
    bytes32 schema; // The unique identifier of the schema.
    AttestationRequestData data; // The arguments of the attestation request.
    Signature signature; // The ECDSA signature data.
    address attester; // The attesting account.
    uint64 deadline; // The deadline of the signature/request.
}

/// @dev A struct representing the full arguments of the multi attestation request.
struct MultiAttestationRequest {
    bytes32 schema; // The unique identifier of the schema.
    AttestationRequestData[] data; // The arguments of the attestation request.
}

/// @dev A struct representing the full arguments of the delegated multi attestation request.
struct MultiDelegatedAttestationRequest {
    bytes32 schema; // The unique identifier of the schema.
    AttestationRequestData[] data; // The arguments of the attestation requests.
    Signature[] signatures; // The ECDSA signatures data. Please note that the signatures are assumed to be signed with
        // increasing nonces.
    address attester; // The attesting account.
    uint64 deadline; // The deadline of the signature/request.
}

/// @dev A struct representing the arguments of the revocation request.
struct RevocationRequestData {
    bytes32 uid; // The UID of the attestation to revoke.
    uint256 value; // An explicit ETH amount to send to the resolver. This is important to prevent accidental user
        // errors.
}

/// @dev A struct representing the full arguments of the revocation request.
struct RevocationRequest {
    bytes32 schema; // The unique identifier of the schema.
    RevocationRequestData data; // The arguments of the revocation request.
}

/// @dev A struct representing the arguments of the full delegated revocation request.
struct DelegatedRevocationRequest {
    bytes32 schema; // The unique identifier of the schema.
    RevocationRequestData data; // The arguments of the revocation request.
    Signature signature; // The ECDSA signature data.
    address revoker; // The revoking account.
    uint64 deadline; // The deadline of the signature/request.
}

/// @dev A struct representing the full arguments of the multi revocation request.
struct MultiRevocationRequest {
    bytes32 schema; // The unique identifier of the schema.
    RevocationRequestData[] data; // The arguments of the revocation request.
}

/// @dev A struct representing the full arguments of the delegated multi revocation request.
struct MultiDelegatedRevocationRequest {
    bytes32 schema; // The unique identifier of the schema.
    RevocationRequestData[] data; // The arguments of the revocation requests.
    Signature[] signatures; // The ECDSA signatures data. Please note that the signatures are assumed to be signed with
        // increasing nonces.
    address revoker; // The revoking account.
    uint64 deadline; // The deadline of the signature/request.
}

/// @title IEAS
/// @notice The Ethereum Attestation Service interface.
interface IEAS {
    /// @dev Emitted when an attestation has been made.
    /// @param recipient The recipient of the attestation.
    /// @param attester The attesting account.
    /// @param uid The UID the revoked attestation.
    /// @param schemaUID The UID of the schema.
    event Attested(address indexed recipient, address indexed attester, bytes32 uid, bytes32 indexed schemaUID);

    /// @dev Emitted when an attestation has been revoked.
    /// @param recipient The recipient of the attestation.
    /// @param attester The attesting account.
    /// @param schemaUID The UID of the schema.
    /// @param uid The UID the revoked attestation.
    event Revoked(address indexed recipient, address indexed attester, bytes32 uid, bytes32 indexed schemaUID);

    /// @dev Emitted when a data has been timestamped.
    /// @param data The data.
    /// @param timestamp The timestamp.
    event Timestamped(bytes32 indexed data, uint64 indexed timestamp);

    /// @dev Emitted when a data has been revoked.
    /// @param revoker The address of the revoker.
    /// @param data The data.
    /// @param timestamp The timestamp.
    event RevokedOffchain(address indexed revoker, bytes32 indexed data, uint64 indexed timestamp);

    /// @notice Returns the address of the global schema registry.
    /// @return The address of the global schema registry.
    function getSchemaRegistry() external view returns (ISchemaRegistry);

    /// @notice Attests to a specific schema.
    ///
    ///      Example:
    ///
    ///      attest({
    ///         schema: "0facc36681cbe2456019c1b0d1e7bedd6d1d40f6f324bf3dd3a4cef2999200a0",
    ///         data: {
    ///             recipient: "0xdEADBeAFdeAdbEafdeadbeafDeAdbEAFdeadbeaf",
    ///             expirationTime: 0,
    ///             revocable: true,
    ///             refUID: "0x0000000000000000000000000000000000000000000000000000000000000000",
    ///             data: "0xF00D",
    ///             value: 0
    ///         }
    ///      })
    ///
    /// @param request The arguments of the attestation request.
    /// @return The UID of the new attestation.
    function attest(AttestationRequest calldata request) external payable returns (bytes32);

    /// @notice Attests to a specific schema via the provided EIP712 signature.
    ///
    ///         Example:
    ///
    ///         attestByDelegation({
    ///             schema: '0x8e72f5bc0a8d4be6aa98360baa889040c50a0e51f32dbf0baa5199bd93472ebc',
    ///             data: {
    ///                 recipient: '0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266',
    ///                 expirationTime: 1673891048,
    ///                 revocable: true,
    ///                 refUID: '0x0000000000000000000000000000000000000000000000000000000000000000',
    ///                 data: '0x1234',
    ///                 value: 0
    ///             },
    ///             signature: {
    ///                 v: 28,
    ///                 r: '0x148c...b25b',
    ///                 s: '0x5a72...be22'
    ///             },
    ///             attester: '0xc5E8740aD971409492b1A63Db8d83025e0Fc427e',
    ///             deadline: 1673891048
    ///         })
    ///
    /// @param delegatedRequest The arguments of the delegated attestation request.
    /// @return The UID of the new attestation.
    function attestByDelegation(DelegatedAttestationRequest calldata delegatedRequest)
        external
        payable
        returns (bytes32);

    /// @notice Attests to multiple schemas.
    ///
    ///         Example:
    ///
    ///         multiAttest([{
    ///             schema: '0x33e9094830a5cba5554d1954310e4fbed2ef5f859ec1404619adea4207f391fd',
    ///             data: [{
    ///                 recipient: '0xdEADBeAFdeAdbEafdeadbeafDeAdbEAFdeadbeaf',
    ///                 expirationTime: 1673891048,
    ///                 revocable: true,
    ///                 refUID: '0x0000000000000000000000000000000000000000000000000000000000000000',
    ///                 data: '0x1234',
    ///                 value: 1000
    ///             },
    ///             {
    ///                 recipient: '0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266',
    ///                 expirationTime: 0,
    ///                 revocable: false,
    ///                 refUID: '0x480df4a039efc31b11bfdf491b383ca138b6bde160988222a2a3509c02cee174',
    ///                 data: '0x00',
    ///                 value: 0
    ///             }],
    ///         },
    ///         {
    ///             schema: '0x5ac273ce41e3c8bfa383efe7c03e54c5f0bff29c9f11ef6ffa930fc84ca32425',
    ///             data: [{
    ///                 recipient: '0xdEADBeAFdeAdbEafdeadbeafDeAdbEAFdeadbeaf',
    ///                 expirationTime: 0,
    ///                 revocable: true,
    ///                 refUID: '0x75bf2ed8dca25a8190c50c52db136664de25b2449535839008ccfdab469b214f',
    ///                 data: '0x12345678',
    ///                 value: 0
    ///             },
    ///         }])
    ///
    /// @param multiRequests The arguments of the multi attestation requests. The requests should be grouped by distinct
    ///        schema ids to benefit from the best batching optimization.
    /// @return The UIDs of the new attestations.
    function multiAttest(MultiAttestationRequest[] calldata multiRequests)
        external
        payable
        returns (bytes32[] memory);

    /// @notice Attests to multiple schemas using via provided EIP712 signatures.
    ///
    ///         Example:
    ///
    ///         multiAttestByDelegation([{
    ///             schema: '0x8e72f5bc0a8d4be6aa98360baa889040c50a0e51f32dbf0baa5199bd93472ebc',
    ///             data: [{
    ///                 recipient: '0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266',
    ///                 expirationTime: 1673891048,
    ///                 revocable: true,
    ///                 refUID: '0x0000000000000000000000000000000000000000000000000000000000000000',
    ///                 data: '0x1234',
    ///                 value: 0
    ///             },
    ///             {
    ///                 recipient: '0xdEADBeAFdeAdbEafdeadbeafDeAdbEAFdeadbeaf',
    ///                 expirationTime: 0,
    ///                 revocable: false,
    ///                 refUID: '0x0000000000000000000000000000000000000000000000000000000000000000',
    ///                 data: '0x00',
    ///                 value: 0
    ///             }],
    ///             signatures: [{
    ///                 v: 28,
    ///                 r: '0x148c...b25b',
    ///                 s: '0x5a72...be22'
    ///             },
    ///             {
    ///                 v: 28,
    ///                 r: '0x487s...67bb',
    ///                 s: '0x12ad...2366'
    ///             }],
    ///             attester: '0x1D86495b2A7B524D747d2839b3C645Bed32e8CF4',
    ///             deadline: 1673891048
    ///         }])
    ///
    /// @param multiDelegatedRequests The arguments of the delegated multi attestation requests. The requests should be
    ///        grouped by distinct schema ids to benefit from the best batching optimization.
    /// @return The UIDs of the new attestations.
    function multiAttestByDelegation(MultiDelegatedAttestationRequest[] calldata multiDelegatedRequests)
        external
        payable
        returns (bytes32[] memory);

    /// @notice Revokes an existing attestation to a specific schema.
    ///
    ///         Example:
    ///
    ///         revoke({
    ///             schema: '0x8e72f5bc0a8d4be6aa98360baa889040c50a0e51f32dbf0baa5199bd93472ebc',
    ///             data: {
    ///                 uid: '0x101032e487642ee04ee17049f99a70590c735b8614079fc9275f9dd57c00966d',
    ///                 value: 0
    ///             }
    ///         })
    ///
    /// @param request The arguments of the revocation request.
    function revoke(RevocationRequest calldata request) external payable;

    /// @notice Revokes an existing attestation to a specific schema via the provided EIP712 signature.
    ///
    ///         Example:
    ///
    ///         revokeByDelegation({
    ///             schema: '0x8e72f5bc0a8d4be6aa98360baa889040c50a0e51f32dbf0baa5199bd93472ebc',
    ///             data: {
    ///                 uid: '0xcbbc12102578c642a0f7b34fe7111e41afa25683b6cd7b5a14caf90fa14d24ba',
    ///                 value: 0
    ///             },
    ///             signature: {
    ///                 v: 27,
    ///                 r: '0xb593...7142',
    ///                 s: '0x0f5b...2cce'
    ///             },
    ///             revoker: '0x244934dd3e31bE2c81f84ECf0b3E6329F5381992',
    ///             deadline: 1673891048
    ///         })
    ///
    /// @param delegatedRequest The arguments of the delegated revocation request.
    function revokeByDelegation(DelegatedRevocationRequest calldata delegatedRequest) external payable;

    /// @notice Revokes existing attestations to multiple schemas.
    ///
    ///         Example:
    ///
    ///         multiRevoke([{
    ///             schema: '0x8e72f5bc0a8d4be6aa98360baa889040c50a0e51f32dbf0baa5199bd93472ebc',
    ///             data: [{
    ///                 uid: '0x211296a1ca0d7f9f2cfebf0daaa575bea9b20e968d81aef4e743d699c6ac4b25',
    ///                 value: 1000
    ///             },
    ///             {
    ///                 uid: '0xe160ac1bd3606a287b4d53d5d1d6da5895f65b4b4bab6d93aaf5046e48167ade',
    ///                 value: 0
    ///             }],
    ///         },
    ///         {
    ///             schema: '0x5ac273ce41e3c8bfa383efe7c03e54c5f0bff29c9f11ef6ffa930fc84ca32425',
    ///             data: [{
    ///                 uid: '0x053d42abce1fd7c8fcddfae21845ad34dae287b2c326220b03ba241bc5a8f019',
    ///                 value: 0
    ///             },
    ///         }])
    ///
    /// @param multiRequests The arguments of the multi revocation requests. The requests should be grouped by distinct
    ///        schema ids to benefit from the best batching optimization.
    function multiRevoke(MultiRevocationRequest[] calldata multiRequests) external payable;

    /// @notice Revokes existing attestations to multiple schemas via provided EIP712 signatures.
    ///
    ///         Example:
    ///
    ///         multiRevokeByDelegation([{
    ///             schema: '0x8e72f5bc0a8d4be6aa98360baa889040c50a0e51f32dbf0baa5199bd93472ebc',
    ///             data: [{
    ///                 uid: '0x211296a1ca0d7f9f2cfebf0daaa575bea9b20e968d81aef4e743d699c6ac4b25',
    ///                 value: 1000
    ///             },
    ///             {
    ///                 uid: '0xe160ac1bd3606a287b4d53d5d1d6da5895f65b4b4bab6d93aaf5046e48167ade',
    ///                 value: 0
    ///             }],
    ///             signatures: [{
    ///                 v: 28,
    ///                 r: '0x148c...b25b',
    ///                 s: '0x5a72...be22'
    ///             },
    ///             {
    ///                 v: 28,
    ///                 r: '0x487s...67bb',
    ///                 s: '0x12ad...2366'
    ///             }],
    ///             revoker: '0x244934dd3e31bE2c81f84ECf0b3E6329F5381992',
    ///             deadline: 1673891048
    ///         }])
    ///
    /// @param multiDelegatedRequests The arguments of the delegated multi revocation attestation requests. The requests
    /// should be
    ///        grouped by distinct schema ids to benefit from the best batching optimization.
    function multiRevokeByDelegation(MultiDelegatedRevocationRequest[] calldata multiDelegatedRequests)
        external
        payable;

    /// @notice Timestamps the specified bytes32 data.
    /// @param data The data to timestamp.
    /// @return The timestamp the data was timestamped with.
    function timestamp(bytes32 data) external returns (uint64);

    /// @notice Timestamps the specified multiple bytes32 data.
    /// @param data The data to timestamp.
    /// @return The timestamp the data was timestamped with.
    function multiTimestamp(bytes32[] calldata data) external returns (uint64);

    /// @notice Revokes the specified bytes32 data.
    /// @param data The data to timestamp.
    /// @return The timestamp the data was revoked with.
    function revokeOffchain(bytes32 data) external returns (uint64);

    /// @notice Revokes the specified multiple bytes32 data.
    /// @param data The data to timestamp.
    /// @return The timestamp the data was revoked with.
    function multiRevokeOffchain(bytes32[] calldata data) external returns (uint64);

    /// @notice Returns an existing attestation by UID.
    /// @param uid The UID of the attestation to retrieve.
    /// @return The attestation data members.
    function getAttestation(bytes32 uid) external view returns (Attestation memory);

    /// @notice Checks whether an attestation exists.
    /// @param uid The UID of the attestation to retrieve.
    /// @return Whether an attestation exists.
    function isAttestationValid(bytes32 uid) external view returns (bool);

    /// @notice Returns the timestamp that the specified data was timestamped with.
    /// @param data The data to query.
    /// @return The timestamp the data was timestamped with.
    function getTimestamp(bytes32 data) external view returns (uint64);

    /// @notice Returns the timestamp that the specified data was timestamped with.
    /// @param data The data to query.
    /// @return The timestamp the data was timestamped with.
    function getRevokeOffchain(address revoker, bytes32 data) external view returns (uint64);
}
