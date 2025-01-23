// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { CommonTest } from "test/setup/CommonTest.sol";

// Libraries
import { Predeploys } from "src/libraries/Predeploys.sol";
import { Unauthorized, ZeroAddress } from "src/libraries/errors/CommonErrors.sol";
import { Preinstalls } from "src/libraries/Preinstalls.sol";

// Interfaces
import { IETHLiquidity } from "interfaces/L2/IETHLiquidity.sol";
import { ISuperchainWETH } from "interfaces/L2/ISuperchainWETH.sol";
import { IERC7802, IERC165 } from "interfaces/L2/IERC7802.sol";
import { IERC20 } from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import { IL2ToL2CrossDomainMessenger } from "interfaces/L2/IL2ToL2CrossDomainMessenger.sol";

/// @title SuperchainWETH_Test
/// @notice Contract for testing the SuperchainWETH contract.
contract SuperchainWETH_Test is CommonTest {
    /// @notice Emitted when a transfer is made.
    event Transfer(address indexed src, address indexed dst, uint256 wad);

    /// @notice Emitted when a deposit is made.
    event Deposit(address indexed dst, uint256 wad);

    /// @notice Emitted when a withdrawal is made.
    event Withdrawal(address indexed src, uint256 wad);

    /// @notice Emitted when a crosschain transfer mints tokens.
    event CrosschainMint(address indexed to, uint256 amount, address indexed sender);

    /// @notice Emitted when a crosschain transfer burns tokens.
    event CrosschainBurn(address indexed from, uint256 amount, address indexed sender);

    event SendETH(address indexed from, address indexed to, uint256 amount, uint256 destination);

    event RelayETH(address indexed from, address indexed to, uint256 amount, uint256 source);

    address internal constant ZERO_ADDRESS = address(0);

    /// @notice Test setup.
    function setUp() public virtual override {
        super.enableInterop();
        super.setUp();
    }

    /// @notice Helper function to setup a mock and expect a call to it.
    function _mockAndExpect(address _receiver, bytes memory _calldata, bytes memory _returned) internal {
        vm.mockCall(_receiver, _calldata, _returned);
        vm.expectCall(_receiver, _calldata);
    }

    /// @notice Tests that the deposit function can be called on a non-custom gas token chain.
    /// @param _amount The amount of WETH to send.
    function testFuzz_deposit_succeeds(uint256 _amount) public {
        // Assume
        _amount = bound(_amount, 0, type(uint248).max - 1);

        // Arrange
        vm.deal(alice, _amount);

        // Act
        vm.expectEmit(address(superchainWeth));
        emit Deposit(alice, _amount);
        vm.prank(alice);
        superchainWeth.deposit{ value: _amount }();

        // Assert
        assertEq(alice.balance, 0);
        assertEq(superchainWeth.balanceOf(alice), _amount);
    }

    /// @notice Tests that the withdraw function can be called on a non-custom gas token chain.
    /// @param _amount The amount of WETH to send.
    function testFuzz_withdraw_succeeds(uint256 _amount) public {
        // Assume
        _amount = bound(_amount, 0, type(uint248).max - 1);

        // Arrange
        vm.deal(alice, _amount);
        vm.prank(alice);
        superchainWeth.deposit{ value: _amount }();

        // Act
        vm.expectEmit(address(superchainWeth));
        emit Withdrawal(alice, _amount);
        vm.prank(alice);
        superchainWeth.withdraw(_amount);

        // Assert
        assertEq(alice.balance, _amount);
        assertEq(superchainWeth.balanceOf(alice), 0);
    }

    /// @notice Tests the `crosschainMint` function reverts when the caller is not the `SuperchainTokenBridge`.
    function testFuzz_crosschainMint_callerNotBridge_reverts(address _caller, address _to, uint256 _amount) public {
        // Ensure the caller is not the bridge
        vm.assume(_caller != Predeploys.SUPERCHAIN_TOKEN_BRIDGE);

        // Expect the revert with `Unauthorized` selector
        vm.expectRevert(ISuperchainWETH.Unauthorized.selector);

        // Call the `mint` function with the non-bridge caller
        vm.prank(_caller);
        superchainWeth.crosschainMint(_to, _amount);
    }

    /// @notice Tests the `crosschainMint` with non custom gas token succeeds and emits the `CrosschainMint` event.
    function testFuzz_crosschainMint_fromBridge_succeeds(address _to, uint256 _amount) public {
        // Ensure `_to` is not the zero address
        vm.assume(_to != ZERO_ADDRESS);
        _amount = bound(_amount, 0, type(uint248).max - 1);

        // Get the total supply and balance of `_to` before the mint to compare later on the assertions
        uint256 _totalSupplyBefore = superchainWeth.totalSupply();
        uint256 _toBalanceBefore = superchainWeth.balanceOf(_to);

        // Look for the emit of the `Transfer` event
        vm.expectEmit(address(superchainWeth));
        emit Transfer(ZERO_ADDRESS, _to, _amount);

        // Look for the emit of the `CrosschainMint` event
        vm.expectEmit(address(superchainWeth));
        emit CrosschainMint(_to, _amount, Predeploys.SUPERCHAIN_TOKEN_BRIDGE);

        // Expect the call to the `mint` function in the `ETHLiquidity` contract
        vm.expectCall(Predeploys.ETH_LIQUIDITY, abi.encodeCall(IETHLiquidity.mint, (_amount)), 1);

        // Call the `mint` function with the bridge caller
        vm.prank(Predeploys.SUPERCHAIN_TOKEN_BRIDGE);
        superchainWeth.crosschainMint(_to, _amount);

        // Check the total supply and balance of `_to` after the mint were updated correctly
        assertEq(superchainWeth.totalSupply(), _totalSupplyBefore + _amount);
        assertEq(superchainWeth.balanceOf(_to), _toBalanceBefore + _amount);
        assertEq(address(superchainWeth).balance, _amount);
    }

    /// @notice Tests the `crosschainBurn` function reverts when the caller is not the `SuperchainTokenBridge`.
    function testFuzz_crosschainBurn_callerNotBridge_reverts(address _caller, address _from, uint256 _amount) public {
        // Ensure the caller is not the bridge
        vm.assume(_caller != Predeploys.SUPERCHAIN_TOKEN_BRIDGE);

        // Expect the revert with `Unauthorized` selector
        vm.expectRevert(ISuperchainWETH.Unauthorized.selector);

        // Call the `burn` function with the non-bridge caller
        vm.prank(_caller);
        superchainWeth.crosschainBurn(_from, _amount);
    }

    /// @notice Tests the `crosschainBurn` with non custom gas token burns the amount and emits the `CrosschainBurn`
    /// event.
    function testFuzz_crosschainBurn_succeeds(address _from, uint256 _amount) public {
        // Ensure `_from` is not the zero address
        vm.assume(_from != ZERO_ADDRESS);
        _amount = bound(_amount, 0, type(uint248).max - 1);

        // Deposit some tokens to `_from` so then they can be burned
        vm.deal(_from, _amount);
        vm.prank(_from);
        superchainWeth.deposit{ value: _amount }();

        // Get the total supply and balance of `_from` before the burn to compare later on the assertions
        uint256 _totalSupplyBefore = superchainWeth.totalSupply();
        uint256 _fromBalanceBefore = superchainWeth.balanceOf(_from);

        // Look for the emit of the `Transfer` event
        vm.expectEmit(address(superchainWeth));
        emit Transfer(_from, ZERO_ADDRESS, _amount);

        // Look for the emit of the `CrosschainBurn` event
        vm.expectEmit(address(superchainWeth));
        emit CrosschainBurn(_from, _amount, Predeploys.SUPERCHAIN_TOKEN_BRIDGE);

        // Expect the call to the `burn` function in the `ETHLiquidity` contract
        vm.expectCall(Predeploys.ETH_LIQUIDITY, abi.encodeCall(IETHLiquidity.burn, ()), 1);

        // Call the `burn` function with the bridge caller
        vm.prank(Predeploys.SUPERCHAIN_TOKEN_BRIDGE);
        superchainWeth.crosschainBurn(_from, _amount);

        // Check the total supply and balance of `_from` after the burn were updated correctly
        assertEq(superchainWeth.totalSupply(), _totalSupplyBefore - _amount);
        assertEq(superchainWeth.balanceOf(_from), _fromBalanceBefore - _amount);
        assertEq(address(superchainWeth).balance, 0);
    }

    /// @notice Tests that the `crosschainBurn` function reverts when called with insufficient balance.
    function testFuzz_crosschainBurn_insufficientBalance_fails(address _from, uint256 _amount) public {
        // Assume
        vm.assume(_from != ZERO_ADDRESS);
        _amount = bound(_amount, 0, type(uint248).max - 1);

        // Arrange
        vm.deal(_from, _amount);
        vm.prank(_from);
        superchainWeth.deposit{ value: _amount }();

        // Act
        vm.expectRevert(); // nosemgrep: sol-safety-expectrevert-no-args
        superchainWeth.crosschainBurn(_from, _amount + 1);
    }

    /// @notice Test that the internal mint function reverts to protect against accidentally changing the visibility.
    function testFuzz_calling_internalMintFunction_reverts(address _caller, address _to, uint256 _amount) public {
        // Arrange
        // nosemgrep: sol-style-use-abi-encodecall
        bytes memory _calldata = abi.encodeWithSignature("_mint(address,uint256)", _to, _amount);
        vm.expectRevert(bytes(""));

        // Act
        vm.prank(_caller);
        (bool success,) = address(superchainWeth).call(_calldata);

        // Assert
        assertFalse(success);
    }

    /// @notice Test that the mint function reverts to protect against accidentally changing the visibility.
    function testFuzz_calling_mintFunction_reverts(address _caller, address _to, uint256 _amount) public {
        // Arrange
        // nosemgrep: sol-style-use-abi-encodecall
        bytes memory _calldata = abi.encodeWithSignature("mint(address,uint256)", _to, _amount);
        vm.expectRevert(bytes(""));

        // Act
        vm.prank(_caller);
        (bool success,) = address(superchainWeth).call(_calldata);

        // Assert
        assertFalse(success);
    }

    /// @notice Test that the internal burn function reverts to protect against accidentally changing the visibility.
    function testFuzz_calling_internalBurnFunction_reverts(address _caller, address _from, uint256 _amount) public {
        // Arrange
        // nosemgrep: sol-style-use-abi-encodecall
        bytes memory _calldata = abi.encodeWithSignature("_burn(address,uint256)", _from, _amount);
        vm.expectRevert(bytes(""));

        // Act
        vm.prank(_caller);
        (bool success,) = address(superchainWeth).call(_calldata);

        // Assert
        assertFalse(success);
    }

    /// @notice Test that the burn function reverts to protect against accidentally changing the visibility.
    function testFuzz_calling_burnFunction_reverts(address _caller, address _from, uint256 _amount) public {
        // Arrange
        // nosemgrep: sol-style-use-abi-encodecall
        bytes memory _calldata = abi.encodeWithSignature("burn(address,uint256)", _from, _amount);
        vm.expectRevert(bytes(""));

        // Act
        vm.prank(_caller);
        (bool success,) = address(superchainWeth).call(_calldata);

        // Assert
        assertFalse(success);
    }

    /// @notice Tests that the allowance function returns the max uint256 value when the spender is Permit.
    /// @param _randomCaller The address that will call the function - used to fuzz better since the behaviour should be
    ///                       the same regardless of the caller.
    /// @param _src The funds owner.
    function testFuzz_allowance_fromPermit2_succeeds(address _randomCaller, address _src) public {
        vm.prank(_randomCaller);
        uint256 _allowance = superchainWeth.allowance(_src, Preinstalls.Permit2);

        assertEq(_allowance, type(uint256).max);
    }

    /// @notice Tests that the allowance function returns the correct allowance when the spender is not Permit.
    /// @param _randomCaller The address that will call the function - used to fuzz better
    ///                       since the behaviour should be the same regardless of the caller.
    /// @param _src The funds owner.
    /// @param _guy The address of the spender - It cannot be Permit2.
    function testFuzz_allowance_succeeds(address _randomCaller, address _src, address _guy, uint256 _wad) public {
        // Assume
        vm.assume(_guy != Preinstalls.Permit2);

        // Arrange
        vm.prank(_src);
        superchainWeth.approve(_guy, _wad);

        // Act
        vm.prank(_randomCaller);
        uint256 _allowance = superchainWeth.allowance(_src, _guy);

        // Assert
        assertEq(_allowance, _wad);
    }

    /// @notice Tests that `transferFrom` works when the caller (spender) is Permit2, without any explicit approval.
    /// @param _src The funds owner.
    /// @param _dst The address of the recipient.
    /// @param _wad The amount of WETH to transfer.
    function testFuzz_transferFrom_whenPermit2IsCaller_succeeds(address _src, address _dst, uint256 _wad) public {
        vm.assume(_src != _dst);

        // Arrange
        deal(address(superchainWeth), _src, _wad);

        vm.expectEmit(address(superchainWeth));
        emit Transfer(_src, _dst, _wad);

        // Act
        vm.prank(Preinstalls.Permit2);
        superchainWeth.transferFrom(_src, _dst, _wad);

        // Assert
        assertEq(superchainWeth.balanceOf(_src), 0);
        assertEq(superchainWeth.balanceOf(_dst), _wad);
    }

    /// @notice Tests that `transferFrom` works when the caller (spender) is Permit2, and `_src` equals `_dst` without
    ///         an explicit approval.
    ///         The balance should remain the same on this scenario.
    /// @param _user The source and destination address.
    /// @param _wad The amount of WETH to transfer.
    function testFuzz_transferFrom_whenPermit2IsCallerAndSourceIsDestination_succeeds(
        address _user,
        uint256 _wad
    )
        public
    {
        // Arrange
        deal(address(superchainWeth), _user, _wad);

        vm.expectEmit(address(superchainWeth));
        emit Transfer(_user, _user, _wad);

        // Act
        vm.prank(Preinstalls.Permit2);
        superchainWeth.transferFrom(_user, _user, _wad);

        // Assert
        assertEq(superchainWeth.balanceOf(_user), _wad);
    }

    /// @notice Tests that the `supportsInterface` function returns true for the `IERC7802` interface.
    function test_supportInterface_succeeds() public view {
        assertTrue(superchainWeth.supportsInterface(type(IERC165).interfaceId));
        assertTrue(superchainWeth.supportsInterface(type(IERC7802).interfaceId));
        assertTrue(superchainWeth.supportsInterface(type(IERC20).interfaceId));
    }

    /// @notice Tests that the `supportsInterface` function returns false for any other interface than the
    /// `IERC7802` one.
    function testFuzz_supportInterface_works(bytes4 _interfaceId) public view {
        vm.assume(_interfaceId != type(IERC165).interfaceId);
        vm.assume(_interfaceId != type(IERC7802).interfaceId);
        vm.assume(_interfaceId != type(IERC20).interfaceId);
        assertFalse(superchainWeth.supportsInterface(_interfaceId));
    }

    /// @notice Tests the `sendETH` function reverts when the address `_to` is zero.
    function testFuzz_sendETH_zeroAddressTo_reverts(address _sender, uint256 _amount, uint256 _chainId) public {
        // Expect the revert with `ZeroAddress` selector
        vm.expectRevert(ZeroAddress.selector);

        vm.deal(_sender, _amount);
        vm.prank(_sender);
        // Call the `sendETH` function with the zero address as `_to`
        superchainWeth.sendETH{ value: _amount }(ZERO_ADDRESS, _chainId);
    }

    /// @notice Tests the `sendETH` function burns the sender ETH, sends the message, and emits the `SendETH`
    /// event.
    function testFuzz_sendETH_succeeds(
        address _sender,
        address _to,
        uint256 _amount,
        uint256 _chainId,
        bytes32 _msgHash
    )
        external
    {
        // Assume
        vm.assume(_sender != address(ethLiquidity));
        vm.assume(_sender != ZERO_ADDRESS);
        vm.assume(_to != ZERO_ADDRESS);
        _amount = bound(_amount, 0, type(uint248).max - 1);

        // Arrange
        vm.deal(_sender, _amount);

        // Get the total balance of `_sender` before the send to compare later on the assertions
        uint256 _senderBalanceBefore = _sender.balance;

        // Look for the emit of the `SendETH` event
        vm.expectEmit(address(superchainWeth));
        emit SendETH(_sender, _to, _amount, _chainId);

        // Expect the call to the `burn` function in the `ETHLiquidity` contract
        vm.expectCall(Predeploys.ETH_LIQUIDITY, abi.encodeCall(IETHLiquidity.burn, ()), 1);

        // Mock the call over the `sendMessage` function and expect it to be called properly
        bytes memory _message = abi.encodeCall(superchainWeth.relayETH, (_sender, _to, _amount));
        _mockAndExpect(
            Predeploys.L2_TO_L2_CROSS_DOMAIN_MESSENGER,
            abi.encodeCall(IL2ToL2CrossDomainMessenger.sendMessage, (_chainId, address(superchainWeth), _message)),
            abi.encode(_msgHash)
        );

        // Call the `sendETH` function
        vm.prank(_sender);
        bytes32 _returnedMsgHash = superchainWeth.sendETH{ value: _amount }(_to, _chainId);

        // Check the message hash was generated correctly
        assertEq(_msgHash, _returnedMsgHash);

        // Check the total supply and balance of `_sender` after the send were updated correctly
        assertEq(_sender.balance, _senderBalanceBefore - _amount);
    }

    /// @notice Tests the `relayETH` function reverts when the caller is not the L2ToL2CrossDomainMessenger.
    function testFuzz_relayETH_notMessenger_reverts(address _caller, address _to, uint256 _amount) public {
        // Ensure the caller is not the messenger
        vm.assume(_caller != Predeploys.L2_TO_L2_CROSS_DOMAIN_MESSENGER);

        // Expect the revert with `Unauthorized` selector
        vm.expectRevert(Unauthorized.selector);

        // Call the `relayETH` function with the non-messenger caller
        vm.prank(_caller);
        superchainWeth.relayETH(_caller, _to, _amount);
    }

    /// @notice Tests the `relayETH` function reverts when the `crossDomainMessageSender` that sent the message is not
    /// the same SuperchainWETH.
    function testFuzz_relayETH_notCrossDomainSender_reverts(
        address _crossDomainMessageSender,
        uint256 _source,
        address _to,
        uint256 _amount
    )
        public
    {
        vm.assume(_crossDomainMessageSender != address(superchainWeth));

        // Mock the call over the `crossDomainMessageContext` function setting a wrong sender
        vm.mockCall(
            Predeploys.L2_TO_L2_CROSS_DOMAIN_MESSENGER,
            abi.encodeCall(IL2ToL2CrossDomainMessenger.crossDomainMessageContext, ()),
            abi.encode(_crossDomainMessageSender, _source)
        );

        // Expect the revert with `InvalidCrossDomainSender` selector
        vm.expectRevert(ISuperchainWETH.InvalidCrossDomainSender.selector);

        // Call the `relayETH` function with the sender caller
        vm.prank(Predeploys.L2_TO_L2_CROSS_DOMAIN_MESSENGER);
        superchainWeth.relayETH(_crossDomainMessageSender, _to, _amount);
    }

    /// @notice Tests the `relayETH` function relays the proper amount of ETH and emits the `RelayETH` event.
    function testFuzz_relayETH_succeeds(address _from, address _to, uint256 _amount, uint256 _source) public {
        // Assume
        vm.assume(_to != ZERO_ADDRESS);
        assumePayable(_to);
        _amount = bound(_amount, 0, type(uint248).max - 1);

        // Arrange
        vm.deal(address(superchainWeth), _amount);
        vm.deal(Predeploys.ETH_LIQUIDITY, _amount);
        _mockAndExpect(
            Predeploys.L2_TO_L2_CROSS_DOMAIN_MESSENGER,
            abi.encodeCall(IL2ToL2CrossDomainMessenger.crossDomainMessageContext, ()),
            abi.encode(address(superchainWeth), _source)
        );

        uint256 _toBalanceBefore = _to.balance;

        // Look for the emit of the `RelayETH` event
        vm.expectEmit(address(superchainWeth));
        emit RelayETH(_from, _to, _amount, _source);

        // Expect the call to the `mint` function in the `ETHLiquidity` contract
        vm.expectCall(Predeploys.ETH_LIQUIDITY, abi.encodeCall(IETHLiquidity.mint, (_amount)), 1);

        // Call the `RelayETH` function with the messenger caller
        vm.prank(Predeploys.L2_TO_L2_CROSS_DOMAIN_MESSENGER);
        superchainWeth.relayETH(_from, _to, _amount);

        assertEq(_to.balance, _toBalanceBefore + _amount);
    }
}
