// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { Ownable } from "@openzeppelin/contracts/access/Ownable.sol";

// Libraries
import { Constants } from "src/libraries/Constants.sol";

// Interfaces
import { IAddressManager } from "interfaces/legacy/IAddressManager.sol";
import { IL1ChugSplashProxy } from "interfaces/legacy/IL1ChugSplashProxy.sol";
import { IStaticL1ChugSplashProxy } from "interfaces/legacy/IL1ChugSplashProxy.sol";
import { IStaticERC1967Proxy } from "interfaces/universal/IStaticERC1967Proxy.sol";
import { IProxy } from "interfaces/universal/IProxy.sol";

/// @title ProxyAdmin
/// @notice This is an auxiliary contract meant to be assigned as the admin of an ERC1967 Proxy,
///         based on the OpenZeppelin implementation. It has backwards compatibility logic to work
///         with the various types of proxies that have been deployed by Optimism in the past.
contract ProxyAdmin is Ownable {
    /// @notice The proxy types that the ProxyAdmin can manage.
    /// @custom:value ERC1967    Represents an ERC1967 compliant transparent proxy interface.
    /// @custom:value CHUGSPLASH Represents the Chugsplash proxy interface (legacy).
    /// @custom:value RESOLVED   Represents the ResolvedDelegate proxy (legacy).
    enum ProxyType {
        ERC1967,
        CHUGSPLASH,
        RESOLVED
    }

    /// @notice A mapping of proxy types, used for backwards compatibility.
    mapping(address => ProxyType) public proxyType;

    /// @notice A reverse mapping of addresses to names held in the AddressManager. This must be
    ///         manually kept up to date with changes in the AddressManager for this contract
    ///         to be able to work as an admin for the ResolvedDelegateProxy type.
    mapping(address => string) public implementationName;

    /// @notice The address of the address manager, this is required to manage the
    ///         ResolvedDelegateProxy type.
    IAddressManager public addressManager;

    /// @notice A legacy upgrading indicator used by the old Chugsplash Proxy.
    bool internal upgrading;

    /// @param _owner Address of the initial owner of this contract.
    constructor(address _owner) Ownable() {
        _transferOwnership(_owner);
    }

    /// @notice Sets the proxy type for a given address. Only required for non-standard (legacy)
    ///         proxy types.
    /// @param _address Address of the proxy.
    /// @param _type    Type of the proxy.
    function setProxyType(address _address, ProxyType _type) external onlyOwner {
        proxyType[_address] = _type;
    }

    /// @notice Sets the implementation name for a given address. Only required for
    ///         ResolvedDelegateProxy type proxies that have an implementation name.
    /// @param _address Address of the ResolvedDelegateProxy.
    /// @param _name    Name of the implementation for the proxy.
    function setImplementationName(address _address, string memory _name) external onlyOwner {
        implementationName[_address] = _name;
    }

    /// @notice Set the address of the AddressManager. This is required to manage legacy
    ///         ResolvedDelegateProxy type proxy contracts.
    /// @param _address Address of the AddressManager.
    function setAddressManager(IAddressManager _address) external onlyOwner {
        addressManager = _address;
    }

    /// @custom:legacy
    /// @notice Set an address in the address manager. Since only the owner of the AddressManager
    ///         can directly modify addresses and the ProxyAdmin will own the AddressManager, this
    ///         gives the owner of the ProxyAdmin the ability to modify addresses directly.
    /// @param _name    Name to set within the AddressManager.
    /// @param _address Address to attach to the given name.
    function setAddress(string memory _name, address _address) external onlyOwner {
        addressManager.setAddress(_name, _address);
    }

    /// @custom:legacy
    /// @notice Set the upgrading status for the Chugsplash proxy type.
    /// @param _upgrading Whether or not the system is upgrading.
    function setUpgrading(bool _upgrading) external onlyOwner {
        upgrading = _upgrading;
    }

    /// @custom:legacy
    /// @notice Legacy function used to tell ChugSplashProxy contracts if an upgrade is happening.
    /// @return Whether or not there is an upgrade going on. May not actually tell you whether an
    ///         upgrade is going on, since we don't currently plan to use this variable for anything
    ///         other than a legacy indicator to fix a UX bug in the ChugSplash proxy.
    function isUpgrading() external view returns (bool) {
        return upgrading;
    }

    /// @notice Returns the implementation of the given proxy address.
    /// @param _proxy Address of the proxy to get the implementation of.
    /// @return Address of the implementation of the proxy.
    function getProxyImplementation(address _proxy) external view returns (address) {
        ProxyType ptype = proxyType[_proxy];
        if (ptype == ProxyType.ERC1967) {
            return IStaticERC1967Proxy(_proxy).implementation();
        } else if (ptype == ProxyType.CHUGSPLASH) {
            return IStaticL1ChugSplashProxy(_proxy).getImplementation();
        } else if (ptype == ProxyType.RESOLVED) {
            return addressManager.getAddress(implementationName[_proxy]);
        } else {
            revert("ProxyAdmin: unknown proxy type");
        }
    }

    /// @notice Returns the admin of the given proxy address.
    /// @param _proxy Address of the proxy to get the admin of.
    /// @return Address of the admin of the proxy.
    function getProxyAdmin(address payable _proxy) external view returns (address) {
        ProxyType ptype = proxyType[_proxy];
        if (ptype == ProxyType.ERC1967) {
            return IStaticERC1967Proxy(_proxy).admin();
        } else if (ptype == ProxyType.CHUGSPLASH) {
            return IStaticL1ChugSplashProxy(_proxy).getOwner();
        } else if (ptype == ProxyType.RESOLVED) {
            return addressManager.owner();
        } else {
            revert("ProxyAdmin: unknown proxy type");
        }
    }

    /// @notice Updates the admin of the given proxy address.
    /// @param _proxy    Address of the proxy to update.
    /// @param _newAdmin Address of the new proxy admin.
    function changeProxyAdmin(address payable _proxy, address _newAdmin) external onlyOwner {
        ProxyType ptype = proxyType[_proxy];
        if (ptype == ProxyType.ERC1967) {
            IProxy(_proxy).changeAdmin(_newAdmin);
        } else if (ptype == ProxyType.CHUGSPLASH) {
            IL1ChugSplashProxy(_proxy).setOwner(_newAdmin);
        } else if (ptype == ProxyType.RESOLVED) {
            addressManager.transferOwnership(_newAdmin);
        } else {
            revert("ProxyAdmin: unknown proxy type");
        }
    }

    /// @notice Changes a proxy's implementation contract.
    /// @param _proxy          Address of the proxy to upgrade.
    /// @param _implementation Address of the new implementation address.
    function upgrade(address payable _proxy, address _implementation) public onlyOwner {
        ProxyType ptype = proxyType[_proxy];
        if (ptype == ProxyType.ERC1967) {
            IProxy(_proxy).upgradeTo(_implementation);
        } else if (ptype == ProxyType.CHUGSPLASH) {
            IL1ChugSplashProxy(_proxy).setStorage(
                Constants.PROXY_IMPLEMENTATION_ADDRESS, bytes32(uint256(uint160(_implementation)))
            );
        } else if (ptype == ProxyType.RESOLVED) {
            string memory name = implementationName[_proxy];
            addressManager.setAddress(name, _implementation);
        } else {
            // It should not be possible to retrieve a ProxyType value which is not matched by
            // one of the previous conditions.
            assert(false);
        }
    }

    /// @notice Changes a proxy's implementation contract and delegatecalls the new implementation
    ///         with some given data. Useful for atomic upgrade-and-initialize calls.
    /// @param _proxy          Address of the proxy to upgrade.
    /// @param _implementation Address of the new implementation address.
    /// @param _data           Data to trigger the new implementation with.
    function upgradeAndCall(
        address payable _proxy,
        address _implementation,
        bytes memory _data
    )
        external
        payable
        onlyOwner
    {
        ProxyType ptype = proxyType[_proxy];
        if (ptype == ProxyType.ERC1967) {
            IProxy(_proxy).upgradeToAndCall{ value: msg.value }(_implementation, _data);
        } else {
            // reverts if proxy type is unknown
            upgrade(_proxy, _implementation);
            (bool success,) = _proxy.call{ value: msg.value }(_data);
            require(success, "ProxyAdmin: call to proxy after upgrade failed");
        }
    }
}
