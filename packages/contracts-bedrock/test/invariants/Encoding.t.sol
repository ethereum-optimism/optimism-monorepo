// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { StdInvariant } from "forge-std/StdInvariant.sol";
import { Encoding } from "src/libraries/Encoding.sol";
import { InvariantTest } from "test/invariants/InvariantTest.sol";

contract Encoding_Converter {
    bool public failedRoundtripAToB;
    bool public failedRoundtripBToA;

    /// @notice Takes a pair of integers to be encoded into a versioned nonce with the
    ///         Encoding library and then decoded and updates the test contract's state
    ///         indicating if the round trip encoding failed.
    function convertRoundTripAToB(uint240 _nonce, uint16 _version) external {
        // Encode the nonce and version
        uint256 encodedVersionedNonce = Encoding.encodeVersionedNonce(_nonce, _version);

        // Decode the nonce and version
        uint240 decodedNonce;
        uint16 decodedVersion;

        (decodedNonce, decodedVersion) = Encoding.decodeVersionedNonce(encodedVersionedNonce);

        // If our round trip encoding did not return the original result, set our state.
        if ((decodedNonce != _nonce) || (decodedVersion != _version)) {
            failedRoundtripAToB = true;
        }
    }

    /// @notice Takes an integer representing a packed version and nonce and attempts
    ///         to decode them using the Encoding library before re-encoding and updates
    ///         the test contract's state indicating if the round trip encoding failed.
    function convertRoundTripBToA(uint256 _versionedNonce) external {
        // Decode the nonce and version
        uint240 decodedNonce;
        uint16 decodedVersion;

        (decodedNonce, decodedVersion) = Encoding.decodeVersionedNonce(_versionedNonce);

        // Encode the nonce and version
        uint256 encodedVersionedNonce = Encoding.encodeVersionedNonce(decodedNonce, decodedVersion);

        // If our round trip encoding did not return the original result, set our state.
        if (encodedVersionedNonce != _versionedNonce) {
            failedRoundtripBToA = true;
        }
    }
}

contract Encoding_Invariant is StdInvariant, InvariantTest {
    Encoding_Converter internal actor;

    function setUp() public override {
        super.setUp();
        // Create a converter actor.
        actor = new Encoding_Converter();

        targetContract(address(actor));

        bytes4[] memory selectors = new bytes4[](2);
        selectors[0] = actor.convertRoundTripAToB.selector;
        selectors[1] = actor.convertRoundTripBToA.selector;
        FuzzSelector memory selector = FuzzSelector({ addr: address(actor), selectors: selectors });
        targetSelector(selector);
    }

    /// @custom:invariant `convertRoundTripAToB` never fails.
    ///
    ///                   Asserts that a raw versioned nonce can be encoded / decoded
    ///                   to reach the same raw value.
    function invariant_round_trip_encoding_AToB() external view {
        // ASSERTION: The round trip encoding done in testRoundTripAToB(...)
        assertEq(actor.failedRoundtripAToB(), false);
    }

    /// @custom:invariant `convertRoundTripBToA` never fails.
    ///
    ///                   Asserts that an encoded versioned nonce can always be decoded /
    ///                   re-encoded to reach the same encoded value.
    function invariant_round_trip_encoding_BToA() external view {
        // ASSERTION: The round trip encoding done in testRoundTripBToA should never
        // fail.
        assertEq(actor.failedRoundtripBToA(), false);
    }
}
