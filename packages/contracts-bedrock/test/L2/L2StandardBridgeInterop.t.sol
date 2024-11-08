// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { CommonTest } from "test/setup/CommonTest.sol";

// Interfaces
import { IMintableAndBurnableERC20 } from "src/L2/interfaces/IMintableAndBurnableERC20.sol";
import { IL2StandardBridgeInterop } from "src/L2/interfaces/IL2StandardBridgeInterop.sol";
import { IERC20Metadata } from "@openzeppelin/contracts/token/ERC20/extensions/IERC20Metadata.sol";
import { IERC165 } from "@openzeppelin/contracts/utils/introspection/IERC165.sol";
import { IOptimismMintableERC20 } from "src/universal/interfaces/IOptimismMintableERC20.sol";
import { ILegacyMintableERC20 } from "src/universal/OptimismMintableERC20.sol";
import { IOptimismERC20Factory } from "src/L2/interfaces/IOptimismERC20Factory.sol";

contract L2StandardBridgeInterop_Test is CommonTest {
    /// @notice Emitted when a conversion is made.
    event Converted(address indexed from, address indexed to, address indexed caller, uint256 amount);

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

    /// @notice Mock ERC20 decimals
    function _mockDecimals(address _token, uint8 _decimals) internal {
        _mockAndExpect(_token, abi.encodeCall(IERC20Metadata.decimals, ()), abi.encode(_decimals));
    }

    /// @notice Mock ERC165 interface
    function _mockInterface(address _token, bytes4 _interfaceId, bool _supported) internal {
        _mockAndExpect(_token, abi.encodeCall(IERC165.supportsInterface, (_interfaceId)), abi.encode(_supported));
    }

    /// @notice Mock factory deployment
    function _mockDeployments(address _factory, address _token, address _deployed) internal {
        _mockAndExpect(_factory, abi.encodeCall(IOptimismERC20Factory.deployments, (_token)), abi.encode(_deployed));
    }

    /// @notice Assume a valid address for fuzzing
    function _assumeAddress(address _address) internal {
        assumeAddressIsNot(_address, AddressType.Precompile, AddressType.ForgeAddress);
    }
}

/// @notice Test suite when converting from a legacy token to a OptimismSuperchainERC20 token
contract L2StandardBridgeInterop_LegacyToSuper_Test is L2StandardBridgeInterop_Test {
    /// @notice Set up the test for converting from a legacy token to a OptimismSuperchainERC20 token
    function _setUpLegacyToSuper(address _from, address _to) internal {
        // Assume
        _assumeAddress(_from);
        _assumeAddress(_to);

        // Mock same decimals
        _mockDecimals(_from, 18);
        _mockDecimals(_to, 18);

        // Mock `_from` to be a legacy address
        _mockInterface(_from, type(IERC165).interfaceId, true);
        _mockInterface(_from, type(ILegacyMintableERC20).interfaceId, true);
    }

    /// @notice Test that the `convert` function with different decimals reverts
    function testFuzz_convert_differentDecimals_reverts(
        address _from,
        uint8 _decimalsFrom,
        address _to,
        uint8 _decimalsTo,
        uint256 _amount
    )
        public
    {
        // Assume
        _assumeAddress(_from);
        _assumeAddress(_to);
        vm.assume(_decimalsFrom != _decimalsTo);
        vm.assume(_from != _to);

        // Arrange
        // Mock the tokens to have different decimals
        _mockDecimals(_from, _decimalsFrom);
        _mockDecimals(_to, _decimalsTo);

        // Expect the revert with `InvalidDecimals` selector
        vm.expectRevert(IL2StandardBridgeInterop.InvalidDecimals.selector);

        // Act
        l2StandardBridge.convert(_from, _to, _amount);
    }

    /// @notice Test that the `convert` function with an invalid legacy ERC20 address reverts
    function testFuzz_convert_invalidLegacyERC20Address_reverts(address _from, address _to, uint256 _amount) public {
        // Arrange
        _setUpLegacyToSuper(_from, _to);

        // Mock the legacy factory to return address(0)
        _mockDeployments(address(l2OptimismMintableERC20Factory), _from, address(0));

        // Expect the revert with `InvalidLegacyERC20Address` selector
        vm.expectRevert(IL2StandardBridgeInterop.InvalidLegacyERC20Address.selector);

        // Act
        l2StandardBridge.convert(_from, _to, _amount);
    }

    /// @notice Test that the `convert` function with an invalid superchain ERC20 address reverts
    function testFuzz_convert_invalidSuperchainERC20Address_reverts(
        address _from,
        address _to,
        uint256 _amount,
        address _remoteToken
    )
        public
    {
        // Assume
        vm.assume(_remoteToken != address(0));

        // Arrange
        _setUpLegacyToSuper(_from, _to);

        // Mock the legacy factory to return `_remoteToken`
        _mockDeployments(address(l2OptimismMintableERC20Factory), _from, _remoteToken);

        // Mock the superchain factory to return address(0)
        _mockDeployments(address(l2OptimismSuperchainERC20Factory), _to, address(0));

        // Expect the revert with `InvalidSuperchainERC20Address` selector
        vm.expectRevert(IL2StandardBridgeInterop.InvalidSuperchainERC20Address.selector);

        // Act
        l2StandardBridge.convert(_from, _to, _amount);
    }

    /// @notice Test that the `convert` function with different remote tokens reverts
    function testFuzz_convert_differentRemoteAddresses_reverts(
        address _from,
        address _to,
        uint256 _amount,
        address _fromRemoteToken,
        address _toRemoteToken
    )
        public
    {
        // Assume
        vm.assume(_fromRemoteToken != address(0));
        vm.assume(_toRemoteToken != address(0));
        vm.assume(_fromRemoteToken != _toRemoteToken);

        // Arrange
        _setUpLegacyToSuper(_from, _to);

        // Mock the legacy factory to return `_fromRemoteToken`
        _mockDeployments(address(l2OptimismMintableERC20Factory), _from, _fromRemoteToken);

        // Mock the superchain factory to return `_toRemoteToken`
        _mockDeployments(address(l2OptimismSuperchainERC20Factory), _to, _toRemoteToken);

        // Expect the revert with `InvalidTokenPair` selector
        vm.expectRevert(IL2StandardBridgeInterop.InvalidTokenPair.selector);

        // Act
        l2StandardBridge.convert(_from, _to, _amount);
    }

    /// @notice Test that the `convert` function succeeds
    function testFuzz_convert_succeeds(
        address _caller,
        address _from,
        address _to,
        uint256 _amount,
        address _remoteToken
    )
        public
    {
        // Assume
        vm.assume(_remoteToken != address(0));

        // Arrange
        _setUpLegacyToSuper(_from, _to);

        // Mock the legacy and superchain factory to return `_remoteToken`
        _mockDeployments(address(l2OptimismMintableERC20Factory), _from, _remoteToken);
        _mockDeployments(address(l2OptimismSuperchainERC20Factory), _to, _remoteToken);

        // Expect the `Converted` event to be emitted
        vm.expectEmit(address(l2StandardBridge));
        emit Converted(_from, _to, _caller, _amount);

        // Mock and expect the `burn` and `mint` functions
        _mockAndExpect(_from, abi.encodeCall(IMintableAndBurnableERC20.burn, (_caller, _amount)), abi.encode());
        _mockAndExpect(_to, abi.encodeCall(IMintableAndBurnableERC20.mint, (_caller, _amount)), abi.encode());

        // Act
        vm.prank(_caller);
        l2StandardBridge.convert(_from, _to, _amount);
    }
}

/// @notice Test suite when converting from a OptimismSuperchainERC20 token to a legacy token
contract L2StandardBridgeInterop_SuperToLegacy_Test is L2StandardBridgeInterop_Test {
    /// @notice Set up the test for converting from a OptimismSuperchainERC20 token to a legacy token
    function _setUpSuperToLegacy(address _from, address _to) internal {
        // Assume
        _assumeAddress(_from);
        _assumeAddress(_to);

        // Mock same decimals
        _mockDecimals(_from, 18);
        _mockDecimals(_to, 18);

        // Mock `_from` so it is not a LegacyMintableERC20 address
        _mockInterface(_from, type(IERC165).interfaceId, true);
        _mockInterface(_from, type(ILegacyMintableERC20).interfaceId, false);
        _mockInterface(_from, type(IOptimismMintableERC20).interfaceId, false);
    }

    /// @notice Test that the `convert` function with different decimals reverts
    function testFuzz_convert_differentDecimals_reverts(
        address _from,
        uint8 _decimalsFrom,
        address _to,
        uint8 _decimalsTo,
        uint256 _amount
    )
        public
    {
        // Assume
        _assumeAddress(_from);
        _assumeAddress(_to);
        vm.assume(_decimalsFrom != _decimalsTo);
        vm.assume(_from != _to);

        // Arrange
        // Mock the tokens to have different decimals
        _mockDecimals(_from, _decimalsFrom);
        _mockDecimals(_to, _decimalsTo);

        // Expect the revert with `InvalidDecimals` selector
        vm.expectRevert(IL2StandardBridgeInterop.InvalidDecimals.selector);

        // Act
        l2StandardBridge.convert(_from, _to, _amount);
    }

    /// @notice Test that the `convert` function with an invalid legacy ERC20 address reverts
    function testFuzz_convert_invalidLegacyERC20Address_reverts(address _from, address _to, uint256 _amount) public {
        // Arrange
        _setUpSuperToLegacy(_from, _to);

        // Mock the legacy factory to return address(0)
        _mockDeployments(address(l2OptimismMintableERC20Factory), _to, address(0));

        // Expect the revert with `InvalidLegacyERC20Address` selector
        vm.expectRevert(IL2StandardBridgeInterop.InvalidLegacyERC20Address.selector);

        // Act
        l2StandardBridge.convert(_from, _to, _amount);
    }

    /// @notice Test that the `convert` function with an invalid superchain ERC20 address reverts
    function testFuzz_convert_invalidSuperchainERC20Address_reverts(
        address _from,
        address _to,
        uint256 _amount,
        address _remoteToken
    )
        public
    {
        // Assume
        vm.assume(_remoteToken != address(0));

        // Arrange
        _setUpSuperToLegacy(_from, _to);

        // Mock the legacy factory to return `_remoteToken`
        _mockDeployments(address(l2OptimismMintableERC20Factory), _to, _remoteToken);

        // Mock the superchain factory to return address(0)
        _mockDeployments(address(l2OptimismSuperchainERC20Factory), _from, address(0));

        // Expect the revert with `InvalidSuperchainERC20Address` selector
        vm.expectRevert(IL2StandardBridgeInterop.InvalidSuperchainERC20Address.selector);

        // Act
        l2StandardBridge.convert(_from, _to, _amount);
    }

    /// @notice Test that the `convert` function with different remote tokens reverts
    function testFuzz_convert_differentRemoteAddresses_reverts(
        address _from,
        address _to,
        uint256 _amount,
        address _fromRemoteToken,
        address _toRemoteToken
    )
        public
    {
        // Assume
        vm.assume(_fromRemoteToken != address(0));
        vm.assume(_toRemoteToken != address(0));
        vm.assume(_fromRemoteToken != _toRemoteToken);

        // Arrange
        _setUpSuperToLegacy(_from, _to);

        // Mock the legacy factory to return `_fromRemoteToken`
        _mockDeployments(address(l2OptimismMintableERC20Factory), _to, _fromRemoteToken);

        // Mock the superchain factory to return `_toRemoteToken`
        _mockDeployments(address(l2OptimismSuperchainERC20Factory), _from, _toRemoteToken);

        // Expect the revert with `InvalidTokenPair` selector
        vm.expectRevert(IL2StandardBridgeInterop.InvalidTokenPair.selector);

        // Act
        l2StandardBridge.convert(_from, _to, _amount);
    }

    /// @notice Test that the `convert` function succeeds
    function testFuzz_convert_succeeds(
        address _caller,
        address _from,
        address _to,
        uint256 _amount,
        address _remoteToken
    )
        public
    {
        // Assume
        vm.assume(_remoteToken != address(0));

        // Arrange
        _setUpSuperToLegacy(_from, _to);

        // Mock the legacy and superchain factory to return `_remoteToken`
        _mockDeployments(address(l2OptimismMintableERC20Factory), _to, _remoteToken);
        _mockDeployments(address(l2OptimismSuperchainERC20Factory), _from, _remoteToken);

        // Expect the `Converted` event to be emitted
        vm.expectEmit(address(l2StandardBridge));
        emit Converted(_from, _to, _caller, _amount);

        // Mock and expect the `burn` and `mint` functions
        _mockAndExpect(_from, abi.encodeCall(IMintableAndBurnableERC20.burn, (_caller, _amount)), abi.encode());
        _mockAndExpect(_to, abi.encodeCall(IMintableAndBurnableERC20.mint, (_caller, _amount)), abi.encode());

        // Act
        vm.prank(_caller);
        l2StandardBridge.convert(_from, _to, _amount);
    }
}
