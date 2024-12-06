// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Libraries
import { Constants } from "src/libraries/Constants.sol";

// Interfaces
import { IL1ChugSplashDeployer } from "interfaces/legacy/IL1ChugSplashProxy.sol";

/// @custom:legacy true
/// @title L1ChugSplashProxy
/// @notice Basic ChugSplash proxy contract for L1. Very close to being a normal proxy but has added
///         functions `setCode` and `setStorage` for changing the code or storage of the contract.
///         Note for future developers: do NOT make anything in this contract 'public' unless you
///         know what you're doing. Anything public can potentially have a function signature that
///         conflicts with a signature attached to the implementation contract. Public functions
///         SHOULD always have the `proxyCallIfNotOwner` modifier unless there's some *really* good
///         reason not to have that modifier. And there almost certainly is not a good reason to not
///         have that modifier. Beware!
contract L1ChugSplashProxy {
    /// @notice "Magic" prefix. When prepended to some arbitrary bytecode and used to create a
    ///         contract, the appended bytecode will be deployed as given.
    bytes13 internal constant DEPLOY_CODE_PREFIX = 0x600D380380600D6000396000f3;

    /// @notice Blocks a function from being called when the parent signals that the system should
    ///         be paused via an isUpgrading function.
    modifier onlyWhenNotPaused() {
        address owner = _getOwner();

        // We do a low-level call because there's no guarantee that the owner actually *is* an
        // L1ChugSplashDeployer contract and Solidity will throw errors if we do a normal call and
        // it turns out that it isn't the right type of contract.
        (bool success, bytes memory returndata) =
            owner.staticcall(abi.encodeWithSelector(IL1ChugSplashDeployer.isUpgrading.selector));

        // If the call was unsuccessful then we assume that there's no "isUpgrading" method and we
        // can just continue as normal. We also expect that the return value is exactly 32 bytes
        // long. If this isn't the case then we can safely ignore the result.
        if (success && returndata.length == 32) {
            // Although the expected value is a *boolean*, it's safer to decode as a uint256 in the
            // case that the isUpgrading function returned something other than 0 or 1. But we only
            // really care about the case where this value is 0 (= false).
            uint256 ret = abi.decode(returndata, (uint256));
            require(ret == 0, "L1ChugSplashProxy: system is currently being upgraded");
        }

        _;
    }

    /// @notice Makes a proxy call instead of triggering the given function when the caller is
    ///         either the owner or the zero address. Caller can only ever be the zero address if
    ///         this function is being called off-chain via eth_call, which is totally fine and can
    ///         be convenient for client-side tooling. Avoids situations where the proxy and
    ///         implementation share a sighash and the proxy function ends up being called instead
    ///         of the implementation one.
    ///         Note: msg.sender == address(0) can ONLY be triggered off-chain via eth_call. If
    ///         there's a way for someone to send a transaction with msg.sender == address(0) in any
    ///         real context then we have much bigger problems. Primary reason to include this
    ///         additional allowed sender is because the owner address can be changed dynamically
    ///         and we do not want clients to have to keep track of the current owner in order to
    ///         make an eth_call that doesn't trigger the proxied contract.
    // slither-disable-next-line incorrect-modifier
    modifier proxyCallIfNotOwner() {
        if (msg.sender == _getOwner() || msg.sender == address(0)) {
            _;
        } else {
            // This WILL halt the call frame on completion.
            _doProxyCall();
        }
    }

    /// @param _owner Address of the initial contract owner.
    constructor(address _owner) {
        _setOwner(_owner);
    }

    // slither-disable-next-line locked-ether
    receive() external payable {
        // Proxy call by default.
        _doProxyCall();
    }

    // slither-disable-next-line locked-ether
    fallback() external payable {
        // Proxy call by default.
        _doProxyCall();
    }

    /// @notice Sets the code that should be running behind this proxy.
    ///         Note: This scheme is a bit different from the standard proxy scheme where one would
    ///         typically deploy the code separately and then set the implementation address. We're
    ///         doing it this way because it gives us a lot more freedom on the client side. Can
    ///         only be triggered by the contract owner.
    /// @param _code New contract code to run inside this contract.
    function setCode(bytes memory _code) external proxyCallIfNotOwner {
        // Get the code hash of the current implementation.
        address implementation = _getImplementation();

        // If the code hash matches the new implementation then we return early.
        if (keccak256(_code) == _getAccountCodeHash(implementation)) {
            return;
        }

        // Create the deploycode by appending the magic prefix.
        bytes memory deploycode = abi.encodePacked(DEPLOY_CODE_PREFIX, _code);

        // Deploy the code and set the new implementation address.
        address newImplementation;
        assembly {
            newImplementation := create(0x0, add(deploycode, 0x20), mload(deploycode))
        }

        // Check that the code was actually deployed correctly. I'm not sure if you can ever
        // actually fail this check. Should only happen if the contract creation from above runs
        // out of gas but this parent execution thread does NOT run out of gas. Seems like we
        // should be doing this check anyway though.
        require(
            _getAccountCodeHash(newImplementation) == keccak256(_code),
            "L1ChugSplashProxy: code was not correctly deployed"
        );

        _setImplementation(newImplementation);
    }

    /// @notice Modifies some storage slot within the proxy contract. Gives us a lot of power to
    ///         perform upgrades in a more transparent way. Only callable by the owner.
    /// @param _key   Storage key to modify.
    /// @param _value New value for the storage key.
    function setStorage(bytes32 _key, bytes32 _value) external proxyCallIfNotOwner {
        assembly {
            sstore(_key, _value)
        }
    }

    /// @notice Changes the owner of the proxy contract. Only callable by the owner.
    /// @param _owner New owner of the proxy contract.
    function setOwner(address _owner) external proxyCallIfNotOwner {
        _setOwner(_owner);
    }

    /// @notice Queries the owner of the proxy contract. Can only be called by the owner OR by
    ///         making an eth_call and setting the "from" address to address(0).
    /// @return Owner address.
    function getOwner() external proxyCallIfNotOwner returns (address) {
        return _getOwner();
    }

    /// @notice Queries the implementation address. Can only be called by the owner OR by making an
    ///         eth_call and setting the "from" address to address(0).
    /// @return Implementation address.
    function getImplementation() external proxyCallIfNotOwner returns (address) {
        return _getImplementation();
    }

    /// @notice Sets the implementation address.
    /// @param _implementation New implementation address.
    function _setImplementation(address _implementation) internal {
        bytes32 proxyImplementation = Constants.PROXY_IMPLEMENTATION_ADDRESS;
        assembly {
            sstore(proxyImplementation, _implementation)
        }
    }

    /// @notice Changes the owner of the proxy contract.
    /// @param _owner New owner of the proxy contract.
    function _setOwner(address _owner) internal {
        bytes32 proxyOwner = Constants.PROXY_OWNER_ADDRESS;
        assembly {
            sstore(proxyOwner, _owner)
        }
    }

    /// @notice Performs the proxy call via a delegatecall.
    function _doProxyCall() internal onlyWhenNotPaused {
        address implementation = _getImplementation();

        require(implementation != address(0), "L1ChugSplashProxy: implementation is not set yet");

        assembly {
            // Copy calldata into memory at 0x0....calldatasize.
            calldatacopy(0x0, 0x0, calldatasize())

            // Perform the delegatecall, make sure to pass all available gas.
            let success := delegatecall(gas(), implementation, 0x0, calldatasize(), 0x0, 0x0)

            // Copy returndata into memory at 0x0....returndatasize. Note that this *will*
            // overwrite the calldata that we just copied into memory but that doesn't really
            // matter because we'll be returning in a second anyway.
            returndatacopy(0x0, 0x0, returndatasize())

            // Success == 0 means a revert. We'll revert too and pass the data up.
            if iszero(success) { revert(0x0, returndatasize()) }

            // Otherwise we'll just return and pass the data up.
            return(0x0, returndatasize())
        }
    }

    /// @notice Queries the implementation address.
    /// @return Implementation address.
    function _getImplementation() internal view returns (address) {
        address implementation;
        bytes32 proxyImplementation = Constants.PROXY_IMPLEMENTATION_ADDRESS;
        assembly {
            implementation := sload(proxyImplementation)
        }
        return implementation;
    }

    /// @notice Queries the owner of the proxy contract.
    /// @return Owner address.
    function _getOwner() internal view returns (address) {
        address owner;
        bytes32 proxyOwner = Constants.PROXY_OWNER_ADDRESS;
        assembly {
            owner := sload(proxyOwner)
        }
        return owner;
    }

    /// @notice Gets the code hash for a given account.
    /// @param _account Address of the account to get a code hash for.
    /// @return Code hash for the account.
    function _getAccountCodeHash(address _account) internal view returns (bytes32) {
        bytes32 codeHash;
        assembly {
            codeHash := extcodehash(_account)
        }
        return codeHash;
    }
}
