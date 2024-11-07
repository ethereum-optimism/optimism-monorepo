// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Initializable } from "@openzeppelin/contracts/proxy/utils/Initializable.sol";
import { ISemver } from "src/universal/interfaces/ISemver.sol";
import { Storage } from "src/libraries/Storage.sol";

/// @custom:proxied true
/// @custom:audit none This contracts is not yet audited.
/// @title SuperchainConfig
/// @notice The SuperchainConfig contract is used to manage configuration of global superchain values.
contract SuperchainConfig is Initializable, ISemver {
    /// @notice Enum representing different types of updates.
    /// @custom:value GUARDIAN            Represents an update to the guardian.
    enum UpdateType {
        GUARDIAN
    }

    /// @notice Whether or not the Superchain is paused.
    bytes32 public constant PAUSED_SLOT = bytes32(uint256(keccak256("superchainConfig.paused")) - 1);

    /// @notice The address of the guardian, which can pause withdrawals from the System.
    ///         It can only be modified by an upgrade.
    bytes32 public constant GUARDIAN_SLOT = bytes32(uint256(keccak256("superchainConfig.guardian")) - 1);

    /// @notice Emitted when the pause is triggered.
    /// @param identifier A string helping to identify provenance of the pause transaction.
    event Paused(string identifier);

    /// @notice Emitted when the pause is lifted.
    event Unpaused();

    /// @notice Emitted when configuration is updated.
    /// @param updateType Type of update.
    /// @param data       Encoded update data.
    event ConfigUpdate(UpdateType indexed updateType, bytes data);

    /// @notice Semantic version.
    /// @custom:semver 1.1.1-beta.1
    string public constant version = "1.1.1-beta.1";

    /// @notice Constructs the SuperchainConfig contract.
    constructor() {
        initialize({ _guardian: address(0), _paused: false });
    }

    /// @notice Initializer.
    /// @param _guardian    Address of the guardian, can pause the OptimismPortal.
    /// @param _paused      Initial paused status.
    function initialize(address _guardian, bool _paused) public initializer {
        _setGuardian(_guardian);
        if (_paused) {
            _pause("Initializer paused");
        }
    }

    /// @notice Getter for the guardian address.
    function guardian() public view returns (address guardian_) {
        guardian_ = Storage.getAddress(GUARDIAN_SLOT);
    }

    /// @notice Getter for the current paused status.
    function paused() public view returns (bool paused_) {
        paused_ = Storage.getBool(PAUSED_SLOT);
    }

    /// @notice Pauses withdrawals.
    /// @param _identifier (Optional) A string to identify provenance of the pause transaction.
    function pause(string memory _identifier) external {
        require(msg.sender == guardian(), "SuperchainConfig: only guardian can pause");
        _pause(_identifier);
    }

    /// @notice Pauses withdrawals.
    /// @param _identifier (Optional) A string to identify provenance of the pause transaction.
    function _pause(string memory _identifier) internal {
        Storage.setBool(PAUSED_SLOT, true);
        emit Paused(_identifier);
    }

    /// @notice Unpauses withdrawals.
    function unpause() external {
        require(msg.sender == guardian(), "SuperchainConfig: only guardian can unpause");
        Storage.setBool(PAUSED_SLOT, false);
        emit Unpaused();
    }

    /// @notice Sets the guardian address. This is only callable during initialization, so an upgrade
    ///         will be required to change the guardian.
    /// @param _guardian The new guardian address.
    function _setGuardian(address _guardian) internal {
        Storage.setAddress(GUARDIAN_SLOT, _guardian);
        emit ConfigUpdate(UpdateType.GUARDIAN, abi.encode(_guardian));
    }
}
