// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

import { Test } from "forge-std/Test.sol";
import { SchemaRegistry, SchemaRecord } from "src/vendor/eas/SchemaRegistry.sol";
import { ISchemaResolver } from "src/vendor/eas/resolver/ISchemaResolver.sol";

contract SchemaRegistryTest is Test {
    SchemaRegistry registry;

    function setUp() public {
        registry = new SchemaRegistry();
    }

    function testVersion() public view {
        assertEq(registry.version(), "1.3.1-beta.1");
    }

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

    function testCannotRegisterSameSchemaTwice() public {
        string memory schema = "bool isFriend";
        address resolver = address(0);
        bool revocable = true;

        registry.register(schema, ISchemaResolver(resolver), revocable);

        vm.expectRevert(abi.encodeWithSignature("AlreadyExists()"));
        registry.register(schema, ISchemaResolver(resolver), revocable);
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

    function testGetNonExistingSchema() public view {
        bytes32 badUid = keccak256(abi.encodePacked("BAD"));

        SchemaRecord memory record = registry.getSchema(badUid);

        assertEq(record.uid, bytes32(0));
        assertEq(record.schema, "");
        assertEq(address(record.resolver), address(0));
        assertEq(record.revocable, false);
    }
}
