// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { ERC20 } from "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import { ERC20Burnable } from "@openzeppelin/contracts/token/ERC20/extensions/ERC20Burnable.sol";
import { ERC20Votes, ERC20Permit } from "@openzeppelin/contracts/token/ERC20/extensions/ERC20Votes.sol";
import { Ownable } from "@openzeppelin/contracts/access/Ownable.sol";

/// @custom:predeploy 0x4200000000000000000000000000000000000042
/// @title GovernanceToken
/// @notice The Optimism token used in governance and supporting voting and delegation. Implements
///         EIP 2612 allowing signed approvals. Contract is "owned" by a `MintManager` instance with
///         permission to the `mint` function only, for the purposes of enforcing the token
///         inflation schedule.
contract GovernanceToken is ERC20Burnable, ERC20Votes, Ownable {
    /// @notice Constructs the GovernanceToken contract.
    constructor() ERC20("Optimism", "OP") ERC20Permit("Optimism") { }

    /// @notice Allows the owner to mint tokens.
    /// @param _account The account receiving minted tokens.
    /// @param _amount  The amount of tokens to mint.
    function mint(address _account, uint256 _amount) public onlyOwner {
        _mint(_account, _amount);
    }

    /// @notice Callback called after a token transfer.
    /// @param from   The account sending tokens.
    /// @param to     The account receiving tokens.
    /// @param amount The amount of tokens being transfered.
    function _afterTokenTransfer(address from, address to, uint256 amount) internal override(ERC20, ERC20Votes) {
        super._afterTokenTransfer(from, to, amount);
    }

    /// @notice Internal mint function.
    /// @param to     The account receiving minted tokens.
    /// @param amount The amount of tokens to mint.
    function _mint(address to, uint256 amount) internal override(ERC20, ERC20Votes) {
        super._mint(to, amount);
    }

    /// @notice Internal burn function.
    /// @param account The account that tokens will be burned from.
    /// @param amount  The amount of tokens that will be burned.
    function _burn(address account, uint256 amount) internal override(ERC20, ERC20Votes) {
        super._burn(account, amount);
    }
}
