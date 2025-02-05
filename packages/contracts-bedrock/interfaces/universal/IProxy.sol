// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

interface IProxy {
    event AdminChanged(address previousAdmin, address newAdmin);
    event Upgraded(address indexed implementation);

    fallback() external payable;

    receive() external payable;

    function admin() external returns (address);
    function changeAdmin(address _admin) external;
    function implementation() external returns (address);
    function upgradeTo(address _implementation) external;
    function upgradeToAndCall(address _implementation, bytes memory _data) external payable returns (bytes memory);

    function __constructor__(address _admin) external;
}
