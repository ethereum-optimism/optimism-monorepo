// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/// @title TransientContext
/// @notice Library for transient storage.
library TransientContext {
    /// @notice Slot for call depth.
    ///         Equal to bytes32(uint256(keccak256("transient.calldepth")) - 1).
    bytes32 internal constant CALL_DEPTH_SLOT = 0x7a74fd168763fd280eaec3bcd2fd62d0e795027adc8183a693c497a7c2b10b5c;

    /// @notice Gets the call depth.
    /// @return callDepth_ Current call depth.
    function callDepth() internal view returns (uint256 callDepth_) {
        assembly ("memory-safe") {
            callDepth_ := tload(CALL_DEPTH_SLOT)
        }
    }

    /// @notice Gets value in transient storage for a slot at the current call depth.
    /// @param _slot Slot to get.
    /// @return value_ Transient value.
    function get(bytes32 _slot) internal view returns (uint256 value_) {
        assembly ("memory-safe") {
            mstore(0, tload(CALL_DEPTH_SLOT))
            mstore(32, _slot)
            value_ := tload(keccak256(0, 64))
        }
    }

    /// @notice Sets a value in transient storage for a slot at the current call depth.
    /// @param _slot    Slot to set.
    /// @param _value   Value to set.
    function set(bytes32 _slot, uint256 _value) internal {
        assembly ("memory-safe") {
            mstore(0, tload(CALL_DEPTH_SLOT))
            mstore(32, _slot)
            tstore(keccak256(0, 64), _value)
        }
    }

    /// @notice Increments call depth.
    ///         This function can overflow. However, this is ok because there's still
    ///         only one value stored per slot.
    function increment() internal {
        assembly ("memory-safe") {
            tstore(CALL_DEPTH_SLOT, add(tload(CALL_DEPTH_SLOT), 1))
        }
    }

    /// @notice Decrements call depth.
    ///         This function can underflow. However, this is ok because there's still
    ///         only one value stored per slot.
    function decrement() internal {
        assembly ("memory-safe") {
            tstore(CALL_DEPTH_SLOT, sub(tload(CALL_DEPTH_SLOT), 1))
        }
    }
}

/// @title TransientReentrancyAware
/// @notice Reentrancy-aware modifier for transient storage, which increments and
///         decrements the call depth when entering and exiting a function.
contract TransientReentrancyAware {
    /// @notice Thrown when a non-written transient storage slot is attempted to be read from.
    error NotEntered();

    /// @notice Thrown when a reentrant call is detected.
    error ReentrantCall();

    /// @notice Storage slot for `entered` value.
    ///         Equal to bytes32(uint256(keccak256("transientreentrancyaware.entered")) - 1)
    bytes32 internal constant ENTERED_SLOT = 0xf13569814868ede994184d5a425471fb19e869768a33421cb701a2ba3d420c0a;

    /// @notice Modifier to make a function reentrancy-aware.
    modifier reentrantAware() {
        TransientContext.increment();
        _;
        TransientContext.decrement();
    }

    /// @notice Enforces that a function cannot be re-entered.
    modifier nonReentrant() {
        if (_entered()) revert ReentrantCall();
        assembly {
            tstore(ENTERED_SLOT, 1)
        }
        _;
        assembly {
            tstore(ENTERED_SLOT, 0)
        }
    }

    /// @notice Enforces that cross domain message sender and source are set. Reverts if not.
    ///         Used to differentiate between 0 and nil in transient storage.
    modifier notEntered() {
        if (TransientContext.callDepth() == 0) revert NotEntered();
        _;
    }

    /// @notice Enforces that cross domain message sender and source are set. Reverts if not.
    ///         Used to differentiate between 0 and nil in transient storage.
    modifier onlyEntered() {
        if (!_entered()) revert NotEntered();
        _;
    }

    /// @notice Retrieves whether the contract is currently entered or not.
    /// @return entered_ True if the contract is entered, and false otherwise.
    function _entered() internal view returns (bool entered_) {
        assembly {
            let value := tload(ENTERED_SLOT)
            entered_ := gt(value, 0)
        }
    }
}
