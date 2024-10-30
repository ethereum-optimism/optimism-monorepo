// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { IStandardBridge } from "src/universal/interfaces/IStandardBridge.sol";
import { ICrossDomainMessenger } from "src/universal/interfaces/ICrossDomainMessenger.sol";

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

    function l1TokenBridge() external view returns (address);
    function MESSENGER() external view returns (ICrossDomainMessenger);
    function messenger() external pure returns (ICrossDomainMessenger);

    function OTHER_BRIDGE() external view returns (IStandardBridge);
    function otherBridge() external view returns (IStandardBridge);

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
