// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { GnosisSafe as Safe } from "safe-contracts/GnosisSafe.sol";
import { Enum } from "safe-contracts/common/Enum.sol";
import { ISemver } from "src/universal/interfaces/ISemver.sol";

interface ILivenessGuard is ISemver {
    event OwnerRecorded(address owner);

    function lastLive(address) external view returns (uint256);
    function version() external view returns (string memory);
    function __constructor__(Safe _safe) external;

    function safe() external view returns (Safe safe_);
    function checkTransaction(
        address _to,
        uint256 _value,
        bytes memory _data,
        Enum.Operation _operation,
        uint256 _safeTxGas,
        uint256 _baseGas,
        uint256 _gasPrice,
        address _gasToken,
        address payable _refundReceiver,
        bytes memory _signatures,
        address _msgSender
    )
        external;
    function checkAfterExecution(bytes32, bool) external;
    function showLiveness() external;
}
