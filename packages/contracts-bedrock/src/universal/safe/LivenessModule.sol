// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Safe
import { GnosisSafe as Safe } from "safe-contracts/GnosisSafe.sol";
import { Enum } from "safe-contracts/common/Enum.sol";
import { OwnerManager } from "safe-contracts/base/OwnerManager.sol";

// Contracts
import { LivenessGuard } from "src/safe/LivenessGuard.sol";

// Interfaces
import { ISemver } from "interfaces/universal/ISemver.sol";

/// @title LivenessModule
/// @notice This module is intended to be used in conjunction with the LivenessGuard. In the event
///         that an owner of the safe is not recorded by the guard during the liveness interval,
///         the owner will be considered inactive and will be removed from the list of owners.
///         If the number of owners falls below the minimum number of owners, the ownership of the
///         safe will be transferred to the fallback owner.
contract LivenessModule is ISemver {
    /// @notice Error message for failed owner removal.
    error OwnerRemovalFailed(string);

    /// @notice Emitted when an owner is removed due to insufficient liveness
    event RemovedOwner(address indexed owner);

    /// @notice Emitted when the fallback owner takes ownership
    event OwnershipTransferredToFallback();

    /// @notice Flag to indicate if the module has been deactivated
    bool public ownershipTransferredToFallback;

    /// @notice The Safe contract instance
    Safe internal immutable SAFE;

    /// @notice The LivenessGuard contract instance
    ///         This can be updated by replacing with a new module and switching out the guard.
    LivenessGuard internal immutable LIVENESS_GUARD;

    /// @notice The interval, in seconds, during which an owner must have demonstrated liveness
    ///         This can be updated by replacing with a new module.
    uint256 internal immutable LIVENESS_INTERVAL;

    /// @notice The minimum number of owners before ownership of the safe is transferred to the fallback owner.
    ///         This can be updated by replacing with a new module.
    uint256 internal immutable MIN_OWNERS;

    /// @notice The percentage used to calculate the threshold for the Safe.
    uint256 internal immutable THRESHOLD_PERCENTAGE;

    /// @notice The fallback owner of the Safe
    ///         This can be updated by replacing with a new module.
    address internal immutable FALLBACK_OWNER;

    /// @notice The storage slot used in the safe to store the guard address
    ///         keccak256("guard_manager.guard.address")
    uint256 internal constant GUARD_STORAGE_SLOT = 0x4a204f620c8c5ccdca3fd54d003badd85ba500436a431f0cbda4f558c93c34c8;

    /// @notice Semantic version.
    /// @custom:semver 1.2.1-beta.3
    string public constant version = "1.2.1-beta.3";

    // Constructor to initialize the Safe and baseModule instances
    constructor(
        Safe _safe,
        LivenessGuard _livenessGuard,
        uint256 _livenessInterval,
        uint256 _minOwners,
        uint256 _thresholdPercentage,
        address _fallbackOwner
    ) {
        SAFE = _safe;
        LIVENESS_GUARD = _livenessGuard;
        LIVENESS_INTERVAL = _livenessInterval;
        THRESHOLD_PERCENTAGE = _thresholdPercentage;
        FALLBACK_OWNER = _fallbackOwner;
        MIN_OWNERS = _minOwners;
        address[] memory owners = _safe.getOwners();
        require(_minOwners <= owners.length, "LivenessModule: minOwners must be less than the number of owners");
        require(_thresholdPercentage > 0, "LivenessModule: thresholdPercentage must be greater than 0");
        require(_thresholdPercentage <= 100, "LivenessModule: thresholdPercentage must be less than or equal to 100");
    }

    /// @notice For a given number of owners, return the lowest threshold which is greater than the required percentage.
    ///         Note: this function returns 1 for numOwners == 1.
    function getRequiredThreshold(uint256 _numOwners) public view returns (uint256 threshold_) {
        threshold_ = (_numOwners * THRESHOLD_PERCENTAGE + 99) / 100;
    }

    /// @notice Getter function for the Safe contract instance
    /// @return safe_ The Safe contract instance
    function safe() public view returns (Safe safe_) {
        safe_ = SAFE;
    }

    /// @notice Getter function for the LivenessGuard contract instance
    /// @return livenessGuard_ The LivenessGuard contract instance
    function livenessGuard() public view returns (LivenessGuard livenessGuard_) {
        livenessGuard_ = LIVENESS_GUARD;
    }

    /// @notice Getter function for the liveness interval
    /// @return livenessInterval_ The liveness interval, in seconds
    function livenessInterval() public view returns (uint256 livenessInterval_) {
        livenessInterval_ = LIVENESS_INTERVAL;
    }

    /// @notice Getter function for the minimum number of owners
    /// @return minOwners_ The minimum number of owners
    function minOwners() public view returns (uint256 minOwners_) {
        minOwners_ = MIN_OWNERS;
    }

    /// @notice Getter function for the required threshold percentage
    /// @return thresholdPercentage_ The minimum number of owners
    function thresholdPercentage() public view returns (uint256 thresholdPercentage_) {
        thresholdPercentage_ = THRESHOLD_PERCENTAGE;
    }

    /// @notice Getter function for the fallback
    /// @return fallbackOwner_ The fallback owner of the Safe
    function fallbackOwner() public view returns (address fallbackOwner_) {
        fallbackOwner_ = FALLBACK_OWNER;
    }

    /// @notice Checks if the owner can be removed
    /// @param _owner The owner to be removed
    /// @return canRemove_ bool indicating if the owner can be removed
    function canRemove(address _owner) public view returns (bool canRemove_) {
        require(SAFE.isOwner(_owner), "LivenessModule: the owner to remove must be an owner of the Safe");
        canRemove_ = LIVENESS_GUARD.lastLive(_owner) + LIVENESS_INTERVAL < block.timestamp;
    }

    /// @notice This function can be called by anyone to remove a set of owners that have not signed a transaction
    ///         during the liveness interval. If the number of owners drops below the minimum, then all owners
    ///         must be removed.
    /// @param _previousOwners The previous owners in the linked list of owners
    /// @param _ownersToRemove The owners to remove
    function removeOwners(address[] memory _previousOwners, address[] memory _ownersToRemove) external {
        require(_previousOwners.length == _ownersToRemove.length, "LivenessModule: arrays must be the same length");
        address[] memory currentOwners = SAFE.getOwners();
        require(
            !ownershipTransferredToFallback,
            "LivenessModule: The safe has been shutdown, the LivenessModule and LivenessGuard should be removed or replaced."
        );

        // Initialize the ownersCount count to the current number of owners, so that we can track the number of
        // owners in the Safe after each removal. The Safe will revert if an owner cannot be removed, so it is safe
        // keep a local count of the number of owners this way.
        uint256 ownersCount = currentOwners.length;
        for (uint256 i = 0; i < _previousOwners.length; i++) {
            // Validate that the owner can be removed, which means that either:
            //   1. the ownersCount is now less than MIN_OWNERS, in which case all owners should be removed regardless
            //      of liveness,
            //   2. the owner has not signed a transaction during the liveness interval.
            if (ownersCount >= MIN_OWNERS) {
                require(canRemove(_ownersToRemove[i]), "LivenessModule: the owner to remove has signed recently");
            }

            // Pre-emptively update our local count of the number of owners.
            // This is safe because _removeOwner will bubble up any revert from the Safe if the owner cannot be removed.
            ownersCount--;

            // We now attempt remove the owner from the safe.
            _removeOwner({
                _prevOwner: _previousOwners[i],
                _ownerToRemove: _ownersToRemove[i],
                _newOwnersCount: ownersCount
            });

            // when all owners are removed and the sole owner is the fallback owner, the
            // ownersCount variable will be incorrectly set to zero.
            // This reflects the fact that all prior owners have been removed. The loop should naturally exit at this
            // point, but for safety we detect this condition and force the loop to terminate.
            if (ownersCount == 0) {
                break;
            }
        }
        _verifyFinalState();
    }

    /// @notice Removes an owner from the Safe and updates the threshold.
    /// @param _prevOwner Owner that pointed to the owner to be removed in the linked list
    /// @param _ownerToRemove Owner address to be removed.
    /// @param _newOwnersCount New number of owners after removal.
    function _removeOwner(address _prevOwner, address _ownerToRemove, uint256 _newOwnersCount) internal {
        if (_newOwnersCount > 0) {
            uint256 newThreshold = getRequiredThreshold(_newOwnersCount);
            // Remove the owner and update the threshold
            _removeOwnerSafeCall({ _prevOwner: _prevOwner, _owner: _ownerToRemove, _threshold: newThreshold });
        } else {
            // There is only one owner left. The Safe will not allow a safe with no owners, so we will
            // need to swap owners instead.
            _swapToFallbackOwnerSafeCall({ _prevOwner: _prevOwner, _oldOwner: _ownerToRemove });
        }
    }

    /// @notice Sets the fallback owner as the sole owner of the Safe with a threshold of 1
    /// @param _prevOwner Owner that pointed to the owner to be replaced in the linked list
    /// @param _oldOwner Owner address to be replaced.
    function _swapToFallbackOwnerSafeCall(address _prevOwner, address _oldOwner) internal {
        (bool success, bytes memory returnData) = SAFE.execTransactionFromModuleReturnData({
            to: address(SAFE),
            value: 0,
            operation: Enum.Operation.Call,
            data: abi.encodeCall(OwnerManager.swapOwner, (_prevOwner, _oldOwner, FALLBACK_OWNER))
        });
        if (!success) {
            revert OwnerRemovalFailed(string(returnData));
        }

        // Deactivate the module to prevent unintended behavior after the fallback owner has taken ownership.
        ownershipTransferredToFallback = true;
        emit OwnershipTransferredToFallback();
    }

    /// @notice Removes the owner `owner` from the Safe and updates the threshold to `_threshold`.
    /// @param _prevOwner Owner that pointed to the owner to be removed in the linked list
    /// @param _owner Owner address to be removed.
    /// @param _threshold New threshold.
    function _removeOwnerSafeCall(address _prevOwner, address _owner, uint256 _threshold) internal {
        (bool success, bytes memory returnData) = SAFE.execTransactionFromModuleReturnData({
            to: address(SAFE),
            value: 0,
            operation: Enum.Operation.Call,
            data: abi.encodeCall(OwnerManager.removeOwner, (_prevOwner, _owner, _threshold))
        });
        if (!success) {
            revert OwnerRemovalFailed(string(returnData));
        }
        emit RemovedOwner(_owner);
    }

    /// @notice A FREI-PI invariant check enforcing requirements on number of owners and threshold.
    function _verifyFinalState() internal view {
        address[] memory owners = SAFE.getOwners();
        uint256 numOwners = owners.length;

        // Ensure that the safe is not being left in a safe state such that either:
        //  1. there are at least the minimum number of owners, or
        //  2. there is a single owner and that owner is the fallback owner.
        if (numOwners == 1) {
            require(owners[0] == FALLBACK_OWNER, "LivenessModule: must transfer ownership to fallback owner");
        } else {
            require(
                numOwners >= MIN_OWNERS,
                "LivenessModule: must remove all owners and transfer to fallback owner if numOwners < minOwners"
            );
        }

        // Check that"LivenessModule: must remove all owners and transfer to fallback owner if numOwners < minOwners"
        // the threshold is correct. This check is also correct when there is a single
        // owner, because getRequiredThreshold(1) returns 1.
        uint256 threshold = SAFE.getThreshold();
        require(
            threshold == getRequiredThreshold(numOwners),
            "LivenessModule: Insufficient threshold for the number of owners"
        );

        // Check that the guard has not been changed
        require(
            address(LIVENESS_GUARD) == address(uint160(uint256(bytes32(SAFE.getStorageAt(GUARD_STORAGE_SLOT, 1))))),
            "LivenessModule: guard has been changed"
        );
    }
}
