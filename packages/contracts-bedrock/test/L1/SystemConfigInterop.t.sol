// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { CommonTest } from "test/setup/CommonTest.sol";

// Interfaces
import { ISystemConfigInterop } from "interfaces/L1/ISystemConfigInterop.sol";

contract SystemConfigInterop_Test is CommonTest {
    /// @notice Marked virtual to be overridden in
    ///         test/kontrol/deployment/DeploymentSummary.t.sol
    function setUp() public virtual override {
        super.enableInterop();
        super.setUp();
    }

    /// @notice Tests that the version function returns a valid string. We avoid testing the
    ///         specific value of the string as it changes frequently.
    function test_version_succeeds() external view {
        assert(bytes(_systemConfigInterop().version()).length > 0);
    }

    /// @dev Returns the SystemConfigInterop instance.
    function _systemConfigInterop() internal view returns (ISystemConfigInterop) {
        return ISystemConfigInterop(address(systemConfig));
    }
}
