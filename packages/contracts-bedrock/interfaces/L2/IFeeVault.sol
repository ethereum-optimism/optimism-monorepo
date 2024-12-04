// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

library Types {
    enum WithdrawalNetwork {
        L1,
        L2
    }
}

interface IFeeVault {
    event Withdrawal(uint256 value, address to, address from);
    event Withdrawal(uint256 value, address to, address from, Types.WithdrawalNetwork withdrawalNetwork);

    receive() external payable;

    function MIN_WITHDRAWAL_AMOUNT() external view returns (uint256);
    function RECIPIENT() external view returns (address);
    function WITHDRAWAL_NETWORK() external view returns (Types.WithdrawalNetwork);
    function minWithdrawalAmount() external view returns (uint256 amount_);
    function recipient() external view returns (address recipient_);
    function totalProcessed() external view returns (uint256);
    function withdraw() external;
    function withdrawalNetwork() external view returns (Types.WithdrawalNetwork network_);

    function __constructor__() external;
}
