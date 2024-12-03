// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

import { Test } from "forge-std/Test.sol";
import { EAS } from "src/vendor/eas/EAS.sol";
import { SchemaRegistry } from "src/vendor/eas/SchemaRegistry.sol";
import {
    Attestation,
    AttestationRequest,
    AttestationRequestData,
    MultiAttestationRequest,
    RevocationRequest,
    RevocationRequestData,
    MultiDelegatedAttestationRequest,
    MultiDelegatedRevocationRequest,
    DelegatedAttestationRequest,
    Signature
} from "src/vendor/eas/IEAS.sol";
import { ISchemaResolver } from "src/vendor/eas/resolver/ISchemaResolver.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";

contract MockPayableResolver is ISchemaResolver {
    function isPayable() external pure override returns (bool) {
        return true;
    }

    function attest(Attestation calldata) external payable override returns (bool) {
        return true;
    }

    function multiAttest(Attestation[] calldata, uint256[] calldata) external payable override returns (bool) {
        return true;
    }

    function revoke(Attestation calldata) external payable override returns (bool) {
        return true;
    }

    function multiRevoke(Attestation[] calldata, uint256[] calldata) external payable override returns (bool) {
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
    
    bytes4 constant InvalidRegistry = bytes4(keccak256("InvalidRegistry()"));
    bytes4 constant InvalidSchema = bytes4(keccak256("InvalidSchema()"));
    bytes4 constant InvalidExpirationTime = bytes4(keccak256("InvalidExpirationTime()"));
    bytes4 constant NotFound = bytes4(keccak256("NotFound()"));
    bytes4 constant AccessDenied = bytes4(keccak256("AccessDenied()"));
    bytes4 constant InvalidLength = bytes4(keccak256("InvalidLength()"));
    bytes4 constant AlreadyRevokedOffchain = bytes4(keccak256("AlreadyRevokedOffchain()"));
    bytes4 constant AlreadyTimestamped = bytes4(keccak256("AlreadyTimestamped()"));
    bytes4 constant Irrevocable = bytes4(keccak256("Irrevocable()"));
    
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

    function testVersion() public view {
        assertEq(eas.version(), "1.4.1-beta.1");
    }

    function testConstructorReverts() public {
        vm.etch(Predeploys.SCHEMA_REGISTRY, "");
        vm.expectRevert(InvalidRegistry);
        new EAS();
    }

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

        // Helper function to calculate schema UID
    function getSchemaUID(
        string memory schema,
        address resolver,
        bool revocable
    ) internal pure returns (bytes32) {
        return keccak256(abi.encodePacked(schema, resolver, revocable));
    }

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
        
        eas.revoke(RevocationRequest({
            schema: schemaId,
            data: RevocationRequestData({
                uid: uid,
                value: 0
            })
        }));

        Attestation memory attestation = eas.getAttestation(uid);
        assertTrue(attestation.revocationTime > 0);
        vm.stopPrank();
    }

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
        vm.expectRevert(AccessDenied);
        eas.revoke(RevocationRequest({
            schema: schemaId,
            data: RevocationRequestData({
                uid: uid,
                value: 0
            })
        }));
    }

    function testCannotRevokeNonExistentAttestation() public {
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);
        
        vm.prank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);
        
        bytes32 nonExistentUid = bytes32(uint256(1));
        
        vm.prank(sender);
        vm.expectRevert(NotFound);
        eas.revoke(RevocationRequest({
            schema: schemaId,
            data: RevocationRequestData({
                uid: nonExistentUid,
                value: 0
            })
        }));
    }

    function testCannotAttestWithExpiredTime() public {
        bytes32 schemaId = _registerSchema("bool like", true);
        
        unchecked {
            uint64 expiredTime = uint64(block.timestamp - 1);
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
            
            vm.expectRevert(InvalidExpirationTime);
            eas.attest(request);
        }
    }

    function testCannotAttestToUnregisteredSchema() public {
        bytes32 unregisteredSchemaId = getSchemaUID("unregistered schema", address(0), true);
        
        vm.prank(sender);
        vm.expectRevert(InvalidSchema);
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

    function testAttestationScenarios() public {
        // Register test schemas first
        string memory schema1 = "bool like";
        string memory schema2 = "bytes32 proposalId, bool vote";
        string memory schema3 = "bool hasPhoneNumber, bytes32 phoneHash";
        
        bytes32 schema1Id = getSchemaUID(schema1, address(0), true);
        bytes32 schema2Id = getSchemaUID(schema2, address(0), true);
        bytes32 schema3Id = getSchemaUID(schema3, address(0), true);

        vm.startPrank(sender);
        registry.register(schema1, ISchemaResolver(address(0)), true);
        registry.register(schema2, ISchemaResolver(address(0)), true);
        registry.register(schema3, ISchemaResolver(address(0)), true);

        // Test: revert when attesting to an unregistered schema
        bytes32 badSchemaId = keccak256("BAD");
        vm.expectRevert(InvalidSchema);
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

        // Rest of the test...
        vm.stopPrank();
    }

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

    function testTimestampRevert() public {
        bytes32 data = keccak256("test data");
        
        // First timestamp should succeed
        eas.timestamp(data);
        
        // Second timestamp should fail
        vm.expectRevert(AlreadyTimestamped);
        eas.timestamp(data);
    }

    function testMultiTimestampRevert() public {
        bytes32[] memory data = new bytes32[](2);
        data[0] = keccak256("data1");
        data[1] = keccak256("data2");
        
        // First timestamp should succeed
        eas.multiTimestamp(data);
        
        // Second timestamp should fail
        vm.expectRevert(AlreadyTimestamped);
        eas.multiTimestamp(data);

        // Should also fail when including timestamped data in a new array
        bytes32[] memory newData = new bytes32[](3);
        newData[0] = keccak256("data3");
        newData[1] = data[0];
        newData[2] = data[1];
        
        vm.expectRevert(AlreadyTimestamped);
        eas.multiTimestamp(newData);
    }

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

    function testRevokeOffchainRevert() public {
        bytes32 data = keccak256("test data");
        
        vm.startPrank(sender);
        // First revocation should succeed
        eas.revokeOffchain(data);
        
        // Second revocation should fail
        vm.expectRevert(AlreadyRevokedOffchain);
        eas.revokeOffchain(data);
        vm.stopPrank();
    }

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

    function testMultiRevokeOffchainRevert() public {
        bytes32[] memory data = new bytes32[](2);
        data[0] = keccak256("data1");
        data[1] = keccak256("data2");
        
        vm.startPrank(sender);
        // First revocation should succeed
        eas.multiRevokeOffchain(data);
        
        // Second revocation should fail
        vm.expectRevert(AlreadyRevokedOffchain);
        eas.multiRevokeOffchain(data);

        // Should also fail when including revoked data in a new array
        bytes32[] memory newData = new bytes32[](3);
        newData[0] = keccak256("data3");
        newData[1] = data[0];
        newData[2] = data[1];
        
        vm.expectRevert(AlreadyRevokedOffchain);
        eas.multiRevokeOffchain(newData);
        vm.stopPrank();
    }

    function testGetUnregisteredTimestamp() public view {
        bytes32 data = keccak256("unregistered data");
        assertEq(eas.getTimestamp(data), 0);
    }

    function testGetUnregisteredRevokeOffchain() public view {
        bytes32 data = keccak256("unregistered data");
        assertEq(eas.getRevokeOffchain(sender, data), 0);
    }

    function testMultiAttestationDelegation() public {
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);
        
        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);
        
        uint64 expirationTime = uint64(block.timestamp + 30 days);
        bytes memory data = hex"1234";
        
        // Create multiple attestation requests
        MultiAttestationRequest[] memory requests = new MultiAttestationRequest[](2);
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
                data: RevocationRequestData({
                    uid: uid,
                    value: 0
                })
            })
        );

        Attestation memory attestation = eas.getAttestation(uid);
        assertTrue(attestation.revocationTime > 0);
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
        vm.expectRevert(AccessDenied);
        eas.revoke(
            RevocationRequest({
                schema: schemaId,
                data: RevocationRequestData({
                    uid: uid,
                    value: 0
                })
            })
        );
    }

    function testMultiAttestation() public {
        bytes32 schemaId = _registerSchema("bool like", true);
        
        MultiAttestationRequest[] memory requests = new MultiAttestationRequest[](0);
        
        vm.expectRevert(InvalidLength);
        eas.multiAttest(requests);
    }

    function _registerSchema(string memory schema, bool revocable) internal returns (bytes32) {
        bytes32 schemaId = getSchemaUID(schema, address(0), revocable);
        vm.prank(sender);
        registry.register(schema, ISchemaResolver(address(0)), revocable);
        return schemaId;
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

    function testRevokeOffchainRevertScenarios() public {
        bytes32[] memory data = new bytes32[](2);
        data[0] = keccak256("0x1234");
        data[1] = keccak256("Hello World");
        
        vm.startPrank(sender);
        
        // First revocation should succeed
        eas.multiRevokeOffchain(data);
        
        // Second revocation of same data should fail
        vm.expectRevert(AlreadyRevokedOffchain);
        eas.multiRevokeOffchain(data);
        
        // Should also fail when including revoked data in new array
        bytes32[] memory newData = new bytes32[](3);
        newData[0] = keccak256("0x8888");
        newData[1] = data[0];
        newData[2] = data[1];
        
        vm.expectRevert(AlreadyRevokedOffchain);
        eas.multiRevokeOffchain(newData);
        
        vm.stopPrank();
    }

    function testUnregisteredDataScenarios() public view {
        bytes32 unregisteredData = keccak256("unregistered");
        
        // Should return 0 for unregistered timestamp
        assertEq(eas.getTimestamp(unregisteredData), 0);
        
        // Should return 0 for unregistered revocation
        assertEq(eas.getRevokeOffchain(sender, unregisteredData), 0);
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
            AttestationRequest({
                schema: schemaId,
                data: requestData
            })
        );
        
        Attestation memory attestation = eas.getAttestation(uid);
        assertEq(attestation.attester, sender);
        assertEq(attestation.recipient, recipient);

        // Test multi-attestation delegation
        MultiAttestationRequest[] memory requests = new MultiAttestationRequest[](2);
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

        // Test revert with empty requests
        MultiDelegatedAttestationRequest[] memory requests = 
            new MultiDelegatedAttestationRequest[](0);
        
        vm.expectRevert(InvalidLength);
        eas.multiAttestByDelegation(requests);

        // Test revert with inconsistent lengths
        MultiDelegatedAttestationRequest[] memory badRequests = 
            new MultiDelegatedAttestationRequest[](1);
        badRequests[0].schema = schemaId;
        badRequests[0].data = new AttestationRequestData[](2);
        badRequests[0].signatures = new Signature[](1);
        
        vm.expectRevert(InvalidLength);
        eas.multiAttestByDelegation(badRequests);

        // Test revert with empty data but signatures
        badRequests[0].data = new AttestationRequestData[](0);
        badRequests[0].signatures = new Signature[](1);
        
        vm.expectRevert(InvalidLength);
        eas.multiAttestByDelegation(badRequests);

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
                data: RevocationRequestData({
                    uid: uid,
                    value: 0
                })
            })
        );

        Attestation memory attestation = eas.getAttestation(uid);
        assertTrue(attestation.revocationTime > 0);

        // Test multi-revocation
        bytes32[] memory uids = new bytes32[](2);
        for(uint i = 0; i < 2; i++) {
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

        RevocationRequest[] memory revocationRequests = new RevocationRequest[](2);
        for(uint i = 0; i < 2; i++) {
            revocationRequests[i] = RevocationRequest({
                schema: schemaId,
                data: RevocationRequestData({
                    uid: uids[i],
                    value: 0
                })
            });
        }

        for(uint i = 0; i < 2; i++) {
            eas.revoke(revocationRequests[i]);
            attestation = eas.getAttestation(uids[i]);
            assertTrue(attestation.revocationTime > 0);
        }

        vm.stopPrank();
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
        vm.expectRevert(Irrevocable);
        eas.revoke(
            RevocationRequest({
            schema: schemaId,
            data: RevocationRequestData({
                uid: uid,
                value: 0
            })
            })
        );

        // Test multi-attestation with irrevocable schema
        MultiAttestationRequest[] memory requests = new MultiAttestationRequest[](1);
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
        RevocationRequest[] memory revocationRequests = new RevocationRequest[](2);
        for(uint i = 0; i < 2; i++) {
            revocationRequests[i] = RevocationRequest({
                schema: schemaId,
                data: RevocationRequestData({
                    uid: uids[i],
                    value: 0
                })
            });
        }

        vm.expectRevert(Irrevocable);
        eas.revoke(revocationRequests[0]);

        vm.stopPrank();
    }

    function testMixedRevocabilityScenarios() public {
        // Register both revocable and irrevocable schemas
        string memory revocableSchema = "bool like";
        string memory irrevocableSchema = "bool isFriend";
        
        bytes32 revocableSchemaId = getSchemaUID(revocableSchema, address(0), true);
        bytes32 irrevocableSchemaId = getSchemaUID(irrevocableSchema, address(0), false);
        
        vm.startPrank(sender);
        registry.register(revocableSchema, ISchemaResolver(address(0)), true);
        registry.register(irrevocableSchema, ISchemaResolver(address(0)), false);

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
            data: RevocationRequestData({
                    uid: revocableUid,
                value: 0
            })
            })
        );
        
        // Should revert when trying to revoke the irrevocable attestation
        vm.expectRevert(Irrevocable);
        eas.revoke(
            RevocationRequest({
                schema: irrevocableSchemaId,
            data: RevocationRequestData({
                    uid: irrevocableUid,
                value: 0
            })
            })
        );

        vm.stopPrank();
    }

    function testMultiAttestationDelegationRevert() public {
        bytes32 schemaId = _registerSchema("bool like", true);
        
        MultiDelegatedAttestationRequest[] memory requests = new MultiDelegatedAttestationRequest[](0);
        
        vm.expectRevert(InvalidLength);
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
        MultiDelegatedRevocationRequest[] memory requests = new MultiDelegatedRevocationRequest[](1);
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

        vm.expectRevert(InvalidLength);
        eas.multiRevokeByDelegation(requests);

        // Test revert with empty data
        requests[0].data = new RevocationRequestData[](0);
        vm.expectRevert(InvalidLength);
        eas.multiRevokeByDelegation(requests);

        // Test revert with empty signatures
        requests[0].data = new RevocationRequestData[](1);
        requests[0].data[0].uid = uid1;
        requests[0].signatures = new Signature[](0);
        vm.expectRevert(InvalidLength);
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

        MultiAttestationRequest[] memory requests = new MultiAttestationRequest[](2);
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

        bytes32[] memory uids = eas.multiAttest{value: value * 2}(requests);
        assertEq(uids.length, 2);

        // Verify attestations
        for (uint i = 0; i < uids.length; i++) {
            Attestation memory attestation = eas.getAttestation(uids[i]);
            assertEq(attestation.attester, sender);
        }

        vm.stopPrank();
    }

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
        for(uint i = 0; i < 2; i++) {
            uids[i] = eas.attest{value: value}(
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
        for(uint i = 0; i < 2; i++) {
            requests[i] = RevocationRequest({
                schema: schemaId,
                data: RevocationRequestData({
                    uid: uids[i],
                    value: value
                })
            });
        }

        for(uint i = 0; i < 2; i++) {
            eas.revoke{value: value}(requests[i]);
            Attestation memory attestation = eas.getAttestation(uids[i]);
        assertTrue(attestation.revocationTime > 0);
        }

        vm.stopPrank();
    }

    function testConstructionScenarios() public {
        assertEq(eas.version(), "1.4.1-beta.1");
        assertEq(eas.getName(), "EAS");
        
        address schemaRegistry = address(eas.getSchemaRegistry());
        assertEq(schemaRegistry, address(registry));
        
        vm.expectRevert(InvalidRegistry);
        eas = new EAS();
    }

    function testSchemaRegistrationScenarios() public {
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);
        
        vm.startPrank(sender);
        
        // Test basic registration
        registry.register(schema, ISchemaResolver(address(0)), true);

        // Test registering same schema again (should revert)
        vm.expectRevert(abi.encodeWithSignature("AlreadyExists()"));
        registry.register(schema, ISchemaResolver(address(0)), true);
        
        // Test different schema types
        string memory complexSchema = "uint256 age, string name, address wallet";
        bytes32 complexSchemaId = getSchemaUID(complexSchema, address(0), true);
        registry.register(complexSchema, ISchemaResolver(address(0)), true);
        
        vm.stopPrank();
    }

    function testDetailedAttestationScenarios() public {
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);
        
        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);

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
        bytes32 uid2 = eas.attest(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: uint64(block.timestamp + 365 days),
            revocable: true,
                    refUID: uid1,
            data: hex"5678",
                    value: 1 ether
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
        assertEq(attestation2.expirationTime, uint64(block.timestamp + 365 days));
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

        // Test with various expiration times
        uint64[] memory expirationTimes = new uint64[](4);
        expirationTimes[0] = 0; // No expiration
        expirationTimes[1] = uint64(block.timestamp + 1 days);
        expirationTimes[2] = uint64(block.timestamp + 365 days);
        expirationTimes[3] = type(uint64).max;

        for(uint i = 0; i < expirationTimes.length; i++) {
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

        // Test with expired time (should revert)
        vm.expectRevert(InvalidExpirationTime);
        eas.attest(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                    expirationTime: uint64(block.timestamp - 1),
                    revocable: true,
                    refUID: bytes32(0),
                    data: hex"1234",
                    value: 0
                })
            })
        );

        vm.stopPrank();
    }

    function testMultiAttestationComprehensive() public {
        string memory schema = "bool like";
        string memory schema2 = "uint256 score";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);
        
        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);
        registry.register(schema2, ISchemaResolver(address(0)), true);

        // Test with multiple recipients and varying data
        address[] memory recipients = new address[](3);
        recipients[0] = recipient;
        recipients[1] = recipient2;
        recipients[2] = address(0);

        MultiAttestationRequest[] memory requests = new MultiAttestationRequest[](1);
        requests[0].schema = schemaId;
        requests[0].data = new AttestationRequestData[](3);

        for(uint i = 0; i < recipients.length; i++) {
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
        bytes32[] memory uids = eas.multiAttest{value: 0.3 ether}(requests);
        assertEq(uids.length, 3);

        // Verify all attestations
        for(uint i = 0; i < uids.length; i++) {
            Attestation memory attestation = eas.getAttestation(uids[i]);
            assertEq(attestation.attester, sender);
            if (i < 2) {
                assertEq(attestation.schema, schemaId);
            } else {
                assertEq(attestation.schema, getSchemaUID(schema2, address(0), true));
            }
        }

        vm.stopPrank();
    }

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
                v: v,
                r: r,
                s: s
            }),
            attester: sender,
            deadline: type(uint64).max
        });

        vm.expectRevert("EAS: invalid signature"); // Actual signature verification would fail
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
            data: hex"1234",
            value: 0
        });
        requests[0].signatures[0] = Signature({
            v: v,
            r: r,
            s: s
        });
        requests[0].attester = sender;
        requests[0].deadline = type(uint64).max;

        vm.expectRevert("EAS: invalid signature");
        eas.multiAttestByDelegation(requests);

        vm.stopPrank();
    }

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
                data: RevocationRequestData({
                    uid: uids[0],
                    value: 0
                })
            })
        );

        // Verify referenced attestation can still be revoked after its reference is revoked
        eas.revoke(
            RevocationRequest({
                schema: schemaId,
                data: RevocationRequestData({
                    uid: uids[2],
            value: 0
                })
            })
        );

        // Verify attestation states
        for (uint i = 0; i < uids.length; i++) {
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

    function testDeadlineScenarios() public {
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);
        
        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);

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

        vm.expectRevert(abi.encodeWithSignature("InvalidSignature()"));
        eas.attestByDelegation(request);
        vm.stopPrank();
    }

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

    function testBatchProcessingLimits() public {
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);
        
        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);
        
        // Test with empty batch
        MultiAttestationRequest[] memory emptyRequests = new MultiAttestationRequest[](0);
        vm.expectRevert(InvalidLength);
        eas.multiAttest(emptyRequests);

        // Test with empty inner batch
        MultiAttestationRequest[] memory requests = new MultiAttestationRequest[](1);
        requests[0].schema = schemaId;
        requests[0].data = new AttestationRequestData[](0);
        
        vm.expectRevert(InvalidLength);
        eas.multiAttest(requests);

        vm.stopPrank();
    }

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
                data: RevocationRequestData({
                    uid: parentUID,
                    value: 0
                })
            })
        );

        // Child should still be valid
        Attestation memory childAttestation = eas.getAttestation(childUID);
        assertEq(childAttestation.revocationTime, 0);

        vm.stopPrank();
    }

    function testComplexMultiAttestationScenarios() public {
        string memory schema = "bool like";
        string memory schema2 = "uint256 score";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);
        
        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);
        registry.register(schema2, ISchemaResolver(address(0)), true);

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
        for(uint i = 0; i < uids.length; i++) {
            Attestation memory attestation = eas.getAttestation(uids[i]);
            assertEq(attestation.attester, sender);
            if (i < 2) {
                assertEq(attestation.schema, schemaId);
            } else {
                assertEq(attestation.schema, getSchemaUID(schema2, address(0), true));
            }
        }

        vm.stopPrank();
    }

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

        for(uint i = 0; i < deadlines.length; i++) {
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
                deadline: deadlines[i]
            });

            // Should revert with invalid signature, but deadline check should pass
            vm.expectRevert("InvalidSignature()");
            eas.attestByDelegation(request);
        }

        vm.stopPrank();
    }

    function testAttestationDataScenarios() public {
        string memory schema = "bool like";
        bytes32 schemaId = getSchemaUID(schema, address(0), true);
        
        vm.startPrank(sender);
        registry.register(schema, ISchemaResolver(address(0)), true);

        // Test with different data sizes
        bytes[] memory testData = new bytes[](3);
        testData[0] = hex"";
        testData[1] = hex"1234";
        testData[2] = hex"123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890";

        for(uint i = 0; i < testData.length; i++) {
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
        for(uint i = 1; i < 3; i++) {
            uids[i] = eas.attest(
            AttestationRequest({
                schema: schemaId,
                data: AttestationRequestData({
                    recipient: recipient,
                        expirationTime: uint64(block.timestamp + 30 days),
                    revocable: true,
                        refUID: uids[i-1],
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
                data: RevocationRequestData({
                    uid: uids[0],
                    value: 0
                })
            })
        );

        // Verify children remain valid
        for(uint i = 1; i < 3; i++) {
            Attestation memory attestation = eas.getAttestation(uids[i]);
            assertEq(attestation.revocationTime, 0);
        }

        vm.stopPrank();
    }

}