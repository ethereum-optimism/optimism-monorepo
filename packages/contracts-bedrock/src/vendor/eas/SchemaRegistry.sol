// SPDX-License-Identifier: MIT
pragma solidity 0.8.19;

import { ISemver } from "interfaces/universal/ISemver.sol";
import { ISchemaResolver } from "src/vendor/eas/resolver/ISchemaResolver.sol";
import { EMPTY_UID, MAX_GAP } from "src/vendor/eas/Common.sol";
import { ISchemaRegistry, SchemaRecord } from "src/vendor/eas/ISchemaRegistry.sol";

/// @custom:proxied true
/// @custom:predeploy 0x4200000000000000000000000000000000000020
/// @title SchemaRegistry
/// @notice The global attestation schemas for the Ethereum Attestation Service protocol.
contract SchemaRegistry is ISchemaRegistry, ISemver {
    error AlreadyExists();

    // The global mapping between schema records and their IDs.
    mapping(bytes32 uid => SchemaRecord schemaRecord) private _registry;

    // Upgrade forward-compatibility storage gap
    uint256[MAX_GAP - 1] private __gap;

    /// @notice Semantic version.
    /// @custom:semver 1.3.1-beta.2
    string public constant version = "1.3.1-beta.2";

    /// @inheritdoc ISchemaRegistry
    function register(string calldata schema, ISchemaResolver resolver, bool revocable) external returns (bytes32) {
        SchemaRecord memory schemaRecord =
            SchemaRecord({ uid: EMPTY_UID, schema: schema, resolver: resolver, revocable: revocable });

        bytes32 uid = _getUID(schemaRecord);
        if (_registry[uid].uid != EMPTY_UID) {
            revert AlreadyExists();
        }

        schemaRecord.uid = uid;
        _registry[uid] = schemaRecord;

        emit Registered(uid, msg.sender, schemaRecord);

        return uid;
    }

    /// @inheritdoc ISchemaRegistry
    function getSchema(bytes32 uid) external view returns (SchemaRecord memory) {
        return _registry[uid];
    }

    /// @dev Calculates a UID for a given schema.
    /// @param schemaRecord The input schema.
    /// @return schema UID.
    function _getUID(SchemaRecord memory schemaRecord) private pure returns (bytes32) {
        return keccak256(abi.encodePacked(schemaRecord.schema, schemaRecord.resolver, schemaRecord.revocable));
    }
}
