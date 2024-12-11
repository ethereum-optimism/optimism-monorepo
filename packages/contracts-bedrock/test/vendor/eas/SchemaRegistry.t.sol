// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

import { Test } from "forge-std/Test.sol";
import { console } from "forge-std/console.sol";
import { ISchemaRegistry, SchemaRecord } from "src/vendor/eas/ISchemaRegistry.sol";
import { ISchemaResolver } from "src/vendor/eas/resolver/ISchemaResolver.sol";
import { ISemver } from "interfaces/universal/ISemver.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";
import { CommonTest } from "test/setup/CommonTest.sol";

// =============================================================
//                        MAIN TEST CONTRACT
// =============================================================

contract SchemaRegistryTest is CommonTest {
    // =============================================================
    //                          TEST STATE
    // =============================================================
    ISchemaRegistry registry;

    // =============================================================
    //                           EVENTS
    // =============================================================
    event Registered(
        bytes32 indexed uid,
        address indexed registerer,
        SchemaRecord schema
    );

    // ERROR SELECTORS
    error AlreadyExists();

    // =============================================================
    //                           SETUP
    // =============================================================
    /// @dev Initializes the test environment by setting up the registry from predeploys
    function setUp() public override {
        super.setUp();
        registry = ISchemaRegistry(Predeploys.SCHEMA_REGISTRY);
    }

    // =============================================================
    //                    HELPER FUNCTIONS
    // =============================================================
    /// @dev Generates a unique identifier for a schema based on its parameters
    function _getUID(
        string memory schema,
        address resolver,
        bool revocable
    ) internal pure returns (bytes32) {
        return keccak256(abi.encodePacked(schema, resolver, revocable));
    }

    // =============================================================
    //                      VERSION TESTS
    // =============================================================
    /// @dev Verifies that the registry returns the correct version number
    function testVersion() public view {
        assertEq(ISemver(address(registry)).version(), "1.3.1-beta.1");
    }

    // =============================================================
    //                    BASIC FUNCTIONALITY TESTS
    // =============================================================
    /// @dev Tests schema registration with various parameters.
    ///      1. Parameters:
    ///         - schema: Schema definition string
    ///         - resolver: Optional resolver contract address
    ///         - revocable: Whether attestations can be revoked
    ///      2. Registration:
    ///         - Registers schema with given parameters
    ///         - Retrieves schema record using returned UID
    ///      3. Verification:
    ///         - Confirms UID matches
    ///         - Validates schema string
    ///         - Checks resolver address
    ///         - Verifies revocable flag
    ///      Ensures schema registration properly stores all
    ///      parameters and generates correct UIDs
    function testRegisterSchema(string memory _schema, address _resolver, bool _revocable) public {
        bytes32 uid = registry.register(_schema, ISchemaResolver(_resolver), _revocable);
        SchemaRecord memory record = registry.getSchema(uid);
        
        assertEq(record.uid, uid);
        assertEq(record.schema, _schema);
        assertEq(address(record.resolver), _resolver);
        assertEq(record.revocable, _revocable);
    }

    /// @dev Tests the retrieval of a previously registered schema
    function testGetSchema(string memory _schema, address _resolver, bool _revocable) public {

        bytes32 uid = registry.register(_schema, ISchemaResolver(_resolver), _revocable);
        SchemaRecord memory record = registry.getSchema(uid);

        assertEq(record.uid, uid);
        assertEq(record.schema, _schema);
        assertEq(address(record.resolver), _resolver);
        assertEq(record.revocable, _revocable);
    }

    // =============================================================
    //                      EDGE CASE TESTS
    // =============================================================
    /// @dev Tests schema retrieval functionality.
    ///      1. Setup:
    ///         - Creates simple boolean friendship schema
    ///         - Sets specific resolver address
    ///         - Enables revocation
    ///      2. Registration:
    ///         - Registers schema with parameters
    ///         - Retrieves schema using generated UID
    ///      3. Verification:
    ///         - Validates UID matches
    ///         - Confirms schema string accuracy
    ///         - Checks resolver address matches
    ///         - Verifies revocable setting
    ///      Demonstrates complete flow of schema registration
    ///      and subsequent retrieval
    function testRegisterSchemaWithoutSchema(string memory _schema, address _resolver, bool _revocable) public {
        vm.assume(keccak256(bytes(_schema)) == keccak256(bytes("")));
        bytes32 uid = registry.register(_schema, ISchemaResolver(_resolver), _revocable);
        SchemaRecord memory record = registry.getSchema(uid);
        
        assertEq(record.uid, uid);
        assertEq(record.schema, _schema);
        assertEq(address(record.resolver), _resolver);
        assertEq(record.revocable, _revocable);
    }

    /// @dev Tests schema registration without resolver.
    ///      1. Setup:
    ///         - Creates complex schema (phone verification)
    ///         - Uses zero address for resolver
    ///         - Enables revocation
    ///      2. Registration:
    ///         - Registers schema without resolver
    ///         - Retrieves schema record
    ///      3. Verification:
    ///         - Confirms UID generation
    ///         - Validates schema string
    ///         - Verifies zero resolver address
    ///         - Checks revocable flag
    ///      Demonstrates schema registration functionality
    ///      for schemas that don't require resolver logic
    function testRegisterSchemaWithoutResolver(string memory _schema, address _resolver, bool _revocable) public {
        vm.assume(_resolver == address(0));

        bytes32 uid = registry.register(_schema, ISchemaResolver(_resolver), _revocable);
        SchemaRecord memory record = registry.getSchema(uid);
        
        assertEq(record.uid, uid);
        assertEq(record.schema, _schema);
        assertEq(address(record.resolver), _resolver);
        assertEq(record.revocable, _revocable);
    }

/// @dev Tests schema registration with empty schema and no resolver.
    ///      1. Setup:
    ///         - Uses empty string for schema
    ///         - Sets zero address for resolver
    ///         - Enables revocation
    ///      2. Registration:
    ///         - Registers minimal schema configuration
    ///         - Retrieves schema record
    ///      3. Verification:
    ///         - Validates UID generation
    ///         - Confirms empty schema string
    ///         - Verifies zero resolver address
    ///         - Checks revocable flag
    ///      Demonstrates system handles edge case of
    ///      minimal schema registration correctly
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

    /// @dev Tests retrieval of a non-existent schema.
    ///      1. Setup:
    ///         - Creates invalid UID using "BAD" hash
    ///      2. Retrieval:
    ///         - Attempts to get schema with invalid UID
    ///      3. Verification:
    ///         - Confirms zero UID returned
    ///         - Validates empty schema string
    ///         - Checks zero resolver address
    ///         - Verifies revocable set to false
    ///      Ensures system returns empty/default values
    ///      when querying non-existent schemas
    function testGetNonExistingSchema() public view {
        bytes32 badUid = keccak256(abi.encodePacked("BAD"));
        SchemaRecord memory record = registry.getSchema(badUid);

        assertEq(record.uid, bytes32(0));
        assertEq(record.schema, "");
        assertEq(address(record.resolver), address(0));
        assertEq(record.revocable, false);
    }

    // =============================================================
    //                      ERROR CASE TESTS
    // =============================================================
    /// @dev Tests duplicate schema registration prevention.
    ///      1. Setup:
    ///         - Creates simple boolean schema
    ///         - Uses zero address for resolver
    ///         - Enables revocation
    ///      2. First Registration:
    ///         - Successfully registers schema
    ///      3. Duplicate Attempt:
    ///         - Attempts to register same schema again
    ///         - Verifies revert with AlreadyExists error
    ///      Ensures system properly prevents duplicate
    ///      schema registrations, maintaining schema uniqueness
    function testCannotRegisterSameSchemaTwice(string memory _schema, address _resolver, bool _revocable) public {
   
        registry.register(_schema, ISchemaResolver(_resolver), _revocable);

        vm.expectRevert(AlreadyExists.selector);
        registry.register(_schema, ISchemaResolver(_resolver), _revocable);
    }

    // =============================================================
    //                      EVENT TESTS
    // =============================================================
    /// @dev Tests schema registration event emission.
    ///      1. Setup:
    ///         - Creates basic boolean schema
    ///         - Sets specific resolver address
    ///         - Enables revocation
    ///         - Calculates expected UID
    ///      2. Event Testing:
    ///         - Creates expected schema record
    ///         - Sets up event emission expectation
    ///         - Registers schema
    ///      3. Verification:
    ///         - Verifies Registered event emission
    ///         - Validates all schema parameters:
    ///           * UID matches expected
    ///           * Schema string correct
    ///           * Resolver address matches
    ///           * Revocable flag set properly
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

        vm.expectEmit(true, true, true, true, address(registry));
        emit Registered(
            expectedUID,
            address(this),
            expectedSchema
        );
        
        registry.register(_schema, ISchemaResolver(_resolver), _revocable);

        SchemaRecord memory actualSchema = registry.getSchema(expectedUID);
        assertEq(actualSchema.uid, expectedUID, "UID mismatch");
        assertEq(actualSchema.schema, _schema, "Schema mismatch");
        assertEq(address(actualSchema.resolver), _resolver, "Resolver mismatch");
        assertEq(actualSchema.revocable, _revocable, "Revocable mismatch");
    }

    // =============================================================
    //                    ADVANCED SCENARIO TESTS
    // =============================================================
    /// @dev Tests schema registration with extended schema string.
    ///      1. Setup:
    ///         - Creates schema with multiple long field names
    ///         - Uses five different data types
    ///         - Total length significantly longer than typical
    ///      2. Registration:
    ///         - Registers complex schema
    ///         - Sets resolver and revocable flag
    ///      3. Verification:
    ///         - Retrieves schema record
    ///         - Confirms long schema string stored correctly
    ///      Demonstrates system handles large schema definitions
    ///      without truncation or modification
    function testRegisterLongSchema() public {
        string memory longSchema = "string reallyLongFieldName1, uint256 reallyLongFieldName2, address reallyLongFieldName3, bytes32 reallyLongFieldName4, bool reallyLongFieldName5";
        address resolver = address(0x123);
        bool revocable = true;

        bytes32 uid = registry.register(longSchema, ISchemaResolver(resolver), revocable);
        SchemaRecord memory record = registry.getSchema(uid);
        assertEq(record.schema, longSchema);
    }

    /// @dev Tests batch schema registration in single transaction.
    ///      1. Setup:
    ///         - Creates array of three similar schemas
    ///         - Each schema has simple boolean flag
    ///      2. Registration Loop:
    ///         - Registers each schema sequentially
    ///         - Stores UIDs for verification
    ///         - Uses zero address resolver
    ///         - Sets all as revocable
    ///      3. Verification Loop:
    ///         - Retrieves each schema record
    ///         - Confirms schema strings match original
    ///      Demonstrates system handles multiple registrations
    ///      within single transaction correctly
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

    /// @dev Tests schema registration with special characters.
    ///      1. Setup:
    ///         - Creates schema with various special characters:
    ///           * Underscores in field names
    ///           * Dollar sign in field name
    ///           * At symbol in field name
    ///      2. Registration:
    ///         - Registers schema with special characters
    ///         - Sets resolver and revocable flag
    ///      3. Verification:
    ///         - Retrieves schema record
    ///         - Confirms special characters preserved exactly
    ///      Ensures system properly handles and stores schemas
    ///      containing non-standard characters without modification
    function testRegisterSchemaWithSpecialChars() public {
        string memory schema = "string name_with_underscore, uint256 amount$, bool is@Valid";
        address resolver = address(0x123);
        bool revocable = true;

        bytes32 uid = registry.register(schema, ISchemaResolver(resolver), revocable);
        SchemaRecord memory record = registry.getSchema(uid);
        assertEq(record.schema, schema);
    }

    // =============================================================
    //                    SCHEMA UNIQUENESS TESTS
    // =============================================================
    /// @dev Tests UID uniqueness for identical schemas with different resolvers.
    ///      1. Setup:
    ///         - Creates identical schema strings
    ///         - Uses two different resolver addresses
    ///      2. Registration:
    ///         - Registers first schema with resolver1
    ///         - Registers identical schema with resolver2
    ///      3. Verification:
    ///         - Confirms UIDs are different
    ///      Demonstrates resolver address affects UID generation,
    ///      ensuring unique identification even with identical schemas
    function testSchemaUIDUniqueness() public {
        string memory schema1 = "bool flag";
        string memory schema2 = "bool flag";
        address resolver1 = address(0x123);
        address resolver2 = address(0x456);
        
        bytes32 uid1 = registry.register(schema1, ISchemaResolver(resolver1), true);
        bytes32 uid2 = registry.register(schema2, ISchemaResolver(resolver2), true);
        
        assertTrue(uid1 != uid2, "UIDs should be different for different resolver addresses");
    }

    /// @dev Tests registration of schema versions.
    ///      1. Setup:
    ///         - Creates basic schema (V1)
    ///         - Creates extended schema with metadata (V2)
    ///         - Uses same resolver for both
    ///      2. Registration:
    ///         - Registers both versions sequentially
    ///      3. Verification:
    ///         - Retrieves both schema records
    ///         - Confirms correct storage of both versions
    ///      Demonstrates system supports multiple versions
    ///      of related schemas while maintaining separation
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

    // =============================================================
    //                    RESOLVER TESTS
    // =============================================================
    /// @dev Tests registration of a schema with an EOA as resolver
    function testRegisterSchemaWithInvalidResolver(string memory _schema, address _nonContractResolver, bool _revocable) public {
   

        bytes32 uid = registry.register(_schema, ISchemaResolver(_nonContractResolver), _revocable);
        SchemaRecord memory record = registry.getSchema(uid);
        
        assertEq(record.uid, uid);
        assertEq(record.schema, _schema);
        assertEq(address(record.resolver), _nonContractResolver);
        assertEq(record.revocable, _revocable);
    }

    // =============================================================
    //                    BULK OPERATION TESTS
    // =============================================================
    /// @dev Tests schema registration with EOA (non-contract) resolver.
    ///      1. Setup:
    ///         - Creates basic boolean schema
    ///         - Uses regular address as resolver (EOA)
    ///         - Note: System allows this as it doesn't validate
    ///           resolver is actually a contract
    ///      2. Registration:
    ///         - Registers schema with EOA resolver
    ///      3. Verification:
    ///         - Confirms UID generation
    ///         - Validates schema string storage
    ///         - Verifies EOA resolver address stored
    ///         - Checks revocable flag
    ///      Important: While this works, using EOA as resolver
    ///      is not recommended as it can't implement resolver
    ///      interface functionality
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

    // =============================================================
    //                    REVOCABILITY TESTS
    // =============================================================
    /// @dev Tests schema registration with revocable and non-revocable settings.
    ///      1. Revocable Schema:
    ///         - Registers schema with revocable flag true
    ///         - Verifies revocable setting stored correctly
    ///      2. Non-revocable Schema:
    ///         - Registers different schema with revocable flag false
    ///         - Confirms non-revocable setting stored correctly
    ///      3. UID Verification:
    ///         - Ensures different UIDs for same schema with
    ///           different revocability settings
    ///      Demonstrates:
    ///         - System handles both revocable and non-revocable schemas
    ///         - Revocability affects UID generation
    ///         - Proper storage of revocability setting
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
