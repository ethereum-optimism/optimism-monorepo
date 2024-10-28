// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { GnosisSafe as Safe } from "safe-contracts/GnosisSafe.sol";
import { LivenessGuard } from "src/safe/LivenessGuard.sol";
import { ISemver } from "src/universal/interfaces/ISemver.sol";

interface ILivenessModule is ISemver {
    error OwnerRemovalFailed(string);

    event RemovedOwner(address indexed owner);
    event OwnershipTransferredToFallback();

    function ownershipTransferredToFallback() external view returns (bool);
    function version() external view returns (string memory);
    function __constructor__(
        Safe _safe,
        LivenessGuard _livenessGuard,
        uint256 _livenessInterval,
        uint256 _minOwners,
        uint256 _thresholdPercentage,
        address _fallbackOwner
    )
        external;
    function getRequiredThreshold(uint256 _numOwners) external view returns (uint256 threshold_);
    function safe() external view returns (Safe safe_);
    function livenessGuard() external view returns (LivenessGuard livenessGuard_);
    function livenessInterval() external view returns (uint256 livenessInterval_);
    function minOwners() external view returns (uint256 minOwners_);
    function thresholdPercentage() external view returns (uint256 thresholdPercentage_);
    function fallbackOwner() external view returns (address fallbackOwner_);
    function canRemove(address _owner) external view returns (bool canRemove_);
    function removeOwners(address[] memory _previousOwners, address[] memory _ownersToRemove) external;
}
