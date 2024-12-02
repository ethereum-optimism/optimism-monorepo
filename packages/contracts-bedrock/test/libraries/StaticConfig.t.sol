// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

// Testing utilities
import { Test } from "forge-std/Test.sol";
import { FFIInterface } from "test/setup/FFIInterface.sol";

// Target contract
import { StaticConfig } from "src/libraries/StaticConfig.sol";

contract StaticConfig_Test is Test {
    FFIInterface constant ffi = FFIInterface(address(uint160(uint256(keccak256(abi.encode("optimism.ffi"))))));

    function setUp() public {
        vm.etch(address(ffi), vm.getDeployedCode("FFIInterface.sol:FFIInterface"));
        vm.label(address(ffi), "FFIInterface");
    }

    /// @dev Tests set gas paying token encoding.
    function testDiff_encodeSetGasPayingToken_succeeds(
        address _token,
        uint8 _decimals,
        bytes32 _name,
        bytes32 _symbol
    )
        external
    {
        bytes memory encoding = StaticConfig.encodeSetGasPayingToken(_token, _decimals, _name, _symbol);

        bytes memory _encoding = ffi.encodeGasPayingToken(_token, _decimals, _name, _symbol);

        assertEq(encoding, _encoding);
    }

    /// @dev Tests set gas paying token decoding.
    function test_decodeSetGasPayingToken_succeeds(
        address _token,
        uint8 _decimals,
        bytes32 _name,
        bytes32 _symbol
    )
        external
    {
        bytes memory encoding = ffi.encodeGasPayingToken(_token, _decimals, _name, _symbol);

        (address token, uint8 decimals, bytes32 name, bytes32 symbol) = StaticConfig.decodeSetGasPayingToken(encoding);

        assertEq(token, _token);
        assertEq(decimals, _decimals);
        assertEq(name, _name);
        assertEq(symbol, _symbol);
    }

    /// @dev Tests add dependency encoding.
    function testDiff_encodeAddDependency_succeeds(uint256 _chainId) external {
        bytes memory encoding = StaticConfig.encodeAddDependency(_chainId);

        bytes memory _encoding = ffi.encodeDependency(_chainId);

        assertEq(encoding, _encoding);
    }

    /// @dev Tests add dependency decoding.
    function test_decodeAddDependency_succeeds(uint256 _chainId) external {
        bytes memory encoding = ffi.encodeDependency(_chainId);

        uint256 chainId = StaticConfig.decodeAddDependency(encoding);

        assertEq(chainId, _chainId);
    }

    /// @dev Tests remove dependency encoding.
    function testDiff_encodeRemoveDependency_succeeds(uint256 _chainId) external {
        bytes memory encoding = StaticConfig.encodeRemoveDependency(_chainId);

        bytes memory _encoding = ffi.encodeDependency(_chainId);

        assertEq(encoding, _encoding);
    }

    /// @dev Tests remove dependency decoding.
    function test_decodeRemoveDependency_succeeds(uint256 _chainId) external {
        bytes memory encoding = ffi.encodeDependency(_chainId);

        uint256 chainId = StaticConfig.decodeRemoveDependency(encoding);

        assertEq(chainId, _chainId);
    }
}
