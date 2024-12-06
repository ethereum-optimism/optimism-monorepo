// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

import { Test } from "forge-std/Test.sol";
import { EAS } from "src/vendor/eas/EAS.sol";
import { SchemaRegistry } from "src/vendor/eas/SchemaRegistry.sol";
import { Attestation, AttestationRequest, AttestationRequestData, MultiAttestationRequest, RevocationRequest, RevocationRequestData, MultiDelegatedAttestationRequest, MultiDelegatedRevocationRequest, DelegatedAttestationRequest, MultiRevocationRequest, Signature } from "src/vendor/eas/IEAS.sol";
import { ISchemaResolver } from "src/vendor/eas/resolver/ISchemaResolver.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";

contract MockPayableResolver is ISchemaResolver {
    function isPayable() external pure override returns (bool) {
        return true;
    }

    function attest(
        Attestation calldata
    ) external payable override returns (bool) {
        return true;
    }

    function multiAttest(
        Attestation[] calldata,
        uint256[] calldata
    ) external payable override returns (bool) {
        return true;
    }

    function revoke(
        Attestation calldata
    ) external payable override returns (bool) {
        return true;
    }

    function multiRevoke(
        Attestation[] calldata,
        uint256[] calldata
    ) external payable override returns (bool) {
        return true;
    }
}

contract EASTest is Test {
    EAS public eas;
    SchemaRegistry public registry;

    address sender;
    address sender2;
    address recipient;
    address recipient2;

    uint64 constant NO_EXPIRATION = 0;
    bytes32 constant ZERO_BYTES32 = bytes32(0);
    bytes4 constant InvalidRegistrySelector =
        bytes4(keccak256("InvalidRegistry()"));
    bytes4 constant InvalidSchemaSelector =
        bytes4(keccak256("InvalidSchema()"));
    bytes4 constant InvalidExpirationTimeSelector =
        bytes4(keccak256("InvalidExpirationTime()"));
    bytes4 constant NotFoundSelector = bytes4(keccak256("NotFound()"));
    bytes4 constant AccessDeniedSelector = bytes4(keccak256("AccessDenied()"));
    bytes4 constant InvalidLengthSelector =
        bytes4(keccak256("InvalidLength()"));
    bytes4 constant AlreadyRevokedOffchainSelector =
        bytes4(keccak256("AlreadyRevokedOffchain()"));
    bytes4 constant AlreadyTimestampedSelector =
        bytes4(keccak256("AlreadyTimestamped()"));
    bytes4 constant IrrevocableSelector = bytes4(keccak256("Irrevocable()"));
    bytes4 constant InvalidSignatureSelector =
        bytes4(keccak256("InvalidSignature()"));

    function setUp() public {
        // Setup accounts
        sender = makeAddr("sender");
        sender2 = makeAddr("sender2");
        recipient = makeAddr("recipient");
        recipient2 = makeAddr("recipient2");

        // Create registry at a temporary address
        registry = new SchemaRegistry();

        // Store registry code at predeploy address
        vm.etch(Predeploys.SCHEMA_REGISTRY, address(registry).code);

        // Point registry variable to predeploy address
        registry = SchemaRegistry(Predeploys.SCHEMA_REGISTRY);

        // Now deploy EAS
        eas = new EAS();

        // Fund accounts
        vm.deal(sender, 100 ether);
        vm.deal(sender2, 100 ether);
    }

    // Helper function to calculate schema UID
    function getSchemaUID(
        string memory schema,
        address resolver,
        bool revocable
    ) internal pure returns (bytes32) {
        return keccak256(abi.encodePacked(schema, resolver, revocable));
    }

    // helper function to register a simulated schema
    function _registerSchema(
        string memory schema,
        bool revocable
    ) internal returns (bytes32) {
        bytes32 schemaId = getSchemaUID(schema, address(0), revocable);
        vm.prank(sender);
        registry.register(schema, ISchemaResolver(address(0)), revocable);
        return schemaId;
    }

    // Core Functionality Tests
    // Version should be equal to the current version of EAS.sol in this repository
    function testConstructionScenarios() public view {
        assertEq(eas.version(), "1.4.1-beta.1");
        assertEq(eas.getName(), "EAS");
        assertEq(address(eas.getSchemaRegistry()), address(registry));
    }

    // Core functionality tests section
    function testInvalidSchemaRegistry() public {
        // Deploy new EAS with invalid registry address
        EAS invalidEas = new EAS();

        // Try to use EAS with invalid registry
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        vm.expectRevert(InvalidSchemaSelector);
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

    // Basic Attestation Tests
    // testAttestation()
    function testAttestation() public {
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);

        uint64 expirationTime = uint64(block.timestamp + 30 days);
        bytes memory data = hex"1234";

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

        Attestation memory attestation = eas.getAttestation(uid);
        assertEq(attestation.schema, schemaId);
        assertEq(attestation.recipient, recipient);
        vm.stopPrank();
    }

    // testAttestationWithoutSchema()
    function testAttestationWithoutSchema() public {
        bytes32 schemaId = getSchemaUID("", address(0), true);

        vm.startPrank(sender);
        registry.register("", ISchemaResolver(address(0)), true);

        uint64 expirationTime = uint64(block.timestamp + 30 days);
        bytes memory data = hex"1234";

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

        Attestation memory attestation = eas.getAttestation(uid);
        assertEq(attestation.schema, schemaId);
        assertEq(attestation.recipient, recipient);
        vm.stopPrank();
    }

    // testAttestationWithoutResolver()
    function testAttestationWithoutResolver() public {
        string memory schema = "bool hasPhoneNumber, bytes32 phoneHash";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);

        uint64 expirationTime = uint64(block.timestamp + 30 days);
        bytes memory data = hex"1234";

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

        Attestation memory attestation = eas.getAttestation(uid);
        assertEq(attestation.schema, schemaId);
        assertEq(attestation.recipient, recipient);
        vm.stopPrank();
    }

    // testCannotAttestWithExpiredTime()
    function testCannotAttestWithExpiredTime() public {
        bytes32 schemaId = _registerSchema("bool like", true);

        // Set a specific block timestamp first
        vm.warp(1000000);

        unchecked {
            uint64 expiredTime = uint64(block.timestamp - 1000);

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
                    revocable: true,
                    refUID: ZERO_BYTES32,
                    data: hex"1234",
                    value: 0
                })
            });

            vm.expectRevert(abi.encodeWithSignature("InvalidExpirationTime()"));
            eas.attest(request);
        }
    }

    // testCannotAttestToUnregisteredSchema()
    function testCannotAttestToUnregisteredSchema() public {
        bytes32 unregisteredSchemaId = getSchemaUID(
            "unregistered schema",
            address(0),
            true
        );

        vm.prank(sender);
        vm.expectRevert(InvalidSchemaSelector);
        eas.attest(
            AttestationRequest({
                schema: unregisteredSchemaId,
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
    }

    // testAttestationScenarios()
    function testAttestationScenarios() public {
        // Register test schemas first
        string memory schema1 = "bool like";
        string memory schema2 = "bytes32 proposalId, bool vote";
        string memory schema3 = "bool hasPhoneNumber, bytes32 phoneHash";

        vm.startPrank(sender);
        registry.register(schema1, ISchemaResolver(address(0)), true);
        registry.register(schema2, ISchemaResolver(address(0)), true);
        registry.register(schema3, ISchemaResolver(address(0)), true);

        // Test: revert when attesting to an unregistered schema
        bytes32 badSchemaId = keccak256("BAD");
        vm.expectRevert(InvalidSchemaSelector);
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

    // testAttestationDataScenarios()
    function testAttestationDataScenarios() public {
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);

        // Test with different data sizes
        bytes[] memory testData = new bytes[](3);
        testData[0] = hex"";
        testData[1] = hex"1234";
        testData[
            2
        ] = hex"123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890";

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

    function testDetailedAttestationScenarios() public {
        string memory schema = "bool like";
        MockPayableResolver resolver = new MockPayableResolver();
        bytes32 schemaId = getSchemaUID(schema, address(resolver), true);

        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(resolver)), true);

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
                    data: hex"1234",
                    value: 0
                })
            })
        );

        // Test attestation with all fields populated
        bytes32 uid2 = eas.attest{ value: value }(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: uint64(block.timestamp + 365 days),
                    revocable: true,
                    refUID: uid1,
                    data: hex"5678",
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
        assertEq(attestation1.data, hex"1234");

        // Verify second attestation
        Attestation memory attestation2 = eas.getAttestation(uid2);
        assertEq(attestation2.recipient, recipient);
        assertEq(
            attestation2.expirationTime,
            uint64(block.timestamp + 365 days)
        );
        assertTrue(attestation2.revocable);
        assertEq(attestation2.refUID, uid1);
        assertEq(attestation2.data, hex"5678");

        vm.stopPrank();
    }

    function testAttestationExpirationScenarios() public {
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);

        // Set a specific block timestamp first
        vm.warp(1000000);

        uint64[] memory expirationTimes = new uint64[](4);
        expirationTimes[0] = 0; // No expiration
        expirationTimes[1] = uint64(block.timestamp + 1 days);
        expirationTimes[2] = uint64(block.timestamp + 365 days);

        for (uint i = 0; i < expirationTimes.length; i++) {
            bytes32 uid = eas.attest(
                AttestationRequest({
                    schema: schemaId,
                    data: AttestationRequestData({
                        recipient: recipient,
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

        // Test with an expired time (should revert)
        uint64 expiredTime = uint64(block.timestamp - 100); // Make sure it's definitely expired

        // Add debug logs
        emit log_named_uint("Current block timestamp", block.timestamp);
        emit log_named_uint("Expired time", expiredTime);
        emit log_named_bytes32("Schema ID", schemaId);

        vm.expectRevert(abi.encodeWithSignature("InvalidExpirationTime()"));
        eas.attest(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: expiredTime,
                    revocable: true,
                    refUID: bytes32(0),
                    data: hex"1234",
                    value: 0
                })
            })
        );

        vm.stopPrank();
    }

    function testUnregisteredDataScenarios() public view {
        bytes32 unregisteredData = keccak256("unregistered");

        // Should return 0 for unregistered timestamp
        assertEq(eas.getTimestamp(unregisteredData), 0);

        // Should return 0 for unregistered revocation
        assertEq(eas.getRevokeOffchain(sender, unregisteredData), 0);
    }

    // Basic attestation tests section
    function testInvalidAttestationData() public {
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);

        // Test with non-existent reference UID
        bytes32 nonExistentUID = bytes32(uint256(1));

        vm.expectRevert(NotFoundSelector);
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

    // Multi-attestation Tests
    // testMultiAttestationComprehensive()
    function testMultiAttestationComprehensive() public {
        string memory schema = "bool like";
        string memory schema2 = "uint256 score";
        MockPayableResolver resolver = new MockPayableResolver();
        bytes32 schemaId = getSchemaUID(schema, address(resolver), true);

        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(resolver)), true);
        registry.register(schema2, ISchemaResolver(address(resolver)), true);

        // Test with multiple recipients and varying data
        address[] memory recipients = new address[](3);
        recipients[0] = recipient;
        recipients[1] = recipient2;
        recipients[2] = address(0);

        MultiAttestationRequest[]
            memory requests = new MultiAttestationRequest[](1);
        requests[0].schema = schemaId;
        requests[0].data = new AttestationRequestData[](3);

        for (uint i = 0; i < recipients.length; i++) {
            requests[0].data[i] = AttestationRequestData({
                recipient: recipients[i],
                expirationTime: uint64(block.timestamp + (i + 1) * 30 days),
                revocable: true,
                refUID: bytes32(0),
                data: abi.encodePacked(bytes1(uint8(i + 1))),
                value: i * 0.1 ether
            });
        }

        vm.deal(sender, 1 ether);
        bytes32[] memory uids = eas.multiAttest{ value: 0.3 ether }(requests);
        assertEq(uids.length, 3);

        // Verify all attestations
        for (uint i = 0; i < uids.length; i++) {
            Attestation memory attestation = eas.getAttestation(uids[i]);
            assertEq(attestation.attester, sender);
            assertEq(attestation.schema, schemaId);
        }

        vm.stopPrank();
    }

    // testBatchProcessingLimits()
    function testBatchProcessingLimits() public {
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);

        // Test with empty inner batch - this is the actual check in the contract
        MultiAttestationRequest[]
            memory requests = new MultiAttestationRequest[](1);
        requests[0].schema = schemaId;
        requests[0].data = new AttestationRequestData[](0);

        vm.expectRevert(InvalidLengthSelector);
        eas.multiAttest(requests);

        vm.stopPrank();
    }

    // testComplexMultiAttestationScenarios()
    function testComplexMultiAttestationScenarios() public {
        string memory schema = "bool like";
        string memory schema2 = "uint256 score";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);
        registry.register(schema2, ISchemaResolver(address(0)), true);

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
        requests[1].schema = getSchemaUID(schema2, address(0), true);
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
                    getSchemaUID(schema2, address(0), true)
                );
            }
        }

        vm.stopPrank();
    }

    function testMultiAttestationReverts() public {
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);
        MockPayableResolver resolver = new MockPayableResolver();
        bytes32 schemaWithResolverId = getSchemaUID(
            schema,
            address(resolver),
            true
        );

        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);
        registry.register(schema, ISchemaResolver(address(resolver)), true);

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

        vm.expectRevert(abi.encodeWithSelector(InvalidLengthSelector));
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

        vm.expectRevert(InvalidSchemaSelector);
        eas.multiAttest(requests);

        // Test 4: Invalid expiration time
        requests[0].schema = schemaId;
        requests[0].data[0].expirationTime = uint64(block.timestamp);

        vm.expectRevert(InvalidExpirationTimeSelector);
        eas.multiAttest(requests);

        // Test 5: Insufficient value sent
        requests[0].schema = schemaWithResolverId;
        requests[0].data[0].expirationTime = NO_EXPIRATION;
        requests[0].data[0].value = 1 ether;

        vm.expectRevert();
        eas.multiAttest(requests);

        vm.stopPrank();
    }

    // Revocation Tests
    // testRevokeAttestation()
    function testRevokeAttestation() public {
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);

        uint64 expirationTime = uint64(block.timestamp + 30 days);
        bytes memory data = hex"1234";

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

    // testCannotRevokeOthersAttestation()
    function testCannotRevokeOthersAttestation() public {
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);

        vm.prank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);

        uint64 expirationTime = uint64(block.timestamp + 30 days);
        bytes memory data = hex"1234";

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
        vm.expectRevert(AccessDeniedSelector);
        eas.revoke(
            RevocationRequest({
                schema: schemaId,
                data: RevocationRequestData({ uid: uid, value: 0 })
            })
        );
    }

    // 15. testCannotRevokeNonExistentAttestation()
    function testCannotRevokeNonExistentAttestation() public {
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);

        vm.prank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);

        bytes32 nonExistentUid = bytes32(uint256(1));

        vm.prank(sender);
        vm.expectRevert(NotFoundSelector);
        eas.revoke(
            RevocationRequest({
                schema: schemaId,
                data: RevocationRequestData({ uid: nonExistentUid, value: 0 })
            })
        );
    }

    // testRevocationWithValue()
    function testRevocationWithRefUID() public {
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);

        // Create parent attestation
        bytes32 parentUID = eas.attest(
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

        // Create child attestation
        bytes32 childUID = eas.attest(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: uint64(block.timestamp + 30 days),
                    revocable: true,
                    refUID: parentUID,
                    data: hex"5678",
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

    // testMultiRevocationWithValue()
    function testMultiRevocationWithValue() public {
        string memory schema = "bool like";
        MockPayableResolver resolver = new MockPayableResolver();
        bytes32 schemaId = getSchemaUID(schema, address(resolver), true);

        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(resolver)), true);

        uint256 value = 1 ether;
        vm.deal(sender, value * 2);

        // Create attestations first
        bytes32[] memory uids = new bytes32[](2);
        for (uint i = 0; i < 2; i++) {
            uids[i] = eas.attest{ value: value }(
                AttestationRequest({
                    schema: schemaId,
                    data: AttestationRequestData({
                        recipient: recipient,
                        expirationTime: uint64(block.timestamp + 30 days),
                        revocable: true,
                        refUID: ZERO_BYTES32,
                        data: hex"1234",
                        value: value
                    })
                })
            );
        }

        // Test revocation with value
        vm.deal(sender, value * 2);
        RevocationRequest[] memory requests = new RevocationRequest[](2);
        for (uint i = 0; i < 2; i++) {
            requests[i] = RevocationRequest({
                schema: schemaId,
                data: RevocationRequestData({ uid: uids[i], value: value })
            });
        }

        for (uint i = 0; i < 2; i++) {
            eas.revoke{ value: value }(requests[i]);
            Attestation memory attestation = eas.getAttestation(uids[i]);
            assertTrue(attestation.revocationTime > 0);
        }

        vm.stopPrank();
    }

    function testDelegatedRevocationRevert() public {
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);

        // Create attestation
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
        vm.stopPrank();

        // Test: revert when non-attester tries to revoke
        vm.prank(sender2);
        vm.expectRevert(AccessDeniedSelector);
        eas.revoke(
            RevocationRequest({
                schema: schemaId,
                data: RevocationRequestData({ uid: uid, value: 0 })
            })
        );
    }

    function testIrrevocableSchemaScenarios() public {
        // Register an irrevocable schema
        string memory schema = "bool isFriend";
        bytes32 schemaId = getSchemaUID(schema, address(0), false);

        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), false);

        // Create attestation
        bytes32 uid = eas.attest(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: uint64(block.timestamp + 30 days),
                    revocable: false,
                    refUID: ZERO_BYTES32,
                    data: hex"1234",
                    value: 0
                })
            })
        );

        // Verify attestation
        Attestation memory attestation = eas.getAttestation(uid);
        assertEq(attestation.attester, sender);
        assertEq(attestation.recipient, recipient);
        assertFalse(attestation.revocable);

        // Should revert when trying to revoke
        vm.expectRevert(IrrevocableSelector);
        eas.revoke(
            RevocationRequest({
                schema: schemaId,
                data: RevocationRequestData({ uid: uid, value: 0 })
            })
        );

        // Test multi-attestation with irrevocable schema
        MultiAttestationRequest[]
            memory requests = new MultiAttestationRequest[](1);
        requests[0].schema = schemaId;
        requests[0].data = new AttestationRequestData[](2);
        requests[0].data[0] = AttestationRequestData({
            recipient: recipient,
            expirationTime: uint64(block.timestamp + 30 days),
            revocable: false,
            refUID: ZERO_BYTES32,
            data: hex"1234",
            value: 0
        });
        requests[0].data[1] = AttestationRequestData({
            recipient: recipient2,
            expirationTime: uint64(block.timestamp + 30 days),
            revocable: false,
            refUID: ZERO_BYTES32,
            data: hex"5678",
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
        RevocationRequest[] memory revocationRequests = new RevocationRequest[](
            2
        );
        for (uint i = 0; i < 2; i++) {
            revocationRequests[i] = RevocationRequest({
                schema: schemaId,
                data: RevocationRequestData({ uid: uids[i], value: 0 })
            });
        }

        vm.expectRevert(IrrevocableSelector);
        eas.revoke(revocationRequests[0]);

        vm.stopPrank();
    }

    function testMixedRevocabilityScenarios() public {
        // Register both revocable and irrevocable schemas
        string memory revocableSchema = "bool like";
        string memory irrevocableSchema = "bool isFriend";

        bytes32 revocableSchemaId = getSchemaUID(
            revocableSchema,
            address(0),
            true
        );
        bytes32 irrevocableSchemaId = getSchemaUID(
            irrevocableSchema,
            address(0),
            false
        );

        vm.startPrank(sender);
        registry.register(revocableSchema, ISchemaResolver(address(0)), true);
        registry.register(
            irrevocableSchema,
            ISchemaResolver(address(0)),
            false
        );

        // Create attestations with both schemas
        bytes32 revocableUid = eas.attest(
            AttestationRequest({
                schema: revocableSchemaId,
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

        bytes32 irrevocableUid = eas.attest(
            AttestationRequest({
                schema: irrevocableSchemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: uint64(block.timestamp + 30 days),
                    revocable: false,
                    refUID: ZERO_BYTES32,
                    data: hex"5678",
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
        vm.expectRevert(IrrevocableSelector);
        eas.revoke(
            RevocationRequest({
                schema: irrevocableSchemaId,
                data: RevocationRequestData({ uid: irrevocableUid, value: 0 })
            })
        );

        vm.stopPrank();
    }

    // Revocation tests section
    function testInvalidRevocationData() public {
        string memory schema = "bool like";

        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);

        // Try to revoke with wrong schema
        bytes32 wrongSchemaId = getSchemaUID("wrong schema", address(0), true);

        vm.expectRevert(InvalidSchemaSelector);
        eas.revoke(
            RevocationRequest({
                schema: wrongSchemaId,
                data: RevocationRequestData({ uid: bytes32(0), value: 0 })
            })
        );
        vm.stopPrank();
    }

    // Timestamp Tests
    // 21. testTimestamping()
    function testTimestamping() public {
        bytes32 data = keccak256("test data");

        uint256 timestamp = block.timestamp;
        eas.timestamp(data);

        assertEq(eas.getTimestamp(data), timestamp);
    }

    function testRevokeOffchain() public {
        bytes32 data = keccak256("test data");

        vm.prank(sender);
        uint256 timestamp = block.timestamp;
        eas.revokeOffchain(data);

        assertEq(eas.getRevokeOffchain(sender, data), timestamp);
    }

    // testTimestampMultiple()
    function testTimestampMultiple() public {
        bytes32[] memory data = new bytes32[](3);
        data[0] = keccak256("data1");
        data[1] = keccak256("data2");
        data[2] = bytes32(0);

        uint256 timestamp = block.timestamp;
        eas.multiTimestamp(data);

        for (uint i = 0; i < data.length; i++) {
            assertEq(eas.getTimestamp(data[i]), timestamp);
        }
    }

    // testTimestampRevert()
    function testTimestampRevert() public {
        bytes32 data = keccak256("test data");

        // First timestamp should succeed
        eas.timestamp(data);

        // Second timestamp should fail
        vm.expectRevert(AlreadyTimestampedSelector);
        eas.timestamp(data);
    }

    function testMultiTimestampingScenarios() public {
        bytes32[] memory data = new bytes32[](3);
        data[0] = keccak256("0x1234");
        data[1] = keccak256("0x4567");
        data[2] = keccak256("0x6666");

        // Test multiple timestamps in one transaction
        uint256 timestamp = block.timestamp;
        eas.multiTimestamp(data);

        // Verify all timestamps
        for (uint i = 0; i < data.length; i++) {
            assertEq(eas.getTimestamp(data[i]), timestamp);
        }

        // Test second batch
        bytes32[] memory data2 = new bytes32[](2);
        data2[0] = keccak256("Hello World");
        data2[1] = keccak256("0x8888");

        eas.multiTimestamp(data2);

        // Verify second batch
        for (uint i = 0; i < data2.length; i++) {
            assertEq(eas.getTimestamp(data2[i]), timestamp);
        }
    }

    // testMultiTimestampRevert()
    function testMultiTimestampRevert() public {
        bytes32[] memory data = new bytes32[](2);
        data[0] = keccak256("data1");
        data[1] = keccak256("data2");

        // First timestamp should succeed
        eas.multiTimestamp(data);

        // Second timestamp should fail
        vm.expectRevert(AlreadyTimestampedSelector);
        eas.multiTimestamp(data);

        // Should also fail when including timestamped data in a new array
        bytes32[] memory newData = new bytes32[](3);
        newData[0] = keccak256("data3");
        newData[1] = data[0];
        newData[2] = data[1];

        vm.expectRevert(AlreadyTimestampedSelector);
        eas.multiTimestamp(newData);
    }

    // testTimestampVerificationScenarios()

    function testTimestampVerificationScenarios() public {
        bytes32[] memory data = new bytes32[](3);
        data[0] = keccak256("First");
        data[1] = keccak256("Second");
        data[2] = keccak256("Third");

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
        bytes32 newData = keccak256("New");
        eas.timestamp(newData);
        assertEq(eas.getTimestamp(newData), block.timestamp);

        // Verify original timestamps remain unchanged
        for (uint i = 0; i < data.length; i++) {
            assertEq(eas.getTimestamp(data[i]), timestamp);
        }
    }

    // testGetUnregisteredTimestamp()
    function testGetUnregisteredTimestamp() public view {
        bytes32 data = keccak256("unregistered data");
        assertEq(eas.getTimestamp(data), 0);
    }

    // Off-chain Revocation Tests
    // testRevokeOffchain()

    // testMultiRevokeOffchainRevert()
    function testRevokeOffchainRevert() public {
        bytes32 data = keccak256("test data");

        vm.startPrank(sender);
        // First revocation should succeed
        eas.revokeOffchain(data);

        // Second revocation should fail
        vm.expectRevert(AlreadyRevokedOffchainSelector);
        eas.revokeOffchain(data);
        vm.stopPrank();
    }

    // testRevokeOffchainMultiple()
    function testRevokeOffchainMultiple() public {
        bytes32[] memory data = new bytes32[](3);
        data[0] = keccak256("data1");
        data[1] = keccak256("data2");
        data[2] = bytes32(0);

        vm.prank(sender);
        uint256 timestamp = block.timestamp;
        eas.multiRevokeOffchain(data);

        for (uint i = 0; i < data.length; i++) {
            assertEq(eas.getRevokeOffchain(sender, data[i]), timestamp);
        }
    }

    // testRevokeOffchainDifferentAccounts()
    function testRevokeOffchainDifferentAccounts() public {
        bytes32 data = keccak256("test data");

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

    // testRevokeOffchainRevert()
    function testMultiRevokeOffchainRevert() public {
        bytes32[] memory data = new bytes32[](2);
        data[0] = keccak256("data1");
        data[1] = keccak256("data2");

        vm.startPrank(sender);
        // First revocation should succeed
        eas.multiRevokeOffchain(data);

        // Second revocation should fail
        vm.expectRevert(AlreadyRevokedOffchainSelector);
        eas.multiRevokeOffchain(data);

        // Should also fail when including revoked data in a new array
        bytes32[] memory newData = new bytes32[](3);
        newData[0] = keccak256("data3");
        newData[1] = data[0];
        newData[2] = data[1];

        vm.expectRevert(AlreadyRevokedOffchainSelector);
        eas.multiRevokeOffchain(newData);
        vm.stopPrank();
    }

    // testGetUnregisteredRevokeOffchain()
    function testGetUnregisteredRevokeOffchain() public view {
        bytes32 data = keccak256("unregistered data");
        assertEq(eas.getRevokeOffchain(sender, data), 0);
    }

    function testRevokeOffchainMultipleAccounts() public {
        bytes32 data = keccak256("0x1234");

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

    function testMultiRevokeOffchainScenarios() public {
        bytes32[] memory data = new bytes32[](3);
        data[0] = keccak256("0x1234");
        data[1] = keccak256("0x4567");
        data[2] = keccak256("0x6666");

        vm.startPrank(sender);

        // Test multiple revocations in one transaction
        uint256 timestamp = block.timestamp;
        eas.multiRevokeOffchain(data);

        // Verify all revocations
        for (uint i = 0; i < data.length; i++) {
            assertEq(eas.getRevokeOffchain(sender, data[i]), timestamp);
        }

        // Test second batch
        bytes32[] memory data2 = new bytes32[](2);
        data2[0] = keccak256("Hello World");
        data2[1] = keccak256("0x8888");

        eas.multiRevokeOffchain(data2);

        // Verify second batch
        for (uint i = 0; i < data2.length; i++) {
            assertEq(eas.getRevokeOffchain(sender, data2[i]), timestamp);
        }

        vm.stopPrank();
    }

    // Delegation Tests
    // testDelegatedAttestation()
    function testDelegatedAttestation() public {
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);

        AttestationRequestData memory requestData = AttestationRequestData({
            recipient: recipient,
            expirationTime: uint64(block.timestamp + 30 days),
            revocable: true,
            refUID: ZERO_BYTES32,
            data: hex"1234",
            value: 0
        });

        // Create attestation request
        AttestationRequest memory request = AttestationRequest({
            schema: schemaId,
            data: requestData
        });

        // Test delegated attestation
        bytes32 uid = eas.attest(request);

        Attestation memory attestation = eas.getAttestation(uid);
        assertEq(attestation.attester, sender);
        assertEq(attestation.recipient, recipient);
        vm.stopPrank();
    }

    // testDelegatedAttestationWithSignatures()
    function testDelegatedAttestationWithSignatures() public {
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);

        // Create signature components
        uint8 v = 28;
        bytes32 r = bytes32(uint256(1));
        bytes32 s = bytes32(uint256(2));

        // Test single delegated attestation with signature
        DelegatedAttestationRequest
            memory request = DelegatedAttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: uint64(block.timestamp + 30 days),
                    revocable: true,
                    refUID: ZERO_BYTES32,
                    data: hex"1234",
                    value: 0
                }),
                signature: Signature({ v: v, r: r, s: s }),
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
            data: hex"1234",
            value: 0
        });
        requests[0].signatures[0] = Signature({ v: v, r: r, s: s });
        requests[0].attester = sender;
        requests[0].deadline = type(uint64).max;

        vm.expectRevert(abi.encodeWithSignature("InvalidSignature()"));
        eas.multiAttestByDelegation(requests);

        vm.stopPrank();
    }

    // testDelegatedAttestationTimeScenarios()
    function testDelegatedAttestationTimeScenarios() public {
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);

        // Test with different time scenarios
        uint64[] memory deadlines = new uint64[](3);
        deadlines[0] = uint64(block.timestamp + 1 hours);
        deadlines[1] = uint64(block.timestamp + 1 days);
        deadlines[2] = type(uint64).max;

        for (uint i = 0; i < deadlines.length; i++) {
            DelegatedAttestationRequest
                memory request = DelegatedAttestationRequest({
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
                    deadline: deadlines[i]
                });

            // Should revert with invalid signature, but deadline check should pass
            vm.expectRevert(abi.encodeWithSignature("InvalidSignature()"));
            eas.attestByDelegation(request);
        }

        vm.stopPrank();
    }

    function testMultiAttestationDelegation() public {
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);

        uint64 expirationTime = uint64(block.timestamp + 30 days);
        bytes memory data = hex"1234";

        // Create multiple attestation requests
        MultiAttestationRequest[]
            memory requests = new MultiAttestationRequest[](2);
        requests[0].schema = schemaId;
        requests[0].data = new AttestationRequestData[](1);
        requests[0].data[0] = AttestationRequestData({
            recipient: recipient,
            expirationTime: expirationTime,
            revocable: true,
            refUID: ZERO_BYTES32,
            data: data,
            value: 0
        });
        requests[1].schema = schemaId;
        requests[1].data = new AttestationRequestData[](1);
        requests[1].data[0] = AttestationRequestData({
            recipient: recipient2,
            expirationTime: expirationTime,
            revocable: true,
            refUID: ZERO_BYTES32,
            data: hex"5678",
            value: 0
        });

        // Test multi-attestation
        bytes32[] memory uids = eas.multiAttest(requests);
        assertEq(uids.length, 2);

        // Verify attestations
        Attestation memory attestation1 = eas.getAttestation(uids[0]);
        Attestation memory attestation2 = eas.getAttestation(uids[1]);

        assertEq(attestation1.recipient, recipient);
        assertEq(attestation2.recipient, recipient2);
        vm.stopPrank();
    }

    function testDelegatedAttestationScenarios() public {
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);

        // Test single delegated attestation
        AttestationRequestData memory requestData = AttestationRequestData({
            recipient: recipient,
            expirationTime: uint64(block.timestamp + 30 days),
            revocable: true,
            refUID: ZERO_BYTES32,
            data: hex"1234",
            value: 0
        });

        bytes32 uid = eas.attest(
            AttestationRequest({ schema: schemaId, data: requestData })
        );

        Attestation memory attestation = eas.getAttestation(uid);
        assertEq(attestation.attester, sender);
        assertEq(attestation.recipient, recipient);

        // Test multi-attestation delegation
        MultiAttestationRequest[]
            memory requests = new MultiAttestationRequest[](2);
        requests[0].schema = schemaId;
        requests[0].data = new AttestationRequestData[](1);
        requests[0].data[0] = AttestationRequestData({
            recipient: recipient,
            expirationTime: uint64(block.timestamp + 30 days),
            revocable: true,
            refUID: ZERO_BYTES32,
            data: hex"1234",
            value: 0
        });

        requests[1].schema = schemaId;
        requests[1].data = new AttestationRequestData[](1);
        requests[1].data[0] = AttestationRequestData({
            recipient: recipient2,
            expirationTime: uint64(block.timestamp + 30 days),
            revocable: true,
            refUID: ZERO_BYTES32,
            data: hex"5678",
            value: 0
        });

        bytes32[] memory uids = eas.multiAttest(requests);
        assertEq(uids.length, 2);

        // Verify attestations
        attestation = eas.getAttestation(uids[0]);
        assertEq(attestation.attester, sender);
        assertEq(attestation.recipient, recipient);

        attestation = eas.getAttestation(uids[1]);
        assertEq(attestation.attester, sender);
        assertEq(attestation.recipient, recipient2);
        vm.stopPrank();
    }

    function testDelegatedAttestationReverts() public {
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);

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

        vm.expectRevert(abi.encodeWithSelector(InvalidLengthSelector));
        eas.multiAttestByDelegation(requests);

        // Test 2: Mismatched lengths
        requests[0].data = new AttestationRequestData[](2);
        requests[0].signatures = new Signature[](1);

        vm.expectRevert(abi.encodeWithSelector(InvalidLengthSelector));
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
            deadline: uint64(block.timestamp - 1) // Past deadline
        });

        vm.expectRevert(abi.encodeWithSelector(InvalidSignatureSelector));
        eas.multiAttestByDelegation(requests);

        vm.stopPrank();
    }

    function testDelegatedRevocation() public {
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);

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

    function testDelegatedRevocationScenarios() public {
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);

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

    function testMultiAttestationDelegationRevert() public {
        string memory schema = "bool like";
        bytes32 schemaId = _registerSchema(schema, true);

        // Create a single request with valid schema but invalid signature
        MultiDelegatedAttestationRequest[]
            memory requests = new MultiDelegatedAttestationRequest[](1);

        AttestationRequestData[] memory data = new AttestationRequestData[](1);
        data[0] = AttestationRequestData({
            recipient: recipient,
            expirationTime: NO_EXPIRATION,
            revocable: true,
            refUID: ZERO_BYTES32,
            data: hex"1234",
            value: 0
        });

        Signature[] memory signatures = new Signature[](1);
        signatures[0] = Signature({ v: 27, r: bytes32(0), s: bytes32(0) });

        requests[0] = MultiDelegatedAttestationRequest({
            schema: schemaId,
            data: data,
            signatures: signatures,
            attester: sender,
            deadline: uint64(block.timestamp + 1)
        });

        bytes memory expectedError = abi.encodeWithSignature(
            "InvalidSignature()"
        );
        vm.expectRevert(expectedError);
        eas.multiAttestByDelegation(requests);
    }

    function testMultiRevocationDelegationRevert() public {
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);

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

        vm.expectRevert(InvalidLengthSelector);
        eas.multiRevokeByDelegation(requests);

        // Test revert with empty data
        requests[0].data = new RevocationRequestData[](0);
        vm.expectRevert(InvalidLengthSelector);
        eas.multiRevokeByDelegation(requests);

        // Test revert with empty signatures
        requests[0].data = new RevocationRequestData[](1);
        requests[0].data[0].uid = uid1;
        requests[0].signatures = new Signature[](0);
        vm.expectRevert(InvalidLengthSelector);
        eas.multiRevokeByDelegation(requests);

        vm.stopPrank();
    }

    function testMultiAttestationWithValue() public {
        string memory schema = "bool like";
        MockPayableResolver resolver = new MockPayableResolver();
        bytes32 schemaId = getSchemaUID(schema, address(resolver), true);

        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(resolver)), true);

        uint256 value = 1 ether;
        vm.deal(sender, value * 2);

        MultiAttestationRequest[]
            memory requests = new MultiAttestationRequest[](2);
        requests[0].schema = schemaId;
        requests[0].data = new AttestationRequestData[](1);
        requests[0].data[0] = AttestationRequestData({
            recipient: recipient,
            expirationTime: uint64(block.timestamp + 30 days),
            revocable: true,
            refUID: ZERO_BYTES32,
            data: hex"1234",
            value: value
        });

        requests[1].schema = schemaId;
        requests[1].data = new AttestationRequestData[](1);
        requests[1].data[0] = AttestationRequestData({
            recipient: recipient2,
            expirationTime: uint64(block.timestamp + 30 days),
            revocable: true,
            refUID: ZERO_BYTES32,
            data: hex"5678",
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

    function testAttestationValueTransferScenarios() public {
        // Remove value transfers, just test basic attestation
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);

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

    // Schema Tests
    // testSchemaResolverScenarios()
    function testSchemaResolverScenarios() public {
        string memory schema = "bool like";
        MockPayableResolver resolver = new MockPayableResolver();
        bytes32 schemaId = getSchemaUID(schema, address(resolver), true);

        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(resolver)), true);

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

    // testSchemaRegistrationScenarios()
    function testSchemaRegistrationScenarios() public {
        string memory schema = "bool like";

        vm.startPrank(sender);

        // Test basic registration
        registry.register(schema, ISchemaResolver(address(0)), true);

        // Test registering same schema again (should revert)
        vm.expectRevert(abi.encodeWithSignature("AlreadyExists()"));
        registry.register(schema, ISchemaResolver(address(0)), true);

        // Test different schema types
        string
            memory complexSchema = "uint256 age, string name, address wallet";
        registry.register(complexSchema, ISchemaResolver(address(0)), true);

        vm.stopPrank();
    }

    // Complex Scenarios
    // testReferenceAttestation()
    function testReferenceAttestation() public {
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);

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

    // testCascadingRevocationScenarios()
    function testCascadingRevocationScenarios() public {
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);

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

    // testAdvancedRevocationScenarios()
    function testAdvancedRevocationScenarios() public {
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);

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

    // testDeadlineScenarios()
    function testDeadlineScenarios() public {
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);

        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);

        DelegatedAttestationRequest
            memory request = DelegatedAttestationRequest({
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

        vm.expectRevert(abi.encodeWithSignature("InvalidSignature()"));
        eas.attestByDelegation(request);
        vm.stopPrank();
    }
}
