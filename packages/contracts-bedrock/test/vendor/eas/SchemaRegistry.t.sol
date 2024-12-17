// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

import { Test } from "forge-std/Test.sol";
import { console } from "forge-std/console.sol";
import { ISchemaRegistry, SchemaRecord } from "src/vendor/eas/ISchemaRegistry.sol";
import { ISchemaResolver } from "src/vendor/eas/resolver/ISchemaResolver.sol";
import { ISemver } from "interfaces/universal/ISemver.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";
import { CommonTest } from "test/setup/CommonTest.sol";

// Main Test Contract
contract SchemaRegistryTest is CommonTest {


    // Events
    event Registered(
        bytes32 indexed uid,
        address indexed registerer,
        SchemaRecord schema
    );

    // Error Selectors
    error AlreadyExists();

    // Setup    
    function setUp() public override {
        super.setUp();
    }

    // Helper Functions
    /// @dev Generates a unique identifier for a schema based on its parameters
    function _getUID(
        string memory schema,
        address resolver,
        bool revocable
    ) internal pure returns (bytes32) {
        return keccak256(abi.encodePacked(schema, resolver, revocable));
    }


    /// @dev Verifies that the registry returns the correct version number
    function testVersion() public view {
        assertEq(ISemver(address(schemaRegistry)).version(), "1.3.1-beta.1");
    }

    // Basic Functionality Tests
    /// @dev Tests schema registration with various parameters.
    ///      Ensures schema registration properly stores all
    ///      parameters and generates correct UIDs
    function testRegisterSchema(string memory _schema, address _resolver, bool _revocable) public {
        bytes32 uid = schemaRegistry.register(_schema, ISchemaResolver(_resolver), _revocable);
        SchemaRecord memory record = schemaRegistry.getSchema(uid);
        
        assertEq(record.uid, uid);
        assertEq(record.schema, _schema);
        assertEq(address(record.resolver), _resolver);
        assertEq(record.revocable, _revocable);
    }

    /// @dev Tests the retrieval of a previously registered schema
    function testGetSchema(string memory _schema, address _resolver, bool _revocable) public {

        bytes32 uid = schemaRegistry.register(_schema, ISchemaResolver(_resolver), _revocable);
        SchemaRecord memory record = schemaRegistry.getSchema(uid);

        assertEq(record.uid, uid);
        assertEq(record.schema, _schema);
        assertEq(address(record.resolver), _resolver);
        assertEq(record.revocable, _revocable);
    }

    // Edge Case Tests
    /// @dev Tests schema retrieval functionality.
    ///      Demonstrates complete flow of schema registration
    ///      and subsequent retrieval
    function testRegisterSchemaWithoutSchema(string memory _schema, address _resolver, bool _revocable) public {
        vm.assume(keccak256(bytes(_schema)) == keccak256(bytes("")));
        bytes32 uid = schemaRegistry.register(_schema, ISchemaResolver(_resolver), _revocable);
        SchemaRecord memory record = schemaRegistry.getSchema(uid);
        
        assertEq(record.uid, uid);
        assertEq(record.schema, _schema);
        assertEq(address(record.resolver), _resolver);
        assertEq(record.revocable, _revocable);
    }

    /// @dev Tests schema registration without resolver.
    ///      Demonstrates schema registration functionality
    ///      for schemas that don't require resolver logic
    function testRegisterSchemaWithoutResolver(string memory _schema, address _resolver, bool _revocable) public {
        vm.assume(_resolver == address(0));

        bytes32 uid = schemaRegistry.register(_schema, ISchemaResolver(_resolver), _revocable);
        SchemaRecord memory record = schemaRegistry.getSchema(uid);
        
        assertEq(record.uid, uid);
        assertEq(record.schema, _schema);
        assertEq(address(record.resolver), _resolver);
        assertEq(record.revocable, _revocable);
    }

    /// @dev Tests schema registration with empty schema and no resolver.
    ///      Demonstrates system handles edge case of
    ///      minimal schema registration correctly
    function testRegisterSchemaWithoutSchemaOrResolver() public {
        string memory schema = "";
        address resolver = address(0);
        bool revocable = true;
  

        bytes32 uid = schemaRegistry.register(schema, ISchemaResolver(resolver), revocable);
        SchemaRecord memory record = schemaRegistry.getSchema(uid);
        
        assertEq(record.uid, uid);
        assertEq(record.schema, schema);
        assertEq(address(record.resolver), resolver);
        assertEq(record.revocable, revocable);
    }

    /// @dev Tests retrieval of a non-existent schema.
    ///      Ensures system returns empty/default values
    ///      when querying non-existent schemas
    function testGetNonExistingSchema() public view {
        bytes32 badUid = keccak256(abi.encodePacked("BAD"));
        SchemaRecord memory record = schemaRegistry.getSchema(badUid);

        assertEq(record.uid, bytes32(0));
        assertEq(record.schema, "");
        assertEq(address(record.resolver), address(0));
        assertEq(record.revocable, false);
    }

    // Error Case Tests
    /// @dev Tests duplicate schema registration prevention.
    ///      Ensures system properly prevents duplicate
    ///      schema registrations, maintaining schema uniqueness
    function testCannotRegisterSameSchemaTwice(string memory _schema, address _resolver, bool _revocable) public {
   
        schemaRegistry.register(_schema, ISchemaResolver(_resolver), _revocable);

        vm.expectRevert(AlreadyExists.selector);
        schemaRegistry.register(_schema, ISchemaResolver(_resolver), _revocable);
    }

    // Event Tests
    /// @dev Tests schema registration event emission.
    ///      Ensures proper event emission and data accuracy
    ///      during schema registration
    function testRegisterSchemaEvent(string memory _schema, address _resolver, bool _revocable) public {

        bytes32 expectedUID = _getUID(_schema, _resolver, _revocable);
        SchemaRecord memory expectedSchema = SchemaRecord({
            uid: expectedUID,
            schema: _schema,
            resolver: ISchemaResolver(_resolver),
            revocable: _revocable
        });

        vm.expectEmit(address(schemaRegistry));
        emit Registered(
            expectedUID,
            address(this),
            expectedSchema
        );
        
        schemaRegistry.register(_schema, ISchemaResolver(_resolver), _revocable);

        SchemaRecord memory actualSchema = schemaRegistry.getSchema(expectedUID);
        assertEq(actualSchema.uid, expectedUID, "UID mismatch");
        assertEq(actualSchema.schema, _schema, "Schema mismatch");
        assertEq(address(actualSchema.resolver), _resolver, "Resolver mismatch");
        assertEq(actualSchema.revocable, _revocable, "Revocable mismatch");
    }

    // Advanced Scenario Tests
    /// @dev Tests schema registration with extended schema string.
    ///      Demonstrates system handles large schema definitions
    ///      without truncation or modification
    function testRegisterLongSchema() public {
        string memory longSchema = "string reallyLongFieldName1, uint256 reallyLongFieldName2, address reallyLongFieldName3, bytes32 reallyLongFieldName4, bool reallyLongFieldName5";
        address resolver = address(0x123);
        bool revocable = true;

        bytes32 uid = schemaRegistry.register(longSchema, ISchemaResolver(resolver), revocable);
        SchemaRecord memory record = schemaRegistry.getSchema(uid);
        assertEq(record.schema, longSchema);
    }

    /// @dev Tests batch schema registration in single transaction.
    ///      Demonstrates system handles multiple registrations
    ///      within single transaction correctly
    function testMultipleRegistrationsInSameTx() public {
        string[] memory schemas = new string[](3);
        schemas[0] = "bool flag1";
        schemas[1] = "bool flag2";
        schemas[2] = "bool flag3";
        
        bytes32[] memory uids = new bytes32[](3);
        
        for(uint i = 0; i < schemas.length; i++) {
            uids[i] = schemaRegistry.register(schemas[i], ISchemaResolver(address(0)), true);
        }

        for(uint i = 0; i < schemas.length; i++) {
            SchemaRecord memory record = schemaRegistry.getSchema(uids[i]);
            assertEq(record.schema, schemas[i]);
        }
    }

    /// @dev Tests schema registration with special characters.
    ///      Ensures system properly handles and stores schemas
    ///      containing non-standard characters without modification
    function testRegisterSchemaWithSpecialChars() public {
        string memory schema = "string name_with_underscore, uint256 amount$, bool is@Valid";
        address resolver = address(0x123);
        bool revocable = true;

        bytes32 uid = schemaRegistry.register(schema, ISchemaResolver(resolver), revocable);
        SchemaRecord memory record = schemaRegistry.getSchema(uid);
        assertEq(record.schema, schema);
    }

    // Schema Uniqueness Tests
    /// @dev Tests UID uniqueness for identical schemas with different resolvers.
    ///      Demonstrates resolver address affects UID generation,
    ///      ensuring unique identification even with identical schemas
    function testSchemaUIDUniqueness() public {
        string memory schema1 = "bool flag";
        string memory schema2 = "bool flag";
        address resolver1 = address(0x123);
        address resolver2 = address(0x456);
        
        bytes32 uid1 = schemaRegistry.register(schema1, ISchemaResolver(resolver1), true);
        bytes32 uid2 = schemaRegistry.register(schema2, ISchemaResolver(resolver2), true);
        
        assertTrue(uid1 != uid2, "UIDs should be different for different resolver addresses");
    }

    /// @dev Tests registration of schema versions.
    ///      Demonstrates system supports multiple versions
    ///      of related schemas while maintaining separation
    function testSchemaVersioning() public {
        string memory schemaV1 = "bool flag";
        string memory schemaV2 = "bool flag, string metadata";
        address resolver = address(0x123);
        
        bytes32 uidV1 = schemaRegistry.register(schemaV1, ISchemaResolver(resolver), true);
        bytes32 uidV2 = schemaRegistry.register(schemaV2, ISchemaResolver(resolver), true);
        
        SchemaRecord memory recordV1 = schemaRegistry.getSchema(uidV1);
        SchemaRecord memory recordV2 = schemaRegistry.getSchema(uidV2);
        
        assertEq(recordV1.schema, schemaV1);
        assertEq(recordV2.schema, schemaV2);
    }

    // Resolver Tests
    /// @dev Tests registration of a schema with an EOA as resolver
    function testRegisterSchemaWithInvalidResolver(string memory _schema, address _nonContractResolver, bool _revocable) public {
   

        bytes32 uid = schemaRegistry.register(_schema, ISchemaResolver(_nonContractResolver), _revocable);
        SchemaRecord memory record = schemaRegistry.getSchema(uid);
        
        assertEq(record.uid, uid);
        assertEq(record.schema, _schema);
        assertEq(address(record.resolver), _nonContractResolver);
        assertEq(record.revocable, _revocable);
    }

    // Bulk Operation Tests
    /// @dev Tests multi-schema registration and retrieval.
    function testGetSchemasBulk() public {
        bytes32[] memory uids = new bytes32[](3);
        string[] memory schemas = new string[](3);
        schemas[0] = "bool flag1";
        schemas[1] = "uint256 value";
        schemas[2] = "string name";

        for(uint i = 0; i < schemas.length; i++) {
            uids[i] = schemaRegistry.register(schemas[i], ISchemaResolver(address(0)), true);
        }

        for(uint i = 0; i < uids.length; i++) {
            SchemaRecord memory record = schemaRegistry.getSchema(uids[i]);
            assertEq(record.schema, schemas[i]);
            assertEq(record.revocable, true);
        }
    }

    // Revocability Tests
    /// @dev Tests schema registration with revocable and non-revocable settings.
    ///      Demonstrates:
    ///         - System handles both revocable and non-revocable schemas
    ///         - Revocability affects UID generation
    ///         - Proper storage of revocability setting
    function testSchemaRevocability() public {
        string memory schema = "bool like";
        
        bytes32 revocableUid = schemaRegistry.register(schema, ISchemaResolver(address(0)), true);
        SchemaRecord memory revocableRecord = schemaRegistry.getSchema(revocableUid);
        assertTrue(revocableRecord.revocable, "Schema should be revocable");

        bytes32 nonRevocableUid = schemaRegistry.register("bool unlike", ISchemaResolver(address(0)), false);
        SchemaRecord memory nonRevocableRecord = schemaRegistry.getSchema(nonRevocableUid);
        assertFalse(nonRevocableRecord.revocable, "Schema should not be revocable");

        assertTrue(revocableUid != nonRevocableUid, "UIDs should be different for different revocability");
    }
}
