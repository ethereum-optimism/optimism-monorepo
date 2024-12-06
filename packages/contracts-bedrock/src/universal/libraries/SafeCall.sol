// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// @title SafeCall
/// @notice Perform low level safe calls
library SafeCall {
    /// @notice Performs a low level call without copying any returndata.
    /// @dev Passes no calldata to the call context.
    /// @param _target   Address to call
    /// @param _gas      Amount of gas to pass to the call
    /// @param _value    Amount of value to pass to the call
    function send(address _target, uint256 _gas, uint256 _value) internal returns (bool success_) {
        assembly {
            success_ :=
                call(
                    _gas, // gas
                    _target, // recipient
                    _value, // ether value
                    0, // inloc
                    0, // inlen
                    0, // outloc
                    0 // outlen
                )
        }
    }

    /// @notice Perform a low level call with all gas without copying any returndata
    /// @param _target   Address to call
    /// @param _value    Amount of value to pass to the call
    function send(address _target, uint256 _value) internal returns (bool success_) {
        success_ = send(_target, gasleft(), _value);
    }

    /// @notice Perform a low level call without copying any returndata
    /// @param _target   Address to call
    /// @param _gas      Amount of gas to pass to the call
    /// @param _value    Amount of value to pass to the call
    /// @param _calldata Calldata to pass to the call
    function call(
        address _target,
        uint256 _gas,
        uint256 _value,
        bytes memory _calldata
    )
        internal
        returns (bool success_)
    {
        assembly {
            success_ :=
                call(
                    _gas, // gas
                    _target, // recipient
                    _value, // ether value
                    add(_calldata, 32), // inloc
                    mload(_calldata), // inlen
                    0, // outloc
                    0 // outlen
                )
        }
    }

    /// @notice Perform a low level call without copying any returndata
    /// @param _target   Address to call
    /// @param _value    Amount of value to pass to the call
    /// @param _calldata Calldata to pass to the call
    function call(address _target, uint256 _value, bytes memory _calldata) internal returns (bool success_) {
        success_ = call({ _target: _target, _gas: gasleft(), _value: _value, _calldata: _calldata });
    }

    /// @notice Perform a low level call without copying any returndata
    /// @param _target   Address to call
    /// @param _calldata Calldata to pass to the call
    function call(address _target, bytes memory _calldata) internal returns (bool success_) {
        success_ = call({ _target: _target, _gas: gasleft(), _value: 0, _calldata: _calldata });
    }

    /// @notice Helper function to determine if there is sufficient gas remaining within the context
    ///         to guarantee that the minimum gas requirement for a call will be met as well as
    ///         optionally reserving a specified amount of gas for after the call has concluded.
    /// @param _minGas      The minimum amount of gas that may be passed to the target context.
    /// @param _reservedGas Optional amount of gas to reserve for the caller after the execution
    ///                     of the target context.
    /// @return `true` if there is enough gas remaining to safely supply `_minGas` to the target
    ///         context as well as reserve `_reservedGas` for the caller after the execution of
    ///         the target context.
    /// @dev !!!!! FOOTGUN ALERT !!!!!
    ///      1.) The 40_000 base buffer is to account for the worst case of the dynamic cost of the
    ///          `CALL` opcode's `address_access_cost`, `positive_value_cost`, and
    ///          `value_to_empty_account_cost` factors with an added buffer of 5,700 gas. It is
    ///          still possible to self-rekt by initiating a withdrawal with a minimum gas limit
    ///          that does not account for the `memory_expansion_cost` & `code_execution_cost`
    ///          factors of the dynamic cost of the `CALL` opcode.
    ///      2.) This function should *directly* precede the external call if possible. There is an
    ///          added buffer to account for gas consumed between this check and the call, but it
    ///          is only 5,700 gas.
    ///      3.) Because EIP-150 ensures that a maximum of 63/64ths of the remaining gas in the call
    ///          frame may be passed to a subcontext, we need to ensure that the gas will not be
    ///          truncated.
    ///      4.) Use wisely. This function is not a silver bullet.
    function hasMinGas(uint256 _minGas, uint256 _reservedGas) internal view returns (bool) {
        bool _hasMinGas;
        assembly {
            // Equation: gas × 63 ≥ minGas × 64 + 63(40_000 + reservedGas)
            _hasMinGas := iszero(lt(mul(gas(), 63), add(mul(_minGas, 64), mul(add(40000, _reservedGas), 63))))
        }
        return _hasMinGas;
    }

    /// @notice Perform a low level call without copying any returndata. This function
    ///         will revert if the call cannot be performed with the specified minimum
    ///         gas.
    /// @param _target   Address to call
    /// @param _minGas   The minimum amount of gas that may be passed to the call
    /// @param _value    Amount of value to pass to the call
    /// @param _calldata Calldata to pass to the call
    function callWithMinGas(
        address _target,
        uint256 _minGas,
        uint256 _value,
        bytes memory _calldata
    )
        internal
        returns (bool)
    {
        bool _success;
        bool _hasMinGas = hasMinGas(_minGas, 0);
        assembly {
            // Assertion: gasleft() >= (_minGas * 64) / 63 + 40_000
            if iszero(_hasMinGas) {
                // Store the "Error(string)" selector in scratch space.
                mstore(0, 0x08c379a0)
                // Store the pointer to the string length in scratch space.
                mstore(32, 32)
                // Store the string.
                //
                // SAFETY:
                // - We pad the beginning of the string with two zero bytes as well as the
                // length (24) to ensure that we override the free memory pointer at offset
                // 0x40. This is necessary because the free memory pointer is likely to
                // be greater than 1 byte when this function is called, but it is incredibly
                // unlikely that it will be greater than 3 bytes. As for the data within
                // 0x60, it is ensured that it is 0 due to 0x60 being the zero offset.
                // - It's fine to clobber the free memory pointer, we're reverting.
                mstore(88, 0x0000185361666543616c6c3a204e6f7420656e6f75676820676173)

                // Revert with 'Error("SafeCall: Not enough gas")'
                revert(28, 100)
            }

            // The call will be supplied at least ((_minGas * 64) / 63) gas due to the
            // above assertion. This ensures that, in all circumstances (except for when the
            // `_minGas` does not account for the `memory_expansion_cost` and `code_execution_cost`
            // factors of the dynamic cost of the `CALL` opcode), the call will receive at least
            // the minimum amount of gas specified.
            _success :=
                call(
                    gas(), // gas
                    _target, // recipient
                    _value, // ether value
                    add(_calldata, 32), // inloc
                    mload(_calldata), // inlen
                    0x00, // outloc
                    0x00 // outlen
                )
        }
        return _success;
    }
}
