// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { StdInvariant } from "forge-std/StdInvariant.sol";
import { Encoding } from "src/libraries/Encoding.sol";
import { Hashing } from "src/libraries/Hashing.sol";
import { InvariantTest } from "test/invariants/InvariantTest.sol";

contract Hash_CrossDomainHasher {
    bool public failedCrossDomainHashHighVersion;
    bool public failedCrossDomainHashV0;
    bool public failedCrossDomainHashV1;

    /// @notice Takes the necessary parameters to perform a cross domain hash with a randomly
    ///         generated version. Only schema versions 0 and 1 are supported and all others
    ///         should revert.
    function hashCrossDomainMessageHighVersion(
        uint16 _version,
        uint240 _nonce,
        address _sender,
        address _target,
        uint256 _value,
        uint256 _gasLimit,
        bytes memory _data
    )
        external
    {
        // generate the versioned nonce
        uint256 encodedNonce = Encoding.encodeVersionedNonce(_nonce, _version);

        // hash the cross domain message. we don't need to store the result since the function
        // validates and should revert if an invalid version (>1) is encoded
        Hashing.hashCrossDomainMessage(encodedNonce, _sender, _target, _value, _gasLimit, _data);

        // check that execution never makes it this far for an invalid version
        if (_version > 1) {
            failedCrossDomainHashHighVersion = true;
        }
    }

    /// @notice Takes the necessary parameters to perform a cross domain hash using the v0 schema
    ///         and compares the output of a call to the unversioned function to the v0 function
    ///         directly.
    function hashCrossDomainMessageV0(
        uint240 _nonce,
        address _sender,
        address _target,
        uint256 _value,
        uint256 _gasLimit,
        bytes memory _data
    )
        external
    {
        // generate the versioned nonce with the version set to 0
        uint256 encodedNonce = Encoding.encodeVersionedNonce(_nonce, 0);

        // hash the cross domain message using the unversioned and versioned functions for
        // comparison
        bytes32 sampleHash1 = Hashing.hashCrossDomainMessage(encodedNonce, _sender, _target, _value, _gasLimit, _data);
        bytes32 sampleHash2 = Hashing.hashCrossDomainMessageV0(_target, _sender, _data, encodedNonce);

        // check that the output of both functions matches
        if (sampleHash1 != sampleHash2) {
            failedCrossDomainHashV0 = true;
        }
    }

    /// @notice Takes the necessary parameters to perform a cross domain hash using the v1 schema
    ///         and compares the output of a call to the unversioned function to the v1 function
    ///         directly.
    function hashCrossDomainMessageV1(
        uint240 _nonce,
        address _sender,
        address _target,
        uint256 _value,
        uint256 _gasLimit,
        bytes memory _data
    )
        external
    {
        // generate the versioned nonce with the version set to 1
        uint256 encodedNonce = Encoding.encodeVersionedNonce(_nonce, 1);

        // hash the cross domain message using the unversioned and versioned functions for
        // comparison
        bytes32 sampleHash1 = Hashing.hashCrossDomainMessage(encodedNonce, _sender, _target, _value, _gasLimit, _data);
        bytes32 sampleHash2 = Hashing.hashCrossDomainMessageV1(encodedNonce, _sender, _target, _value, _gasLimit, _data);

        // check that the output of both functions matches
        if (sampleHash1 != sampleHash2) {
            failedCrossDomainHashV1 = true;
        }
    }
}

contract Hashing_Invariant is StdInvariant, InvariantTest {
    Hash_CrossDomainHasher internal actor;

    function setUp() public override {
        super.setUp();
        // Create a hasher actor.
        actor = new Hash_CrossDomainHasher();

        targetContract(address(actor));

        bytes4[] memory selectors = new bytes4[](3);
        selectors[0] = actor.hashCrossDomainMessageHighVersion.selector;
        selectors[1] = actor.hashCrossDomainMessageV0.selector;
        selectors[2] = actor.hashCrossDomainMessageV1.selector;
        FuzzSelector memory selector = FuzzSelector({ addr: address(actor), selectors: selectors });
        targetSelector(selector);
    }

    /// @custom:invariant `hashCrossDomainMessage` reverts if `version` is > `1`.
    ///
    ///                   The `hashCrossDomainMessage` function should always revert if
    ///                   the `version` passed is > `1`.
    function invariant_hash_xdomain_msg_high_version() external view {
        // ASSERTION: The round trip aliasing done in testRoundTrip(...) should never fail.
        assertFalse(actor.failedCrossDomainHashHighVersion());
    }

    /// @custom:invariant `version` = `0`: `hashCrossDomainMessage` and `hashCrossDomainMessageV0`
    ///                   are equivalent.
    ///
    ///                   If the version passed is 0, `hashCrossDomainMessage` and
    ///                   `hashCrossDomainMessageV0` should be equivalent.
    function invariant_hash_xdomain_msg_0() external view {
        // ASSERTION: A call to hashCrossDomainMessage and hashCrossDomainMessageV0
        // should always match when the version passed is 0
        assertFalse(actor.failedCrossDomainHashV0());
    }

    /// @custom:invariant `version` = `1`: `hashCrossDomainMessage` and `hashCrossDomainMessageV1`
    ///                   are equivalent.
    ///
    ///                   If the version passed is 1, `hashCrossDomainMessage` and
    ///                   `hashCrossDomainMessageV1` should be equivalent.
    function invariant_hash_xdomain_msg_1() external view {
        // ASSERTION: A call to hashCrossDomainMessage and hashCrossDomainMessageV1
        // should always match when the version passed is 1
        assertFalse(actor.failedCrossDomainHashV1());
    }
}
