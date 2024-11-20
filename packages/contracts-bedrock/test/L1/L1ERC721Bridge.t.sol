// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { CommonTest } from "test/setup/CommonTest.sol";

// Contracts
import { ERC721 } from "@openzeppelin/contracts/token/ERC721/ERC721.sol";

// Libraries
import { Predeploys } from "src/libraries/Predeploys.sol";

// Interfaces
import { ISuperchainConfig } from "src/L1/interfaces/ISuperchainConfig.sol";
import { ICrossDomainMessenger } from "src/universal/interfaces/ICrossDomainMessenger.sol";
import { IL1ERC721Bridge } from "src/L1/interfaces/IL1ERC721Bridge.sol";
import { IL2ERC721Bridge } from "src/L2/interfaces/IL2ERC721Bridge.sol";

/// @dev Test ERC721 contract.
contract TestERC721 is ERC721 {
    constructor() ERC721("Test", "TST") { }

    function mint(address to, uint256 tokenId) public {
        _mint(to, tokenId);
    }
}

contract L1ERC721Bridge_Test is CommonTest {
    TestERC721 internal localToken;
    TestERC721 internal remoteToken;
    uint256 internal constant tokenId = 1;

    event ERC721BridgeInitiated(
        address indexed localToken,
        address indexed remoteToken,
        address indexed from,
        address to,
        uint256 tokenId,
        bytes extraData
    );

    event ERC721BridgeFinalized(
        address indexed localToken,
        address indexed remoteToken,
        address indexed from,
        address to,
        uint256 tokenId,
        bytes extraData
    );

    /// @dev Sets up the testing environment.
    /// @notice Marked virtual to be overridden in
    ///         test/kontrol/deployment/DeploymentSummary.t.sol
    function setUp() public virtual override {
        super.setUp();

        localToken = new TestERC721();
        remoteToken = new TestERC721();

        // Mint alice a token.
        localToken.mint(alice, tokenId);

        // Approve the bridge to transfer the token.
        vm.prank(alice);
        localToken.approve(address(l1ERC721Bridge), tokenId);
    }

    /// @dev Tests that the impl is created with the correct values.
    /// @notice Marked virtual to be overridden in
    ///         test/kontrol/deployment/DeploymentSummary.t.sol
    function test_constructor_succeeds() public virtual {
        IL1ERC721Bridge impl = IL1ERC721Bridge(deploy.mustGetAddress("L1ERC721Bridge"));
        assertEq(address(impl.MESSENGER()), address(0));
        assertEq(address(impl.messenger()), address(0));
        assertEq(address(impl.OTHER_BRIDGE()), Predeploys.L2_ERC721_BRIDGE);
        assertEq(address(impl.otherBridge()), Predeploys.L2_ERC721_BRIDGE);
        assertEq(address(impl.superchainConfig()), address(0));
    }

    /// @dev Tests that the proxy is initialized with the correct values.
    function test_initialize_succeeds() public view {
        assertEq(address(l1ERC721Bridge.MESSENGER()), address(l1CrossDomainMessenger));
        assertEq(address(l1ERC721Bridge.messenger()), address(l1CrossDomainMessenger));
        assertEq(address(l1ERC721Bridge.OTHER_BRIDGE()), Predeploys.L2_ERC721_BRIDGE);
        assertEq(address(l1ERC721Bridge.otherBridge()), Predeploys.L2_ERC721_BRIDGE);
        assertEq(address(l1ERC721Bridge.superchainConfig()), address(superchainConfig));
    }

    /// @dev Tests that the ERC721 can be bridged successfully.
    function test_bridgeERC721_succeeds() public {
        // Expect a call to the messenger.
        vm.expectCall(
            address(l1CrossDomainMessenger),
            abi.encodeCall(
                l1CrossDomainMessenger.sendMessage,
                (
                    address(l2ERC721Bridge),
                    abi.encodeCall(
                        IL2ERC721Bridge.finalizeBridgeERC721,
                        (address(remoteToken), address(localToken), alice, alice, tokenId, hex"5678")
                    ),
                    1234
                )
            )
        );

        // Expect an event to be emitted.
        vm.expectEmit(true, true, true, true);
        emit ERC721BridgeInitiated(address(localToken), address(remoteToken), alice, alice, tokenId, hex"5678");

        // Bridge the token.
        vm.prank(alice);
        l1ERC721Bridge.bridgeERC721(address(localToken), address(remoteToken), tokenId, 1234, hex"5678");

        // Token is locked in the bridge.
        assertEq(l1ERC721Bridge.deposits(address(localToken), address(remoteToken), tokenId), true);
        assertEq(localToken.ownerOf(tokenId), address(l1ERC721Bridge));
    }

    /// @dev Tests that the ERC721 bridge reverts for non externally owned accounts.
    function test_bridgeERC721_fromContract_reverts() external {
        // Bridge the token.
        vm.etch(alice, hex"01");
        vm.prank(alice);
        vm.expectRevert("ERC721Bridge: account is not externally owned");
        l1ERC721Bridge.bridgeERC721(address(localToken), address(remoteToken), tokenId, 1234, hex"5678");

        // Token is not locked in the bridge.
        assertEq(l1ERC721Bridge.deposits(address(localToken), address(remoteToken), tokenId), false);
        assertEq(localToken.ownerOf(tokenId), alice);
    }

    /// @dev Tests that the ERC721 bridge reverts for a zero address local token.
    function test_bridgeERC721_localTokenZeroAddress_reverts() external {
        // Bridge the token.
        vm.prank(alice);
        vm.expectRevert(bytes(""));
        l1ERC721Bridge.bridgeERC721(address(0), address(remoteToken), tokenId, 1234, hex"5678");

        // Token is not locked in the bridge.
        assertEq(l1ERC721Bridge.deposits(address(localToken), address(remoteToken), tokenId), false);
        assertEq(localToken.ownerOf(tokenId), alice);
    }

    /// @dev Tests that the ERC721 bridge reverts for a zero address remote token.
    function test_bridgeERC721_remoteTokenZeroAddress_reverts() external {
        // Bridge the token.
        vm.prank(alice);
        vm.expectRevert("L1ERC721Bridge: remote token cannot be address(0)");
        l1ERC721Bridge.bridgeERC721(address(localToken), address(0), tokenId, 1234, hex"5678");

        // Token is not locked in the bridge.
        assertEq(l1ERC721Bridge.deposits(address(localToken), address(remoteToken), tokenId), false);
        assertEq(localToken.ownerOf(tokenId), alice);
    }

    /// @dev Tests that the ERC721 bridge reverts for an incorrect owner.
    function test_bridgeERC721_wrongOwner_reverts() external {
        // Bridge the token.
        vm.prank(bob);
        vm.expectRevert("ERC721: transfer from incorrect owner");
        l1ERC721Bridge.bridgeERC721(address(localToken), address(remoteToken), tokenId, 1234, hex"5678");

        // Token is not locked in the bridge.
        assertEq(l1ERC721Bridge.deposits(address(localToken), address(remoteToken), tokenId), false);
        assertEq(localToken.ownerOf(tokenId), alice);
    }

    /// @dev Tests that the ERC721 bridge successfully sends a token
    ///      to a different address than the owner.
    function test_bridgeERC721To_succeeds() external {
        // Expect a call to the messenger.
        vm.expectCall(
            address(l1CrossDomainMessenger),
            abi.encodeCall(
                l1CrossDomainMessenger.sendMessage,
                (
                    address(Predeploys.L2_ERC721_BRIDGE),
                    abi.encodeCall(
                        IL2ERC721Bridge.finalizeBridgeERC721,
                        (address(remoteToken), address(localToken), alice, bob, tokenId, hex"5678")
                    ),
                    1234
                )
            )
        );

        // Expect an event to be emitted.
        vm.expectEmit(true, true, true, true);
        emit ERC721BridgeInitiated(address(localToken), address(remoteToken), alice, bob, tokenId, hex"5678");

        // Bridge the token.
        vm.prank(alice);
        l1ERC721Bridge.bridgeERC721To(address(localToken), address(remoteToken), bob, tokenId, 1234, hex"5678");

        // Token is locked in the bridge.
        assertEq(l1ERC721Bridge.deposits(address(localToken), address(remoteToken), tokenId), true);
        assertEq(localToken.ownerOf(tokenId), address(l1ERC721Bridge));
    }

    /// @dev Tests that the ERC721 bridge reverts for non externally owned accounts
    ///      when sending to a different address than the owner.
    function test_bridgeERC721To_localTokenZeroAddress_reverts() external {
        // Bridge the token.
        vm.prank(alice);
        vm.expectRevert(bytes(""));
        l1ERC721Bridge.bridgeERC721To(address(0), address(remoteToken), bob, tokenId, 1234, hex"5678");

        // Token is not locked in the bridge.
        assertEq(l1ERC721Bridge.deposits(address(localToken), address(remoteToken), tokenId), false);
        assertEq(localToken.ownerOf(tokenId), alice);
    }

    /// @dev Tests that the ERC721 bridge reverts for a zero address remote token
    ///      when sending to a different address than the owner.
    function test_bridgeERC721To_remoteTokenZeroAddress_reverts() external {
        // Bridge the token.
        vm.prank(alice);
        vm.expectRevert("L1ERC721Bridge: remote token cannot be address(0)");
        l1ERC721Bridge.bridgeERC721To(address(localToken), address(0), bob, tokenId, 1234, hex"5678");

        // Token is not locked in the bridge.
        assertEq(l1ERC721Bridge.deposits(address(localToken), address(remoteToken), tokenId), false);
        assertEq(localToken.ownerOf(tokenId), alice);
    }

    /// @dev Tests that the ERC721 bridge reverts for an incorrect owner
    ////     when sending to a different address than the owner.
    function test_bridgeERC721To_wrongOwner_reverts() external {
        // Bridge the token.
        vm.prank(bob);
        vm.expectRevert("ERC721: transfer from incorrect owner");
        l1ERC721Bridge.bridgeERC721To(address(localToken), address(remoteToken), bob, tokenId, 1234, hex"5678");

        // Token is not locked in the bridge.
        assertEq(l1ERC721Bridge.deposits(address(localToken), address(remoteToken), tokenId), false);
        assertEq(localToken.ownerOf(tokenId), alice);
    }

    /// @dev Tests that `bridgeERC721To` reverts if the to address is the zero address.
    function test_bridgeERC721To_toZeroAddress_reverts() external {
        // Bridge the token.
        vm.prank(bob);
        vm.expectRevert("ERC721Bridge: nft recipient cannot be address(0)");
        l1ERC721Bridge.bridgeERC721To(address(localToken), address(remoteToken), address(0), tokenId, 1234, hex"5678");
    }

    /// @dev Tests that the ERC721 bridge successfully finalizes a withdrawal.
    function test_finalizeBridgeERC721_succeeds() external {
        // Bridge the token.
        vm.prank(alice);
        l1ERC721Bridge.bridgeERC721(address(localToken), address(remoteToken), tokenId, 1234, hex"5678");

        // Expect an event to be emitted.
        vm.expectEmit(true, true, true, true);
        emit ERC721BridgeFinalized(address(localToken), address(remoteToken), alice, alice, tokenId, hex"5678");

        // Finalize a withdrawal.
        vm.mockCall(
            address(l1CrossDomainMessenger),
            abi.encodeCall(l1CrossDomainMessenger.xDomainMessageSender, ()),
            abi.encode(Predeploys.L2_ERC721_BRIDGE)
        );
        vm.prank(address(l1CrossDomainMessenger));
        l1ERC721Bridge.finalizeBridgeERC721(address(localToken), address(remoteToken), alice, alice, tokenId, hex"5678");

        // Token is not locked in the bridge.
        assertEq(l1ERC721Bridge.deposits(address(localToken), address(remoteToken), tokenId), false);
        assertEq(localToken.ownerOf(tokenId), alice);
    }

    /// @dev Tests that the ERC721 bridge finalize reverts when not called
    ///      by the remote bridge.
    function test_finalizeBridgeERC721_notViaLocalMessenger_reverts() external {
        // Finalize a withdrawal.
        vm.prank(alice);
        vm.expectRevert("ERC721Bridge: function can only be called from the other bridge");
        l1ERC721Bridge.finalizeBridgeERC721(address(localToken), address(remoteToken), alice, alice, tokenId, hex"5678");
    }

    /// @dev Tests that the ERC721 bridge finalize reverts when not called
    ///      from the remote messenger.
    function test_finalizeBridgeERC721_notFromRemoteMessenger_reverts() external {
        // Finalize a withdrawal.
        vm.mockCall(
            address(l1CrossDomainMessenger),
            abi.encodeCall(l1CrossDomainMessenger.xDomainMessageSender, ()),
            abi.encode(alice)
        );
        vm.prank(address(l1CrossDomainMessenger));
        vm.expectRevert("ERC721Bridge: function can only be called from the other bridge");
        l1ERC721Bridge.finalizeBridgeERC721(address(localToken), address(remoteToken), alice, alice, tokenId, hex"5678");
    }

    /// @dev Tests that the ERC721 bridge finalize reverts when the local token
    ///      is set as the bridge itself.
    function test_finalizeBridgeERC721_selfToken_reverts() external {
        // Finalize a withdrawal.
        vm.mockCall(
            address(l1CrossDomainMessenger),
            abi.encodeCall(l1CrossDomainMessenger.xDomainMessageSender, ()),
            abi.encode(Predeploys.L2_ERC721_BRIDGE)
        );
        vm.prank(address(l1CrossDomainMessenger));
        vm.expectRevert("L1ERC721Bridge: local token cannot be self");
        l1ERC721Bridge.finalizeBridgeERC721(
            address(l1ERC721Bridge), address(remoteToken), alice, alice, tokenId, hex"5678"
        );
    }

    /// @dev Tests that the ERC721 bridge finalize reverts when the remote token
    ///      is not escrowed in the L1 bridge.
    function test_finalizeBridgeERC721_notEscrowed_reverts() external {
        // Finalize a withdrawal.
        vm.mockCall(
            address(l1CrossDomainMessenger),
            abi.encodeCall(l1CrossDomainMessenger.xDomainMessageSender, ()),
            abi.encode(Predeploys.L2_ERC721_BRIDGE)
        );
        vm.prank(address(l1CrossDomainMessenger));
        vm.expectRevert("L1ERC721Bridge: Token ID is not escrowed in the L1 Bridge");
        l1ERC721Bridge.finalizeBridgeERC721(address(localToken), address(remoteToken), alice, alice, tokenId, hex"5678");
    }
}

contract L1ERC721Bridge_Pause_Test is CommonTest {
    /// @dev Verifies that the `paused` accessor returns the same value as the `paused` function of the
    ///      `superchainConfig`.
    function test_paused_succeeds() external view {
        assertEq(l1ERC721Bridge.paused(), superchainConfig.paused());
    }

    /// @dev Ensures that the `paused` function of the bridge contract actually calls the `paused` function of the
    ///      `superchainConfig`.
    function test_pause_callsSuperchainConfig_succeeds() external {
        vm.expectCall(address(superchainConfig), abi.encodeCall(ISuperchainConfig.paused, ()));
        l1ERC721Bridge.paused();
    }

    /// @dev Checks that the `paused` state of the bridge matches the `paused` state of the `superchainConfig` after
    ///      it's been changed.
    function test_pause_matchesSuperchainConfig_succeeds() external {
        assertFalse(l1StandardBridge.paused());
        assertEq(l1StandardBridge.paused(), superchainConfig.paused());

        vm.prank(superchainConfig.guardian());
        superchainConfig.pause("identifier");

        assertTrue(l1StandardBridge.paused());
        assertEq(l1StandardBridge.paused(), superchainConfig.paused());
    }
}

contract L1ERC721Bridge_Pause_TestFail is CommonTest {
    /// @dev Sets up the test by pausing the bridge, giving ether to the bridge and mocking
    ///      the calls to the xDomainMessageSender so that it returns the correct value.
    function setUp() public override {
        super.setUp();
        vm.prank(superchainConfig.guardian());
        superchainConfig.pause("identifier");
        assertTrue(l1ERC721Bridge.paused());

        vm.mockCall(
            address(l1ERC721Bridge.messenger()),
            abi.encodeCall(ICrossDomainMessenger.xDomainMessageSender, ()),
            abi.encode(address(l1ERC721Bridge.otherBridge()))
        );
    }

    // @dev Ensures that the `bridgeERC721` function reverts when the bridge is paused.
    function test_pause_finalizeBridgeERC721_reverts() external {
        vm.prank(address(l1ERC721Bridge.messenger()));
        vm.expectRevert("L1ERC721Bridge: paused");
        l1ERC721Bridge.finalizeBridgeERC721({
            _localToken: address(0),
            _remoteToken: address(0),
            _from: address(0),
            _to: address(0),
            _tokenId: 0,
            _extraData: hex""
        });
    }
}
