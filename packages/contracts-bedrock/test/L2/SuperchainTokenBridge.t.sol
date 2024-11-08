// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing utilities
import { CommonTest } from "test/setup/CommonTest.sol";

// Libraries
import { Predeploys } from "src/libraries/Predeploys.sol";
import { IL2ToL2CrossDomainMessenger } from "src/L2/interfaces/IL2ToL2CrossDomainMessenger.sol";

// Target contract
import { ISuperchainTokenBridge } from "src/L2/interfaces/ISuperchainTokenBridge.sol";
import { ISuperchainERC20 } from "src/L2/interfaces/ISuperchainERC20.sol";
import { IOptimismSuperchainERC20Factory } from "src/L2/interfaces/IOptimismSuperchainERC20Factory.sol";
import { IERC20 } from "@openzeppelin/contracts/interfaces/IERC20.sol";
import { IERC7802 } from "src/L2/interfaces/IERC7802.sol";

/// @title SuperchainTokenBridgeTest
/// @notice Contract for testing the SuperchainTokenBridge contract.
contract SuperchainTokenBridgeTest is CommonTest {
    address internal constant ZERO_ADDRESS = address(0);
    string internal constant NAME = "SuperchainERC20";
    string internal constant SYMBOL = "OSE";
    address internal constant REMOTE_TOKEN = address(0x123);

    event Transfer(address indexed from, address indexed to, uint256 value);

    event SendERC20(
        address indexed token, address indexed from, address indexed to, uint256 amount, uint256 destination
    );

    event RelayERC20(address indexed token, address indexed from, address indexed to, uint256 amount, uint256 source);

    ISuperchainERC20 public superchainERC20;

    /// @notice Sets up the test suite.
    function setUp() public override {
        super.enableInterop();
        super.setUp();

        superchainERC20 = ISuperchainERC20(
            IOptimismSuperchainERC20Factory(Predeploys.OPTIMISM_SUPERCHAIN_ERC20_FACTORY).deploy(
                REMOTE_TOKEN, NAME, SYMBOL, 18
            )
        );
    }

    /// @notice Helper function to setup a mock and expect a call to it.
    function _mockAndExpect(address _receiver, bytes memory _calldata, bytes memory _returned) internal {
        vm.mockCall(_receiver, _calldata, _returned);
        vm.expectCall(_receiver, _calldata);
    }

    /// @notice Tests the `sendERC20` function reverts when the address `_to` is zero.
    function testFuzz_sendERC20_zeroAddressTo_reverts(address _sender, uint256 _amount, uint256 _chainId) public {
        // Expect the revert with `ZeroAddress` selector
        vm.expectRevert(ISuperchainTokenBridge.ZeroAddress.selector);

        // Call the `sendERC20` function with the zero address as `_to`
        vm.prank(_sender);
        superchainTokenBridge.sendERC20(address(superchainERC20), ZERO_ADDRESS, _amount, _chainId);
    }

    /// @notice Tests the `sendERC20` function reverts when the `token` does not support the IERC7802 interface.
    function testFuzz_sendERC20_notSupportedIERC7802_reverts(
        address _token,
        address _sender,
        address _to,
        uint256 _amount,
        uint256 _chainId
    )
        public
    {
        vm.assume(_to != ZERO_ADDRESS);
        assumeAddressIsNot(_token, AddressType.Precompile, AddressType.ForgeAddress);

        // Mock the call over the `supportsInterface` function to return false
        vm.mockCall(
            _token, abi.encodeCall(ISuperchainERC20.supportsInterface, (type(IERC7802).interfaceId)), abi.encode(false)
        );

        // Expect the revert with `InvalidERC7802` selector
        vm.expectRevert(ISuperchainTokenBridge.InvalidERC7802.selector);

        // Call the `sendERC20` function
        vm.prank(_sender);
        superchainTokenBridge.sendERC20(_token, _to, _amount, _chainId);
    }

    /// @notice Tests the `sendERC20` function burns the sender tokens, sends the message, and emits the `SendERC20`
    /// event.
    function testFuzz_sendERC20_succeeds(
        address _sender,
        address _to,
        uint256 _amount,
        uint256 _chainId,
        bytes32 _msgHash
    )
        external
    {
        // Ensure `_sender` and `_to` is not the zero address
        vm.assume(_sender != ZERO_ADDRESS);
        vm.assume(_to != ZERO_ADDRESS);

        // Mint some tokens to the sender so then they can be sent
        vm.prank(Predeploys.SUPERCHAIN_TOKEN_BRIDGE);
        superchainERC20.crosschainMint(_sender, _amount);

        // Get the total supply and balance of `_sender` before the send to compare later on the assertions
        uint256 _totalSupplyBefore = IERC20(address(superchainERC20)).totalSupply();
        uint256 _senderBalanceBefore = IERC20(address(superchainERC20)).balanceOf(_sender);

        // Look for the emit of the `Transfer` event
        vm.expectEmit(address(superchainERC20));
        emit Transfer(_sender, ZERO_ADDRESS, _amount);

        // Look for the emit of the `SendERC20` event
        vm.expectEmit(address(superchainTokenBridge));
        emit SendERC20(address(superchainERC20), _sender, _to, _amount, _chainId);

        // Mock the call over the `sendMessage` function and expect it to be called properly
        bytes memory _message =
            abi.encodeCall(superchainTokenBridge.relayERC20, (address(superchainERC20), _sender, _to, _amount));
        _mockAndExpect(
            Predeploys.L2_TO_L2_CROSS_DOMAIN_MESSENGER,
            abi.encodeCall(
                IL2ToL2CrossDomainMessenger.sendMessage, (_chainId, address(superchainTokenBridge), _message)
            ),
            abi.encode(_msgHash)
        );

        // Call the `sendERC20` function
        vm.prank(_sender);
        bytes32 _returnedMsgHash = superchainTokenBridge.sendERC20(address(superchainERC20), _to, _amount, _chainId);

        // Check the message hash was generated correctly
        assertEq(_msgHash, _returnedMsgHash);

        // Check the total supply and balance of `_sender` after the send were updated correctly
        assertEq(IERC20(address(superchainERC20)).totalSupply(), _totalSupplyBefore - _amount);
        assertEq(IERC20(address(superchainERC20)).balanceOf(_sender), _senderBalanceBefore - _amount);
    }

    /// @notice Tests the `relayERC20` function reverts when the caller is not the L2ToL2CrossDomainMessenger.
    function testFuzz_relayERC20_notMessenger_reverts(
        address _token,
        address _caller,
        address _to,
        uint256 _amount
    )
        public
    {
        // Ensure the caller is not the messenger
        vm.assume(_caller != Predeploys.L2_TO_L2_CROSS_DOMAIN_MESSENGER);

        // Expect the revert with `Unauthorized` selector
        vm.expectRevert(ISuperchainTokenBridge.Unauthorized.selector);

        // Call the `relayERC20` function with the non-messenger caller
        vm.prank(_caller);
        superchainTokenBridge.relayERC20(_token, _caller, _to, _amount);
    }

    /// @notice Tests the `relayERC20` function reverts when the `crossDomainMessageSender` that sent the message is not
    /// the same SuperchainTokenBridge.
    function testFuzz_relayERC20_notCrossDomainSender_reverts(
        address _crossDomainMessageSender,
        uint256 _source,
        address _to,
        uint256 _amount
    )
        public
    {
        vm.assume(_crossDomainMessageSender != address(superchainTokenBridge));

        // Mock the call over the `crossDomainMessageContext` function setting a wrong sender
        vm.mockCall(
            Predeploys.L2_TO_L2_CROSS_DOMAIN_MESSENGER,
            abi.encodeCall(IL2ToL2CrossDomainMessenger.crossDomainMessageContext, ()),
            abi.encode(_crossDomainMessageSender, _source)
        );

        // Expect the revert with `InvalidCrossDomainSender` selector
        vm.expectRevert(ISuperchainTokenBridge.InvalidCrossDomainSender.selector);

        // Call the `relayERC20` function with the sender caller
        vm.prank(Predeploys.L2_TO_L2_CROSS_DOMAIN_MESSENGER);
        superchainTokenBridge.relayERC20(address(superchainERC20), _crossDomainMessageSender, _to, _amount);
    }

    /// @notice Tests the `relayERC20` mints the proper amount and emits the `RelayERC20` event.
    function testFuzz_relayERC20_succeeds(address _from, address _to, uint256 _amount, uint256 _source) public {
        vm.assume(_to != ZERO_ADDRESS);

        // Mock the call over the `crossDomainMessageContext` function setting the same address as value
        _mockAndExpect(
            Predeploys.L2_TO_L2_CROSS_DOMAIN_MESSENGER,
            abi.encodeCall(IL2ToL2CrossDomainMessenger.crossDomainMessageContext, ()),
            abi.encode(address(superchainTokenBridge), _source)
        );

        // Get the total supply and balance of `_to` before the relay to compare later on the assertions
        uint256 _totalSupplyBefore = IERC20(address(superchainERC20)).totalSupply();
        uint256 _toBalanceBefore = IERC20(address(superchainERC20)).balanceOf(_to);

        // Look for the emit of the `Transfer` event
        vm.expectEmit(address(superchainERC20));
        emit Transfer(ZERO_ADDRESS, _to, _amount);

        // Look for the emit of the `RelayERC20` event
        vm.expectEmit(address(superchainTokenBridge));
        emit RelayERC20(address(superchainERC20), _from, _to, _amount, _source);

        // Call the `relayERC20` function with the messenger caller
        vm.prank(Predeploys.L2_TO_L2_CROSS_DOMAIN_MESSENGER);
        superchainTokenBridge.relayERC20(address(superchainERC20), _from, _to, _amount);

        // Check the total supply and balance of `_to` after the relay were updated correctly
        assertEq(IERC20(address(superchainERC20)).totalSupply(), _totalSupplyBefore + _amount);
        assertEq(IERC20(address(superchainERC20)).balanceOf(_to), _toBalanceBefore + _amount);
    }
}
