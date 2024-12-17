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

// Mock contracts
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

// Main test contract
contract EIP1271VerifierTest is Test {

    // Constants
    bytes32 constant ZERO_BYTES32 = bytes32(0);
    uint64 constant NO_EXPIRATION = 0;
    bytes32 private constant ATTEST_TYPEHASH = 0xfeb2925a02bae3dae48d424a0437a2b6ac939aa9230ddc55a1a76f065d988076;

    // Test state
    TestEIP1271Verifier public verifier;
    MockEIP1271Signer public mockSigner;
    address public recipient;
    uint256 public signerPrivateKey;
    address public signer;

    // Error types
    error InvalidNonce();
    
    // Events
    event NonceIncreased(uint256 oldNonce, uint256 newNonce);

    // Setup
    /// @dev Deploys contracts and initializes test state
    function setUp() public {
        verifier = new TestEIP1271Verifier("EAS");
        mockSigner = new MockEIP1271Signer();
        recipient = makeAddr("recipient");
        signerPrivateKey = 0xA11CE;
        signer = vm.addr(signerPrivateKey);
    }

    // Helper Functions
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

    // Basic State Tests
    /// @dev Tests initial verifier configuration
    function testInitialState() public view {
        assertEq(verifier.getName(), "EAS");
        assertEq(verifier.getNonce(signer), 0);
    }

    // Nonce Tests
    /// @dev Tests nonce increase functionality and validation.
    function testIncreaseNonce(uint256 _nonce) public {
        vm.assume(_nonce > 0);
        vm.startPrank(signer);
        
        vm.expectEmit(true, true, true, true);
        emit NonceIncreased(0, _nonce);
        verifier.increaseNonce(_nonce);
        
        assertEq(verifier.getNonce(signer), _nonce);
        
        vm.expectRevert(abi.encodeWithSelector(InvalidNonce.selector));
        verifier.increaseNonce(_nonce - 1);
        
        vm.stopPrank();
    }

    // Deadline
    /// @dev Tests attestation request deadline validation.
    ///      Demonstrates:
    ///         - Proper deadline validation
    ///         - System prevents attestations with expired deadlines
    ///         - Timestamp-based security checks
    function testDeadlineExpired(uint64 _timestamp, uint64 _deadline) public {
        vm.assume(_timestamp > _deadline);
        vm.assume(_timestamp < type(uint64).max);
        vm.assume(_deadline > 0);
        
        vm.warp(_timestamp);
        
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
            deadline: _deadline,
            attester: signer,
            signature: Signature({
                v: 27,
                r: bytes32(0),
                s: bytes32(0)
            })
        });

        // Create valid signature
        bytes32 hash = _hashTypedDataV4(request);
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(signerPrivateKey, hash);
        request.signature = Signature(v, r, s);

        vm.expectRevert(abi.encodeWithSelector(DeadlineExpired.selector));
        verifier.verifyAttest(request);
    }


    // Signature Tests
    /// @dev Tests EIP-712 signature verification for attestations.
    ///      Demonstrates:
    ///         - Signature validation process
    ///         - EIP-712 typed data handling
    ///         - Proper signature verification
    ///         - Invalid signature rejection
    function testSignatureVerification(
        uint64 _deadline,
        uint256 _value,
        bytes calldata _data
    ) public {
        // Ensure deadline is in the future
        vm.assume(_deadline > block.timestamp);
        vm.assume(_deadline < type(uint64).max);
        
        DelegatedAttestationRequest memory request = DelegatedAttestationRequest({
            schema: ZERO_BYTES32,
            data: AttestationRequestData({
                recipient: recipient,
                expirationTime: NO_EXPIRATION,
                revocable: true,
                refUID: ZERO_BYTES32,
                data: _data,
                value: _value
            }),
            deadline: _deadline,
            attester: signer,
            signature: Signature({
                v: 27,
                r: bytes32(0),
                s: bytes32(0)
            })
        });

        // Test invalid signature first
        vm.expectRevert(abi.encodeWithSelector(InvalidSignature.selector));
        verifier.verifyAttest(request);

        // Create valid signature and verify
        bytes32 hash = _hashTypedDataV4(request);
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(signerPrivateKey, hash);
        request.signature = Signature(v, r, s);

        verifier.verifyAttest(request);
    }

    // =============================================================
    //                    MULTI-ATTESTATION TESTS
    // =============================================================
    /// @dev Tests multiple delegated attestation verifications.
    ///      Demonstrates:
    ///         - Batch attestation processing
    ///         - Consistent signature generation
    ///         - Multiple verification handling
    ///         - EIP-712 compliance across requests
    function testMultipleAttestationDelegation(
        uint64 _deadline,
        uint256[] calldata _values,
        bytes[] calldata _data
    ) public {
        // Ensure valid inputs
        vm.assume(_deadline > block.timestamp);
        vm.assume(_deadline < type(uint64).max);
        vm.assume(_values.length >= 2);
        vm.assume(_data.length >= 2);
        
        bytes32 schemaId = ZERO_BYTES32;
        DelegatedAttestationRequest[] memory requests = new DelegatedAttestationRequest[](2);
        
        for(uint i = 0; i < 2; i++) {
            requests[i] = DelegatedAttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: NO_EXPIRATION,
                    revocable: true,
                    refUID: ZERO_BYTES32,
                    data: _data[i],
                    value: _values[i]
                }),
                deadline: _deadline,
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

    /// Complex Nonce Tests
    /// @dev Tests complex nonce scenarios
    function testComplexNonceScenarios(
        uint256 _nonce1,
        uint256 _nonce2,
        uint256 _nonce3,
        uint256 _nonce4,
        uint256 _nonce5
    ) public {
        vm.assume(_nonce1 > 0);
        vm.assume(_nonce2 > _nonce1);
        vm.assume(_nonce3 > _nonce2);
        vm.assume(_nonce4 > _nonce3);
        vm.assume(_nonce5 > _nonce4);

        address user1 = makeAddr("user1");
        address user2 = makeAddr("user2");

        vm.prank(user1);
        verifier.increaseNonce(_nonce1);
        
        vm.prank(user2);
        verifier.increaseNonce(_nonce2);

        assertEq(verifier.getNonce(user1), _nonce1);
        assertEq(verifier.getNonce(user2), _nonce2);

        vm.startPrank(user1);
        verifier.increaseNonce(_nonce3);
        verifier.increaseNonce(_nonce4);
        verifier.increaseNonce(_nonce5);
        vm.stopPrank();

        assertEq(verifier.getNonce(user1), _nonce5);
    }

    // EIP1271 Tests
    /// @dev Tests EIP1271 signature validation for delegated attestations.
    ///      Demonstrates:
    ///         - EIP1271 contract signature validation
    ///         - Custom signature verification logic
    ///         - Integration with attestation system
    function testEIP1271SignatureValidation(
        uint64 _deadline,
        uint256 _value,
        bytes calldata _data,
        uint256 _r,
        uint256 _s
    ) public {
        vm.assume(_deadline > block.timestamp);
        vm.assume(_deadline < type(uint64).max);
        vm.assume(_r != 0 && _s != 0);  // Ensure non-zero signature components
        
        MockEIP1271Signer eip1271Contract = new MockEIP1271Signer();
        bytes32 schemaId = ZERO_BYTES32;
        
        DelegatedAttestationRequest memory request = DelegatedAttestationRequest({
            schema: schemaId,
            data: AttestationRequestData({
                recipient: recipient,
                expirationTime: NO_EXPIRATION,
                revocable: true,
                refUID: ZERO_BYTES32,
                data: _data,
                value: _value
            }),
            deadline: _deadline,
            attester: address(eip1271Contract),
            signature: Signature({
                v: 27,
                r: bytes32(0),
                s: bytes32(0)
            })
        });

        bytes32 hash = _hashTypedDataV4(request);
        bytes memory signature = abi.encodePacked(bytes32(_r), bytes32(_s), uint8(27));
        eip1271Contract.mockSignature(hash, signature);

        request.signature = Signature({
            v: 27,
            r: bytes32(_r),
            s: bytes32(_s)
        });

        verifier.verifyAttest(request);
    }
}
