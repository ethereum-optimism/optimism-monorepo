// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

import { Test } from "forge-std/Test.sol";
import { console } from "forge-std/console.sol";
import { ISchemaRegistry, SchemaRecord } from "src/vendor/eas/ISchemaRegistry.sol";
import { ISchemaResolver } from "src/vendor/eas/resolver/ISchemaResolver.sol";
import { ISemver } from "src/universal/interfaces/ISemver.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";
import { CommonTest } from "test/setup/CommonTest.sol";

contract SchemaRegistryTest is CommonTest {
    // State variables
    ISchemaRegistry registry;

    // Events
    event Registered(
        bytes32 indexed uid,
        address indexed registerer,
        SchemaRecord schema
    );

    function setUp() public override {
        super.setUp();  // Call parent setUp first
        registry = ISchemaRegistry(Predeploys.SCHEMA_REGISTRY);  // Get registry from predeploy
    }

    // Helper functions
    function getUID(
        string memory schema,
        address resolver,
        bool revocable
    ) internal pure returns (bytes32) {
        return keccak256(abi.encodePacked(schema, resolver, revocable));
    }

    // Should be equal to the current version of SchemaRegistry.sol in this repository
    function testVersion() public view {
     assertEq(ISemver(address(registry)).version(), "1.3.1-beta.1");
    }

    // Basic functionality tests
    function testRegisterSchema() public {
        string memory schema = "bool like";
        address resolver = address(0x123);
        bool revocable = true;

        bytes32 uid = registry.register(schema, ISchemaResolver(resolver), revocable);
        SchemaRecord memory record = registry.getSchema(uid);
        
        assertEq(record.uid, uid);
        assertEq(record.schema, schema);
        assertEq(address(record.resolver), resolver);
        assertEq(record.revocable, revocable);
    }

    function testGetSchema() public {
        string memory schema = "bool isFriend";
        address resolver = address(0x456);
        bool revocable = true;

        bytes32 uid = registry.register(schema, ISchemaResolver(resolver), revocable);
        SchemaRecord memory record = registry.getSchema(uid);

        assertEq(record.uid, uid);
        assertEq(record.schema, schema);
        assertEq(address(record.resolver), resolver);
        assertEq(record.revocable, revocable);
    }

    // Edge cases
    function testRegisterSchemaWithoutSchema() public {
        string memory schema = "";
        address resolver = address(0x123);
        bool revocable = true;

        bytes32 uid = registry.register(schema, ISchemaResolver(resolver), revocable);
        SchemaRecord memory record = registry.getSchema(uid);
        
        assertEq(record.uid, uid);
        assertEq(record.schema, schema);
        assertEq(address(record.resolver), resolver);
        assertEq(record.revocable, revocable);
    }

    function testRegisterSchemaWithoutResolver() public {
        string memory schema = "bool hasPhoneNumber, bytes32 phoneHash";
        address resolver = address(0);
        bool revocable = true;

        bytes32 uid = registry.register(schema, ISchemaResolver(resolver), revocable);
        SchemaRecord memory record = registry.getSchema(uid);
        
        assertEq(record.uid, uid);
        assertEq(record.schema, schema);
        assertEq(address(record.resolver), resolver);
        assertEq(record.revocable, revocable);
    }

    function testRegisterSchemaWithoutSchemaOrResolver() public {
        string memory schema = "";
        address resolver = address(0);
        bool revocable = true;

        bytes32 uid = registry.register(schema, ISchemaResolver(resolver), revocable);
        SchemaRecord memory record = registry.getSchema(uid);
        
        assertEq(record.uid, uid);
        assertEq(record.schema, schema);
        assertEq(address(record.resolver), resolver);
        assertEq(record.revocable, revocable);
    }

    function testGetNonExistingSchema() public view {
        bytes32 badUid = keccak256(abi.encodePacked("BAD"));
        SchemaRecord memory record = registry.getSchema(badUid);

        assertEq(record.uid, bytes32(0));
        assertEq(record.schema, "");
        assertEq(address(record.resolver), address(0));
        assertEq(record.revocable, false);
    }

    // Error cases
    function testCannotRegisterSameSchemaTwice() public {
        string memory schema = "bool isFriend";
        address resolver = address(0);
        bool revocable = true;

        registry.register(schema, ISchemaResolver(resolver), revocable);

        vm.expectRevert(abi.encodeWithSignature("AlreadyExists()"));
        registry.register(schema, ISchemaResolver(resolver), revocable);
    }

    // Event tests
    function testRegisterSchemaEvent() public {
        string memory schema = "bool like";
        address resolver = address(0x123);
        bool revocable = true;

        bytes32 expectedUID = getUID(schema, resolver, revocable);
        SchemaRecord memory expectedSchema = SchemaRecord({
            uid: expectedUID,
            schema: schema,
            resolver: ISchemaResolver(resolver),
            revocable: revocable
        });

        vm.expectEmit(true, true, true, true, address(registry));
        emit Registered(
            expectedUID,
            address(this),
            expectedSchema
        );
        
        registry.register(schema, ISchemaResolver(resolver), revocable);

        SchemaRecord memory actualSchema = registry.getSchema(expectedUID);
        assertEq(actualSchema.uid, expectedUID, "UID mismatch");
        assertEq(actualSchema.schema, schema, "Schema mismatch");
        assertEq(address(actualSchema.resolver), resolver, "Resolver mismatch");
        assertEq(actualSchema.revocable, revocable, "Revocable mismatch");
    }

    // Gas tests
    function testRegisterSchemaGas() public {
        string memory schema = "bool like";
        address resolver = address(0x123);
        bool revocable = true;

        registry.register(schema, ISchemaResolver(resolver), revocable);
    }

    // Advanced scenarios
    function testRegisterLongSchema() public {
        string memory longSchema = "string reallyLongFieldName1, uint256 reallyLongFieldName2, address reallyLongFieldName3, bytes32 reallyLongFieldName4, bool reallyLongFieldName5";
        address resolver = address(0x123);
        bool revocable = true;

        bytes32 uid = registry.register(longSchema, ISchemaResolver(resolver), revocable);
        SchemaRecord memory record = registry.getSchema(uid);
        assertEq(record.schema, longSchema);
    }

    function testMultipleRegistrationsInSameTx() public {
        string[] memory schemas = new string[](3);
        schemas[0] = "bool flag1";
        schemas[1] = "bool flag2";
        schemas[2] = "bool flag3";
        
        bytes32[] memory uids = new bytes32[](3);
        
        for(uint i = 0; i < schemas.length; i++) {
            uids[i] = registry.register(schemas[i], ISchemaResolver(address(0)), true);
        }

        for(uint i = 0; i < schemas.length; i++) {
            SchemaRecord memory record = registry.getSchema(uids[i]);
            assertEq(record.schema, schemas[i]);
        }
    }

    function testRegisterSchemaWithSpecialChars() public {
        string memory schema = "string name_with_underscore, uint256 amount$, bool is@Valid";
        address resolver = address(0x123);
        bool revocable = true;

        bytes32 uid = registry.register(schema, ISchemaResolver(resolver), revocable);
        SchemaRecord memory record = registry.getSchema(uid);
        assertEq(record.schema, schema);
    }

    // Schema uniqueness and versioning
    function testSchemaUIDUniqueness() public {
        string memory schema1 = "bool flag";
        string memory schema2 = "bool flag";
        address resolver1 = address(0x123);
        address resolver2 = address(0x456);
        
        bytes32 uid1 = registry.register(schema1, ISchemaResolver(resolver1), true);
        bytes32 uid2 = registry.register(schema2, ISchemaResolver(resolver2), true);
        
        assertTrue(uid1 != uid2, "UIDs should be different for different resolver addresses");
    }

    function testSchemaVersioning() public {
        string memory schemaV1 = "bool flag";
        string memory schemaV2 = "bool flag, string metadata";
        address resolver = address(0x123);
        
        bytes32 uidV1 = registry.register(schemaV1, ISchemaResolver(resolver), true);
        bytes32 uidV2 = registry.register(schemaV2, ISchemaResolver(resolver), true);
        
        SchemaRecord memory recordV1 = registry.getSchema(uidV1);
        SchemaRecord memory recordV2 = registry.getSchema(uidV2);
        
        assertEq(recordV1.schema, schemaV1);
        assertEq(recordV2.schema, schemaV2);
    }

    // Resolver tests
    function testRegisterSchemaWithInvalidResolver() public {
        string memory schema = "bool like";
        address nonContractResolver = address(0x123);  // EOA address
        bool revocable = true;

        bytes32 uid = registry.register(schema, ISchemaResolver(nonContractResolver), revocable);
        SchemaRecord memory record = registry.getSchema(uid);
        
        assertEq(record.uid, uid);
        assertEq(record.schema, schema);
        assertEq(address(record.resolver), nonContractResolver);
        assertEq(record.revocable, revocable);
    }

    // Bulk operations
    function testGetSchemasBulk() public {
        bytes32[] memory uids = new bytes32[](3);
        string[] memory schemas = new string[](3);
        schemas[0] = "bool flag1";
        schemas[1] = "uint256 value";
        schemas[2] = "string name";

        for(uint i = 0; i < schemas.length; i++) {
            uids[i] = registry.register(schemas[i], ISchemaResolver(address(0)), true);
        }

        for(uint i = 0; i < uids.length; i++) {
            SchemaRecord memory record = registry.getSchema(uids[i]);
            assertEq(record.schema, schemas[i]);
            assertEq(record.revocable, true);
        }
    }

    // Revocability tests
    function testSchemaRevocability() public {
        string memory schema = "bool like";
        
        bytes32 revocableUid = registry.register(schema, ISchemaResolver(address(0)), true);
        SchemaRecord memory revocableRecord = registry.getSchema(revocableUid);
        assertTrue(revocableRecord.revocable, "Schema should be revocable");

        bytes32 nonRevocableUid = registry.register("bool unlike", ISchemaResolver(address(0)), false);
        SchemaRecord memory nonRevocableRecord = registry.getSchema(nonRevocableUid);
        assertFalse(nonRevocableRecord.revocable, "Schema should not be revocable");

        assertTrue(revocableUid != nonRevocableUid, "UIDs should be different for different revocability");
    }
}
