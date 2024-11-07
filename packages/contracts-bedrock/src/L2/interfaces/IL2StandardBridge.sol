// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { IStandardBridge } from "src/universal/interfaces/IStandardBridge.sol";

interface IL2StandardBridge is IStandardBridge {
    event DepositFinalized(
        address indexed l1Token,
        address indexed l2Token,
        address indexed from,
        address to,
        uint256 amount,
        bytes extraData
    );
    event WithdrawalInitiated(
        address indexed l1Token,
        address indexed l2Token,
        address indexed from,
        address to,
        uint256 amount,
        bytes extraData
    );

    receive() external payable;

    function initialize(IStandardBridge _otherBridge) external;
    function l1TokenBridge() external view returns (address);
    function version() external pure returns (string memory);
    function withdraw(
        address _l2Token,
        uint256 _amount,
        uint32 _minGasLimit,
        bytes memory _extraData
    )
        external
        payable;
    function withdrawTo(
        address _l2Token,
        address _to,
        uint256 _amount,
        uint32 _minGasLimit,
        bytes memory _extraData
    )
        external
        payable;

    function __constructor__() external;
}
