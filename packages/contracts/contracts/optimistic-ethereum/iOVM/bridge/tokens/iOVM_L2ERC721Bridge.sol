// SPDX-License-Identifier: MIT
pragma solidity >0.5.0;
pragma experimental ABIEncoderV2;

/**
 * @title iOVM_L2ERC721Bridge
 */
interface iOVM_L2ERC721Bridge {

    /**********
     * Events *
     **********/

    event ERC721WithdrawalInitiated (
        address indexed _l1Token,
        address indexed _l2Token,
        address indexed _from,
        address _to,
        uint256 _tokenId,
        bytes _data
    );

    event ERC721DepositFinalized (
        address indexed _l1Token,
        address indexed _l2Token,
        address indexed _from,
        address _to,
        uint256 _tokenId,
        bytes _data
    );

    event ERC721DepositFailed (
        address indexed _l1Token,
        address indexed _l2Token,
        address indexed _from,
        address _to,
        uint256 _tokenId,
        bytes _data
    );


    /********************
     * Public Functions *
     ********************/

    /**
     * @dev initiate a withdrawal of some NFT to the caller's account on L1.
     * @param _l2Token Address of L2 token where withdrawal was initiated.
     * @param _tokenId The NFT to withdraw.
     * @param _l1Gas Unused, but included for potential forward compatibility considerations.
     * @param _data Optional data to forward to L1. This data is provided
     *        solely as a convenience for external contracts. Aside from enforcing a maximum
     *        length, these contracts provide no guarantees about its content.
     */
    function withdrawERC721 (
        address _l2Token,
        uint256 _tokenId,
        uint32 _l1Gas,
        bytes calldata _data
    )
        external;

    /**
     * @dev initiate a withdrawal of some NFT to a recipient's account on L1.
     * @param _l2Token Address of L2 token where withdrawal is initiated.
     * @param _to L1 adress to credit the withdrawal to.
     * @param _tokenId The NFT to withdraw.
     * @param _l1Gas Unused, but included for potential forward compatibility considerations.
     * @param _data Optional data to forward to L1. This data is provided
     *        solely as a convenience for external contracts. Aside from enforcing a maximum
     *        length, these contracts provide no guarantees about its content.
     */
    function withdrawERC721To (
        address _l2Token,
        address _to,
        uint256 _tokenId,
        uint32 _l1Gas,
        bytes calldata _data
    )
        external;

    /*************************
     * Cross-chain Functions *
     *************************/

    /**
     * @dev Complete a deposit from L1 to L2, and credits the NFT to the recipient of this
     * L2 token. This call will fail if it did not originate from a corresponding deposit in
     * OVM_l1TokenGateway.
     * @param _l1Token Address for the l1 token this is called with.
     * @param _l2Token Address for the l2 token this is called with.
     * @param _from Account to pull the deposit from on L2.
     * @param _to Address to receive the withdrawal at.
     * @param _tokenId The NFT token to deposit.
     * @param _data Data provider by the sender on L1. This data is provided
     *        solely as a convenience for external contracts. Aside from enforcing a maximum
     *        length, these contracts provide no guarantees about its content.
     */
    function finalizeERC721Deposit (
        address _l1Token,
        address _l2Token,
        address _from,
        address _to,
        uint256 _tokenId,
        bytes calldata _data
    )
        external;
}
