// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { CrossDomainMessenger } from "src/universal/CrossDomainMessenger.sol";

// Libraries
import { Predeploys } from "src/libraries/Predeploys.sol";

// Interfaces
import { ISemver } from "src/universal/interfaces/ISemver.sol";
import { ISuperchainConfig } from "src/L1/interfaces/ISuperchainConfig.sol";
import { ISystemConfig } from "src/L1/interfaces/ISystemConfig.sol";
import { IOptimismPortal } from "src/L1/interfaces/IOptimismPortal.sol";

/// @custom:proxied true
/// @title L1CrossDomainMessenger
/// @notice The L1CrossDomainMessenger is a message passing interface between L1 and L2 responsible
///         for sending and receiving data on the L1 side. Users are encouraged to use this
///         interface instead of interacting with lower-level contracts directly.
contract L1CrossDomainMessenger is CrossDomainMessenger, ISemver {
    /// @notice Contract of the SuperchainConfig.
    ISuperchainConfig public superchainConfig;

    /// @notice Contract of the OptimismPortal.
    /// @custom:network-specific
    IOptimismPortal public portal;

    /// @notice Address of the SystemConfig contract.
    ISystemConfig public systemConfig;

    /// @notice Semantic version.
    /// @custom:semver 2.4.1-beta.2
    string public constant version = "2.4.1-beta.2";

    /// @notice Constructs the L1CrossDomainMessenger contract.
    constructor() CrossDomainMessenger() {
        initialize({
            _superchainConfig: ISuperchainConfig(address(0)),
            _portal: IOptimismPortal(payable(address(0))),
            _systemConfig: ISystemConfig(address(0))
        });
    }

    /// @notice Initializes the contract.
    /// @param _superchainConfig Contract of the SuperchainConfig contract on this network.
    /// @param _portal Contract of the OptimismPortal contract on this network.
    /// @param _systemConfig Contract of the SystemConfig contract on this network.
    function initialize(
        ISuperchainConfig _superchainConfig,
        IOptimismPortal _portal,
        ISystemConfig _systemConfig
    )
        public
        initializer
    {
        superchainConfig = _superchainConfig;
        portal = _portal;
        systemConfig = _systemConfig;
        __CrossDomainMessenger_init({ _otherMessenger: CrossDomainMessenger(Predeploys.L2_CROSS_DOMAIN_MESSENGER) });
    }

    /// @inheritdoc CrossDomainMessenger
    function gasPayingToken() internal view override returns (address addr_, uint8 decimals_) {
        (addr_, decimals_) = systemConfig.gasPayingToken();
    }

    /// @notice Getter function for the OptimismPortal contract on this chain.
    ///         Public getter is legacy and will be removed in the future. Use `portal()` instead.
    /// @return Contract of the OptimismPortal on this chain.
    /// @custom:legacy
    function PORTAL() external view returns (IOptimismPortal) {
        return portal;
    }

    /// @inheritdoc CrossDomainMessenger
    function _sendMessage(address _to, uint64 _gasLimit, uint256 _value, bytes memory _data) internal override {
        portal.depositTransaction{ value: _value }({
            _to: _to,
            _value: _value,
            _gasLimit: _gasLimit,
            _isCreation: false,
            _data: _data
        });
    }

    /// @inheritdoc CrossDomainMessenger
    function _isOtherMessenger() internal view override returns (bool) {
        return msg.sender == address(portal) && portal.l2Sender() == address(otherMessenger);
    }

    /// @inheritdoc CrossDomainMessenger
    function _isUnsafeTarget(address _target) internal view override returns (bool) {
        return _target == address(this) || _target == address(portal);
    }

    /// @inheritdoc CrossDomainMessenger
    function paused() public view override returns (bool) {
        return superchainConfig.paused();
    }
}
