// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { ERC721Bridge } from "src/universal/ERC721Bridge.sol";

// Libraries
import { ERC165Checker } from "@openzeppelin/contracts/utils/introspection/ERC165Checker.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";

// Interfaces
import { IL1ERC721Bridge } from "src/L1/interfaces/IL1ERC721Bridge.sol";
import { IOptimismMintableERC721 } from "src/universal/interfaces/IOptimismMintableERC721.sol";
import { ICrossDomainMessenger } from "src/universal/interfaces/ICrossDomainMessenger.sol";
import { ISemver } from "src/universal/interfaces/ISemver.sol";

/// @custom:proxied true
/// @custom:predeploy 0x4200000000000000000000000000000000000014
/// @title L2ERC721Bridge
/// @notice The L2 ERC721 bridge is a contract which works together with the L1 ERC721 bridge to
///         make it possible to transfer ERC721 tokens from Ethereum to Optimism. This contract
///         acts as a minter for new tokens when it hears about deposits into the L1 ERC721 bridge.
///         This contract also acts as a burner for tokens being withdrawn.
///         **WARNING**: Do not bridge an ERC721 that was originally deployed on Optimism. This
///         bridge ONLY supports ERC721s originally deployed on Ethereum. Users will need to
///         wait for the one-week challenge period to elapse before their Optimism-native NFT
///         can be refunded on L2.
contract L2ERC721Bridge is ERC721Bridge, ISemver {
    /// @custom:semver 1.8.0-beta.2
    string public constant version = "1.8.0-beta.2";

    /// @notice Constructs the L2ERC721Bridge contract.
    constructor() ERC721Bridge() {
        initialize({ _l1ERC721Bridge: payable(address(0)) });
    }

    /// @notice Initializes the contract.
    /// @param _l1ERC721Bridge Address of the ERC721 bridge contract on the other network.
    function initialize(address payable _l1ERC721Bridge) public initializer {
        __ERC721Bridge_init({
            _messenger: ICrossDomainMessenger(Predeploys.L2_CROSS_DOMAIN_MESSENGER),
            _otherBridge: ERC721Bridge(_l1ERC721Bridge)
        });
    }

    /// @notice Completes an ERC721 bridge from the other domain and sends the ERC721 token to the
    ///         recipient on this domain.
    /// @param _localToken  Address of the ERC721 token on this domain.
    /// @param _remoteToken Address of the ERC721 token on the other domain.
    /// @param _from        Address that triggered the bridge on the other domain.
    /// @param _to          Address to receive the token on this domain.
    /// @param _tokenId     ID of the token being deposited.
    /// @param _extraData   Optional data to forward to L1.
    ///                     Data supplied here will not be used to execute any code on L1 and is
    ///                     only emitted as extra data for the convenience of off-chain tooling.
    function finalizeBridgeERC721(
        address _localToken,
        address _remoteToken,
        address _from,
        address _to,
        uint256 _tokenId,
        bytes calldata _extraData
    )
        external
        onlyOtherBridge
    {
        require(_localToken != address(this), "L2ERC721Bridge: local token cannot be self");

        // Note that supportsInterface makes a callback to the _localToken address which is user
        // provided.
        require(
            ERC165Checker.supportsInterface(_localToken, type(IOptimismMintableERC721).interfaceId),
            "L2ERC721Bridge: local token interface is not compliant"
        );

        require(
            _remoteToken == IOptimismMintableERC721(_localToken).remoteToken(),
            "L2ERC721Bridge: wrong remote token for Optimism Mintable ERC721 local token"
        );

        // When a deposit is finalized, we give the NFT with the same tokenId to the account
        // on L2. Note that safeMint makes a callback to the _to address which is user provided.
        IOptimismMintableERC721(_localToken).safeMint(_to, _tokenId);

        // slither-disable-next-line reentrancy-events
        emit ERC721BridgeFinalized(_localToken, _remoteToken, _from, _to, _tokenId, _extraData);
    }

    /// @inheritdoc ERC721Bridge
    function _initiateBridgeERC721(
        address _localToken,
        address _remoteToken,
        address _from,
        address _to,
        uint256 _tokenId,
        uint32 _minGasLimit,
        bytes calldata _extraData
    )
        internal
        override
    {
        require(_remoteToken != address(0), "L2ERC721Bridge: remote token cannot be address(0)");

        // Check that the withdrawal is being initiated by the NFT owner
        require(
            _from == IOptimismMintableERC721(_localToken).ownerOf(_tokenId),
            "L2ERC721Bridge: Withdrawal is not being initiated by NFT owner"
        );

        // Construct calldata for l1ERC721Bridge.finalizeBridgeERC721(_to, _tokenId)
        // slither-disable-next-line reentrancy-events
        address remoteToken = IOptimismMintableERC721(_localToken).remoteToken();
        require(remoteToken == _remoteToken, "L2ERC721Bridge: remote token does not match given value");

        // When a withdrawal is initiated, we burn the withdrawer's NFT to prevent subsequent L2
        // usage
        // slither-disable-next-line reentrancy-events
        IOptimismMintableERC721(_localToken).burn(_from, _tokenId);

        bytes memory message = abi.encodeCall(
            IL1ERC721Bridge.finalizeBridgeERC721, (remoteToken, _localToken, _from, _to, _tokenId, _extraData)
        );

        // Send message to L1 bridge
        // slither-disable-next-line reentrancy-events
        messenger.sendMessage({ _target: address(otherBridge), _message: message, _minGasLimit: _minGasLimit });

        // slither-disable-next-line reentrancy-events
        emit ERC721BridgeInitiated(_localToken, remoteToken, _from, _to, _tokenId, _extraData);
    }
}
