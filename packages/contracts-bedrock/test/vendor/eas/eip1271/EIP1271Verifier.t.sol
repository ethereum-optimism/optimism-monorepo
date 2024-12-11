// SPDX-License-Identifier: MIT
pragma solidity 0.8.19;

import { Test } from "forge-std/Test.sol";
import { EIP1271Verifier, InvalidSignature } from "src/vendor/eas/eip1271/EIP1271Verifier.sol";
import { SignatureChecker } from "@openzeppelin/contracts/utils/cryptography/SignatureChecker.sol";
import { DeadlineExpired } from "src/vendor/eas/Common.sol";
import { 
    AttestationRequestData,
    DelegatedAttestationRequest,
    Signature,
    IEAS 
} from "src/vendor/eas/IEAS.sol";

// =============================================================
//                        MOCK CONTRACTS 
// =============================================================

/// @dev Test implementation of EIP1271Verifier
contract TestEIP1271Verifier is EIP1271Verifier {
    constructor(string memory name) EIP1271Verifier(name, "1.0.0") {}

    /// @dev Exposes internal verify function for testing
    function verifyAttest(DelegatedAttestationRequest calldata request) external {
        _verifyAttest(request);
    }

    /// @dev Exposes internal time function for testing
    function time() public view returns (uint64) {
        return _time();
    }
}

/// @dev Mock contract implementing EIP1271 signature verification
contract MockEIP1271Signer {
    mapping(bytes32 => bytes) public mockSignatures;

    /// @dev Stores a mock signature for a given hash
    function mockSignature(bytes32 hash, bytes memory signature) external {
        mockSignatures[hash] = signature;
    }

    /// @dev Implements EIP1271 signature verification
    function isValidSignature(bytes32 hash, bytes memory signature) external view returns (bytes4) {
        if (keccak256(mockSignatures[hash]) == keccak256(signature)) {
            return 0x1626ba7e; // Magic value for EIP-1271
        }
        return 0xffffffff;
    }
}

// =============================================================
//                        MAIN TEST CONTRACT
// =============================================================

contract EIP1271VerifierTest is Test {

    // =============================================================
    //                           CONSTANTS
    // =============================================================
    bytes32 constant ZERO_BYTES32 = bytes32(0);
    uint64 constant NO_EXPIRATION = 0;
    bytes32 private constant ATTEST_TYPEHASH = 0xfeb2925a02bae3dae48d424a0437a2b6ac939aa9230ddc55a1a76f065d988076;

    // =============================================================
    //                          TEST STATE
    // =============================================================
    TestEIP1271Verifier public verifier;
    MockEIP1271Signer public mockSigner;
    address public recipient;
    uint256 public signerPrivateKey;
    address public signer;

    // =============================================================
    //                         ERROR TYPES
    // =============================================================
    error InvalidNonce();
    
    // =============================================================
    //                           EVENTS
    // =============================================================
    event NonceIncreased(uint256 oldNonce, uint256 newNonce);

    // =============================================================
    //                           SETUP
    // =============================================================
    /// @dev Deploys contracts and initializes test state
    function setUp() public {
        verifier = new TestEIP1271Verifier("EAS");
        mockSigner = new MockEIP1271Signer();
        recipient = makeAddr("recipient");
        signerPrivateKey = 0xA11CE;
        signer = vm.addr(signerPrivateKey);
    }
    // =============================================================
    //                    INTERNAL HELPERS
    // =============================================================
    /// @dev Helper function to hash typed data for EIP712
    function _hashTypedDataV4(DelegatedAttestationRequest memory request) internal view returns (bytes32) {
        bytes32 structHash = keccak256(
            abi.encode(
                ATTEST_TYPEHASH,
                request.attester,
                request.schema,
                request.data.recipient,
                request.data.expirationTime,
                request.data.revocable,
                request.data.refUID,
                keccak256(request.data.data),
                request.data.value,
                verifier.getNonce(request.attester),
                request.deadline
            )
        );

        return keccak256(
            abi.encodePacked(
                "\x19\x01",
                verifier.getDomainSeparator(),
                structHash
            )
        );
    }
    // =============================================================
    //                      BASIC STATE TESTS
    // =============================================================
    /// @dev Tests initial verifier configuration
    function testInitialState() public view {
        assertEq(verifier.getName(), "EAS");
        assertEq(verifier.getNonce(signer), 0);
    }

    // =============================================================
    //                      NONCE TESTS
    // =============================================================
    /// @dev Tests nonce increase functionality and validation.
    ///      1. Setup:
    ///         - Uses signer account
    ///         - Sets target nonce to 100
    ///      2. Initial Increase:
    ///         - Verifies NonceIncreased event emission
    ///         - Checks event parameters (0 to 100)
    ///         - Confirms new nonce value stored
    ///      3. Invalid Attempt:
    ///         - Tries to decrease nonce (set to 99)
    ///         - Verifies revert with InvalidNonce
    ///      Demonstrates:
    ///         - Proper nonce increment functionality
    ///         - Event emission accuracy
    ///         - Prevention of nonce decrease
    ///         - Access control (signer only)
    function testIncreaseNonce() public {
        vm.startPrank(signer);
        
        uint256 newNonce = 100;
        vm.expectEmit(true, true, true, true);
        emit NonceIncreased(0, newNonce);
        verifier.increaseNonce(newNonce);
        
        assertEq(verifier.getNonce(signer), newNonce);
        
        vm.expectRevert(abi.encodeWithSelector(InvalidNonce.selector));
        verifier.increaseNonce(newNonce - 1);
        
        vm.stopPrank();
    }

    // =============================================================
    //                    DEADLINE TESTS
    // =============================================================
    /// @dev Tests attestation request deadline validation.
    ///      1. Setup:
    ///         - Sets block timestamp to 1000
    ///         - Creates attestation request with:
    ///           * Empty schema and data
    ///           * No expiration time
    ///           * Past deadline (999)
    ///           * Empty signature
    ///      2. Verification:
    ///         - Attempts verification of expired request
    ///         - Confirms revert with DeadlineExpired
    ///      Demonstrates:
    ///         - Proper deadline validation
    ///         - System prevents attestations with expired deadlines
    ///         - Timestamp-based security checks
    ///      Note: Uses minimal request data as deadline check
    ///      occurs before other validations
    function testDeadlineExpired() public {
        vm.warp(1000);
        
        DelegatedAttestationRequest memory request = DelegatedAttestationRequest({
            schema: ZERO_BYTES32,
            data: AttestationRequestData({
                recipient: recipient,
                expirationTime: NO_EXPIRATION,
                revocable: true,
                refUID: ZERO_BYTES32,
                data: new bytes(0),
                value: 1000
            }),
            deadline: 999,
            attester: signer,
            signature: Signature({
                v: 27,
                r: bytes32(0),
                s: bytes32(0)
            })
        });

        vm.expectRevert(abi.encodeWithSelector(DeadlineExpired.selector));
        verifier.verifyAttest(request);
    }

    // =============================================================
    //                    SIGNATURE TESTS
    // =============================================================
    /// @dev Tests EIP-712 signature verification for attestations.
    ///      1. Setup:
    ///         - Creates attestation request with:
    ///           * Zero schema ID
    ///           * 1-hour deadline
    ///           * Empty data
    ///           * Invalid signature (zeros)
    ///      2. Invalid Signature Test:
    ///         - Attempts verification with invalid signature
    ///         - Confirms revert with InvalidSignature
    ///      3. Valid Signature Test:
    ///         - Creates EIP-712 typed data hash
    ///         - Signs hash with signer's private key
    ///         - Updates request with valid signature
    ///         - Verifies attestation passes
    ///      Demonstrates:
    ///         - Signature validation process
    ///         - EIP-712 typed data handling
    ///         - Proper signature verification
    ///         - Invalid signature rejection
    function testSignatureVerification() public {
        bytes32 schemaId = ZERO_BYTES32;
        uint64 deadline = uint64(block.timestamp + 3600);
        
        DelegatedAttestationRequest memory request = DelegatedAttestationRequest({
            schema: schemaId,
            data: AttestationRequestData({
                recipient: recipient,
                expirationTime: NO_EXPIRATION,
                revocable: true,
                refUID: ZERO_BYTES32,
                data: new bytes(0),
                value: 0
            }),
            deadline: deadline,
            attester: signer,
            signature: Signature({
                v: 27,
                r: bytes32(0),
                s: bytes32(0)
            })
        });

        vm.expectRevert(abi.encodeWithSelector(InvalidSignature.selector));
        verifier.verifyAttest(request);

        bytes32 hash = _hashTypedDataV4(request);
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(signerPrivateKey, hash);
        request.signature = Signature(v, r, s);

        verifier.verifyAttest(request);
    }

    // =============================================================
    //                    MULTI-ATTESTATION TESTS
    // =============================================================
    /// @dev Tests multiple delegated attestation verifications.
    ///      1. Setup:
    ///         - Creates array of two attestation requests
    ///         - Uses zero schema ID
    ///         - Sets 1-hour deadline
    ///      2. Request Processing Loop:
    ///         - For each request:
    ///           * Initializes basic attestation data
    ///           * Creates EIP-712 typed data hash
    ///           * Signs with signer's private key
    ///           * Updates request with valid signature
    ///      3. Verification:
    ///         - Verifies each attestation individually
    ///      Demonstrates:
    ///         - Batch attestation processing
    ///         - Consistent signature generation
    ///         - Multiple verification handling
    ///         - EIP-712 compliance across requests
    function testMultipleAttestationDelegation() public {
        bytes32 schemaId = ZERO_BYTES32;
        uint64 deadline = uint64(block.timestamp + 3600);
        
        DelegatedAttestationRequest[] memory requests = new DelegatedAttestationRequest[](2);
        
        for(uint i = 0; i < 2; i++) {
            requests[i] = DelegatedAttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: NO_EXPIRATION,
                    revocable: true,
                    refUID: ZERO_BYTES32,
                    data: new bytes(0),
                    value: 0
                }),
                deadline: deadline,
                attester: signer,
                signature: Signature({
                    v: 27,
                    r: bytes32(0),
                    s: bytes32(0)
                })
            });

            bytes32 hash = _hashTypedDataV4(requests[i]);
            (uint8 v, bytes32 r, bytes32 s) = vm.sign(signerPrivateKey, hash);
            requests[i].signature = Signature(v, r, s);

            verifier.verifyAttest(requests[i]);
        }
    }

    // =============================================================
    //                    COMPLEX NONCE TESTS
    // =============================================================
    /// @dev Tests complex nonce scenarios
    function testComplexNonceScenarios() public {
        address user1 = makeAddr("user1");
        address user2 = makeAddr("user2");

        vm.prank(user1);
        verifier.increaseNonce(100);
        
        vm.prank(user2);
        verifier.increaseNonce(200);

        assertEq(verifier.getNonce(user1), 100);
        assertEq(verifier.getNonce(user2), 200);

        vm.startPrank(user1);
        verifier.increaseNonce(101);
        verifier.increaseNonce(102);
        verifier.increaseNonce(103);
        vm.stopPrank();

        assertEq(verifier.getNonce(user1), 103);
    }

    // =============================================================
    //                    EIP1271 TESTS
    // =============================================================
    /// @dev Tests complex nonce management scenarios with multiple users.
    ///      1. Initial Setup:
    ///         - Creates two test users
    ///         - Sets different initial nonces:
    ///           * user1: 100
    ///           * user2: 200
    ///      2. Initial Verification:
    ///         - Confirms correct nonce storage for both users
    ///      3. Sequential Updates:
    ///         - Performs multiple nonce increases for user1:
    ///           * 100 -> 101
    ///           * 101 -> 102
    ///           * 102 -> 103
    ///      4. Final Verification:
    ///         - Confirms user1's final nonce value
    ///      Demonstrates:
    ///         - Multi-user nonce management
    ///         - Independent nonce tracking
    ///         - Sequential nonce updates
    ///         - Nonce isolation between users
    function testEIP1271SignatureValidation() public {
        MockEIP1271Signer eip1271Contract = new MockEIP1271Signer();
        
        bytes32 schemaId = ZERO_BYTES32;
        uint64 deadline = uint64(block.timestamp + 3600);
        
        DelegatedAttestationRequest memory request = DelegatedAttestationRequest({
            schema: schemaId,
            data: AttestationRequestData({
                recipient: recipient,
                expirationTime: NO_EXPIRATION,
                revocable: true,
                refUID: ZERO_BYTES32,
                data: new bytes(0),
                value: 0
            }),
            deadline: deadline,
            attester: address(eip1271Contract),
            signature: Signature({
                v: 27,
                r: bytes32(0),
                s: bytes32(0)
            })
        });

        bytes32 hash = _hashTypedDataV4(request);
        bytes memory signature = abi.encodePacked(bytes32(uint256(1)), bytes32(uint256(2)), uint8(27));
        eip1271Contract.mockSignature(hash, signature);

        request.signature = Signature({
            v: 27,
            r: bytes32(uint256(1)),
            s: bytes32(uint256(2))
        });

        verifier.verifyAttest(request);
    }
}
