// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { CommonTest } from "test/setup/CommonTest.sol";
import { EIP1967Helper } from "test/mocks/EIP1967Helper.sol";

// Interfaces
import { IProxy } from "interfaces/universal/IProxy.sol";
import { IProtocolVersions, ProtocolVersion } from "interfaces/L1/IProtocolVersions.sol";

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
}
