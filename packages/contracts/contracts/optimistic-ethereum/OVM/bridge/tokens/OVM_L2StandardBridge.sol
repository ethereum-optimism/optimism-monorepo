// SPDX-License-Identifier: MIT
pragma solidity >0.5.0 <0.8.0;
pragma experimental ABIEncoderV2;

/* Interface Imports */
import { iOVM_L1StandardBridge } from "../../../iOVM/bridge/tokens/iOVM_L1StandardBridge.sol";
import { iOVM_L1ERC20Bridge } from "../../../iOVM/bridge/tokens/iOVM_L1ERC20Bridge.sol";
import { iOVM_L2ERC20Bridge } from "../../../iOVM/bridge/tokens/iOVM_L2ERC20Bridge.sol";

/* Library Imports */
import { OVM_CrossDomainEnabled } from "../../../libraries/bridge/OVM_CrossDomainEnabled.sol";
import { Lib_PredeployAddresses } from "../../../libraries/constants/Lib_PredeployAddresses.sol";

/* Contract Imports */
import { L2StandardERC20 } from "../../../libraries/standards/L2StandardERC20.sol";

/**
 * @title OVM_L2StandardBridge
 * @dev The L2 Standard bridge is a contract which works together with the L1 Standard bridge to enable
 * ETH and ERC20 transitions between L1 and L2.
 * This contract acts as a minter for new tokens when it hears about deposits into the L1 Standard bridge.
 * This contract also acts as a burner of the tokens intended for withdrawal, informing the L1 bridge to release L1 funds.
 *
 * Compiler used: optimistic-solc
 * Runtime target: OVM
 */
contract OVM_L2StandardBridge is iOVM_L2ERC20Bridge, OVM_CrossDomainEnabled {

    /********************************
     * External Contract References *
     ********************************/

    address public l1TokenBridge;

    /***************
     * Constructor *
     ***************/

    /**
     * @param _l2CrossDomainMessenger Cross-domain messenger used by this contract.
     * @param _l1TokenBridge Address of the L1 bridge deployed to the main chain.

     */
    constructor(
        address _l2CrossDomainMessenger,
        address _l1TokenBridge
    )
        OVM_CrossDomainEnabled(_l2CrossDomainMessenger)
    {
        l1TokenBridge = _l1TokenBridge;
    }

    /***************
     * Withdrawing *
     ***************/

    /**
     * @inheritdoc iOVM_L2ERC20Bridge
     */
    function withdraw(
        address _l2Token,
        uint256 _amount,
        uint32, // _l1Gas,
        bytes calldata _data
    )
        external
        override
        virtual
    {
        _initiateWithdrawal(
            _l2Token,
            msg.sender,
            msg.sender,
            _amount,
            0,
            _data
        );
    }

    /**
     * @inheritdoc iOVM_L2ERC20Bridge
     */
    function withdrawTo(
        address _l2Token,
        address _to,
        uint256 _amount,
        uint32, // _l1Gas,
        bytes calldata _data
    )
        external
        override
        virtual
    {
        _initiateWithdrawal(
            _l2Token,
            msg.sender,
            _to,
            _amount,
            0,
            _data
        );
    }

    /**
     * @dev Performs the logic for deposits by storing the token and informing the L2 token Gateway of the deposit.
     * @param _l2Token Address of L2 token where withdrawal was initiated.
     * @param _from Account to pull the deposit from on L2.
     * @param _to Account to give the withdrawal to on L1.
     * @param _amount Amount of the token to withdraw.
     * param _l1Gas Unused, but included for potential forward compatibility considerations.
     * @param _data Optional data to forward to L1. This data is provided
     *        solely as a convenience for external contracts. Aside from enforcing a maximum
     *        length, these contracts provide no guarantees about its content.
     */
    function _initiateWithdrawal(
        address _l2Token,
        address _from,
        address _to,
        uint256 _amount,
        uint32, // _l1Gas,
        bytes calldata _data
    )
        internal
    {
        // When a withdrawal is initiated, we burn the withdrawer's funds to prevent subsequent L2 usage
        L2StandardERC20(_l2Token).burn(msg.sender, _amount);

        // Construct calldata for l1TokenBridge.finalizeERC20Withdrawal(_to, _amount)
        address l1Token = L2StandardERC20(_l2Token).l1Token();
        bytes memory message;

        if (_l2Token == Lib_PredeployAddresses.OVM_ETH) {
            // do we need to require that the l1 token is 0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE ? Probably not
            message = abi.encodeWithSelector(
                        iOVM_L1StandardBridge.finalizeETHWithdrawal.selector,
                        _from,
                        _to,
                        _amount,
                        _data
                    );
        } else {
            message = abi.encodeWithSelector(
                        iOVM_L1ERC20Bridge.finalizeERC20Withdrawal.selector,
                        l1Token,
                        _l2Token,
                        _from,
                        _to,
                        _amount,
                        _data
                    );
        }

        // Send message up to L1 bridge
        sendCrossDomainMessage(
            l1TokenBridge,
            0,
            message
        );

        emit WithdrawalInitiated(l1Token, _l2Token, msg.sender, _to, _amount, _data);
    }

    /************************************
     * Cross-chain Function: Depositing *
     ************************************/

    /**
     * @inheritdoc iOVM_L2ERC20Bridge
     */
    function finalizeDeposit(
        address _l1Token,
        address _l2Token,
        address _from,
        address _to,
        uint256 _amount,
        bytes calldata _data
    )
        external
        override
        virtual
        onlyFromCrossDomainAccount(l1TokenBridge)
    {
        // Verify the deposited token on L1 matches the L2 deposited token representation here
        // Otherwise immediately queue a withdrawal
        if(_l1Token != L2StandardERC20(_l2Token).l1Token()) {

            bytes memory message = abi.encodeWithSelector(
                iOVM_L1ERC20Bridge.finalizeERC20Withdrawal.selector,
                _l1Token,
                _l2Token,
                _to,   // switched the _to and _from here to bounce back the deposit to the sender
                _from,
                _amount,
                _data
            );

            // Send message up to L1 bridge
            sendCrossDomainMessage(
                l1TokenBridge,
                0,
                message
            );
        } else {
            // When a deposit is finalized, we credit the account on L2 with the same amount of tokens.
            L2StandardERC20(_l2Token).mint(_to, _amount);
            emit DepositFinalized(_l1Token, _l2Token, _from, _to, _amount, _data);
        }
    }
}
