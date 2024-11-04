// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Test } from "forge-std/Test.sol";
import { StdUtils } from "forge-std/StdUtils.sol";
import { StdCheats } from "forge-std/StdCheats.sol";
import { GnosisSafe as Safe } from "safe-contracts/GnosisSafe.sol";
import { OwnerManager } from "safe-contracts/base/OwnerManager.sol";
import { Enum } from "safe-contracts/common/Enum.sol";
import "test/safe-tools/SafeTestTools.sol";
import { EnumerableSet } from "@openzeppelin/contracts/utils/structs/EnumerableSet.sol";

import { LivenessGuard } from "src/safe/LivenessGuard.sol";

/// @dev A wrapper contract exposing the length of the ownersBefore set in the LivenessGuard.
contract WrappedGuard is LivenessGuard {
    using EnumerableSet for EnumerableSet.AddressSet;

    constructor(Safe safe) LivenessGuard(safe) { }

    function ownersBeforeLength() public view returns (uint256) {
        return ownersBefore.length();
    }
}

contract LivenessGuard_TestInit is Test, SafeTestTools {
    using SafeTestLib for SafeInstance;

    event OwnerRecorded(address owner);

    WrappedGuard livenessGuard;
    SafeInstance safeInstance;

    // This needs to be non-zero so that the `lastLive` mapping can record non-zero timestamps
    uint256 initTime = 10;
    // These values reflect the planned state of the mainnet Security Council Safe.
    uint256 threshold = 10;
    uint256 ownerCount = 13;

    /// @dev Sets up the test environment
    function setUp() public {
        vm.warp(initTime);
        (, uint256[] memory privKeys) = SafeTestLib.makeAddrsAndKeys("test-owners", ownerCount);
        safeInstance = _setupSafe(privKeys, threshold);
        livenessGuard = new WrappedGuard(safeInstance.safe);
        safeInstance.setGuard(address(livenessGuard));
    }
}

contract LivenessGuard_Constructor_Test is LivenessGuard_TestInit {
    /// @dev Tests that the constructor correctly sets the current time as the lastLive time for each owner
    function test_constructor_works() external {
        address[] memory owners = safeInstance.owners;
        livenessGuard = new WrappedGuard(safeInstance.safe);
        for (uint256 i; i < owners.length; i++) {
            assertEq(livenessGuard.lastLive(owners[i]), initTime);
        }
    }
}

contract LivenessGuard_Getters_Test is LivenessGuard_TestInit {
    /// @dev Tests that the getters return the correct values
    function test_getters_works() external view {
        assertEq(address(livenessGuard.safe()), address(safeInstance.safe));
        assertEq(livenessGuard.lastLive(address(0)), 0);
    }
}

contract LivenessGuard_CheckTx_TestFails is LivenessGuard_TestInit {
    /// @dev Tests that the checkTransaction function reverts if the caller is not the Safe
    function test_checkTransaction_callerIsNotSafe_reverts() external {
        vm.expectRevert("LivenessGuard: only Safe can call this function");
        livenessGuard.checkTransaction({
            _to: address(0),
            _value: 0,
            _data: hex"00",
            _operation: Enum.Operation.Call,
            _safeTxGas: 0,
            _baseGas: 0,
            _gasPrice: 0,
            _gasToken: address(0),
            _refundReceiver: payable(address(0)),
            _signatures: hex"00",
            _msgSender: address(0)
        });
    }
}

contract LivenessGuard_CheckTx_Test is LivenessGuard_TestInit {
    using SafeTestLib for SafeInstance;

    /// @dev Tests that the checkTransaction function succeeds
    function test_checkTransaction_succeeds() external {
        // Create an array of the addresses who will sign the transaction. SafeTestTools
        // will generate these signatures up to the threshold by iterating over the owners array.
        address[] memory signers = new address[](safeInstance.threshold);
        // copy the first threshold owners into the signers array
        for (uint256 i; i < safeInstance.threshold; i++) {
            signers[i] = safeInstance.owners[i];
        }

        // Record the timestamps before the transaction
        uint256[] memory beforeTimestamps = new uint256[](safeInstance.owners.length);

        // Jump ahead
        uint256 newTimestamp = block.timestamp + 100;
        vm.warp(newTimestamp);

        for (uint256 i; i < signers.length; i++) {
            vm.expectEmit(address(livenessGuard));
            emit OwnerRecorded(signers[i]);
        }
        vm.expectCall(address(safeInstance.safe), abi.encodeCall(safeInstance.safe.nonce, ()));
        vm.expectCall(address(safeInstance.safe), abi.encodeCall(OwnerManager.getThreshold, ()));
        safeInstance.execTransaction({ to: address(1111), value: 0, data: hex"abba" });
        for (uint256 i; i < safeInstance.threshold; i++) {
            uint256 lastLive = livenessGuard.lastLive(safeInstance.owners[i]);
            assertGe(lastLive, beforeTimestamps[i]);
            assertEq(lastLive, newTimestamp);
        }
    }
}

contract LivenessGuard_CheckAfterExecution_TestFails is LivenessGuard_TestInit {
    /// @dev Tests that the checkAfterExecution function reverts if the caller is not the Safe
    function test_checkAfterExecution_callerIsNotSafe_reverts() external {
        vm.expectRevert("LivenessGuard: only Safe can call this function");
        livenessGuard.checkAfterExecution(bytes32(0), false);
    }
}

contract LivenessGuard_ShowLiveness_TestFail is LivenessGuard_TestInit {
    /// @dev Tests that the showLiveness function reverts if the caller is not an owner
    function test_showLiveness_callIsNotSafeOwner_reverts() external {
        vm.expectRevert("LivenessGuard: only Safe owners may demonstrate liveness");
        livenessGuard.showLiveness();
    }
}

contract LivenessGuard_ShowLiveness_Test is LivenessGuard_TestInit {
    /// @dev Tests that the showLiveness function succeeds
    function test_showLiveness_succeeds() external {
        // Cache the caller
        address caller = safeInstance.owners[0];

        vm.expectEmit(address(livenessGuard));
        emit OwnerRecorded(caller);

        vm.prank(caller);
        livenessGuard.showLiveness();

        assertEq(livenessGuard.lastLive(caller), block.timestamp);
    }
}

contract LivenessGuard_OwnerManagement_Test is LivenessGuard_TestInit {
    using SafeTestLib for SafeInstance;

    /// @dev Tests that the guard correctly deletes the owner from the lastLive mapping when it is removed
    function test_removeOwner_succeeds() external {
        address ownerToRemove = safeInstance.owners[0];
        assertGe(livenessGuard.lastLive(ownerToRemove), 0);
        assertTrue(safeInstance.safe.isOwner(ownerToRemove));

        assertEq(livenessGuard.ownersBeforeLength(), 0);
        safeInstance.removeOwner({ prevOwner: address(0), owner: ownerToRemove, threshold: 1 });
        assertEq(livenessGuard.ownersBeforeLength(), 0);

        assertFalse(safeInstance.safe.isOwner(ownerToRemove));
        assertEq(livenessGuard.lastLive(ownerToRemove), 0);
    }

    /// @dev Tests that the guard correctly adds an owner to the lastLive mapping when it is added
    function test_addOwner_succeeds() external {
        address ownerToAdd = makeAddr("new owner");
        assertEq(livenessGuard.lastLive(ownerToAdd), 0);
        assertFalse(safeInstance.safe.isOwner(ownerToAdd));

        assertEq(livenessGuard.ownersBeforeLength(), 0);
        safeInstance.addOwnerWithThreshold({ owner: ownerToAdd, threshold: 1 });
        assertEq(livenessGuard.ownersBeforeLength(), 0);

        assertTrue(safeInstance.safe.isOwner(ownerToAdd));
        assertEq(livenessGuard.lastLive(ownerToAdd), block.timestamp);
    }

    /// @dev Tests that the guard correctly adds an owner to the lastLive mapping when it is added
    function test_swapOwner_succeeds() external {
        address ownerToRemove = safeInstance.owners[0];
        assertGe(livenessGuard.lastLive(ownerToRemove), 0);
        assertTrue(safeInstance.safe.isOwner(ownerToRemove));

        address ownerToAdd = makeAddr("new owner");
        assertEq(livenessGuard.lastLive(ownerToAdd), 0);
        assertFalse(safeInstance.safe.isOwner(ownerToAdd));

        assertEq(livenessGuard.ownersBeforeLength(), 0);
        safeInstance.swapOwner({ prevOwner: address(0), oldOwner: ownerToRemove, newOwner: ownerToAdd });
        assertEq(livenessGuard.ownersBeforeLength(), 0);

        assertFalse(safeInstance.safe.isOwner(ownerToRemove));
        assertEq(livenessGuard.lastLive(ownerToRemove), 0);

        assertTrue(safeInstance.safe.isOwner(ownerToAdd));
        assertEq(livenessGuard.lastLive(ownerToAdd), block.timestamp);
    }
}

contract LivenessGuard_FuzzOwnerManagement_Test is StdCheats, StdUtils, LivenessGuard_TestInit {
    using SafeTestLib for SafeInstance;

    /// @dev Enumerates the possible owner management operations
    enum OwnerOp {
        Add,
        Remove,
        Swap
    }

    /// @dev Describes a change to be made to the safe
    struct OwnerChange {
        uint8 timeDelta; // used to warp the vm
        uint8 operation; // used to choose an OwnerOp
        uint256 ownerIndex; // used to choose the owner to remove or swap out
        uint256 newThreshold;
    }

    /// @dev Maps addresses to private keys
    mapping(address => uint256) privateKeys;

    /// @dev Tests that the guard correctly manages the lastLive mapping when owners are added, removed, or swapped
    /// forge-config: ciheavy.fuzz.runs = 8192
    function testFuzz_ownerManagement_works(
        uint256 initialOwners,
        uint256 threshold,
        OwnerChange[] memory changes
    )
        external
    {
        // Cut down the changes array to a maximum of 20.
        // We don't use vm.assume to avoid throwing out too many inputs.
        OwnerChange[] memory boundedChanges = new OwnerChange[](bound(changes.length, 0, 20));
        for (uint256 i; i < boundedChanges.length; i++) {
            boundedChanges[i] = changes[i];
        }

        // Update the original array.
        changes = boundedChanges;

        // Initialize the safe with more owners than changes, to ensure we don't try to remove them all
        initialOwners = bound(initialOwners, changes.length, 2 * changes.length);

        // We need at least one owner
        initialOwners = initialOwners < 1 ? 1 : initialOwners;

        // Limit the threshold to the number of owners
        threshold = bound(threshold, 1, initialOwners);

        // Generate the initial owners and keys and setup the safe
        (address[] memory ownerAddrs, uint256[] memory ownerkeys) =
            SafeTestLib.makeAddrsAndKeys("safeTest", initialOwners);
        // record the private keys for later use
        for (uint256 i; i < ownerAddrs.length; i++) {
            privateKeys[ownerAddrs[i]] = ownerkeys[i];
        }

        // Override the saltNonce to ensure prevent a create2 collision.
        saltNonce = uint256(keccak256(bytes("LIVENESS GUARD OWNER MANAGEMENT TEST")));
        // Create the new safe and register the guard.
        SafeInstance memory safeInstance = _setupSafe(ownerkeys, threshold);
        livenessGuard = new WrappedGuard(safeInstance.safe);
        safeInstance.setGuard(address(livenessGuard));

        for (uint256 i; i < changes.length; i++) {
            vm.warp(block.timestamp + changes[i].timeDelta);
            OwnerChange memory change = changes[i];
            address[] memory currentOwners = safeInstance.safe.getOwners();

            // Create a new owner address to add and store the key
            (address newOwner, uint256 newKey) = makeAddrAndKey(string.concat("new owner", vm.toString(i)));
            privateKeys[newOwner] = newKey;

            OwnerOp op = OwnerOp(bound(change.operation, 0, uint256(type(OwnerOp).max)));

            assertEq(livenessGuard.ownersBeforeLength(), 0);
            if (op == OwnerOp.Add) {
                assertEq(livenessGuard.lastLive(newOwner), 0);
                assertFalse(safeInstance.safe.isOwner(newOwner));
                change.newThreshold = bound(change.newThreshold, 1, currentOwners.length + 1);

                safeInstance.addOwnerWithThreshold(newOwner, change.newThreshold);

                assertTrue(safeInstance.safe.isOwner(newOwner));
                assertEq(livenessGuard.lastLive(newOwner), block.timestamp);
            } else {
                // Ensure we're removing an owner at an index within bounds
                uint256 ownerIndexToRemove = bound(change.ownerIndex, 0, currentOwners.length - 1);
                address ownerToRemove = currentOwners[ownerIndexToRemove];
                address prevOwner = safeInstance.getPrevOwner(ownerToRemove);

                if (op == OwnerOp.Remove) {
                    if (currentOwners.length == 1) continue;
                    assertGe(livenessGuard.lastLive(ownerToRemove), 0);
                    assertTrue(safeInstance.safe.isOwner(ownerToRemove));
                    change.newThreshold = bound(change.newThreshold, 1, currentOwners.length - 1);

                    safeInstance.removeOwner(prevOwner, ownerToRemove, change.newThreshold);

                    assertFalse(safeInstance.safe.isOwner(ownerToRemove));
                    assertEq(livenessGuard.lastLive(ownerToRemove), 0);
                } else if (op == OwnerOp.Swap) {
                    assertGe(livenessGuard.lastLive(ownerToRemove), 0);
                    assertTrue(safeInstance.safe.isOwner(ownerToRemove));

                    safeInstance.swapOwner(prevOwner, ownerToRemove, newOwner);

                    assertTrue(safeInstance.safe.isOwner(newOwner));
                    assertFalse(safeInstance.safe.isOwner(ownerToRemove));
                    assertEq(livenessGuard.lastLive(ownerToRemove), 0);
                    assertEq(livenessGuard.lastLive(newOwner), block.timestamp);
                }
            }
            assertEq(livenessGuard.ownersBeforeLength(), 0);
            _refreshOwners(safeInstance);
        }
    }

    /// @dev Refreshes the owners and ownerPKs arrays in the SafeInstance
    function _refreshOwners(SafeInstance memory instance) internal view {
        // Get the current owners
        instance.owners = instance.safe.getOwners();

        // Looks up the private key for each owner
        uint256[] memory unsortedOwnerPKs = new uint256[](instance.owners.length);
        for (uint256 i; i < instance.owners.length; i++) {
            unsortedOwnerPKs[i] = privateKeys[instance.owners[i]];
        }

        // Sort the keys by address and store them in the SafeInstance
        instance.ownerPKs = SafeTestLib.sortPKsByComputedAddress(unsortedOwnerPKs);

        // Overwrite the SafeInstances owners array with the computed addresses from the ownerPKs array
        for (uint256 i; i < instance.owners.length; i++) {
            instance.owners[i] = SafeTestLib.getAddr(instance.ownerPKs[i]);
        }
    }
}
