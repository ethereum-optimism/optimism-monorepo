// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

// Testing
import { FaultDisputeGame_Init, _changeClaimStatus } from "test/dispute/FaultDisputeGame.t.sol";

// Libraries
import { GameType, GameStatus, Hash, Claim, VMStatuses, OutputRoot } from "src/dispute/lib/Types.sol";

// Interfaces
import { IFaultDisputeGame } from "interfaces/dispute/IFaultDisputeGame.sol";
import { IAnchorStateRegistry } from "interfaces/dispute/IAnchorStateRegistry.sol";

contract AnchorStateRegistry_Init is FaultDisputeGame_Init {
    event AnchorNotUpdated(IFaultDisputeGame indexed game);
    event AnchorUpdated(IFaultDisputeGame indexed game);

    function setUp() public virtual override {
        // Duplicating the initialization/setup logic of FaultDisputeGame_Test.
        // See that test for more information, actual values here not really important.
        Claim rootClaim = Claim.wrap(bytes32((uint256(1) << 248) | uint256(10)));
        bytes memory absolutePrestateData = abi.encode(0);
        Claim absolutePrestate = _changeClaimStatus(Claim.wrap(keccak256(absolutePrestateData)), VMStatuses.UNFINISHED);

        super.setUp();
        super.init({ rootClaim: rootClaim, absolutePrestate: absolutePrestate, l2BlockNumber: 0x10 });
    }
}

contract AnchorStateRegistry_Initialize_Test is AnchorStateRegistry_Init {
    /// @dev Tests that initialization is successful.
    function test_initialize_succeeds() public view {
        // Verify starting anchor root.
        (Hash root, uint256 l2BlockNumber) = anchorStateRegistry.getAnchorRoot();
        assertEq(root.raw(), 0xDEADBEEFDEADBEEFDEADBEEFDEADBEEFDEADBEEFDEADBEEFDEADBEEFDEADBEEF);
        assertEq(l2BlockNumber, 0);

        // Verify contract addresses.
        assert(anchorStateRegistry.superchainConfig() == superchainConfig);
        assert(anchorStateRegistry.disputeGameFactory() == disputeGameFactory);
        assert(anchorStateRegistry.portal() == optimismPortal2);
    }
}

contract AnchorStateRegistry_Initialize_TestFail is AnchorStateRegistry_Init {
    /// @notice Tests that initialization cannot be done twice
    function test_initialize_twice_reverts() public {
        vm.expectRevert("Initializable: contract is already initialized");
        anchorStateRegistry.initialize(
            superchainConfig,
            disputeGameFactory,
            optimismPortal2,
            OutputRoot({
                root: Hash.wrap(0xDEADBEEFDEADBEEFDEADBEEFDEADBEEFDEADBEEFDEADBEEFDEADBEEFDEADBEEF),
                l2BlockNumber: 0
            })
        );
    }
}

contract AnchorStateRegistry_Version_Test is AnchorStateRegistry_Init {
    /// @notice Tests that the version function returns a string.
    function test_version_succeeds() public view {
        assert(bytes(anchorStateRegistry.version()).length > 0);
    }
}

contract AnchorStateRegistry_GetAnchorRoot_Test is AnchorStateRegistry_Init {
    /// @notice Tests that getAnchorRoot will return the value of the starting anchor root when no
    ///         anchor game exists yet.
    function test_getAnchorRoot_noAnchorGame_succeeds() public view {
        // Assert that we nave no anchor game yet.
        assert(address(anchorStateRegistry.anchorGame()) == address(0));

        // We should get the starting anchor root back.
        (Hash root, uint256 l2BlockNumber) = anchorStateRegistry.getAnchorRoot();
        assertEq(root.raw(), 0xDEADBEEFDEADBEEFDEADBEEFDEADBEEFDEADBEEFDEADBEEFDEADBEEFDEADBEEF);
        assertEq(l2BlockNumber, 0);
    }

    /// @notice Tests that getAnchorRoot will return the correct anchor root if an anchor game exists.
    function test_getAnchorRoot_anchorGameExists_succeeds() public {
        // Mock the game to be resolved.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.resolvedAt, ()), abi.encode(block.timestamp));
        vm.warp(block.timestamp + optimismPortal2.disputeGameFinalityDelaySeconds() + 1);

        // Mock the game to be the defender wins.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.status, ()), abi.encode(GameStatus.DEFENDER_WINS));

        // Set the anchor game to the game proxy.
        anchorStateRegistry.setAnchorState(gameProxy);

        // We should get the anchor root back.
        (Hash root, uint256 l2BlockNumber) = anchorStateRegistry.getAnchorRoot();
        assertEq(root.raw(), gameProxy.rootClaim().raw());
        assertEq(l2BlockNumber, gameProxy.l2BlockNumber());
    }
}

contract AnchorStateRegistry_GetAnchorRoot_TestFail is AnchorStateRegistry_Init {
    /// @notice Tests that getAnchorRoot will revert if the anchor game is blacklisted.
    function test_getAnchorRoot_blacklistedGame_fails() public {
        // Mock the game to be resolved.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.resolvedAt, ()), abi.encode(block.timestamp));
        vm.warp(block.timestamp + optimismPortal2.disputeGameFinalityDelaySeconds() + 1);

        // Mock the game to be the defender wins.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.status, ()), abi.encode(GameStatus.DEFENDER_WINS));

        // Set the anchor game to the game proxy.
        anchorStateRegistry.setAnchorState(gameProxy);

        // Mock the disputeGameBlacklist call to return true.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.disputeGameBlacklist, (gameProxy)),
            abi.encode(true)
        );
        vm.expectRevert(IAnchorStateRegistry.AnchorStateRegistry_AnchorGameBlacklisted.selector);
        anchorStateRegistry.getAnchorRoot();
    }
}

contract AnchorStateRegistry_Anchors_Test is AnchorStateRegistry_Init {
    /// @notice Tests that the anchors() function always matches the result of the getAnchorRoot()
    ///         function regardless of the game type used.
    /// @param _gameType Game type to use as input to anchors().
    function testFuzz_anchors_matchesGetAnchorRoot_succeeds(GameType _gameType) public view {
        // Get the anchor root according to getAnchorRoot().
        (Hash root1, uint256 l2BlockNumber1) = anchorStateRegistry.getAnchorRoot();

        // Get the anchor root according to anchors().
        (Hash root2, uint256 l2BlockNumber2) = anchorStateRegistry.anchors(_gameType);

        // Assert that the two roots are the same.
        assertEq(root1.raw(), root2.raw());
        assertEq(l2BlockNumber1, l2BlockNumber2);
    }
}

contract AnchorStateRegistry_IsGameRegistered_Test is AnchorStateRegistry_Init {
    /// @notice Tests that isGameRegistered will return true if the game is registered.
    function test_isGameRegistered_isActuallyFactoryRegistered_succeeds() public view {
        assertTrue(anchorStateRegistry.isGameRegistered(gameProxy));
    }

    /// @notice Tests that isGameRegistered will return false if the game is not registered.
    function test_isGameRegistered_isNotFactoryRegistered_succeeds() public {
        // Mock the DisputeGameFactory to make it seem that the game was not registered.
        vm.mockCall(
            address(disputeGameFactory),
            abi.encodeCall(
                disputeGameFactory.games, (gameProxy.gameType(), gameProxy.rootClaim(), gameProxy.extraData())
            ),
            abi.encode(address(0), 0)
        );
        assertFalse(anchorStateRegistry.isGameRegistered(gameProxy));
    }
}

contract AnchorStateRegistry_IsGameBlacklisted_Test is AnchorStateRegistry_Init {
    /// @notice Tests that isGameBlacklisted will return true if the game is blacklisted.
    function test_isGameBlacklisted_isActuallyBlacklisted_succeeds() public {
        // Mock the disputeGameBlacklist call to return true.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.disputeGameBlacklist, (gameProxy)),
            abi.encode(true)
        );
        assertTrue(anchorStateRegistry.isGameBlacklisted(gameProxy));
    }

    /// @notice Tests that isGameBlacklisted will return false if the game is not blacklisted.
    function test_isGameBlacklisted_isNotBlacklisted_succeeds() public {
        // Mock the disputeGameBlacklist call to return false.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.disputeGameBlacklist, (gameProxy)),
            abi.encode(false)
        );
        assertFalse(anchorStateRegistry.isGameBlacklisted(gameProxy));
    }
}

contract AnchorStateRegistry_IsGameRespected_Test is AnchorStateRegistry_Init {
    /// @notice Tests that isGameRespected will return true if the game is of the respected game type.
    function test_isGameRespected_isRespected_succeeds() public {
        // Mock that the game was respected.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.wasRespectedGameTypeWhenCreated, ()), abi.encode(true));
        assertTrue(anchorStateRegistry.isGameRespected(gameProxy));
    }

    /// @notice Tests that isGameRespected will return false if the game is not of the respected game
    ///         type.
    function test_isGameRespected_isNotRespected_succeeds() public {
        // Mock that the game was not respected.
        vm.mockCall(
            address(gameProxy), abi.encodeCall(gameProxy.wasRespectedGameTypeWhenCreated, ()), abi.encode(false)
        );
        assertFalse(anchorStateRegistry.isGameRespected(gameProxy));
    }
}

contract AnchorStateRegistry_IsGameRetired_Test is AnchorStateRegistry_Init {
    /// @notice Tests that isGameRetired will return true if the game is retired.
    /// @param _retirementTimestamp The retirement timestamp to use for the test.
    function testFuzz_isGameRetired_isRetired_succeeds(uint64 _retirementTimestamp) public {
        // Make sure retirement timestamp is greater than or equal to the game's creation time.
        _retirementTimestamp = uint64(bound(_retirementTimestamp, gameProxy.createdAt().raw(), type(uint64).max));

        // Mock the respectedGameTypeUpdatedAt call.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.respectedGameTypeUpdatedAt, ()),
            abi.encode(_retirementTimestamp)
        );

        // Game should be retired.
        assertTrue(anchorStateRegistry.isGameRetired(gameProxy));
    }

    /// @notice Tests that isGameRetired will return false if the game is not retired.
    /// @param _retirementTimestamp The retirement timestamp to use for the test.
    function testFuzz_isGameRetired_isNotRetired_succeeds(uint64 _retirementTimestamp) public {
        // Make sure retirement timestamp is earlier than the game's creation time.
        _retirementTimestamp = uint64(bound(_retirementTimestamp, 0, gameProxy.createdAt().raw() - 1));

        // Mock the respectedGameTypeUpdatedAt call to be earlier than the game's creation time.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.respectedGameTypeUpdatedAt, ()),
            abi.encode(_retirementTimestamp)
        );

        // Game should not be retired.
        assertFalse(anchorStateRegistry.isGameRetired(gameProxy));
    }
}

contract AnchorStateRegistry_IsGameProper_Test is AnchorStateRegistry_Init {
    /// @notice Tests that isGameProper will return true if the game meets all conditions.
    function test_isGameProper_meetsAllConditions_succeeds() public view {
        // Game will meet all conditions by default.
        assertTrue(anchorStateRegistry.isGameProper(gameProxy));
    }

    /// @notice Tests that isGameProper will return false if the game is not registered.
    function test_isGameProper_isNotFactoryRegistered_succeeds() public {
        // Mock the DisputeGameFactory to make it seem that the game was not registered.
        vm.mockCall(
            address(disputeGameFactory),
            abi.encodeCall(
                disputeGameFactory.games, (gameProxy.gameType(), gameProxy.rootClaim(), gameProxy.extraData())
            ),
            abi.encode(address(0), 0)
        );

        assertFalse(anchorStateRegistry.isGameProper(gameProxy));
    }

    /// @notice Tests that isGameProper will return false if the game is not the respected game type.
    /// @param _gameType The game type to use for the test.
    function testFuzz_isGameProper_anyGameType_succeeds(GameType _gameType) public {
        if (_gameType.raw() == gameProxy.gameType().raw()) {
            _gameType = GameType.wrap(_gameType.raw() + 1);
        }

        // Mock that the game was not respected.
        vm.mockCall(
            address(gameProxy), abi.encodeCall(gameProxy.wasRespectedGameTypeWhenCreated, ()), abi.encode(false)
        );

        // Still a proper game.
        assertTrue(anchorStateRegistry.isGameProper(gameProxy));
    }

    /// @notice Tests that isGameProper will return false if the game is blacklisted.
    function test_isGameProper_isBlacklisted_succeeds() public {
        // Mock the disputeGameBlacklist call to return true.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.disputeGameBlacklist, (gameProxy)),
            abi.encode(true)
        );

        assertFalse(anchorStateRegistry.isGameProper(gameProxy));
    }

    /// @notice Tests that isGameProper will return false if the game is retired.
    /// @param _retirementTimestamp The retirement timestamp to use for the test.
    function testFuzz_isGameProper_isRetired_succeeds(uint64 _retirementTimestamp) public {
        // Make sure retirement timestamp is later than the game's creation time.
        _retirementTimestamp = uint64(bound(_retirementTimestamp, gameProxy.createdAt().raw() + 1, type(uint64).max));

        // Mock the respectedGameTypeUpdatedAt call to be later than the game's creation time.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.respectedGameTypeUpdatedAt, ()),
            abi.encode(_retirementTimestamp)
        );

        assertFalse(anchorStateRegistry.isGameProper(gameProxy));
    }
}

contract AnchorStateRegistry_IsGameResolved_Test is AnchorStateRegistry_Init {
    /// @notice Tests that isGameResolved will return true if the game is resolved.
    /// @param _resolvedAtTimestamp The resolvedAt timestamp to use for the test.
    function testFuzz_isGameResolved_challengerWins_succeeds(uint256 _resolvedAtTimestamp) public {
        // Bound resolvedAt to be less than or equal to current timestamp.
        _resolvedAtTimestamp = bound(_resolvedAtTimestamp, 1, block.timestamp);

        // Mock the resolvedAt timestamp.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.resolvedAt, ()), abi.encode(_resolvedAtTimestamp));

        // Mock the status to be CHALLENGER_WINS.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.status, ()), abi.encode(GameStatus.CHALLENGER_WINS));

        // Game should be resolved.
        assertTrue(anchorStateRegistry.isGameResolved(gameProxy));
    }

    /// @notice Tests that isGameResolved will return true if the game is resolved.
    /// @param _resolvedAtTimestamp The resolvedAt timestamp to use for the test.
    function testFuzz_isGameResolved_defenderWins_succeeds(uint256 _resolvedAtTimestamp) public {
        // Bound resolvedAt to be less than or equal to current timestamp.
        _resolvedAtTimestamp = bound(_resolvedAtTimestamp, 1, block.timestamp);

        // Mock the resolvedAt timestamp.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.resolvedAt, ()), abi.encode(_resolvedAtTimestamp));

        // Mock the status to be DEFENDER_WINS.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.status, ()), abi.encode(GameStatus.DEFENDER_WINS));

        // Game should be resolved.
        assertTrue(anchorStateRegistry.isGameResolved(gameProxy));
    }

    /// @notice Tests that isGameResolved will return false if the game is in progress and not resolved.
    /// @param _resolvedAtTimestamp The resolvedAt timestamp to use for the test.
    function testFuzz_isGameResolved_inProgressNotResolved_succeeds(uint256 _resolvedAtTimestamp) public {
        // Bound resolvedAt to be less than or equal to current timestamp.
        _resolvedAtTimestamp = bound(_resolvedAtTimestamp, 1, block.timestamp);

        // Mock the resolvedAt timestamp.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.resolvedAt, ()), abi.encode(_resolvedAtTimestamp));

        // Mock the status to be IN_PROGRESS.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.status, ()), abi.encode(GameStatus.IN_PROGRESS));

        // Game should not be resolved.
        assertFalse(anchorStateRegistry.isGameResolved(gameProxy));
    }
}

contract AnchorStateRegistry_IsGameAirgapped_TestFail is AnchorStateRegistry_Init {
    /// @notice Tests that isGameAirgapped will return true if the game is airgapped.
    /// @param _resolvedAtTimestamp The resolvedAt timestamp to use for the test.
    function testFuzz_isGameAirgapped_isAirgapped_succeeds(uint256 _resolvedAtTimestamp) public {
        // Warp forward by disputeGameFinalityDelaySeconds.
        vm.warp(block.timestamp + optimismPortal2.disputeGameFinalityDelaySeconds());

        // Bound resolvedAt to be at least disputeGameFinalityDelaySeconds in the past.
        _resolvedAtTimestamp =
            bound(_resolvedAtTimestamp, 0, block.timestamp - optimismPortal2.disputeGameFinalityDelaySeconds() - 1);

        // Mock the resolvedAt timestamp.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.resolvedAt, ()), abi.encode(_resolvedAtTimestamp));

        // Game should be airgapped.
        assertTrue(anchorStateRegistry.isGameAirgapped(gameProxy));
    }

    /// @notice Tests that isGameAirgapped will return false if the game is not airgapped.
    /// @param _resolvedAtTimestamp The resolvedAt timestamp to use for the test.
    function testFuzz_isGameAirgapped_isNotAirgapped_succeeds(uint256 _resolvedAtTimestamp) public {
        // Warp forward by disputeGameFinalityDelaySeconds.
        vm.warp(block.timestamp + optimismPortal2.disputeGameFinalityDelaySeconds());

        // Bound resolvedAt to be less than disputeGameFinalityDelaySeconds in the past.
        _resolvedAtTimestamp = bound(
            _resolvedAtTimestamp, block.timestamp - optimismPortal2.disputeGameFinalityDelaySeconds(), block.timestamp
        );

        // Mock the resolvedAt timestamp.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.resolvedAt, ()), abi.encode(_resolvedAtTimestamp));

        // Game should not be airgapped.
        assertFalse(anchorStateRegistry.isGameAirgapped(gameProxy));
    }
}

contract AnchorStateRegistry_IsGameClaimValid_Test is AnchorStateRegistry_Init {
    /// @notice Tests that isGameClaimValid will return true if the game claim is valid.
    /// @param _resolvedAtTimestamp The resolvedAt timestamp to use for the test.
    function testFuzz_isGameClaimValid_claimIsValid_succeeds(uint256 _resolvedAtTimestamp) public {
        // Warp forward by disputeGameFinalityDelaySeconds.
        vm.warp(block.timestamp + optimismPortal2.disputeGameFinalityDelaySeconds());

        // Bound resolvedAt to be at least disputeGameFinalityDelaySeconds in the past.
        _resolvedAtTimestamp =
            bound(_resolvedAtTimestamp, 1, block.timestamp - optimismPortal2.disputeGameFinalityDelaySeconds() - 1);

        // Mock that the game was respected.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.wasRespectedGameTypeWhenCreated, ()), abi.encode(true));

        // Mock the resolvedAt timestamp.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.resolvedAt, ()), abi.encode(_resolvedAtTimestamp));

        // Mock the status to be DEFENDER_WINS.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.status, ()), abi.encode(GameStatus.DEFENDER_WINS));

        // Claim should be valid.
        assertTrue(anchorStateRegistry.isGameClaimValid(gameProxy));
    }

    /// @notice Tests that isGameClaimValid will return false if the game is not registered.
    function testFuzz_isGameClaimValid_notRegistered_succeeds() public {
        // Mock the DisputeGameFactory to make it seem that the game was not registered.
        vm.mockCall(
            address(disputeGameFactory),
            abi.encodeCall(
                disputeGameFactory.games, (gameProxy.gameType(), gameProxy.rootClaim(), gameProxy.extraData())
            ),
            abi.encode(address(0), 0)
        );

        // Claim should not be valid.
        assertFalse(anchorStateRegistry.isGameClaimValid(gameProxy));
    }

    /// @notice Tests that isGameClaimValid will return false if the game is not respected.
    /// @param _gameType The game type to use for the test.
    function testFuzz_isGameClaimValid_isNotRespected_succeeds(GameType _gameType) public {
        if (_gameType.raw() == gameProxy.gameType().raw()) {
            _gameType = GameType.wrap(_gameType.raw() + 1);
        }

        // Mock that the game was not respected.
        vm.mockCall(
            address(gameProxy), abi.encodeCall(gameProxy.wasRespectedGameTypeWhenCreated, ()), abi.encode(false)
        );

        // Claim should not be valid.
        assertFalse(anchorStateRegistry.isGameClaimValid(gameProxy));
    }

    /// @notice Tests that isGameClaimValid will return false if the game is blacklisted.
    function testFuzz_isGameClaimValid_isBlacklisted_succeeds() public {
        // Mock the disputeGameBlacklist call to return true.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.disputeGameBlacklist, (gameProxy)),
            abi.encode(true)
        );

        // Claim should not be valid.
        assertFalse(anchorStateRegistry.isGameClaimValid(gameProxy));
    }

    /// @notice Tests that isGameClaimValid will return false if the game is retired.
    /// @param _resolvedAtTimestamp The resolvedAt timestamp to use for the test.
    function testFuzz_isGameClaimValid_isRetired_succeeds(uint256 _resolvedAtTimestamp) public {
        // Make sure retirement timestamp is later than the game's creation time.
        _resolvedAtTimestamp = uint64(bound(_resolvedAtTimestamp, gameProxy.createdAt().raw() + 1, type(uint64).max));

        // Mock the respectedGameTypeUpdatedAt call to be later than the game's creation time.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.respectedGameTypeUpdatedAt, ()),
            abi.encode(_resolvedAtTimestamp)
        );

        // Claim should not be valid.
        assertFalse(anchorStateRegistry.isGameClaimValid(gameProxy));
    }

    /// @notice Tests that isGameClaimValid will return false if the game is not resolved.
    function testFuzz_isGameClaimValid_notResolved_succeeds() public {
        // Mock the status to be IN_PROGRESS.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.status, ()), abi.encode(GameStatus.IN_PROGRESS));

        // Claim should not be valid.
        assertFalse(anchorStateRegistry.isGameClaimValid(gameProxy));
    }

    /// @notice Tests that isGameClaimValid will return false if the game is not airgapped.
    /// @param _resolvedAtTimestamp The resolvedAt timestamp to use for the test.
    function testFuzz_isGameClaimValid_notAirgapped_succeeds(uint256 _resolvedAtTimestamp) public {
        // Warp forward by disputeGameFinalityDelaySeconds.
        vm.warp(block.timestamp + optimismPortal2.disputeGameFinalityDelaySeconds());

        // Bound resolvedAt to be less than disputeGameFinalityDelaySeconds in the past.
        _resolvedAtTimestamp = bound(
            _resolvedAtTimestamp, block.timestamp - optimismPortal2.disputeGameFinalityDelaySeconds(), block.timestamp
        );

        // Mock the resolvedAt timestamp.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.resolvedAt, ()), abi.encode(_resolvedAtTimestamp));

        // Claim should not be valid.
        assertFalse(anchorStateRegistry.isGameClaimValid(gameProxy));
    }
}

contract AnchorStateRegistry_SetAnchorState_Test is AnchorStateRegistry_Init {
    /// @notice Tests that setAnchorState will succeed if the game is valid, the game block
    ///         number is greater than the current anchor root block number, and the game is the
    ///         currently respected game type.
    /// @param _l2BlockNumber The L2 block number to use for the game.
    function testFuzz_setAnchorState_validNewerState_succeeds(uint256 _l2BlockNumber) public {
        // Grab block number of the existing anchor root.
        (Hash root, uint256 l2BlockNumber) = anchorStateRegistry.getAnchorRoot();

        // Bound the new block number.
        _l2BlockNumber = bound(_l2BlockNumber, l2BlockNumber + 1, type(uint256).max);

        // Mock the l2BlockNumber call.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.l2BlockNumber, ()), abi.encode(_l2BlockNumber));

        // Mock the DEFENDER_WINS state.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.status, ()), abi.encode(GameStatus.DEFENDER_WINS));

        // Mock that the game was respected.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.wasRespectedGameTypeWhenCreated, ()), abi.encode(true));

        // Mock the resolvedAt timestamp and fast forward to beyond the delay.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.resolvedAt, ()), abi.encode(block.timestamp));
        vm.warp(block.timestamp + optimismPortal2.disputeGameFinalityDelaySeconds() + 1);

        // Update the anchor state.
        vm.prank(address(gameProxy));
        vm.expectEmit(address(anchorStateRegistry));
        emit AnchorUpdated(gameProxy);
        anchorStateRegistry.setAnchorState(gameProxy);

        // Confirm that the anchor state is now the same as the game state.
        (root, l2BlockNumber) = anchorStateRegistry.getAnchorRoot();
        assertEq(l2BlockNumber, gameProxy.l2BlockNumber());
        assertEq(root.raw(), gameProxy.rootClaim().raw());

        // Confirm that the anchor game is now set.
        IFaultDisputeGame anchorGame = anchorStateRegistry.anchorGame();
        assertEq(address(anchorGame), address(gameProxy));
    }
}

contract AnchorStateRegistry_SetAnchorState_TestFail is AnchorStateRegistry_Init {
    /// @notice Tests that setAnchorState will revert if the game is valid and the game block
    ///         number is less than or equal to the current anchor root block number.
    /// @param _l2BlockNumber The L2 block number to use for the game.
    function testFuzz_setAnchorState_olderValidGameClaim_fails(uint256 _l2BlockNumber) public {
        // Grab block number of the existing anchor root.
        (Hash root, uint256 l2BlockNumber) = anchorStateRegistry.getAnchorRoot();

        // Bound the new block number.
        _l2BlockNumber = bound(_l2BlockNumber, 0, l2BlockNumber);

        // Mock the l2BlockNumber call.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.l2BlockNumber, ()), abi.encode(_l2BlockNumber));

        // Mock the DEFENDER_WINS state.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.status, ()), abi.encode(GameStatus.DEFENDER_WINS));

        // Mock that the game was respected.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.wasRespectedGameTypeWhenCreated, ()), abi.encode(true));

        // Mock the resolvedAt timestamp and fast forward to beyond the delay.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.resolvedAt, ()), abi.encode(block.timestamp));
        vm.warp(block.timestamp + optimismPortal2.disputeGameFinalityDelaySeconds() + 1);

        // Try to update the anchor state.
        vm.prank(address(gameProxy));
        vm.expectRevert(IAnchorStateRegistry.AnchorStateRegistry_InvalidAnchorGame.selector);
        anchorStateRegistry.setAnchorState(gameProxy);

        // Confirm that the anchor state has not updated.
        (Hash updatedRoot, uint256 updatedL2BlockNumber) = anchorStateRegistry.getAnchorRoot();
        assertEq(updatedL2BlockNumber, l2BlockNumber);
        assertEq(updatedRoot.raw(), root.raw());
    }

    /// @notice Tests that setAnchorState will revert if the game is not registered.
    /// @param _l2BlockNumber The L2 block number to use for the game.
    function testFuzz_setAnchorState_notFactoryRegisteredGame_fails(uint256 _l2BlockNumber) public {
        // Grab block number of the existing anchor root.
        (Hash root, uint256 l2BlockNumber) = anchorStateRegistry.getAnchorRoot();

        // Bound the new block number.
        _l2BlockNumber = bound(_l2BlockNumber, l2BlockNumber, type(uint256).max);

        // Mock the l2BlockNumber call.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.l2BlockNumber, ()), abi.encode(_l2BlockNumber));

        // Mock the DEFENDER_WINS state.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.status, ()), abi.encode(GameStatus.DEFENDER_WINS));

        // Mock that the game was respected.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.wasRespectedGameTypeWhenCreated, ()), abi.encode(true));

        // Mock the DisputeGameFactory to make it seem that the game was not registered.
        vm.mockCall(
            address(disputeGameFactory),
            abi.encodeCall(
                disputeGameFactory.games, (gameProxy.gameType(), gameProxy.rootClaim(), gameProxy.extraData())
            ),
            abi.encode(address(0), 0)
        );

        // Try to update the anchor state.
        vm.prank(superchainConfig.guardian());
        vm.expectRevert(IAnchorStateRegistry.AnchorStateRegistry_InvalidAnchorGame.selector);
        anchorStateRegistry.setAnchorState(gameProxy);

        // Confirm that the anchor state has not updated.
        (Hash updatedRoot, uint256 updatedL2BlockNumber) = anchorStateRegistry.getAnchorRoot();
        assertEq(updatedL2BlockNumber, l2BlockNumber);
        assertEq(updatedRoot.raw(), root.raw());
    }

    /// @notice Tests that setAnchorState will revert if the game is valid and the game status
    ///         is CHALLENGER_WINS.
    /// @param _l2BlockNumber The L2 block number to use for the game.
    function testFuzz_setAnchorState_challengerWins_fails(uint256 _l2BlockNumber) public {
        // Grab block number of the existing anchor root.
        (Hash root, uint256 l2BlockNumber) = anchorStateRegistry.getAnchorRoot();

        // Bound the new block number.
        _l2BlockNumber = bound(_l2BlockNumber, l2BlockNumber, type(uint256).max);

        // Mock the l2BlockNumber call.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.l2BlockNumber, ()), abi.encode(_l2BlockNumber));

        // Mock the CHALLENGER_WINS state.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.status, ()), abi.encode(GameStatus.CHALLENGER_WINS));

        // Mock that the game was respected.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.wasRespectedGameTypeWhenCreated, ()), abi.encode(true));

        // Mock the resolvedAt timestamp and fast forward to beyond the delay.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.resolvedAt, ()), abi.encode(block.timestamp));
        vm.warp(block.timestamp + optimismPortal2.disputeGameFinalityDelaySeconds() + 1);

        // Try to update the anchor state.
        vm.prank(address(gameProxy));
        vm.expectRevert(IAnchorStateRegistry.AnchorStateRegistry_InvalidAnchorGame.selector);
        anchorStateRegistry.setAnchorState(gameProxy);

        // Confirm that the anchor state has not updated.
        (Hash updatedRoot, uint256 updatedL2BlockNumber) = anchorStateRegistry.getAnchorRoot();
        assertEq(updatedL2BlockNumber, l2BlockNumber);
        assertEq(updatedRoot.raw(), root.raw());
    }

    /// @notice Tests that setAnchorState will revert if the game is valid and the game status
    ///         is IN_PROGRESS.
    /// @param _l2BlockNumber The L2 block number to use for the game.
    function testFuzz_setAnchorState_inProgress_fails(uint256 _l2BlockNumber) public {
        // Grab block number of the existing anchor root.
        (Hash root, uint256 l2BlockNumber) = anchorStateRegistry.getAnchorRoot();

        // Bound the new block number.
        _l2BlockNumber = bound(_l2BlockNumber, l2BlockNumber, type(uint256).max);

        // Mock the l2BlockNumber call.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.l2BlockNumber, ()), abi.encode(_l2BlockNumber));

        // Mock the CHALLENGER_WINS state.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.status, ()), abi.encode(GameStatus.IN_PROGRESS));

        // Mock that the game was respected.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.wasRespectedGameTypeWhenCreated, ()), abi.encode(true));

        // Mock the resolvedAt timestamp and fast forward to beyond the delay.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.resolvedAt, ()), abi.encode(block.timestamp));
        vm.warp(block.timestamp + optimismPortal2.disputeGameFinalityDelaySeconds() + 1);

        // Try to update the anchor state.
        vm.prank(address(gameProxy));
        vm.expectRevert(IAnchorStateRegistry.AnchorStateRegistry_InvalidAnchorGame.selector);
        anchorStateRegistry.setAnchorState(gameProxy);

        // Confirm that the anchor state has not updated.
        (Hash updatedRoot, uint256 updatedL2BlockNumber) = anchorStateRegistry.getAnchorRoot();
        assertEq(updatedL2BlockNumber, l2BlockNumber);
        assertEq(updatedRoot.raw(), root.raw());
    }

    /// @notice Tests that setAnchorState will revert if the game is not respected.
    /// @param _l2BlockNumber The L2 block number to use for the game.
    function testFuzz_setAnchorState_isNotRespectedGame_fails(uint256 _l2BlockNumber) public {
        // Grab block number of the existing anchor root.
        (Hash root, uint256 l2BlockNumber) = anchorStateRegistry.anchors(gameProxy.gameType());

        // Bound the new block number.
        _l2BlockNumber = bound(_l2BlockNumber, l2BlockNumber, type(uint256).max);

        // Mock the l2BlockNumber call.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.l2BlockNumber, ()), abi.encode(_l2BlockNumber));

        // Mock the DEFENDER_WINS state.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.status, ()), abi.encode(GameStatus.DEFENDER_WINS));

        // Mock the resolvedAt timestamp and fast forward to beyond the delay.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.resolvedAt, ()), abi.encode(block.timestamp));
        vm.warp(block.timestamp + optimismPortal2.disputeGameFinalityDelaySeconds() + 1);

        // Mock that the game was not respected when created.
        vm.mockCall(
            address(gameProxy), abi.encodeCall(gameProxy.wasRespectedGameTypeWhenCreated, ()), abi.encode(false)
        );

        // Try to update the anchor state.
        vm.prank(address(gameProxy));
        vm.expectRevert(IAnchorStateRegistry.AnchorStateRegistry_InvalidAnchorGame.selector);
        anchorStateRegistry.setAnchorState(gameProxy);

        // Confirm that the anchor state has not updated.
        (Hash updatedRoot, uint256 updatedL2BlockNumber) = anchorStateRegistry.anchors(gameProxy.gameType());
        assertEq(updatedL2BlockNumber, l2BlockNumber);
        assertEq(updatedRoot.raw(), root.raw());
    }

    /// @notice Tests that setAnchorState will revert if the game is valid and the game is
    ///         blacklisted.
    /// @param _l2BlockNumber The L2 block number to use for the game.
    function testFuzz_setAnchorState_blacklistedGame_fails(uint256 _l2BlockNumber) public {
        // Grab block number of the existing anchor root.
        (Hash root, uint256 l2BlockNumber) = anchorStateRegistry.getAnchorRoot();

        // Bound the new block number.
        _l2BlockNumber = bound(_l2BlockNumber, l2BlockNumber + 1, type(uint256).max);

        // Mock the DEFENDER_WINS state.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.status, ()), abi.encode(GameStatus.DEFENDER_WINS));

        // Mock that the game was respected.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.wasRespectedGameTypeWhenCreated, ()), abi.encode(true));

        // Mock the resolvedAt timestamp and fast forward to beyond the delay.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.resolvedAt, ()), abi.encode(block.timestamp));
        vm.warp(block.timestamp + optimismPortal2.disputeGameFinalityDelaySeconds() + 1);

        // Mock the disputeGameBlacklist call to return true.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.disputeGameBlacklist, (gameProxy)),
            abi.encode(true)
        );

        // Update the anchor state.
        vm.prank(address(gameProxy));
        vm.expectRevert(IAnchorStateRegistry.AnchorStateRegistry_InvalidAnchorGame.selector);
        anchorStateRegistry.setAnchorState(gameProxy);

        // Confirm that the anchor state has not updated.
        (Hash updatedRoot, uint256 updatedL2BlockNumber) = anchorStateRegistry.anchors(gameProxy.gameType());
        assertEq(updatedL2BlockNumber, l2BlockNumber);
        assertEq(updatedRoot.raw(), root.raw());
    }

    /// @notice Tests that setAnchorState will revert if the game is valid and the game is
    ///         retired.
    /// @param _l2BlockNumber The L2 block number to use for the game.
    function testFuzz_setAnchorState_retiredGame_fails(uint256 _l2BlockNumber) public {
        // Grab block number of the existing anchor root.
        (Hash root, uint256 l2BlockNumber) = anchorStateRegistry.getAnchorRoot();

        // Bound the new block number.
        _l2BlockNumber = bound(_l2BlockNumber, l2BlockNumber + 1, type(uint256).max);

        // Mock the DEFENDER_WINS state.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.status, ()), abi.encode(GameStatus.DEFENDER_WINS));

        // Mock that the game was respected.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.wasRespectedGameTypeWhenCreated, ()), abi.encode(true));

        // Mock the respectedGameTypeUpdatedAt call to be later than the game's creation time.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.respectedGameTypeUpdatedAt, ()),
            abi.encode(gameProxy.createdAt().raw() + 1)
        );

        // Update the anchor state.
        vm.prank(address(gameProxy));
        vm.expectRevert(IAnchorStateRegistry.AnchorStateRegistry_InvalidAnchorGame.selector);
        anchorStateRegistry.setAnchorState(gameProxy);

        // Confirm that the anchor state has not updated.
        (Hash updatedRoot, uint256 updatedL2BlockNumber) = anchorStateRegistry.anchors(gameProxy.gameType());
        assertEq(updatedL2BlockNumber, l2BlockNumber);
        assertEq(updatedRoot.raw(), root.raw());
    }
}
