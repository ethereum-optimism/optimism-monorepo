// SPDX-License-Identifier: MIT
pragma solidity 0.8.25;

// Libraries
import { Predeploys } from "src/libraries/Predeploys.sol";
import { ZeroAddress, Unauthorized } from "src/libraries/errors/CommonErrors.sol";

// Interfaces
import { ISuperchainERC20 } from "src/L2/interfaces/ISuperchainERC20.sol";
import { IERC7802, IERC165 } from "src/L2/interfaces/IERC7802.sol";
import { IL2ToL2CrossDomainMessenger } from "src/L2/interfaces/IL2ToL2CrossDomainMessenger.sol";

/// @custom:proxied true
/// @custom:predeploy 0x4200000000000000000000000000000000000028
/// @title SuperchainTokenBridge
/// @notice The SuperchainTokenBridge allows for the bridging of ERC20 tokens to make them fungible across the
///         Superchain. It builds on top of the L2ToL2CrossDomainMessenger for both replay protection and domain
///         binding.
contract SuperchainTokenBridge {
    /// @notice Thrown when attempting to relay a message and the cross domain message sender is not the
    /// SuperchainTokenBridge.
    error InvalidCrossDomainSender();

    /// @notice Thrown when attempting to use a token that does not implement the ERC7802 interface.
    error InvalidERC7802();

    /// @notice Emitted when tokens are sent from one chain to another.
    /// @param token         Address of the token sent.
    /// @param from          Address of the sender.
    /// @param to            Address of the recipient.
    /// @param amount        Number of tokens sent.
    /// @param destination   Chain ID of the destination chain.
    event SendERC20(
        address indexed token, address indexed from, address indexed to, uint256 amount, uint256 destination
    );

    /// @notice Emitted whenever tokens are successfully relayed on this chain.
    /// @param token         Address of the token relayed.
    /// @param from          Address of the msg.sender of sendERC20 on the source chain.
    /// @param to            Address of the recipient.
    /// @param amount        Amount of tokens relayed.
    /// @param source        Chain ID of the source chain.
    event RelayERC20(address indexed token, address indexed from, address indexed to, uint256 amount, uint256 source);

    /// @notice Address of the L2ToL2CrossDomainMessenger Predeploy.
    address internal constant MESSENGER = Predeploys.L2_TO_L2_CROSS_DOMAIN_MESSENGER;

    /// @notice Semantic version.
    /// @custom:semver 1.0.0-beta.3
    string public constant version = "1.0.0-beta.3";

    /// @notice Sends tokens to a target address on another chain.
    /// @dev Tokens are burned on the source chain.
    /// @param _token    Token to send.
    /// @param _to       Address to send tokens to.
    /// @param _amount   Amount of tokens to send.
    /// @param _chainId  Chain ID of the destination chain.
    /// @return msgHash_ Hash of the message sent.
    function sendERC20(
        address _token,
        address _to,
        uint256 _amount,
        uint256 _chainId
    )
        external
        returns (bytes32 msgHash_)
    {
        if (_to == address(0)) revert ZeroAddress();

        if (!IERC165(_token).supportsInterface(type(IERC7802).interfaceId)) revert InvalidERC7802();

        ISuperchainERC20(_token).crosschainBurn(msg.sender, _amount);

        bytes memory message = abi.encodeCall(this.relayERC20, (_token, msg.sender, _to, _amount));
        msgHash_ = IL2ToL2CrossDomainMessenger(MESSENGER).sendMessage(_chainId, address(this), message);

        emit SendERC20(_token, msg.sender, _to, _amount, _chainId);
    }

    /// @notice Relays tokens received from another chain.
    /// @dev Tokens are minted on the destination chain.
    /// @param _token   Token to relay.
    /// @param _from    Address of the msg.sender of sendERC20 on the source chain.
    /// @param _to      Address to relay tokens to.
    /// @param _amount  Amount of tokens to relay.
    function relayERC20(address _token, address _from, address _to, uint256 _amount) external {
        if (msg.sender != MESSENGER) revert Unauthorized();

        (address crossDomainMessageSender, uint256 source) =
            IL2ToL2CrossDomainMessenger(MESSENGER).crossDomainMessageContext();

        if (crossDomainMessageSender != address(this)) revert InvalidCrossDomainSender();

        ISuperchainERC20(_token).crosschainMint(_to, _amount);

        emit RelayERC20(_token, _from, _to, _amount, source);
    }
}
