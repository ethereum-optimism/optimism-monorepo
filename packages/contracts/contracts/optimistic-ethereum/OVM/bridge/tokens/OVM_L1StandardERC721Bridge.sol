// SPDX-License-Identifier: MIT
// @unsupported: ovm
pragma solidity >0.5.0 <0.8.0;
pragma experimental ABIEncoderV2;

/* Interface Imports */
import {iOVM_L1ERC721Bridge} from "../../../iOVM/bridge/tokens/iOVM_L1ERC721Bridge.sol";
import {iOVM_L2ERC721Bridge} from "../../../iOVM/bridge/tokens/iOVM_L2ERC721Bridge.sol";
import {IERC721} from "@openzeppelin/contracts/token/ERC721/IERC721.sol";

/* Library Imports */
import {OVM_CrossDomainEnabled} from "../../../libraries/bridge/OVM_CrossDomainEnabled.sol";
import {Lib_PredeployAddresses} from "../../../libraries/constants/Lib_PredeployAddresses.sol";
import {SafeMath} from "@openzeppelin/contracts/math/SafeMath.sol";
import {Address} from "@openzeppelin/contracts/utils/Address.sol";

/* Contract Imports */
import {ERC721Holder} from "@openzeppelin/contracts/token/ERC721/ERC721Holder.sol";

/**
 * @title OVM_L1StandardERC721Bridge
 * @dev The L1 ERC721 Bridge is a contract which stores deposited L1 NFTs that are in use on L2.
 * It synchronizes a corresponding L2 Bridge, informing it of deposits  and listening to it for
 * newly finalized withdrawals.
 *
 * Compiler used: solc
 * Runtime target: EVM
 */
contract OVM_L1StandardERC721Bridge is iOVM_L1ERC721Bridge, OVM_CrossDomainEnabled, ERC721Holder {
    using SafeMath for uint256;

    /********************************
    * External Contract References *
    ********************************/

    address public l2TokenBridge;

    // Maps L1 token to L2 token to balance of the L1 token deposited
    mapping(address => mapping(address => uint256)) public deposits;

    /** @dev Modifier requiring sender to be EOA.  This check could be bypassed by a malicious
    *  contract via initcode, but it takes care of the user error we want to avoid.
    */
    modifier onlyEOA() {
        // Used to stop deposits from contracts (avoid accidentally lost tokens)
        require(!Address.isContract(msg.sender), "Account not EOA");
        _;
    }

    /***************
    * Constructor *
    ***************/

    // This contract lives behind a proxy, so the constructor parameters will go unused.
    constructor()
        OVM_CrossDomainEnabled(address(0))
    {}

    /******************
    * Initialization *
    ******************/

    /**
    * @param _l1messenger L1 Messenger address being used for cross-chain communications.
    * @param _l2TokenBridge L2 standard bridge address.
    */
    function initialize(address _l1messenger, address _l2TokenBridge)
        public
    {
        require(messenger == address(0), "Contract has already been initialized.");
        messenger = _l1messenger;
        l2TokenBridge = _l2TokenBridge;
    }

    /**************
    * Depositing *
    **************/

    /**
    * @inheritdoc iOVM_L1ERC721Bridge
    */
    function depositERC721(
        address _l1Token,
        address _l2Token,
        uint256 _tokenId,
        uint32 _l2Gas,
        bytes calldata _data
    )
        external
        virtual
        override
        onlyEOA
    {
        _initiateERC721Deposit(
            _l1Token,
            _l2Token,
            msg.sender,
            msg.sender,
            _tokenId,
            _l2Gas,
            _data
        );
    }

    /**
    * @inheritdoc iOVM_L1ERC721Bridge
    */
    function depositERC721To(
        address _l1Token,
        address _l2Token,
        address _to,
        uint256 _tokenId,
        uint32 _l2Gas,
        bytes calldata _data
    )
        external
        virtual
        override
    {
        _initiateERC721Deposit(
            _l1Token,
            _l2Token,
            msg.sender,
            _to,
            _tokenId,
            _l2Gas,
            _data
        );
    }

    /**
    * @dev Performs the logic for deposits by informing the L2 Deposited ERC721 Token
    * contract of the deposit and calling a handler to lock the L1 funds (e.g. safeTransferFrom).
    *
    * @param _l1Token Address of the L1 ERC721 we are depositing.
    * @param _l2Token Address of the L1 respective L2 ERC721.
    * @param _from Account to pull the deposit from on L1.
    * @param _to Account to give the deposit to on L2.
    * @param _tokenId The NFT to deposit.
    * @param _l2Gas Gas limit required to complete the deposit on L2.
    * @param _data Optional data to forward to L2. This data is provided
    *        solely as a convenience for external contracts. Aside from enforcing a maximum
    *        length, these contracts provide no guarantees about its content.
    */
    function _initiateERC721Deposit(
        address _l1Token,
        address _l2Token,
        address _from,
        address _to,
        uint256 _tokenId,
        uint32 _l2Gas,
        bytes calldata _data
    )
        internal
    {
        // When a deposit is initiated on L1, the L1 Bridge transfers the funds to itself for future
        // withdrawals. safeTransferFrom also checks if this contract knows how to handle ERC721, which
        // it does.
        IERC721(_l1Token).safeTransferFrom(_from, address(this), _tokenId);

        // Construct calldata for _l2Token.finalizeERC721Deposit(_to, _amount)
        bytes memory message = abi.encodeWithSelector(
            iOVM_L2ERC721Bridge.finalizeERC721Deposit.selector,
            _l1Token,
            _l2Token,
            _from,
            _to,
            _tokenId,
            _data
        );

        // Send calldata into L2
        sendCrossDomainMessage(l2TokenBridge, _l2Gas, message);

        deposits[_l1Token][_l2Token] = deposits[_l1Token][_l2Token].add(1);

        emit ERC721DepositInitiated(
            _l1Token,
            _l2Token,
            _from,
            _to,
            _tokenId,
            _data
        );
    }

    /*************************
    * Cross-chain Functions *
    *************************/

    /**
    * @inheritdoc iOVM_L1ERC721Bridge
    */
    function finalizeERC721Withdrawal(
        address _l1Token,
        address _l2Token,
        address _from,
        address _to,
        uint256 _tokenId,
        bytes calldata _data
    )
        external
        override
        onlyFromCrossDomainAccount(l2TokenBridge)
    {
        deposits[_l1Token][_l2Token] = deposits[_l1Token][_l2Token].sub(1);

        // When a withdrawal is finalized on L1, the L1 Bridge transfers the funds to the withdrawer
        IERC721(_l1Token).safeTransferFrom(address(this), _to, _tokenId);

        emit ERC721WithdrawalFinalized(
            _l1Token,
            _l2Token,
            _from,
            _to,
            _tokenId,
            _data
        );
    }
}
