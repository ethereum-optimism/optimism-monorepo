// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

// Testing
import { CommonTest } from "test/setup/CommonTest.sol";

// Scripts
import { ForgeArtifacts } from "scripts/libraries/ForgeArtifacts.sol";

// Libraries
import "src/dispute/lib/Types.sol";
import "src/dispute/lib/Errors.sol";

// Interfaces
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { IDisputeGame } from "interfaces/dispute/IDisputeGame.sol";

contract DisputeGameFactory_Init is CommonTest {
    FakeClone fakeClone;

    event DisputeGameCreated(address indexed disputeProxy, GameType indexed gameType, Claim indexed rootClaim);
    event ImplementationSet(address indexed impl, GameType indexed gameType);
    event InitBondUpdated(GameType indexed gameType, uint256 indexed newBond);

    function setUp() public virtual override {
        super.setUp();
        fakeClone = new FakeClone();

        // Transfer ownership of the factory to the test contract.
        vm.prank(disputeGameFactory.owner());
        disputeGameFactory.transferOwnership(address(this));
    }
}

contract DisputeGameFactory_Create_Test is DisputeGameFactory_Init {
    /// @dev Tests that the `create` function succeeds when creating a new dispute game
    ///      with a `GameType` that has an implementation set.
    function testFuzz_create_succeeds(
        uint32 gameType,
        Claim rootClaim,
        bytes calldata extraData,
        uint256 _value
    )
        public
    {
        // Ensure that the `gameType` is within the bounds of the `GameType` enum's possible values.
        GameType gt = GameType.wrap(uint8(bound(gameType, 0, 2)));
        // Ensure the rootClaim has a VMStatus that disagrees with the validity.
        rootClaim = changeClaimStatus(rootClaim, VMStatuses.INVALID);

        // Set all three implementations to the same `FakeClone` contract.
        for (uint8 i; i < 3; i++) {
            GameType lgt = GameType.wrap(i);
            disputeGameFactory.setImplementation(lgt, IDisputeGame(address(fakeClone)));
            disputeGameFactory.setInitBond(lgt, _value);
        }

        vm.deal(address(this), _value);

        uint256 gameCountBefore = disputeGameFactory.gameCount();

        vm.expectEmit(false, true, true, false);
        emit DisputeGameCreated(address(0), gt, rootClaim);
        IDisputeGame proxy = disputeGameFactory.create{ value: _value }(gt, rootClaim, extraData);

        (IDisputeGame game, Timestamp timestamp) = disputeGameFactory.games(gt, rootClaim, extraData);

        // Ensure that the dispute game was assigned to the `disputeGames` mapping.
        assertEq(address(game), address(proxy));
        assertEq(Timestamp.unwrap(timestamp), block.timestamp);
        assertEq(disputeGameFactory.gameCount(), gameCountBefore + 1);

        (, Timestamp timestamp2, IDisputeGame game2) = disputeGameFactory.gameAtIndex(gameCountBefore);
        assertEq(address(game2), address(proxy));
        assertEq(Timestamp.unwrap(timestamp2), block.timestamp);

        // Ensure that the game proxy received the bonded ETH.
        assertEq(address(proxy).balance, _value);
    }

    /// @dev Tests that the `create` function reverts when creating a new dispute game with an incorrect bond amount.
    function testFuzz_create_incorrectBondAmount_reverts(
        uint32 gameType,
        Claim rootClaim,
        bytes calldata extraData
    )
        public
    {
        // Ensure that the `gameType` is within the bounds of the `GameType` enum's possible values.
        GameType gt = GameType.wrap(uint8(bound(gameType, 0, 2)));
        // Ensure the rootClaim has a VMStatus that disagrees with the validity.
        rootClaim = changeClaimStatus(rootClaim, VMStatuses.INVALID);

        // Set all three implementations to the same `FakeClone` contract.
        for (uint8 i; i < 3; i++) {
            GameType lgt = GameType.wrap(i);
            disputeGameFactory.setImplementation(lgt, IDisputeGame(address(fakeClone)));
            disputeGameFactory.setInitBond(lgt, 1 ether);
        }

        vm.expectRevert(IncorrectBondAmount.selector);
        disputeGameFactory.create(gt, rootClaim, extraData);
    }

    /// @dev Tests that the `create` function reverts when there is no implementation
    ///      set for the given `GameType`.
    function testFuzz_create_noImpl_reverts(uint32 gameType, Claim rootClaim, bytes calldata extraData) public {
        // Ensure that the `gameType` is within the bounds of the `GameType` enum's possible values. We skip over
        // game type = 0, since the deploy script set the implementation for that game type.
        GameType gt = GameType.wrap(uint32(bound(gameType, 2, type(uint32).max)));
        // Ensure the rootClaim has a VMStatus that disagrees with the validity.
        rootClaim = changeClaimStatus(rootClaim, VMStatuses.INVALID);

        vm.expectRevert(abi.encodeWithSelector(NoImplementation.selector, gt));
        disputeGameFactory.create(gt, rootClaim, extraData);
    }

    /// @dev Tests that the `create` function reverts when there exists a dispute game with the same UUID.
    function testFuzz_create_sameUUID_reverts(uint32 gameType, Claim rootClaim, bytes calldata extraData) public {
        // Ensure that the `gameType` is within the bounds of the `GameType` enum's possible values.
        GameType gt = GameType.wrap(uint8(bound(gameType, 0, 2)));
        // Ensure the rootClaim has a VMStatus that disagrees with the validity.
        rootClaim = changeClaimStatus(rootClaim, VMStatuses.INVALID);

        // Set all three implementations to the same `FakeClone` contract.
        for (uint8 i; i < 3; i++) {
            disputeGameFactory.setImplementation(GameType.wrap(i), IDisputeGame(address(fakeClone)));
        }

        uint256 bondAmount = disputeGameFactory.initBonds(gt);

        // Create our first dispute game - this should succeed.
        vm.expectEmit(false, true, true, false);
        emit DisputeGameCreated(address(0), gt, rootClaim);
        IDisputeGame proxy = disputeGameFactory.create{ value: bondAmount }(gt, rootClaim, extraData);

        (IDisputeGame game, Timestamp timestamp) = disputeGameFactory.games(gt, rootClaim, extraData);
        // Ensure that the dispute game was assigned to the `disputeGames` mapping.
        assertEq(address(game), address(proxy));
        assertEq(Timestamp.unwrap(timestamp), block.timestamp);

        // Ensure that the `create` function reverts when called with parameters that would result in the same UUID.
        vm.expectRevert(
            abi.encodeWithSelector(GameAlreadyExists.selector, disputeGameFactory.getGameUUID(gt, rootClaim, extraData))
        );
        disputeGameFactory.create{ value: bondAmount }(gt, rootClaim, extraData);
    }

    function changeClaimStatus(Claim _claim, VMStatus _status) public pure returns (Claim out_) {
        assembly {
            out_ := or(and(not(shl(248, 0xFF)), _claim), shl(248, _status))
        }
    }
}

contract DisputeGameFactory_SetImplementation_Test is DisputeGameFactory_Init {
    /// @dev Tests that the `setImplementation` function properly sets the implementation for a given `GameType`.
    function test_setImplementation_succeeds() public {
        vm.expectEmit(true, true, true, true, address(disputeGameFactory));
        emit ImplementationSet(address(1), GameTypes.CANNON);

        // Set the implementation for the `GameTypes.CANNON` enum value.
        disputeGameFactory.setImplementation(GameTypes.CANNON, IDisputeGame(address(1)));

        // Ensure that the implementation for the `GameTypes.CANNON` enum value is set.
        assertEq(address(disputeGameFactory.gameImpls(GameTypes.CANNON)), address(1));
    }

    /// @dev Tests that the `setImplementation` function reverts when called by a non-owner.
    function test_setImplementation_notOwner_reverts() public {
        // Ensure that the `setImplementation` function reverts when called by a non-owner.
        vm.prank(address(0));
        vm.expectRevert("Ownable: caller is not the owner");
        disputeGameFactory.setImplementation(GameTypes.CANNON, IDisputeGame(address(1)));
    }
}

contract DisputeGameFactory_SetInitBond_Test is DisputeGameFactory_Init {
    /// @dev Tests that the `setInitBond` function properly sets the init bond for a given `GameType`.
    function test_setInitBond_succeeds() public {
        // There should be no init bond for the `GameTypes.CANNON` enum value, it has not been set.
        if (!isForkTest()) {
            assertEq(disputeGameFactory.initBonds(GameTypes.CANNON), 0);
        }

        vm.expectEmit(true, true, true, true, address(disputeGameFactory));
        emit InitBondUpdated(GameTypes.CANNON, 1 ether);

        // Set the init bond for the `GameTypes.CANNON` enum value.
        disputeGameFactory.setInitBond(GameTypes.CANNON, 1 ether);

        // Ensure that the init bond for the `GameTypes.CANNON` enum value is set.
        assertEq(disputeGameFactory.initBonds(GameTypes.CANNON), 1 ether);
    }

    /// @dev Tests that the `setInitBond` function reverts when called by a non-owner.
    function test_setInitBond_notOwner_reverts() public {
        // Ensure that the `setInitBond` function reverts when called by a non-owner.
        vm.prank(address(0));
        vm.expectRevert("Ownable: caller is not the owner");
        disputeGameFactory.setInitBond(GameTypes.CANNON, 1 ether);
    }
}

contract DisputeGameFactory_GetGameUUID_Test is DisputeGameFactory_Init {
    /// @dev Tests that the `getGameUUID` function returns the correct hash when comparing
    ///      against the keccak256 hash of the abi-encoded parameters.
    function testDiff_getGameUUID_succeeds(uint32 gameType, Claim rootClaim, bytes calldata extraData) public view {
        // Ensure that the `gameType` is within the bounds of the `GameType` enum's possible values.
        GameType gt = GameType.wrap(uint8(bound(gameType, 0, 2)));

        assertEq(
            Hash.unwrap(disputeGameFactory.getGameUUID(gt, rootClaim, extraData)),
            keccak256(abi.encode(gt, rootClaim, extraData))
        );
    }
}

contract DisputeGameFactory_Owner_Test is DisputeGameFactory_Init {
    /// @dev Tests that the `owner` function returns the correct address after deployment.
    function test_owner_succeeds() public view {
        assertEq(disputeGameFactory.owner(), address(this));
    }
}

contract DisputeGameFactory_TransferOwnership_Test is DisputeGameFactory_Init {
    /// @dev Tests that the `transferOwnership` function succeeds when called by the owner.
    function test_transferOwnership_succeeds() public {
        disputeGameFactory.transferOwnership(address(1));
        assertEq(disputeGameFactory.owner(), address(1));
    }

    /// @dev Tests that the `transferOwnership` function reverts when called by a non-owner.
    function test_transferOwnership_notOwner_reverts() public {
        vm.prank(address(0));
        vm.expectRevert("Ownable: caller is not the owner");
        disputeGameFactory.transferOwnership(address(1));
    }
}

contract DisputeGameFactory_FindLatestGames_Test is DisputeGameFactory_Init {
    function setUp() public override {
        super.setUp();

        // Set three implementations to the same `FakeClone` contract.
        for (uint8 i; i < 3; i++) {
            GameType lgt = GameType.wrap(i);
            disputeGameFactory.setImplementation(lgt, IDisputeGame(address(fakeClone)));
            disputeGameFactory.setInitBond(lgt, 0);
        }
    }

    /// @dev Tests that `findLatestGames` returns an empty array when the passed starting index is greater than or equal
    ///      to the game count.
    function testFuzz_findLatestGames_greaterThanLength_succeeds(uint256 _start) public {
        // Creation count should be 32 for normal tests, 5 for upgrade tests.
        uint256 creationCount = isForkTest() ? 5 : 32;

        // Create some dispute games of varying game types.
        for (uint256 i; i < creationCount; i++) {
            disputeGameFactory.create(GameType.wrap(uint8(i % 2)), Claim.wrap(bytes32(i)), abi.encode(i));
        }

        // Bound the starting index to a number greater than the length of the game list.
        uint256 gameCount = disputeGameFactory.gameCount();
        _start = bound(_start, gameCount, type(uint256).max);

        // The array's length should always be 0.
        IDisputeGameFactory.GameSearchResult[] memory games =
            disputeGameFactory.findLatestGames(GameTypes.CANNON, _start, 1);
        assertEq(games.length, 0);
    }

    /// @dev Tests that `findLatestGames` returns the correct games.
    function test_findLatestGames_static_succeeds() public {
        // Creation count should be 32 for normal tests, 5 for upgrade tests.
        uint256 creationCount = isForkTest() ? 5 : 32;

        // Create some dispute games of varying game types, repeatedly iterating over the game types 0, 1, 2.
        for (uint256 i; i < creationCount; i++) {
            disputeGameFactory.create(GameType.wrap(uint8(i % 3)), Claim.wrap(bytes32(i)), abi.encode(i));
        }

        uint256 gameCount = disputeGameFactory.gameCount();

        IDisputeGameFactory.GameSearchResult[] memory games;

        uint256 start = gameCount - 1;

        // Find type 1 games.
        games = disputeGameFactory.findLatestGames(GameType.wrap(1), start, 1);
        assertEq(games.length, 1);

        // The type 1 game should be the last one added.
        assertEq(games[0].index, start);
        (GameType gameType, Timestamp createdAt, address game) = games[0].metadata.unpack();
        assertEq(gameType.raw(), 1);
        assertEq(createdAt.raw(), block.timestamp);

        // Find type 0 games.
        games = disputeGameFactory.findLatestGames(GameType.wrap(0), start, 1);
        assertEq(games.length, 1);

        // The type 0 game should be the second to last one added.
        assertEq(games[0].index, start - 1);
        (gameType, createdAt, game) = games[0].metadata.unpack();
        assertEq(gameType.raw(), 0);
        assertEq(createdAt.raw(), block.timestamp);

        // Find type 2 games.
        games = disputeGameFactory.findLatestGames(GameType.wrap(2), start, 1);
        assertEq(games.length, 1);

        // The type 2 game should be the third to last one added.
        assertEq(games[0].index, start - 2);
        (gameType, createdAt, game) = games[0].metadata.unpack();
        assertEq(gameType.raw(), 2);
        assertEq(createdAt.raw(), block.timestamp);
    }

    /// @dev Tests that `findLatestGames` returns the correct games, if there are less than `_n` games of the given type
    ///      available.
    function test_findLatestGames_lessThanNAvailable_succeeds() public {
        // Need to clear out the length of the game list on forked list to avoid massive iteration.
        if (isForkTest()) {
            vm.store(
                address(disputeGameFactory),
                bytes32(ForgeArtifacts.getSlot("DisputeGameFactory", "_disputeGameList").slot),
                bytes32(0)
            );
        }

        // Create some dispute games of varying game types.
        disputeGameFactory.create(GameType.wrap(1), Claim.wrap(bytes32(0)), abi.encode(0));
        disputeGameFactory.create(GameType.wrap(1), Claim.wrap(bytes32(uint256(1))), abi.encode(1));
        for (uint256 i; i < 1 << 3; i++) {
            disputeGameFactory.create(GameType.wrap(0), Claim.wrap(bytes32(i)), abi.encode(i));
        }

        // Grab the existing game count.
        uint256 gameCount = disputeGameFactory.gameCount();

        // Try to find 5 games of type 2, but there are none.
        IDisputeGameFactory.GameSearchResult[] memory games;
        games = disputeGameFactory.findLatestGames(GameType.wrap(2), gameCount - 1, 5);
        assertEq(games.length, 0);

        // Try to find 2 games of type 1, but there are only 2.
        games = disputeGameFactory.findLatestGames(GameType.wrap(1), gameCount - 1, 5);
        assertEq(games.length, 2);
        assertEq(games[0].index, 1);
        assertEq(games[1].index, 0);
    }

    /// @dev Tests that the expected number of games are returned when `findLatestGames` is called.
    function testFuzz_findLatestGames_correctAmount_succeeds(
        uint256 _numGames,
        uint256 _numSearchedGames,
        uint256 _n
    )
        public
    {
        _numGames = bound(_numGames, 0, isForkTest() ? 5 : 256);
        _numSearchedGames = bound(_numSearchedGames, 0, _numGames);
        _n = bound(_n, 0, _numSearchedGames);

        // Create `_numGames` dispute games, with at least `_numSearchedGames` games.
        for (uint256 i; i < _numGames; i++) {
            uint32 gameType = i < _numSearchedGames ? 0 : 1;
            disputeGameFactory.create(GameType.wrap(gameType), Claim.wrap(bytes32(i)), abi.encode(i));
        }

        // Ensure that the correct number of games are returned.
        uint256 start = _numGames == 0 ? 0 : _numGames - 1;
        IDisputeGameFactory.GameSearchResult[] memory games =
            disputeGameFactory.findLatestGames(GameType.wrap(0), start, _n);
        assertEq(games.length, _n);
    }
}

/// @dev A fake clone used for testing the `DisputeGameFactory` contract's `create` function.
contract FakeClone {
    function initialize() external payable {
        // noop
    }

    function extraData() external pure returns (bytes memory) {
        return hex"FF0420";
    }

    function parentHash() external pure returns (bytes32) {
        return bytes32(0);
    }

    function rootClaim() external pure returns (Claim) {
        return Claim.wrap(bytes32(0));
    }
}
