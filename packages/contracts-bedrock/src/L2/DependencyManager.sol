// SPDX-License-Identifier: MIT
pragma solidity 0.8.25;

// Libraries
import { EnumerableSet } from "@openzeppelin/contracts/utils/structs/EnumerableSet.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";
import { Unauthorized } from "src/libraries/errors/CommonErrors.sol";
import { Constants } from "src/libraries/Constants.sol";

// Interfaces
import { ISemver } from "interfaces/universal/ISemver.sol";
import { IL2ToL1MessagePasser } from "interfaces/L2/IL2ToL1MessagePasser.sol";
import { ISuperchainConfigInterop } from "interfaces/L1/ISuperchainConfigInterop.sol";

/// @custom:proxied true
/// @custom:predeploy 0x4200000000000000000000000000000000000029
/// @title DependencyManager
/// @notice The DependencyManager contract is used to manage the interop dependency set. This set contains the chain IDs
///         that the current chain is dependent on. When updating the dependency set, the DependencyManager will
///         initiate a withdrawal tx to update the dependency set on L1.
contract DependencyManager is ISemver {
    using EnumerableSet for EnumerableSet.UintSet;

    /// @notice Error when the interop dependency set size is too large.
    error DependencySetSizeTooLarge();

    /// @notice Error when a chain ID already in the interop dependency set is attempted to be added.
    error AlreadyDependency();

    /// @notice Event emitted when a new dependency is added to the interop dependency set.
    event DependencyAdded(uint256 indexed chainId, address indexed systemConfig, address indexed superchainConfig);

    /// @notice The minimum gas limit for the withdrawal tx to update the dependency set on L1.
    uint256 internal constant ADD_DEPENDENCY_WITHDRAWAL_GAS_LIMIT = 400_000;

    /// @notice The interop dependency set, containing the chain IDs in it.
    EnumerableSet.UintSet internal _dependencySet;

    /// @notice Semantic version.
    /// @custom:semver 1.0.0-beta.1
    string public constant version = "1.0.0-beta.1";

    /// @notice Adds a new dependency to the dependency set. This function is only callable by the derivation pipeline.
    ///         It will initiate a withdrawal tx to update the dependency set on L1.
    /// @param _superchainConfig    Address of the SuperchainConfig contract on L1.
    /// @param _chainId             The new chain's ID to add to the dependency set.
    /// @param _systemConfig        The new chain's SystemConfig contract address on L1.
    function addDependency(address _superchainConfig, uint256 _chainId, address _systemConfig) external {
        if (msg.sender != Constants.DEPOSITOR_ACCOUNT) revert Unauthorized();

        if (_dependencySet.length() == type(uint8).max) revert DependencySetSizeTooLarge();

        if (!_dependencySet.add(_chainId)) revert AlreadyDependency();

        // Initiate a withdrawal tx to update the dependency set on L1.
        IL2ToL1MessagePasser(payable(Predeploys.L2_TO_L1_MESSAGE_PASSER)).initiateWithdrawal(
            _superchainConfig,
            ADD_DEPENDENCY_WITHDRAWAL_GAS_LIMIT,
            abi.encodeCall(ISuperchainConfigInterop.addDependency, (_chainId, _systemConfig))
        );

        emit DependencyAdded(_chainId, _systemConfig, _superchainConfig);
    }

    /// @notice Returns true if a chain ID is in the interop dependency set and false otherwise.
    ///         The chain's chain ID is always considered to be in the dependency set.
    /// @param _chainId The chain ID to check.
    /// @return True if the chain ID to check is in the interop dependency set. False otherwise.
    function isInDependencySet(uint256 _chainId) public view returns (bool) {
        return _chainId == block.chainid || _dependencySet.contains(_chainId);
    }

    /// @notice Returns the size of the interop dependency set.
    /// @return The size of the interop dependency set.
    function dependencySetSize() external view returns (uint256) {
        return _dependencySet.length();
    }

    /// @notice Getter for the chain ids list on the dependency set.
    function dependencySet() external view returns (uint256[] memory) {
        return _dependencySet.values();
    }
}
