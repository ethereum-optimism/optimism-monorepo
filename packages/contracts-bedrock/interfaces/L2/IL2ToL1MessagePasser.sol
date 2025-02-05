// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

interface IL2ToL1MessagePasser {
    event MessagePassed(
        uint256 indexed nonce,
        address indexed sender,
        address indexed target,
        uint256 value,
        uint256 gasLimit,
        bytes data,
        bytes32 withdrawalHash
    );
    event WithdrawerBalanceBurnt(uint256 indexed amount);

    receive() external payable;

    function MESSAGE_VERSION() external view returns (uint16);
    function burn() external;
    function initiateWithdrawal(address _target, uint256 _gasLimit, bytes memory _data) external payable;
    function messageNonce() external view returns (uint256);
    function sentMessages(bytes32) external view returns (bool);
    function version() external view returns (string memory);

    function __constructor__() external;
}
