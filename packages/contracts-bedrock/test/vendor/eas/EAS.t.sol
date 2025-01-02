// SPDX-License-Identifier: MIT
pragma solidity =0.8.15;

// imports
import { CommonTest } from "test/setup/CommonTest.sol";
import { IEAS } from "src/vendor/eas/IEAS.sol";
import { ISchemaRegistry } from "src/vendor/eas/ISchemaRegistry.sol";
import { IEAS_EIP1271 } from "src/vendor/eas/IEAS_EIP1271.sol";
import {
    Attestation,
    AttestationRequest,
    AttestationRequestData,
    MultiAttestationRequest,
    RevocationRequest,
    DelegatedRevocationRequest,
    RevocationRequestData,
    MultiDelegatedAttestationRequest,
    MultiDelegatedRevocationRequest,
    DelegatedAttestationRequest,
    MultiRevocationRequest,
    Signature
} from "src/vendor/eas/IEAS.sol";
import { ISchemaResolver } from "src/vendor/eas/resolver/ISchemaResolver.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";
import { EIP712 } from "@openzeppelin/contracts/utils/cryptography/draft-EIP712.sol";
import { ISemver } from "interfaces/universal/ISemver.sol";

/// @dev Helper contract for testing EIP712 signature verification
contract TestEIP712Helper is EIP712 {
    constructor() EIP712("EAS", "1.3.0") { }

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

// Mainnet Contract
contract EASTest is CommonTest {
    // Constants
    uint64 constant NO_EXPIRATION = 0;
    bytes32 constant ZERO_BYTES32 = bytes32(0);
    bytes32 private constant ATTEST_TYPEHASH = 0xfeb2925a02bae3dae48d424a0437a2b6ac939aa9230ddc55a1a76f065d988076;
    bytes32 constant REVOKE_TYPEHASH = 0xb5d556f07587ec0f08cf386545cc4362c702a001650c2058002615ee5c9d1e75;
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

    // Error Selectors
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

    // Test State
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

    // Helper Functions
    /// @dev Calculates the unique identifier for a schema based on its parameters
    function _getSchemaUID(string memory schema, address resolver, bool revocable) internal pure returns (bytes32) {
        return keccak256(abi.encodePacked(schema, resolver, revocable));
    }

    /// @dev Registers a test schema with specified parameters
    function _registerSchema(string memory schema, bool revocable) internal returns (bytes32) {
        bytes32 schemaId = _getSchemaUID(schema, address(0), revocable);
        vm.prank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), revocable);
        return schemaId;
    }

    /// @dev Registers a test schema with specified parameters
    function _registerFuzzSchema(string memory schema, address resolver, bool revocable) internal returns (bytes32) {
        bytes32 schemaId = _getSchemaUID(schema, resolver, revocable);
        vm.prank(sender);
        schemaRegistry.register(schema, ISchemaResolver(resolver), revocable);
        return schemaId;
    }

    function _signMessage(bytes32 hash, uint256 privateKey) internal pure returns (Signature memory) {
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(privateKey, hash);
        return Signature({ v: v, r: r, s: s });
    }

    /// @dev Returns the type hash for delegated attestations
    function _getDelegatedAttestationTypeHash() internal pure returns (bytes32) {
        return ATTEST_TYPEHASH;
    }

    /// @dev Creates standard attestation request data for testing
    function _createAttestationRequestData() internal view returns (AttestationRequestData memory) {
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
    )
        internal
        view
        returns (bytes32)
    {
        bytes32 structHash = keccak256(
            abi.encode(
                _getDelegatedAttestationTypeHash(),
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

        return keccak256(abi.encodePacked("\x19\x01", _createDomainSeparator(), structHash));
    }

    function _createRevocationDigest(
        bytes32 schemaId,
        bytes32 uid,
        address revoker,
        uint64 deadline,
        uint256 nonce
    )
        public
        view
        returns (bytes32)
    {
        bytes32 structHash = keccak256(
            abi.encode(
                REVOKE_TYPEHASH,
                revoker,
                schemaId,
                uid,
                0, // value
                nonce, // nonce
                deadline
            )
        );

        bytes32 digest = keccak256(abi.encodePacked("\x19\x01", _createDomainSeparator(), structHash));

        return digest;
    }

    /// @dev Creates the domain separator for EIP712 signatures
    function _createDomainSeparator() internal view returns (bytes32) {
        bytes32 TYPE_HASH =
            keccak256("EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)");
        bytes32 nameHash = keccak256(bytes("EAS"));
        bytes32 versionHash = keccak256(bytes("1.3.0"));

        return keccak256(abi.encode(TYPE_HASH, nameHash, versionHash, block.chainid, address(eas)));
    }

    /// @dev Creates and verifies a direct signature attestation
    function _testDirectSignature(bytes32 schemaId, uint256 signerKey) internal {
        AttestationRequestData memory requestData = _createAttestationRequestData();
        address signer = vm.addr(signerKey);
        // Test direct attestation
        vm.prank(signer);
        bytes32 uid = eas.attest(AttestationRequest({ schema: schemaId, data: requestData }));
        assertTrue(uid != bytes32(0), "Direct attestation should succeed");

        // Verify the attestation
        Attestation memory attestation = eas.getAttestation(uid);
        assertEq(attestation.attester, signer);
        assertEq(attestation.recipient, requestData.recipient);
    }

    /// @dev Creates and verifies a proxy-based signature attestation
    function _testProxySignature(bytes32 schemaId) internal {
        AttestationRequestData memory requestData = _createAttestationRequestData();
        uint64 deadline = uint64(block.timestamp + 1 days);

        uint256 signerKey = 0x12345;
        address signer = vm.addr(signerKey);
        vm.deal(signer, 100 ether);

        // Create signature using proxy's domain separator
        bytes32 digest = keccak256(
            abi.encodePacked(
                "\x19\x01", _createDomainSeparator(), _getStructHash(schemaId, requestData, signer, deadline)
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
    function _testDelegatedSignature(bytes32 schemaId, uint256 signerKey) internal {
        AttestationRequestData memory requestData = _createAttestationRequestData();
        uint64 deadline = uint64(block.timestamp + 1 days);
        address signer = vm.addr(signerKey);
        vm.deal(signer, 100 ether);

        bytes32 digest = _createAttestationDigest(schemaId, requestData, signer, deadline, 0);
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
    )
        internal
        pure
        returns (bytes32)
    {
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

    function _getRevocationStructHash(
        bytes32 schemaId,
        bytes32 uid,
        address revoker,
        uint256 value,
        uint256 nonce,
        uint64 deadline
    )
        internal
        pure
        returns (bytes32)
    {
        return keccak256(
            abi.encode(
                keccak256(
                    "Revoke(address revoker,bytes32 schema,bytes32 uid,uint256 value,uint256 nonce,uint64 deadline)"
                ),
                revoker,
                schemaId,
                uid,
                value,
                nonce,
                deadline
            )
        );
    }

    function _hasSpace(string memory str) internal pure returns (bool) {
        bytes memory strBytes = bytes(str);
        for (uint256 i = 0; i < strBytes.length; i++) {
            if (strBytes[i] == 0x20) {
                // ASCII space
                return true;
            }
        }
        return false;
    }

    function _isAlphanumeric(string memory str) internal pure returns (bool) {
        bytes memory strBytes = bytes(str);
        for (uint256 i = 0; i < strBytes.length; i++) {
            bytes1 char = strBytes[i];
            if (
                // 0-9
                // A-Z
                !((char >= 0x30 && char <= 0x39) || (char >= 0x41 && char <= 0x5A) || (char >= 0x61 && char <= 0x7A)) // a-z
            ) {
                return false;
            }
        }
        return true;
    }

    // Setup
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

        vm.mockCall(payableResolver, abi.encodeWithSelector(ISchemaResolver.isPayable.selector), abi.encode(true));
        vm.mockCall(payableResolver, abi.encodeWithSelector(ISchemaResolver.attest.selector), abi.encode(true));
        vm.mockCall(payableResolver, abi.encodeWithSelector(ISchemaResolver.multiAttest.selector), abi.encode(true));
        vm.mockCall(payableResolver, abi.encodeWithSelector(ISchemaResolver.revoke.selector), abi.encode(true));
        vm.mockCall(payableResolver, abi.encodeWithSelector(ISchemaResolver.multiRevoke.selector), abi.encode(true));

        // Fund accounts
        vm.deal(sender, 100 ether);
        vm.deal(sender2, 100 ether);

        // Initialize helpers
        eip712Helper = new TestEIP712Helper();
        proxy = new TestEIP712Proxy(address(eas), "EAS-Proxy");
    }

    // Construction Tests
    /// @dev Tests the initial construction state of the EAS contract
    function test_constructionScenarios_succeeds() public view {
        assertEq(ISemver(address(eas)).version(), "1.4.1-beta.2");
    }

    /// @dev Tests behavior when using an invalid schema registry
    function test_schemaRegistry_invalid_reverts() public {
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

    // Signature Verification Tests
    /// @dev Tests the flow of a valid EIP-712 signed attestation.
    ///      Demonstrates complete flow of EIP-712 signed attestation
    function test_validAttestationSignature_succeeds(uint256 _signerKey) public {
        uint256 CURVE_ORDER = 115792089237316195423570985008687907852837564279074904382605163141518161494337;
        vm.assume(_signerKey > 0 && _signerKey < CURVE_ORDER);
        bytes32 schemaId = _registerSchema("bool like", true);
        uint64 deadline = uint64(block.timestamp + 1 days);

        uint256 signerKey = _signerKey;
        address signer = vm.addr(signerKey);
        vm.deal(signer, 100 ether);

        AttestationRequestData memory requestData = _createAttestationRequestData();
        bytes32 DOMAIN_SEPARATOR = _createDomainSeparator();

        bytes32 structHash = keccak256(
            abi.encode(
                _getDelegatedAttestationTypeHash(),
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

        bytes32 digest = keccak256(abi.encodePacked("\x19\x01", DOMAIN_SEPARATOR, structHash));

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
        assertTrue(uid != bytes32(0), "Attestation should succeed");
    }

    /// @dev Tests rejection of attestations with expired signature deadlines.
    ///      Ensures proper enforcement of signature deadlines in delegated attestations.
    function test_attestationSignature_expiredDeadline_reverts(uint256 _signerKey) public {
        uint256 CURVE_ORDER = 115792089237316195423570985008687907852837564279074904382605163141518161494337;
        vm.assume(_signerKey > 0 && _signerKey < CURVE_ORDER);
        bytes32 schemaId = _registerSchema("bool like", true);
        AttestationRequestData memory requestData = _createAttestationRequestData();

        // Set a specific timestamp
        vm.warp(1000);
        uint64 expiredDeadline = uint64(block.timestamp - 100); // 900
        uint256 signerKey = _signerKey;
        address signer = vm.addr(signerKey);

        bytes32 digest = _createAttestationDigest(schemaId, requestData, signer, expiredDeadline, 0);
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(signerKey, digest);

        DelegatedAttestationRequest memory request = DelegatedAttestationRequest({
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
    ///      Ensures attestations cannot be submitted with signatures from addresses
    ///      different than the specified attester.
    function test_attestationSignature_wrongSigner_reverts(uint256 _wrongSignerKey) public {
        uint256 CURVE_ORDER = 115792089237316195423570985008687907852837564279074904382605163141518161494337;
        vm.assume(_wrongSignerKey > 0 && _wrongSignerKey < CURVE_ORDER);

        bytes32 schemaId = _registerSchema("bool like", true);
        uint64 deadline = uint64(block.timestamp + 1 days);

        address wrongSigner = vm.addr(_wrongSignerKey);
        vm.assume(wrongSigner != sender); // Make sure wrong signer isn't the actual sender

        AttestationRequestData memory requestData = _createAttestationRequestData();
        bytes32 digest = _createAttestationDigest(schemaId, requestData, sender, deadline, 0); // Use sender here, not
            // wrongSigner
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(_wrongSignerKey, digest);

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
    ///      Ensures EIP-712 signature verification properly detects
    ///      post-signing data modifications
    function test_attestationSignature_tamperedData_reverts(uint256 _signerKey, uint256 _modifiedData) public {
        uint256 CURVE_ORDER = 115792089237316195423570985008687907852837564279074904382605163141518161494337;
        vm.assume(_signerKey > 0 && _signerKey < CURVE_ORDER);
        bytes32 schemaId = _registerSchema("bool like", true);
        uint64 deadline = uint64(block.timestamp + 1 days);

        uint256 signerKey = 0x12345;
        address signer = vm.addr(signerKey);

        AttestationRequestData memory requestData = _createAttestationRequestData();
        bytes32 digest = _createAttestationDigest(schemaId, requestData, signer, deadline, 0);
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(signerKey, digest);

        // Modify the data after signing
        requestData.data = abi.encode(_modifiedData);

        DelegatedAttestationRequest memory request = DelegatedAttestationRequest({
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
    function test_signatures_directDelegated_succeeds(uint256 _signerKey) public {
        uint256 CURVE_ORDER = 115792089237316195423570985008687907852837564279074904382605163141518161494337;
        vm.assume(_signerKey > 0 && _signerKey < CURVE_ORDER);
        // Register schema once at the start
        bytes32 schemaId = _registerSchema("bool like", true);

        SignatureType[2] memory sigTypes = [SignatureType.Direct, SignatureType.Delegated];

        for (uint256 i = 0; i < sigTypes.length; i++) {
            SignatureType sigType = sigTypes[i];

            if (sigType == SignatureType.Direct) {
                _testDirectSignature(schemaId, _signerKey);
            } else if (sigType == SignatureType.Delegated) {
                _testDelegatedSignature(schemaId, _signerKey);
            }
        }
    }

    /// @dev Tests attestation through proxy contract with signature verification.
    ///      Demonstrates complete proxy attestation flow with
    ///      proper signature verification and delegation.
    function test_signatures_proxyAttestation_succeeds(uint256 _signerKey, string memory _name) public {
        uint256 CURVE_ORDER = 115792089237316195423570985008687907852837564279074904382605163141518161494337;
        vm.assume(_signerKey > 0 && _signerKey < CURVE_ORDER);
        vm.warp(1000);

        address signer = vm.addr(_signerKey);
        bytes32 proxySchemaId = _registerSchema("string name", true);

        AttestationRequestData memory data = AttestationRequestData({
            recipient: recipient,
            expirationTime: NO_EXPIRATION,
            revocable: true,
            refUID: bytes32(0),
            data: abi.encode(_name),
            value: 0
        });

        uint64 deadline = uint64(block.timestamp + 1 days);

        DelegatedAttestationRequest memory request = DelegatedAttestationRequest({
            schema: proxySchemaId,
            data: data,
            signature: Signature(0, bytes32(0), bytes32(0)),
            attester: signer,
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

        bytes32 digest = keccak256(abi.encodePacked("\x19\x01", _createDomainSeparator(), structHash));

        (uint8 v, bytes32 r, bytes32 s) = vm.sign(_signerKey, digest);
        request.signature = Signature(v, r, s);

        vm.prank(sender);
        bytes32 uid = proxy.attestByDelegation(request);
        assertTrue(uid != bytes32(0));

        Attestation memory attestation = eas.getAttestation(uid);
        assertEq(attestation.attester, signer);
        assertEq(attestation.recipient, recipient);
    }

    /// @dev Tests signature verification against data tampering.
    ///      Demonstrates EIP-712 signature verification prevents
    ///      data tampering after signing
    function test_signature_tampering_reverts(uint256 _signerKey) public {
        uint256 CURVE_ORDER = 115792089237316195423570985008687907852837564279074904382605163141518161494337;
        vm.assume(_signerKey > 0 && _signerKey < CURVE_ORDER);
        bytes32 schemaId = _registerSchema("bool like", true);
        uint64 deadline = uint64(block.timestamp + 1 days);

        address signer = vm.addr(_signerKey);
        vm.deal(signer, 100 ether);

        AttestationRequestData memory requestData = _createAttestationRequestData();
        bytes32 digest = _createAttestationDigest(schemaId, requestData, signer, deadline, 0);
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(_signerKey, digest);

        DelegatedAttestationRequest memory request = DelegatedAttestationRequest({
            schema: schemaId,
            data: requestData,
            signature: Signature({ v: v, r: r, s: s }),
            attester: signer,
            deadline: deadline
        });

        vm.prank(signer);
        eas.attestByDelegation(request); // First attempt should succeed

        // Modify the requestData to simulate tampering
        requestData.data = abi.encode("tampered data"); // Change the data to something else

        vm.expectRevert(InvalidSignature.selector);
        eas.attestByDelegation(request); // Second attempt should fail
    }

    // Timestamping Tests
    /// @dev Tests basic timestamping functionality.
    ///      Records current block timestamp, timestamps
    ///      a data hash, and confirms stored timestamps match.
    function test_basic_timestamp_succeeds(bytes memory _randomData) public {
        bytes32 data = keccak256(_randomData);

        uint256 timestamp = block.timestamp;
        eas.timestamp(data);

        assertEq(eas.getTimestamp(data), timestamp);
    }

    /// @dev Tests batch timestamping functionality.
    // Records and verifies timestamp of 3 different data items.
    function test_timestamp_multiple_succeeds(bytes memory _randomData, bytes memory _randomData2) public {
        vm.assume(bytes(_randomData).length > 0 && bytes(_randomData2).length > 0);
        vm.assume(keccak256(_randomData) != keccak256(_randomData2));
        bytes32[] memory data = new bytes32[](3);
        data[0] = keccak256(_randomData);
        data[1] = keccak256(_randomData2);
        data[2] = bytes32(0);

        uint256 timestamp = block.timestamp;
        eas.multiTimestamp(data);

        for (uint256 i = 0; i < data.length; i++) {
            assertEq(eas.getTimestamp(data[i]), timestamp);
        }
    }

    /// @dev Tests timestamp duplication prevention.
    ///      Ensures data can only be timestamped once.
    function test_timestamp_alreadyTimestamped_reverts(bytes memory _randomData) public {
        bytes32 data = keccak256(_randomData);

        // First timestamp should succeed
        eas.timestamp(data);

        // Second timestamp should fail
        vm.expectRevert(AlreadyTimestamped.selector);
        eas.timestamp(data);
    }

    /// @dev Tests comprehensive batch timestamping scenarios.
    /// Demonstrates efficient batch processing of multiple stimestamp recrods.
    function test_attestationMultiTimestamp_succeeds(
        uint256 _randomData1,
        uint256 _randomData2,
        uint256 _randomData3,
        uint256 _randomData4,
        uint256 _randomData5
    )
        public
    {
        // Ensure that all random data inputs are unique
        vm.assume(
            _randomData1 != _randomData2 && _randomData1 != _randomData3 && _randomData1 != _randomData4
                && _randomData1 != _randomData5 && _randomData2 != _randomData3 && _randomData2 != _randomData4
                && _randomData2 != _randomData5 && _randomData3 != _randomData4 && _randomData3 != _randomData5
                && _randomData4 != _randomData5
        );
        bytes32[] memory data = new bytes32[](3);
        data[0] = keccak256(abi.encodePacked(_randomData1));
        data[1] = keccak256(abi.encodePacked(_randomData2));
        data[2] = keccak256(abi.encodePacked(_randomData3));

        // Test multiple timestamps in one transaction
        uint256 timestamp = block.timestamp;
        eas.multiTimestamp(data);

        // Verify all timestamps
        for (uint256 i = 0; i < data.length; i++) {
            assertEq(eas.getTimestamp(data[i]), timestamp);
        }

        // Test second batch
        bytes32[] memory data2 = new bytes32[](2);
        data2[0] = keccak256(abi.encodePacked(_randomData4));
        data2[1] = keccak256(abi.encodePacked(_randomData5));

        eas.multiTimestamp(data2);

        // Verify second batch
        for (uint256 i = 0; i < data2.length; i++) {
            assertEq(eas.getTimestamp(data2[i]), timestamp);
        }
    }

    /// @dev Tests revert conditions for batch timestamping.
    /// Ensures proper duplicate detection in all batch scenarios.
    function test_multiOffchainRevoke_alreadyTimestamped_reverts(
        uint256 _randomData1,
        uint256 _randomData2,
        uint256 _randomData3
    )
        public
    {
        vm.assume(_randomData1 < type(uint128).max);
        vm.assume(_randomData2 < type(uint128).max);
        vm.assume(_randomData3 < type(uint128).max);
        vm.assume(_randomData1 != _randomData2);
        vm.assume(_randomData1 != _randomData3);
        vm.assume(_randomData2 != _randomData3);

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
    /// Demonstrates timestamp immutability and independence

    function test_timestamp_verification_works(
        uint256 _randomData1,
        uint256 _randomData2,
        uint256 _randomData3,
        uint256 _randomData4
    )
        public
    {
        vm.assume(
            _randomData1 != _randomData2 && _randomData1 != _randomData3 && _randomData1 != _randomData4
                && _randomData2 != _randomData3 && _randomData2 != _randomData4 && _randomData3 != _randomData4
        );
        bytes32[] memory data = new bytes32[](3);
        data[0] = keccak256(abi.encodePacked(_randomData1));
        data[1] = keccak256(abi.encodePacked(_randomData2));
        data[2] = keccak256(abi.encodePacked(_randomData3));

        // Test initial timestamps
        uint256 timestamp = block.timestamp;
        eas.multiTimestamp(data);

        // Verify all timestamps match block timestamp
        for (uint256 i = 0; i < data.length; i++) {
            assertEq(eas.getTimestamp(data[i]), timestamp);
        }

        // Advance time and verify timestamps don't change
        vm.warp(block.timestamp + 1 days);
        for (uint256 i = 0; i < data.length; i++) {
            assertEq(eas.getTimestamp(data[i]), timestamp);
        }

        // Test timestamp immutability
        bytes32 newData = keccak256(abi.encodePacked(_randomData4));
        eas.timestamp(newData);
        assertEq(eas.getTimestamp(newData), block.timestamp);

        // Verify original timestamps remain unchanged
        for (uint256 i = 0; i < data.length; i++) {
            assertEq(eas.getTimestamp(data[i]), timestamp);
        }
    }

    /// @dev Tests retrieval of unregistered timestamps.
    ///      Ensures system properly handles queries for
    ///      non-existent timestamp records.
    function test_unregistered_timestamp_succeeds(uint256 _randomData) public view {
        bytes32 data = keccak256(abi.encodePacked(_randomData));
        assertEq(eas.getTimestamp(data), 0);
    }

    // Basic Attestation Tests
    /// @dev Tests basic attestation functionality.
    ///      Demonstrates core attestation flow with
    ///      standard schema and parameters.
    function test_basic_attestation_succeeds(
        string memory _property1,
        uint256 _property2,
        bool _property3,
        address _resolver,
        bool _revocable
    )
        public
    {
        vm.assume(_resolver != address(0));
        vm.assume(_resolver.code.length == 0);
        vm.assume(_resolver != 0x000000000000000000636F6e736F6c652e6c6f67);
        vm.assume(uint160(_resolver) > 0xFF);
        // Create schema string using valid types from docs
        string memory schema = "string name,uint256 age,bool isStudent";

        bytes32 schemaId = _getSchemaUID(schema, _resolver, _revocable);

        vm.mockCall(_resolver, abi.encodeWithSelector(ISchemaResolver.attest.selector), abi.encode(true));

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(_resolver), _revocable);

        // Encode data according to schema
        bytes memory data = abi.encode(
            _property1, // string
            _property2, // uint
            _property3 // bool
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
    ///      Demonstrates system handles empty schemas correctly,
    ///      allowing attestations without schema definitions
    function test_attestation_noSchema_succeeds(address _resolver, bool _revocable) public {
        vm.assume(_resolver != address(0));
        vm.assume(_resolver.code.length == 0);
        vm.assume(_resolver != 0x000000000000000000636F6e736F6c652e6c6f67);
        vm.assume(uint160(_resolver) > 0xFF);
        bytes32 schemaId = _getSchemaUID("", _resolver, _revocable);

        vm.startPrank(sender);
        schemaRegistry.register("", ISchemaResolver(_resolver), _revocable);

        uint64 expirationTime = uint64(block.timestamp + 30 days);
        bytes memory data = hex"1234";

        vm.assume(_resolver.code.length == 0); // Ensure it's not a contract already
        vm.mockCall(_resolver, abi.encodeWithSelector(ISchemaResolver.attest.selector), abi.encode(true));

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
    ///      Demonstrates attestation flow with resolver integration,
    ///      ensuring resolver-enabled schemas work correctly.
    function test_attestation_resolver_succeeds(
        string memory _name,
        uint256 _age,
        bool _isStudent,
        bool _revocable
    )
        public
    {
        string memory schema = "string name,uint256 age,bool isStudent";
        bytes32 schemaId = _getSchemaUID(schema, address(payableResolver), _revocable);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(payableResolver), _revocable);
        // Encode data according to schema
        bytes memory data = abi.encode(_name, _age, _isStudent);
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
    ///      Demonstrates attestation flow for schemas without
    ///      resolver integration, ensuring basic schema
    ///      functionality works independently.
    function test_attestation_noResolver_succeeds(
        string memory _name,
        uint256 _age,
        bool _isStudent,
        bool _revocable
    )
        public
    {
        string memory schema = "string name,uint256 age,bool isStudent";

        bytes32 schemaId = _getSchemaUID(schema, address(0), _revocable);
        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), _revocable);

        uint64 expirationTime = uint64(block.timestamp + 30 days);
        bytes memory data = abi.encode(_name, _age, _isStudent);

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

    /// @dev Tests the behavior of the attestation system with respect to schema registration and validation.
    ///      This function serves to ensure that the system correctly handles
    ///      both valid and invalid schema scenarios.
    function test_schema_unregistered_reverts() public {
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
    ///      Verifies that the system correctly handles and stores data of different sizes
    ///      by checking that stored attestation data matches the input data for each case.
    function test_attestation_dataScenarios_succeeds(
        string memory _smallTestData,
        string memory _mediumTestData
    )
        public
    {
        // Relax the constraints
        vm.assume(bytes(_smallTestData).length > 0);
        vm.assume(bytes(_smallTestData).length < 100);
        vm.assume(bytes(_mediumTestData).length > 0);
        vm.assume(bytes(_mediumTestData).length < 1000);

        string memory schema = "bool like";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);

        // Test with different data sizes
        bytes[] memory testData = new bytes[](3);
        testData[0] = "";
        testData[1] = bytes(_smallTestData);
        testData[2] = bytes(_mediumTestData);

        for (uint256 i = 0; i < testData.length; i++) {
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
    ///      Also tests value transfer functionality with the resolver contract
    function test_attestation_comprehensive_succeeds(
        address _recipient,
        uint256 _expirationOffset,
        bytes memory _data1,
        bytes memory _data2
    )
        public
    {
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
                    expirationTime: 0,
                    revocable: true,
                    refUID: bytes32(0),
                    data: _data1,
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
                    expirationTime: uint64(block.timestamp + _expirationOffset),
                    revocable: true,
                    refUID: uid1,
                    data: _data2,
                    value: value
                })
            })
        );

        // Verify first attestation
        Attestation memory attestation1 = eas.getAttestation(uid1);
        assertEq(attestation1.recipient, address(0));
        assertEq(attestation1.expirationTime, 0);
        assertTrue(attestation1.revocable);
        assertEq(attestation1.refUID, bytes32(0));
        assertEq(attestation1.data, _data1);

        // Verify second attestation
        Attestation memory attestation2 = eas.getAttestation(uid2);
        assertEq(attestation2.recipient, _recipient);
        assertEq(attestation2.expirationTime, uint64(block.timestamp + _expirationOffset));
        assertTrue(attestation2.revocable);
        assertEq(attestation2.refUID, uid1);
        assertEq(attestation2.data, _data2);

        vm.stopPrank();
    }

    /// @dev Tests a valid attestation with a valid expiration
    function test_attestation_validExpiration_succeeds(address _recipient, uint256 _validExpirationOffset) public {
        string memory schema = "bool like";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);

        vm.assume(_recipient != address(0));
        vm.assume(_validExpirationOffset > 0 && _validExpirationOffset <= 365 days);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);

        vm.warp(1000000);

        // Test with valid expiration times
        uint64[] memory expirationTimes = new uint64[](3);
        expirationTimes[0] = 0;
        expirationTimes[1] = uint64(block.timestamp + _validExpirationOffset);
        expirationTimes[2] = uint64(block.timestamp + 365 days);

        for (uint256 i = 0; i < expirationTimes.length; i++) {
            bytes32 uid = eas.attest(
                AttestationRequest({
                    schema: schemaId,
                    data: AttestationRequestData({
                        recipient: _recipient,
                        expirationTime: expirationTimes[i],
                        revocable: true,
                        refUID: bytes32(0),
                        data: hex"1234",
                        value: 0
                    })
                })
            );

            Attestation memory attestation = eas.getAttestation(uid);
            assertEq(attestation.expirationTime, expirationTimes[i]);
        }
        vm.stopPrank();
    }

    /// @dev Tests rejection of attestations with expired timestamps.
    ///      Ensures system properly validates attestation
    ///      expiration times, preventing backdated attestations.
    function test_attestation_invalidExpiration_reverts(
        string memory _name,
        uint256 _age,
        bool _isStudent,
        address _resolver,
        bool _revocable,
        address _recipient
    )
        public
    {
        vm.assume(_recipient != address(0));

        string memory schema = "string name,uint256 age,bool isStudent";

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(_resolver), _revocable);
        bytes32 schemaId = _getSchemaUID(schema, _resolver, _revocable);

        // Set a specific block timestamp
        vm.warp(1000000);
        uint64 expiredTime = uint64(block.timestamp - 1000);
        bytes memory data = abi.encode(_name, _age, _isStudent);

        AttestationRequest memory request = AttestationRequest({
            schema: schemaId,
            data: AttestationRequestData({
                recipient: _recipient,
                expirationTime: expiredTime,
                revocable: _revocable,
                refUID: ZERO_BYTES32,
                data: data,
                value: 0
            })
        });

        vm.expectRevert(InvalidExpirationTime.selector);
        eas.attest(request);
        vm.stopPrank();
    }

    /// @dev Tests behavior when querying non-existent (unregistered) attestations and revocations.
    ///      Ensures system handles queries for non-existent data gracefully.
    function test_unregistered_data_succeeds(string memory _unregisteredData) public view {
        bytes32 unregisteredData = keccak256(abi.encodePacked(_unregisteredData));

        // Should return 0 for unregistered timestamp
        assertEq(eas.getTimestamp(unregisteredData), 0);

        // Should return 0 for unregistered revocation
        assertEq(eas.getRevokeOffchain(sender, unregisteredData), 0);
    }

    /// @dev Tests rejection of attestations with invalid reference UIDs.
    ///      Ensures system properly validates referenced attestations,
    ///      preventing attestations that reference non-existent UIDs.
    function test_attestation_invalidData_reverts(uint256 _nonExistentUID) public {
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

    /// @dev Tests attestation referencing functionality.
    ///      Demonstrates the ability to create linked attestations
    ///      through reference UIDs
    function test_attestation_reference_succeeds(
        string memory _name,
        string memory _name2,
        bytes32 _id,
        bytes32 _id2
    )
        public
    {
        string memory schema = "string name,bytes32 id";
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
                    data: abi.encode(_name, _id),
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
                    data: abi.encode(_name2, _id2),
                    value: 0
                })
            })
        );

        Attestation memory attestation = eas.getAttestation(uid);
        assertEq(attestation.refUID, refUid);
        vm.stopPrank();
    }

    // Multi-attestation Tests
    /// @dev This function tests the multi-attestation functionality of the smart contract.
    /// It validates that multiple attestations can be created in a single transaction,
    /// ensuring that each attestation corresponds to the correct schema and that the
    /// data matches the expected values for both "like" and "score".
    function test_attestation_multiDelegated_succeeds(
        address _recipient,
        address _recipient2,
        bool _like,
        uint256 _score
    )
        public
    {
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
        MultiAttestationRequest[] memory requests = new MultiAttestationRequest[](recipients.length * 2); // 2 requests
            // per recipient

        for (uint256 i = 0; i < recipients.length; i++) {
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
        uint256 totalValue = 0.2 ether * recipients.length; // 0.2 ether for each recipient (0.1 ether for like + 0.1
            // ether for score)
        vm.deal(sender, totalValue); // Ensure sender has enough Ether
        bytes32[] memory uids = eas.multiAttest{ value: totalValue }(requests);
        assertEq(uids.length, 6); // Expecting 2 attestations per recipient

        // Verify all attestations
        for (uint256 i = 0; i < uids.length; i++) {
            Attestation memory attestation = eas.getAttestation(uids[i]);

            assertEq(attestation.attester, sender);
            if (i % 2 == 0) {
                assertEq(attestation.schema, schemaId);
                bool like = abi.decode(attestation.data, (bool));
                assertEq(like, _like);
            } else {
                assertEq(attestation.schema, schemaId2);
                uint256 score = abi.decode(attestation.data, (uint256));
                assertEq(score, _score);
            }
        }

        vm.stopPrank();
    }

    /// @dev Tests complex multi-attestation expiration scenarios across different schemas.
    ///      Verifies correct schema assignment and attester for all attestations
    function test_attestation_complexMulti_succeeds() public {
        string memory schema = "bool like";
        string memory schema2 = "uint256 score";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);
        schemaRegistry.register(schema2, ISchemaResolver(address(0)), true);

        // Test with multiple schemas in single transaction
        MultiAttestationRequest[] memory requests = new MultiAttestationRequest[](2);

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
        for (uint256 i = 0; i < uids.length; i++) {
            Attestation memory attestation = eas.getAttestation(uids[i]);
            assertEq(attestation.attester, sender);
            if (i < 2) {
                assertEq(attestation.schema, schemaId);
            } else {
                assertEq(attestation.schema, _getSchemaUID(schema2, address(0), true));
            }
        }

        vm.stopPrank();
    }

    /// @dev Tests multiple error conditions for multi-attestation requests,
    ///      including InvalidLength, InvalidSchema, InvalidExpirationTime,
    ///      and insufficient ETH value (default error).
    function test_attestation_multiDelegated_reverts() public {
        string memory schema = "bool like";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);
        bytes32 schemaWithResolverId = _getSchemaUID(schema, address(payableResolver), true);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);
        schemaRegistry.register(schema, ISchemaResolver(address(payableResolver)), true);

        // Test 1: Empty requests array should return empty array
        MultiAttestationRequest[] memory requests = new MultiAttestationRequest[](0);

        bytes32[] memory uids = eas.multiAttest(requests);
        assertEq(uids.length, 0);

        // Test 2: Empty data array in request
        requests = new MultiAttestationRequest[](1);
        requests[0] = MultiAttestationRequest({ schema: schemaId, data: new AttestationRequestData[](0) });

        vm.expectRevert(abi.encodeWithSelector(InvalidLength.selector));
        eas.multiAttest(requests);

        // Test 3: Invalid schema
        requests[0] = MultiAttestationRequest({ schema: bytes32(0), data: new AttestationRequestData[](1) });
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
    function test_attestation_multiValue_succeeds(
        uint256 _value,
        address _userAddress,
        string memory _userName,
        bool _isActive,
        address _userAddress2,
        string memory _userName2,
        bool _isActive2
    )
        public
    {
        string memory schema =
            "address userAddress,string userName,bool isActive,address userAddress2,string userName2,bool isActive2";
        _value = bound(_value, 1, 10);
        bytes32 schemaId = _getSchemaUID(schema, address(payableResolver), true);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(payableResolver)), true);

        uint256 value = _value * 1 ether;
        vm.deal(sender, value * 2);
        bytes memory data = abi.encode(
            _userAddress, // address
            _userName, // string
            _isActive // bool
        );
        bytes memory data2 = abi.encode(
            _userAddress2, // address
            _userName2, // string
            _isActive2 // bool
        );

        MultiAttestationRequest[] memory requests = new MultiAttestationRequest[](2);
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
        for (uint256 i = 0; i < uids.length; i++) {
            Attestation memory attestation = eas.getAttestation(uids[i]);
            assertEq(attestation.attester, sender);
        }

        vm.stopPrank();
    }

    /// Delegation Tests
    /// @dev Tests basic delegated attestation functionality.
    ///      Demonstrates standard delegated attestation flow.
    function test_attestation_delegated_succeeds(
        string memory _name,
        uint256 _age,
        bool _isStudent,
        uint256 _expirationTimeOffset,
        uint256 _deadlineOffset
    )
        public
    {
        _expirationTimeOffset = bound(_expirationTimeOffset, 1, 364);
        _deadlineOffset = bound(_deadlineOffset, 1, 364);
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
        Signature memory signature = Signature({ v: v, r: r, s: s });

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

    /// @dev Tests delegated attestation with various deadline scenarios.
    ///      Tests three different deadline configurations and verifies
    ///      deadline passing in all scenarios.
    function test_attestation_delegatedTime_succeeds(
        string memory _name,
        uint256 _age,
        bool _isStudent,
        uint256 _expirationTimeOffset,
        uint256 _deadlineOffset
    )
        public
    {
        _expirationTimeOffset = bound(_expirationTimeOffset, 1, 364);
        _deadlineOffset = bound(_deadlineOffset, 1, 364);
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

        for (uint256 i = 0; i < deadlines.length; i++) {
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

            DelegatedAttestationRequest memory request = DelegatedAttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: uint64(block.timestamp + 30 days),
                    revocable: true,
                    refUID: ZERO_BYTES32,
                    data: data,
                    value: 0
                }),
                signature: Signature({ v: v, r: r, s: s }),
                attester: attester,
                deadline: deadlines[i]
            });

            bytes32 uid = eas.attestByDelegation(request);

            Attestation memory attestation = eas.getAttestation(uid);
            assertEq(attestation.attester, attester);
            assertEq(attestation.recipient, recipient);
        }

        vm.stopPrank();
    }

    /// @dev Tests deadline validation in delegated attestations.
    ///      Ensures proper enforcement of deadline constraints
    ///      in delegated attestations.
    function test_attestationDelegated_expiredDeadline_reverts(string memory _name) public {
        string memory schema = "string name";
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
                data: abi.encode(_name),
                value: 0
            }),
            signature: Signature({ v: 28, r: bytes32(uint256(1)), s: bytes32(uint256(2)) }),
            attester: sender,
            deadline: uint64(block.timestamp - 1)
        });

        vm.expectRevert(DeadlineExpired.selector);
        eas.attestByDelegation(request);
        vm.stopPrank();
    }

    /// Multi-delegation Tests
    /// @dev Tests batch attestation delegation functionality.
    ///      Demonstrates efficient batch processing of delegated attestations
    function test_attestation_multiDelegatedBatch_succeeds(
        address _recipient,
        address _recipient2,
        string[] memory _names
    )
        public
    {
        vm.assume(_names.length >= 2);
        string memory schema = "string name";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);

        bytes[] memory dataArray = new bytes[](2);
        dataArray[0] = abi.encode(_names[0]);
        dataArray[1] = abi.encode(_names[1]);

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
        multiRequest[0].signatures[0] = Signature({ v: signatures[0].v, r: signatures[0].r, s: signatures[0].s });
        multiRequest[0].signatures[1] = Signature({ v: signatures[1].v, r: signatures[1].r, s: signatures[1].s });

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

    /// @dev Tests delegated attestations with signatures.
    ///      Ensures signature validation works for both single and batch cases.
    function test_attestationDelegated_invalidSignature_reverts(
        uint8 _v,
        bytes32 _r,
        bytes32 _s,
        string memory _name,
        uint256 _age,
        bool _isStudent
    )
        public
    {
        string memory schema = "string name,uint256 age,bool isStudent";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);

        bytes memory data = abi.encode(_name, _age, _isStudent);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);

        // Test single delegated attestation with signature
        DelegatedAttestationRequest memory request = DelegatedAttestationRequest({
            schema: schemaId,
            data: AttestationRequestData({
                recipient: recipient,
                expirationTime: uint64(block.timestamp + 30 days),
                revocable: true,
                refUID: ZERO_BYTES32,
                data: data,
                value: 0
            }),
            signature: Signature({ v: _v, r: _r, s: _s }),
            attester: sender,
            deadline: type(uint64).max
        });

        vm.expectRevert(abi.encodeWithSignature("InvalidSignature()"));
        eas.attestByDelegation(request);

        // Test multi-attestation with signatures
        MultiDelegatedAttestationRequest[] memory requests = new MultiDelegatedAttestationRequest[](2);
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
        requests[0].signatures[0] = Signature({ v: _v, r: _r, s: _s });
        requests[0].attester = sender;
        requests[0].deadline = type(uint64).max;

        vm.expectRevert(abi.encodeWithSignature("InvalidSignature()"));
        eas.multiAttestByDelegation(requests);

        vm.stopPrank();
    }

    /// @dev Creates and verifies multiple delegated attestations.
    ///      Demonstrates complete flow of multiple delegated
    ///      attestations with different signers

    function test_attestation_multiDelegatedUnique_succeeds(string[3] memory _names) public {
        bytes32 schemaId = schemaRegistry.register("string name", ISchemaResolver(address(0)), true);
        uint64 deadline = uint64(block.timestamp + 1 days);

        // Create multiple signers
        uint256[] memory signerKeys = new uint256[](3);
        address[] memory signers = new address[](3);
        for (uint256 i = 0; i < 3; i++) {
            signerKeys[i] = 0x12345 + i;
            signers[i] = vm.addr(signerKeys[i]);
            vm.deal(signers[i], 100 ether);
        }

        MultiDelegatedAttestationRequest[] memory requests = new MultiDelegatedAttestationRequest[](3);
        for (uint256 i = 0; i < 3; i++) {
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
                data: abi.encodePacked(_names[i]),
                value: 0
            });

            bytes32 digest = _createAttestationDigest(schemaId, requests[i].data[0], signers[i], deadline, 0);

            (uint8 v, bytes32 r, bytes32 s) = vm.sign(signerKeys[i], digest);
            requests[i].signatures[0] = Signature({ v: v, r: r, s: s });
        }

        vm.prank(signers[0]);
        bytes32[] memory uids = eas.multiAttestByDelegation(requests);

        // Verify attestations
        for (uint256 i = 0; i < uids.length; i++) {
            Attestation memory attestation = eas.getAttestation(uids[i]);
            assertEq(attestation.attester, signers[i]);
            assertEq(attestation.recipient, recipient);
        }
    }

    /// @dev Tests error conditions for delegated attestations.
    ///      Ensures proper error handling for InvalidLength and DeadlineExpired
    function test_attestationMultiDelegated_invalidLengthAndDeadlineExpired_reverts(
        string memory _propertyName,
        string memory _value
    )
        public
    {
        string memory schema = string.concat("string ", _propertyName);
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);

        // Test 1: Empty data arrays

        MultiDelegatedAttestationRequest[] memory requests = new MultiDelegatedAttestationRequest[](1);

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
            data: abi.encode(_value),
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

    /// Revocation Tests
    /// @dev Tests the attestation revocation process.
    /// Creates a revocable attestation, revokes the attestation,
    /// and verified the revocation.
    function test_attestation_revoke_succeeds(
        address _userAddress,
        string memory _userName,
        bool _isActive,
        uint64 _expirationOffset
    )
        public
    {
        vm.assume(_expirationOffset > 0 && _expirationOffset < 366 days);
        string memory schema = "address userAddress,string userName,bool isActive";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);

        uint64 expirationTime = uint64(block.timestamp + _expirationOffset);
        bytes memory data = abi.encode(_userAddress, _userName, _isActive);

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

        eas.revoke(RevocationRequest({ schema: schemaId, data: RevocationRequestData({ uid: uid, value: 0 }) }));

        Attestation memory attestation = eas.getAttestation(uid);
        assertTrue(attestation.revocationTime > 0);
        vm.stopPrank();
    }

    /// @dev Tests that attestations can only be revoked by their original attester.
    /// Creates an attestation from one address and attempts to revoke it from another address.
    /// Expects an Access Denied error.
    function test_attestationRevoke_unauthorized_reverts(
        address _userAddress,
        string memory _userName,
        bool _isActive,
        uint64 _expirationOffset
    )
        public
    {
        vm.assume(_expirationOffset > 0 && _expirationOffset < 366 days);
        string memory schema = "address userAddress,string userName,bool isActive";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);

        vm.prank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);

        uint64 expirationTime = uint64(block.timestamp + _expirationOffset);
        bytes memory data = abi.encode(_userAddress, _userName, _isActive);

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
        eas.revoke(RevocationRequest({ schema: schemaId, data: RevocationRequestData({ uid: uid, value: 0 }) }));
    }

    /// @dev Tests revocation attempt of a non-existent attestation.
    /// Expects a NotFound error.
    function test_attestationRevoke_nonexistent_reverts(bytes32 _nonExistentUid) public {
        string memory schema = "bool like";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);

        vm.prank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);

        bytes32 nonExistentUid = _nonExistentUid;

        vm.prank(sender);
        vm.expectRevert(NotFound.selector);
        eas.revoke(
            RevocationRequest({ schema: schemaId, data: RevocationRequestData({ uid: nonExistentUid, value: 0 }) })
        );
    }

    /// @dev Tests revocation behavior with referenced attestations (parent-child relationship).
    ///      Ensures that attestation references don't create revocation dependencies
    function test_revocation_refUID_succeeds(
        address _userAddress,
        string memory _userName,
        bool _isActive,
        address _userAddress2,
        string memory _userName2,
        bool _isActive2,
        uint64 _expirationOffset
    )
        public
    {
        vm.assume(_expirationOffset > 0 && _expirationOffset < 366 days);
        string memory schema = "address userAddress,string userName,bool isActive";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);
        bytes memory data = abi.encode(_userAddress, _userName, _isActive);
        bytes memory data2 = abi.encode(_userAddress2, _userName2, _isActive2);

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
        eas.revoke(RevocationRequest({ schema: schemaId, data: RevocationRequestData({ uid: parentUID, value: 0 }) }));

        // Child should still be valid
        Attestation memory childAttestation = eas.getAttestation(childUID);
        assertEq(childAttestation.revocationTime, 0);

        vm.stopPrank();
    }

    /// @dev Tests batch revocation of multiple attestations.
    ///      Creates three attestations and revokes all in one
    ///      attestation. Then Vefifies all have a non-zero revocationTime.
    function test_revocation_multi_succeeds(uint256 _count, uint64 _expirationOffset) public {
        _count = bound(_count, 1, 10);
        _expirationOffset = uint64(bound(_expirationOffset, 1, 365 days));
        bytes32 schemaId = _registerSchema("bool like", true);

        vm.startPrank(sender);

        // Create multiple attestations with unique data
        bytes32[] memory uids = new bytes32[](_count);
        for (uint256 i = 0; i < _count; i++) {
            // Add timestamp to make each attestation unique
            uids[i] = eas.attest(
                AttestationRequest({
                    schema: schemaId,
                    data: AttestationRequestData({
                        recipient: recipient,
                        expirationTime: uint64(block.timestamp + _expirationOffset),
                        revocable: true,
                        refUID: ZERO_BYTES32,
                        data: abi.encodePacked(bytes1(uint8(i + 1)), block.timestamp), // Added timestamp
                        value: 0
                    })
                })
            );
        }

        // Create revocation requests
        MultiRevocationRequest[] memory requests = new MultiRevocationRequest[](1);
        requests[0].schema = schemaId;
        requests[0].data = new RevocationRequestData[](_count);
        for (uint256 i = 0; i < _count; i++) {
            requests[0].data[i] = RevocationRequestData({ uid: uids[i], value: 0 });
        }

        // Revoke all attestations
        eas.multiRevoke(requests);

        // Verify all attestations are revoked
        for (uint256 i = 0; i < uids.length; i++) {
            Attestation memory attestation = eas.getAttestation(uids[i]);
            assertTrue(attestation.revocationTime > 0, "Attestation should be revoked");
        }

        vm.stopPrank();
    }

    function test_revocation_delegated_succeeds(string memory _name) public {
        // Define schema and register it
        string memory schema = "string name";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);

        vm.startPrank(attester);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);
        vm.stopPrank();
        vm.warp(1000);

        // Create attestation request data
        bytes memory data = abi.encode(_name);
        AttestationRequestData memory requestData = AttestationRequestData({
            recipient: address(0),
            expirationTime: uint64(block.timestamp + 5 days),
            revocable: true,
            refUID: ZERO_BYTES32,
            data: data,
            value: 0
        });

        bytes32 requestHash =
            _createAttestationDigest(schemaId, requestData, attester, uint64(block.timestamp + 1 days), 0);

        vm.startPrank(attester);
        Signature memory attestSignature = _signMessage(requestHash, attesterKey);
        vm.stopPrank();

        // Create the delegated attestation request
        DelegatedAttestationRequest memory attestRequest = DelegatedAttestationRequest({
            schema: schemaId,
            data: requestData,
            signature: attestSignature,
            attester: attester,
            deadline: uint64(block.timestamp + 1 days)
        });

        // Test delegated attestation by the sender
        vm.startPrank(sender);
        bytes32 uid = eas.attestByDelegation(attestRequest);

        // Verify the attestation
        Attestation memory attestation = eas.getAttestation(uid);

        assertEq(attestation.attester, attester);
        assertEq(attestation.recipient, address(0));
        vm.stopPrank();
        vm.warp(1000);
        // Create revocation request data
        RevocationRequestData memory revokeData = RevocationRequestData({ uid: uid, value: 0 });

        uint256 currentNonce = IEAS_EIP1271(address(eas)).getNonce(attester);

        // Use current nonce when creating digest
        bytes32 revokeHash = _createRevocationDigest(schemaId, revokeData.uid, attester, type(uint64).max, currentNonce);

        Signature memory revokeSignature = _signMessage(revokeHash, attesterKey);

        DelegatedRevocationRequest memory revokeRequest = DelegatedRevocationRequest({
            schema: schemaId,
            data: revokeData,
            signature: revokeSignature,
            revoker: attester,
            deadline: type(uint64).max
        });

        // Test delegated revocation by the sender
        vm.startPrank(sender);

        eas.revokeByDelegation(revokeRequest);
        // Verify the revocation
        attestation = eas.getAttestation(uid);

        assertTrue(attestation.revocationTime > 0);
        vm.stopPrank();
    }

    /// @dev Tests multi-delegated revocation functionality
    /// Creates multiple attestations and revokes them through delegation
    function test_revocation_multiDelegated_succeeds(uint256 _uint256Value) public {
        vm.assume(_uint256Value < 1000000);
        string memory schema = "uint256 value";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);
        vm.stopPrank();

        vm.startPrank(attester);

        // Create attestations to revoke
        bytes32[] memory uids = new bytes32[](2);
        for (uint256 i = 0; i < 2; i++) {
            uids[i] = eas.attest(
                AttestationRequest({
                    schema: schemaId,
                    data: AttestationRequestData({
                        recipient: recipient,
                        expirationTime: uint64(block.timestamp + 30 days),
                        revocable: true,
                        refUID: ZERO_BYTES32,
                        data: abi.encode(_uint256Value + i),
                        value: 0
                    })
                })
            );
        }

        // Setup multi-delegated revocation request
        MultiDelegatedRevocationRequest[] memory requests = new MultiDelegatedRevocationRequest[](1);
        requests[0].schema = schemaId;
        requests[0].data = new RevocationRequestData[](2);
        requests[0].signatures = new Signature[](2);
        requests[0].revoker = attester;
        requests[0].deadline = type(uint64).max;

        // Create signatures for each revocation
        for (uint256 i = 0; i < 2; i++) {
            requests[0].data[i] = RevocationRequestData({ uid: uids[i], value: 0 });

            uint256 currentNonce = IEAS_EIP1271(address(eas)).getNonce(attester);
            bytes32 revokeHash =
                _createRevocationDigest(schemaId, uids[i], attester, type(uint64).max, currentNonce + i);

            requests[0].signatures[i] = _signMessage(revokeHash, attesterKey);
        }
        vm.stopPrank();

        vm.prank(sender);
        eas.multiRevokeByDelegation(requests);

        // Verify all attestations were revoked
        for (uint256 i = 0; i < uids.length; i++) {
            Attestation memory attestation = eas.getAttestation(uids[i]);
            assertTrue(attestation.revocationTime > 0);
        }
    }

    /// @dev Tests access control for delegated revocations.
    /// Verifies that only the designated revoker can revoke through delegation
    function test_revocationDelegated_invalidSignature_reverts(uint64 _expirationOffset) public {
        vm.assume(_expirationOffset > 0 && _expirationOffset <= 365 days);
        vm.assume(sender2 != address(0));
        vm.assume(sender2 != attester); // Ensure sender2 is different from attester

        string memory schema = "uint256 value";
        bytes32 schemaId = _getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);
        vm.stopPrank();

        // Create attestation as attester
        vm.startPrank(attester);
        bytes32 uid = eas.attest(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: uint64(block.timestamp + _expirationOffset),
                    revocable: true,
                    refUID: ZERO_BYTES32,
                    data: abi.encode(1234),
                    value: 0
                })
            })
        );

        // Create revocation request with attester's signature
        uint256 currentNonce = IEAS_EIP1271(address(eas)).getNonce(attester);
        bytes32 revokeHash = _createRevocationDigest(schemaId, uid, attester, type(uint64).max, currentNonce);

        Signature memory revokeSignature = _signMessage(revokeHash, senderKey);
        vm.stopPrank();

        DelegatedRevocationRequest memory revokeRequest = DelegatedRevocationRequest({
            schema: schemaId,
            data: RevocationRequestData({ uid: uid, value: 0 }),
            signature: revokeSignature,
            revoker: attester,
            deadline: type(uint64).max
        });

        // Try to revoke with wrong revoker
        vm.prank(sender2);
        vm.expectRevert(InvalidSignature.selector); // Should fail with invalid signature
        eas.revokeByDelegation(revokeRequest);

        // Verify attestation is still valid
        Attestation memory attestation = eas.getAttestation(uid);
        assertEq(attestation.revocationTime, 0);
    }

    /// @dev Tests comprehensive scenarios for irrevocable schemas.
    ///      Ensures that  irrevocable property is enforced in all scenarios.
    function test_irrevocableSchema_succeeds(
        address _userAddress,
        string memory _userName,
        bool _isActive,
        address _userAddress2,
        string memory _userName2,
        bool _isActive2,
        address _recipient,
        address _recipient2,
        uint64 _expirationOffset
    )
        public
    {
        // Ensure the fuzzed expiration offset is within a reasonable range
        vm.assume(_expirationOffset > 0 && _expirationOffset <= 365 days);
        vm.assume(_recipient != address(0)); // Ensure recipient is not zero address
        vm.assume(_recipient2 != address(0)); // Ensure second recipient is not zero address

        string memory schema = "address userAddress,string userName,bool isActive";
        bytes32 schemaId = _getSchemaUID(schema, address(0), false);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), false);
        bytes memory data = abi.encode(_userAddress, _userName, _isActive);
        bytes memory data2 = abi.encode(_userAddress2, _userName2, _isActive2);

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
        eas.revoke(RevocationRequest({ schema: schemaId, data: RevocationRequestData({ uid: uid, value: 0 }) }));

        // Test multi-attestation with irrevocable schema
        MultiAttestationRequest[] memory requests = new MultiAttestationRequest[](1);
        requests[0].schema = schemaId;
        requests[0].data = new AttestationRequestData[](2);
        requests[0].data[0] = AttestationRequestData({
            recipient: _recipient,
            expirationTime: uint64(block.timestamp + _expirationOffset),
            revocable: false,
            refUID: ZERO_BYTES32,
            data: data,
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
        for (uint256 i = 0; i < uids.length; i++) {
            attestation = eas.getAttestation(uids[i]);
            assertEq(attestation.attester, sender);
            assertFalse(attestation.revocable);
        }

        // Should revert when trying to revoke multiple attestations
        RevocationRequest[] memory revocationRequests = new RevocationRequest[](2);
        for (uint256 i = 0; i < 2; i++) {
            revocationRequests[i] =
                RevocationRequest({ schema: schemaId, data: RevocationRequestData({ uid: uids[i], value: 0 }) });
        }

        vm.expectRevert(Irrevocable.selector);
        eas.revoke(revocationRequests[0]);

        vm.stopPrank();
    }

    /// @dev Tests scenarios mixing revocable and irrevocable schemas.
    ///      Verifies that revocable attestations can be revoked
    ///      and that irrevocable attestations cannot be revoked.
    function test_mixed_revocability_succeeds(
        string memory _revocableSchemaProperty,
        string memory _irrevocableSchemaProperty,
        string memory _revocableSchemaValue,
        string memory _irrevocableSchemaValue,
        address _recipient,
        uint64 _expirationOffset
    )
        public
    {
        // Ensure the fuzzed expiration offset is within a reasonable range
        vm.assume(_expirationOffset > 0 && _expirationOffset <= 365 days);
        vm.assume(_recipient != address(0));

        string memory revocableSchema = string.concat("string ", _revocableSchemaProperty);
        string memory irrevocableSchema = string.concat("string ", _irrevocableSchemaProperty);

        bytes32 revocableSchemaId = _getSchemaUID(revocableSchema, address(0), true);
        bytes32 irrevocableSchemaId = _getSchemaUID(irrevocableSchema, address(0), false);

        bytes memory data = abi.encode(_revocableSchemaValue);
        bytes memory data2 = abi.encode(_irrevocableSchemaValue);

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
            RevocationRequest({ schema: revocableSchemaId, data: RevocationRequestData({ uid: revocableUid, value: 0 }) })
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
    ///      Verifies the revocations fail with InvalidSchema error.
    ///      Ensures revocations are properly validated against registered schemas.
    function test_revocation_invalidSchema_reverts(string memory _stringName, string memory _invalidSchema) public {
        vm.assume(bytes(_stringName).length > 0 && bytes(_invalidSchema).length < 32);
        string memory schema = string.concat("string ", _stringName);

        vm.startPrank(sender);
        schemaRegistry.register(schema, ISchemaResolver(address(0)), true);

        // Try to revoke with wrong schema
        bytes32 wrongSchemaId = _getSchemaUID(_invalidSchema, address(0), true);

        vm.expectRevert(InvalidSchema.selector);
        eas.revoke(
            RevocationRequest({ schema: wrongSchemaId, data: RevocationRequestData({ uid: bytes32(0), value: 0 }) })
        );
        vm.stopPrank();
    }

    /// @dev Tests revocation behavior in referenced attestations.
    ///      Demonstrates that revocation does not cascade through
    ///      referenced attestations.
    function test_cascadingRevocation_succeeds(string[2] memory _names) public {
        string memory schema = "string name";
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
                    data: abi.encode(_names[0]),
                    value: 0
                })
            })
        );

        // Create chain of references
        for (uint256 i = 1; i < 3; i++) {
            uids[i] = eas.attest(
                AttestationRequest({
                    schema: schemaId,
                    data: AttestationRequestData({
                        recipient: recipient,
                        expirationTime: uint64(block.timestamp + 30 days),
                        revocable: true,
                        refUID: uids[i - 1],
                        data: abi.encode(_names[1], i),
                        value: 0
                    })
                })
            );
        }

        // Revoke root
        eas.revoke(RevocationRequest({ schema: schemaId, data: RevocationRequestData({ uid: uids[0], value: 0 }) }));

        // Verify children remain valid
        for (uint256 i = 0; i < uids.length; i++) {
            Attestation memory attestation = eas.getAttestation(uids[i]);
            if (i == 0) {
                assertTrue(attestation.revocationTime > 0);
            } else {
                assertEq(attestation.revocationTime, 0);
            }
        }

        vm.stopPrank();
    }

    /// @dev Tests complex revocation scenarios with multiple attestations.
    ///      Demonstrates independence of revocations and proper
    ///      handling of referenced attestations in complex scenarios
    function test_advancedRevocation_succeeds(string[] memory _names) public {
        vm.assume(_names.length >= 3);
        string memory schema = "string name";
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
                    data: abi.encode(_names[0]),
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
                    data: abi.encode(_names[1]),
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
                    data: abi.encode(_names[2]),
                    value: 0
                })
            })
        );

        // Test revocation order effects
        eas.revoke(RevocationRequest({ schema: schemaId, data: RevocationRequestData({ uid: uids[0], value: 0 }) }));

        // Verify referenced attestation can still be revoked after its reference is revoked
        eas.revoke(RevocationRequest({ schema: schemaId, data: RevocationRequestData({ uid: uids[2], value: 0 }) }));

        // Verify attestation states
        for (uint256 i = 0; i < uids.length; i++) {
            Attestation memory attestation = eas.getAttestation(uids[i]);
            if (i == 0 || i == 2) {
                // First and third attestations should be revoked
                assertTrue(attestation.revocationTime > 0, "Attestation should be revoked");
            } else {
                // Second attestation should not be revoked
                assertTrue(attestation.revocationTime == 0, "Attestation should not be revoked");
            }
        }

        vm.stopPrank();
    }

    /// @dev Tests error conditions for multi-attestation delegation revocation.
    ///      Ensures proper validation of batch revocation parameters
    function test_attestationMultiDelegation_invalidLength_reverts(string[2] memory _names, uint8 _v) public {
        string memory schema = "string name";
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
                    data: abi.encode(_names[0]),
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
                    data: abi.encode(_names[1]),
                    value: 0
                })
            })
        );

        // Test revert with inconsistent lengths
        MultiDelegatedRevocationRequest[] memory requests = new MultiDelegatedRevocationRequest[](1);
        requests[0].schema = schemaId;
        requests[0].data = new RevocationRequestData[](2);
        requests[0].data[0].uid = uid1;
        requests[0].data[0].value = 0;
        requests[0].data[1].uid = uid2;
        requests[0].data[1].value = 0;

        requests[0].signatures = new Signature[](1);
        requests[0].signatures[0] = Signature({ v: _v, r: bytes32(uint256(1)), s: bytes32(uint256(2)) });
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

    /// Off-chain revocation tests
    /// @dev Tests off-chain revocation functionality.
    ///      Demonstrates off-chain revocation mechanism,
    ///      useful for revoking attestations without on-chain
    ///      transaction for each attestation
    function test_revoke_offchain_succeeds(bytes memory _randomData) public {
        bytes32 data = keccak256(_randomData);

        vm.prank(sender);
        uint256 timestamp = block.timestamp;
        eas.revokeOffchain(data);

        assertEq(eas.getRevokeOffchain(sender, data), timestamp);
    }

    /// @dev Tests off-chain revocation behavior.
    ///      Ensures off-chain revocations cannot be duplicated
    function test_revokeoffChain_alreadyRevoked_reverts(bytes memory _randomData) public {
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
    ///      Demonstrates efficient batch processing of off-chain revocations
    function test_revocationMulti_mixed_succeeds(bytes memory _randomData, bytes memory _randomData2) public {
        vm.assume(_randomData.length > 0 && _randomData2.length > 0);
        vm.assume(keccak256(_randomData) != keccak256(_randomData2));
        bytes32[] memory data = new bytes32[](3);
        data[0] = keccak256(_randomData);
        data[1] = keccak256(_randomData2);
        data[2] = bytes32(0);

        vm.prank(sender);
        uint256 timestamp = block.timestamp;
        eas.multiRevokeOffchain(data);

        for (uint256 i = 0; i < data.length; i++) {
            assertEq(eas.getRevokeOffchain(sender, data[i]), timestamp);
        }
    }

    /// @dev Tests off-chain revocation independence between accounts.
    function test_revokeoffChain_differentAccounts_succeeds(bytes memory _randomData) public {
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
    ///      Ensures that batch revocations properly check
    ///      for already revoked data in all scenarios
    function test_revokeMultiOffchain_alreadyRevoked_reverts(
        bytes memory _randomData,
        bytes memory _randomData2,
        bytes memory _randomData3
    )
        public
    {
        vm.assume(_randomData.length > 0 && _randomData2.length > 0 && _randomData3.length > 0);
        vm.assume(keccak256(_randomData) != keccak256(_randomData2));
        vm.assume(keccak256(_randomData) != keccak256(_randomData3));
        vm.assume(keccak256(_randomData2) != keccak256(_randomData3));

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
    ///      Ensures system properly handles queries for
    ///      non-existent off-chain revocations
    function test_revokeoffChain_unregistered_succeeds(bytes memory _randomData) public view {
        bytes32 data = keccak256(_randomData);
        assertEq(eas.getRevokeOffchain(sender, data), 0);
    }

    /// @dev Tests multiple account off-chain revocations of the same data.
    function test_revokeoffChain_multipleAccounts_succeeds(bytes memory _randomData) public {
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
    ///      Demonstrates the system's ability to handle multiple batch
    ///      revocations from the same account.
    function test_revokeoffChain_multiple_succeeds(
        uint256 _randomData1,
        uint256 _randomData2,
        uint256 _randomData3,
        uint256 _randomData4,
        uint256 _randomData5
    )
        public
    {
        // Ensure that all random data inputs are unique
        vm.assume(
            _randomData1 != _randomData2 && _randomData1 != _randomData3 && _randomData1 != _randomData4
                && _randomData1 != _randomData5 && _randomData2 != _randomData3 && _randomData2 != _randomData4
                && _randomData2 != _randomData5 && _randomData3 != _randomData4 && _randomData3 != _randomData5
                && _randomData4 != _randomData5
        );

        // Create the first batch of data to revoke
        bytes32[] memory data = new bytes32[](3);
        data[0] = keccak256(abi.encodePacked(_randomData1, block.timestamp, uint256(1)));
        data[1] = keccak256(abi.encodePacked(_randomData2, block.timestamp, uint256(2)));
        data[2] = keccak256(abi.encodePacked(_randomData3, block.timestamp, uint256(3)));

        vm.startPrank(sender);

        // Test multiple revocations in one transaction
        uint256 timestamp = block.timestamp;
        eas.multiRevokeOffchain(data);

        // Verify all revocations
        for (uint256 i = 0; i < data.length; i++) {
            assertEq(eas.getRevokeOffchain(sender, data[i]), timestamp);
        }

        // Create the second batch of data to revoke
        bytes32[] memory data2 = new bytes32[](2);
        data2[0] = keccak256(abi.encodePacked(_randomData4, block.timestamp, uint256(4)));
        data2[1] = keccak256(abi.encodePacked(_randomData5, block.timestamp, uint256(5)));

        eas.multiRevokeOffchain(data2);

        // Verify second batch
        for (uint256 i = 0; i < data2.length; i++) {
            assertEq(eas.getRevokeOffchain(sender, data2[i]), timestamp);
        }

        vm.stopPrank();
    }

    function test_revokeMultiOffchain_alreadyRevoked_Revert(
        bytes memory _data1,
        bytes memory _data2,
        bytes memory _data3
    )
        public
    {
        // Ensure data is unique and not empty
        vm.assume(_data1.length > 0);
        vm.assume(_data2.length > 0);
        vm.assume(_data3.length > 0);
        vm.assume(keccak256(_data1) != keccak256(_data2));
        vm.assume(keccak256(_data2) != keccak256(_data3));
        vm.assume(keccak256(_data1) != keccak256(_data3));

        bytes32[] memory data = new bytes32[](3);
        data[0] = keccak256(_data1);
        data[1] = keccak256(_data2);
        data[2] = keccak256(_data3);

        // First revocation should succeed
        vm.prank(sender);
        eas.multiRevokeOffchain(data);

        // Second revocation of same data should fail
        vm.prank(sender);
        vm.expectRevert(AlreadyRevokedOffchain.selector);
        eas.multiRevokeOffchain(data);

        // Even a partial overlap should fail
        bytes32[] memory newData = new bytes32[](2);
        newData[0] = data[0]; // Using previously revoked data
        newData[1] = keccak256(abi.encodePacked("new data")); // New data

        vm.prank(sender);
        vm.expectRevert(AlreadyRevokedOffchain.selector);
        eas.multiRevokeOffchain(newData);
    }
}
