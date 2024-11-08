// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing utilities
import { CommonTest } from "test/setup/CommonTest.sol";
import { ERC20 } from "@openzeppelin/contracts/token/ERC20/ERC20.sol";

contract DeployVariations_Test is CommonTest {
    function setUp() public override {
        // Prevent calling the base CommonTest.setUp() function, as we will run it within the test functions
        // after setting the feature flags
    }

    // Enable features which should be possible to enable or disable regardless of other options.
    function enableAddOns(bool _enableCGT, bool _enableAltDa) public {
        if (_enableCGT) {
            ERC20 token = new ERC20("Silly", "SIL");
            super.enableCustomGasToken(address(token));
        }
        if (_enableAltDa) {
            super.enableAltDA();
        }
    }

    /// forge-config: ciheavy.fuzz.runs = 512
    /// @dev It should be possible to enable Fault Proofs with any mix of CGT and Alt-DA.
    function testFuzz_enableFaultProofs_succeeds(bool _enableCGT, bool _enableAltDa) public virtual {
        enableAddOns(_enableCGT, _enableAltDa);

        super.setUp();
    }

    /// forge-config: ciheavy.fuzz.runs = 512
    /// @dev It should be possible to enable Fault Proofs and Interop with any mix of CGT and Alt-DA.
    function test_enableInteropAndFaultProofs_succeeds(bool _enableCGT, bool _enableAltDa) public virtual {
        enableAddOns(_enableCGT, _enableAltDa);
        super.enableInterop();

        super.setUp();
    }
}
