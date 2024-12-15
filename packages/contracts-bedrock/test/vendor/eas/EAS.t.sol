// SPDX-License-Identifier: MIT
pragma solidity =0.8.15;

import { CommonTest } from "test/setup/CommonTest.sol";
import { IEAS } from "src/vendor/eas/IEAS.sol";
import { ISchemaRegistry } from "src/vendor/eas/ISchemaRegistry.sol";
import { Attestation, AttestationRequest, AttestationRequestData, MultiAttestationRequest, RevocationRequest, RevocationRequestData, MultiDelegatedAttestationRequest, MultiDelegatedRevocationRequest, DelegatedAttestationRequest, MultiRevocationRequest, Signature } from "src/vendor/eas/IEAS.sol";
import { ISchemaResolver } from "src/vendor/eas/resolver/ISchemaResolver.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";
import { EIP712 } from "@openzeppelin/contracts/utils/cryptography/draft-EIP712.sol";
import { IERC1271 } from "@openzeppelin/contracts/interfaces/IERC1271.sol";
import { ISemver } from "interfaces/universal/ISemver.sol";
import { Strings } from "@openzeppelin/contracts/utils/Strings.sol";

// =============================================================
//                        MOCK CONTRACTS
// =============================================================

/// @dev Helper contract for testing EIP712 signature verification
contract TestEIP712Helper is EIP712 {
    constructor() EIP712("EAS", "1.3.0") {}

    /// @dev Exposes internal hash function for testing
    function hashTypedDataV4(bytes32 structHash) public view returns (bytes32) {
        return _hashTypedDataV4(structHash);
    }
}

/// @dev Proxy contract for testing EIP712 signature verification
contract TestEIP712Proxy is EIP712 {
    address public verifyingContract;
    string private _name;
    
    constructor(address _verifyingContract, string memory name_) EIP712(name_, "1.3.0") {
        verifyingContract = _verifyingContract;
        _name = name_;
    }

    /// @dev Returns the name of the contract
    function name() public view returns (string memory) {
        return _name;
    }

    /// @dev Returns the domain separator for EIP712
    function DOMAIN_SEPARATOR() public view returns (bytes32) {
        return _domainSeparatorV4();
    }

    /// @dev Exposes internal hash function for testing
    function hashTypedDataV4(bytes32 structHash) public view returns (bytes32) {
        return _hashTypedDataV4(structHash);
    }
    
    /// @dev Forwards attestation request to the verifying contract
    function attestByDelegation(DelegatedAttestationRequest calldata request) external returns (bytes32) {
        return IEAS(verifyingContract).attestByDelegation(request);
    }
}

// =============================================================
//                        MAIN TEST CONTRACT
// =============================================================

contract EASTest is CommonTest {
    // =============================================================
    //                           CONSTANTS
    // =============================================================
    uint64 constant NO_EXPIRATION = 0;
    bytes32 constant ZERO_BYTES32 = bytes32(0);
    bytes32 private constant ATTEST_TYPEHASH = 0xfeb2925a02bae3dae48d424a0437a2b6ac939aa9230ddc55a1a76f065d988076;
    bytes32 private constant REVOKE_TYPEHASH = 0x4e1c85c87bc4c1867b4225cc3fb634a4e0fd8a91feb1ebca195aeaf6611a773b;
    bytes32 constant ATTEST_PROXY_TYPEHASH = 0xea02ffba7dcb45f6fc649714d23f315eef12e3b27f9a7735d8d8bf41eb2b1af1;

    enum SignatureType {
        Direct,
        Delegated,
        DelegatedProxy
    }

    /// @dev A struct representing the full arguments of the delegated multi attestation request.
    struct SignatureComponents {
        uint8 v;
        bytes32 r;
        bytes32 s;
    }


    // =============================================================
    //                        ERROR SELECTORS
    // =============================================================
    error InvalidSchema();
    error InvalidExpirationTime();
    error NotFound();
    error AccessDenied();
    error InvalidLength();
    error AlreadyRevokedOffchain();
    error AlreadyTimestamped();
    error Irrevocable();
    error InvalidSignature();
    error DeadlineExpired();

    // =============================================================
    //                         TEST STATE
    // =============================================================
    ISchemaRegistry public registry;
    address public attester;
    address public sender;
    address public sender2;
    address public recipient;
    address public recipient2;
    address public payableResolver;
    TestEIP712Helper public eip712Helper;
    uint256 public senderKey;
    uint256 public attesterKey;
    TestEIP712Proxy public proxy;

    // =============================================================
    //                      HELPER FUNCTIONS
    // =============================================================
    /// @dev Calculates the unique identifier for a schema based on its parameters
    function _getSchemaUID(
        string memory schema,
        address resolver,
        bool revocable
    ) internal pure returns (bytes32) {
        return keccak256(abi.encodePacked(schema, resolver, revocable));
    }

    /// @dev Registers a test schema with specified parameters
    function _registerSchema(
        string memory schema,
        bool revocable
    ) internal returns (bytes32) {
        bytes32 schemaId = _getSchemaUID(schema, address(0), revocable);
        vm.prank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), revocable);
        return schemaId;
    }
       /// @dev Registers a test schema with specified parameters
    function _registerFuzzSchema(
        string memory schema,
        address resolver,
        bool revocable
    ) internal returns (bytes32) {
        bytes32 schemaId = _getSchemaUID(schema, resolver, revocable);
        vm.prank(sender);
        schemaRegistry.register(schema, ISchemaResolver(resolver), revocable);
        return schemaId;
    }

    /// @dev Returns the type hash for delegated attestations
    function getDelegatedAttestationTypeHash() internal pure returns (bytes32) {
        return ATTEST_TYPEHASH;
    }

    /// @dev Creates standard attestation request data for testing
    function createAttestationRequestData()
        internal
        view
        returns (AttestationRequestData memory)
    {
        return AttestationRequestData({
            recipient: recipient,
            expirationTime: uint64(block.timestamp + 30 days),
            revocable: true,
            refUID: ZERO_BYTES32,
            data: hex"1234",
            value: 0
        });
    }

    /// @dev Creates an attestation digest for signature verification
    function _createAttestationDigest(
        bytes32 schemaId,
        AttestationRequestData memory data,
        address attesterAddress,
        uint64 deadline,
        uint256 nonce
    ) internal view returns (bytes32) {
        bytes32 structHash = keccak256(
            abi.encode(
                getDelegatedAttestationTypeHash(),
                attesterAddress,
                schemaId,
                data.recipient,
                data.expirationTime,
                data.revocable,
                data.refUID,
                keccak256(data.data),
                data.value,
                nonce,
                deadline
            )
        );

        return keccak256(
            abi.encodePacked(
                "\x19\x01",
                _createDomainSeparator(),
                structHash
            )
        );
    }

    /// @dev Creates a revocation digest for signature verification
    function _createRevocationDigest(
        bytes32 schemaId,
        bytes32 uid,
        address revoker,
        uint64 deadline,
        uint256 nonce
    ) internal view returns (bytes32) {
        bytes32 structHash = keccak256(
            abi.encode(
                REVOKE_TYPEHASH,
                revoker,
                schemaId,
                uid,
                0,
                nonce,
                deadline
            )
        );

        return keccak256(
            abi.encodePacked(
                "\x19\x01",
                _createDomainSeparator(),
                structHash
            )
        );
    }

    /// @dev Creates the domain separator for EIP712 signatures
    function _createDomainSeparator() internal view returns (bytes32) {
        bytes32 TYPE_HASH = keccak256(
            "EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)"
        );
        bytes32 nameHash = keccak256(bytes("EAS"));
        bytes32 versionHash = keccak256(bytes("1.3.0"));

        return keccak256(
            abi.encode(
                TYPE_HASH,
                nameHash,
                versionHash,
                block.chainid,
                address(eas)
            )
        );
    }
        /// @dev Creates and verifies a direct signature attestation
    function _testDirectSignature(bytes32 schemaId) internal {
        AttestationRequestData memory requestData = createAttestationRequestData();

        // Test direct attestation
        vm.prank(sender);
        bytes32 uid = eas.attest(
            AttestationRequest({
                schema: schemaId,
                data: requestData
            })
        );
        assertTrue(uid != bytes32(0), "Direct attestation should succeed");

        // Verify the attestation
        Attestation memory attestation = eas.getAttestation(uid);
        assertEq(attestation.attester, sender);
        assertEq(attestation.recipient, requestData.recipient);
    }


    /// @dev Creates and verifies a proxy-based signature attestation
    function _testProxySignature(bytes32 schemaId) internal {
        AttestationRequestData memory requestData = createAttestationRequestData();
        uint64 deadline = uint64(block.timestamp + 1 days);

        uint256 signerKey = 0x12345;
        address signer = vm.addr(signerKey);
        vm.deal(signer, 100 ether);

        // Create signature using proxy's domain separator
        bytes32 digest = keccak256(
            abi.encodePacked(
                "\x19\x01",
               _createDomainSeparator(), // Use proxy's domain separator instead of EAS
                _getStructHash(schemaId, requestData, signer, deadline)
            )
        );
        
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(signerKey, digest);

        DelegatedAttestationRequest memory request = DelegatedAttestationRequest({
            schema: schemaId,
            data: requestData,
            signature: Signature({ v: v, r: r, s: s }),
            attester: signer,
            deadline: deadline
        });

        vm.prank(signer);
        bytes32 uid = proxy.attestByDelegation(request);
        assertTrue(uid != bytes32(0), "Proxy attestation should succeed");

        Attestation memory attestation = eas.getAttestation(uid);
        assertEq(attestation.attester, signer);
        assertEq(attestation.recipient, requestData.recipient);
    }

    /// @dev Creates and verifies a delegated signature attestation
    function _testDelegatedSignature(bytes32 schemaId) internal {
        AttestationRequestData memory requestData = createAttestationRequestData();
        uint64 deadline = uint64(block.timestamp + 1 days);

        uint256 signerKey = 0x12345;
        address signer = vm.addr(signerKey);
        vm.deal(signer, 100 ether);

        bytes32 digest = _createAttestationDigest(
            schemaId,
            requestData,
            signer,
            deadline,
            0
        );
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(signerKey, digest);

        DelegatedAttestationRequest memory request = DelegatedAttestationRequest({
            schema: schemaId,
            data: requestData,
            signature: Signature({ v: v, r: r, s: s }),
            attester: signer,
            deadline: deadline
        });

        vm.prank(signer);
        bytes32 uid = eas.attestByDelegation(request);
        assertTrue(uid != bytes32(0), "Delegated attestation should succeed");

        // Verify the attestation
        Attestation memory attestation = eas.getAttestation(uid);
        assertEq(attestation.attester, signer);
        assertEq(attestation.recipient, requestData.recipient);
    }
    /// @dev Generates the EIP-712 struct hash for an attestation request
    function _getStructHash(
        bytes32 schemaId,
        AttestationRequestData memory data,
        address attesterAddress,
        uint64 deadline
    ) internal pure returns (bytes32) {
        return keccak256(
            abi.encode(
                keccak256(
                    "Attest(bytes32 schema,address recipient,uint64 expirationTime,bool revocable,bytes32 refUID,bytes data,uint256 value,address attester,uint64 deadline)"
                ),
                schemaId,
                data.recipient,
                data.expirationTime,
                data.revocable,
                data.refUID,
                keccak256(data.data),
                data.value,
                attesterAddress,
                deadline
            )
        );
    }
    function _hasSpace(string memory str) internal pure returns (bool) {
        bytes memory strBytes = bytes(str);
        for (uint i = 0; i < strBytes.length; i++) {
            if (strBytes[i] == 0x20)  // ASCII space
                return true;
        }
        return false;
}
    function _isAlphanumeric(string memory str) internal pure returns (bool) {
        bytes memory strBytes = bytes(str);
        for (uint i = 0; i < strBytes.length; i++) {
            bytes1 char = strBytes[i];
            if (!(
                (char >= 0x30 && char <= 0x39) || // 0-9
                (char >= 0x41 && char <= 0x5A) || // A-Z
                (char >= 0x61 && char <= 0x7A)    // a-z
            )) return false;
        }
        return true;
    }
    // =============================================================
    //                     SETUP
    // =============================================================
    function setUp() public override {
        super.setUp(); 
        
        // Initialize test variables
        attesterKey = 0x54321;
        senderKey = 0x12345;
        attester = vm.addr(attesterKey);
        sender = vm.addr(senderKey);
        sender2 = makeAddr("sender2");
        recipient = makeAddr("recipient");
        recipient2 = makeAddr("recipient2");

        payableResolver = makeAddr("payableResolver");

        vm.mockCall(
            payableResolver,
            abi.encodeWithSelector(ISchemaResolver.isPayable.selector),
            abi.encode(true)
        );
        vm.mockCall(
            payableResolver,
            abi.encodeWithSelector(ISchemaResolver.attest.selector),
            abi.encode(true)
        );
        vm.mockCall(
            payableResolver,
            abi.encodeWithSelector(ISchemaResolver.multiAttest.selector),
            abi.encode(true)
        );
        vm.mockCall(
            payableResolver,
            abi.encodeWithSelector(ISchemaResolver.revoke.selector),
            abi.encode(true)
        );
        vm.mockCall(
            payableResolver,
            abi.encodeWithSelector(ISchemaResolver.multiRevoke.selector),
            abi.encode(true)
        );

        // Fund accounts
        vm.deal(sender, 100 ether);
        vm.deal(sender2, 100 ether);

        // Initialize helpers
        eip712Helper = new TestEIP712Helper();
        proxy = new TestEIP712Proxy(address(eas), "EAS-Proxy");
    }

    // =============================================================
    //                    CONSTRUCTION TESTS
    // =============================================================
    /// @dev Tests the initial construction state of the EAS contract
    function testConstructionScenarios() public view {
         assertEq(ISemver(address(eas)).version(), "1.4.1-beta.1");
    }

    /// @dev Tests behavior when using an invalid schema registry
    function testInvalidSchemaRegistry() public {
        // Deploy new EAS with invalid registry address
        IEAS invalidEas = IEAS(Predeploys.EAS);

        string memory schema = "bool like";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        vm.expectRevert(InvalidSchema.selector);
        invalidEas.attest(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: uint64(block.timestamp + 30 days),
                    revocable: true,
                    refUID: ZERO_BYTES32,
                    data: hex"1234",
                    value: 0
                })
            })
        );
        vm.stopPrank();
    }

    // =============================================================
    //                   SIGNATURE VERIFICATION TESTS
    // =============================================================
    /// @dev Tests attestation with valid EIP-712 signature.
    ///      1. Setup:
    ///         - Registers schema and creates signer
    ///         - Prepares attestation request data
    ///         - Creates domain separator
    ///      2. Signature Creation:
    ///         - Generates struct hash with typed data
    ///         - Creates EIP-712 digest
    ///         - Signs digest with signer's key
    ///      3. Attestation:
    ///         - Submits attestation with valid signature
    ///         - Verifies successful attestation via UID check
    ///      Demonstrates complete flow of EIP-712 signed attestation
    function testValidSignatureAttestation() public {
        bytes32 schemaId = _registerSchema("bool like", true);
        uint64 deadline = uint64(block.timestamp + 1 days);

        uint256 signerKey = 0x12345;
        address signer = vm.addr(signerKey);
        vm.deal(signer, 100 ether);

        AttestationRequestData memory requestData = createAttestationRequestData();
        bytes32 DOMAIN_SEPARATOR = _createDomainSeparator();

        bytes32 structHash = keccak256(
            abi.encode(
                getDelegatedAttestationTypeHash(),
                signer,
                schemaId,
                requestData.recipient,
                requestData.expirationTime,
                requestData.revocable,
                requestData.refUID,
                keccak256(requestData.data),
                requestData.value,
                0,
                deadline
            )
        );

        bytes32 digest = keccak256(
            abi.encodePacked("\x19\x01", DOMAIN_SEPARATOR, structHash)
        );

        (uint8 v, bytes32 r, bytes32 s) = vm.sign(signerKey, digest);

        DelegatedAttestationRequest
            memory request = DelegatedAttestationRequest({
                schema: schemaId,
                data: requestData,
                signature: Signature({ v: v, r: r, s: s }),
                attester: signer,
                deadline: deadline
            });

        vm.prank(signer);
        bytes32 uid = eas.attestByDelegation(request);
        assertTrue(uid != bytes32(0), "Attestation should succeed");
    }

    /// @dev Tests rejection of attestations with expired signature deadlines.
    ///      1. Setup:
    ///         - Registers schema
    ///         - Sets block timestamp to 1000
    ///         - Creates deadline in the past (900)
    ///      2. Signature Creation:
    ///         - Creates attestation digest with expired deadline
    ///         - Generates valid signature for expired request
    ///      3. Verification:
    ///         - Attempts attestation with expired signature
    ///         - Confirms transaction reverts with DeadlineExpired
    ///      Ensures proper enforcement of signature deadlines in
    ///      delegated attestations
    function testExpiredDeadlineSignature() public {
        bytes32 schemaId = _registerSchema("bool like", true);
        AttestationRequestData
            memory requestData = createAttestationRequestData();

        // Set a specific timestamp
        vm.warp(1000);
        uint64 expiredDeadline = uint64(block.timestamp - 100); // 900

        uint256 signerKey = 0x12345;
        address signer = vm.addr(signerKey);

        bytes32 digest = _createAttestationDigest(
            schemaId,
            requestData,
            signer,
            expiredDeadline,
            0
        );
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(signerKey, digest);

        DelegatedAttestationRequest
            memory request = DelegatedAttestationRequest({
                schema: schemaId,
                data: requestData,
                signature: Signature({ v: v, r: r, s: s }),
                attester: signer,
                deadline: expiredDeadline
            });

        vm.prank(signer);
        vm.expectRevert(DeadlineExpired.selector);
        eas.attestByDelegation(request);
    }

    /// @dev Tests rejection of attestations with mismatched signer and attester.
    ///      1. Setup:
    ///         - Registers schema
    ///         - Creates signature with wrongSigner's key
    ///      2. Signature Mismatch:
    ///         - Signs digest with wrongSigner
    ///         - Sets attester as sender (different from signer)
    ///      3. Verification:
    ///         - Attempts attestation with mismatched addresses
    ///         - Confirms transaction reverts with InvalidSignature
    ///      Ensures attestations cannot be submitted with signatures
    ///      from addresses different than the specified attester
    function testWrongSignerAttestation() public {
        bytes32 schemaId = _registerSchema("bool like", true);
        uint64 deadline = uint64(block.timestamp + 1 days);

        uint256 wrongSignerKey = 0x54321; // Different from senderKey
        address wrongSigner = vm.addr(wrongSignerKey);

        AttestationRequestData
            memory requestData = createAttestationRequestData();
        bytes32 digest = _createAttestationDigest(
            schemaId,
            requestData,
            wrongSigner,
            deadline,
            0
        );
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(wrongSignerKey, digest);

        DelegatedAttestationRequest memory request = DelegatedAttestationRequest({
            schema: schemaId,
            data: requestData,
            signature: Signature({ v: v, r: r, s: s }),
            attester: sender, 
            deadline: deadline
        });

        vm.expectRevert(InvalidSignature.selector);
        eas.attestByDelegation(request);
    }

    /// @dev Tests signature verification against data tampering in attestations.
    ///      Part of EIP-712 signature verification test suite.
    ///      1. Setup:
    ///         - Registers schema
    ///         - Creates signer and deadline
    ///         - Prepares initial attestation data
    ///      2. Signature Process:
    ///         - Creates EIP-712 digest for original data
    ///         - Signs digest with signer's key
    ///      3. Tampering:
    ///         - Modifies attestation data after signature creation
    ///      4. Verification:
    ///         - Attempts attestation with tampered data
    ///         - Confirms signature verification fails with InvalidSignature
    ///      Ensures EIP-712 signature verification properly detects
    ///      post-signing data modifications
    function testSignatureVerificationDataTampering() public {
        bytes32 schemaId = _registerSchema("bool like", true);
        uint64 deadline = uint64(block.timestamp + 1 days);

        uint256 signerKey = 0x12345;
        address signer = vm.addr(signerKey);

        AttestationRequestData
            memory requestData = createAttestationRequestData();
        bytes32 digest = _createAttestationDigest(
            schemaId,
            requestData,
            signer,
            deadline,
            0
        );
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(signerKey, digest);

        // Modify the data after signing
        requestData.data = hex"5678";

        DelegatedAttestationRequest
            memory request = DelegatedAttestationRequest({
                schema: schemaId,
                data: requestData,
                signature: Signature({ v: v, r: r, s: s }),
                attester: signer,
                deadline: deadline
            });

        vm.expectRevert(InvalidSignature.selector);
        eas.attestByDelegation(request);
    }

        /// @dev Tests direct and delegated signature types.
    ///      Registers a schema and tests two signature scenarios:
    ///      1. Direct signatures
    ///      2. Delegated signatures
    ///      Note: This test does not cover EIP712 signatures,
    ///      despite the original name suggesting all types
    function testDirectAndDelegatedSignatures() public {
        // Register schema once at the start
        bytes32 schemaId = _registerSchema("bool like", true);

        SignatureType[2] memory sigTypes = [
            SignatureType.Direct,
            SignatureType.Delegated
        ];

        for (uint i = 0; i < sigTypes.length; i++) {
            SignatureType sigType = sigTypes[i];
            
            if (sigType == SignatureType.Direct) {
                _testDirectSignature(schemaId);
            } else if (sigType == SignatureType.Delegated) {
                _testDelegatedSignature(schemaId);
            }
        }
    }

    /// @dev Tests signature verification against data tampering.
    ///      Part of signature verification test suite.
    ///      1. Setup:
    ///         - Registers schema
    ///         - Creates valid signature for original data
    ///      2. Data Tampering:
    ///         - Signs original attestation data
    ///         - Modifies request data after signature creation
    ///      3. Verification:
    ///         - Attempts attestation with tampered data
    ///         - Confirms signature verification fails
    ///      Demonstrates EIP-712 signature verification prevents
    ///      data tampering after signing
function testSignatureVerificationTampering() public {
        bytes32 schemaId = _registerSchema("bool like", true);
        uint64 deadline = uint64(block.timestamp + 1 days);

        uint256 signerKey = 0x12345;
        address signer = vm.addr(signerKey);
        vm.deal(signer, 100 ether);

        AttestationRequestData
            memory requestData = createAttestationRequestData();
        bytes32 digest = _createAttestationDigest(
            schemaId,
            requestData,
            signer,
            deadline,
            0
        );
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(signerKey, digest);

        DelegatedAttestationRequest
            memory request = DelegatedAttestationRequest({
                schema: schemaId,
                data: requestData,
                signature: Signature({ v: v, r: r, s: s }),
                attester: signer,
                deadline: deadline
            });

        vm.prank(signer);
        eas.attestByDelegation(request); // First attempt should succeed

        vm.expectRevert(InvalidSignature.selector);
        eas.attestByDelegation(request); // Second attempt should fail
    }

    /// @dev Tests attestation through proxy contract with signature verification.
    ///      Part of proxy attestation test suite.
    ///      1. Setup:
    ///         - Sets specific timestamp (1000)
    ///         - Registers schema for proxy use
    ///         - Creates attestation request data
    ///      2. Signature Creation:
    ///         - Generates EIP-712 struct hash using proxy's domain
    ///         - Creates digest with proxy's domain separator
    ///         - Signs digest with sender's key
    ///      3. Proxy Interaction:
    ///         - Submits attestation through proxy contract
    ///         - Verifies attestation recorded correctly:
    ///           * Confirms valid UID
    ///           * Verifies attester address
    ///           * Checks recipient address
    ///      Demonstrates complete proxy attestation flow with
    ///      proper signature verification and delegation
    function testProxyAttestation() public {
        vm.warp(1000);
        
        bytes32 proxySchemaId = _registerSchema("bool like", true);
        
        AttestationRequestData memory data = AttestationRequestData({
            recipient: recipient,
            expirationTime: NO_EXPIRATION,
            revocable: true,
            refUID: bytes32(0),
            data: hex"1234",
            value: 0
        });

        uint64 deadline = uint64(block.timestamp + 1 days);

        DelegatedAttestationRequest memory request = DelegatedAttestationRequest({
            schema: proxySchemaId,
            data: data,
            signature: Signature(0, bytes32(0), bytes32(0)),
            attester: sender,
            deadline: deadline
        });

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
                0, 
                request.deadline
            )
        );

        bytes32 digest = keccak256(
            abi.encodePacked(
                "\x19\x01",
                _createDomainSeparator(), 
                structHash
            )
        );

        (uint8 v, bytes32 r, bytes32 s) = vm.sign(senderKey, digest);
        request.signature = Signature(v, r, s);

        vm.prank(sender);
        bytes32 uid = proxy.attestByDelegation(request);
        assertTrue(uid != bytes32(0));

        Attestation memory attestation = eas.getAttestation(uid);
        assertEq(attestation.attester, sender);
        assertEq(attestation.recipient, recipient);
    }

    // =============================================================
    //                   BASIC ATTESTATION TESTS
    // =============================================================
    /// @dev Tests basic attestation functionality.
    ///      1. Setup:
    ///         - Creates and registers simple boolean schema
    ///         - Sets 30-day expiration time
    ///      2. Attestation:
    ///         - Creates standard attestation request
    ///         - Submits attestation through EAS
    ///      3. Verification:
    ///         - Confirms schema ID matches
    ///         - Verifies recipient address
    ///      Demonstrates core attestation flow with
    ///      standard schema and parameters
    function testAttestation(
        string memory _property1, 
        uint256 _property2, 
        bool _property3, 
        address _resolver, 
        bool _revocable
    ) public {

        // Create schema string using valid types from docs
        string memory schema = 
        "string name,uint256 age,bool isStudent";

        bytes32 schemaId = _getSchemaUID(schema, _resolver, _revocable);
        emit log_string("=== Schema Debug ===");
        emit log_string(schema);  // Schema string
        emit log_named_address("Resolver", _resolver);
        // For bool we need to convert to string
        emit log_string(_revocable ? "Revocable: true" : "Revocable: false");
        emit log_named_bytes32("Schema ID", schemaId);

        vm.mockCall(
            _resolver,
            abi.encodeWithSelector(ISchemaResolver.attest.selector),
            abi.encode(true)
        );


        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(_resolver), _revocable);

        // Encode data according to schema
        bytes memory data = abi.encode(
            _property1,    // string
            _property2,    // uint
            _property3     // bool
        );

        bytes32 uid = eas.attest(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({  
                    recipient: recipient,
                    expirationTime: uint64(block.timestamp + 30 days),
                    revocable: _revocable,
                    refUID: ZERO_BYTES32,
                    data: data,
                    value: 0
                })
            })
        );

        Attestation memory attestation = eas.getAttestation(uid);
        assertEq(attestation.schema, schemaId);
        assertEq(attestation.recipient, recipient);
        vm.stopPrank();
    }

    /// @dev Tests attestation with empty schema registration.
    ///      1. Setup:
    ///         - Creates schema ID with empty string
    ///         - Registers empty schema with resolver
    ///      2. Attestation:
    ///         - Creates attestation with 30-day expiration
    ///         - Uses empty schema ID
    ///      3. Verification:
    ///         - Confirms attestation recorded with correct schema ID
    ///         - Verifies recipient address
    ///      Demonstrates system handles empty schemas correctly,
    ///      allowing attestations without schema definitions
    function testAttestationWithoutSchema(address _resolver, bool _revocable) public {
        bytes32 schemaId = _getSchemaUID("", _resolver, _revocable);

        vm.startPrank(sender);
        schemaRegistry.register("", ISchemaResolver(_resolver), _revocable);

        uint64 expirationTime = uint64(block.timestamp + 30 days);
        bytes memory data = hex"1234";
         vm.mockCall(
            _resolver,
            abi.encodeWithSelector(ISchemaResolver.attest.selector),
            abi.encode(true)
        );

        bytes32 uid = eas.attest(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: expirationTime,
                    revocable: _revocable,
                    refUID: ZERO_BYTES32,
                    data: data,
                    value: 0
                })
            })
        );

        Attestation memory attestation = eas.getAttestation(uid);
        assertEq(attestation.schema, schemaId);
        assertEq(attestation.recipient, recipient);
        vm.stopPrank();
    }

    /// @dev Tests attestation with schema resolver integration.
    ///      1. Setup:
    ///         - Deploys mock payable resolver
    ///         - Creates and registers schema with resolver
    ///      2. Attestation:
    ///         - Creates attestation request with resolver-enabled schema
    ///         - Submits through EAS
    ///      3. Verification:
    ///         - Confirms attester address
    ///         - Verifies recipient address
    ///         - Validates schema ID matches
    ///      Demonstrates attestation flow with resolver integration,
    ///      ensuring resolver-enabled schemas work correctly
    function testAttestationWithResolver(        
        string memory _property1, 
        uint256 _property2, 
        bool _property3,  
        bool _revocable) public {
        string memory schema = "string name,uint256 age,bool isStudent";
        bytes32 schemaId = _getSchemaUID(schema, address(payableResolver), _revocable);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(payableResolver), _revocable);
        // Encode data according to schema
            bytes memory data = abi.encode(
                _property1,    // string
                _property2,    // uint
                _property3     // bool
            );
        AttestationRequest memory request = AttestationRequest({
            schema: schemaId,
            data: AttestationRequestData({
                recipient: recipient,
                expirationTime: uint64(block.timestamp + 30 days),
                revocable: _revocable,
                refUID: ZERO_BYTES32,
                data: data,
                value: 0
            })
        });

        bytes32 uid = eas.attest(request);
        Attestation memory attestation = eas.getAttestation(uid);

        assertEq(attestation.attester, sender);
        assertEq(attestation.recipient, recipient);
        assertEq(attestation.schema, schemaId);
        vm.stopPrank();
    }

    /// @dev Tests attestation with schema that has no resolver.
    ///      1. Setup:
    ///         - Creates complex schema (phone number data)
    ///         - Registers schema without resolver (address(0))
    ///      2. Attestation:
    ///         - Creates attestation with 30-day expiration
    ///         - Uses schema without resolver
    ///      3. Verification:
    ///         - Confirms schema ID matches
    ///         - Verifies recipient address
    ///      Demonstrates attestation flow for schemas without
    ///      resolver integration, ensuring basic schema
    ///      functionality works independently
    function testAttestationWithoutResolver(        
        string memory _property1, 
        uint256 _property2, 
        bool _property3, 
        bool _revocable) public {
        string memory schema = "string name,uint256 age,bool isStudent";

        bytes32 schemaId = _getSchemaUID(schema, address(0), _revocable);
        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), _revocable);

        uint64 expirationTime = uint64(block.timestamp + 30 days);
            bytes memory data = abi.encode(
            _property1, 
            _property2, 
            _property3  
        );

        bytes32 uid = eas.attest(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: expirationTime,
                    revocable: _revocable,
                    refUID: ZERO_BYTES32,
                    data: data,
                    value: 0
                })
            })
        );

        Attestation memory attestation = eas.getAttestation(uid);
        assertEq(attestation.schema, schemaId);
        assertEq(attestation.recipient, recipient);
        vm.stopPrank();
    }

    /// @dev Tests rejection of attestations with expired timestamps.
    ///      1. Setup:
    ///         - Registers basic schema
    ///         - Sets block timestamp to 1000000
    ///         - Creates expiration time in the past (current - 1000)
    ///      2. Debug Logging:
    ///         - Logs current timestamp
    ///         - Logs expired time
    ///         - Logs schema ID
    ///      3. Verification:
    ///         - Attempts attestation with expired time
    ///         - Confirms revert with InvalidExpirationTime
    ///      Ensures system properly validates attestation
    ///      expiration times, preventing backdated attestations
    function testCannotAttestWithExpiredTime(       
        string memory _property1, 
        uint256 _property2, 
        bool _property3,  
        address _resolver,
        bool _revocable) public {
        string memory schema = "string name,uint256 age,bool isStudent";
        schemaRegistry.register(schema, ISchemaResolver(_resolver), _revocable);
        bytes32 schemaId = _getSchemaUID(schema, _resolver, _revocable);

        // Set a specific block timestamp first
        vm.warp(1000000);

        unchecked {
            uint64 expiredTime = uint64(block.timestamp - 1000);
            bytes memory data = abi.encode(
                _property1,    // string
                _property2,    // uint
                _property3     // bool
            );

            // Add debug logs
            emit log_string("Testing attestation with expired time");
            emit log_named_uint("Current block timestamp", block.timestamp);
            emit log_named_uint("Expired time", expiredTime);
            emit log_named_bytes32("Schema ID", schemaId);

            AttestationRequest memory request = AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: expiredTime,
                    revocable: _revocable,
                    refUID: ZERO_BYTES32,
                    data: data,
                    value: 0
                })
            });

            vm.expectRevert(InvalidExpirationTime.selector);
            eas.attest(request);
        }
    }

    /// @dev Tests the behavior of the attestation system with respect to schema registration and validation.
    ///      1. Registers three different schemas:
    ///         - Basic boolean schema
    ///         - Proposal voting schema
    ///         - Phone verification schema
    ///      2. Attempts to attest with an unregistered schema (BAD).
    ///      3. Verifies that the transaction reverts with the InvalidSchema error.
    ///      This function serves to ensure that the system correctly handles
    ///      both valid and invalid schema scenarios.
    function testUnregisteredSchemaAttestationRevert() public {
        // Register test schemas first
        string memory schema1 = "bool like";
        string memory schema2 = "bytes32 proposalId, bool vote";
        string memory schema3 = "bool hasPhoneNumber, bytes32 phoneHash";

        vm.startPrank(sender);
        schemaRegistry.register(schema1, ISchemaResolver(address(0)), true);
        schemaRegistry.register(schema2, ISchemaResolver(address(0)), true);
        schemaRegistry.register(schema3, ISchemaResolver(address(0)), true);

        // Test: revert when attesting to an unregistered schema
        bytes32 badSchemaId = keccak256("BAD");
        vm.expectRevert(InvalidSchema.selector);
        eas.attest(
            AttestationRequest({
                schema: badSchemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: uint64(block.timestamp + 30 days),
                    revocable: true,
                    refUID: ZERO_BYTES32,
                    data: hex"1234",
                    value: 0
                })
            })
        );

        vm.stopPrank();
    }
    /// @dev Tests attestations with varying data payload sizes.
    ///      Registers a simple boolean schema and tests attestations with three different data sizes:
    ///      1. Empty data (0 bytes)
    ///      2. Small data (2-32 bytes)
    ///      3. Large data (34-90 bytes)
    ///      Verifies that the system correctly handles and stores data of different sizes
    ///      by checking that stored attestation data matches the input data for each case
    function testAttestationDataScenarios(string memory _smallTestData, string memory _mediumTestData) public {
        vm.assume(bytes(_smallTestData).length > 2);
        vm.assume(bytes(_smallTestData).length < 32);
        vm.assume(bytes(_mediumTestData).length > 32);
        vm.assume(bytes(_mediumTestData).length < 90);
        string memory schema = "bool like";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);

        // Test with different data sizes
        bytes[] memory testData = new bytes[](3);
            testData[0] = "";
            testData[1] = bytes(_smallTestData);
            testData[2] = bytes(_mediumTestData); 

        for (uint i = 0; i < testData.length; i++) {
            bytes32 uid = eas.attest(
                AttestationRequest({
                    schema: schemaId,
                    data: AttestationRequestData({
                        recipient: recipient,
                        expirationTime: uint64(block.timestamp + 30 days),
                        revocable: true,
                        refUID: ZERO_BYTES32,
                        data: testData[i],
                        value: 0
                    })
                })
            );

            Attestation memory attestation = eas.getAttestation(uid);
            assertEq(attestation.data, testData[i]);
        }

        vm.stopPrank();
    }
    /// @dev Tests comprehensive attestation scenarios with varying parameter combinations.
    ///      Creates two attestations with a payable resolver:
    ///      1. Minimal attestation: zero address recipient, no expiration, no value, empty refUID
    ///      2. Full attestation: specified recipient, 1-year expiration, 1 ETH value, references first attestation
    ///      Verifies all attestation fields are correctly stored for both cases, including
    ///      recipients, expiration times, revocability, reference UIDs, and attached data.
    ///      Also tests value transfer functionality with the resolver contract
    function testDetailedAttestationScenarios(
        address _recipient,
        uint256 _expirationOffset,
        bytes memory _data1,
        bytes memory _data2
    ) public {
        // Fixed schema
        string memory schema = "bool like";
        bytes32 schemaId = _getSchemaUID(schema, address(payableResolver), true);

        // Fuzzing input validation
        vm.assume(_recipient != address(0)); // Ensure recipient is not the zero address
        vm.assume(_expirationOffset > 0 && _expirationOffset <= 365 days);
        vm.assume(_data1.length > 0 && _data1.length <= 32); 
        vm.assume(_data2.length > 0 && _data2.length <= 32);
        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(payableResolver)), true);

        uint256 value = 1 ether;
        vm.deal(sender, value);

        // Test attestation with minimum values
        bytes32 uid1 = eas.attest(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: address(0),
                    expirationTime: 0, // This may need to be adjusted if zero is invalid
                    revocable: true,
                    refUID: bytes32(0),
                    data: _data1, // Use fuzzed data
                    value: 0
                })
            })
        );

        // Test attestation with all fields populated
        bytes32 uid2 = eas.attest{ value: value }(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: _recipient, // Use fuzzed recipient
                    expirationTime: uint64(block.timestamp + _expirationOffset), // Ensure this is a future time
                    revocable: true,
                    refUID: uid1,
                    data: _data2, // Use fuzzed data
                    value: value
                })
            })
        );

        // Verify first attestation
        Attestation memory attestation1 = eas.getAttestation(uid1);
        assertEq(attestation1.recipient, address(0));
        assertEq(attestation1.expirationTime, 0); // Check if this is valid
        assertTrue(attestation1.revocable);
        assertEq(attestation1.refUID, bytes32(0));
        assertEq(attestation1.data, _data1);

        // Verify second attestation
        Attestation memory attestation2 = eas.getAttestation(uid2);
        assertEq(attestation2.recipient, _recipient);
        assertEq(
            attestation2.expirationTime,
            uint64(block.timestamp + _expirationOffset)
        );
        assertTrue(attestation2.revocable);
        assertEq(attestation2.refUID, uid1);
        assertEq(attestation2.data, _data2);

        vm.stopPrank();
    }

    /// @dev Tests the behavior of the attestation system with respect to detailed scenarios.
    ///      1. Registers a fixed schema for attestations.
    ///      2. Creates two attestations with varying parameters.
    ///      3. Verifies the properties of both attestations to ensure correctness.
function testAttestationExpirationScenarios(
    address _recipient,          // Fuzzed recipient address
    uint256 _validExpirationOffset, // Fuzzed valid expiration offset (in seconds)
    uint256 _invalidExpirationOffset // Fuzzed invalid expiration offset (to test reverts)
) public {
    // Fixed schema
    string memory schema = "bool like";
    bytes32 schemaId = _getSchemaUID(schema, address(0), true);

    // Fuzzing input validation
    vm.assume(_recipient != address(0)); // Ensure recipient is not the zero address
    vm.assume(_validExpirationOffset > 0 && _validExpirationOffset <= 365 days); // Ensure valid expiration offset
    vm.assume(_invalidExpirationOffset > 365 days); // Ensure invalid expiration offset is greater than 1 year

    vm.startPrank(sender);
    schemaRegistry.register(schema, ISchemaResolver(address(0)), true);

    // Set a specific block timestamp first
    vm.warp(1000000);

    // Test with valid expiration times
    uint64[] memory expirationTimes = new uint64[](3);
    expirationTimes[0] = 0; // No expiration
    expirationTimes[1] = uint64(block.timestamp + _validExpirationOffset); // Valid future expiration
    expirationTimes[2] = uint64(block.timestamp + 365 days); // Valid future expiration

    for (uint i = 0; i < expirationTimes.length; i++) {
        bytes32 uid = eas.attest(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: _recipient, // Use fuzzed recipient
                    expirationTime: expirationTimes[i], // Use valid expiration times
                    revocable: true,
                    refUID: bytes32(0),
                    data: hex"1234",
                    value: 0
                })
            })
        );

        Attestation memory attestation = eas.getAttestation(uid);
        assertEq(attestation.expirationTime, expirationTimes[i]); // Verify expiration time matches
    }

    // Test with an expired time (should revert)
    uint64 expiredTime = uint64(block.timestamp - 100); // Make sure it's definitely expired

    // Add debug logs
    emit log_named_uint("Current block timestamp", block.timestamp);
    emit log_named_uint("Expired time", expiredTime);
    emit log_named_bytes32("Schema ID", schemaId);

    vm.expectRevert(InvalidExpirationTime.selector);
    eas.attest(
        AttestationRequest({
            schema: schemaId,
            data: AttestationRequestData({
                recipient: _recipient, // Use fuzzed recipient
                expirationTime: expiredTime, // Use expired time
                revocable: true,
                refUID: bytes32(0),
                data: hex"1234",
                value: 0
            })
        })
    );
    vm.stopPrank();
}

    /// @dev Tests behavior when querying non-existent (unregistered) data.
    ///      Verifies two scenarios:
    ///      1. getTimestamp returns 0 for unregistered attestation data
    ///      2. getRevokeOffchain returns 0 for unregistered revocation data
    ///      Ensures system handles queries for non-existent data gracefully
    function testUnregisteredDataScenarios(string memory _unregisteredData) public view {
        bytes32 unregisteredData = keccak256(abi.encodePacked(_unregisteredData));

        // Should return 0 for unregistered timestamp
        assertEq(eas.getTimestamp(unregisteredData), 0);

        // Should return 0 for unregistered revocation
        assertEq(eas.getRevokeOffchain(sender, unregisteredData), 0);
    }

    /// @dev Tests rejection of attestations with invalid reference UIDs.
    ///      1. Setup:
    ///         - Registers basic boolean schema
    ///      2. Invalid Reference:
    ///         - Creates attestation request with non-existent refUID
    ///      3. Verification:
    ///         - Attempts attestation with invalid reference
    ///         - Confirms revert with NotFound error
    ///      Ensures system properly validates referenced attestations,
    ///      preventing attestations that reference non-existent UIDs
    function testInvalidAttestationData(uint256 _nonExistentUID) public {
        vm.assume(_nonExistentUID > 0);
        string memory schema = "bool like";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);

        // Test with non-existent reference UID
        bytes32 nonExistentUID = bytes32(_nonExistentUID);

        vm.expectRevert(NotFound.selector);
        eas.attest(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: uint64(block.timestamp + 30 days),
                    revocable: true,
                    refUID: nonExistentUID, // Reference to non-existent attestation
                    data: hex"1234",
                    value: 0
                })
            })
        );
        vm.stopPrank();
    }

    // =============================================================
    //                  MULTI ATTESTATION TESTS
    // =============================================================
    /// @dev This function tests the multi-attestation functionality of the smart contract.
    /// It validates that multiple attestations can be created in a single transaction,
    /// ensuring that each attestation corresponds to the correct schema and that the
    /// data matches the expected values for both "like" and "score".
    ///
    /// Key operations include:
    /// 1. Setting up schemas for "like" and "score".
    /// 2. Registering the schemas with the schema registry.
    /// 3. Initializing an array of recipients, including a zero address for testing.
    /// 4. Creating a multi-attestation request array to accommodate two attestations
    ///    (one for "like" and one for "score") for each recipient.
    /// 5. Populating the request array with attestation data for each recipient.
    /// 6. Calculating the total Ether value required for the transaction.
    /// 7. Executing the multi-attestation using the `multiAttest` function.
    /// 8. Asserting that the expected number of unique identifiers (UIDs) for the
    ///    attestations matches the actual count and verifying the details of each
    ///    attestation.
    function testMultiAttestationComprehensive(
        address _recipient,
        address _recipient2,
        bool _like,
        uint256 _score
    ) public {
        string memory schema = "bool like";
        string memory schema2 = "uint256 score";
        bytes32 schemaId = _getSchemaUID(schema, address(payableResolver), true);
        bytes32 schemaId2 = _getSchemaUID(schema2, address(payableResolver), true);
        vm.assume(_like == true);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(payableResolver)), true);
        schemaRegistry.register(schema2, ISchemaResolver(address(payableResolver)), true);

        // Test with multiple recipients and varying data
        address[] memory recipients = new address[](3);
        recipients[0] = _recipient;
        recipients[1] = _recipient2;
        recipients[2] = address(0); // Testing with zero address as well

        // Create a multi-attestation request array
        MultiAttestationRequest[] memory requests = new MultiAttestationRequest[](recipients.length * 2); // 2 requests per recipient

        for (uint i = 0; i < recipients.length; i++) {
            emit log_named_string("Like Value", _like ? "true" : "false");
            emit log_named_uint("Score Value", _score);

            // Create attestation for the "like" schema
            requests[i * 2] = MultiAttestationRequest({
                schema: schemaId,
                data: new AttestationRequestData[](1) // One data entry for "like"
            });
            requests[i * 2].data[0] = AttestationRequestData({
                recipient: recipients[i],
                expirationTime: 0, // Set expiration time as needed
                revocable: true,
                refUID: bytes32(0),
                data: abi.encode(_like), // Encode the like value
                value: 0.1 ether // Ensure value is non-zero
            });

            // Create attestation for the "score" schema
            requests[i * 2 + 1] = MultiAttestationRequest({
                schema: schemaId2,
                data: new AttestationRequestData[](1) // One data entry for "score"
            });
            requests[i * 2 + 1].data[0] = AttestationRequestData({
                recipient: recipients[i],
                expirationTime: 0, // Set expiration time as needed
                revocable: true,
                refUID: bytes32(0),
                data: abi.encode(_score), // Encode the score value
                value: 0.1 ether // Ensure value is non-zero
            });
        }

        // Calculate total value required for the transaction
        uint256 totalValue = 0.2 ether * recipients.length; // 0.2 ether for each recipient (0.1 ether for like + 0.1 ether for score)
        vm.deal(sender, totalValue); // Ensure sender has enough Ether
        bytes32[] memory uids = eas.multiAttest{ value: totalValue }(requests);
        assertEq(uids.length, 6); // Expecting 2 attestations per recipient

        // Verify all attestations
        for (uint i = 0; i < uids.length; i++) {
            Attestation memory attestation = eas.getAttestation(uids[i]);

            assertEq(attestation.attester, sender);
            if (i % 2 == 0) {
                assertEq(attestation.schema, schemaId);
                (bool like) = abi.decode(attestation.data, (bool));
                assertEq(like, _like);
            } else {
                assertEq(attestation.schema, schemaId2);
                (uint256 score) = abi.decode(attestation.data, (uint256));
                assertEq(score, _score);
            }
        }

        vm.stopPrank();
    }

        /// @dev Tests complex multi-attestation scenarios across different schemas.
        ///      Creates a batch of attestations that includes:
        ///      1. Two attestations for first schema:
        ///         - To recipient with 30 day expiration
        ///         - To recipient2 with 60 day expiration
        ///      2. One attestation for second schema:
        ///         - To recipient with 90 day expiration
        ///      Verifies correct schema assignment and attester for all attestations
        function testComplexMultiAttestationScenarios() public {
            string memory schema = "bool like";
            string memory schema2 = "uint256 score";
            bytes32 schemaId = _getSchemaUID(schema, address(0), true);

            vm.startPrank(sender);
            schemaRegistry.register(schema, ISchemaResolver(address(0)), true);
            schemaRegistry.register(schema2, ISchemaResolver(address(0)), true);

            // Test with multiple schemas in single transaction
            MultiAttestationRequest[]
                memory requests = new MultiAttestationRequest[](2);

            // First schema batch
            requests[0].schema = schemaId;
            requests[0].data = new AttestationRequestData[](2);
            requests[0].data[0] = AttestationRequestData({
                recipient: recipient,
                expirationTime: uint64(block.timestamp + 30 days),
                revocable: true,
                refUID: ZERO_BYTES32,
                data: hex"1234",
                value: 0
            });
            requests[0].data[1] = AttestationRequestData({
                recipient: recipient2,
                expirationTime: uint64(block.timestamp + 60 days),
                revocable: true,
                refUID: ZERO_BYTES32,
                data: hex"5678",
                value: 0
            });

            // Second schema batch
            requests[1].schema = _getSchemaUID(schema2, address(0), true);
            requests[1].data = new AttestationRequestData[](1);
            requests[1].data[0] = AttestationRequestData({
                recipient: recipient,
                expirationTime: uint64(block.timestamp + 90 days),
                revocable: true,
                refUID: ZERO_BYTES32,
                data: hex"9012",
                value: 0
            });

            bytes32[] memory uids = eas.multiAttest(requests);
            assertEq(uids.length, 3);

            // Verify all attestations
            for (uint i = 0; i < uids.length; i++) {
                Attestation memory attestation = eas.getAttestation(uids[i]);
                assertEq(attestation.attester, sender);
                if (i < 2) {
                    assertEq(attestation.schema, schemaId);
                } else {
                    assertEq(
                        attestation.schema,
                        _getSchemaUID(schema2, address(0), true)
                    );
                }
            }

            vm.stopPrank();
        }

    /// @dev Tests all error conditions for multi-attestation requests:
    ///      1. Empty requests array (returns empty array, no revert)
    ///      2. Empty data array in request (reverts with InvalidLength)
    ///      3. Invalid schema ID (reverts with InvalidSchema)
    ///      4. Invalid expiration time (reverts with InvalidExpirationTime)
    ///      5. Insufficient ETH value (reverts with default error)
    ///      Each test case validates a different aspect of input validation
    ///      and error handling in the multi-attestation process
    function testMultiAttestationReverts() public {
        string memory schema = "bool like";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);
        bytes32 schemaWithResolverId = _getSchemaUID(
            schema,
            address(payableResolver),
            true
        );

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);
        schemaRegistry.register(schema, ISchemaResolver(address(payableResolver)), true);

        // Test 1: Empty requests array should return empty array
        MultiAttestationRequest[]
            memory requests = new MultiAttestationRequest[](0);

        bytes32[] memory uids = eas.multiAttest(requests);
        assertEq(uids.length, 0);

        // Test 2: Empty data array in request
        requests = new MultiAttestationRequest[](1);
        requests[0] = MultiAttestationRequest({
            schema: schemaId,
            data: new AttestationRequestData[](0)
        });

        vm.expectRevert(abi.encodeWithSelector(InvalidLength.selector));
        eas.multiAttest(requests);

        // Test 3: Invalid schema
        requests[0] = MultiAttestationRequest({
            schema: bytes32(0),
            data: new AttestationRequestData[](1)
        });
        requests[0].data[0] = AttestationRequestData({
            recipient: recipient,
            expirationTime: NO_EXPIRATION,
            revocable: true,
            refUID: ZERO_BYTES32,
            data: hex"1234",
            value: 0
        });

        vm.expectRevert(InvalidSchema.selector);
        eas.multiAttest(requests);

        // Test 4: Invalid expiration time
        requests[0].schema = schemaId;
        requests[0].data[0].expirationTime = uint64(block.timestamp);

        vm.expectRevert(InvalidExpirationTime.selector);
        eas.multiAttest(requests);

        // Test 5: Insufficient value sent
        requests[0].schema = schemaWithResolverId;
        requests[0].data[0].expirationTime = NO_EXPIRATION;
        requests[0].data[0].value = 1 ether;

        vm.expectRevert();
        eas.multiAttest(requests);

        vm.stopPrank();
    }
    /// @dev Tests the multi-attestation functionality with value transfers.
    ///      This function creates two attestations for two different users,
    ///      each with a specified amount of ether attached. It verifies that
    ///      the attestations are correctly recorded and that the sender's
    ///      balance is appropriately managed.
    /// 
    ///      1. Assumes the input value is less than or equal to 10.
    ///      2. Registers a schema for the attestations.
    ///      3. Allocates double the specified value to the sender's balance.
    ///      4. Encodes data for two users to be included in the attestations.
    ///      5. Creates and submits multi-attestation requests with the encoded data.
    ///      6. Verifies that the correct number of attestations were created
    ///         and checks that the attester is the expected sender.
    function testMultiAttestationWithValue(uint256 _value, address _userAddress, string memory _userName, bool _isActive, address _userAddress2, string memory _userName2, bool _isActive2) public {
        string memory schema = "address userAddress,string userName,bool isActive,address userAddress2,string userName2,bool isActive2";
        vm.assume(_value <= 10);
        bytes32 schemaId = _getSchemaUID(schema, address(payableResolver), true);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(payableResolver)), true);

        uint256 value = _value * 1 ether;
        vm.deal(sender, value * 2);
        bytes memory data = abi.encode(
            _userAddress,    // address
            _userName,    // string
            _isActive    // bool
        );
        bytes memory data2 = abi.encode(
            _userAddress2,    // address
            _userName2,    // string
            _isActive2    // bool
        );

        MultiAttestationRequest[]
            memory requests = new MultiAttestationRequest[](2);
        requests[0].schema = schemaId;
        requests[0].data = new AttestationRequestData[](1);
        requests[0].data[0] = AttestationRequestData({
            recipient: recipient,
            expirationTime: uint64(block.timestamp + 30 days),
            revocable: true,
            refUID: ZERO_BYTES32,
            data: data,
            value: value
        });

        requests[1].schema = schemaId;
        requests[1].data = new AttestationRequestData[](1);
        requests[1].data[0] = AttestationRequestData({
            recipient: recipient2,
            expirationTime: uint64(block.timestamp + 30 days),
            revocable: true,
            refUID: ZERO_BYTES32,
            data: data2,
            value: value
        });

        bytes32[] memory uids = eas.multiAttest{ value: value * 2 }(requests);
        assertEq(uids.length, 2);

        // Verify attestations
        for (uint i = 0; i < uids.length; i++) {
            Attestation memory attestation = eas.getAttestation(uids[i]);
            assertEq(attestation.attester, sender);
        }

        vm.stopPrank();
    }

    // =============================================================
    //                     REVOCATION TESTS
    // =============================================================
    /// @dev Tests the attestation revocation process.
    ///      1. Creates a revocable attestation with a fuzz expiration time
    ///      2. Revokes the attestation using its UID
    ///      3. Verifies the revocation by checking that revocationTime
    ///         is set to a non-zero value in the attestation data
    function testRevokeAttestation(address _userAddress, string memory _userName, bool _isActive, uint64 _expirationOffset) public {
        vm.assume(_expirationOffset > 0 && _expirationOffset < 366 days);
        string memory schema = "address userAddress,string userName,bool isActive";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);

        uint64 expirationTime = uint64(block.timestamp + _expirationOffset);
        bytes memory data = abi.encode(
            _userAddress,
            _userName,
            _isActive 
        );

        bytes32 uid = eas.attest(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: expirationTime,
                    revocable: true,
                    refUID: ZERO_BYTES32,
                    data: data,
                    value: 0
                })
            })
        );

        eas.revoke(
            RevocationRequest({
                schema: schemaId,
                data: RevocationRequestData({ uid: uid, value: 0 })
            })
        );

        Attestation memory attestation = eas.getAttestation(uid);
        assertTrue(attestation.revocationTime > 0);
        vm.stopPrank();
    }

    /// @dev Tests that attestations can only be revoked by their original attester.
    ///      1. Creates an attestation from sender address
    ///      2. Attempts to revoke it from sender2 address
    ///      3. Verifies the revocation fails with AccessDenied error
    ///      Ensures attestation revocation permissions are properly enforced
    function testCannotRevokeOthersAttestation(address _userAddress, string memory _userName, bool _isActive, uint64 _expirationOffset) public {
        vm.assume(_expirationOffset > 0 && _expirationOffset < 366 days);
         string memory schema = "address userAddress,string userName,bool isActive";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);

        vm.prank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);

        uint64 expirationTime = uint64(block.timestamp + _expirationOffset);
        bytes memory data = abi.encode(
            _userAddress,
            _userName,
            _isActive 
        );

        vm.prank(sender);
        bytes32 uid = eas.attest(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: expirationTime,
                    revocable: true,
                    refUID: ZERO_BYTES32,
                    data: data,
                    value: 0
                })
            })
        );

        vm.prank(sender2);
        vm.expectRevert(AccessDenied.selector);
        eas.revoke(
            RevocationRequest({
                schema: schemaId,
                data: RevocationRequestData({ uid: uid, value: 0 })
            })
        );
    }

    /// @dev Tests revocation attempt of a non-existent attestation.
    ///      1. Registers a schema but creates no attestations
    ///      2. Attempts to revoke using a fabricated UID (1)
    ///      3. Verifies the revocation fails with NotFound error
    ///      Ensures system properly handles revocation requests for non-existent UIDs
    function testCannotRevokeNonExistentAttestation(bytes32 _nonExistentUid) public {
        string memory schema = "bool like";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);

        vm.prank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);

        bytes32 nonExistentUid = _nonExistentUid;

        vm.prank(sender);
        vm.expectRevert(NotFound.selector);
        eas.revoke(
            RevocationRequest({
                schema: schemaId,
                data: RevocationRequestData({ uid: nonExistentUid, value: 0 })
            })
        );
    }

    /// @dev Tests revocation behavior with referenced attestations (parent-child relationship).
    ///      1. Creates a parent attestation
    ///      2. Creates a child attestation that references the parent
    ///      3. Revokes the parent attestation
    ///      4. Verifies that revoking parent doesn't affect child attestation
    ///      Ensures that attestation references don't create revocation dependencies
    function testRevocationWithRefUID( 
        address _userAddress, 
        string memory _userName, 
        bool _isActive, 
        address _userAddress2, 
        string memory _userName2, 
        bool _isActive2, 
        uint64 _expirationOffset
        ) public {
        vm.assume(_expirationOffset > 0 && _expirationOffset < 366 days);
        string memory schema = "address userAddress,string userName,bool isActive";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);
        bytes memory data = abi.encode(
            _userAddress,
            _userName,
            _isActive 
        );
        bytes memory data2 = abi.encode(
            _userAddress2,
            _userName2,
            _isActive2 
        );

        // Create parent attestation
        bytes32 parentUID = eas.attest(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: uint64(block.timestamp + _expirationOffset),
                    revocable: true,
                    refUID: ZERO_BYTES32,
                    data: data,
                    value: 0
                })
            })
        );

        // Create child attestation
        bytes32 childUID = eas.attest(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: uint64(block.timestamp + _expirationOffset),
                    revocable: true,
                    refUID: parentUID,
                    data: data2,
                    value: 0
                })
            })
        );

        // Revoke parent
        eas.revoke(
            RevocationRequest({
                schema: schemaId,
                data: RevocationRequestData({ uid: parentUID, value: 0 })
            })
        );

        // Child should still be valid
        Attestation memory childAttestation = eas.getAttestation(childUID);
        assertEq(childAttestation.revocationTime, 0);

        vm.stopPrank();
    }

    /// @dev Tests batch revocation of multiple attestations.
    ///      1. Creates three attestations with different data values (1,2,3)
    ///      2. Revokes all attestations in a single transaction
    ///      3. Verifies each attestation has a non-zero revocationTime
    ///      Demonstrates the efficiency of batch revocation for multiple
    ///      attestations sharing the same schema
    function testMultiRevocation(uint256 _count, uint64 _expirationOffset) public {
        vm.assume(_count > 0 && _count <=10);
        vm.assume(_expirationOffset > 0 && _expirationOffset <= 365 days);
        bytes32 schemaId = _registerSchema("bool like", true);

        vm.startPrank(sender);

        // Create multiple attestations
        bytes32[] memory uids = new bytes32[](_count);
        for (uint i = 0; i < _count; i++) {
            uids[i] = eas.attest(
                AttestationRequest({
                    schema: schemaId,
                    data: AttestationRequestData({
                        recipient: recipient,
                        expirationTime: uint64(block.timestamp + _expirationOffset),
                        revocable: true,
                        refUID: ZERO_BYTES32,
                        data: abi.encodePacked(bytes1(uint8(i + 1))),
                        value: 0
                    })
                })
            );
        }

        // Create revocation requests
        MultiRevocationRequest[] memory requests = new MultiRevocationRequest[](
            1
        );
        requests[0].schema = schemaId;
        requests[0].data = new RevocationRequestData[](_count);
        for (uint i = 0; i < _count; i++) {
            requests[0].data[i] = RevocationRequestData({
                uid: uids[i],
                value: 0
            });
        }

        // Revoke all attestations
        eas.multiRevoke(requests);

        // Verify all attestations are revoked
        for (uint i = 0; i < uids.length; i++) {
            Attestation memory attestation = eas.getAttestation(uids[i]);
            assertTrue(
                attestation.revocationTime > 0,
                "Attestation should be revoked"
            );
        }

        vm.stopPrank();
    }

    /// @dev Tests access control for delegated revocations.
    ///      1. Creates an attestation from sender address
    ///      2. Attempts unauthorized revocation from sender2 address
    ///      3. Verifies revocation fails with AccessDenied
    ///      Ensures that delegated revocation permissions are properly
    ///      enforced and only the original attester can revoke
    function testDelegatedRevocationRevert(
        address _recipient,
        uint64 _expirationOffset,
        address _sender2
    ) public {
        vm.assume(_expirationOffset > 0 && _expirationOffset <= 365 days);
        vm.assume(_recipient != address(0)); 

        string memory schema = "address userAddress,string userName,bool isActive";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);

        // Create attestation
        bytes32 uid = eas.attest(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: _recipient, 
                    expirationTime: uint64(block.timestamp + _expirationOffset), // Fuzzed expiration time
                    revocable: true,
                    refUID: ZERO_BYTES32,
                    data: hex"1234",
                    value: 0
                })
            })
        );
        vm.stopPrank();

        // Test: revert when non-attester tries to revoke
        vm.prank(_sender2); 
        vm.expectRevert(AccessDenied.selector);
        eas.revoke(
            RevocationRequest({
                schema: schemaId,
                data: RevocationRequestData({ uid: uid, value: 0 })
            })
        );
    }

    /// @dev Tests comprehensive scenarios for irrevocable schemas.
    ///      1. Tests single attestation:
    ///         - Creates attestation with irrevocable schema
    ///         - Verifies attestation properties
    ///         - Confirms revocation attempt fails
    ///      2. Tests batch attestation:
    ///         - Creates multiple attestations
    ///         - Verifies all are properly marked irrevocable
    ///         - Confirms batch revocation attempts fail
    ///      Ensures irrevocable property is enforced in all scenarios
    function testIrrevocableSchemaScenarios(
        address _userAddress, 
        string memory _userName, 
        bool _isActive, 
        address _userAddress2, 
        string memory _userName2, 
        bool _isActive2,
        address _recipient,
        address _recipient2,
        uint64 _expirationOffset
    ) public {
        // Ensure the fuzzed expiration offset is within a reasonable range
        vm.assume(_expirationOffset > 0 && _expirationOffset <= 365 days);
        vm.assume(_recipient != address(0)); // Ensure recipient is not zero address
        vm.assume(_recipient2 != address(0)); // Ensure second recipient is not zero address

        string memory schema = "address userAddress,string userName,bool isActive";
        bytes32 schemaId = _getSchemaUID(schema, address(0), false);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), false);
        bytes memory data = abi.encode(
            _userAddress,
            _userName,
            _isActive 
        );
        bytes memory data2 = abi.encode(
            _userAddress2,
            _userName2,
            _isActive2 
        );

        // Create attestation
        bytes32 uid = eas.attest(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: _recipient, 
                    expirationTime: uint64(block.timestamp + _expirationOffset),
                    revocable: false,
                    refUID: ZERO_BYTES32,
                    data: data,
                    value: 0
                })
            })
        );

        // Verify attestation
        Attestation memory attestation = eas.getAttestation(uid);
        assertEq(attestation.attester, sender);
        assertEq(attestation.recipient, _recipient);
        assertFalse(attestation.revocable);

        // Should revert when trying to revoke
        vm.expectRevert(Irrevocable.selector);
        eas.revoke(
            RevocationRequest({
                schema: schemaId,
                data: RevocationRequestData({ uid: uid, value: 0 })
            })
        );

        // Test multi-attestation with irrevocable schema
        MultiAttestationRequest[] memory requests = new MultiAttestationRequest[](1);
        requests[0].schema = schemaId;
        requests[0].data = new AttestationRequestData[](2);
        requests[0].data[0] = AttestationRequestData({
            recipient: _recipient, // Use fuzzed recipient
            expirationTime: uint64(block.timestamp + _expirationOffset),
            revocable: false,
            refUID: ZERO_BYTES32,
            data: hex"1234",
            value: 0
        });
        requests[0].data[1] = AttestationRequestData({
            recipient: _recipient2, 
            expirationTime: uint64(block.timestamp + _expirationOffset),
            revocable: false,
            refUID: ZERO_BYTES32,
            data: data2,
            value: 0
        });

        bytes32[] memory uids = eas.multiAttest(requests);
        assertEq(uids.length, 2);

        // Verify multi-attestations
        for (uint i = 0; i < uids.length; i++) {
            attestation = eas.getAttestation(uids[i]);
            assertEq(attestation.attester, sender);
            assertFalse(attestation.revocable);
        }

        // Should revert when trying to revoke multiple attestations
        RevocationRequest[] memory revocationRequests = new RevocationRequest[](2);
        for (uint i = 0; i < 2; i++) {
            revocationRequests[i] = RevocationRequest({
                schema: schemaId,
                data: RevocationRequestData({ uid: uids[i], value: 0 })
            });
        }

        vm.expectRevert(Irrevocable.selector);
        eas.revoke(revocationRequests[0]);

        vm.stopPrank();
    }

    /// @dev Tests scenarios mixing revocable and irrevocable schemas.
    ///      1. Sets up two schemas:
    ///         - Revocable schema "bool like"
    ///         - Irrevocable schema "bool isFriend"
    ///      2. Creates one attestation for each schema
    ///      3. Verifies:
    ///         - Revocable attestation can be revoked
    ///         - Irrevocable attestation cannot be revoked
    ///      Ensures system correctly handles different revocation rules

    function testMixedRevocabilityScenarios(
        string memory _revocableSchemaProperty,
        string memory _irrevocableSchemaProperty,
        string memory _revocableSchemaValue,
        string memory _irrevocableSchemaValue,
        address _recipient,
        uint64 _expirationOffset
    ) public {
        // Ensure the fuzzed expiration offset is within a reasonable range
        vm.assume(_expirationOffset > 0 && _expirationOffset <= 365 days);
        vm.assume(_recipient != address(0)); // Ensure recipient is not zero address

        string memory revocableSchema = string.concat("string ", _revocableSchemaProperty);
        string memory irrevocableSchema = string.concat("string ", _irrevocableSchemaProperty);

        bytes32 revocableSchemaId = _getSchemaUID(revocableSchema, address(0), true);
        bytes32 irrevocableSchemaId = _getSchemaUID(irrevocableSchema, address(0), false);

        bytes memory data = abi.encode(
            _revocableSchemaValue
        );
        bytes memory data2 = abi.encode(
            _irrevocableSchemaValue
        );

        vm.startPrank(sender);
        schemaRegistry.register(revocableSchema, ISchemaResolver(address(0)), true);
        schemaRegistry.register(irrevocableSchema, ISchemaResolver(address(0)), false);

        // Create attestations with both schemas
        bytes32 revocableUid = eas.attest(
            AttestationRequest({
                schema: revocableSchemaId,
                data: AttestationRequestData({
                    recipient: _recipient, // Use fuzzed recipient
                    expirationTime: uint64(block.timestamp + _expirationOffset),
                    revocable: true,
                    refUID: ZERO_BYTES32,
                    data: data,
                    value: 0
                })
            })
        );

        bytes32 irrevocableUid = eas.attest(
            AttestationRequest({
                schema: irrevocableSchemaId,
                data: AttestationRequestData({
                    recipient: _recipient, // Use fuzzed recipient
                    expirationTime: uint64(block.timestamp + _expirationOffset),
                    revocable: false,
                    refUID: ZERO_BYTES32,
                    data: data2,
                    value: 0
                })
            })
        );

        // Should be able to revoke the revocable attestation
        eas.revoke(
            RevocationRequest({
                schema: revocableSchemaId,
                data: RevocationRequestData({ uid: revocableUid, value: 0 })
            })
        );

        // Should revert when trying to revoke the irrevocable attestation
        vm.expectRevert(Irrevocable.selector);
        eas.revoke(
            RevocationRequest({
                schema: irrevocableSchemaId,
                data: RevocationRequestData({ uid: irrevocableUid, value: 0 })
            })
        );

        vm.stopPrank();
    }

    /// @dev Tests revocation with invalid schema data.
    ///      1. Registers a valid schema "bool like"
    ///      2. Attempts to revoke using a different, unregistered schema
    ///      3. Verifies the revocation fails with InvalidSchema error
    ///      Ensures revocations are properly validated against registered schemas
    function testInvalidRevocationData(string memory _stringName, string memory _invalidSchema) public {
        vm.assume(bytes(_stringName).length > 0 && bytes(_invalidSchema).length < 32);
        string memory schema = string.concat("string ", _stringName);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);

        // Try to revoke with wrong schema
        bytes32 wrongSchemaId = _getSchemaUID(_invalidSchema, address(0), true);

        vm.expectRevert(InvalidSchema.selector);
        eas.revoke(
            RevocationRequest({
                schema: wrongSchemaId,
                data: RevocationRequestData({ uid: bytes32(0), value: 0 })
            })
        );
        vm.stopPrank();
    }

    // =============================================================
    //                     OFF-CHAIN REVOCATION TESTS
    // =============================================================
    /// @dev Tests off-chain revocation functionality.
    ///      1. Setup:
    ///         - Creates test data hash
    ///      2. Revocation:
    ///         - Executes off-chain revocation as sender
    ///         - Records current block timestamp
    ///      3. Verification:
    ///         - Confirms revocation timestamp matches
    ///         - Verifies using sender's address and data hash
    ///      Demonstrates off-chain revocation mechanism,
    ///      useful for revoking attestations without on-chain
    ///      transaction for each attestation
    function testRevokeOffchain(bytes memory _randomData) public {
        bytes32 data = keccak256(_randomData);

        vm.prank(sender);
        uint256 timestamp = block.timestamp;
        eas.revokeOffchain(data);

        assertEq(eas.getRevokeOffchain(sender, data), timestamp);
    }

    /// @dev Tests off-chain revocation behavior.
    ///      1. Performs initial off-chain revocation of test data
    ///      2. Attempts to revoke the same data again
    ///      3. Verifies second attempt fails with AlreadyRevokedOffchain
    ///      Ensures off-chain revocations cannot be duplicated
    function testRevokeOffchainRevert(bytes memory _randomData) public {
        bytes32 data = keccak256(_randomData);

        vm.startPrank(sender);
        // First revocation should succeed
        eas.revokeOffchain(data);

        // Second revocation should fail
        vm.expectRevert(AlreadyRevokedOffchain.selector);
        eas.revokeOffchain(data);
        vm.stopPrank();
    }

    /// @dev Tests batch off-chain revocation functionality.
    ///      1. Creates array of three different data hashes:
    ///         - Hash of "data1"
    ///         - Hash of "data2"
    ///         - Zero bytes32
    ///      2. Revokes all data in single transaction
    ///      3. Verifies each revocation is recorded with correct timestamp
    ///      Demonstrates efficient batch processing of off-chain revocations
    function testRevokeOffchainMultiple(bytes memory _randomData, bytes memory _randomData2) public {
        bytes32[] memory data = new bytes32[](3);
        data[0] = keccak256(_randomData);
        data[1] = keccak256(_randomData2);
        data[2] = bytes32(0);

        vm.prank(sender);
        uint256 timestamp = block.timestamp;
        eas.multiRevokeOffchain(data);

        for (uint i = 0; i < data.length; i++) {
            assertEq(eas.getRevokeOffchain(sender, data[i]), timestamp);
        }
    }

    /// @dev Tests off-chain revocation independence between accounts.
    ///      1. First account (sender) revokes specific data
    ///      2. Second account (sender2) revokes same data
    ///      3. Verifies both revocations are recorded separately with
    ///         their own timestamps, demonstrating that off-chain
    ///         revocations are account-specific
    function testRevokeOffchainDifferentAccounts(bytes memory _randomData) public {
        bytes32 data = keccak256(_randomData);

        // First account revokes
        vm.prank(sender);
        uint256 timestamp1 = block.timestamp;
        eas.revokeOffchain(data);

        // Second account should be able to revoke the same data
        vm.prank(sender2);
        uint256 timestamp2 = block.timestamp;
        eas.revokeOffchain(data);

        assertEq(eas.getRevokeOffchain(sender, data), timestamp1);
        assertEq(eas.getRevokeOffchain(sender2, data), timestamp2);
    }

    /// @dev Tests revert conditions for batch off-chain revocations.
    ///      1. Successfully revokes initial batch of data
    ///      2. Attempts to revoke same batch again (fails)
    ///      3. Attempts to revoke new batch containing previously revoked
    ///         data (fails). Ensures that batch revocations properly check
    ///         for already revoked data in all scenarios
    function testMultiRevokeOffchainRevert(bytes memory _randomData, bytes memory _randomData2, bytes memory _randomData3) public {
        bytes32[] memory data = new bytes32[](2);
        data[0] = keccak256(_randomData);
        data[1] = keccak256(_randomData2);

        vm.startPrank(sender);
        // First revocation should succeed
        eas.multiRevokeOffchain(data);

        // Second revocation should fail
        vm.expectRevert(AlreadyRevokedOffchain.selector);
        eas.multiRevokeOffchain(data);

        // Should also fail when including revoked data in a new array
        bytes32[] memory newData = new bytes32[](3);
        newData[0] = keccak256(_randomData3);
        newData[1] = data[0];
        newData[2] = data[1];

        vm.expectRevert(AlreadyRevokedOffchain.selector);
        eas.multiRevokeOffchain(newData);
        vm.stopPrank();
    }

    /// @dev Tests retrieval of unregistered off-chain revocations.
    ///      1. Setup:
    ///         - Creates hash of unregistered data
    ///      2. Verification:
    ///         - Checks revocation timestamp for unregistered data
    ///         - Confirms returns zero (default timestamp)
    ///      Ensures system properly handles queries for
    ///      non-existent off-chain revocations
    function testGetUnregisteredRevokeOffchain(bytes memory _randomData) public view {
        bytes32 data = keccak256(_randomData);
        assertEq(eas.getRevokeOffchain(sender, data), 0);
    }

    /// @dev Tests multiple account off-chain revocations of the same data.
    ///      1. First account (sender) revokes specific data
    ///      2. Second account (sender2) revokes same data
    ///      3. Verifies each account's revocation is recorded independently
    ///         with its own timestamp, demonstrating per-account revocation
    ///         storage
    function testRevokeOffchainMultipleAccounts(bytes memory _randomData) public {
        bytes32 data = keccak256(_randomData);

        // First account revokes
        vm.prank(sender);
        uint256 timestamp1 = block.timestamp;
        eas.revokeOffchain(data);

        // Second account should be able to revoke the same data
        vm.prank(sender2);
        uint256 timestamp2 = block.timestamp;
        eas.revokeOffchain(data);

        // Verify both revocations
        assertEq(eas.getRevokeOffchain(sender, data), timestamp1);
        assertEq(eas.getRevokeOffchain(sender2, data), timestamp2);
    }

    /// @dev Tests comprehensive scenarios for batch off-chain revocations.
    ///      1. First batch: Revokes three different data items
    ///      2. Second batch: Revokes two new data items
    ///      Verifies all revocations in both batches are recorded with
    ///      correct timestamps, demonstrating the system's ability to
    ///      handle multiple batch revocations from the same account
    function testMultiRevokeOffchainScenarios(
        uint256 _randomData1,
        uint256 _randomData2,
        uint256 _randomData3,
        uint256 _randomData4,
        uint256 _randomData5
    ) public {
        // Ensure that all random data inputs are unique
        vm.assume(
            _randomData1 != _randomData2 &&
            _randomData1 != _randomData3 &&
            _randomData1 != _randomData4 &&
            _randomData1 != _randomData5 &&
            _randomData2 != _randomData3 &&
            _randomData2 != _randomData4 &&
            _randomData2 != _randomData5 &&
            _randomData3 != _randomData4 &&
            _randomData3 != _randomData5 &&
            _randomData4 != _randomData5
        );

        // Create the first batch of data to revoke
        bytes32[] memory data = new bytes32[](3);
        data[0] = keccak256(abi.encodePacked(_randomData1));
        data[1] = keccak256(abi.encodePacked(_randomData2));
        data[2] = keccak256(abi.encodePacked(_randomData3));

        vm.startPrank(sender);

        // Test multiple revocations in one transaction
        uint256 timestamp = block.timestamp;
        eas.multiRevokeOffchain(data);

        // Verify all revocations
        for (uint i = 0; i < data.length; i++) {
            assertEq(eas.getRevokeOffchain(sender, data[i]), timestamp);
        }

        // Create the second batch of data to revoke
        bytes32[] memory data2 = new bytes32[](2);
        data2[0] = keccak256(abi.encodePacked(_randomData4));
        data2[1] = keccak256(abi.encodePacked(_randomData5));

        eas.multiRevokeOffchain(data2);

        // Verify second batch
        for (uint i = 0; i < data2.length; i++) {
            assertEq(eas.getRevokeOffchain(sender, data2[i]), timestamp);
        }

        vm.stopPrank();
    }

    // =============================================================
    //                     TIMESTAMP TESTS
    // =============================================================
    /// @dev Tests basic timestamping functionality.
    ///      1. Setup:
    ///         - Creates test data hash
    ///      2. Timestamping:
    ///         - Records current block timestamp
    ///         - Timestamps the data hash
    ///      3. Verification:
    ///         - Confirms stored timestamp matches
    ///           block timestamp at recording
    ///      Demonstrates basic timestamp recording and
    ///      retrieval functionality
    function testTimestamping(bytes memory _randomData) public {
        bytes32 data = keccak256(_randomData);

        uint256 timestamp = block.timestamp;
        eas.timestamp(data);

        assertEq(eas.getTimestamp(data), timestamp);
    }

    /// @dev Tests batch timestamping functionality.
    ///      Creates array of three different data items:
    ///      1. Hash of "data1"
    ///      2. Hash of "data2"
    ///      3. Zero bytes32
    ///      Records timestamps for all items in single transaction
    ///      and verifies each timestamp matches block timestamp
    function testTimestampMultiple(bytes memory _randomData, bytes memory _randomData2) public {
        vm.assume(bytes(_randomData).length > 0 && bytes(_randomData2).length > 0);
        bytes32[] memory data = new bytes32[](3);
        data[0] = keccak256(_randomData);
        data[1] = keccak256(_randomData2);
        data[2] = bytes32(0);

        uint256 timestamp = block.timestamp;
        eas.multiTimestamp(data);

        for (uint i = 0; i < data.length; i++) {
            assertEq(eas.getTimestamp(data[i]), timestamp);
        }
    }

    /// @dev Tests timestamp duplication prevention.
    ///      1. Successfully records initial timestamp
    ///      2. Attempts to timestamp same data again
    ///      3. Verifies second attempt reverts with AlreadyTimestamped
    ///      Ensures data can only be timestamped once
    function testTimestampRevert(bytes memory _randomData) public {
        bytes32 data = keccak256(_randomData);

        // First timestamp should succeed
        eas.timestamp(data);

        // Second timestamp should fail
        vm.expectRevert(AlreadyTimestamped.selector);
        eas.timestamp(data);
    }

    /// @dev Tests comprehensive batch timestamping scenarios.
    ///      1. First batch: Timestamps three different data items
    ///      2. Second batch: Timestamps two new data items
    ///      Verifies all timestamps in both batches match block
    ///      timestamp, demonstrating efficient batch processing
    ///      of multiple timestamp records
    function testMultiTimestampingScenarios(uint256 _randomData1,
        uint256 _randomData2,
        uint256 _randomData3,
        uint256 _randomData4,
        uint256 _randomData5
    ) public {
        // Ensure that all random data inputs are unique
        vm.assume(
            _randomData1 != _randomData2 &&
            _randomData1 != _randomData3 &&
            _randomData1 != _randomData4 &&
            _randomData1 != _randomData5 &&
            _randomData2 != _randomData3 &&
            _randomData2 != _randomData4 &&
            _randomData2 != _randomData5 &&
            _randomData3 != _randomData4 &&
            _randomData3 != _randomData5 &&
            _randomData4 != _randomData5
        );
        bytes32[] memory data = new bytes32[](3);
        data[0] = keccak256(abi.encodePacked(_randomData1));
        data[1] = keccak256(abi.encodePacked(_randomData2));
        data[2] = keccak256(abi.encodePacked(_randomData3));

        // Test multiple timestamps in one transaction
        uint256 timestamp = block.timestamp;
        eas.multiTimestamp(data);

        // Verify all timestamps
        for (uint i = 0; i < data.length; i++) {
            assertEq(eas.getTimestamp(data[i]), timestamp);
        }

        // Test second batch
        bytes32[] memory data2 = new bytes32[](2);
        data2[0] = keccak256(abi.encodePacked(_randomData4));
        data2[1] = keccak256(abi.encodePacked(_randomData5));

        eas.multiTimestamp(data2);

        // Verify second batch
        for (uint i = 0; i < data2.length; i++) {
            assertEq(eas.getTimestamp(data2[i]), timestamp);
        }
    }

    /// @dev Tests revert conditions for batch timestamping.
    ///      1. Successfully timestamps initial batch
    ///      2. Attempts to timestamp same batch again (fails)
    ///      3. Attempts to timestamp new batch containing previously
    ///         timestamped data (fails). Ensures proper duplicate
    ///         detection in all batch scenarios
    function testMultiTimestampRevert(uint256 _randomData1, uint256 _randomData2, uint256 _randomData3) public {
        bytes32[] memory data = new bytes32[](2);
        data[0] = keccak256(abi.encodePacked(_randomData1));
        data[1] = keccak256(abi.encodePacked(_randomData2));

        // First timestamp should succeed
        eas.multiTimestamp(data);

        // Second timestamp should fail
        vm.expectRevert(AlreadyTimestamped.selector);
        eas.multiTimestamp(data);

        // Should also fail when including timestamped data in a new array
        bytes32[] memory newData = new bytes32[](3);
        newData[0] = keccak256(abi.encodePacked(_randomData3));
        newData[1] = data[0];
        newData[2] = data[1];

        vm.expectRevert(AlreadyTimestamped.selector);
        eas.multiTimestamp(newData);
    }

    /// @dev Tests timestamp immutability and verification.
    ///      1. Records timestamps for multiple data items
    ///      2. Advances block time and verifies original timestamps remain unchanged
    ///      3. Records new timestamp and verifies it doesn't affect existing ones
    ///      4. Demonstrates timestamp immutability and independence
    ///      across different time periods and data items
    function testTimestampVerificationScenarios(uint256 _randomData1, uint256 _randomData2, uint256 _randomData3, uint256 _randomData4) public {
        vm.assume(_randomData1 != _randomData2 && 
        _randomData1 != _randomData3 && 
        _randomData1 != _randomData4 &&
        _randomData2 != _randomData3 &&
        _randomData2 != _randomData4 &&
        _randomData3 != _randomData4);
        bytes32[] memory data = new bytes32[](3);
        data[0] = keccak256(abi.encodePacked(_randomData1));
        data[1] = keccak256(abi.encodePacked(_randomData2));
        data[2] = keccak256(abi.encodePacked(_randomData3));

        // Test initial timestamps
        uint256 timestamp = block.timestamp;
        eas.multiTimestamp(data);

        // Verify all timestamps match block timestamp
        for (uint i = 0; i < data.length; i++) {
            assertEq(eas.getTimestamp(data[i]), timestamp);
        }

        // Advance time and verify timestamps don't change
        vm.warp(block.timestamp + 1 days);
        for (uint i = 0; i < data.length; i++) {
            assertEq(eas.getTimestamp(data[i]), timestamp);
        }

        // Test timestamp immutability
        bytes32 newData = keccak256(abi.encodePacked(_randomData4));
        eas.timestamp(newData);
        assertEq(eas.getTimestamp(newData), block.timestamp);

        // Verify original timestamps remain unchanged
        for (uint i = 0; i < data.length; i++) {
            assertEq(eas.getTimestamp(data[i]), timestamp);
        }
    }

    /// @dev Tests retrieval of unregistered timestamps.
    ///      1. Setup:
    ///         - Creates hash of unregistered data
    ///      2. Verification:
    ///         - Checks timestamp for unregistered data
    ///         - Confirms returns zero (default timestamp)
    ///      Ensures system properly handles queries for
    ///      non-existent timestamp records
    function testGetUnregisteredTimestamp(uint256 _randomData) public view {
        bytes32 data = keccak256(abi.encodePacked(_randomData));
        assertEq(eas.getTimestamp(data), 0);
    }

    // =============================================================
    //                   DELEGATION TESTS
    // =============================================================
    /// @dev Tests basic delegated attestation functionality.
    ///      1. Registers a schema and creates an attestation request
    ///      2. Performs attestation through delegation
    ///      3. Verifies the attestation is properly recorded with
    ///         correct attester and recipient addresses
    ///      Demonstrates standard delegated attestation flow
    function testDelegatedAttestation(string memory _name, uint256 _age, bool _isStudent, uint256 _expirationTimeOffset, uint256 _deadlineOffset) public {
        vm.assume(_expirationTimeOffset > 0 && _deadlineOffset > 0);
        vm.assume(_expirationTimeOffset < 365 && _deadlineOffset < 365);
        string memory schema = "string name,uint256 age,bool isStudent";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);

        vm.startPrank(attester);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);
        vm.stopPrank();

        bytes memory data = abi.encode(_name, _age, _isStudent);

        // Create attestation request data
        AttestationRequestData memory requestData = AttestationRequestData({
            recipient: recipient,
            expirationTime: uint64(block.timestamp + _expirationTimeOffset * 1 days),
            revocable: true,
            refUID: ZERO_BYTES32,
            data: data,
            value: 0
        });

        uint64 deadline = uint64(block.timestamp + _deadlineOffset * 1 days);
        bytes32 requestHash = _createAttestationDigest(schemaId, requestData, attester, deadline, 0);


        vm.startPrank(attester); 
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(attesterKey, requestHash); 
        vm.stopPrank();

        // Create the signature
        Signature memory signature = Signature({
            v: v,
            r: r,
            s: s
        });

        // Create the delegated attestation request
        DelegatedAttestationRequest memory request = DelegatedAttestationRequest({
            schema: schemaId,
            data: requestData,
            signature: signature,
            attester: attester, // The original attester who delegated authority
            deadline: deadline
        });

        // Test delegated attestation by the sender
        vm.startPrank(sender); // Switch to the sender who is performing the attestation
        bytes32 uid = eas.attestByDelegation(request);

        // Retrieve the attestation
        Attestation memory attestation = eas.getAttestation(uid);
        assertEq(attestation.attester, attester); // Check that the attester is the original attester
        assertEq(attestation.recipient, recipient); // Check that the recipient is correct
        vm.stopPrank();
    }

    /// @dev Tests delegated attestations with signatures.
    ///      1. Tests single attestation with invalid signature:
    ///         - Creates request with mock signature values
    ///         - Verifies it reverts with InvalidSignature
    ///      2. Tests batch attestation with invalid signatures:
    ///         - Creates multiple requests with mock signatures
    ///         - Verifies batch also reverts with InvalidSignature
    ///      Ensures signature validation works for both single and batch cases
    function testDelegatedAttestationInvalidSignatureReverts(
        uint8 _v, 
        bytes32 _r, 
        bytes32 _s, 
        string memory _name, 
        uint256 _age, 
        bool _isStudent
    ) public {
        string memory schema = "string name,uint256 age,bool isStudent";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);

        bytes memory data = abi.encode(_name, _age, _isStudent);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);

        // Test single delegated attestation with signature
        DelegatedAttestationRequest
            memory request = DelegatedAttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: uint64(block.timestamp + 30 days),
                    revocable: true,
                    refUID: ZERO_BYTES32,
                    data: data,
                    value: 0
                }),
                signature: Signature({ 
                    v: _v, 
                    r: _r, 
                    s: _s 
                }),
                attester: sender,
                deadline: type(uint64).max
            });

        vm.expectRevert(abi.encodeWithSignature("InvalidSignature()"));
        eas.attestByDelegation(request);

        // Test multi-attestation with signatures
        MultiDelegatedAttestationRequest[]
            memory requests = new MultiDelegatedAttestationRequest[](2);
        requests[0].schema = schemaId;
        requests[0].data = new AttestationRequestData[](1);
        requests[0].signatures = new Signature[](1);
        requests[0].data[0] = AttestationRequestData({
            recipient: recipient,
            expirationTime: uint64(block.timestamp + 30 days),
            revocable: true,
            refUID: ZERO_BYTES32,
            data: data,
            value: 0
        });
        requests[0].signatures[0] = Signature({ 
            v: _v, 
            r: _r, 
            s: _s 
        });
        requests[0].attester = sender;
        requests[0].deadline = type(uint64).max;

        vm.expectRevert(abi.encodeWithSignature("InvalidSignature()"));
        eas.multiAttestByDelegation(requests);

        vm.stopPrank();
    }

    /// @dev Tests delegated attestation with various deadline scenarios.
    ///      Tests three different deadline configurations:
    ///      1. Short deadline (1 hour)
    ///      2. Medium deadline (1 day)
    ///      3. Maximum possible deadline (type(uint64).max)
    ///      Verifies that while signature validation fails as expected,
    ///      deadline validation passes in all cases
    function testDelegatedAttestationTimeScenarios(
        string memory _name, 
        uint256 _age, 
        bool _isStudent, 
        uint256 _expirationTimeOffset, 
        uint256 _deadlineOffset
    ) public {
        vm.assume(_expirationTimeOffset > 0 && _deadlineOffset > 0);
        vm.assume(_expirationTimeOffset < 365 && _deadlineOffset < 365);
        string memory schema = "string name,uint256 age,bool isStudent";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);
        bytes memory data = abi.encode(_name, _age, _isStudent);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);

        // Test with different time scenarios
        uint64[] memory deadlines = new uint64[](3);
        deadlines[0] = uint64(block.timestamp + 1 hours);
        deadlines[1] = uint64(block.timestamp + 1 days);
        deadlines[2] = type(uint64).max;

        for (uint i = 0; i < deadlines.length; i++) {
                   // Test single delegated attestation
            AttestationRequestData memory requestData = AttestationRequestData({
                recipient: recipient,
                expirationTime: uint64(block.timestamp + 30 days),
                revocable: true,
                refUID: ZERO_BYTES32,
                data: data,
                value: 0
            });
            vm.startPrank(attester); 
             bytes32 requestHash = _createAttestationDigest(schemaId, requestData, attester, deadlines[i], i);
            (uint8 v, bytes32 r, bytes32 s) = vm.sign(attesterKey, requestHash); 
            vm.stopPrank();
            
            DelegatedAttestationRequest
                memory request = DelegatedAttestationRequest({
                    schema: schemaId,
                    data: AttestationRequestData({
                        recipient: recipient,
                        expirationTime: uint64(block.timestamp + 30 days),
                        revocable: true,
                        refUID: ZERO_BYTES32,
                        data: data,
                        value: 0
                    }),
                    signature: Signature({
                        v: v,
                        r: r,
                        s: s
                    }),
                    attester: attester,
                    deadline: deadlines[i]
                });

            bytes32 uid = eas.attestByDelegation(request);
           
            Attestation memory attestation = eas.getAttestation(uid);
            assertEq(attestation.attester, attester); // Check that the attester is the original attester
            assertEq(attestation.recipient, recipient); // Check that the recipient is correct
           
        }

        vm.stopPrank();
    }

    /// @dev Tests batch attestation delegation functionality.
    ///      1. Creates two attestation requests:
    ///         - First to recipient with specific data
    ///         - Second to recipient2 with different data
    ///      2. Processes both attestations in single transaction
    ///      3. Verifies both attestations are recorded correctly
    ///         with proper recipient addresses
    ///      Demonstrates efficient batch processing of delegated attestations


    function testMultiAttestationDelegation(    
        address _recipient,
        address _recipient2
    ) public {
        bytes32 schemaId = _getSchemaUID("bool like", address(0), true);

        vm.startPrank(sender);
        schemaRegistry.register("bool like", ISchemaResolver(address(0)), true);

        bytes[] memory dataArray = new bytes[](2);
        dataArray[0] = hex"1234"; 
        dataArray[1] = hex"5678";

        AttestationRequestData[] memory requestDataArray = new AttestationRequestData[](2);
        requestDataArray[0] = AttestationRequestData({
            recipient: _recipient,
            expirationTime: uint64(block.timestamp + 30 days),
            revocable: true,
            refUID: ZERO_BYTES32,
                data: dataArray[0],
                value: 0
            });
        requestDataArray[1] = AttestationRequestData({
            recipient: _recipient2,
            expirationTime: uint64(block.timestamp + 30 days),
            revocable: true,
            refUID: ZERO_BYTES32,
            data: dataArray[1],
            value: 0
            });

        uint64 deadline = uint64(block.timestamp + 1 days);
        bytes32[] memory requestHashes = new bytes32[](2);
        requestHashes[0] = _createAttestationDigest(schemaId, requestDataArray[0], attester, deadline, 0);
        requestHashes[1] = _createAttestationDigest(schemaId, requestDataArray[1], attester, deadline, 1);

  // Loop through the request hashes to sign them
    SignatureComponents[2] memory signatures;
    for (uint256 i = 0; i < requestHashes.length; i++) {
        (signatures[i].v, signatures[i].r, signatures[i].s) = vm.sign(attesterKey, requestHashes[i]);
    }

        MultiDelegatedAttestationRequest[] memory multiRequest = new MultiDelegatedAttestationRequest[](1);
        multiRequest[0].schema = schemaId;
        multiRequest[0].data = new AttestationRequestData[](2);
        multiRequest[0].signatures = new Signature[](2);
        multiRequest[0].attester = attester;
        multiRequest[0].deadline = deadline;

        multiRequest[0].data[0] = requestDataArray[0];
        multiRequest[0].data[1] = requestDataArray[1];
        multiRequest[0].signatures[0] = Signature({
            v: signatures[0].v, 
            r: signatures[0].r,
            s: signatures[0].s
        });
        multiRequest[0].signatures[1] = Signature({
            v: signatures[1].v,
            r: signatures[1].r,
            s: signatures[1].s
        });

        // Test multi-attestation
        bytes32[] memory uids = eas.multiAttestByDelegation(multiRequest);
        assertEq(uids.length, 2);

        // Verify attestations
        Attestation[] memory attestations = new Attestation[](2);
        attestations[0] = eas.getAttestation(uids[0]);
        attestations[1] = eas.getAttestation(uids[1]);

        assertEq(attestations[0].recipient, _recipient);
        assertEq(attestations[1].recipient, _recipient2);
        vm.stopPrank();
    }

        /// @dev Creates and verifies multiple delegated attestations.
    ///      Setup for each request:
    ///      1. Request Configuration:
    ///         - Sets schema ID and deadline
    ///         - Initializes data and signature arrays
    ///         - Assigns unique attester from signers array
    ///      2. Attestation Data:
    ///         - Sets 30-day expiration
    ///         - Makes revocable
    ///         - Uses incremental data values
    ///      3. Signature Creation:
    ///         - Generates EIP-712 digest
    ///         - Signs with corresponding signer key
    ///      4. Verification:
    ///         - Submits batch through first signer
    ///         - Verifies each attestation:
    ///           * Confirms correct attester
    ///           * Validates recipient
    ///      Demonstrates complete flow of multiple delegated
    ///      attestations with different signers

    function testMultiAttestationDelegationWithUniqueSigners() public {
        bytes32 schemaId = _registerSchema("bool like", true);
        uint64 deadline = uint64(block.timestamp + 1 days);

        // Create multiple signers
        uint256[] memory signerKeys = new uint256[](3);
        address[] memory signers = new address[](3);
        for (uint i = 0; i < 3; i++) {
            signerKeys[i] = 0x12345 + i;
            signers[i] = vm.addr(signerKeys[i]);
            vm.deal(signers[i], 100 ether);
        }

        MultiDelegatedAttestationRequest[]
            memory requests = new MultiDelegatedAttestationRequest[](3);
        for (uint i = 0; i < 3; i++) {
            requests[i].schema = schemaId;
            requests[i].data = new AttestationRequestData[](1);
            requests[i].signatures = new Signature[](1);
            requests[i].attester = signers[i]; // Each request has its own attester
            requests[i].deadline = deadline;

            requests[i].data[0] = AttestationRequestData({
                recipient: recipient,
                expirationTime: uint64(block.timestamp + 30 days),
                revocable: true,
                refUID: ZERO_BYTES32,
                data: abi.encodePacked(bytes1(uint8(i + 1))),
                value: 0
            });

            bytes32 digest = _createAttestationDigest(
                schemaId,
                requests[i].data[0],
                signers[i],
                deadline,
                0
            );

            (uint8 v, bytes32 r, bytes32 s) = vm.sign(signerKeys[i], digest);
            requests[i].signatures[0] = Signature({ v: v, r: r, s: s });
        }

        vm.prank(signers[0]);
        bytes32[] memory uids = eas.multiAttestByDelegation(requests);

        // Verify attestations
        for (uint i = 0; i < uids.length; i++) {
            Attestation memory attestation = eas.getAttestation(uids[i]);
            assertEq(attestation.attester, signers[i]);
            assertEq(attestation.recipient, recipient);
        }
    }


    /// @dev Tests error conditions for delegated attestations.
    ///      1. Tests empty arrays:
    ///         - Attempts delegation with empty data/signature arrays
    ///         - Verifies revert with InvalidLength
    ///      2. Tests mismatched lengths:
    ///         - Uses different sizes for data and signature arrays
    ///         - Verifies revert with InvalidLength
    ///      3. Tests expired deadline:
    ///         - Attempts delegation with past deadline
    ///         - Verifies revert with DeadlineExpired
    ///      Ensures proper validation of delegation parameters
    function testDelegatedAttestationReverts() public {
    
        string memory schema = "bool like";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);

        // Test 1: Empty data arrays

        MultiDelegatedAttestationRequest[]
            memory requests = new MultiDelegatedAttestationRequest[](1);

        requests[0] = MultiDelegatedAttestationRequest({
            schema: schemaId,
            data: new AttestationRequestData[](0),
            signatures: new Signature[](0),
            attester: sender,
            deadline: uint64(block.timestamp + 1)
        });

        vm.expectRevert(abi.encodeWithSelector(InvalidLength.selector));
        eas.multiAttestByDelegation(requests);

        // Test 2: Mismatched lengths
        requests[0].data = new AttestationRequestData[](2);
        requests[0].signatures = new Signature[](1);

        vm.expectRevert(abi.encodeWithSelector(InvalidLength.selector));
        eas.multiAttestByDelegation(requests);

        // Test 3: Invalid signature first
        AttestationRequestData[] memory data = new AttestationRequestData[](1);
        data[0] = AttestationRequestData({
            recipient: recipient,
            expirationTime: NO_EXPIRATION,
            revocable: true,
            refUID: ZERO_BYTES32,
            data: hex"1234",
            value: 0
        });

        Signature[] memory sigs = new Signature[](1);
        sigs[0] = Signature({ v: 27, r: bytes32(0), s: bytes32(0) });

        requests[0] = MultiDelegatedAttestationRequest({
            schema: schemaId,
            data: data,
            signatures: sigs,
            attester: sender,
            deadline: uint64(block.timestamp - 1) 
        }); 

        vm.expectRevert(abi.encodeWithSelector(DeadlineExpired.selector));
        eas.multiAttestByDelegation(requests);

        vm.stopPrank();
    }

    /// @dev Tests delegated revocation functionality.
    ///      1. Creates a revocable attestation with 30-day expiration
    ///      2. Revokes the attestation through delegation
    ///      3. Verifies revocation by checking revocationTime is set
    ///      Demonstrates complete flow of attestation creation
    ///      and subsequent delegated revocation
    function testDelegatedRevocation() public {
        string memory schema = "bool like";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);

        // Create attestation first
        bytes32 uid = eas.attest(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: uint64(block.timestamp + 30 days),
                    revocable: true,
                    refUID: ZERO_BYTES32,
                    data: hex"1234",
                    value: 0
                })
            })
        );

        // Test delegated revocation
        eas.revoke(
            RevocationRequest({
                schema: schemaId,
                data: RevocationRequestData({ uid: uid, value: 0 })
            })
        );

        Attestation memory attestation = eas.getAttestation(uid);
        assertTrue(attestation.revocationTime > 0);
        vm.stopPrank();
    }

    /// @dev Tests comprehensive delegated revocation scenarios.
    ///      1. Tests single revocation:
    ///         - Creates and revokes one attestation
    ///         - Verifies revocation timestamp
    ///      2. Tests multiple revocations:
    ///         - Creates two new attestations
    ///         - Revokes them individually
    ///         - Verifies each revocation timestamp
    ///      Demonstrates both single and multiple revocation patterns
    ///      through delegation
    function testDelegatedRevocationScenarios() public {
        string memory schema = "bool like";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);

        // Create attestation first
        bytes32 uid = eas.attest(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: uint64(block.timestamp + 30 days),
                    revocable: true,
                    refUID: ZERO_BYTES32,
                    data: hex"1234",
                    value: 0
                })
            })
        );

        // Test delegated revocation
        eas.revoke(
            RevocationRequest({
                schema: schemaId,
                data: RevocationRequestData({ uid: uid, value: 0 })
            })
        );

        Attestation memory attestation = eas.getAttestation(uid);
        assertTrue(attestation.revocationTime > 0);

        // Test multi-revocation
        bytes32[] memory uids = new bytes32[](2);
        for (uint i = 0; i < 2; i++) {
            uids[i] = eas.attest(
                AttestationRequest({
                    schema: schemaId,
                    data: AttestationRequestData({
                        recipient: recipient,
                        expirationTime: uint64(block.timestamp + 30 days),
                        revocable: true,
                        refUID: ZERO_BYTES32,
                        data: hex"5678",
                        value: 0
                    })
                })
            );
        }

        RevocationRequest[] memory revocationRequests = new RevocationRequest[](
            2
        );
        for (uint i = 0; i < 2; i++) {
            revocationRequests[i] = RevocationRequest({
                schema: schemaId,
                data: RevocationRequestData({ uid: uids[i], value: 0 })
            });
        }

        for (uint i = 0; i < 2; i++) {
            eas.revoke(revocationRequests[i]);
            attestation = eas.getAttestation(uids[i]);
            assertTrue(attestation.revocationTime > 0);
        }

        vm.stopPrank();
    }

    /// @dev Tests error conditions for multi-attestation delegation revocation.
    ///      1. Tests mismatched array lengths:
    ///         - Two revocation requests with one signature
    ///         - Verifies revert with InvalidLength
    ///      2. Tests empty data array:
    ///         - Zero revocation requests
    ///         - Verifies revert with InvalidLength
    ///      3. Tests empty signature array:
    ///         - One revocation request with no signatures
    ///         - Verifies revert with InvalidLength
    ///      Ensures proper validation of batch revocation parameters
    function testMultiAttestationDelegationRevert() public {
        string memory schema = "bool like";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);

        // Create two attestations
        bytes32 uid1 = eas.attest(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: uint64(block.timestamp + 30 days),
                    revocable: true,
                    refUID: ZERO_BYTES32,
                    data: hex"1234",
                    value: 0
                })
            })
        );

        bytes32 uid2 = eas.attest(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: uint64(block.timestamp + 30 days),
                    revocable: true,
                    refUID: ZERO_BYTES32,
                    data: hex"5678",
                    value: 0
                })
            })
        );

        // Test revert with inconsistent lengths
        MultiDelegatedRevocationRequest[]
            memory requests = new MultiDelegatedRevocationRequest[](1);
        requests[0].schema = schemaId;
        requests[0].data = new RevocationRequestData[](2);
        requests[0].data[0].uid = uid1;
        requests[0].data[0].value = 0;
        requests[0].data[1].uid = uid2;
        requests[0].data[1].value = 0;

        requests[0].signatures = new Signature[](1);
        requests[0].signatures[0] = Signature({
            v: 28,
            r: bytes32(uint256(1)),
            s: bytes32(uint256(2))
        });
        requests[0].revoker = sender;
        requests[0].deadline = type(uint64).max;

        vm.expectRevert(InvalidLength.selector);
        eas.multiRevokeByDelegation(requests);

        // Test revert with empty data
        requests[0].data = new RevocationRequestData[](0);
        vm.expectRevert(InvalidLength.selector);
        eas.multiRevokeByDelegation(requests);

        // Test revert with empty signatures
        requests[0].data = new RevocationRequestData[](1);
        requests[0].data[0].uid = uid1;
        requests[0].signatures = new Signature[](0);
        vm.expectRevert(InvalidLength.selector);
        eas.multiRevokeByDelegation(requests);

        vm.stopPrank();
    }

    /// @dev Tests basic attestation without value transfer.
    ///      Creates a simple attestation with:
    ///      - 30-day expiration
    ///      - Revocable flag set
    ///      - No ETH value
    ///      Verifies the attestation is properly recorded with
    ///      correct attester address. Note: Despite function name,
    ///      this test focuses on basic attestation functionality
    ///      rather than value transfer scenarios
    function testAttestationValueTransferScenarios() public {
        // Remove value transfers, just test basic attestation
        string memory schema = "bool like";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);

        AttestationRequest memory request = AttestationRequest({
            schema: schemaId,
            data: AttestationRequestData({
                recipient: recipient,
                expirationTime: uint64(block.timestamp + 30 days),
                revocable: true,
                refUID: ZERO_BYTES32,
                data: hex"1234",
                value: 0
            })
        });

        bytes32 uid = eas.attest(request);
        Attestation memory attestation = eas.getAttestation(uid);
        assertEq(attestation.attester, sender);
        vm.stopPrank();
    }


    /// @dev Tests array length validation in multi-delegated attestations.
    ///      Tests four scenarios with inconsistent array lengths:
    ///      1. More data items than signatures
    ///      2. Empty data array with signatures
    ///      3. More signatures than data items
    ///      4. Data items with empty signatures array
    ///      Verifies all cases revert with InvalidLength error,
    ///      ensuring proper validation of array lengths in
    ///      multi-delegated attestations
    function testRevertMultiDelegationInconsistentLengths() public {
        // Register a schema
        string memory schema = "bool count,bytes32 id";
        bytes32 schemaId = _registerSchema(schema, true);

        vm.startPrank(sender);

        // Test 1: More data items than signatures
        MultiDelegatedAttestationRequest[]
            memory requests1 = new MultiDelegatedAttestationRequest[](1);
        requests1[0] = MultiDelegatedAttestationRequest({
            schema: schemaId,
            data: new AttestationRequestData[](2),
            signatures: new Signature[](1),
            attester: sender,
            deadline: NO_EXPIRATION
        });
        vm.expectRevert(InvalidLength.selector);
        eas.multiAttestByDelegation(requests1);

        // Test 2: Empty data array with signatures
        MultiDelegatedAttestationRequest[]
            memory requests2 = new MultiDelegatedAttestationRequest[](1);
        requests2[0] = MultiDelegatedAttestationRequest({
            schema: schemaId,
            data: new AttestationRequestData[](0),
            signatures: new Signature[](1),
            attester: sender,
            deadline: NO_EXPIRATION
        });
        vm.expectRevert(InvalidLength.selector);
        eas.multiAttestByDelegation(requests2);

        // Test 3: More signatures than data items
        MultiDelegatedAttestationRequest[]
            memory requests3 = new MultiDelegatedAttestationRequest[](1);
        requests3[0] = MultiDelegatedAttestationRequest({
            schema: schemaId,
            data: new AttestationRequestData[](1),
            signatures: new Signature[](2),
            attester: sender,
            deadline: NO_EXPIRATION
        });
        vm.expectRevert(InvalidLength.selector);
        eas.multiAttestByDelegation(requests3);

        // Test 4: Data items with empty signatures array
        MultiDelegatedAttestationRequest[]
            memory requests4 = new MultiDelegatedAttestationRequest[](1);
        requests4[0] = MultiDelegatedAttestationRequest({
            schema: schemaId,
            data: new AttestationRequestData[](1),
            signatures: new Signature[](0),
            attester: sender,
            deadline: NO_EXPIRATION
        });
        vm.expectRevert(InvalidLength.selector);
        eas.multiAttestByDelegation(requests4);

        vm.stopPrank();
    }


    // =============================================================
    //                      SCHEMA TESTS
    // =============================================================
    /// @dev Tests attestation scenarios with schema resolvers.
    ///      1. Creates a payable mock resolver
    ///      2. Registers schema with resolver
    ///      3. Creates attestation using resolver-enabled schema
    ///      4. Verifies attestation is properly recorded
    ///      Demonstrates integration between attestations and
    ///      schema resolvers
    function testSchemaResolverScenarios() public {
        string memory schema = "bool like";
        bytes32 schemaId = _getSchemaUID(schema, address(payableResolver), true);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(payableResolver)), true);

        // Test attestation with resolver
        bytes32 uid = eas.attest(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: uint64(block.timestamp + 30 days),
                    revocable: true,
                    refUID: ZERO_BYTES32,
                    data: hex"1234",
                    value: 0
                })
            })
        );

        Attestation memory attestation = eas.getAttestation(uid);
        assertEq(attestation.attester, sender);

        vm.stopPrank();
    }

    /// @dev Tests schema registration scenarios.
    ///      1. Tests successful registration:
    ///         - Registers basic boolean schema
    ///      2. Tests duplicate registration:
    ///         - Attempts to register same schema again
    ///         - Verifies revert with AlreadyExists
    ///      3. Tests complex schema:
    ///         - Registers schema with multiple fields
    ///      Ensures proper schema registration and uniqueness
    function testSchemaRegistrationScenarios() public {
        string memory schema = "bool like";

        vm.startPrank(sender);

        // Test basic registration
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);

        // Test registering same schema again (should revert)
        vm.expectRevert(abi.encodeWithSignature("AlreadyExists()"));
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);

        // Test different schema types
        string
            memory complexSchema = "uint256 age, string name, address wallet";
        schemaRegistry.register(complexSchema, ISchemaResolver(address(0)), true);

        vm.stopPrank();
    }

    // =============================================================
    //                      COMPLEX SCENARIOS
    // =============================================================
    /// @dev Tests attestation referencing functionality.
    ///      1. Creates initial attestation (parent)
    ///      2. Creates second attestation referencing the first
    ///      3. Verifies reference UID is correctly recorded
    ///      Demonstrates the ability to create linked attestations
    ///      through reference UIDs
    function testReferenceAttestation() public {
        string memory schema = "bool like";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);

        // Create first attestation
        bytes32 refUid = eas.attest(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: uint64(block.timestamp + 30 days),
                    revocable: true,
                    refUID: ZERO_BYTES32,
                    data: hex"1234",
                    value: 0
                })
            })
        );

        // Create attestation referencing the first one
        bytes32 uid = eas.attest(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: uint64(block.timestamp + 30 days),
                    revocable: true,
                    refUID: refUid,
                    data: hex"5678",
                    value: 0
                })
            })
        );

        Attestation memory attestation = eas.getAttestation(uid);
        assertEq(attestation.refUID, refUid);
        vm.stopPrank();
    }

    /// @dev Tests revocation behavior in referenced attestations.
    ///      1. Creates chain of three linked attestations:
    ///         - Root attestation
    ///         - Two child attestations each referencing previous
    ///      2. Revokes root attestation
    ///      3. Verifies child attestations remain valid
    ///      Demonstrates that revocation does not cascade through
    ///      referenced attestations
    function testCascadingRevocationScenarios() public {
        string memory schema = "bool like";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);

        // Create chain of attestations
        bytes32[] memory uids = new bytes32[](3);

        // Root attestation
        uids[0] = eas.attest(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: uint64(block.timestamp + 30 days),
                    revocable: true,
                    refUID: ZERO_BYTES32,
                    data: hex"1234",
                    value: 0
                })
            })
        );

        // Create chain of references
        for (uint i = 1; i < 3; i++) {
            uids[i] = eas.attest(
                AttestationRequest({
                    schema: schemaId,
                    data: AttestationRequestData({
                        recipient: recipient,
                        expirationTime: uint64(block.timestamp + 30 days),
                        revocable: true,
                        refUID: uids[i - 1],
                        data: hex"5678",
                        value: 0
                    })
                })
            );
        }

        // Revoke root
        eas.revoke(
            RevocationRequest({
                schema: schemaId,
                data: RevocationRequestData({ uid: uids[0], value: 0 })
            })
        );

        // Verify children remain valid
        for (uint i = 1; i < 3; i++) {
            Attestation memory attestation = eas.getAttestation(uids[i]);
            assertEq(attestation.revocationTime, 0);
        }

        vm.stopPrank();
    }

    /// @dev Tests complex revocation scenarios with multiple attestations.
    ///      Creates three attestations with different configurations:
    ///      1. Standard revocable attestation
    ///      2. Attestation with value
    ///      3. Attestation referencing first attestation
    ///      Tests revocation behavior:
    ///      - Revokes parent attestation
    ///      - Verifies child can still be revoked after parent
    ///      - Confirms second attestation remains unaffected
    ///      Demonstrates independence of revocations and proper
    ///      handling of referenced attestations in complex scenarios
    function testAdvancedRevocationScenarios() public {
        string memory schema = "bool like";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);

        // Create multiple attestations with different configurations
        bytes32[] memory uids = new bytes32[](3);

        // First attestation: standard revocable
        uids[0] = eas.attest(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: uint64(block.timestamp + 30 days),
                    revocable: true,
                    refUID: ZERO_BYTES32,
                    data: hex"1234",
                    value: 0
                })
            })
        );

        // Second attestation: with value
        uids[1] = eas.attest(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: uint64(block.timestamp + 30 days),
                    revocable: true,
                    refUID: ZERO_BYTES32,
                    data: hex"5678",
                    value: 0
                })
            })
        );

        // Third attestation: with reference to first
        uids[2] = eas.attest(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: uint64(block.timestamp + 30 days),
                    revocable: true,
                    refUID: uids[0],
                    data: hex"9012",
                    value: 0
                })
            })
        );

        // Test revocation order effects
        eas.revoke(
            RevocationRequest({
                schema: schemaId,
                data: RevocationRequestData({ uid: uids[0], value: 0 })
            })
        );

        // Verify referenced attestation can still be revoked after its reference is revoked
        eas.revoke(
            RevocationRequest({
                schema: schemaId,
                data: RevocationRequestData({ uid: uids[2], value: 0 })
            })
        );

        // Verify attestation states
        for (uint i = 0; i < uids.length; i++) {
            Attestation memory attestation = eas.getAttestation(uids[i]);
            if (i == 0 || i == 2) {
                // First and third attestations should be revoked
                assertTrue(
                    attestation.revocationTime > 0,
                    "Attestation should be revoked"
                );
            } else {
                // Second attestation should not be revoked
                assertTrue(
                    attestation.revocationTime == 0,
                    "Attestation should not be revoked"
                );
            }
        }

        vm.stopPrank();
    }

    /// @dev Tests deadline validation in delegated attestations.
    ///      1. Creates attestation request with expired deadline
    ///         (timestamp set to past)
    ///      2. Attempts to attest by delegation
    ///      3. Verifies transaction reverts with DeadlineExpired
    ///      Ensures proper enforcement of temporal constraints
    ///      in delegated attestations
    function testDeadlineScenarios() public {
        
        string memory schema = "bool like";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);

        DelegatedAttestationRequest memory request = DelegatedAttestationRequest({
            schema: schemaId,
            data: AttestationRequestData({
                recipient: recipient,
                expirationTime: uint64(block.timestamp + 30 days),
                revocable: true,
                refUID: ZERO_BYTES32,
                data: hex"1234",
                value: 0
            }),
            signature: Signature({
                v: 28,
                r: bytes32(uint256(1)),
                s: bytes32(uint256(2))
            }),
            attester: sender,
            deadline: uint64(block.timestamp - 1) 
        });

        vm.expectRevert(DeadlineExpired.selector);
        eas.attestByDelegation(request);
        vm.stopPrank();
    }

}
