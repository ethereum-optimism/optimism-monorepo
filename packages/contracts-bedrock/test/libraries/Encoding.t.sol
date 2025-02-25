// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { CommonTest } from "test/setup/CommonTest.sol";

// Libraries
import { Encoding } from "src/libraries/Encoding.sol";
import { Types } from "src/libraries/Types.sol";
import { LegacyCrossDomainUtils } from "src/libraries/LegacyCrossDomainUtils.sol";

contract Encoding_TestInit is CommonTest {
    Encoding_Harness encoding;

    function setUp() public override {
        super.setUp();
        encoding = new Encoding_Harness();
    }
}

contract Encoding_Test is Encoding_TestInit {
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
}

contract Encoding_encodeSuperRootProof_Test is Encoding_TestInit {
    /// @notice Tests successful encoding of a valid super root proof
    /// @param _timestamp The timestamp of the super root proof
    /// @param _length The number of output roots in the super root proof
    /// @param _seed The seed used to generate the output roots
    function testFuzz_encodeSuperRootProof_succeeds(uint64 _timestamp, uint256 _length, uint256 _seed) external pure {
        // Ensure at least 1 element and cap at a reasonable maximum to avoid gas issues
        _length = uint256(bound(_length, 1, 50));

        // Create output roots array
        Types.OutputRootWithChainId[] memory outputRoots = new Types.OutputRootWithChainId[](_length);

        // Generate deterministic chain IDs and roots based on the seed
        for (uint256 i = 0; i < _length; i++) {
            // Use different derivations of the seed for each value
            uint256 chainId = uint256(keccak256(abi.encode(_seed, "chainId", i)));
            bytes32 root = keccak256(abi.encode(_seed, "root", i));

            outputRoots[i] = Types.OutputRootWithChainId({ chainId: chainId, root: root });
        }

        // Create the super root proof
        Types.SuperRootProof memory proof =
            Types.SuperRootProof({ version: 0x01, timestamp: _timestamp, outputRoots: outputRoots });

        // Encode the proof
        bytes memory encoded = Encoding.encodeSuperRootProof(proof);

        // Verify encoding structure
        assertEq(encoded[0], bytes1(0x01), "Version byte should be 0x01");

        // Verify timestamp (bytes 1-8)
        bytes8 encodedTimestamp;
        for (uint256 i = 0; i < 8; i++) {
            encodedTimestamp |= bytes8(encoded[i + 1]) >> (i * 8);
        }
        assertEq(uint64(encodedTimestamp), _timestamp, "Timestamp should match");

        // Verify each chain ID and root is encoded correctly
        uint256 offset = 9; // 1 byte version + 8 bytes timestamp
        for (uint256 i = 0; i < _length; i++) {
            // Extract chain ID (32 bytes)
            uint256 encodedChainId;
            assembly {
                // Load 32 bytes from encoded at position offset
                encodedChainId := mload(add(add(encoded, 32), offset))
            }
            assertEq(encodedChainId, outputRoots[i].chainId, "Chain ID should match");
            offset += 32;

            // Extract root (32 bytes)
            bytes32 encodedRoot;
            assembly {
                // Load 32 bytes from encoded at position offset
                encodedRoot := mload(add(add(encoded, 32), offset))
            }
            assertEq(encodedRoot, outputRoots[i].root, "Root should match");
            offset += 32;
        }

        // Verify total length
        assertEq(encoded.length, 9 + (_length * 64), "Encoded length should match expected");
    }

    /// @notice Tests encoding with a single output root
    function test_encodeSuperRootProof_singleOutputRoot_succeeds() external pure {
        // Create a single output root
        Types.OutputRootWithChainId[] memory outputRoots = new Types.OutputRootWithChainId[](1);
        outputRoots[0] = Types.OutputRootWithChainId({ chainId: 10, root: bytes32(uint256(0xdeadbeef)) });

        // Create the super root proof
        Types.SuperRootProof memory proof =
            Types.SuperRootProof({ version: 0x01, timestamp: 1234567890, outputRoots: outputRoots });

        // Encode the proof
        bytes memory encoded = Encoding.encodeSuperRootProof(proof);

        // Expected: 1 byte version + 8 bytes timestamp + (32 bytes chainId + 32 bytes root)
        assertEq(encoded.length, 1 + 8 + 64, "Encoded length should be 73 bytes");
        assertEq(encoded[0], bytes1(0x01), "First byte should be version 0x01");
    }

    /// @notice Tests encoding with multiple output roots
    function test_encodeSuperRootProof_multipleOutputRoots_succeeds() external pure {
        // Create multiple output roots
        Types.OutputRootWithChainId[] memory outputRoots = new Types.OutputRootWithChainId[](3);
        outputRoots[0] = Types.OutputRootWithChainId({ chainId: 10, root: bytes32(uint256(0xdeadbeef)) });
        outputRoots[1] = Types.OutputRootWithChainId({ chainId: 20, root: bytes32(uint256(0xbeefcafe)) });
        outputRoots[2] = Types.OutputRootWithChainId({ chainId: 30, root: bytes32(uint256(0xcafebabe)) });

        // Create the super root proof
        Types.SuperRootProof memory proof =
            Types.SuperRootProof({ version: 0x01, timestamp: 1234567890, outputRoots: outputRoots });

        // Encode the proof
        bytes memory encoded = Encoding.encodeSuperRootProof(proof);

        // Expected: 1 byte version + 8 bytes timestamp + 3 * (32 bytes chainId + 32 bytes root)
        assertEq(encoded.length, 1 + 8 + (3 * 64), "Encoded length should be 201 bytes");
    }

    /// @notice Tests that the Solidity impl of encodeSuperRootProof matches the FFI impl
    /// @param _timestamp The timestamp of the super root proof
    /// @param _length The number of output roots in the super root proof
    /// @param _seed The seed used to generate the output roots
    function testDiff_encodeSuperRootProof_succeeds(uint64 _timestamp, uint256 _length, uint256 _seed) external {
        // Ensure at least 1 element and cap at a reasonable maximum to avoid gas issues
        _length = uint256(bound(_length, 1, 50));

        // Create output roots array
        Types.OutputRootWithChainId[] memory outputRoots = new Types.OutputRootWithChainId[](_length);

        // Generate deterministic chain IDs and roots based on the seed
        for (uint256 i = 0; i < _length; i++) {
            // Use different derivations of the seed for each value
            uint256 chainId = uint256(keccak256(abi.encode(_seed, "chainId", i)));
            bytes32 root = keccak256(abi.encode(_seed, "root", i));

            outputRoots[i] = Types.OutputRootWithChainId({ chainId: chainId, root: root });
        }

        // Create the super root proof
        Types.SuperRootProof memory proof =
            Types.SuperRootProof({ version: 0x01, timestamp: _timestamp, outputRoots: outputRoots });

        // Encode using the Solidity implementation
        bytes memory encoding1 = Encoding.encodeSuperRootProof(proof);

        // Encode using the FFI implementation
        bytes memory encoding2 = ffi.encodeSuperRootProof(proof);

        // Compare the results
        assertEq(encoding1, encoding2, "Solidity and FFI implementations should match");
    }
}

contract Encoding_encodeSuperRootProof_TestFail is Encoding_TestInit {
    /// @notice Tests that encoding fails when version is not 0x01
    /// @param _version The version to use for the super root proof
    /// @param _timestamp The timestamp of the super root proof
    function testFuzz_encodeSuperRootProof_invalidVersion_reverts(bytes1 _version, uint64 _timestamp) external {
        // Ensure version is not 0x01
        if (_version == 0x01) {
            _version = 0x02;
        }

        // Create a minimal valid output roots array
        Types.OutputRootWithChainId[] memory outputRoots = new Types.OutputRootWithChainId[](1);
        outputRoots[0] = Types.OutputRootWithChainId({ chainId: 1, root: bytes32(uint256(1)) });

        // Create the super root proof with invalid version
        Types.SuperRootProof memory proof =
            Types.SuperRootProof({ version: _version, timestamp: _timestamp, outputRoots: outputRoots });

        // Expect revert when encoding
        vm.expectRevert(Encoding.Encoding_InvalidSuperRootVersion.selector);
        encoding.encodeSuperRootProof(proof);
    }

    /// @notice Tests that encoding fails when output roots array is empty
    /// @param _timestamp The timestamp of the super root proof
    function testFuzz_encodeSuperRootProof_emptyOutputRoots_reverts(uint64 _timestamp) external {
        // Create an empty output roots array
        Types.OutputRootWithChainId[] memory outputRoots = new Types.OutputRootWithChainId[](0);

        // Create the super root proof with empty output roots
        Types.SuperRootProof memory proof =
            Types.SuperRootProof({ version: 0x01, timestamp: _timestamp, outputRoots: outputRoots });

        // Expect revert when encoding
        vm.expectRevert(Encoding.Encoding_EmptySuperRoot.selector);
        encoding.encodeSuperRootProof(proof);
    }
}

contract Encoding_Harness {
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

    function encodeSuperRootProof(Types.SuperRootProof memory proof) external pure returns (bytes memory) {
        return Encoding.encodeSuperRootProof(proof);
    }
}
