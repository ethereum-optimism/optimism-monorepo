// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

// Contracts
import { ERC20 } from "@rari-capital/solmate/src/tokens/ERC20.sol";
import { ERC721 } from "@rari-capital/solmate/src/tokens/ERC721.sol";
import { Transactor } from "src/periphery/Transactor.sol";

/// @title AssetReceiver
/// @notice AssetReceiver is a minimal contract for receiving funds assets in the form of either
///         ETH, ERC20 tokens, or ERC721 tokens. Only the contract owner may withdraw the assets.
contract AssetReceiver is Transactor {
    /// @notice Emitted when ETH is received by this address.
    /// @param from   Address that sent ETH to this contract.
    /// @param amount Amount of ETH received.
    event ReceivedETH(address indexed from, uint256 amount);

    /// @notice Emitted when ETH is withdrawn from this address.
    /// @param withdrawer Address that triggered the withdrawal.
    /// @param recipient  Address that received the withdrawal.
    /// @param amount     ETH amount withdrawn.
    event WithdrewETH(address indexed withdrawer, address indexed recipient, uint256 amount);

    /// @notice Emitted when ERC20 tokens are withdrawn from this address.
    /// @param withdrawer Address that triggered the withdrawal.
    /// @param recipient  Address that received the withdrawal.
    /// @param asset      Address of the token being withdrawn.
    /// @param amount     ERC20 amount withdrawn.
    event WithdrewERC20(address indexed withdrawer, address indexed recipient, address indexed asset, uint256 amount);

    /// @notice Emitted when ERC20 tokens are withdrawn from this address.
    /// @param withdrawer Address that triggered the withdrawal.
    /// @param recipient  Address that received the withdrawal.
    /// @param asset      Address of the token being withdrawn.
    /// @param id         Token ID being withdrawn.
    event WithdrewERC721(address indexed withdrawer, address indexed recipient, address indexed asset, uint256 id);

    /// @param _owner Initial contract owner.
    constructor(address _owner) Transactor(_owner) { }

    /// @notice Make sure we can receive ETH.
    receive() external payable {
        emit ReceivedETH(msg.sender, msg.value);
    }

    /// @notice Withdraws full ETH balance to the recipient.
    /// @param _to Address to receive the ETH balance.
    function withdrawETH(address payable _to) external onlyOwner {
        withdrawETH(_to, address(this).balance);
    }

    /// @notice Withdraws partial ETH balance to the recipient.
    /// @param _to     Address to receive the ETH balance.
    /// @param _amount Amount of ETH to withdraw.
    function withdrawETH(address payable _to, uint256 _amount) public onlyOwner {
        // slither-disable-next-line reentrancy-unlimited-gas
        (bool success,) = _to.call{ value: _amount }("");
        success; // Suppress warning; We ignore the low-level call result.
        emit WithdrewETH(msg.sender, _to, _amount);
    }

    /// @notice Withdraws full ERC20 balance to the recipient.
    /// @param _asset ERC20 token to withdraw.
    /// @param _to    Address to receive the ERC20 balance.
    function withdrawERC20(ERC20 _asset, address _to) external onlyOwner {
        withdrawERC20(_asset, _to, _asset.balanceOf(address(this)));
    }

    /// @notice Withdraws partial ERC20 balance to the recipient.
    /// @param _asset  ERC20 token to withdraw.
    /// @param _to     Address to receive the ERC20 balance.
    /// @param _amount Amount of ERC20 to withdraw.
    function withdrawERC20(ERC20 _asset, address _to, uint256 _amount) public onlyOwner {
        // slither-disable-next-line unchecked-transfer
        _asset.transfer(_to, _amount);
        // slither-disable-next-line reentrancy-events
        emit WithdrewERC20(msg.sender, _to, address(_asset), _amount);
    }

    /// @notice Withdraws ERC721 token to the recipient.
    /// @param _asset ERC721 token to withdraw.
    /// @param _to    Address to receive the ERC721 token.
    /// @param _id    Token ID of the ERC721 token to withdraw.
    function withdrawERC721(ERC721 _asset, address _to, uint256 _id) external onlyOwner {
        _asset.transferFrom(address(this), _to, _id);
        // slither-disable-next-line reentrancy-events
        emit WithdrewERC721(msg.sender, _to, address(_asset), _id);
    }
}
