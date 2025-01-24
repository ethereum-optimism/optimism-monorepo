// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { WETH98 } from "src/universal/WETH98.sol";

// Libraries
import { Unauthorized, ZeroAddress } from "src/libraries/errors/CommonErrors.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";
import { Preinstalls } from "src/libraries/Preinstalls.sol";
import { SafeSend } from "src/universal/SafeSend.sol";

// Interfaces
import { ISemver } from "interfaces/universal/ISemver.sol";
import { IL2ToL2CrossDomainMessenger } from "interfaces/L2/IL2ToL2CrossDomainMessenger.sol";
import { IETHLiquidity } from "interfaces/L2/IETHLiquidity.sol";
import { IERC7802, IERC165 } from "interfaces/L2/IERC7802.sol";
import { IERC20 } from "@openzeppelin/contracts/token/ERC20/IERC20.sol";

/// @custom:proxied true
/// @custom:predeploy 0x4200000000000000000000000000000000000024
/// @title SuperchainWETH
/// @notice SuperchainWETH is a version of WETH that can be freely transfrered between chains
///         within the superchain. SuperchainWETH can be converted into native ETH on chains that
///         do not use a custom gas token.
contract SuperchainWETH is WETH98, IERC7802, ISemver {
    /// @notice Thrown when attempting to relay a message and the cross domain message sender is not SuperchainWETH.
    error InvalidCrossDomainSender();

    /// @notice Emitted when ETH is sent from one chain to another.
    /// @param from          Address of the sender.
    /// @param to            Address of the recipient.
    /// @param amount        Amount of ETH sent.
    /// @param destination   Chain ID of the destination chain.
    event SendETH(address indexed from, address indexed to, uint256 amount, uint256 destination);

    /// @notice Emitted whenever ETH is successfully relayed on this chain.
    /// @param from          Address of the msg.sender of sendETH on the source chain.
    /// @param to            Address of the recipient.
    /// @param amount        Amount of ETH relayed.
    /// @param source        Chain ID of the source chain.
    event RelayETH(address indexed from, address indexed to, uint256 amount, uint256 source);

    /// @notice Semantic version.
    /// @custom:semver 1.0.0-beta.14
    string public constant version = "1.0.0-beta.14";

    /// @inheritdoc WETH98
    function allowance(address owner, address spender) public view override returns (uint256) {
        if (spender == Preinstalls.Permit2) return type(uint256).max;
        return super.allowance(owner, spender);
    }

    /// @notice Mints WETH to an address.
    /// @param _to The address to mint WETH to.
    /// @param _amount The amount of WETH to mint.
    function _mint(address _to, uint256 _amount) internal {
        _balanceOf[_to] += _amount;
        emit Transfer(address(0), _to, _amount);
    }

    /// @notice Burns WETH from an address.
    /// @param _from The address to burn WETH from.
    /// @param _amount The amount of WETH to burn.
    function _burn(address _from, uint256 _amount) internal {
        _balanceOf[_from] -= _amount;
        emit Transfer(_from, address(0), _amount);
    }

    /// @notice Allows the SuperchainTokenBridge to mint tokens.
    /// @param _to     Address to mint tokens to.
    /// @param _amount Amount of tokens to mint.
    function crosschainMint(address _to, uint256 _amount) external {
        if (msg.sender != Predeploys.SUPERCHAIN_TOKEN_BRIDGE) revert Unauthorized();

        _mint(_to, _amount);

        // Withdraw from ETHLiquidity contract.
        // NOTE: 'mint' will soon change to 'withdraw'.
        IETHLiquidity(Predeploys.ETH_LIQUIDITY).mint(_amount);

        emit CrosschainMint(_to, _amount, msg.sender);
    }

    /// @notice Allows the SuperchainTokenBridge to burn tokens.
    /// @param _from   Address to burn tokens from.
    /// @param _amount Amount of tokens to burn.
    function crosschainBurn(address _from, uint256 _amount) external {
        if (msg.sender != Predeploys.SUPERCHAIN_TOKEN_BRIDGE) revert Unauthorized();

        _burn(_from, _amount);

        // Deposit to ETHLiquidity contract.
        // NOTE: 'burn' will soon change to 'deposit'.
        IETHLiquidity(Predeploys.ETH_LIQUIDITY).burn{ value: _amount }();

        emit CrosschainBurn(_from, _amount, msg.sender);
    }

    /// @inheritdoc IERC165
    function supportsInterface(bytes4 _interfaceId) public view virtual returns (bool) {
        return _interfaceId == type(IERC7802).interfaceId || _interfaceId == type(IERC20).interfaceId
            || _interfaceId == type(IERC165).interfaceId;
    }

    /// @notice Sends ETH to some target address on another chain.
    /// @param _to       Address to send ETH to.
    /// @param _chainId  Chain ID of the destination chain.
    /// @return msgHash_ Hash of the message sent.
    function sendETH(address _to, uint256 _chainId) external payable returns (bytes32 msgHash_) {
        if (_to == address(0)) revert ZeroAddress();

        // NOTE: 'burn' will soon change to 'deposit'.
        IETHLiquidity(Predeploys.ETH_LIQUIDITY).burn{ value: msg.value }();

        msgHash_ = IL2ToL2CrossDomainMessenger(Predeploys.L2_TO_L2_CROSS_DOMAIN_MESSENGER).sendMessage({
            _destination: _chainId,
            _target: address(this),
            _message: abi.encodeCall(this.relayETH, (msg.sender, _to, msg.value))
        });

        emit SendETH(msg.sender, _to, msg.value, _chainId);
    }

    /// @notice Relays ETH received from another chain.
    /// @param _from       Address of the msg.sender of sendETH on the source chain.
    /// @param _to         Address to relay ETH to.
    /// @param _amount     Amount of ETH to relay.
    function relayETH(address _from, address _to, uint256 _amount) external {
        if (msg.sender != Predeploys.L2_TO_L2_CROSS_DOMAIN_MESSENGER) revert Unauthorized();

        (address crossDomainMessageSender, uint256 source) =
            IL2ToL2CrossDomainMessenger(Predeploys.L2_TO_L2_CROSS_DOMAIN_MESSENGER).crossDomainMessageContext();

        if (crossDomainMessageSender != address(this)) revert InvalidCrossDomainSender();

        // NOTE: 'mint' will soon change to 'withdraw'.
        IETHLiquidity(Predeploys.ETH_LIQUIDITY).mint(_amount);

        // This is a forced ETH send to the recipient, the recipient should NOT expect to be called.
        new SafeSend{ value: _amount }(payable(_to));

        emit RelayETH(_from, _to, _amount, source);
    }
}
