// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { WETH98 } from "src/universal/WETH98.sol";

// Libraries
import { Predeploys } from "src/libraries/Predeploys.sol";

// Interfaces
import { ISemver } from "interfaces/universal/ISemver.sol";
import { IL1Block } from "interfaces/L2/IL1Block.sol";

/// @title WETH contract that reads the name and symbol from the L1Block contract.
///        Allows for nice rendering of token names for chains using custom gas token.
///        This contract is not proxied and contains calls to the custom gas token methods.
contract WETH is WETH98, ISemver {
    /// @custom:semver 1.1.0-beta.5
    string public constant version = "1.1.0-beta.5";

    /// @notice Returns the name of the wrapped native asset. Will be "Wrapped Ether"
    ///         if the native asset is Ether.
    function name() external pure override returns (string memory name_) {
        name_ = string.concat("Wrapped ", IL1Block(Predeploys.L1_BLOCK_ATTRIBUTES).gasPayingTokenName());
    }

    /// @notice Returns the symbol of the wrapped native asset. Will be "WETH" if the
    ///         native asset is Ether.
    function symbol() external pure override returns (string memory symbol_) {
        symbol_ = string.concat("W", IL1Block(Predeploys.L1_BLOCK_ATTRIBUTES).gasPayingTokenSymbol());
    }
}
