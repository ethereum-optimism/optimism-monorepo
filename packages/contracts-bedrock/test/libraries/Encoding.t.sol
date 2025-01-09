// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing utilities
import { CommonTest } from "test/setup/CommonTest.sol";

// Libraries
import { Types } from "src/libraries/Types.sol";
import { LegacyCrossDomainUtils } from "src/libraries/LegacyCrossDomainUtils.sol";

// Target contract
import { Encoding } from "src/libraries/Encoding.sol";

contract Encoding_Test is CommonTest {
    /// @dev Tests encoding and decoding a nonce and version.
    function testFuzz_nonceVersioning_succeeds(uint240 _nonce, uint16 _version) external pure {
        (uint240 nonce, uint16 version) = Encoding.decodeVersionedNonce(Encoding.encodeVersionedNonce(_nonce, _version));
        assertEq(version, _version);
        assertEq(nonce, _nonce);
    }

    /// @dev Tests decoding a versioned nonce.
    function testDiff_decodeVersionedNonce_succeeds(uint240 _nonce, uint16 _version) external {
        uint256 nonce = uint256(Encoding.encodeVersionedNonce(_nonce, _version));
        (uint256 decodedNonce, uint256 decodedVersion) = ffi.decodeVersionedNonce(nonce);

        assertEq(_version, uint16(decodedVersion));

        assertEq(_nonce, uint240(decodedNonce));
    }

    /// @dev Tests cross domain message encoding.
    function testDiff_encodeCrossDomainMessage_succeeds(
        uint240 _nonce,
        uint8 _version,
        address _sender,
        address _target,
        uint256 _value,
        uint256 _gasLimit,
        bytes memory _data
    )
        external
    {
        uint8 version = _version % 2;
        uint256 nonce = Encoding.encodeVersionedNonce(_nonce, version);

        bytes memory encoding = Encoding.encodeCrossDomainMessage(nonce, _sender, _target, _value, _gasLimit, _data);

        bytes memory _encoding = ffi.encodeCrossDomainMessage(nonce, _sender, _target, _value, _gasLimit, _data);

        assertEq(encoding, _encoding);
    }

    /// @dev Tests legacy cross domain message encoding.
    function testFuzz_encodeCrossDomainMessageV0_matchesLegacy_succeeds(
        uint240 _nonce,
        address _sender,
        address _target,
        bytes memory _data
    )
        external
        pure
    {
        uint8 version = 0;
        uint256 nonce = Encoding.encodeVersionedNonce(_nonce, version);

        bytes memory legacyEncoding = LegacyCrossDomainUtils.encodeXDomainCalldata(_target, _sender, _data, nonce);

        bytes memory bedrockEncoding = Encoding.encodeCrossDomainMessageV0(_target, _sender, _data, nonce);

        assertEq(legacyEncoding, bedrockEncoding);
    }

    /// @dev Tests that encodeCrossDomainMessage reverts if version is greater than 1.
    function testFuzz_encodeCrossDomainMessage_versionGreaterThanOne_reverts(uint256 nonce) external {
        // nonce >> 240 must be greater than 1
        uint256 minInvalidNonce = (uint256(type(uint240).max) + 1) * 2;
        nonce = bound(nonce, minInvalidNonce, type(uint256).max);

        EncodingContract encoding = new EncodingContract();

        vm.expectRevert(bytes("Encoding: unknown cross domain message version"));
        encoding.encodeCrossDomainMessage(nonce, address(this), address(this), 1, 100, hex"");
    }

    /// @dev Tests deposit transaction encoding.
    function testDiff_encodeDepositTransaction_succeeds(
        address _from,
        address _to,
        uint256 _mint,
        uint256 _value,
        uint64 _gas,
        bool isCreate,
        bytes memory _data,
        uint64 _logIndex
    )
        external
    {
        Types.UserDepositTransaction memory t = Types.UserDepositTransaction(
            _from, _to, isCreate, _value, _mint, _gas, _data, bytes32(uint256(0)), _logIndex
        );

        bytes memory txn = Encoding.encodeDepositTransaction(t);
        bytes memory _txn = ffi.encodeDepositTransaction(t);

        assertEq(txn, _txn);
    }

    /// @notice Test that decoding and re-encoding preserves all components
    function testFuzz_encodeDecode_protocolVersion_succeeds(
        bytes8 _build,
        uint32 _major,
        uint32 _minor,
        uint32 _patch,
        uint32 _preRelease
    )
        public
        pure
    {
        bytes32 encoded = Encoding.encodeProtocolVersion(_build, _major, _minor, _patch, _preRelease);
        string memory decoded = Encoding.decodeProtocolVersion(encoded);
        // Base version string
        string memory expected = string(
            abi.encodePacked(
                "v", Encoding.uint2str(_major), ".", Encoding.uint2str(_minor), ".", Encoding.uint2str(_patch)
            )
        );
        // Add prerelease if non-zero
        if (_preRelease != 0) {
            expected = string(abi.encodePacked(expected, "-", Encoding.uint2str(_preRelease)));
        }
        // Add build if not all zeros
        if (uint64(_build) != 0) {
            expected = string(abi.encodePacked(expected, "+0x", Encoding.bytes2hex(_build)));
        }
        assertEq(decoded, expected);
    }

    /// @notice Test specific known values for verification
    function test_protocolVersion_specific_succeeds() public pure {
        bytes32 encoded = Encoding.encodeProtocolVersion(bytes8(hex"0123456789abcdef"), 1, 2, 3, 4);
        string memory decoded = Encoding.decodeProtocolVersion(encoded);
        string memory expected = "v1.2.3-4+0x0123456789abcdef";
        assertEq(decoded, expected);
    }

    /// @notice Test encoding with no prerelease version
    function test_protocolVersion_noPrerelease_succeeds() public pure {
        bytes32 encoded = Encoding.encodeProtocolVersion(bytes8(hex"0123456789abcdef"), 1, 2, 3, 0);
        string memory decoded = Encoding.decodeProtocolVersion(encoded);
        string memory expected = "v1.2.3+0x0123456789abcdef";
        assertEq(decoded, expected);
    }

    function test_protocolVersion_allZeros_succeeds() public pure {
        bytes32 encoded = Encoding.encodeProtocolVersion(bytes8(0), 0, 0, 0, 0);
        string memory decoded = Encoding.decodeProtocolVersion(encoded);
        string memory expected = "v0.0.0"; // No prerelease or build
        assertEq(decoded, expected);
    }

    /// @notice Test specific known values for verification with Go implementation
    function testDiff_encodeProtocolVersion_matchesGo_succeeds() public {
        bytes32 encoded = Encoding.encodeProtocolVersion(
            bytes8(hex"0123456789abcdef"), // build
            1, // major
            2, // minor
            3, // patch
            4 // prerelease
        );
        emit log_bytes32(encoded);

        bytes memory goEncoded = ffi.encodeProtocolVersion(
            hex"0123456789abcdef", // build
            1, // major
            2, // minor
            3, // patch
            4 // prerelease
        );
        emit log_bytes(goEncoded);

        assertEq(encoded, bytes32(goEncoded));
    }

    /// @notice Test that Go and Solidity implementations match for decoding
    function testDiff_decodeProtocolVersion_matchesGo_succeeds() public {
        bytes32 version = bytes32(hex"00000000000000000123456789abcdef00000001000000020000000300000004");

        string memory solString = Encoding.decodeProtocolVersion(version);
        string memory goString = ffi.decodeProtocolVersion(version);

        assertEq(solString, goString);
    }

    /// @notice Test maximum values for all fields
    function test_protocolVersion_maxValues_succeeds() public pure {
        bytes32 encoded = Encoding.encodeProtocolVersion(
            bytes8(hex"ffffffffffffffff"),
            type(uint32).max, // 4294967295
            type(uint32).max,
            type(uint32).max,
            type(uint32).max
        );
        string memory decoded = Encoding.decodeProtocolVersion(encoded);
        string memory expected = "v4294967295.4294967295.4294967295-4294967295+0xffffffffffffffff";
        assertEq(decoded, expected);
    }

    /// @notice Test minimal non-zero build ID
    function test_protocolVersion_minimalBuild_succeeds() public pure {
        bytes32 encoded = Encoding.encodeProtocolVersion(bytes8(hex"0000000000000001"), 1, 0, 0, 0);
        string memory decoded = Encoding.decodeProtocolVersion(encoded);
        string memory expected = "v1.0.0+0x0000000000000001";
        assertEq(decoded, expected);
    }

    /// @notice Test prerelease without build
    function test_protocolVersion_onlyPrerelease_succeeds() public pure {
        bytes32 encoded = Encoding.encodeProtocolVersion(bytes8(0), 1, 0, 0, 1);
        string memory decoded = Encoding.decodeProtocolVersion(encoded);
        string memory expected = "v1.0.0-1";
        assertEq(decoded, expected);
    }
}

contract EncodingContract {
    function encodeCrossDomainMessage(
        uint256 nonce,
        address sender,
        address target,
        uint256 value,
        uint256 gasLimit,
        bytes memory data
    )
        external
        pure
        returns (bytes memory)
    {
        return Encoding.encodeCrossDomainMessage(nonce, sender, target, value, gasLimit, data);
    }
}
