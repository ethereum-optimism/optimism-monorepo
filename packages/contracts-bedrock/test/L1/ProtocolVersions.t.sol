// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { CommonTest } from "test/setup/CommonTest.sol";
import { EIP1967Helper } from "test/mocks/EIP1967Helper.sol";

// Interfaces
import { IProxy } from "interfaces/universal/IProxy.sol";
import { IProtocolVersions, ProtocolVersion } from "interfaces/L1/IProtocolVersions.sol";

// Libraries
import { Encoding } from "src/libraries/Encoding.sol";

contract ProtocolVersions_Init is CommonTest {
    event ConfigUpdate(uint256 indexed version, IProtocolVersions.UpdateType indexed updateType, bytes data);

    ProtocolVersion required;
    ProtocolVersion recommended;

    function setUp() public virtual override {
        super.setUp();
        required = ProtocolVersion.wrap(deploy.cfg().requiredProtocolVersion());
        recommended = ProtocolVersion.wrap(deploy.cfg().recommendedProtocolVersion());
    }
}

contract ProtocolVersions_Initialize_Test is ProtocolVersions_Init {
    /// @dev Tests that initialization sets the correct values.
    function test_initialize_values_succeeds() external {
        skipIfForkTest(
            "ProtocolVersions_Initialize_Test: cannot test initialization on forked network against hardhat config"
        );
        IProtocolVersions protocolVersionsImpl = IProtocolVersions(artifacts.mustGetAddress("ProtocolVersionsImpl"));
        address owner = deploy.cfg().finalSystemOwner();

        assertEq(ProtocolVersion.unwrap(protocolVersions.required()), ProtocolVersion.unwrap(required));
        assertEq(ProtocolVersion.unwrap(protocolVersions.recommended()), ProtocolVersion.unwrap(recommended));
        assertEq(protocolVersions.owner(), owner);

        assertEq(ProtocolVersion.unwrap(protocolVersionsImpl.required()), 0);
        assertEq(ProtocolVersion.unwrap(protocolVersionsImpl.recommended()), 0);
        assertEq(protocolVersionsImpl.owner(), address(0));
    }

    /// @dev Ensures that the events are emitted during initialization.
    function test_initialize_events_succeeds() external {
        IProtocolVersions protocolVersionsImpl = IProtocolVersions(artifacts.mustGetAddress("ProtocolVersionsImpl"));

        // Wipe out the initialized slot so the proxy can be initialized again
        vm.store(address(protocolVersions), bytes32(0), bytes32(0));

        // The order depends here
        vm.expectEmit(true, true, true, true, address(protocolVersions));
        emit ConfigUpdate(0, IProtocolVersions.UpdateType.REQUIRED_PROTOCOL_VERSION, abi.encode(required));
        vm.expectEmit(true, true, true, true, address(protocolVersions));
        emit ConfigUpdate(0, IProtocolVersions.UpdateType.RECOMMENDED_PROTOCOL_VERSION, abi.encode(recommended));

        vm.prank(EIP1967Helper.getAdmin(address(protocolVersions)));
        IProxy(payable(address(protocolVersions))).upgradeToAndCall(
            address(protocolVersionsImpl),
            abi.encodeCall(
                IProtocolVersions.initialize,
                (
                    alice, // _owner
                    required, // _required
                    recommended // recommended
                )
            )
        );
    }
}

contract ProtocolVersions_Setters_TestFail is ProtocolVersions_Init {
    /// @dev Tests that `setRequired` reverts if the caller is not the owner.
    function test_setRequired_notOwner_reverts() external {
        vm.expectRevert("Ownable: caller is not the owner");
        protocolVersions.setRequired(ProtocolVersion.wrap(0));
    }

    /// @dev Tests that `setRecommended` reverts if the caller is not the owner.
    function test_setRecommended_notOwner_reverts() external {
        vm.expectRevert("Ownable: caller is not the owner");
        protocolVersions.setRecommended(ProtocolVersion.wrap(0));
    }
}

contract ProtocolVersions_Setters_Test is ProtocolVersions_Init {
    /// @dev Tests that `setRequired` updates the required protocol version successfully.
    function testFuzz_setRequired_succeeds(uint256 _version) external {
        vm.expectEmit(true, true, true, true);
        emit ConfigUpdate(0, IProtocolVersions.UpdateType.REQUIRED_PROTOCOL_VERSION, abi.encode(_version));

        vm.prank(protocolVersions.owner());
        protocolVersions.setRequired(ProtocolVersion.wrap(_version));
        assertEq(ProtocolVersion.unwrap(protocolVersions.required()), _version);
    }

    /// @dev Tests that `setRecommended` updates the recommended protocol version successfully.
    function testFuzz_setRecommended_succeeds(uint256 _version) external {
        vm.expectEmit(true, true, true, true);
        emit ConfigUpdate(0, IProtocolVersions.UpdateType.RECOMMENDED_PROTOCOL_VERSION, abi.encode(_version));

        vm.prank(protocolVersions.owner());
        protocolVersions.setRecommended(ProtocolVersion.wrap(_version));
        assertEq(ProtocolVersion.unwrap(protocolVersions.recommended()), _version);
    }

    /// @dev Tests that `requiredVersion` returns the correct string representation
    function test_requiredVersion_succeeds() external {
        // Set a known protocol version
        ProtocolVersion version = ProtocolVersion.wrap(
            uint256(bytes32(hex"00000000000000000123456789abcdef00000001000000020000000300000004"))
        );

        vm.prank(protocolVersions.owner());
        protocolVersions.setRequired(version);

        // Check the string representation
        assertEq(protocolVersions.requiredVersion(), "v1.2.3-4+0x0123456789abcdef");
    }

    /// @dev Tests that `recommendedVersion` returns the correct string representation
    function test_recommendedVersion_succeeds() external {
        // Set a known protocol version
        ProtocolVersion version = ProtocolVersion.wrap(
            uint256(bytes32(hex"00000000000000000123456789abcdef00000001000000020000000300000004"))
        );

        vm.prank(protocolVersions.owner());
        protocolVersions.setRecommended(version);

        // Check the string representation
        assertEq(protocolVersions.recommendedVersion(), "v1.2.3-4+0x0123456789abcdef");
    }

    /// @dev Tests that version strings are correct for versions without build/prerelease
    function test_versionStrings_minimal_succeeds() external {
        // Version with no build ID or prerelease
        ProtocolVersion version = ProtocolVersion.wrap(
            uint256(bytes32(hex"0000000000000000000000000000000000000001000000020000000300000000"))
        );

        vm.startPrank(protocolVersions.owner());
        protocolVersions.setRequired(version);
        protocolVersions.setRecommended(version);
        vm.stopPrank();

        assertEq(protocolVersions.requiredVersion(), "v1.2.3");
        assertEq(protocolVersions.recommendedVersion(), "v1.2.3");
    }

    /// @dev Fuzz test that required version string encoding/decoding is consistent
    function testFuzz_requiredVersion_succeeds(
        bytes8 _build,
        uint32 _major,
        uint32 _minor,
        uint32 _patch,
        uint32 _preRelease
    )
        external
    {
        ProtocolVersion version =
            ProtocolVersion.wrap(uint256(Encoding.encodeProtocolVersion(_build, _major, _minor, _patch, _preRelease)));

        vm.prank(protocolVersions.owner());
        protocolVersions.setRequired(version);

        // Get version string from contract
        string memory versionString = protocolVersions.requiredVersion();

        // Build expected string
        string memory expected = string(
            abi.encodePacked(
                "v", Encoding.uint2str(_major), ".", Encoding.uint2str(_minor), ".", Encoding.uint2str(_patch)
            )
        );

        if (_preRelease != 0) {
            expected = string(abi.encodePacked(expected, "-", Encoding.uint2str(_preRelease)));
        }

        if (uint64(_build) != 0) {
            expected = string(abi.encodePacked(expected, "+0x", Encoding.bytes2hex(_build)));
        }

        assertEq(versionString, expected);
    }

    /// @dev Fuzz test that recommended version string encoding/decoding is consistent
    function testFuzz_recommendedVersion_succeeds(
        bytes8 _build,
        uint32 _major,
        uint32 _minor,
        uint32 _patch,
        uint32 _preRelease
    )
        external
    {
        ProtocolVersion version =
            ProtocolVersion.wrap(uint256(Encoding.encodeProtocolVersion(_build, _major, _minor, _patch, _preRelease)));

        vm.prank(protocolVersions.owner());
        protocolVersions.setRecommended(version);

        // Get version string from contract
        string memory versionString = protocolVersions.recommendedVersion();

        // Build expected string
        string memory expected = string(
            abi.encodePacked(
                "v", Encoding.uint2str(_major), ".", Encoding.uint2str(_minor), ".", Encoding.uint2str(_patch)
            )
        );

        if (_preRelease != 0) {
            expected = string(abi.encodePacked(expected, "-", Encoding.uint2str(_preRelease)));
        }

        if (uint64(_build) != 0) {
            expected = string(abi.encodePacked(expected, "+0x", Encoding.bytes2hex(_build)));
        }

        assertEq(versionString, expected);
    }
}
