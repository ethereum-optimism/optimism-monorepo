// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { stdStorage, StdStorage } from "forge-std/Test.sol";
import { CommonTest } from "test/setup/CommonTest.sol";

// Contracts
import { ERC20 } from "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import { StandardBridge } from "src/universal/StandardBridge.sol";

// Libraries
import { Predeploys } from "src/libraries/Predeploys.sol";
import { AddressAliasHelper } from "src/vendor/AddressAliasHelper.sol";

// Interfaces
import { ICrossDomainMessenger } from "src/universal/interfaces/ICrossDomainMessenger.sol";
import { ISuperchainConfig } from "src/L1/interfaces/ISuperchainConfig.sol";
import { IOptimismPortal } from "src/L1/interfaces/IOptimismPortal.sol";
import { IL1StandardBridge } from "src/L1/interfaces/IL1StandardBridge.sol";

contract L1StandardBridge_Getter_Test is CommonTest {
    /// @dev Test that the accessors return the correct initialized values.
    function test_getters_succeeds() external view {
        assert(l1StandardBridge.l2TokenBridge() == address(l2StandardBridge));
        assert(address(l1StandardBridge.OTHER_BRIDGE()) == address(l2StandardBridge));
        assert(address(l1StandardBridge.messenger()) == address(l1CrossDomainMessenger));
        assert(address(l1StandardBridge.MESSENGER()) == address(l1CrossDomainMessenger));
        assert(l1StandardBridge.superchainConfig() == superchainConfig);
        assert(l1StandardBridge.systemConfig() == systemConfig);
    }
}

contract L1StandardBridge_Initialize_Test is CommonTest {
    /// @dev Test that the constructor sets the correct values.
    /// @notice Marked virtual to be overridden in
    ///         test/kontrol/deployment/DeploymentSummary.t.sol
    function test_constructor_succeeds() external virtual {
        IL1StandardBridge impl = IL1StandardBridge(deploy.mustGetAddress("L1StandardBridge"));
        assertEq(address(impl.superchainConfig()), address(0));
        assertEq(address(impl.MESSENGER()), address(0));
        assertEq(address(impl.messenger()), address(0));
        assertEq(address(impl.OTHER_BRIDGE()), Predeploys.L2_STANDARD_BRIDGE);
        assertEq(address(impl.otherBridge()), Predeploys.L2_STANDARD_BRIDGE);
        assertEq(address(l2StandardBridge), Predeploys.L2_STANDARD_BRIDGE);
        assertEq(address(impl.systemConfig()), address(0));
    }

    /// @dev Test that the initialize function sets the correct values.
    function test_initialize_succeeds() external view {
        assertEq(address(l1StandardBridge.superchainConfig()), address(superchainConfig));
        assertEq(address(l1StandardBridge.MESSENGER()), address(l1CrossDomainMessenger));
        assertEq(address(l1StandardBridge.messenger()), address(l1CrossDomainMessenger));
        assertEq(address(l1StandardBridge.OTHER_BRIDGE()), Predeploys.L2_STANDARD_BRIDGE);
        assertEq(address(l1StandardBridge.otherBridge()), Predeploys.L2_STANDARD_BRIDGE);
        assertEq(address(l2StandardBridge), Predeploys.L2_STANDARD_BRIDGE);
        assertEq(address(l1StandardBridge.systemConfig()), address(systemConfig));
    }
}

contract L1StandardBridge_Pause_Test is CommonTest {
    /// @dev Verifies that the `paused` accessor returns the same value as the `paused` function of the
    ///      `superchainConfig`.
    function test_paused_succeeds() external view {
        assertEq(l1StandardBridge.paused(), superchainConfig.paused());
    }

    /// @dev Ensures that the `paused` function of the bridge contract actually calls the `paused` function of the
    ///      `superchainConfig`.
    function test_pause_callsSuperchainConfig_succeeds() external {
        vm.expectCall(address(superchainConfig), abi.encodeCall(ISuperchainConfig.paused, ()));
        l1StandardBridge.paused();
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

contract L1StandardBridge_Pause_TestFail is CommonTest {
    /// @dev Sets up the test by pausing the bridge, giving ether to the bridge and mocking
    ///      the calls to the xDomainMessageSender so that it returns the correct value.
    function setUp() public override {
        super.setUp();
        vm.prank(superchainConfig.guardian());
        superchainConfig.pause("identifier");
        assertTrue(l1StandardBridge.paused());

        vm.deal(address(l1StandardBridge.messenger()), 1 ether);

        vm.mockCall(
            address(l1StandardBridge.messenger()),
            abi.encodeCall(ICrossDomainMessenger.xDomainMessageSender, ()),
            abi.encode(address(l1StandardBridge.otherBridge()))
        );
    }

    /// @dev Confirms that the `finalizeBridgeETH` function reverts when the bridge is paused.
    function test_pause_finalizeBridgeETH_reverts() external {
        vm.prank(address(l1StandardBridge.messenger()));
        vm.expectRevert("StandardBridge: paused");
        l1StandardBridge.finalizeBridgeETH{ value: 100 }({
            _from: address(2),
            _to: address(3),
            _amount: 100,
            _extraData: hex""
        });
    }

    /// @dev Confirms that the `finalizeETHWithdrawal` function reverts when the bridge is paused.
    function test_pause_finalizeETHWithdrawal_reverts() external {
        vm.prank(address(l1StandardBridge.messenger()));
        vm.expectRevert("StandardBridge: paused");
        l1StandardBridge.finalizeETHWithdrawal{ value: 100 }({
            _from: address(2),
            _to: address(3),
            _amount: 100,
            _extraData: hex""
        });
    }

    /// @dev Confirms that the `finalizeERC20Withdrawal` function reverts when the bridge is paused.
    function test_pause_finalizeERC20Withdrawal_reverts() external {
        vm.prank(address(l1StandardBridge.messenger()));
        vm.expectRevert("StandardBridge: paused");
        l1StandardBridge.finalizeERC20Withdrawal({
            _l1Token: address(0),
            _l2Token: address(0),
            _from: address(0),
            _to: address(0),
            _amount: 0,
            _extraData: hex""
        });
    }

    /// @dev Confirms that the `finalizeBridgeERC20` function reverts when the bridge is paused.
    function test_pause_finalizeBridgeERC20_reverts() external {
        vm.prank(address(l1StandardBridge.messenger()));
        vm.expectRevert("StandardBridge: paused");
        l1StandardBridge.finalizeBridgeERC20({
            _localToken: address(0),
            _remoteToken: address(0),
            _from: address(0),
            _to: address(0),
            _amount: 0,
            _extraData: hex""
        });
    }
}

contract L1StandardBridge_Initialize_TestFail is CommonTest { }

contract L1StandardBridge_Receive_Test is CommonTest {
    /// @dev Tests receive bridges ETH successfully.
    function test_receive_succeeds() external {
        assertEq(address(optimismPortal).balance, 0);

        // The legacy event must be emitted for backwards compatibility
        vm.expectEmit(address(l1StandardBridge));
        emit ETHDepositInitiated(alice, alice, 100, hex"");

        vm.expectEmit(address(l1StandardBridge));
        emit ETHBridgeInitiated(alice, alice, 100, hex"");

        vm.expectCall(
            address(l1CrossDomainMessenger),
            abi.encodeCall(
                ICrossDomainMessenger.sendMessage,
                (
                    address(l2StandardBridge),
                    abi.encodeCall(StandardBridge.finalizeBridgeETH, (alice, alice, 100, hex"")),
                    200_000
                )
            )
        );

        vm.prank(alice, alice);
        (bool success,) = address(l1StandardBridge).call{ value: 100 }(hex"");
        assertEq(success, true);
        assertEq(address(optimismPortal).balance, 100);
    }
}

contract L1StandardBridge_Receive_TestFail is CommonTest {
    /// @dev Tests receive function reverts with custom gas token.
    function testFuzz_receive_customGasToken_reverts(uint256 _value) external {
        vm.prank(alice, alice);
        vm.mockCall(
            address(systemConfig), abi.encodeCall(systemConfig.gasPayingToken, ()), abi.encode(address(1), uint8(18))
        );
        vm.deal(alice, _value);
        (bool success, bytes memory data) = address(l1StandardBridge).call{ value: _value }(hex"");
        assertFalse(success);
        assembly {
            data := add(data, 0x04)
        }
        assertEq(abi.decode(data, (string)), "StandardBridge: cannot bridge ETH with custom gas token");
    }
}

contract PreBridgeETH is CommonTest {
    /// @dev Asserts the expected calls and events for bridging ETH depending
    ///      on whether the bridge call is legacy or not.
    function _preBridgeETH(bool isLegacy, uint256 value) internal {
        assertEq(address(optimismPortal).balance, 0);
        uint256 nonce = l1CrossDomainMessenger.messageNonce();
        uint256 version = 0; // Internal constant in the OptimismPortal: DEPOSIT_VERSION
        address l1MessengerAliased = AddressAliasHelper.applyL1ToL2Alias(address(l1CrossDomainMessenger));

        bytes memory message = abi.encodeCall(StandardBridge.finalizeBridgeETH, (alice, alice, value, hex"dead"));

        if (isLegacy) {
            vm.expectCall(
                address(l1StandardBridge), value, abi.encodeCall(l1StandardBridge.depositETH, (50000, hex"dead"))
            );
        } else {
            vm.expectCall(
                address(l1StandardBridge), value, abi.encodeCall(l1StandardBridge.bridgeETH, (50000, hex"dead"))
            );
        }
        vm.expectCall(
            address(l1CrossDomainMessenger),
            value,
            abi.encodeCall(ICrossDomainMessenger.sendMessage, (address(l2StandardBridge), message, 50000))
        );

        bytes memory innerMessage = abi.encodeCall(
            ICrossDomainMessenger.relayMessage,
            (nonce, address(l1StandardBridge), address(l2StandardBridge), value, 50000, message)
        );

        uint64 baseGas = l1CrossDomainMessenger.baseGas(message, 50000);
        vm.expectCall(
            address(optimismPortal),
            value,
            abi.encodeCall(
                IOptimismPortal.depositTransaction,
                (address(l2CrossDomainMessenger), value, baseGas, false, innerMessage)
            )
        );

        bytes memory opaqueData = abi.encodePacked(uint256(value), uint256(value), baseGas, false, innerMessage);

        vm.expectEmit(address(l1StandardBridge));
        emit ETHDepositInitiated(alice, alice, value, hex"dead");

        vm.expectEmit(address(l1StandardBridge));
        emit ETHBridgeInitiated(alice, alice, value, hex"dead");

        // OptimismPortal emits a TransactionDeposited event on `depositTransaction` call
        vm.expectEmit(address(optimismPortal));
        emit TransactionDeposited(l1MessengerAliased, address(l2CrossDomainMessenger), version, opaqueData);

        // SentMessage event emitted by the CrossDomainMessenger
        vm.expectEmit(address(l1CrossDomainMessenger));
        emit SentMessage(address(l2StandardBridge), address(l1StandardBridge), message, nonce, 50000);

        // SentMessageExtension1 event emitted by the CrossDomainMessenger
        vm.expectEmit(address(l1CrossDomainMessenger));
        emit SentMessageExtension1(address(l1StandardBridge), value);

        vm.prank(alice, alice);
    }
}

contract L1StandardBridge_DepositETH_Test is PreBridgeETH {
    /// @dev Tests that depositing ETH succeeds.
    ///      Emits ETHDepositInitiated and ETHBridgeInitiated events.
    ///      Calls depositTransaction on the OptimismPortal.
    ///      Only EOA can call depositETH.
    ///      ETH ends up in the optimismPortal.
    function test_depositETH_succeeds() external {
        _preBridgeETH({ isLegacy: true, value: 500 });
        l1StandardBridge.depositETH{ value: 500 }(50000, hex"dead");
        assertEq(address(optimismPortal).balance, 500);
    }
}

contract L1StandardBridge_DepositETH_TestFail is CommonTest {
    /// @dev Tests that depositing ETH reverts if the call is not from an EOA.
    function test_depositETH_notEoa_reverts() external {
        vm.etch(alice, address(L1Token).code);
        vm.expectRevert("StandardBridge: function can only be called from an EOA");
        vm.prank(alice);
        l1StandardBridge.depositETH{ value: 1 }(300, hex"");
    }

    /// @dev Tests that depositing reverts with custom gas token.
    function test_depositETH_customGasToken_reverts() external {
        vm.mockCall(
            address(systemConfig), abi.encodeCall(systemConfig.gasPayingToken, ()), abi.encode(address(1), uint8(2))
        );
        vm.prank(alice, alice);
        vm.expectRevert("StandardBridge: cannot bridge ETH with custom gas token");
        l1StandardBridge.depositETH(50000, hex"dead");
    }
}

contract L1StandardBridge_BridgeETH_Test is PreBridgeETH {
    /// @dev Tests that bridging ETH succeeds.
    ///      Emits ETHDepositInitiated and ETHBridgeInitiated events.
    ///      Calls depositTransaction on the OptimismPortal.
    ///      Only EOA can call bridgeETH.
    ///      ETH ends up in the optimismPortal.
    function test_bridgeETH_succeeds() external {
        _preBridgeETH({ isLegacy: false, value: 500 });
        l1StandardBridge.bridgeETH{ value: 500 }(50000, hex"dead");
        assertEq(address(optimismPortal).balance, 500);
    }
}

contract L1StandardBridge_BridgeETH_TestFail is PreBridgeETH {
    /// @dev Tests that bridging eth reverts with custom gas token.
    function test_bridgeETH_customGasToken_reverts() external {
        vm.prank(alice, alice);
        vm.mockCall(
            address(systemConfig), abi.encodeCall(systemConfig.gasPayingToken, ()), abi.encode(address(1), uint8(2))
        );
        vm.expectRevert("StandardBridge: cannot bridge ETH with custom gas token");

        l1StandardBridge.bridgeETH(50000, hex"dead");
    }
}

contract PreBridgeETHTo is CommonTest {
    /// @dev Asserts the expected calls and events for bridging ETH to a different
    ///      address depending on whether the bridge call is legacy or not.
    function _preBridgeETHTo(bool isLegacy, uint256 value) internal {
        assertEq(address(optimismPortal).balance, 0);
        uint256 nonce = l1CrossDomainMessenger.messageNonce();
        uint256 version = 0; // Internal constant in the OptimismPortal: DEPOSIT_VERSION
        address l1MessengerAliased = AddressAliasHelper.applyL1ToL2Alias(address(l1CrossDomainMessenger));

        if (isLegacy) {
            vm.expectCall(
                address(l1StandardBridge), value, abi.encodeCall(l1StandardBridge.depositETHTo, (bob, 60000, hex"dead"))
            );
        } else {
            vm.expectCall(
                address(l1StandardBridge), value, abi.encodeCall(l1StandardBridge.bridgeETHTo, (bob, 60000, hex"dead"))
            );
        }

        bytes memory message = abi.encodeCall(StandardBridge.finalizeBridgeETH, (alice, bob, value, hex"dead"));

        // the L1 bridge should call
        // L1CrossDomainMessenger.sendMessage
        vm.expectCall(
            address(l1CrossDomainMessenger),
            abi.encodeCall(ICrossDomainMessenger.sendMessage, (address(l2StandardBridge), message, 60000))
        );

        bytes memory innerMessage = abi.encodeCall(
            ICrossDomainMessenger.relayMessage,
            (nonce, address(l1StandardBridge), address(l2StandardBridge), value, 60000, message)
        );

        uint64 baseGas = l1CrossDomainMessenger.baseGas(message, 60000);
        vm.expectCall(
            address(optimismPortal),
            abi.encodeCall(
                IOptimismPortal.depositTransaction,
                (address(l2CrossDomainMessenger), value, baseGas, false, innerMessage)
            )
        );

        bytes memory opaqueData = abi.encodePacked(uint256(value), uint256(value), baseGas, false, innerMessage);

        vm.expectEmit(address(l1StandardBridge));
        emit ETHDepositInitiated(alice, bob, value, hex"dead");

        vm.expectEmit(address(l1StandardBridge));
        emit ETHBridgeInitiated(alice, bob, value, hex"dead");

        // OptimismPortal emits a TransactionDeposited event on `depositTransaction` call
        vm.expectEmit(address(optimismPortal));
        emit TransactionDeposited(l1MessengerAliased, address(l2CrossDomainMessenger), version, opaqueData);

        // SentMessage event emitted by the CrossDomainMessenger
        vm.expectEmit(address(l1CrossDomainMessenger));
        emit SentMessage(address(l2StandardBridge), address(l1StandardBridge), message, nonce, 60000);

        // SentMessageExtension1 event emitted by the CrossDomainMessenger
        vm.expectEmit(address(l1CrossDomainMessenger));
        emit SentMessageExtension1(address(l1StandardBridge), value);

        // deposit eth to bob
        vm.prank(alice, alice);
    }
}

contract L1StandardBridge_DepositETHTo_Test is PreBridgeETHTo {
    /// @dev Tests that depositing ETH to a different address succeeds.
    ///      Emits ETHDepositInitiated event.
    ///      Calls depositTransaction on the OptimismPortal.
    ///      EOA or contract can call depositETHTo.
    ///      ETH ends up in the optimismPortal.
    function test_depositETHTo_succeeds() external {
        _preBridgeETHTo({ isLegacy: true, value: 600 });
        l1StandardBridge.depositETHTo{ value: 600 }(bob, 60000, hex"dead");
        assertEq(address(optimismPortal).balance, 600);
    }
}

contract L1StandardBridge_DepositETHTo_TestFail is CommonTest {
    /// @dev Tests that depositETHTo reverts with custom gas token.
    function testFuzz_depositETHTo_customGasToken_reverts(
        uint256 _value,
        address _to,
        uint32 _minGasLimit,
        bytes calldata _extraData
    )
        external
    {
        vm.mockCall(
            address(systemConfig), abi.encodeCall(systemConfig.gasPayingToken, ()), abi.encode(address(1), uint8(2))
        );
        vm.deal(address(this), _value);
        vm.expectRevert("StandardBridge: cannot bridge ETH with custom gas token");

        l1StandardBridge.depositETHTo{ value: _value }(_to, _minGasLimit, _extraData);
    }
}

contract L1StandardBridge_BridgeETHTo_Test is PreBridgeETHTo {
    /// @dev Tests that bridging ETH to a different address succeeds.
    ///      Emits ETHDepositInitiated and ETHBridgeInitiated events.
    ///      Calls depositTransaction on the OptimismPortal.
    ///      Only EOA can call bridgeETHTo.
    ///      ETH ends up in the optimismPortal.
    function test_bridgeETHTo_succeeds() external {
        _preBridgeETHTo({ isLegacy: false, value: 600 });
        l1StandardBridge.bridgeETHTo{ value: 600 }(bob, 60000, hex"dead");
        assertEq(address(optimismPortal).balance, 600);
    }
}

contract L1StandardBridge_BridgeETHTo_TestFail is PreBridgeETHTo {
    /// @dev Tests that bridging reverts with custom gas token.
    function testFuzz_bridgeETHTo_customGasToken_reverts(
        uint256 _value,
        uint32 _minGasLimit,
        bytes calldata _extraData
    )
        external
    {
        vm.mockCall(
            address(systemConfig), abi.encodeCall(systemConfig.gasPayingToken, ()), abi.encode(address(1), uint8(2))
        );
        vm.deal(address(this), _value);
        vm.expectRevert("StandardBridge: cannot bridge ETH with custom gas token");

        l1StandardBridge.bridgeETHTo{ value: _value }(bob, _minGasLimit, _extraData);
    }
}

contract L1StandardBridge_DepositERC20_Test is CommonTest {
    using stdStorage for StdStorage;

    // depositERC20
    // - updates bridge.deposits
    // - emits ERC20DepositInitiated
    // - calls optimismPortal.depositTransaction
    // - only callable by EOA

    /// @dev Tests that depositing ERC20 to the bridge succeeds.
    ///      Bridge deposits are updated.
    ///      Emits ERC20DepositInitiated event.
    ///      Calls depositTransaction on the OptimismPortal.
    ///      Only EOA can call depositERC20.
    function test_depositERC20_succeeds() external {
        uint256 nonce = l1CrossDomainMessenger.messageNonce();
        uint256 version = 0; // Internal constant in the OptimismPortal: DEPOSIT_VERSION
        address l1MessengerAliased = AddressAliasHelper.applyL1ToL2Alias(address(l1CrossDomainMessenger));

        // Deal Alice's ERC20 State
        deal(address(L1Token), alice, 100000, true);
        vm.prank(alice);
        L1Token.approve(address(l1StandardBridge), type(uint256).max);

        // The l1StandardBridge should transfer alice's tokens to itself
        vm.expectCall(address(L1Token), abi.encodeCall(ERC20.transferFrom, (alice, address(l1StandardBridge), 100)));

        bytes memory message = abi.encodeCall(
            StandardBridge.finalizeBridgeERC20, (address(L2Token), address(L1Token), alice, alice, 100, hex"")
        );

        // the L1 bridge should call L1CrossDomainMessenger.sendMessage
        vm.expectCall(
            address(l1CrossDomainMessenger),
            abi.encodeCall(ICrossDomainMessenger.sendMessage, (address(l2StandardBridge), message, 10000))
        );

        bytes memory innerMessage = abi.encodeCall(
            ICrossDomainMessenger.relayMessage,
            (nonce, address(l1StandardBridge), address(l2StandardBridge), 0, 10000, message)
        );

        uint64 baseGas = l1CrossDomainMessenger.baseGas(message, 10000);
        vm.expectCall(
            address(optimismPortal),
            abi.encodeCall(
                IOptimismPortal.depositTransaction, (address(l2CrossDomainMessenger), 0, baseGas, false, innerMessage)
            )
        );

        bytes memory opaqueData = abi.encodePacked(uint256(0), uint256(0), baseGas, false, innerMessage);

        // Should emit both the bedrock and legacy events
        vm.expectEmit(address(l1StandardBridge));
        emit ERC20DepositInitiated(address(L1Token), address(L2Token), alice, alice, 100, hex"");

        vm.expectEmit(address(l1StandardBridge));
        emit ERC20BridgeInitiated(address(L1Token), address(L2Token), alice, alice, 100, hex"");

        // OptimismPortal emits a TransactionDeposited event on `depositTransaction` call
        vm.expectEmit(address(optimismPortal));
        emit TransactionDeposited(l1MessengerAliased, address(l2CrossDomainMessenger), version, opaqueData);

        // SentMessage event emitted by the CrossDomainMessenger
        vm.expectEmit(address(l1CrossDomainMessenger));
        emit SentMessage(address(l2StandardBridge), address(l1StandardBridge), message, nonce, 10000);

        // SentMessageExtension1 event emitted by the CrossDomainMessenger
        vm.expectEmit(address(l1CrossDomainMessenger));
        emit SentMessageExtension1(address(l1StandardBridge), 0);

        vm.prank(alice);
        l1StandardBridge.depositERC20(address(L1Token), address(L2Token), 100, 10000, hex"");
        assertEq(l1StandardBridge.deposits(address(L1Token), address(L2Token)), 100);
    }
}

contract L1StandardBridge_DepositERC20_TestFail is CommonTest {
    /// @dev Tests that depositing an ERC20 to the bridge reverts
    ///      if the caller is not an EOA.
    function test_depositERC20_notEoa_reverts() external {
        // turn alice into a contract
        vm.etch(alice, hex"ffff");

        vm.expectRevert("StandardBridge: function can only be called from an EOA");
        vm.prank(alice, alice);
        l1StandardBridge.depositERC20(address(0), address(0), 100, 100, hex"");
    }
}

contract L1StandardBridge_DepositERC20To_Test is CommonTest {
    /// @dev Tests that depositing ERC20 to the bridge succeeds when
    ///      sent to a different address.
    ///      Bridge deposits are updated.
    ///      Emits ERC20DepositInitiated event.
    ///      Calls depositTransaction on the OptimismPortal.
    ///      Contracts can call depositERC20.
    function test_depositERC20To_succeeds() external {
        uint256 nonce = l1CrossDomainMessenger.messageNonce();
        uint256 version = 0; // Internal constant in the OptimismPortal: DEPOSIT_VERSION
        address l1MessengerAliased = AddressAliasHelper.applyL1ToL2Alias(address(l1CrossDomainMessenger));

        bytes memory message = abi.encodeCall(
            StandardBridge.finalizeBridgeERC20, (address(L2Token), address(L1Token), alice, bob, 1000, hex"")
        );

        bytes memory innerMessage = abi.encodeCall(
            ICrossDomainMessenger.relayMessage,
            (nonce, address(l1StandardBridge), address(l2StandardBridge), 0, 10000, message)
        );

        uint64 baseGas = l1CrossDomainMessenger.baseGas(message, 10000);
        bytes memory opaqueData = abi.encodePacked(uint256(0), uint256(0), baseGas, false, innerMessage);

        deal(address(L1Token), alice, 100000, true);

        vm.prank(alice);
        L1Token.approve(address(l1StandardBridge), type(uint256).max);

        // Should emit both the bedrock and legacy events
        vm.expectEmit(address(l1StandardBridge));
        emit ERC20DepositInitiated(address(L1Token), address(L2Token), alice, bob, 1000, hex"");

        vm.expectEmit(address(l1StandardBridge));
        emit ERC20BridgeInitiated(address(L1Token), address(L2Token), alice, bob, 1000, hex"");

        // OptimismPortal emits a TransactionDeposited event on `depositTransaction` call
        vm.expectEmit(address(optimismPortal));
        emit TransactionDeposited(l1MessengerAliased, address(l2CrossDomainMessenger), version, opaqueData);

        // SentMessage event emitted by the CrossDomainMessenger
        vm.expectEmit(address(l1CrossDomainMessenger));
        emit SentMessage(address(l2StandardBridge), address(l1StandardBridge), message, nonce, 10000);

        // SentMessageExtension1 event emitted by the CrossDomainMessenger
        vm.expectEmit(address(l1CrossDomainMessenger));
        emit SentMessageExtension1(address(l1StandardBridge), 0);

        // the L1 bridge should call L1CrossDomainMessenger.sendMessage
        vm.expectCall(
            address(l1CrossDomainMessenger),
            abi.encodeCall(ICrossDomainMessenger.sendMessage, (address(l2StandardBridge), message, 10000))
        );
        // The L1 XDM should call OptimismPortal.depositTransaction
        vm.expectCall(
            address(optimismPortal),
            abi.encodeCall(
                IOptimismPortal.depositTransaction, (address(l2CrossDomainMessenger), 0, baseGas, false, innerMessage)
            )
        );
        vm.expectCall(address(L1Token), abi.encodeCall(ERC20.transferFrom, (alice, address(l1StandardBridge), 1000)));

        vm.prank(alice);
        l1StandardBridge.depositERC20To(address(L1Token), address(L2Token), bob, 1000, 10000, hex"");

        assertEq(l1StandardBridge.deposits(address(L1Token), address(L2Token)), 1000);
    }
}

contract L1StandardBridge_FinalizeETHWithdrawal_Test is CommonTest {
    using stdStorage for StdStorage;

    /// @dev Tests that finalizing an ETH withdrawal succeeds.
    ///      Emits ETHWithdrawalFinalized event.
    ///      Only callable by the L2 bridge.
    function test_finalizeETHWithdrawal_succeeds() external {
        uint256 aliceBalance = alice.balance;

        vm.expectEmit(address(l1StandardBridge));
        emit ETHWithdrawalFinalized(alice, alice, 100, hex"");

        vm.expectEmit(address(l1StandardBridge));
        emit ETHBridgeFinalized(alice, alice, 100, hex"");

        vm.expectCall(alice, hex"");

        vm.mockCall(
            address(l1StandardBridge.messenger()),
            abi.encodeCall(ICrossDomainMessenger.xDomainMessageSender, ()),
            abi.encode(address(l1StandardBridge.OTHER_BRIDGE()))
        );
        // ensure that the messenger has ETH to call with
        vm.deal(address(l1StandardBridge.messenger()), 100);
        vm.prank(address(l1StandardBridge.messenger()));
        l1StandardBridge.finalizeETHWithdrawal{ value: 100 }(alice, alice, 100, hex"");

        assertEq(address(l1StandardBridge.messenger()).balance, 0);
        assertEq(aliceBalance + 100, alice.balance);
    }
}

contract L1StandardBridge_FinalizeETHWithdrawal_TestFail is CommonTest {
    /// @dev Tests that finalizeETHWithdrawal reverts with custom gas token.
    function testFuzz_finalizeETHWithdrawal_customGasToken_reverts(
        uint256 _value,
        bytes calldata _extraData
    )
        external
    {
        vm.mockCall(
            address(systemConfig), abi.encodeCall(systemConfig.gasPayingToken, ()), abi.encode(address(1), uint8(2))
        );
        vm.mockCall(
            address(l1StandardBridge.messenger()),
            abi.encodeCall(ICrossDomainMessenger.xDomainMessageSender, ()),
            abi.encode(address(l1StandardBridge.OTHER_BRIDGE()))
        );
        vm.deal(address(l1StandardBridge.messenger()), _value);
        vm.prank(address(l1StandardBridge.messenger()));
        vm.expectRevert("StandardBridge: cannot bridge ETH with custom gas token");

        l1StandardBridge.finalizeETHWithdrawal{ value: _value }(alice, alice, _value, _extraData);
    }
}

contract L1StandardBridge_FinalizeERC20Withdrawal_Test is CommonTest {
    using stdStorage for StdStorage;

    /// @dev Tests that finalizing an ERC20 withdrawal succeeds.
    ///      Bridge deposits are updated.
    ///      Emits ERC20WithdrawalFinalized event.
    ///      Only callable by the L2 bridge.
    function test_finalizeERC20Withdrawal_succeeds() external {
        deal(address(L1Token), address(l1StandardBridge), 100, true);

        uint256 slot = stdstore.target(address(l1StandardBridge)).sig("deposits(address,address)").with_key(
            address(L1Token)
        ).with_key(address(L2Token)).find();

        // Give the L1 bridge some ERC20 tokens
        vm.store(address(l1StandardBridge), bytes32(slot), bytes32(uint256(100)));
        assertEq(l1StandardBridge.deposits(address(L1Token), address(L2Token)), 100);

        vm.expectEmit(address(l1StandardBridge));
        emit ERC20WithdrawalFinalized(address(L1Token), address(L2Token), alice, alice, 100, hex"");

        vm.expectEmit(address(l1StandardBridge));
        emit ERC20BridgeFinalized(address(L1Token), address(L2Token), alice, alice, 100, hex"");

        vm.expectCall(address(L1Token), abi.encodeCall(ERC20.transfer, (alice, 100)));

        vm.mockCall(
            address(l1StandardBridge.messenger()),
            abi.encodeCall(ICrossDomainMessenger.xDomainMessageSender, ()),
            abi.encode(address(l1StandardBridge.OTHER_BRIDGE()))
        );
        vm.prank(address(l1StandardBridge.messenger()));
        l1StandardBridge.finalizeERC20Withdrawal(address(L1Token), address(L2Token), alice, alice, 100, hex"");

        assertEq(L1Token.balanceOf(address(l1StandardBridge)), 0);
        assertEq(L1Token.balanceOf(address(alice)), 100);
    }
}

contract L1StandardBridge_FinalizeERC20Withdrawal_TestFail is CommonTest {
    /// @dev Tests that finalizing an ERC20 withdrawal reverts if the caller is not the L2 bridge.
    function test_finalizeERC20Withdrawal_notMessenger_reverts() external {
        vm.mockCall(
            address(l1StandardBridge.messenger()),
            abi.encodeCall(ICrossDomainMessenger.xDomainMessageSender, ()),
            abi.encode(address(l1StandardBridge.OTHER_BRIDGE()))
        );
        vm.prank(address(28));
        vm.expectRevert("StandardBridge: function can only be called from the other bridge");
        l1StandardBridge.finalizeERC20Withdrawal(address(L1Token), address(L2Token), alice, alice, 100, hex"");
    }

    /// @dev Tests that finalizing an ERC20 withdrawal reverts if the caller is not the L2 bridge.
    function test_finalizeERC20Withdrawal_notOtherBridge_reverts() external {
        vm.mockCall(
            address(l1StandardBridge.messenger()),
            abi.encodeCall(ICrossDomainMessenger.xDomainMessageSender, ()),
            abi.encode(address(address(0)))
        );
        vm.prank(address(l1StandardBridge.messenger()));
        vm.expectRevert("StandardBridge: function can only be called from the other bridge");
        l1StandardBridge.finalizeERC20Withdrawal(address(L1Token), address(L2Token), alice, alice, 100, hex"");
    }
}

contract L1StandardBridge_FinalizeBridgeETH_Test is CommonTest {
    /// @dev Tests that finalizing bridged ETH succeeds.
    function test_finalizeBridgeETH_succeeds() external {
        address messenger = address(l1StandardBridge.messenger());
        vm.mockCall(
            messenger,
            abi.encodeCall(ICrossDomainMessenger.xDomainMessageSender, ()),
            abi.encode(address(l1StandardBridge.OTHER_BRIDGE()))
        );
        vm.deal(messenger, 100);
        vm.prank(messenger);

        vm.expectEmit(address(l1StandardBridge));
        emit ETHBridgeFinalized(alice, alice, 100, hex"");

        l1StandardBridge.finalizeBridgeETH{ value: 100 }(alice, alice, 100, hex"");
    }
}

contract L1StandardBridge_FinalizeBridgeETH_TestFail is CommonTest {
    /// @dev Tests that finalizing bridged reverts with custom gas token.
    function testFuzz_finalizeBridgeETH_customGasToken_reverts(uint256 _value, bytes calldata _extraData) external {
        vm.mockCall(
            address(l1StandardBridge.messenger()),
            abi.encodeCall(ICrossDomainMessenger.xDomainMessageSender, ()),
            abi.encode(address(l1StandardBridge.OTHER_BRIDGE()))
        );
        vm.deal(address(l1CrossDomainMessenger), _value);
        vm.prank(address(l1CrossDomainMessenger));
        vm.mockCall(
            address(systemConfig), abi.encodeCall(systemConfig.gasPayingToken, ()), abi.encode(address(1), uint8(2))
        );
        vm.expectRevert("StandardBridge: cannot bridge ETH with custom gas token");

        l1StandardBridge.finalizeBridgeETH{ value: _value }(alice, alice, _value, _extraData);
    }

    /// @dev Tests that finalizing bridged ETH reverts if the amount is incorrect.
    function test_finalizeBridgeETH_incorrectValue_reverts() external {
        address messenger = address(l1StandardBridge.messenger());
        vm.mockCall(
            messenger,
            abi.encodeCall(ICrossDomainMessenger.xDomainMessageSender, ()),
            abi.encode(address(l1StandardBridge.OTHER_BRIDGE()))
        );
        vm.deal(messenger, 100);
        vm.prank(messenger);
        vm.expectRevert("StandardBridge: amount sent does not match amount required");
        l1StandardBridge.finalizeBridgeETH{ value: 50 }(alice, alice, 100, hex"");
    }

    /// @dev Tests that finalizing bridged ETH reverts if the destination is the L1 bridge.
    function test_finalizeBridgeETH_sendToSelf_reverts() external {
        address messenger = address(l1StandardBridge.messenger());
        vm.mockCall(
            messenger,
            abi.encodeCall(ICrossDomainMessenger.xDomainMessageSender, ()),
            abi.encode(address(l1StandardBridge.OTHER_BRIDGE()))
        );
        vm.deal(messenger, 100);
        vm.prank(messenger);
        vm.expectRevert("StandardBridge: cannot send to self");
        l1StandardBridge.finalizeBridgeETH{ value: 100 }(alice, address(l1StandardBridge), 100, hex"");
    }

    /// @dev Tests that finalizing bridged ETH reverts if the destination is the messenger.
    function test_finalizeBridgeETH_sendToMessenger_reverts() external {
        address messenger = address(l1StandardBridge.messenger());
        vm.mockCall(
            messenger,
            abi.encodeCall(ICrossDomainMessenger.xDomainMessageSender, ()),
            abi.encode(address(l1StandardBridge.OTHER_BRIDGE()))
        );
        vm.deal(messenger, 100);
        vm.prank(messenger);
        vm.expectRevert("StandardBridge: cannot send to messenger");
        l1StandardBridge.finalizeBridgeETH{ value: 100 }(alice, messenger, 100, hex"");
    }
}
