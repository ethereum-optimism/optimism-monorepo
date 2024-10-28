// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing utilities
import { CommonTest } from "test/setup/CommonTest.sol";

// Target contract
import { WETH } from "src/L2/WETH.sol";

contract WETH_Test is CommonTest {
    /// @dev Tests that the name function returns the correct value.
    function testFuzz_name_succeeds(string memory _gasPayingTokenName) external {
        vm.mockCall(address(l1Block), abi.encodeCall(l1Block.gasPayingTokenName, ()), abi.encode(_gasPayingTokenName));

        assertEq(string.concat("Wrapped ", _gasPayingTokenName), weth.name());
    }

    /// @dev Tests that the symbol function returns the correct value.
    function testFuzz_symbol_succeeds(string memory _gasPayingTokenSymbol) external {
        vm.mockCall(
            address(l1Block), abi.encodeCall(l1Block.gasPayingTokenSymbol, ()), abi.encode(_gasPayingTokenSymbol)
        );

        assertEq(string.concat("W", _gasPayingTokenSymbol), weth.symbol());
    }

    /// @dev Tests that the name function returns the correct value.
    function test_name_ether_succeeds() external view {
        assertFalse(l1Block.isCustomGasToken());
        assertEq("Wrapped Ether", weth.name());
    }

    /// @dev Tests that the symbol function returns the correct value.
    function test_symbol_ether_succeeds() external view {
        assertFalse(l1Block.isCustomGasToken());
        assertEq("WETH", weth.symbol());
    }
}
