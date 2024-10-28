// SPDX-License-Identifier: GPL-3.0
// Copyright (C) 2015, 2016, 2017 Dapphub

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

// Based on WETH9 by Dapphub.
// Modified by OP Labs.

pragma solidity 0.8.15;

/// @title WETH98
/// @notice WETH98 is a version of WETH9 upgraded for Solidity 0.8.x.
contract WETH98 {
    /// @notice Returns the number of decimals the token uses.
    /// @return The number of decimals the token uses.
    uint8 public constant decimals = 18;

    mapping(address => uint256) internal _balanceOf;
    mapping(address => mapping(address => uint256)) internal _allowance;

    /// @notice Emitted when an approval is made.
    /// @param src The address that approved the transfer.
    /// @param guy The address that was approved to transfer.
    /// @param wad The amount that was approved to transfer.
    event Approval(address indexed src, address indexed guy, uint256 wad);

    /// @notice Emitted when a transfer is made.
    /// @param src The address that transferred the WETH.
    /// @param dst The address that received the WETH.
    /// @param wad The amount of WETH that was transferred.
    event Transfer(address indexed src, address indexed dst, uint256 wad);

    /// @notice Emitted when a deposit is made.
    /// @param dst The address that deposited the WETH.
    /// @param wad The amount of WETH that was deposited.
    event Deposit(address indexed dst, uint256 wad);

    /// @notice Emitted when a withdrawal is made.
    /// @param src The address that withdrew the WETH.
    /// @param wad The amount of WETH that was withdrawn.
    event Withdrawal(address indexed src, uint256 wad);

    /// @notice Pipes to deposit.
    receive() external payable {
        deposit();
    }

    /// @notice Pipes to deposit.
    fallback() external payable {
        deposit();
    }

    /// @notice Returns the name of the token.
    /// @return name_ The name of the token.
    function name() external view virtual returns (string memory) {
        return "Wrapped Ether";
    }

    /// @notice Returns the symbol of the token.
    /// @return symbol_ The symbol of the token.
    function symbol() external view virtual returns (string memory) {
        return "WETH";
    }

    /// @notice Returns the amount of WETH that the spender can transfer on behalf of the owner.
    /// @param owner The address that owns the WETH.
    /// @param spender The address that is approved to transfer the WETH.
    /// @return The amount of WETH that the spender can transfer on behalf of the owner.
    function allowance(address owner, address spender) public view virtual returns (uint256) {
        return _allowance[owner][spender];
    }

    /// @notice Returns the balance of the given address.
    /// @param src The address to query the balance of.
    /// @return The balance of the given address.
    function balanceOf(address src) public view returns (uint256) {
        return _balanceOf[src];
    }

    /// @notice Allows WETH to be deposited by sending ether to the contract.
    function deposit() public payable virtual {
        _balanceOf[msg.sender] += msg.value;
        emit Deposit(msg.sender, msg.value);
    }

    /// @notice Withdraws an amount of ETH.
    /// @param wad The amount of ETH to withdraw.
    function withdraw(uint256 wad) public virtual {
        require(_balanceOf[msg.sender] >= wad);
        _balanceOf[msg.sender] -= wad;
        payable(msg.sender).transfer(wad);
        emit Withdrawal(msg.sender, wad);
    }

    /// @notice Returns the total supply of WETH.
    /// @return The total supply of WETH.
    function totalSupply() external view returns (uint256) {
        return address(this).balance;
    }

    /// @notice Approves the given address to transfer the WETH on behalf of the caller.
    /// @param guy The address that is approved to transfer the WETH.
    /// @param wad The amount that is approved to transfer.
    /// @return True if the approval was successful.
    function approve(address guy, uint256 wad) external returns (bool) {
        _allowance[msg.sender][guy] = wad;
        emit Approval(msg.sender, guy, wad);
        return true;
    }

    /// @notice Transfers the given amount of WETH to the given address.
    /// @param dst The address to transfer the WETH to.
    /// @param wad The amount of WETH to transfer.
    /// @return True if the transfer was successful.
    function transfer(address dst, uint256 wad) external returns (bool) {
        return transferFrom(msg.sender, dst, wad);
    }

    /// @notice Transfers the given amount of WETH from the given address to the given address.
    /// @param src The address to transfer the WETH from.
    /// @param dst The address to transfer the WETH to.
    /// @param wad The amount of WETH to transfer.
    /// @return True if the transfer was successful.
    function transferFrom(address src, address dst, uint256 wad) public returns (bool) {
        require(_balanceOf[src] >= wad);

        uint256 senderAllowance = allowance(src, msg.sender);
        if (src != msg.sender && senderAllowance != type(uint256).max) {
            require(senderAllowance >= wad);
            _allowance[src][msg.sender] -= wad;
        }

        _balanceOf[src] -= wad;
        _balanceOf[dst] += wad;

        emit Transfer(src, dst, wad);

        return true;
    }
}
