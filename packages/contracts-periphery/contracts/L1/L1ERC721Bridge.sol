// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { ERC721Bridge } from "../universal/op-erc721/ERC721Bridge.sol";
import {
    CrossDomainEnabled
} from "@eth-optimism/contracts/contracts/libraries/bridge/CrossDomainEnabled.sol";
import { IERC721 } from "@openzeppelin/contracts/token/ERC721/IERC721.sol";
import { Address } from "@openzeppelin/contracts/utils/Address.sol";
import { L2ERC721Bridge } from "../L2/L2ERC721Bridge.sol";
import { Semver } from "@eth-optimism/contracts-bedrock/contracts/universal/Semver.sol";
import { Initializable } from "@openzeppelin/contracts/proxy/utils/Initializable.sol";

/**
 * @title L1ERC721Bridge
 * @notice The L1 ERC721 bridge is a contract which works together with the L2 ERC721 bridge to
 *         make it possible to transfer ERC721 tokens from Ethereum to Optimism. This contract
 *         acts as an escrow for ERC721 tokens deposted into L2.
 */
contract L1ERC721Bridge is ERC721Bridge, Semver {
    /**
     * @notice Emitted when an ERC721 bridge to the other network is initiated.
     *
     * @param localToken  Address of the token on this domain.
     * @param remoteToken Address of the token on the remote domain.
     * @param from        Address that initiated bridging action.
     * @param to          Address to receive the token.
     * @param tokenId     ID of the specific token deposited.
     * @param extraData   Extra data for use on the client-side.
     */
    event ERC721BridgeInitiated(
        address indexed localToken,
        address indexed remoteToken,
        address indexed from,
        address to,
        uint256 tokenId,
        bytes extraData
    );

    /**
     * @notice Emitted when an ERC721 bridge from the other network is finalized.
     *
     * @param localToken  Address of the token on this domain.
     * @param remoteToken Address of the token on the remote domain.
     * @param from        Address that initiated bridging action.
     * @param to          Address to receive the token.
     * @param tokenId     ID of the specific token deposited.
     * @param extraData   Extra data for use on the client-side.
     */
    event ERC721BridgeFinalized(
        address indexed localToken,
        address indexed remoteToken,
        address indexed from,
        address to,
        uint256 tokenId,
        bytes extraData
    );

    /**
     * @notice Emitted when an ERC721 bridge from the other network fails.
     *
     * @param localToken  Address of the token on this domain.
     * @param remoteToken Address of the token on the remote domain.
     * @param from        Address that initiated bridging action.
     * @param to          Address to receive the token.
     * @param tokenId     ID of the specific token deposited.
     * @param extraData   Extra data for use on the client-side.
     */
    event ERC721BridgeFailed(
        address indexed localToken,
        address indexed remoteToken,
        address indexed from,
        address to,
        uint256 tokenId,
        bytes extraData
    );

    /**
     * @notice Address of the bridge on the other network.
     */
    address public otherBridge;

    /**
     * @notice Mapping of L1 token to L2 token to ID to boolean, indicating if the given L1 token
     *         by ID was deposited for a given L2 token.
     */
    mapping(address => mapping(address => mapping(uint256 => bool))) public deposits;

    /**
     * @custom:semver 1.0.0
     *
     * @param _messenger   Address of the CrossDomainMessenger on this network.
     * @param _otherBridge Address of the ERC721 bridge on the other network.
     */
    constructor(address _messenger, address _otherBridge)
        Semver(1, 0, 0)
        CrossDomainEnabled(address(0))
    {
        initialize(_messenger, _otherBridge);
    }

    /**
     * @param _messenger   Address of the CrossDomainMessenger on this network.
     * @param _otherBridge Address of the ERC721 bridge on the other network.
     */
    function initialize(address _messenger, address _otherBridge) public initializer {
        require(_messenger != address(0), "ERC721Bridge: messenger cannot be address(0)");
        require(_otherBridge != address(0), "ERC721Bridge: other bridge cannot be address(0)");

        messenger = _messenger;
        otherBridge = _otherBridge;
    }

    /**
     * @notice Initiates a bridge of an NFT to the caller's account on L2. Note that this function
     *         can only be called by EOAs. Smart contract wallets should use the `bridgeERC721To`
     *         function after ensuring that the recipient address on the remote chain exists. Also
     *         note that the current owner of the token on this chain must approve this contract to
     *         operate the NFT before it can be bridged.
     *
     * @param _localToken  Address of the ERC721 on this domain.
     * @param _remoteToken Address of the ERC721 on the remote domain.
     * @param _tokenId     Token ID to bridge.
     * @param _minGasLimit Minimum gas limit for the bridge message on the other domain.
     * @param _extraData   Optional data to forward to L2. Data supplied here will not be used to
     *                     execute any code on L2 and is only emitted as extra data for the
     *                     convenience of off-chain tooling.
     */
    function bridgeERC721(
        address _localToken,
        address _remoteToken,
        uint256 _tokenId,
        uint32 _minGasLimit,
        bytes calldata _extraData
    ) external {
        // Modifier requiring sender to be EOA. This prevents against a user error that would occur
        // if the sender is a smart contract wallet that has a different address on the remote chain
        // (or doesn't have an address on the remote chain at all). The user would fail to receive
        // the NFT if they use this function because it sends the NFT to the same address as the
        // caller. This check could be bypassed by a malicious contract via initcode, but it takes
        // care of the user error we want to avoid.
        require(!Address.isContract(msg.sender), "L1ERC721Bridge: account is not externally owned");

        _initiateBridgeERC721(
            _localToken,
            _remoteToken,
            msg.sender,
            msg.sender,
            _tokenId,
            _minGasLimit,
            _extraData
        );
    }

    /**
     * @notice Initiates a bridge of an NFT to some recipient's account on L2. Note that the current
     *         owner of the token on this chain must approve this contract to operate the NFT before
     *         it can be bridged.
     *
     * @param _localToken  Address of the ERC721 on this domain.
     * @param _remoteToken Address of the ERC721 on the remote domain.
     * @param _to          Address to receive the token on the other domain.
     * @param _tokenId     Token ID to bridge.
     * @param _minGasLimit Minimum gas limit for the bridge message on the other domain.
     * @param _extraData   Optional data to forward to L2. Data supplied here will not be used to
     *                     execute any code on L2 and is only emitted as extra data for the
     *                     convenience of off-chain tooling.
     */
    function bridgeERC721To(
        address _localToken,
        address _remoteToken,
        address _to,
        uint256 _tokenId,
        uint32 _minGasLimit,
        bytes calldata _extraData
    ) external {
        require(_to != address(0), "ERC721Bridge: nft recipient cannot be address(0)");

        _initiateBridgeERC721(
            _localToken,
            _remoteToken,
            msg.sender,
            _to,
            _tokenId,
            _minGasLimit,
            _extraData
        );
    }

    /*************************
     * Cross-chain Functions *
     *************************/

    /**
     * @notice Completes an ERC721 bridge from the other domain and sends the ERC721 token to the
     *         recipient on this domain.
     *
     * @param _localToken  Address of the ERC721 token on this domain.
     * @param _remoteToken Address of the ERC721 token on the other domain.
     * @param _from        Address that triggered the bridge on the other domain.
     * @param _to          Address to receive the token on this domain.
     * @param _tokenId     ID of the token being deposited.
     * @param _extraData   Optional data to forward to L2. Data supplied here will not be used to
     *                     execute any code on L2 and is only emitted as extra data for the
     *                     convenience of off-chain tooling.
     */
    function finalizeBridgeERC721(
        address _localToken,
        address _remoteToken,
        address _from,
        address _to,
        uint256 _tokenId,
        bytes calldata _extraData
    ) external onlyFromCrossDomainAccount(otherBridge) {
        try this.completeOutboundTransfer(_localToken, _remoteToken, _to, _tokenId) {
            if (_from == otherBridge) {
                // The _from address is the address of the remote bridge if a transfer fails to be
                // finalized on the remote chain.
                // slither-disable-next-line reentrancy-events
                emit ERC721Refunded(_localToken, _remoteToken, _to, _tokenId, _extraData);
            } else {
                // slither-disable-next-line reentrancy-events
                emit ERC721BridgeFinalized(
                    _localToken,
                    _remoteToken,
                    _from,
                    _to,
                    _tokenId,
                    _extraData
                );
            }
        } catch {
            // If the token ID for this L1/L2 NFT pair is not escrowed in the L1 Bridge or if
            // another error occurred during finalization, we initiate a cross-domain message to
            // send the NFT back to its original owner on L2. This can happen if an L2 native NFT is
            // bridged to L1, or if a user mistakenly entered an incorrect L1 ERC721 address.
            bytes memory message = abi.encodeWithSelector(
                L2ERC721Bridge.finalizeBridgeERC721.selector,
                _remoteToken,
                _localToken,
                address(this), // Set the new _from address to be this contract since the NFT was
                // never transferred to the recipient on this chain.
                _from, // Refund the NFT to the original owner on the remote chain.
                _tokenId,
                _extraData
            );

            // Send the message to the L2 bridge.
            // slither-disable-next-line reentrancy-events
            sendCrossDomainMessage(otherBridge, 600_000, message);

            // slither-disable-next-line reentrancy-events
            emit ERC721BridgeFailed(_localToken, _remoteToken, _from, _to, _tokenId, _extraData);
        }
    }

    /**
     * @notice Completes an outbound token transfer. Public function, but can only be called by
     *         this contract. It's security critical that there be absolutely no way for anyone to
     *         trigger this function, except by explicit trigger within this contract. Used as a
     *         simple way to be able to try/catch any type of revert that could occur during an
     *         ERC721 mint/transfer.
     *
     * @param _localToken  Address of the ERC721 on this chain.
     * @param _remoteToken Address of the corresponding token on the remote chain.
     * @param _to          Address of the receiver.
     * @param _tokenId     ID of the token being deposited.
     */
    function completeOutboundTransfer(
        address _localToken,
        address _remoteToken,
        address _to,
        uint256 _tokenId
    ) external onlySelf {
        // Checks that the L1/L2 NFT pair has a token ID that is escrowed in the L1 Bridge. Without
        // this check, an attacker could steal a legitimate L1 NFT by supplying an arbitrary L2 NFT
        // that maps to the L1 NFT.
        require(
            deposits[_localToken][_remoteToken][_tokenId] == true,
            "L1ERC721Bridge: token ID is not escrowed in l1 bridge for this l1/l2 nft pair"
        );

        // Mark that the token ID for this L1/L2 token pair is no longer escrowed in the L1
        // Bridge.
        deposits[_localToken][_remoteToken][_tokenId] = false;

        // When a withdrawal is finalized on L1, the L1 Bridge transfers the NFT to the
        // withdrawer.
        IERC721(_localToken).safeTransferFrom(address(this), _to, _tokenId);
    }

    /**
     * @notice Internal function for initiating a token bridge to the other domain.
     *
     * @param _localToken  Address of the ERC721 on this domain.
     * @param _remoteToken Address of the ERC721 on the remote domain.
     * @param _from        Address of the sender on this domain.
     * @param _to          Address to receive the token on the other domain.
     * @param _tokenId     Token ID to bridge.
     * @param _minGasLimit Minimum gas limit for the bridge message on the other domain.
     * @param _extraData   Optional data to forward to L2. Data supplied here will not be used to
     *                     execute any code on L2 and is only emitted as extra data for the
     *                     convenience of off-chain tooling.
     */
    function _initiateBridgeERC721(
        address _localToken,
        address _remoteToken,
        address _from,
        address _to,
        uint256 _tokenId,
        uint32 _minGasLimit,
        bytes calldata _extraData
    ) internal {
        require(_remoteToken != address(0), "ERC721Bridge: remote token cannot be address(0)");

        // Construct calldata for _l2Token.finalizeBridgeERC721(_to, _tokenId)
        bytes memory message = abi.encodeWithSelector(
            L2ERC721Bridge.finalizeBridgeERC721.selector,
            _remoteToken,
            _localToken,
            _from,
            _to,
            _tokenId,
            _extraData
        );

        // Lock token into bridge
        deposits[_localToken][_remoteToken][_tokenId] = true;
        IERC721(_localToken).transferFrom(_from, address(this), _tokenId);

        // Send calldata into L2
        sendCrossDomainMessage(otherBridge, _minGasLimit, message);
        emit ERC721BridgeInitiated(_localToken, _remoteToken, _from, _to, _tokenId, _extraData);
    }
}
