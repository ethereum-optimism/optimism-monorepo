// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { AssetReceiver } from "src/periphery/AssetReceiver.sol";

// Interfaces
import { IDripCheck } from "src/periphery/drippie/IDripCheck.sol";

/// @title Drippie
/// @notice Drippie is a system for managing automated contract interactions. A specific interaction
///         is called a "drip" and can be executed according to some condition (called a dripcheck)
///         and an execution interval. Drips cannot be executed faster than the execution interval.
///         Drips can trigger arbitrary contract calls where the calling contract is this contract
///         address. Drips can also send ETH value, which makes them ideal for keeping addresses
///         sufficiently funded with ETH. Drippie is designed to be connected with smart contract
///         automation services so that drips can be executed automatically. However, Drippie is
///         specifically designed to be separated from these services so that trust assumptions are
///         better compartmentalized.
contract Drippie is AssetReceiver {
    /// @notice Enum representing different status options for a given drip.
    /// @custom:value NONE     Drip does not exist.
    /// @custom:value PAUSED   Drip is paused and cannot be executed until reactivated.
    /// @custom:value ACTIVE   Drip is active and can be executed.
    /// @custom:value ARCHIVED Drip is archived and can no longer be executed or reactivated.
    enum DripStatus {
        NONE,
        PAUSED,
        ACTIVE,
        ARCHIVED
    }

    /// @notice Represents a drip action.
    struct DripAction {
        address payable target;
        bytes data;
        uint256 value;
    }

    /// @notice Represents the configuration for a given drip.
    struct DripConfig {
        bool reentrant;
        uint256 interval;
        IDripCheck dripcheck;
        bytes checkparams;
        DripAction[] actions;
    }

    /// @notice Represents the state of an active drip.
    struct DripState {
        DripStatus status;
        DripConfig config;
        uint256 last;
        uint256 count;
    }

    /// @notice Emitted when a new drip is created.
    /// @param nameref Indexed name parameter (hashed).
    /// @param name    Unindexed name parameter (unhashed).
    /// @param config  Config for the created drip.
    // Emit name twice because indexed version is hashed.
    event DripCreated(string indexed nameref, string name, DripConfig config);

    /// @notice Emitted when a drip status is updated.
    /// @param nameref Indexed name parameter (hashed).
    /// @param name    Unindexed name parameter (unhashed).
    /// @param status  New drip status.
    // Emit name twice because indexed version is hashed.
    event DripStatusUpdated(string indexed nameref, string name, DripStatus status);

    /// @notice Emitted when a drip is executed.
    /// @param nameref   Indexed name parameter (hashed).
    /// @param name      Unindexed name parameter (unhashed).
    /// @param executor  Address that executed the drip.
    /// @param timestamp Time when the drip was executed.
    // Emit name twice because indexed version is hashed.
    event DripExecuted(string indexed nameref, string name, address executor, uint256 timestamp);

    /// @notice Maps from drip names to drip states.
    mapping(string => DripState) public drips;

    /// @notice Array of created drips. Used so offchain services can easily iterate over all drips
    ///         without resorting to event queries. Convenience feature and shouldn't really be
    ///         used onchain because the array will grow indefinitely.
    string[] public created;

    //// @param _owner Initial contract owner.
    constructor(address _owner) AssetReceiver(_owner) { }

    /// @notice Creates a new drip with the given name and configuration. Once created, drips cannot
    ///         be modified in any way (this is a security measure). If you want to update a drip,
    ///         simply pause (and potentially archive) the existing drip and create a new one.
    /// @param _name   Name of the drip.
    /// @param _config Configuration for the drip.
    function create(string calldata _name, DripConfig calldata _config) external onlyOwner {
        // Make sure this drip doesn't already exist. We *must* guarantee that no other function
        // will ever set the status of a drip back to NONE after it's been created. This is why
        // archival is a separate status.
        require(drips[_name].status == DripStatus.NONE, "Drippie: drip with that name already exists");

        // Validate the drip interval, only allowing an interval of zero if the drip has explicitly
        // been marked as reentrant. Prevents client-side bugs making a drip infinitely executable
        // within the same block (of course, restricted by gas limits).
        if (_config.reentrant) {
            require(_config.interval == 0, "Drippie: if allowing reentrant drip, must set interval to zero");
        } else {
            require(_config.interval > 0, "Drippie: interval must be greater than zero if drip is not reentrant");
        }

        // We initialize this way because Solidity won't let us copy arrays into storage yet.
        DripState storage state = drips[_name];
        state.status = DripStatus.PAUSED;
        state.config.reentrant = _config.reentrant;
        state.config.interval = _config.interval;
        state.config.dripcheck = _config.dripcheck;
        state.config.checkparams = _config.checkparams;

        // Solidity doesn't let us copy arrays into storage, so we push each array one by one.
        for (uint256 i = 0; i < _config.actions.length; i++) {
            state.config.actions.push(_config.actions[i]);
        }

        // Add the name of the drip to the array of created drips.
        created.push(_name);

        // Tell the world!
        emit DripCreated(_name, _name, _config);
    }

    /// @notice Sets the status for a given drip. The behavior of this function depends on the
    ///         status that the user is trying to set. A drip can always move between ACTIVE and
    ///         PAUSED, but it can never move back to NONE and once ARCHIVED, it can never move back
    ///         to ACTIVE or PAUSED.
    /// @param _name   Name of the drip to update.
    /// @param _status New drip status.
    function status(string calldata _name, DripStatus _status) external onlyOwner {
        // Make sure we can never set drip status back to NONE. A simple security measure to
        // prevent accidental overwrites if this code is ever updated down the line.
        require(_status != DripStatus.NONE, "Drippie: drip status can never be set back to NONE after creation");

        // Load the drip status once to avoid unnecessary SLOADs.
        DripStatus curr = drips[_name].status;

        // Make sure the drip in question actually exists. Not strictly necessary but there doesn't
        // seem to be any clear reason why you would want to do this, and it may save some gas in
        // the case of a front-end bug.
        require(curr != DripStatus.NONE, "Drippie: drip with that name does not exist and cannot be updated");

        // Once a drip has been archived, it cannot be un-archived. This is, after all, the entire
        // point of archiving a drip.
        require(curr != DripStatus.ARCHIVED, "Drippie: drip with that name has been archived and cannot be updated");

        // Although not strictly necessary, we make sure that the status here is actually changing.
        // This may save the client some gas if there's a front-end bug and the user accidentally
        // tries to "change" the status to the same value as before.
        require(curr != _status, "Drippie: cannot set drip status to the same status as its current status");

        // If the user is trying to archive this drip, make sure the drip has been paused. We do
        // not allow users to archive active drips so that the effects of this action are more
        // abundantly clear.
        if (_status == DripStatus.ARCHIVED) {
            require(curr == DripStatus.PAUSED, "Drippie: drip must first be paused before being archived");
        }

        // If we made it here then we can safely update the status.
        drips[_name].status = _status;
        emit DripStatusUpdated(_name, _name, _status);
    }

    /// @notice Checks if a given drip is executable.
    /// @param _name Drip to check.
    /// @return True if the drip is executable, reverts otherwise.
    function executable(string calldata _name) public view returns (bool) {
        DripState storage state = drips[_name];

        // Only allow active drips to be executed, an obvious security measure.
        require(state.status == DripStatus.ACTIVE, "Drippie: selected drip does not exist or is not currently active");

        // Don't drip if the drip interval has not yet elapsed since the last time we dripped. This
        // is a safety measure that prevents a malicious recipient from, e.g., spending all of
        // their funds and repeatedly requesting new drips. Limits the potential impact of a
        // compromised recipient to just a single drip interval, after which the drip can be paused
        // by the owner address.
        require(
            state.last + state.config.interval <= block.timestamp,
            "Drippie: drip interval has not elapsed since last drip"
        );

        // Make sure we're allowed to execute this drip.
        require(
            state.config.dripcheck.check(state.config.checkparams),
            "Drippie: dripcheck failed so drip is not yet ready to be triggered"
        );

        // Alright, we're good to execute.
        return true;
    }

    /// @notice Triggers a drip. This function is deliberately left as a public function because the
    ///         assumption being made here is that setting the drip to ACTIVE is an affirmative
    ///         signal that the drip should be executable according to the drip parameters, drip
    ///         check, and drip interval. Note that drip parameters are read entirely from the state
    ///         and are not supplied as user input, so there should not be any way for a
    ///         non-authorized user to influence the behavior of the drip. Note that the drip check
    ///         is executed only **once** at the beginning of the call to the drip function and will
    ///         not be executed again between the drip actions within this call.
    /// @param _name Name of the drip to trigger.
    function drip(string calldata _name) external {
        DripState storage state = drips[_name];

        // Make sure the drip can be executed. Since executable reverts if the drip is not ready to
        // be executed, we don't need to do an assertion that the returned value is true.
        executable(_name);

        // Update the last execution time for this drip before the call. Note that it's entirely
        // possible for a drip to be executed multiple times per block or even multiple times
        // within the same transaction (via re-entrancy) if the drip interval is set to zero. Users
        // should set a drip interval of 1 if they'd like the drip to be executed only once per
        // block (since this will then prevent re-entrancy).
        state.last = block.timestamp;

        // Update the number of times this drip has been executed. Although this increases the cost
        // of using Drippie, it slightly simplifies the client-side by not having to worry about
        // counting drips via events. Useful for monitoring the rate of drip execution.
        state.count++;

        // Execute each action in the drip. We allow drips to have multiple actions because there
        // are scenarios in which a contract must do multiple things atomically. For example, the
        // contract may need to withdraw ETH from one account and then deposit that ETH into
        // another account within the same transaction.
        uint256 len = state.config.actions.length;
        for (uint256 i = 0; i < len; i++) {
            // Must be marked as "storage" because copying structs into memory is not yet supported
            // by Solidity. Won't significantly reduce gas costs but at least makes it easier to
            // read what the rest of this section is doing.
            DripAction storage action = state.config.actions[i];

            // Actually execute the action. We could use ExcessivelySafeCall here but not strictly
            // necessary (worst case, a drip gets bricked IFF the target is malicious, doubt this
            // will ever happen in practice). Could save a marginal amount of gas to ignore the
            // returndata.
            // slither-disable-next-line calls-loop
            (bool success,) = action.target.call{ value: action.value }(action.data);

            // Generally should not happen, but could if there's a misconfiguration (e.g., passing
            // the wrong data to the target contract), the recipient is not payable, or
            // insufficient gas was supplied to this transaction. We revert so the drip can be
            // fixed and triggered again later. Means we cannot emit an event to alert of the
            // failure, but can reasonably be detected by off-chain services even without an event.
            // Note that this forces the drip executor to supply sufficient gas to the call
            // (assuming there is some sufficient gas limit that exists, otherwise the drip will
            // not execute).
            require(success, "Drippie: drip was unsuccessful, please check your configuration for mistakes");
        }

        emit DripExecuted(_name, _name, msg.sender, block.timestamp);
    }

    /// @notice Returns the status of a given drip.
    /// @param _name Drip to check.
    /// @return DripStatus of the given drip.
    function getDripStatus(string calldata _name) public view returns (DripStatus) {
        return drips[_name].status;
    }

    /// @notice Returns the interval of a given drip.
    /// @param _name Drip to check.
    /// @return Interval of the given drip.
    function getDripInterval(string calldata _name) public view returns (uint256) {
        return drips[_name].config.interval;
    }

    /// @notice Returns the number of created drips.
    /// @return Number of created drips.
    function getDripCount() public view returns (uint256) {
        return created.length;
    }
}
