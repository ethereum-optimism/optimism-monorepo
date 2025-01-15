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
        // Make our game type the respected game type.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.respectedGameType, ()),
            abi.encode(gameProxy.gameType())
        );
        assertTrue(anchorStateRegistry.isGameRespected(gameProxy));
    }

    /// @notice Tests that isGameRespected will return false if the game is not of the respected game
    ///         type.
    /// @param _gameType The game type to use for the test.
    function testFuzz_isGameRespected_isNotRespected_succeeds(GameType _gameType) public {
        if (_gameType.raw() == gameProxy.gameType().raw()) {
            _gameType = GameType.wrap(_gameType.raw() + 1);
        }

        // Make our game type NOT the respected game type.
        vm.mockCall(
            address(optimismPortal2), abi.encodeCall(optimismPortal2.respectedGameType, ()), abi.encode(_gameType)
        );
        assertFalse(anchorStateRegistry.isGameRespected(gameProxy));
    }
}

contract AnchorStateRegistry_IsGameRetired_Test is AnchorStateRegistry_Init {
    /// @notice Tests that isGameRetired will return true if the game is retired.
    /// @param _retirementTimestamp The retirement timestamp to use for the test.
    function testFuzz_isGameRetired_isRetired_succeeds(uint64 _retirementTimestamp) public {
        // Make sure retirement timestamp is later than the game's creation time.
        _retirementTimestamp = uint64(bound(_retirementTimestamp, gameProxy.createdAt().raw() + 1, type(uint64).max));

        // Mock the respectedGameTypeUpdatedAt call to be later than the game's creation time.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.respectedGameTypeUpdatedAt, ()),
            abi.encode(_retirementTimestamp)
        );
        assertTrue(anchorStateRegistry.isGameRetired(gameProxy));
    }

    /// @notice Tests that isGameRetired will return false if the game is not retired.
    /// @param _retirementTimestamp The retirement timestamp to use for the test.
    function testFuzz_isGameRetired_isNotRetired_succeeds(uint64 _retirementTimestamp) public {
        // Make sure retirement timestamp is earlier than the game's creation time.
        _retirementTimestamp = uint64(bound(_retirementTimestamp, 0, gameProxy.createdAt().raw()));

        // Mock the respectedGameTypeUpdatedAt call to be earlier than the game's creation time.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.respectedGameTypeUpdatedAt, ()),
            abi.encode(_retirementTimestamp)
        );
        assertFalse(anchorStateRegistry.isGameRetired(gameProxy));
    }
}

contract AnchorStateRegistry_IsGameProper_Test is AnchorStateRegistry_Init {
    /// @notice Tests that isGameProper will return true if the game meets all conditions.
    function test_isGameProper_meetsAllConditions_succeeds() public {
        // Make our game type the respected game type.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.respectedGameType, ()),
            abi.encode(gameProxy.gameType())
        );

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
    function testFuzz_isGameProper_isNotRespected_succeeds(GameType _gameType) public {
        if (_gameType.raw() == gameProxy.gameType().raw()) {
            _gameType = GameType.wrap(_gameType.raw() + 1);
        }

        // Make our game type NOT the respected game type.
        vm.mockCall(
            address(optimismPortal2), abi.encodeCall(optimismPortal2.respectedGameType, ()), abi.encode(_gameType)
        );

        assertFalse(anchorStateRegistry.isGameProper(gameProxy));
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

contract AnchorStateRegistry_TryUpdateAnchorState_Test is AnchorStateRegistry_Init {
    /// @notice Tests that tryUpdateAnchorState will succeed if the game is valid, the game block
    ///         number is greater than the current anchor root block number, and the game is the
    ///         currently respected game type.
    /// @param _l2BlockNumber The L2 block number to use for the game.
    function testFuzz_tryUpdateAnchorState_validNewerState_succeeds(uint256 _l2BlockNumber) public {
        // Grab block number of the existing anchor root.
        (Hash root, uint256 l2BlockNumber) = anchorStateRegistry.getAnchorRoot();

        // Bound the new block number.
        _l2BlockNumber = bound(_l2BlockNumber, l2BlockNumber + 1, type(uint256).max);

        // Mock the l2BlockNumber call.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.l2BlockNumber, ()), abi.encode(_l2BlockNumber));

        // Mock the DEFENDER_WINS state.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.status, ()), abi.encode(GameStatus.DEFENDER_WINS));

        // Make our game type the respected game type.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.respectedGameType, ()),
            abi.encode(gameProxy.gameType())
        );

        // Update the anchor state.
        vm.prank(address(gameProxy));
        vm.expectEmit(address(anchorStateRegistry));
        emit AnchorUpdated(gameProxy);
        anchorStateRegistry.tryUpdateAnchorState();

        // Confirm that the anchor state is now the same as the game state.
        (root, l2BlockNumber) = anchorStateRegistry.getAnchorRoot();
        assertEq(l2BlockNumber, gameProxy.l2BlockNumber());
        assertEq(root.raw(), gameProxy.rootClaim().raw());

        // Confirm that the anchor game is now set.
        IFaultDisputeGame anchorGame = anchorStateRegistry.anchorGame();
        assertEq(address(anchorGame), address(gameProxy));
    }

    /// @notice Tests that tryUpdateAnchorState will not update the anchor state if the game block
    ///         number is less than or equal to the current anchor root block number.
    /// @param _l2BlockNumber The L2 block number to use for the game.
    function testFuzz_tryUpdateAnchorState_validOlderStateNoUpdate_succeeds(uint256 _l2BlockNumber) public {
        // Grab block number of the existing anchor root.
        (Hash root, uint256 l2BlockNumber) = anchorStateRegistry.getAnchorRoot();

        // Bound the new block number.
        _l2BlockNumber = bound(_l2BlockNumber, 0, l2BlockNumber);

        // Mock the l2BlockNumber call.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.l2BlockNumber, ()), abi.encode(_l2BlockNumber));

        // Mock the DEFENDER_WINS state.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.status, ()), abi.encode(GameStatus.DEFENDER_WINS));

        // Make our game type the respected game type.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.respectedGameType, ()),
            abi.encode(gameProxy.gameType())
        );

        // Try to update the anchor state.
        vm.prank(address(gameProxy));
        vm.expectEmit(address(anchorStateRegistry));
        emit AnchorNotUpdated(gameProxy);
        anchorStateRegistry.tryUpdateAnchorState();

        // Confirm that the anchor state has not updated.
        (Hash updatedRoot, uint256 updatedL2BlockNumber) = anchorStateRegistry.getAnchorRoot();
        assertEq(updatedL2BlockNumber, l2BlockNumber);
        assertEq(updatedRoot.raw(), root.raw());
    }

    /// @notice Tests that tryUpdateAnchorState will not update the anchor state if the game is not
    ///         registered.
    /// @param _l2BlockNumber The L2 block number to use for the game.
    function testFuzz_tryUpdateAnchorState_notFactoryRegisteredGameNoUpdate_succeeds(uint256 _l2BlockNumber) public {
        // Grab block number of the existing anchor root.
        (Hash root, uint256 l2BlockNumber) = anchorStateRegistry.getAnchorRoot();

        // Bound the new block number.
        _l2BlockNumber = bound(_l2BlockNumber, l2BlockNumber, type(uint256).max);

        // Mock the l2BlockNumber call.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.l2BlockNumber, ()), abi.encode(_l2BlockNumber));

        // Mock the DEFENDER_WINS state.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.status, ()), abi.encode(GameStatus.DEFENDER_WINS));

        // Make our game type the respected game type.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.respectedGameType, ()),
            abi.encode(gameProxy.gameType())
        );

        // Mock the DisputeGameFactory to make it seem that the game was not registered.
        vm.mockCall(
            address(disputeGameFactory),
            abi.encodeCall(
                disputeGameFactory.games, (gameProxy.gameType(), gameProxy.rootClaim(), gameProxy.extraData())
            ),
            abi.encode(address(0), 0)
        );

        // Try to update the anchor state.
        vm.prank(address(gameProxy));
        vm.expectEmit(address(anchorStateRegistry));
        emit AnchorNotUpdated(gameProxy);
        anchorStateRegistry.tryUpdateAnchorState();

        // Confirm that the anchor state has not updated.
        (Hash updatedRoot, uint256 updatedL2BlockNumber) = anchorStateRegistry.getAnchorRoot();
        assertEq(updatedL2BlockNumber, l2BlockNumber);
        assertEq(updatedRoot.raw(), root.raw());
    }

    /// @notice Tests that tryUpdateAnchorState will not update the anchor state if the game status
    ///         is CHALLENGER_WINS.
    /// @param _l2BlockNumber The L2 block number to use for the game.
    function testFuzz_tryUpdateAnchorState_challengerWinsNoUpdate_succeeds(uint256 _l2BlockNumber) public {
        // Grab block number of the existing anchor root.
        (Hash root, uint256 l2BlockNumber) = anchorStateRegistry.getAnchorRoot();

        // Bound the new block number.
        _l2BlockNumber = bound(_l2BlockNumber, l2BlockNumber, type(uint256).max);

        // Mock the l2BlockNumber call.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.l2BlockNumber, ()), abi.encode(_l2BlockNumber));

        // Mock the CHALLENGER_WINS state.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.status, ()), abi.encode(GameStatus.CHALLENGER_WINS));

        // Make our game type the respected game type.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.respectedGameType, ()),
            abi.encode(gameProxy.gameType())
        );

        // Try to update the anchor state.
        vm.prank(address(gameProxy));
        vm.expectEmit(address(anchorStateRegistry));
        emit AnchorNotUpdated(gameProxy);
        anchorStateRegistry.tryUpdateAnchorState();

        // Confirm that the anchor state has not updated.
        (Hash updatedRoot, uint256 updatedL2BlockNumber) = anchorStateRegistry.getAnchorRoot();
        assertEq(updatedL2BlockNumber, l2BlockNumber);
        assertEq(updatedRoot.raw(), root.raw());
    }

    /// @notice Tests that tryUpdateAnchorState will not update the anchor state if the game status
    ///         is IN_PROGRESS.
    /// @param _l2BlockNumber The L2 block number to use for the game.
    function testFuzz_tryUpdateAnchorState_inProgressNoUpdate_succeeds(uint256 _l2BlockNumber) public {
        // Grab block number of the existing anchor root.
        (Hash root, uint256 l2BlockNumber) = anchorStateRegistry.getAnchorRoot();

        // Bound the new block number.
        _l2BlockNumber = bound(_l2BlockNumber, l2BlockNumber, type(uint256).max);

        // Mock the l2BlockNumber call.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.l2BlockNumber, ()), abi.encode(_l2BlockNumber));

        // Mock the CHALLENGER_WINS state.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.status, ()), abi.encode(GameStatus.IN_PROGRESS));

        // Make our game type the respected game type.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.respectedGameType, ()),
            abi.encode(gameProxy.gameType())
        );

        // Try to update the anchor state.
        vm.prank(address(gameProxy));
        vm.expectEmit(address(anchorStateRegistry));
        emit AnchorNotUpdated(gameProxy);
        anchorStateRegistry.tryUpdateAnchorState();

        // Confirm that the anchor state has not updated.
        (Hash updatedRoot, uint256 updatedL2BlockNumber) = anchorStateRegistry.getAnchorRoot();
        assertEq(updatedL2BlockNumber, l2BlockNumber);
        assertEq(updatedRoot.raw(), root.raw());
    }

    /// @notice Tests that tryUpdateAnchorState will not update the anchor state if the game type
    ///         is not the respected game type.
    /// @param _l2BlockNumber The L2 block number to use for the game.
    function testFuzz_tryUpdateAnchorState_notRespectedGameTypeNoUpdate_succeeds(uint256 _l2BlockNumber) public {
        // Grab block number of the existing anchor root.
        (Hash root, uint256 l2BlockNumber) = anchorStateRegistry.anchors(gameProxy.gameType());

        // Bound the new block number.
        _l2BlockNumber = bound(_l2BlockNumber, l2BlockNumber, type(uint256).max);

        // Mock the l2BlockNumber call.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.l2BlockNumber, ()), abi.encode(_l2BlockNumber));

        // Mock the DEFENDER_WINS state.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.status, ()), abi.encode(GameStatus.DEFENDER_WINS));

        // Mock the respectedGameType call so that it does NOT match our game type.
        vm.mockCall(address(optimismPortal2), abi.encodeCall(optimismPortal2.respectedGameType, ()), abi.encode(999));

        // Try to update the anchor state.
        vm.prank(address(gameProxy));
        vm.expectEmit(address(anchorStateRegistry));
        emit AnchorNotUpdated(gameProxy);
        anchorStateRegistry.tryUpdateAnchorState();

        // Confirm that the anchor state has not updated.
        (Hash updatedRoot, uint256 updatedL2BlockNumber) = anchorStateRegistry.anchors(gameProxy.gameType());
        assertEq(updatedL2BlockNumber, l2BlockNumber);
        assertEq(updatedRoot.raw(), root.raw());
    }

    /// @notice Tests that tryUpdateAnchorState will not update the anchor state if the game is
    ///         blacklisted.
    /// @param _l2BlockNumber The L2 block number to use for the game.
    function testFuzz_tryUpdateAnchorState_blacklistedGameNoUpdate_succeeds(uint256 _l2BlockNumber) public {
        // Grab block number of the existing anchor root.
        (Hash root, uint256 l2BlockNumber) = anchorStateRegistry.getAnchorRoot();

        // Bound the new block number.
        _l2BlockNumber = bound(_l2BlockNumber, l2BlockNumber + 1, type(uint256).max);

        // Mock the l2BlockNumber call.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.l2BlockNumber, ()), abi.encode(_l2BlockNumber));

        // Mock the DEFENDER_WINS state.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.status, ()), abi.encode(GameStatus.DEFENDER_WINS));

        // Make our game type the respected game type.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.respectedGameType, ()),
            abi.encode(gameProxy.gameType())
        );

        // Mock the disputeGameBlacklist call to return true.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.disputeGameBlacklist, (gameProxy)),
            abi.encode(true)
        );

        // Update the anchor state.
        vm.prank(address(gameProxy));
        vm.expectEmit(address(anchorStateRegistry));
        emit AnchorNotUpdated(gameProxy);
        anchorStateRegistry.tryUpdateAnchorState();

        // Confirm that the anchor state has not updated.
        (Hash updatedRoot, uint256 updatedL2BlockNumber) = anchorStateRegistry.anchors(gameProxy.gameType());
        assertEq(updatedL2BlockNumber, l2BlockNumber);
        assertEq(updatedRoot.raw(), root.raw());
    }

    /// @notice Tests that tryUpdateAnchorState will not update the anchor state if the game is
    ///         retired.
    /// @param _l2BlockNumber The L2 block number to use for the game.
    function testFuzz_tryUpdateAnchorState_retiredGameNoUpdate_succeeds(uint256 _l2BlockNumber) public {
        // Grab block number of the existing anchor root.
        (Hash root, uint256 l2BlockNumber) = anchorStateRegistry.getAnchorRoot();

        // Bound the new block number.
        _l2BlockNumber = bound(_l2BlockNumber, l2BlockNumber + 1, type(uint256).max);

        // Mock the l2BlockNumber call.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.l2BlockNumber, ()), abi.encode(_l2BlockNumber));

        // Mock the DEFENDER_WINS state.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.status, ()), abi.encode(GameStatus.DEFENDER_WINS));

        // Make our game type the respected game type.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.respectedGameType, ()),
            abi.encode(gameProxy.gameType())
        );

        // Mock the respectedGameTypeUpdatedAt call to be later than the game's creation time.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.respectedGameTypeUpdatedAt, ()),
            abi.encode(gameProxy.createdAt().raw() + 1)
        );

        // Update the anchor state.
        vm.prank(address(gameProxy));
        vm.expectEmit(address(anchorStateRegistry));
        emit AnchorNotUpdated(gameProxy);
        anchorStateRegistry.tryUpdateAnchorState();

        // Confirm that the anchor state has not updated.
        (Hash updatedRoot, uint256 updatedL2BlockNumber) = anchorStateRegistry.anchors(gameProxy.gameType());
        assertEq(updatedL2BlockNumber, l2BlockNumber);
        assertEq(updatedRoot.raw(), root.raw());
    }
}

contract AnchorStateRegistry_SetAnchorState_Test is AnchorStateRegistry_Init {
    /// @notice Tests that setAnchorState will succeed with a game with any L2 block number as long
    ///         as the game is valid and is the currently respected game type.
    /// @param _l2BlockNumber The L2 block number to use for the game.
    function testFuzz_setAnchorState_anyL2BlockNumber_succeeds(uint256 _l2BlockNumber) public {
        // Mock the l2BlockNumber call.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.l2BlockNumber, ()), abi.encode(_l2BlockNumber));

        // Mock the DEFENDER_WINS state.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.status, ()), abi.encode(GameStatus.DEFENDER_WINS));

        // Make our game type the respected game type.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.respectedGameType, ()),
            abi.encode(gameProxy.gameType())
        );

        // Set the anchor state.
        vm.prank(superchainConfig.guardian());
        vm.expectEmit(address(anchorStateRegistry));
        emit AnchorUpdated(gameProxy);
        anchorStateRegistry.setAnchorState(gameProxy);

        // Confirm that the anchor state has updated.
        (Hash updatedRoot, uint256 updatedL2BlockNumber) = anchorStateRegistry.anchors(gameProxy.gameType());
        assertEq(updatedL2BlockNumber, gameProxy.l2BlockNumber());
        assertEq(updatedRoot.raw(), gameProxy.rootClaim().raw());

        // Confirm that the anchor game is now set.
        IFaultDisputeGame anchorGame = anchorStateRegistry.anchorGame();
        assertEq(address(anchorGame), address(gameProxy));
    }
}

contract AnchorStateRegistry_SetAnchorState_TestFail is AnchorStateRegistry_Init {
    /// @notice Tests that setAnchorState will revert if the sender is not the guardian.
    /// @param _sender The address of the sender.
    /// @param _l2BlockNumber The L2 block number to use for the game.
    function testFuzz_setAnchorState_notGuardian_fails(address _sender, uint256 _l2BlockNumber) public {
        // Grab block number of the existing anchor root.
        (Hash root, uint256 l2BlockNumber) = anchorStateRegistry.getAnchorRoot();

        // Mock the l2BlockNumber call.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.l2BlockNumber, ()), abi.encode(_l2BlockNumber));

        // Mock the DEFENDER_WINS state.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.status, ()), abi.encode(GameStatus.DEFENDER_WINS));

        // Make our game type the respected game type.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.respectedGameType, ()),
            abi.encode(gameProxy.gameType())
        );

        // Mock the DisputeGameFactory to make it seem that the game was not registered.
        vm.mockCall(
            address(disputeGameFactory),
            abi.encodeCall(
                disputeGameFactory.games, (gameProxy.gameType(), gameProxy.rootClaim(), gameProxy.extraData())
            ),
            abi.encode(address(0), 0)
        );

        // Try to update the anchor state.
        vm.prank(_sender);
        vm.expectRevert(IAnchorStateRegistry.AnchorStateRegistry_Unauthorized.selector);
        anchorStateRegistry.setAnchorState(gameProxy);

        // Confirm that the anchor state has not updated.
        (Hash updatedRoot, uint256 updatedL2BlockNumber) = anchorStateRegistry.anchors(gameProxy.gameType());
        assertEq(updatedL2BlockNumber, l2BlockNumber);
        assertEq(updatedRoot.raw(), root.raw());
    }

    /// @notice Tests that setAnchorState will revert if the game is not registered.
    /// @param _l2BlockNumber The L2 block number to use for the game.
    function testFuzz_setAnchorState_notFactoryRegisteredGame_fails(uint256 _l2BlockNumber) public {
        // Grab block number of the existing anchor root.
        (Hash root, uint256 l2BlockNumber) = anchorStateRegistry.getAnchorRoot();

        // Mock the l2BlockNumber call.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.l2BlockNumber, ()), abi.encode(_l2BlockNumber));

        // Mock the DEFENDER_WINS state.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.status, ()), abi.encode(GameStatus.DEFENDER_WINS));

        // Make our game type the respected game type.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.respectedGameType, ()),
            abi.encode(gameProxy.gameType())
        );

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
        vm.expectRevert(IAnchorStateRegistry.AnchorStateRegistry_ImproperAnchorGame.selector);
        anchorStateRegistry.setAnchorState(gameProxy);

        // Confirm that the anchor state has not updated.
        (Hash updatedRoot, uint256 updatedL2BlockNumber) = anchorStateRegistry.anchors(gameProxy.gameType());
        assertEq(updatedL2BlockNumber, l2BlockNumber);
        assertEq(updatedRoot.raw(), root.raw());
    }

    /// @notice Tests that setAnchorState will revert if the game is valid and the game status is
    ///         CHALLENGER_WINS.
    /// @param _l2BlockNumber The L2 block number to use for the game.
    function testFuzz_setAnchorState_challengerWins_fails(uint256 _l2BlockNumber) public {
        // Grab block number of the existing anchor root.
        (Hash root, uint256 l2BlockNumber) = anchorStateRegistry.getAnchorRoot();

        // Mock the l2BlockNumber call.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.l2BlockNumber, ()), abi.encode(_l2BlockNumber));

        // Mock the CHALLENGER_WINS state.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.status, ()), abi.encode(GameStatus.CHALLENGER_WINS));

        // Make our game type the respected game type.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.respectedGameType, ()),
            abi.encode(gameProxy.gameType())
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

    /// @notice Tests that setAnchorState will revert if the game is valid and the game status is
    ///         IN_PROGRESS.
    /// @param _l2BlockNumber The L2 block number to use for the game.
    function testFuzz_setAnchorState_inProgress_fails(uint256 _l2BlockNumber) public {
        // Grab block number of the existing anchor root.
        (Hash root, uint256 l2BlockNumber) = anchorStateRegistry.getAnchorRoot();

        // Bound the new block number.
        _l2BlockNumber = bound(_l2BlockNumber, l2BlockNumber, type(uint256).max);

        // Mock the l2BlockNumber call.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.l2BlockNumber, ()), abi.encode(_l2BlockNumber));

        // Mock the IN_PROGRESS state.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.status, ()), abi.encode(GameStatus.IN_PROGRESS));

        // Make our game type the respected game type.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.respectedGameType, ()),
            abi.encode(gameProxy.gameType())
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

    /// @notice Tests that setAnchorState will revert if the game is valid and the game type is not
    ///         the respected game type.
    /// @param _l2BlockNumber The L2 block number to use for the game.
    function testFuzz_setAnchorState_notRespectedGameType_fails(uint256 _l2BlockNumber) public {
        // Grab block number of the existing anchor root.
        (Hash root, uint256 l2BlockNumber) = anchorStateRegistry.anchors(gameProxy.gameType());

        // Mock the l2BlockNumber call.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.l2BlockNumber, ()), abi.encode(_l2BlockNumber));

        // Mock the DEFENDER_WINS state.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.status, ()), abi.encode(GameStatus.DEFENDER_WINS));

        // Mock the respectedGameType call so that it does NOT match our game type.
        vm.mockCall(address(optimismPortal2), abi.encodeCall(optimismPortal2.respectedGameType, ()), abi.encode(999));

        // Try to update the anchor state.
        vm.prank(superchainConfig.guardian());
        vm.expectRevert(IAnchorStateRegistry.AnchorStateRegistry_ImproperAnchorGame.selector);
        anchorStateRegistry.setAnchorState(gameProxy);

        // Confirm that the anchor state has not updated.
        (Hash updatedRoot, uint256 updatedL2BlockNumber) = anchorStateRegistry.anchors(gameProxy.gameType());
        assertEq(updatedL2BlockNumber, l2BlockNumber);
        assertEq(updatedRoot.raw(), root.raw());
    }

    /// @notice Tests that setAnchorState will revert if the game is valid and the game is blacklisted.
    /// @param _l2BlockNumber The L2 block number to use for the game.
    function test_setAnchorState_blacklistedGame_fails(uint256 _l2BlockNumber) public {
        // Grab block number of the existing anchor root.
        (Hash root, uint256 l2BlockNumber) = anchorStateRegistry.anchors(gameProxy.gameType());

        // Mock the l2BlockNumber call.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.l2BlockNumber, ()), abi.encode(_l2BlockNumber));

        // Mock the DEFENDER_WINS state.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.status, ()), abi.encode(GameStatus.DEFENDER_WINS));

        // Make our game type the respected game type.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.respectedGameType, ()),
            abi.encode(gameProxy.gameType())
        );

        // Mock the disputeGameBlacklist call to return true.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.disputeGameBlacklist, (gameProxy)),
            abi.encode(true)
        );

        // Set the anchor state.
        vm.prank(superchainConfig.guardian());
        vm.expectRevert(IAnchorStateRegistry.AnchorStateRegistry_ImproperAnchorGame.selector);
        anchorStateRegistry.setAnchorState(gameProxy);

        // Confirm that the anchor state has not updated.
        (Hash updatedRoot, uint256 updatedL2BlockNumber) = anchorStateRegistry.anchors(gameProxy.gameType());
        assertEq(updatedL2BlockNumber, l2BlockNumber);
        assertEq(updatedRoot.raw(), root.raw());
    }

    /// @notice Tests that setAnchorState will revert if the game is valid and the game is retired.
    /// @param _l2BlockNumber The L2 block number to use for the game.
    function test_setAnchorState_retiredGame_fails(uint256 _l2BlockNumber) public {
        // Grab block number of the existing anchor root.
        (Hash root, uint256 l2BlockNumber) = anchorStateRegistry.anchors(gameProxy.gameType());

        // Mock the l2BlockNumber call.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.l2BlockNumber, ()), abi.encode(_l2BlockNumber));

        // Mock the DEFENDER_WINS state.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.status, ()), abi.encode(GameStatus.DEFENDER_WINS));

        // Make our game type the respected game type.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.respectedGameType, ()),
            abi.encode(gameProxy.gameType())
        );

        // Mock the respectedGameTypeUpdatedAt call to be later than the game's creation time.
        vm.mockCall(
            address(optimismPortal2),
            abi.encodeCall(optimismPortal2.respectedGameTypeUpdatedAt, ()),
            abi.encode(gameProxy.createdAt().raw() + 1)
        );

        // Set the anchor state.
        vm.prank(superchainConfig.guardian());
        vm.expectRevert(IAnchorStateRegistry.AnchorStateRegistry_ImproperAnchorGame.selector);
        anchorStateRegistry.setAnchorState(gameProxy);

        // Confirm that the anchor state has not updated.
        (Hash updatedRoot, uint256 updatedL2BlockNumber) = anchorStateRegistry.anchors(gameProxy.gameType());
        assertEq(updatedL2BlockNumber, l2BlockNumber);
        assertEq(updatedRoot.raw(), root.raw());
    }
}
