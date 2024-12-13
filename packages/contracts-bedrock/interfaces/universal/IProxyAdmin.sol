// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { IAddressManager } from "interfaces/legacy/IAddressManager.sol";

interface IProxyAdmin {
    enum ProxyType {
        ERC1967,
        CHUGSPLASH,
        RESOLVED
    }

    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);

    function addressManager() external view returns (IAddressManager);
    function changeProxyAdmin(address payable _proxy, address _newAdmin) external;
    function getProxyAdmin(address payable _proxy) external view returns (address);
    function getProxyImplementation(address _proxy) external view returns (address);
    function implementationName(address) external view returns (string memory);
    function isUpgrading() external view returns (bool);
    function owner() external view returns (address);
    function proxyType(address) external view returns (ProxyType);
    function renounceOwnership() external;
    function setAddress(string memory _name, address _address) external;
    function setAddressManager(IAddressManager _address) external;
    function setImplementationName(address _address, string memory _name) external;
    function setProxyType(address _address, ProxyType _type) external;
    function setUpgrading(bool _upgrading) external;
    function transferOwnership(address newOwner) external; // nosemgrep
    function upgrade(address payable _proxy, address _implementation) external;
    function upgradeAndCall(address payable _proxy, address _implementation, bytes memory _data) external payable;

    function __constructor__(address _owner) external;
}
