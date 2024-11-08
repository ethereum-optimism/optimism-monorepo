// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Test } from "forge-std/Test.sol";
import { GnosisSafe as Safe } from "safe-contracts/GnosisSafe.sol";
import { OwnerManager } from "safe-contracts/base/OwnerManager.sol";
import "test/safe-tools/SafeTestTools.sol";

import { LivenessModule } from "src/safe/LivenessModule.sol";
import { LivenessGuard } from "src/safe/LivenessGuard.sol";

contract LivenessModule_TestInit is Test, SafeTestTools {
    using SafeTestLib for SafeInstance;

    error OwnerRemovalFailed(string reason);

    // LivenessModule events
    event SignersRecorded(bytes32 indexed txHash, address[] signers);
    event RemovedOwner(address indexed owner);
    event OwnershipTransferredToFallback();

    uint256 constant INIT_TIME = 10;
    uint256 constant LIVENESS_INTERVAL = 30 days;
    uint256 constant MIN_OWNERS = 6;
    uint256 constant THRESHOLD_PERCENTAGE = 75;
    LivenessModule livenessModule;
    LivenessGuard livenessGuard;
    SafeInstance safeInstance;
    address fallbackOwner;

    /// @dev Removes an owner from the safe
    function _removeAnOwner(address _ownerToRemove, address[] memory _owners) internal {
        address[] memory prevOwners = new address[](1);
        address[] memory ownersToRemove = new address[](1);
        ownersToRemove[0] = _ownerToRemove;
        prevOwners[0] = SafeTestLib.getPrevOwnerFromList(_ownerToRemove, _owners);

        livenessModule.removeOwners(prevOwners, ownersToRemove);
    }

    /// @dev Set the current time to after the liveness interval
    function _warpPastLivenessInterval() internal {
        vm.warp(INIT_TIME + LIVENESS_INTERVAL + 1);
    }

    /// @dev Helper function to calculate the required threshold for a given number of owners and threshold percentage.
    ///      This does a lot of extra work to set up a dummy LivenessModule and Safe, but helps to keep the tests clean.
    function _getRequiredThreshold(uint256 _numOwners, uint256 _thresholdPercentage) internal returns (uint256) {
        // Mock calls made to the Safe by the LivenessModule's constructor
        address[] memory dummyOwners = new address[](_numOwners);
        dummyOwners[0] = address(0);
        vm.mockCall(address(0), abi.encodeCall(OwnerManager.getOwners, ()), abi.encode(dummyOwners));
        vm.mockCall(address(0), abi.encodeCall(OwnerManager.getThreshold, ()), abi.encode(dummyOwners.length));
        // This module is only used to calculate the required threshold for the Safe. It's state will not be modified
        // after it is created so the constructor arguments are not important.
        LivenessModule dummyLivenessModule = new LivenessModule({
            _safe: Safe(payable(address(0))),
            _livenessGuard: LivenessGuard(address(0)),
            _livenessInterval: 0,
            _thresholdPercentage: _thresholdPercentage,
            _minOwners: 0,
            _fallbackOwner: address(0)
        });
        return dummyLivenessModule.getRequiredThreshold(_numOwners);
    }

    /// @dev Sets up the test environment
    function setUp() public virtual {
        // Set the block timestamp to the initTime, so that signatures recorded in the first block
        // are non-zero.
        vm.warp(INIT_TIME);

        // Create a Safe with 10 owners
        (, uint256[] memory keys) = SafeTestLib.makeAddrsAndKeys("moduleTest", 10);
        safeInstance = _setupSafe(keys, 8);

        livenessGuard = new LivenessGuard(safeInstance.safe);
        fallbackOwner = makeAddr("fallbackOwner");
        livenessModule = new LivenessModule({
            _safe: safeInstance.safe,
            _livenessGuard: livenessGuard,
            _livenessInterval: LIVENESS_INTERVAL,
            _thresholdPercentage: THRESHOLD_PERCENTAGE,
            _minOwners: MIN_OWNERS,
            _fallbackOwner: fallbackOwner
        });
        safeInstance.setGuard(address(livenessGuard));
        safeInstance.enableModule(address(livenessModule));
    }
}

contract LivenessModule_Constructor_TestFail is LivenessModule_TestInit {
    /// @dev Tests that the constructor fails if the minOwners is greater than the number of owners
    function test_constructor_minOwnersGreaterThanOwners_reverts() external {
        vm.expectRevert("LivenessModule: minOwners must be less than the number of owners");
        new LivenessModule({
            _safe: safeInstance.safe,
            _livenessGuard: livenessGuard,
            _livenessInterval: LIVENESS_INTERVAL,
            _thresholdPercentage: THRESHOLD_PERCENTAGE,
            _minOwners: safeInstance.owners.length + 1,
            _fallbackOwner: address(0)
        });
    }
}

contract LivenessModule_Getters_Test is LivenessModule_TestInit {
    /// @dev Tests if the getters work correctly
    function test_getters_works() external view {
        assertEq(address(livenessModule.safe()), address(safeInstance.safe));
        assertEq(address(livenessModule.livenessGuard()), address(livenessGuard));
        assertEq(livenessModule.livenessInterval(), 30 days);
        assertEq(livenessModule.minOwners(), 6);
        assertEq(livenessModule.thresholdPercentage(), THRESHOLD_PERCENTAGE);
        assertEq(safeInstance.safe.getThreshold(), livenessModule.getRequiredThreshold(safeInstance.owners.length));
        assertEq(livenessModule.fallbackOwner(), fallbackOwner);
        assertFalse(livenessModule.ownershipTransferredToFallback());
    }
}

contract LivenessModule_CanRemove_TestFail is LivenessModule_TestInit {
    /// @dev Tests if canRemove work correctly
    function test_canRemove_notSafeOwner_reverts() external {
        address nonOwner = makeAddr("nonOwner");
        vm.expectRevert("LivenessModule: the owner to remove must be an owner of the Safe");
        livenessModule.canRemove(nonOwner);
    }
}

contract LivenessModule_CanRemove_Test is LivenessModule_TestInit {
    /// @dev Tests if canRemove work correctly
    function test_canRemove_works() external {
        _warpPastLivenessInterval();
        bool canRemove = livenessModule.canRemove(safeInstance.owners[0]);
        assertTrue(canRemove);
    }
}

contract LivenessModule_GetRequiredThreshold_Test is LivenessModule_TestInit {
    /// @dev Tests if getRequiredThreshold work correctly by implementing the same logic in a different manner
    function _getLeastIntegerValueAbovePercentage(
        uint256 _total,
        uint256 _percentage
    )
        internal
        pure
        returns (uint256)
    {
        require(_percentage > 0 && _percentage <= 100);
        uint256 toAdd;

        // If the total multiplied by the percentage is not divisible by 100, we need to add 1 to the result to
        // compensate for the rounding down by integer division.
        if ((_total * _percentage) % 100 > 0) {
            toAdd = 1;
        }
        return (_total * _percentage) / 100 + toAdd;
    }

    /// @dev Differentially tests the getRequiredThreshold function against _getLeastIntegerValueAbovePercentage
    function testFuzz_getRequiredThreshold_works(uint256 _numOwners, uint256 _percentage) external {
        // Enforce valid percentages
        uint256 percentage = bound(_percentage, 1, 100);
        // Enforce a sane number of owners to keep runtime in check
        uint256 numOwners = bound(_numOwners, 1, 100);
        assertEq(
            _getRequiredThreshold(numOwners, percentage), _getLeastIntegerValueAbovePercentage(numOwners, percentage)
        );
    }

    /// @dev check the return values of the getRequiredThreshold function against the boundary conditions of 1 and 100
    ///      percent.
    function testFuzz_getRequiredThreshold_atBoundaries_works(uint256 _numOwners) external {
        // Enforce a sane number of owners to keep runtime in check
        uint256 numOwners = bound(_numOwners, 1, 100);
        assertEq(_getRequiredThreshold(numOwners, 100), numOwners);
        assertEq(_getRequiredThreshold(numOwners, 1), 1);
    }

    /// @dev check the return values of the getRequiredThreshold function against manually
    ///      calculated values.
    function test_getRequiredThreshold_hardcoded_works() external {
        // 75% threshold
        assertEq(_getRequiredThreshold(20, 75), 15);
        assertEq(_getRequiredThreshold(19, 75), 15);
        assertEq(_getRequiredThreshold(18, 75), 14);
        assertEq(_getRequiredThreshold(17, 75), 13);
        assertEq(_getRequiredThreshold(16, 75), 12);
        assertEq(_getRequiredThreshold(15, 75), 12);
        assertEq(_getRequiredThreshold(14, 75), 11);
        assertEq(_getRequiredThreshold(13, 75), 10);
        assertEq(_getRequiredThreshold(12, 75), 9);
        assertEq(_getRequiredThreshold(11, 75), 9);
        assertEq(_getRequiredThreshold(10, 75), 8);
        assertEq(_getRequiredThreshold(9, 75), 7);
        assertEq(_getRequiredThreshold(8, 75), 6);
        assertEq(_getRequiredThreshold(7, 75), 6);
        assertEq(_getRequiredThreshold(6, 75), 5);
        assertEq(_getRequiredThreshold(5, 75), 4);
        assertEq(_getRequiredThreshold(4, 75), 3);
        assertEq(_getRequiredThreshold(3, 75), 3);
        assertEq(_getRequiredThreshold(2, 75), 2);
        assertEq(_getRequiredThreshold(1, 75), 1);

        // 33% threshold
        assertEq(_getRequiredThreshold(20, 33), 7);
        assertEq(_getRequiredThreshold(19, 33), 7);
        assertEq(_getRequiredThreshold(18, 33), 6);
        assertEq(_getRequiredThreshold(17, 33), 6);
        assertEq(_getRequiredThreshold(16, 33), 6);
        assertEq(_getRequiredThreshold(15, 33), 5);
        assertEq(_getRequiredThreshold(14, 33), 5);
        assertEq(_getRequiredThreshold(13, 33), 5);
        assertEq(_getRequiredThreshold(12, 33), 4);
        assertEq(_getRequiredThreshold(11, 33), 4);
        assertEq(_getRequiredThreshold(10, 33), 4);
        assertEq(_getRequiredThreshold(9, 33), 3);
        assertEq(_getRequiredThreshold(8, 33), 3);
        assertEq(_getRequiredThreshold(7, 33), 3);
        assertEq(_getRequiredThreshold(6, 33), 2);
        assertEq(_getRequiredThreshold(5, 33), 2);
        assertEq(_getRequiredThreshold(4, 33), 2);
        assertEq(_getRequiredThreshold(3, 33), 1);
        assertEq(_getRequiredThreshold(2, 33), 1);
        assertEq(_getRequiredThreshold(1, 33), 1);
    }
}

contract LivenessModule_RemoveOwners_TestFail is LivenessModule_TestInit {
    using SafeTestLib for SafeInstance;

    /// @dev Tests with different length owner arrays
    function test_removeOwners_differentArrayLengths_reverts() external {
        address[] memory ownersToRemove = new address[](1);
        address[] memory prevOwners = new address[](2);
        vm.expectRevert("LivenessModule: arrays must be the same length");
        livenessModule.removeOwners(prevOwners, ownersToRemove);
    }

    /// @dev Test removing an owner which has recently signed a transaction
    function test_removeOwners_ownerHasSignedRecently_reverts() external {
        /// Will sign a transaction with the first M owners in the owners list
        safeInstance.execTransaction({ to: address(1111), value: 0, data: hex"abba" });

        address[] memory owners = safeInstance.safe.getOwners();

        vm.expectRevert("LivenessModule: the owner to remove has signed recently");
        _removeAnOwner(safeInstance.owners[0], owners);
    }

    /// @dev Test removing an owner which has recently called showLiveness
    function test_removeOwners_ownerHasShownLivenessRecently_reverts() external {
        /// Will sign a transaction with the first M owners in the owners list
        vm.prank(safeInstance.owners[0]);
        livenessGuard.showLiveness();
        address[] memory owners = safeInstance.safe.getOwners();
        vm.expectRevert("LivenessModule: the owner to remove has signed recently");
        _removeAnOwner(safeInstance.owners[0], owners);
    }

    /// @dev Test removing an owner with an incorrect previous owner
    function test_removeOwners_wrongPreviousOwner_reverts() external {
        address[] memory prevOwners = new address[](1);
        address[] memory ownersToRemove = new address[](1);
        ownersToRemove[0] = safeInstance.owners[0];
        // Incorrectly set the previous owner as equal to the owner to remove, which will cause the Safe to revert.
        prevOwners[0] = ownersToRemove[0];

        _warpPastLivenessInterval();
        vm.expectRevert(
            abi.encodeWithSelector(OwnerRemovalFailed.selector, (abi.encodeWithSignature("Error(string)", "GS205")))
        );
        livenessModule.removeOwners(prevOwners, ownersToRemove);
    }

    /// @dev Tests if removing all owners works correctly
    function test_removeOwners_swapToFallbackOwner_reverts() external {
        uint256 numOwners = safeInstance.owners.length;

        address[] memory ownersToRemove = new address[](numOwners);
        for (uint256 i; i < numOwners; i++) {
            ownersToRemove[i] = safeInstance.owners[i];
        }
        address[] memory prevOwners = safeInstance.getPrevOwners(ownersToRemove);

        // Incorrectly set the final owner to address(0), causing the Safe to revert.
        ownersToRemove[ownersToRemove.length - 1] = address(0);

        _warpPastLivenessInterval();
        vm.expectRevert(
            abi.encodeWithSelector(OwnerRemovalFailed.selector, (abi.encodeWithSignature("Error(string)", "GS203")))
        );
        livenessModule.removeOwners(prevOwners, ownersToRemove);
    }

    /// @dev Tests if remove owners reverts if it removes too many owners without removing all of them
    function test_removeOwners_belowMinButNotEmptied_reverts() external {
        // Remove all but one owner
        uint256 numOwners = safeInstance.owners.length - 2;

        address[] memory ownersToRemove = new address[](numOwners);
        for (uint256 i; i < numOwners; i++) {
            ownersToRemove[i] = safeInstance.owners[i];
        }
        address[] memory prevOwners = safeInstance.getPrevOwners(ownersToRemove);

        _warpPastLivenessInterval();
        vm.expectRevert(
            "LivenessModule: must remove all owners and transfer to fallback owner if numOwners < minOwners"
        );
        livenessModule.removeOwners(prevOwners, ownersToRemove);
    }

    /// @dev Tests if remove owners reverts if it removes too many owners transferring to the shutDown owner
    function test_removeOwners_belowEmptiedButNotShutDown_reverts() external {
        // Remove all but one owner
        uint256 numOwners = safeInstance.owners.length - 1;

        address[] memory ownersToRemove = new address[](numOwners);
        for (uint256 i; i < numOwners; i++) {
            ownersToRemove[i] = safeInstance.owners[i];
        }
        address[] memory prevOwners = safeInstance.getPrevOwners(ownersToRemove);

        _warpPastLivenessInterval();
        vm.expectRevert("LivenessModule: must transfer ownership to fallback owner");
        livenessModule.removeOwners(prevOwners, ownersToRemove);
    }

    /// @dev Tests if remove owners reverts if the current Safe.guard does note match the expected
    ///      livenessGuard address.
    function test_removeOwners_guardChanged_reverts() external {
        address[] memory ownersToRemove = new address[](1);
        ownersToRemove[0] = safeInstance.owners[0];
        address[] memory prevOwners = safeInstance.getPrevOwners(ownersToRemove);

        // Change the guard
        livenessGuard = new LivenessGuard(safeInstance.safe);
        safeInstance.setGuard(address(livenessGuard));

        _warpPastLivenessInterval();
        vm.expectRevert("LivenessModule: guard has been changed");
        livenessModule.removeOwners(prevOwners, ownersToRemove);
    }

    function test_removeOwners_invalidThreshold_reverts() external {
        address[] memory ownersToRemove = new address[](0);
        address[] memory prevOwners = new address[](0);
        uint256 wrongThreshold = safeInstance.safe.getThreshold() + 1;

        vm.mockCall(
            address(safeInstance.safe), abi.encodeCall(OwnerManager.getThreshold, ()), abi.encode(wrongThreshold)
        );

        _warpPastLivenessInterval();
        vm.expectRevert("LivenessModule: Insufficient threshold for the number of owners");
        livenessModule.removeOwners(prevOwners, ownersToRemove);
    }
}

contract LivenessModule_RemoveOwners_Test is LivenessModule_TestInit {
    using SafeTestLib for SafeInstance;

    /// @dev Tests if removing one owner works correctly
    function test_removeOwners_oneOwner_succeeds() external {
        uint256 ownersBefore = safeInstance.owners.length;
        address ownerToRemove = safeInstance.owners[0];

        _warpPastLivenessInterval();
        vm.expectEmit(address(livenessModule));
        emit RemovedOwner(ownerToRemove);
        _removeAnOwner(ownerToRemove, safeInstance.owners);

        assertFalse(safeInstance.safe.isOwner(ownerToRemove));
        assertEq(safeInstance.safe.getOwners().length, ownersBefore - 1);
    }

    /// @dev Tests if removing all owners works correctly
    function test_removeOwners_allOwners_succeeds() external {
        uint256 numOwners = safeInstance.owners.length;

        address[] memory ownersToRemove = new address[](numOwners);
        for (uint256 i; i < numOwners; i++) {
            ownersToRemove[i] = safeInstance.owners[i];
        }
        address[] memory prevOwners = safeInstance.getPrevOwners(ownersToRemove);

        _warpPastLivenessInterval();
        for (uint256 i; i < ownersToRemove.length; i++) {
            if (i != ownersToRemove.length - 1) {
                vm.expectEmit(address(livenessModule));
                emit RemovedOwner(ownersToRemove[i]);
            }
        }

        vm.expectEmit(address(livenessModule));
        emit OwnershipTransferredToFallback();

        livenessModule.removeOwners(prevOwners, ownersToRemove);
        assertEq(safeInstance.safe.getOwners().length, 1);
        assertEq(safeInstance.safe.getOwners()[0], fallbackOwner);
        assertEq(safeInstance.safe.getThreshold(), 1);

        // Ensure that the LivenessModule's removeOwners function is now disabled
        assertTrue(livenessModule.ownershipTransferredToFallback());
        vm.expectRevert(
            "LivenessModule: The safe has been shutdown, the LivenessModule and LivenessGuard should be removed or replaced."
        );
        livenessModule.removeOwners(prevOwners, ownersToRemove);
    }
}

contract LivenessModule_RemoveOwnersFuzz_Test is LivenessModule_TestInit {
    using SafeTestLib for SafeInstance;

    /// @dev We put this array in storage so that we can more easily populate it using push in the tests below.
    address[] ownersToRemove;

    /// @dev Options for handling the event that the number of owners remaining is less than minOwners
    enum ShutDownBehavior {
        // Correctly removes the owners and transfers to the shutDown owner
        Correct,
        // Removes all but one owner, and does not transfer to the shutDown owner
        DoesNotTransferToFallbackOwner,
        // Leaves more than one owner when below minOwners
        DoesNotRemoveAllOwners
    }

    /// @dev This contract inherits the storage layout from the LivenessModule_TestInit contract, but we
    ///      override the base setUp function, to avoid instantiating an unnecessary Safe and liveness checking system.
    function setUp() public override {
        vm.warp(INIT_TIME);
        fallbackOwner = makeAddr("fallbackOwner");
    }

    /// @dev Extracts the setup of the test environment into a separate function.
    function _prepare(
        uint256 _numOwners,
        uint256 _minOwners,
        uint256 _numLiveOwners,
        uint256 _thresholdPercentage
    )
        internal
        returns (uint256 numOwners_, uint256 minOwners_, uint256 numLiveOwners_, uint256 thresholdPercentage_)
    {
        // First we modify the test parameters to ensure that they describe a plausible starting point.
        //
        // _numOwners must be at least 4, so that _minOwners can be set to at least 3 by the following bound() call.
        // Limiting the owner set to 20 helps to keep the runtime of the test reasonable.
        numOwners_ = bound(_numOwners, 4, 20);
        // _minOwners must be at least 3, otherwise we don't have any range below _minOwners in which to test all of the
        // ShutDownBehavior options.
        minOwners_ = bound(_minOwners, 3, numOwners_ - 1);

        // Ensure that _numLiveOwners is less than _numOwners so that we can remove at least one owner.
        numLiveOwners_ = bound(_numLiveOwners, 0, numOwners_ - 1);

        // thresholdPercentage must be between a percentage greater than 0.
        thresholdPercentage_ = bound(_thresholdPercentage, 1, 100);

        // The above bounds are a bit tricky, so we assert that the resulting parameters enable us to test all possible
        // success and revert cases in the removeOwners function.
        // This is also necessary to avoid underflows or out of bounds accesses in the test.
        assertTrue(
            numOwners_ > minOwners_ // We need to be able to remove at least one owner
                && numOwners_ >= numLiveOwners_ // We can have more live owners than there are owners
                && minOwners_ >= 3 // Allows us to test all of the ShutDownBehavior options when removing an owner
        );

        // Create a Safe with _numOwners owners
        (, uint256[] memory keys) = SafeTestLib.makeAddrsAndKeys("rmOwnersTest", numOwners_);
        uint256 threshold = _getRequiredThreshold(numOwners_, thresholdPercentage_);
        safeInstance = _setupSafe(keys, threshold);
        livenessGuard = new LivenessGuard(safeInstance.safe);
        livenessModule = new LivenessModule({
            _safe: safeInstance.safe,
            _livenessGuard: livenessGuard,
            _livenessInterval: LIVENESS_INTERVAL,
            _thresholdPercentage: thresholdPercentage_,
            _minOwners: minOwners_,
            _fallbackOwner: fallbackOwner
        });
        safeInstance.setGuard(address(livenessGuard));
        safeInstance.enableModule(address(livenessModule));

        // Warp ahead so that all owners non-live
        _warpPastLivenessInterval();
    }

    /// @dev Tests if removing owners works correctly for various safe configurations and numbeers of live owners
    function testFuzz_removeOwners_works(
        uint256 _numOwners,
        uint256 _minOwners,
        uint256 _numLiveOwners,
        uint256 _thresholdPercentage,
        uint256 _shutDownBehavior,
        uint256 _numOwnersToRemoveinShutDown
    )
        external
    {
        // Prepare the test env and test params
        (uint256 numOwners, uint256 minOwners, uint256 numLiveOwners, uint256 thresholdPercentage) =
            _prepare(_numOwners, _minOwners, _numLiveOwners, _thresholdPercentage);

        // Create an array of live owners, and call showLiveness for each of them
        address[] memory liveOwners = new address[](numLiveOwners);
        for (uint256 i; i < numLiveOwners; i++) {
            liveOwners[i] = safeInstance.owners[i];
            vm.prank(safeInstance.owners[i]);
            livenessGuard.showLiveness();
        }

        address[] memory nonLiveOwners = new address[](numOwners - numLiveOwners);
        for (uint256 i; i < numOwners - numLiveOwners; i++) {
            nonLiveOwners[i] = safeInstance.owners[i + numLiveOwners];
        }

        address[] memory prevOwners;
        if (numLiveOwners >= minOwners) {
            // The safe will remain above the minimum number of owners, so we can remove only those owners which are not
            // live.
            prevOwners = safeInstance.getPrevOwners(nonLiveOwners);
            livenessModule.removeOwners(prevOwners, nonLiveOwners);

            // Validate the resulting state of the Safe
            assertEq(safeInstance.safe.getOwners().length, numLiveOwners);
            assertEq(safeInstance.safe.getThreshold(), _getRequiredThreshold(numLiveOwners, thresholdPercentage));
            for (uint256 i; i < numLiveOwners; i++) {
                assertTrue(safeInstance.safe.isOwner(liveOwners[i]));
            }
            for (uint256 i; i < nonLiveOwners.length; i++) {
                assertFalse(safeInstance.safe.isOwner(nonLiveOwners[i]));
            }
        } else {
            // The number of non-live owners will push the safe below the minimum number of owners.
            // We need to test all of the possible ShutDownBehavior options, so we'll create a ShutDownBehavior enum
            // from the _shutDownBehavior input.
            ShutDownBehavior shutDownBehavior =
                ShutDownBehavior(bound(_shutDownBehavior, 0, uint256(type(ShutDownBehavior).max)));
            // The safe is below the minimum number of owners.
            // The ShutDownBehavior enum determines how we handle this case.
            if (shutDownBehavior == ShutDownBehavior.Correct) {
                // We remove all owners, and transfer ownership to the shutDown owner.
                // but we need to do remove the non-live owners first, so we reverse the owners array, since
                // the first owners in the array were the ones to call showLiveness.
                for (uint256 i; i < numOwners; i++) {
                    ownersToRemove.push(safeInstance.owners[numOwners - i - 1]);
                }
                prevOwners = safeInstance.getPrevOwners(ownersToRemove);
                livenessModule.removeOwners(prevOwners, ownersToRemove);

                // Validate the resulting state of the Safe
                assertEq(safeInstance.safe.getOwners().length, 1);
                assertEq(safeInstance.safe.getOwners()[0], fallbackOwner);
                assertEq(safeInstance.safe.getThreshold(), 1);
                // Ensure that the LivenessModule's removeOwners function is now disabled
                assertTrue(livenessModule.ownershipTransferredToFallback());
                vm.expectRevert(
                    "LivenessModule: The safe has been shutdown, the LivenessModule and LivenessGuard should be removed or replaced."
                );
                livenessModule.removeOwners(prevOwners, ownersToRemove);
            } else {
                // For both of the incorrect behaviors, we need to calculate the number of owners to remove to
                // trigger that behavior. We initialize that value here then set it in the if statements below.
                uint256 numOwnersToRemoveinShutDown;
                if (shutDownBehavior == ShutDownBehavior.DoesNotRemoveAllOwners) {
                    // In the DoesNotRemoveAllOwners case, we should have more than 1 of the pre-existing owners
                    // remaining
                    numOwnersToRemoveinShutDown =
                        bound(_numOwnersToRemoveinShutDown, numOwners - minOwners + 1, numOwners - 2);
                    for (uint256 i; i < numOwnersToRemoveinShutDown; i++) {
                        // Add non-live owners to remove first
                        if (i < nonLiveOwners.length) {
                            ownersToRemove.push(nonLiveOwners[i]);
                        } else {
                            // Then add live owners to remove
                            ownersToRemove.push(liveOwners[i - nonLiveOwners.length]);
                        }
                    }
                    prevOwners = safeInstance.getPrevOwners(ownersToRemove);
                    vm.expectRevert(
                        "LivenessModule: must remove all owners and transfer to fallback owner if numOwners < minOwners"
                    );
                    livenessModule.removeOwners(prevOwners, ownersToRemove);
                } else if (shutDownBehavior == ShutDownBehavior.DoesNotTransferToFallbackOwner) {
                    // In the DoesNotRemoveAllOwners case, we should have exactly 1 pre-existing owners remaining
                    numOwnersToRemoveinShutDown = numOwners - 1;
                    for (uint256 i; i < numOwnersToRemoveinShutDown; i++) {
                        // Add non-live owners to remove first
                        if (i < nonLiveOwners.length) {
                            ownersToRemove.push(nonLiveOwners[i]);
                        } else {
                            // Then add live owners to remove
                            ownersToRemove.push(liveOwners[i - nonLiveOwners.length]);
                        }
                    }
                    prevOwners = safeInstance.getPrevOwners(ownersToRemove);
                    vm.expectRevert("LivenessModule: must transfer ownership to fallback owner");
                    livenessModule.removeOwners(prevOwners, ownersToRemove);
                }
                // For both of the incorrect behaviors, verify no change to the Safe state
                assertEq(safeInstance.safe.getOwners().length, numOwners);
                assertEq(safeInstance.safe.getThreshold(), _getRequiredThreshold(numOwners, thresholdPercentage));
                for (uint256 i; i < numOwners; i++) {
                    assertTrue(safeInstance.safe.isOwner(safeInstance.owners[i]));
                }
            }
        }
    }
}
