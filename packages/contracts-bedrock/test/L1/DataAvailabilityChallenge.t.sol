// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import {
    IDataAvailabilityChallenge,
    ChallengeStatus,
    Challenge,
    CommitmentType
} from "src/L1/interfaces/IDataAvailabilityChallenge.sol";
import { computeCommitmentKeccak256 } from "src/L1/DataAvailabilityChallenge.sol";
import { CommonTest } from "test/setup/CommonTest.sol";
import { Preinstalls } from "src/libraries/Preinstalls.sol";

contract DataAvailabilityChallengeTest is CommonTest {
    function setUp() public virtual override {
        super.enableAltDA();
        super.setUp();
    }

    function testDeposit() public {
        assertEq(dataAvailabilityChallenge.balances(address(this)), 0);
        dataAvailabilityChallenge.deposit{ value: 1000 }();
        assertEq(dataAvailabilityChallenge.balances(address(this)), 1000);
    }

    function testReceive() public {
        assertEq(dataAvailabilityChallenge.balances(address(this)), 0);
        (bool success,) = payable(address(dataAvailabilityChallenge)).call{ value: 1000 }("");
        assertTrue(success);
        assertEq(dataAvailabilityChallenge.balances(address(this)), 1000);
    }

    function testWithdraw(address sender, uint256 amount) public {
        assumePayable(sender);
        assumeNotPrecompile(sender);
        // EntryPoint will revert if using amount > type(uint112).max.
        vm.assume(sender != Preinstalls.EntryPoint_v060);
        vm.assume(sender != address(dataAvailabilityChallenge));
        vm.assume(sender.balance == 0);
        vm.deal(sender, amount);

        vm.prank(sender);
        dataAvailabilityChallenge.deposit{ value: amount }();

        assertEq(dataAvailabilityChallenge.balances(sender), amount);
        assertEq(sender.balance, 0);

        vm.prank(sender);
        dataAvailabilityChallenge.withdraw();

        assertEq(dataAvailabilityChallenge.balances(sender), 0);
        assertEq(sender.balance, amount);
    }

    function testChallengeSuccess(address challenger, uint256 challengedBlockNumber, bytes calldata preImage) public {
        bytes memory challengedCommitment = computeCommitmentKeccak256(preImage);

        // Assume the challenger is not the 0 address
        vm.assume(challenger != address(0));

        // Assume the block number is not close to the max uint256 value
        vm.assume(
            challengedBlockNumber
                < type(uint256).max - dataAvailabilityChallenge.challengeWindow()
                    - dataAvailabilityChallenge.resolveWindow()
        );
        uint256 requiredBond = dataAvailabilityChallenge.bondSize();

        // Move to a block after the challenged block
        vm.roll(challengedBlockNumber + 1);

        // Deposit the required bond
        vm.deal(challenger, requiredBond);
        vm.prank(challenger);
        dataAvailabilityChallenge.deposit{ value: requiredBond }();

        // Expect the challenge status to be uninitialized
        assertEq(
            uint8(dataAvailabilityChallenge.getChallengeStatus(challengedBlockNumber, challengedCommitment)),
            uint8(ChallengeStatus.Uninitialized)
        );

        // Challenge a (blockNumber,hash) tuple
        vm.prank(challenger);
        dataAvailabilityChallenge.challenge(challengedBlockNumber, challengedCommitment);

        // Challenge should have been created
        Challenge memory challenge = dataAvailabilityChallenge.getChallenge(challengedBlockNumber, challengedCommitment);
        assertEq(challenge.challenger, challenger);
        assertEq(challenge.startBlock, block.number);
        assertEq(challenge.resolvedBlock, 0);
        assertEq(challenge.lockedBond, requiredBond);
        assertEq(
            uint8(dataAvailabilityChallenge.getChallengeStatus(challengedBlockNumber, challengedCommitment)),
            uint8(ChallengeStatus.Active)
        );

        // Challenge should have decreased the challenger's bond size
        assertEq(dataAvailabilityChallenge.balances(challenger), 0);
    }

    function testChallengeDeposit(address challenger, uint256 challengedBlockNumber, bytes memory preImage) public {
        bytes memory challengedCommitment = computeCommitmentKeccak256(preImage);

        // Assume the challenger is not the 0 address
        vm.assume(challenger != address(0));

        // Assume the block number is not close to the max uint256 value
        vm.assume(
            challengedBlockNumber
                < type(uint256).max - dataAvailabilityChallenge.challengeWindow()
                    - dataAvailabilityChallenge.resolveWindow()
        );
        uint256 requiredBond = dataAvailabilityChallenge.bondSize();

        // Move to a block after the challenged block
        vm.roll(challengedBlockNumber + 1);

        // Expect the challenge status to be uninitialized
        assertEq(
            uint8(dataAvailabilityChallenge.getChallengeStatus(challengedBlockNumber, challengedCommitment)),
            uint8(ChallengeStatus.Uninitialized)
        );

        // Deposit the required bond as part of the challenge transaction
        vm.deal(challenger, requiredBond);
        vm.prank(challenger);
        dataAvailabilityChallenge.challenge{ value: requiredBond }(challengedBlockNumber, challengedCommitment);

        // Challenge should have been created
        Challenge memory challenge = dataAvailabilityChallenge.getChallenge(challengedBlockNumber, challengedCommitment);
        assertEq(challenge.challenger, challenger);
        assertEq(challenge.startBlock, block.number);
        assertEq(challenge.resolvedBlock, 0);
        assertEq(challenge.lockedBond, requiredBond);
        assertEq(
            uint8(dataAvailabilityChallenge.getChallengeStatus(challengedBlockNumber, challengedCommitment)),
            uint8(ChallengeStatus.Active)
        );

        // Challenge should have decreased the challenger's bond size
        assertEq(dataAvailabilityChallenge.balances(challenger), 0);
    }

    function testChallengeFailBondTooLow() public {
        uint256 requiredBond = dataAvailabilityChallenge.bondSize();
        uint256 actualBond = requiredBond - 1;
        dataAvailabilityChallenge.deposit{ value: actualBond }();

        vm.expectRevert(
            abi.encodeWithSelector(IDataAvailabilityChallenge.BondTooLow.selector, actualBond, requiredBond)
        );
        dataAvailabilityChallenge.challenge(0, computeCommitmentKeccak256("some hash"));
    }

    function testChallengeFailChallengeExists() public {
        // Move to a block after the hash to challenge
        vm.roll(2);

        // First challenge succeeds
        bytes memory challengedCommitment = computeCommitmentKeccak256("some data");
        dataAvailabilityChallenge.deposit{ value: dataAvailabilityChallenge.bondSize() }();
        dataAvailabilityChallenge.challenge(0, challengedCommitment);

        // Second challenge of the same hash/blockNumber fails
        dataAvailabilityChallenge.deposit{ value: dataAvailabilityChallenge.bondSize() }();
        vm.expectRevert(abi.encodeWithSelector(IDataAvailabilityChallenge.ChallengeExists.selector));
        dataAvailabilityChallenge.challenge(0, challengedCommitment);

        // Challenge succeed if the challenged block number is different
        dataAvailabilityChallenge.deposit{ value: dataAvailabilityChallenge.bondSize() }();
        dataAvailabilityChallenge.challenge(1, challengedCommitment);

        // Challenge succeed if the challenged hash is different
        dataAvailabilityChallenge.deposit{ value: dataAvailabilityChallenge.bondSize() }();
        dataAvailabilityChallenge.challenge(0, computeCommitmentKeccak256("some other hash"));
    }

    function testChallengeFailBeforeChallengeWindow() public {
        uint256 challengedBlockNumber = 1;
        bytes memory challengedCommitment = computeCommitmentKeccak256("some hash");

        // Move to challenged block
        vm.roll(challengedBlockNumber - 1);

        // Challenge fails because the current block number must be after the challenged block
        dataAvailabilityChallenge.deposit{ value: dataAvailabilityChallenge.bondSize() }();
        vm.expectRevert(abi.encodeWithSelector(IDataAvailabilityChallenge.ChallengeWindowNotOpen.selector));
        dataAvailabilityChallenge.challenge(challengedBlockNumber, challengedCommitment);
    }

    function testChallengeFailAfterChallengeWindow() public {
        uint256 challengedBlockNumber = 1;
        bytes memory challengedCommitment = computeCommitmentKeccak256("some hash");

        // Move to block after the challenge window
        vm.roll(challengedBlockNumber + dataAvailabilityChallenge.challengeWindow() + 1);

        // Challenge fails because the block number is after the challenge window
        dataAvailabilityChallenge.deposit{ value: dataAvailabilityChallenge.bondSize() }();
        vm.expectRevert(abi.encodeWithSelector(IDataAvailabilityChallenge.ChallengeWindowNotOpen.selector));
        dataAvailabilityChallenge.challenge(challengedBlockNumber, challengedCommitment);
    }

    function testResolveSuccess(
        address challenger,
        address resolver,
        bytes memory preImage,
        uint256 challengedBlockNumber,
        uint256 resolverRefundPercentage,
        uint128 txGasPrice
    )
        public
    {
        // Assume neither the challenger nor resolver is address(0) and that they're not the same entity
        vm.assume(challenger != address(0));
        vm.assume(resolver != address(0));
        vm.assume(challenger != resolver);

        // Bound the resolver refund percentage to 100
        resolverRefundPercentage = bound(resolverRefundPercentage, 0, 100);

        // Set the gas price to a fuzzed value to test bond distribution logic
        vm.txGasPrice(txGasPrice);

        // Change the resolver refund percentage
        vm.prank(dataAvailabilityChallenge.owner());
        dataAvailabilityChallenge.setResolverRefundPercentage(resolverRefundPercentage);

        // Assume the block number is not close to the max uint256 value
        vm.assume(
            challengedBlockNumber
                < type(uint256).max - dataAvailabilityChallenge.challengeWindow()
                    - dataAvailabilityChallenge.resolveWindow()
        );
        bytes memory challengedCommitment = computeCommitmentKeccak256(preImage);

        // Move to block after challenged block
        vm.roll(challengedBlockNumber + 1);

        // Challenge the hash
        uint256 bondSize = dataAvailabilityChallenge.bondSize();
        vm.deal(challenger, bondSize);
        vm.prank(challenger);
        dataAvailabilityChallenge.challenge{ value: bondSize }(challengedBlockNumber, challengedCommitment);

        // Store the address(0) balance before resolving to assert the burned amount later
        uint256 zeroAddressBalanceBeforeResolve = address(0).balance;

        // Resolve the challenge
        vm.prank(resolver);
        dataAvailabilityChallenge.resolve(challengedBlockNumber, challengedCommitment, preImage);

        // Expect the challenge to be resolved
        Challenge memory challenge = dataAvailabilityChallenge.getChallenge(challengedBlockNumber, challengedCommitment);

        assertEq(challenge.challenger, challenger);
        assertEq(challenge.lockedBond, 0);
        assertEq(challenge.startBlock, block.number);
        assertEq(challenge.resolvedBlock, block.number);
        assertEq(
            uint8(dataAvailabilityChallenge.getChallengeStatus(challengedBlockNumber, challengedCommitment)),
            uint8(ChallengeStatus.Resolved)
        );

        // Assert challenger balance after bond distribution
        uint256 resolutionCost = (
            dataAvailabilityChallenge.fixedResolutionCost()
                + preImage.length * dataAvailabilityChallenge.variableResolutionCost()
                    / dataAvailabilityChallenge.variableResolutionCostPrecision()
        ) * block.basefee;
        uint256 challengerRefund = bondSize > resolutionCost ? bondSize - resolutionCost : 0;
        assertEq(dataAvailabilityChallenge.balances(challenger), challengerRefund, "challenger refund");

        // Assert resolver balance after bond distribution
        uint256 resolverRefund = resolutionCost * dataAvailabilityChallenge.resolverRefundPercentage() / 100;
        resolverRefund = resolverRefund > resolutionCost ? resolutionCost : resolverRefund;
        resolverRefund = resolverRefund > bondSize ? bondSize : resolverRefund;
        assertEq(dataAvailabilityChallenge.balances(resolver), resolverRefund, "resolver refund");

        // Assert burned amount after bond distribution
        uint256 burned = bondSize - challengerRefund - resolverRefund;
        assertEq(address(0).balance - zeroAddressBalanceBeforeResolve, burned, "burned bond");
    }

    function testResolveFailNonExistentChallenge() public {
        bytes memory preImage = "some preimage";
        uint256 challengedBlockNumber = 1;

        // Move to block after challenged block
        vm.roll(challengedBlockNumber + 1);

        // Resolving a non-existent challenge fails
        vm.expectRevert(abi.encodeWithSelector(IDataAvailabilityChallenge.ChallengeNotActive.selector));
        dataAvailabilityChallenge.resolve(challengedBlockNumber, computeCommitmentKeccak256(preImage), preImage);
    }

    function testResolveFailResolved() public {
        bytes memory preImage = "some preimage";
        bytes memory challengedCommitment = computeCommitmentKeccak256(preImage);
        uint256 challengedBlockNumber = 1;

        // Move to block after challenged block
        vm.roll(challengedBlockNumber + 1);

        // Challenge the hash
        dataAvailabilityChallenge.deposit{ value: dataAvailabilityChallenge.bondSize() }();
        dataAvailabilityChallenge.challenge(challengedBlockNumber, challengedCommitment);

        // Resolve the challenge
        dataAvailabilityChallenge.resolve(challengedBlockNumber, challengedCommitment, preImage);

        // Resolving an already resolved challenge fails
        vm.expectRevert(abi.encodeWithSelector(IDataAvailabilityChallenge.ChallengeNotActive.selector));
        dataAvailabilityChallenge.resolve(challengedBlockNumber, challengedCommitment, preImage);
    }

    function testResolveFailExpired() public {
        bytes memory preImage = "some preimage";
        bytes memory challengedCommitment = computeCommitmentKeccak256(preImage);
        uint256 challengedBlockNumber = 1;

        // Move to block after challenged block
        vm.roll(challengedBlockNumber + 1);

        // Challenge the hash
        dataAvailabilityChallenge.deposit{ value: dataAvailabilityChallenge.bondSize() }();
        dataAvailabilityChallenge.challenge(challengedBlockNumber, challengedCommitment);

        // Move to a block after the resolve window
        vm.roll(block.number + dataAvailabilityChallenge.resolveWindow() + 1);

        // Resolving an expired challenge fails
        vm.expectRevert(abi.encodeWithSelector(IDataAvailabilityChallenge.ChallengeNotActive.selector));
        dataAvailabilityChallenge.resolve(challengedBlockNumber, challengedCommitment, preImage);
    }

    function testResolveFailAfterResolveWindow() public {
        bytes memory preImage = "some preimage";
        bytes memory challengedCommitment = computeCommitmentKeccak256(preImage);
        uint256 challengedBlockNumber = 1;

        // Move to block after challenged block
        vm.roll(challengedBlockNumber + 1);

        // Challenge the hash
        dataAvailabilityChallenge.deposit{ value: dataAvailabilityChallenge.bondSize() }();
        dataAvailabilityChallenge.challenge(challengedBlockNumber, challengedCommitment);

        // Move to block after resolve window
        vm.roll(block.number + dataAvailabilityChallenge.resolveWindow() + 1);

        // Resolve the challenge
        vm.expectRevert(abi.encodeWithSelector(IDataAvailabilityChallenge.ChallengeNotActive.selector));
        dataAvailabilityChallenge.resolve(challengedBlockNumber, challengedCommitment, preImage);
    }

    function testUnlockBondSuccess(bytes memory preImage, uint256 challengedBlockNumber) public {
        // Assume the block number is not close to the max uint256 value
        vm.assume(
            challengedBlockNumber
                < type(uint256).max - dataAvailabilityChallenge.challengeWindow()
                    - dataAvailabilityChallenge.resolveWindow()
        );
        bytes memory challengedCommitment = computeCommitmentKeccak256(preImage);

        // Move to block after challenged block
        vm.roll(challengedBlockNumber + 1);

        // Challenge the hash
        dataAvailabilityChallenge.deposit{ value: dataAvailabilityChallenge.bondSize() }();
        dataAvailabilityChallenge.challenge(challengedBlockNumber, challengedCommitment);

        // Move to a block after the resolve window
        vm.roll(block.number + dataAvailabilityChallenge.resolveWindow() + 1);

        uint256 balanceBeforeUnlock = dataAvailabilityChallenge.balances(address(this));

        // Unlock the bond associated with the challenge
        dataAvailabilityChallenge.unlockBond(challengedBlockNumber, challengedCommitment);

        // Expect the balance to be increased by the bond size
        uint256 balanceAfterUnlock = dataAvailabilityChallenge.balances(address(this));
        assertEq(balanceAfterUnlock, balanceBeforeUnlock + dataAvailabilityChallenge.bondSize());

        // Expect the bond to be unlocked
        Challenge memory challenge = dataAvailabilityChallenge.getChallenge(challengedBlockNumber, challengedCommitment);

        assertEq(challenge.challenger, address(this));
        assertEq(challenge.lockedBond, 0);
        assertEq(challenge.startBlock, challengedBlockNumber + 1);
        assertEq(challenge.resolvedBlock, 0);
        assertEq(
            uint8(dataAvailabilityChallenge.getChallengeStatus(challengedBlockNumber, challengedCommitment)),
            uint8(ChallengeStatus.Expired)
        );

        // Unlock the bond again, expect the balance to remain the same
        dataAvailabilityChallenge.unlockBond(challengedBlockNumber, challengedCommitment);
        assertEq(dataAvailabilityChallenge.balances(address(this)), balanceAfterUnlock);
    }

    function testUnlockBondFailNonExistentChallenge() public {
        bytes memory preImage = "some preimage";
        bytes memory challengedCommitment = computeCommitmentKeccak256(preImage);
        uint256 challengedBlockNumber = 1;

        // Move to block after challenged block
        vm.roll(challengedBlockNumber + 1);

        // Unlock a bond of a non-existent challenge fails
        vm.expectRevert(abi.encodeWithSelector(IDataAvailabilityChallenge.ChallengeNotExpired.selector));
        dataAvailabilityChallenge.unlockBond(challengedBlockNumber, challengedCommitment);
    }

    function testUnlockBondFailResolvedChallenge() public {
        bytes memory preImage = "some preimage";
        bytes memory challengedCommitment = computeCommitmentKeccak256(preImage);
        uint256 challengedBlockNumber = 1;

        // Move to block after challenged block
        vm.roll(challengedBlockNumber + 1);

        // Challenge the hash
        dataAvailabilityChallenge.deposit{ value: dataAvailabilityChallenge.bondSize() }();
        dataAvailabilityChallenge.challenge(challengedBlockNumber, challengedCommitment);

        // Resolve the challenge
        dataAvailabilityChallenge.resolve(challengedBlockNumber, challengedCommitment, preImage);

        // Attempting to unlock a bond of a resolved challenge fails
        vm.expectRevert(abi.encodeWithSelector(IDataAvailabilityChallenge.ChallengeNotExpired.selector));
        dataAvailabilityChallenge.unlockBond(challengedBlockNumber, challengedCommitment);
    }

    function testUnlockBondExpiredChallengeTwice() public {
        bytes memory preImage = "some preimage";
        bytes memory challengedCommitment = computeCommitmentKeccak256(preImage);
        uint256 challengedBlockNumber = 1;

        // Move to block after challenged block
        vm.roll(challengedBlockNumber + 1);

        // Challenge the hash
        dataAvailabilityChallenge.deposit{ value: dataAvailabilityChallenge.bondSize() }();
        dataAvailabilityChallenge.challenge(challengedBlockNumber, challengedCommitment);

        // Move to a block after the challenge window
        vm.roll(block.number + dataAvailabilityChallenge.resolveWindow() + 1);

        // Unlock the bond
        dataAvailabilityChallenge.unlockBond(challengedBlockNumber, challengedCommitment);

        uint256 balanceAfterUnlock = dataAvailabilityChallenge.balances(address(this));

        // Unlock the bond again doesn't change the balance
        dataAvailabilityChallenge.unlockBond(challengedBlockNumber, challengedCommitment);
        assertEq(dataAvailabilityChallenge.balances(address(this)), balanceAfterUnlock);
    }

    function testUnlockFailResolveWindowNotClosed() public {
        bytes memory preImage = "some preimage";
        bytes memory challengedCommitment = computeCommitmentKeccak256(preImage);
        uint256 challengedBlockNumber = 1;

        // Move to block after challenged block
        vm.roll(challengedBlockNumber + 1);

        // Challenge the hash
        dataAvailabilityChallenge.deposit{ value: dataAvailabilityChallenge.bondSize() }();
        dataAvailabilityChallenge.challenge(challengedBlockNumber, challengedCommitment);

        vm.roll(block.number + dataAvailabilityChallenge.resolveWindow() - 1);

        // Expiring the challenge before the resolve window closes fails
        vm.expectRevert(abi.encodeWithSelector(IDataAvailabilityChallenge.ChallengeNotExpired.selector));
        dataAvailabilityChallenge.unlockBond(challengedBlockNumber, challengedCommitment);
    }

    function testSetBondSize() public {
        uint256 requiredBond = dataAvailabilityChallenge.bondSize();
        uint256 actualBond = requiredBond - 1;
        dataAvailabilityChallenge.deposit{ value: actualBond }();

        // Expect the challenge to fail because the bond is too low
        bytes memory challengedCommitment = computeCommitmentKeccak256("some hash");
        vm.expectRevert(
            abi.encodeWithSelector(IDataAvailabilityChallenge.BondTooLow.selector, actualBond, requiredBond)
        );
        dataAvailabilityChallenge.challenge(0, challengedCommitment);

        // Reduce the required bond
        vm.prank(dataAvailabilityChallenge.owner());
        dataAvailabilityChallenge.setBondSize(actualBond);

        // Expect the challenge to succeed
        dataAvailabilityChallenge.challenge(0, challengedCommitment);
    }

    function testSetResolverRefundPercentage(uint256 resolverRefundPercentage) public {
        resolverRefundPercentage = bound(resolverRefundPercentage, 0, 100);
        vm.prank(dataAvailabilityChallenge.owner());
        dataAvailabilityChallenge.setResolverRefundPercentage(resolverRefundPercentage);
        assertEq(dataAvailabilityChallenge.resolverRefundPercentage(), resolverRefundPercentage);
    }

    function testSetResolverRefundPercentageFail() public {
        address owner = dataAvailabilityChallenge.owner();
        vm.expectRevert(
            abi.encodeWithSelector(IDataAvailabilityChallenge.InvalidResolverRefundPercentage.selector, 101)
        );
        vm.prank(owner);
        dataAvailabilityChallenge.setResolverRefundPercentage(101);
    }

    function testSetBondSizeFailOnlyOwner(address notOwner, uint256 newBondSize) public {
        vm.assume(notOwner != dataAvailabilityChallenge.owner());

        // Expect setting the bond size to fail because the sender is not the owner
        vm.prank(notOwner);
        vm.expectRevert("Ownable: caller is not the owner");
        dataAvailabilityChallenge.setBondSize(newBondSize);
    }

    function testValidateCommitment() public {
        // Should not revert given a valid commitment
        bytes memory validCommitment = abi.encodePacked(CommitmentType.Keccak256, keccak256("test"));
        dataAvailabilityChallenge.validateCommitment(validCommitment);

        // Should revert if the commitment type is unknown
        vm.expectRevert(abi.encodeWithSelector(IDataAvailabilityChallenge.UnknownCommitmentType.selector, uint8(1)));
        bytes memory unknownType = abi.encodePacked(uint8(1), keccak256("test"));
        dataAvailabilityChallenge.validateCommitment(unknownType);

        // Should revert if the commitment length does not match
        vm.expectRevert(
            abi.encodeWithSelector(IDataAvailabilityChallenge.InvalidCommitmentLength.selector, uint8(0), 33, 34)
        );
        bytes memory invalidLength = abi.encodePacked(CommitmentType.Keccak256, keccak256("test"), "x");
        dataAvailabilityChallenge.validateCommitment(invalidLength);
    }
}
