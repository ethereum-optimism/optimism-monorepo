// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Forge
import { Test } from "forge-std/Test.sol";

// Libraries
import { EOA } from "src/libraries/EOA.sol";

/// @title EOA_Harness
/// @notice A helper contract to test the EOA library.
contract EOA_Harness {
    /// @notice Returns true if the sender is an EOA.
    /// @return isEOA_ True if the sender is an EOA.
    function isSenderEOA() external view returns (bool isEOA_) {
        return EOA.isSenderEOA();
    }
}

/// @title EOA_isEOA_Test
contract EOA_isEOA_Test is Test {
    EOA_Harness harness;

    /// @notice Sets up the test.
    function setUp() public {
        harness = new EOA_Harness();
    }

    /// @notice Tests that a standard EOA is detected as an EOA.
    /// @param _privateKey The private key of the sender.
    function testFuzz_isEOA_isStandardEOA_succeeds(uint256 _privateKey) external {
        // Make sure that the private key is in the range of a valid secp256k1 private key.
        _privateKey = boundPrivateKey(_privateKey);

        // Make sure that the sender is a standard EOA with no code.
        address sender = vm.addr(_privateKey);
        vm.assume(sender.code.length == 0);

        // Should be considered an EOA
        vm.prank(sender, sender);
        assertEq(harness.isSenderEOA(), true);
    }

    /// @notice Tests that a 7702 EOA is detected as an EOA.
    /// @param _privateKey The private key of the sender.
    /// @param _7702Target The target of the 7702 EOA.
    function testFuzz_isEOA_is7702EOA_succeeds(uint256 _privateKey, address _7702Target) external {
        // Make sure that the private key is in the range of a valid secp256k1 private key.
        _privateKey = boundPrivateKey(_privateKey);

        // Make sure that the sender is a 7702 EOA.
        address sender = vm.addr(_privateKey);
        vm.etch(sender, abi.encodePacked(hex"EF0100", _7702Target));

        // Should be considered a 7702 EOA.
        vm.prank(sender, sender);
        assertEq(harness.isSenderEOA(), true);

        // Should still be considered an EOA even if origin is different.
        vm.prank(sender, address(0x0420));
        assertEq(harness.isSenderEOA(), true);
    }

    /// @notice Tests that a contract is not detected as an EOA.
    /// @param _privateKey The private key of the sender.
    /// @param _code The code of the sender.
    function testFuzz_isEOA_isContract_succeeds(uint256 _privateKey, bytes memory _code) external {
        // Make sure that the private key is in the range of a valid secp256k1 private key.
        _privateKey = boundPrivateKey(_privateKey);

        // If code is empty or starts with EF, change it.
        if (_code.length == 0 || _code[0] == 0xEF) {
            _code = bytes.concat(hex"FFFFFF", _code);
        }

        // Make sure that the sender is a contract.
        address sender = vm.addr(_privateKey);
        vm.etch(sender, _code);

        // Should not be considered an EOA.
        vm.prank(sender);
        assertEq(harness.isSenderEOA(), false);
    }
}
