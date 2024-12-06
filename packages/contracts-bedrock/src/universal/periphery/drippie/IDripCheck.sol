// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

interface IDripCheck {
    // DripCheck contracts that want to take parameters as inputs MUST expose a struct called
    // Params and an event _EventForExposingParamsStructInABI(Params params). This makes it
    // possible to easily encode parameters on the client side. Solidity does not support generics
    // so it's not possible to do this with explicit typing.

    /// @notice Returns the name of the drip check.
    /// @return name_ The name of the drip check.
    function name() external view returns (string memory name_);

    /// @notice Checks whether a drip should be executable.
    /// @param _params Encoded parameters for the drip check.
    /// @return execute_ Whether the drip should be executed.
    function check(bytes memory _params) external view returns (bool execute_);
}
