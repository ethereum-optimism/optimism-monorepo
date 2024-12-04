// SPDX-License-Identifier: MIT
pragma solidity 0.8.25;

// Contracts
import { ERC20 } from "@solady-v0.0.245/tokens/ERC20.sol";

// Libraries
import { Predeploys } from "src/libraries/Predeploys.sol";
import { Unauthorized } from "src/libraries/errors/CommonErrors.sol";

// Interfaces
import { IERC20 } from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import { ISemver } from "src/universal/interfaces/ISemver.sol";
import { IERC7802, IERC165 } from "src/L2/interfaces/IERC7802.sol";
import {AbstractSuperchainERC20} from "src/L2/AbstractSuperchainERC20.sol";

/// @title SuperchainERC20
/// @notice A standard ERC20 extension implementing IERC7802 for unified cross-chain fungibility across
///         the Superchain. Allows the SuperchainTokenBridge to mint and burn tokens as needed.
abstract contract SuperchainERC20 is ERC20, AbstractSuperchainERC20, ISemver {
    /// @notice Semantic version.
    /// @custom:semver 1.0.0-beta.7
    function version() external view virtual returns (string memory) {
        return "1.0.0-beta.7";
    }

    function _crosschainMint(address _to, uint256 _amount) internal virtual override {
        _mint(_to, _amount);
    }

    function _crosschainBurn(address _from, uint256 _amount) internal virtual override {
        _burn(_from, _amount);
    }

}
