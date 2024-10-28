// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

// Testing
import { Test } from "forge-std/Test.sol";
import { Vm } from "forge-std/Vm.sol";
import { DisputeGameFactory_Init } from "test/dispute/DisputeGameFactory.t.sol";
import { AlphabetVM } from "test/mocks/AlphabetVM.sol";
import { stdError } from "forge-std/StdError.sol";

// Scripts
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

// Contracts
import { DisputeActor, HonestDisputeActor } from "test/actors/FaultDisputeActors.sol";

// Libraries
import { Types } from "src/libraries/Types.sol";
import { Hashing } from "src/libraries/Hashing.sol";
import { RLPWriter } from "src/libraries/rlp/RLPWriter.sol";
import { LibClock } from "src/dispute/lib/LibUDT.sol";
import { LibPosition } from "src/dispute/lib/LibPosition.sol";
import "src/dispute/lib/Types.sol";
import "src/dispute/lib/Errors.sol";

// Interfaces
import { IDisputeGame } from "src/dispute/interfaces/IDisputeGame.sol";
import { IPreimageOracle } from "src/dispute/interfaces/IBigStepper.sol";
import { IAnchorStateRegistry } from "src/dispute/interfaces/IAnchorStateRegistry.sol";
import { IFaultDisputeGame } from "src/dispute/interfaces/IFaultDisputeGame.sol";
import { IDelayedWETH } from "src/dispute/interfaces/IDelayedWETH.sol";

contract FaultDisputeGame_Init is DisputeGameFactory_Init {
    /// @dev The type of the game being tested.
    GameType internal constant GAME_TYPE = GameType.wrap(0);

    /// @dev The implementation of the game.
    IFaultDisputeGame internal gameImpl;
    /// @dev The `Clone` proxy of the game.
    IFaultDisputeGame internal gameProxy;

    /// @dev The extra data passed to the game for initialization.
    bytes internal extraData;

    event Move(uint256 indexed parentIndex, Claim indexed pivot, address indexed claimant);

    event ReceiveETH(uint256 amount);

    function init(Claim rootClaim, Claim absolutePrestate, uint256 l2BlockNumber) public {
        // Set the time to a realistic date.
        vm.warp(1690906994);

        // Set the extra data for the game creation
        extraData = abi.encode(l2BlockNumber);

        // Set preimage oracle challenge period to something arbitrary (4 seconds) just so we can
        // actually test the clock extensions later on. This is not a realistic value.
        AlphabetVM _vm = new AlphabetVM(
            absolutePrestate,
            IPreimageOracle(
                DeployUtils.create1({
                    _name: "PreimageOracle",
                    _args: DeployUtils.encodeConstructor(abi.encodeCall(IPreimageOracle.__constructor__, (0, 4)))
                })
            )
        );

        // Deploy an implementation of the fault game
        gameImpl = IFaultDisputeGame(
            DeployUtils.create1({
                _name: "FaultDisputeGame",
                _args: DeployUtils.encodeConstructor(
                    abi.encodeCall(
                        IFaultDisputeGame.__constructor__,
                        (
                            GAME_TYPE,
                            absolutePrestate,
                            2 ** 3,
                            2 ** 2,
                            Duration.wrap(3 hours),
                            Duration.wrap(3.5 days),
                            _vm,
                            delayedWeth,
                            anchorStateRegistry,
                            10
                        )
                    )
                )
            })
        );

        // Register the game implementation with the factory.
        disputeGameFactory.setImplementation(GAME_TYPE, gameImpl);
        // Create a new game.
        gameProxy = IFaultDisputeGame(payable(address(disputeGameFactory.create(GAME_TYPE, rootClaim, extraData))));

        // Check immutables
        assertEq(gameProxy.gameType().raw(), GAME_TYPE.raw());
        assertEq(gameProxy.absolutePrestate().raw(), absolutePrestate.raw());
        assertEq(gameProxy.maxGameDepth(), 2 ** 3);
        assertEq(gameProxy.splitDepth(), 2 ** 2);
        assertEq(gameProxy.clockExtension().raw(), 3 hours);
        assertEq(gameProxy.maxClockDuration().raw(), 3.5 days);
        assertEq(address(gameProxy.weth()), address(delayedWeth));
        assertEq(address(gameProxy.anchorStateRegistry()), address(anchorStateRegistry));
        assertEq(address(gameProxy.vm()), address(_vm));

        // Label the proxy
        vm.label(address(gameProxy), "FaultDisputeGame_Clone");
    }

    fallback() external payable { }

    receive() external payable { }
}

contract FaultDisputeGame_Test is FaultDisputeGame_Init {
    /// @dev The root claim of the game.
    Claim internal constant ROOT_CLAIM = Claim.wrap(bytes32((uint256(1) << 248) | uint256(10)));

    /// @dev The preimage of the absolute prestate claim
    bytes internal absolutePrestateData;
    /// @dev The absolute prestate of the trace.
    Claim internal absolutePrestate;

    function setUp() public override {
        absolutePrestateData = abi.encode(0);
        absolutePrestate = _changeClaimStatus(Claim.wrap(keccak256(absolutePrestateData)), VMStatuses.UNFINISHED);

        super.setUp();
        super.init({ rootClaim: ROOT_CLAIM, absolutePrestate: absolutePrestate, l2BlockNumber: 0x10 });
    }

    ////////////////////////////////////////////////////////////////
    //            `IDisputeGame` Implementation Tests             //
    ////////////////////////////////////////////////////////////////

    /// @dev Tests that the constructor of the `FaultDisputeGame` reverts when the `MAX_GAME_DEPTH` parameter is
    ///      greater  than `LibPosition.MAX_POSITION_BITLEN - 1`.
    function testFuzz_constructor_maxDepthTooLarge_reverts(uint256 _maxGameDepth) public {
        IPreimageOracle oracle = IPreimageOracle(
            DeployUtils.create1({
                _name: "PreimageOracle",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IPreimageOracle.__constructor__, (0, 0)))
            })
        );
        AlphabetVM alphabetVM = new AlphabetVM(absolutePrestate, oracle);

        _maxGameDepth = bound(_maxGameDepth, LibPosition.MAX_POSITION_BITLEN, type(uint256).max - 1);
        vm.expectRevert(MaxDepthTooLarge.selector);
        DeployUtils.create1({
            _name: "FaultDisputeGame",
            _args: DeployUtils.encodeConstructor(
                abi.encodeCall(
                    IFaultDisputeGame.__constructor__,
                    (
                        GAME_TYPE,
                        absolutePrestate,
                        _maxGameDepth,
                        _maxGameDepth + 1,
                        Duration.wrap(3 hours),
                        Duration.wrap(3.5 days),
                        alphabetVM,
                        IDelayedWETH(payable(address(0))),
                        IAnchorStateRegistry(address(0)),
                        10
                    )
                )
            )
        });
    }

    /// @dev Tests that the constructor of the `FaultDisputeGame` reverts when the challenge period
    //       of the preimage oracle being used by the game's VM is too large.
    /// @param _challengePeriod The challenge period of the preimage oracle.
    function testFuzz_constructor_oracleChallengePeriodTooLarge_reverts(uint256 _challengePeriod) public {
        _challengePeriod = bound(_challengePeriod, uint256(type(uint64).max) + 1, type(uint256).max);

        IPreimageOracle oracle = IPreimageOracle(
            DeployUtils.create1({
                _name: "PreimageOracle",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IPreimageOracle.__constructor__, (0, 0)))
            })
        );
        AlphabetVM alphabetVM = new AlphabetVM(absolutePrestate, IPreimageOracle(address(oracle)));

        // PreimageOracle constructor will revert if the challenge period is too large, so we need
        // to mock the call to pretend this is a bugged implementation where the challenge period
        // is allowed to be too large.
        vm.mockCall(address(oracle), abi.encodeCall(IPreimageOracle.challengePeriod, ()), abi.encode(_challengePeriod));

        vm.expectRevert(InvalidChallengePeriod.selector);
        DeployUtils.create1({
            _name: "FaultDisputeGame",
            _args: DeployUtils.encodeConstructor(
                abi.encodeCall(
                    IFaultDisputeGame.__constructor__,
                    (
                        GAME_TYPE,
                        absolutePrestate,
                        2 ** 3,
                        2 ** 2,
                        Duration.wrap(3 hours),
                        Duration.wrap(3.5 days),
                        alphabetVM,
                        IDelayedWETH(payable(address(0))),
                        IAnchorStateRegistry(address(0)),
                        10
                    )
                )
            )
        });
    }

    /// @dev Tests that the constructor of the `FaultDisputeGame` reverts when the `_splitDepth`
    ///      parameter is greater than or equal to the `MAX_GAME_DEPTH`
    function testFuzz_constructor_invalidSplitDepth_reverts(uint256 _splitDepth) public {
        AlphabetVM alphabetVM = new AlphabetVM(
            absolutePrestate,
            IPreimageOracle(
                DeployUtils.create1({
                    _name: "PreimageOracle",
                    _args: DeployUtils.encodeConstructor(abi.encodeCall(IPreimageOracle.__constructor__, (0, 0)))
                })
            )
        );

        uint256 maxGameDepth = 2 ** 3;
        _splitDepth = bound(_splitDepth, maxGameDepth - 1, type(uint256).max);
        vm.expectRevert(InvalidSplitDepth.selector);
        DeployUtils.create1({
            _name: "FaultDisputeGame",
            _args: DeployUtils.encodeConstructor(
                abi.encodeCall(
                    IFaultDisputeGame.__constructor__,
                    (
                        GAME_TYPE,
                        absolutePrestate,
                        maxGameDepth,
                        _splitDepth,
                        Duration.wrap(3 hours),
                        Duration.wrap(3.5 days),
                        alphabetVM,
                        IDelayedWETH(payable(address(0))),
                        IAnchorStateRegistry(address(0)),
                        10
                    )
                )
            )
        });
    }

    /// @dev Tests that the constructor of the `FaultDisputeGame` reverts when the `_splitDepth`
    ///      parameter is less than the minimum split depth (currently 2).
    function testFuzz_constructor_lowSplitDepth_reverts(uint256 _splitDepth) public {
        AlphabetVM alphabetVM = new AlphabetVM(
            absolutePrestate,
            IPreimageOracle(
                DeployUtils.create1({
                    _name: "PreimageOracle",
                    _args: DeployUtils.encodeConstructor(abi.encodeCall(IPreimageOracle.__constructor__, (0, 0)))
                })
            )
        );

        uint256 minSplitDepth = 2;
        _splitDepth = bound(_splitDepth, 0, minSplitDepth - 1);
        vm.expectRevert(InvalidSplitDepth.selector);
        DeployUtils.create1({
            _name: "FaultDisputeGame",
            _args: DeployUtils.encodeConstructor(
                abi.encodeCall(
                    IFaultDisputeGame.__constructor__,
                    (
                        GAME_TYPE,
                        absolutePrestate,
                        2 ** 3,
                        _splitDepth,
                        Duration.wrap(3 hours),
                        Duration.wrap(3.5 days),
                        alphabetVM,
                        IDelayedWETH(payable(address(0))),
                        IAnchorStateRegistry(address(0)),
                        10
                    )
                )
            )
        });
    }

    /// @dev Tests that the constructor of the `FaultDisputeGame` reverts when clock extension * 2 is greater than
    ///      the max clock duration.
    function testFuzz_constructor_clockExtensionTooLong_reverts(
        uint64 _maxClockDuration,
        uint64 _clockExtension
    )
        public
    {
        AlphabetVM alphabetVM = new AlphabetVM(
            absolutePrestate,
            IPreimageOracle(
                DeployUtils.create1({
                    _name: "PreimageOracle",
                    _args: DeployUtils.encodeConstructor(abi.encodeCall(IPreimageOracle.__constructor__, (0, 0)))
                })
            )
        );

        // Force the clock extension * 2 to be greater than the max clock duration, but keep things within
        // bounds of the uint64 type.
        _maxClockDuration = uint64(bound(_maxClockDuration, 0, type(uint64).max / 2 - 1));
        _clockExtension = uint64(bound(_clockExtension, _maxClockDuration / 2 + 1, type(uint64).max / 2));

        vm.expectRevert(InvalidClockExtension.selector);
        DeployUtils.create1({
            _name: "FaultDisputeGame",
            _args: DeployUtils.encodeConstructor(
                abi.encodeCall(
                    IFaultDisputeGame.__constructor__,
                    (
                        GAME_TYPE,
                        absolutePrestate,
                        16,
                        8,
                        Duration.wrap(_clockExtension),
                        Duration.wrap(_maxClockDuration),
                        alphabetVM,
                        IDelayedWETH(payable(address(0))),
                        IAnchorStateRegistry(address(0)),
                        10
                    )
                )
            )
        });
    }

    /// @dev Tests that the game's root claim is set correctly.
    function test_rootClaim_succeeds() public view {
        assertEq(gameProxy.rootClaim().raw(), ROOT_CLAIM.raw());
    }

    /// @dev Tests that the game's extra data is set correctly.
    function test_extraData_succeeds() public view {
        assertEq(gameProxy.extraData(), extraData);
    }

    /// @dev Tests that the game's starting timestamp is set correctly.
    function test_createdAt_succeeds() public view {
        assertEq(gameProxy.createdAt().raw(), block.timestamp);
    }

    /// @dev Tests that the game's type is set correctly.
    function test_gameType_succeeds() public view {
        assertEq(gameProxy.gameType().raw(), GAME_TYPE.raw());
    }

    /// @dev Tests that the game's data is set correctly.
    function test_gameData_succeeds() public view {
        (GameType gameType, Claim rootClaim, bytes memory _extraData) = gameProxy.gameData();

        assertEq(gameType.raw(), GAME_TYPE.raw());
        assertEq(rootClaim.raw(), ROOT_CLAIM.raw());
        assertEq(_extraData, extraData);
    }

    ////////////////////////////////////////////////////////////////
    //          `IFaultDisputeGame` Implementation Tests       //
    ////////////////////////////////////////////////////////////////

    /// @dev Tests that the game cannot be initialized with an output root that commits to <= the configured starting
    ///      block number
    function testFuzz_initialize_cannotProposeGenesis_reverts(uint256 _blockNumber) public {
        (, uint256 startingL2Block) = gameProxy.startingOutputRoot();
        _blockNumber = bound(_blockNumber, 0, startingL2Block);

        Claim claim = _dummyClaim();
        vm.expectRevert(abi.encodeWithSelector(UnexpectedRootClaim.selector, claim));
        gameProxy =
            IFaultDisputeGame(payable(address(disputeGameFactory.create(GAME_TYPE, claim, abi.encode(_blockNumber)))));
    }

    /// @dev Tests that the proxy receives ETH from the dispute game factory.
    function test_initialize_receivesETH_succeeds() public {
        uint256 _value = disputeGameFactory.initBonds(GAME_TYPE);
        vm.deal(address(this), _value);

        assertEq(address(gameProxy).balance, 0);
        gameProxy = IFaultDisputeGame(
            payable(address(disputeGameFactory.create{ value: _value }(GAME_TYPE, ROOT_CLAIM, abi.encode(1))))
        );
        assertEq(address(gameProxy).balance, 0);
        assertEq(delayedWeth.balanceOf(address(gameProxy)), _value);
    }

    /// @dev Tests that the game cannot be initialized with extra data of the incorrect length (must be 32 bytes)
    function testFuzz_initialize_badExtraData_reverts(uint256 _extraDataLen) public {
        // The `DisputeGameFactory` will pack the root claim and the extra data into a single array, which is enforced
        // to be at least 64 bytes long.
        // We bound the upper end to 23.5KB to ensure that the minimal proxy never surpasses the contract size limit
        // in this test, as CWIA proxies store the immutable args in their bytecode.
        // [0 bytes, 31 bytes] u [33 bytes, 23.5 KB]
        _extraDataLen = bound(_extraDataLen, 0, 23_500);
        if (_extraDataLen == 32) {
            _extraDataLen++;
        }
        bytes memory _extraData = new bytes(_extraDataLen);

        // Assign the first 32 bytes in `extraData` to a valid L2 block number passed the starting block.
        (, uint256 startingL2Block) = gameProxy.startingOutputRoot();
        assembly {
            mstore(add(_extraData, 0x20), add(startingL2Block, 1))
        }

        Claim claim = _dummyClaim();
        vm.expectRevert(abi.encodeWithSelector(BadExtraData.selector));
        gameProxy = IFaultDisputeGame(payable(address(disputeGameFactory.create(GAME_TYPE, claim, _extraData))));
    }

    /// @dev Tests that the game is initialized with the correct data.
    function test_initialize_correctData_succeeds() public view {
        // Assert that the root claim is initialized correctly.
        (
            uint32 parentIndex,
            address counteredBy,
            address claimant,
            uint128 bond,
            Claim claim,
            Position position,
            Clock clock
        ) = gameProxy.claimData(0);
        assertEq(parentIndex, type(uint32).max);
        assertEq(counteredBy, address(0));
        assertEq(claimant, address(this));
        assertEq(bond, 0);
        assertEq(claim.raw(), ROOT_CLAIM.raw());
        assertEq(position.raw(), 1);
        assertEq(clock.raw(), LibClock.wrap(Duration.wrap(0), Timestamp.wrap(uint64(block.timestamp))).raw());

        // Assert that the `createdAt` timestamp is correct.
        assertEq(gameProxy.createdAt().raw(), block.timestamp);

        // Assert that the blockhash provided is correct.
        assertEq(gameProxy.l1Head().raw(), blockhash(block.number - 1));
    }

    /// @dev Tests that the game cannot be initialized twice.
    function test_initialize_onlyOnce_succeeds() public {
        vm.expectRevert(AlreadyInitialized.selector);
        gameProxy.initialize();
    }

    /// @dev Tests that the user cannot control the first 4 bytes of the CWIA data, disallowing them to control the
    ///      entrypoint when no calldata is provided to a call.
    function test_cwiaCalldata_userCannotControlSelector_succeeds() public {
        // Construct the expected CWIA data that the proxy will pass to the implementation, alongside any extra
        // calldata passed by the user.
        Hash l1Head = gameProxy.l1Head();
        bytes memory cwiaData = abi.encodePacked(address(this), gameProxy.rootClaim(), l1Head, gameProxy.extraData());

        // We expect a `ReceiveETH` event to be emitted when 0 bytes of calldata are sent; The fallback is always
        // reached *within the minimal proxy* in `LibClone`'s version of `clones-with-immutable-args`
        vm.expectEmit(false, false, false, true);
        emit ReceiveETH(0);
        // We expect no delegatecall to the implementation contract if 0 bytes are sent. Assert that this happens
        // 0 times.
        vm.expectCall(address(gameImpl), cwiaData, 0);
        (bool successA,) = address(gameProxy).call(hex"");
        assertTrue(successA);

        // When calldata is forwarded, we do expect a delegatecall to the implementation.
        bytes memory data = abi.encodePacked(gameProxy.l1Head.selector);
        vm.expectCall(address(gameImpl), abi.encodePacked(data, cwiaData), 1);
        (bool successB, bytes memory returnData) = address(gameProxy).call(data);
        assertTrue(successB);
        assertEq(returnData, abi.encode(l1Head));
    }

    /// @dev Tests that the bond during the bisection game depths is correct.
    function test_getRequiredBond_succeeds() public view {
        for (uint8 i = 0; i < uint8(gameProxy.splitDepth()); i++) {
            Position pos = LibPosition.wrap(i, 0);
            uint256 bond = gameProxy.getRequiredBond(pos);

            // Reasonable approximation for a max depth of 8.
            uint256 expected = 0.08 ether;
            for (uint64 j = 0; j < i; j++) {
                expected = expected * 22876;
                expected = expected / 10000;
            }

            assertApproxEqAbs(bond, expected, 0.01 ether);
        }
    }

    /// @dev Tests that the bond at a depth greater than the maximum game depth reverts.
    function test_getRequiredBond_outOfBounds_reverts() public {
        Position pos = LibPosition.wrap(uint8(gameProxy.maxGameDepth() + 1), 0);
        vm.expectRevert(GameDepthExceeded.selector);
        gameProxy.getRequiredBond(pos);
    }

    /// @dev Tests that a move while the game status is not `IN_PROGRESS` causes the call to revert
    ///      with the `GameNotInProgress` error
    function test_move_gameNotInProgress_reverts() public {
        uint256 chalWins = uint256(GameStatus.CHALLENGER_WINS);

        // Replace the game status in storage. It exists in slot 0 at offset 16.
        uint256 slot = uint256(vm.load(address(gameProxy), bytes32(0)));
        uint256 offset = 16 << 3;
        uint256 mask = 0xFF << offset;
        // Replace the byte in the slot value with the challenger wins status.
        slot = (slot & ~mask) | (chalWins << offset);
        vm.store(address(gameProxy), bytes32(0), bytes32(slot));

        // Ensure that the game status was properly updated.
        GameStatus status = gameProxy.status();
        assertEq(uint256(status), chalWins);

        (,,,, Claim root,,) = gameProxy.claimData(0);
        // Attempt to make a move. Should revert.
        vm.expectRevert(GameNotInProgress.selector);
        gameProxy.attack(root, 0, Claim.wrap(0));
    }

    /// @dev Tests that an attempt to defend the root claim reverts with the `CannotDefendRootClaim` error.
    function test_move_defendRoot_reverts() public {
        (,,,, Claim root,,) = gameProxy.claimData(0);
        vm.expectRevert(CannotDefendRootClaim.selector);
        gameProxy.defend(root, 0, _dummyClaim());
    }

    /// @dev Tests that an attempt to move against a claim that does not exist reverts with the
    ///      `ParentDoesNotExist` error.
    function test_move_nonExistentParent_reverts() public {
        Claim claim = _dummyClaim();

        // Expect an out of bounds revert for an attack
        vm.expectRevert(stdError.indexOOBError);
        gameProxy.attack(_dummyClaim(), 1, claim);

        // Expect an out of bounds revert for a defense
        vm.expectRevert(stdError.indexOOBError);
        gameProxy.defend(_dummyClaim(), 1, claim);
    }

    /// @dev Tests that an attempt to move at the maximum game depth reverts with the
    ///      `GameDepthExceeded` error.
    function test_move_gameDepthExceeded_reverts() public {
        Claim claim = _changeClaimStatus(_dummyClaim(), VMStatuses.PANIC);

        uint256 maxDepth = gameProxy.maxGameDepth();

        for (uint256 i = 0; i <= maxDepth; i++) {
            (,,,, Claim disputed,,) = gameProxy.claimData(i);
            // At the max game depth, the `_move` function should revert with
            // the `GameDepthExceeded` error.
            if (i == maxDepth) {
                vm.expectRevert(GameDepthExceeded.selector);
                gameProxy.attack{ value: 100 ether }(disputed, i, claim);
            } else {
                gameProxy.attack{ value: _getRequiredBond(i) }(disputed, i, claim);
            }
        }
    }

    /// @dev Tests that a move made after the clock time has exceeded reverts with the
    ///      `ClockTimeExceeded` error.
    function test_move_clockTimeExceeded_reverts() public {
        // Warp ahead past the clock time for the first move (3 1/2 days)
        vm.warp(block.timestamp + 3 days + 12 hours + 1);
        uint256 bond = _getRequiredBond(0);
        (,,,, Claim disputed,,) = gameProxy.claimData(0);
        vm.expectRevert(ClockTimeExceeded.selector);
        gameProxy.attack{ value: bond }(disputed, 0, _dummyClaim());
    }

    /// @notice Static unit test for the correctness of the chess clock incrementation.
    function test_move_clockCorrectness_succeeds() public {
        (,,,,,, Clock clock) = gameProxy.claimData(0);
        assertEq(clock.raw(), LibClock.wrap(Duration.wrap(0), Timestamp.wrap(uint64(block.timestamp))).raw());

        Claim claim = _dummyClaim();

        vm.warp(block.timestamp + 15);
        uint256 bond = _getRequiredBond(0);
        (,,,, Claim disputed,,) = gameProxy.claimData(0);
        gameProxy.attack{ value: bond }(disputed, 0, claim);
        (,,,,,, clock) = gameProxy.claimData(1);
        assertEq(clock.raw(), LibClock.wrap(Duration.wrap(15), Timestamp.wrap(uint64(block.timestamp))).raw());

        vm.warp(block.timestamp + 10);
        bond = _getRequiredBond(1);
        (,,,, disputed,,) = gameProxy.claimData(1);
        gameProxy.attack{ value: bond }(disputed, 1, claim);
        (,,,,,, clock) = gameProxy.claimData(2);
        assertEq(clock.raw(), LibClock.wrap(Duration.wrap(10), Timestamp.wrap(uint64(block.timestamp))).raw());

        // We are at the split depth, so we need to set the status byte of the claim
        // for the next move.
        claim = _changeClaimStatus(claim, VMStatuses.PANIC);

        vm.warp(block.timestamp + 10);
        bond = _getRequiredBond(2);
        (,,,, disputed,,) = gameProxy.claimData(2);
        gameProxy.attack{ value: bond }(disputed, 2, claim);
        (,,,,,, clock) = gameProxy.claimData(3);
        assertEq(clock.raw(), LibClock.wrap(Duration.wrap(25), Timestamp.wrap(uint64(block.timestamp))).raw());

        vm.warp(block.timestamp + 10);
        bond = _getRequiredBond(3);
        (,,,, disputed,,) = gameProxy.claimData(3);
        gameProxy.attack{ value: bond }(disputed, 3, claim);
        (,,,,,, clock) = gameProxy.claimData(4);
        assertEq(clock.raw(), LibClock.wrap(Duration.wrap(20), Timestamp.wrap(uint64(block.timestamp))).raw());
    }

    /// @dev Tests that the standard clock extension is triggered for a move that is not the
    ///      split depth or the max game depth.
    function test_move_standardClockExtension_succeeds() public {
        (,,,,,, Clock clock) = gameProxy.claimData(0);
        assertEq(clock.raw(), LibClock.wrap(Duration.wrap(0), Timestamp.wrap(uint64(block.timestamp))).raw());

        uint256 bond;
        Claim disputed;
        Claim claim = _dummyClaim();
        uint256 splitDepth = gameProxy.splitDepth();
        uint64 halfGameDuration = gameProxy.maxClockDuration().raw();
        uint64 clockExtension = gameProxy.clockExtension().raw();

        // Warp ahead so that the next move will trigger a clock extension. We warp to the very
        // first timestamp where a clock extension should be triggered.
        vm.warp(block.timestamp + halfGameDuration - clockExtension + 1 seconds);

        // Execute a move that should cause a clock extension.
        bond = _getRequiredBond(0);
        (,,,, disputed,,) = gameProxy.claimData(0);
        gameProxy.attack{ value: bond }(disputed, 0, claim);
        (,,,,,, clock) = gameProxy.claimData(1);

        // The clock should have been pushed back to the clock extension time.
        assertEq(clock.duration().raw(), halfGameDuration - clockExtension);

        // Warp ahead again so that clock extensions will also trigger for the other team. Here we
        // only warp to the clockExtension time because we'll be warping ahead by one second during
        // each additional move.
        vm.warp(block.timestamp + halfGameDuration - clockExtension);

        // Work our way down to the split depth.
        for (uint256 i = 1; i < splitDepth - 2; i++) {
            // Warp ahead by one second so that the next move will trigger a clock extension.
            vm.warp(block.timestamp + 1 seconds);

            // Execute a move that should cause a clock extension.
            bond = _getRequiredBond(i);
            (,,,, disputed,,) = gameProxy.claimData(i);
            gameProxy.attack{ value: bond }(disputed, i, claim);
            (,,,,,, clock) = gameProxy.claimData(i + 1);

            // The clock should have been pushed back to the clock extension time.
            assertEq(clock.duration().raw(), halfGameDuration - clockExtension);
        }
    }

    function test_move_splitDepthClockExtension_succeeds() public {
        (,,,,,, Clock clock) = gameProxy.claimData(0);
        assertEq(clock.raw(), LibClock.wrap(Duration.wrap(0), Timestamp.wrap(uint64(block.timestamp))).raw());

        uint256 bond;
        Claim disputed;
        Claim claim = _dummyClaim();
        uint256 splitDepth = gameProxy.splitDepth();
        uint64 halfGameDuration = gameProxy.maxClockDuration().raw();
        uint64 clockExtension = gameProxy.clockExtension().raw();

        // Work our way down to the split depth without moving ahead in time, we don't care about
        // the exact clock here, just don't want take the clock below the clock extension time that
        // we're trying to test here.
        for (uint256 i = 0; i < splitDepth - 2; i++) {
            bond = _getRequiredBond(i);
            (,,,, disputed,,) = gameProxy.claimData(i);
            gameProxy.attack{ value: bond }(disputed, i, claim);
        }

        // Warp ahead to the very first timestamp where a clock extension should be triggered.
        vm.warp(block.timestamp + halfGameDuration - clockExtension * 2 + 1 seconds);

        // Execute a move that should cause a clock extension.
        bond = _getRequiredBond(splitDepth - 2);
        (,,,, disputed,,) = gameProxy.claimData(splitDepth - 2);
        gameProxy.attack{ value: bond }(disputed, splitDepth - 2, claim);
        (,,,,,, clock) = gameProxy.claimData(splitDepth - 1);

        // The clock should have been pushed back to the clock extension time.
        assertEq(clock.duration().raw(), halfGameDuration - clockExtension * 2);
    }

    function test_move_maxGameDepthClockExtension_succeeds() public {
        (,,,,,, Clock clock) = gameProxy.claimData(0);
        assertEq(clock.raw(), LibClock.wrap(Duration.wrap(0), Timestamp.wrap(uint64(block.timestamp))).raw());

        uint256 bond;
        Claim disputed;
        Claim claim = _dummyClaim();
        uint256 splitDepth = gameProxy.splitDepth();
        uint64 halfGameDuration = gameProxy.maxClockDuration().raw();
        uint64 clockExtension = gameProxy.clockExtension().raw();

        // Work our way down to the split depth without moving ahead in time, we don't care about
        // the exact clock here, just don't want take the clock below the clock extension time that
        // we're trying to test here.
        for (uint256 i = 0; i < gameProxy.maxGameDepth() - 2; i++) {
            bond = _getRequiredBond(i);
            (,,,, disputed,,) = gameProxy.claimData(i);
            gameProxy.attack{ value: bond }(disputed, i, claim);

            // Change the claim status when we're crossing the split depth.
            if (i == splitDepth - 2) {
                claim = _changeClaimStatus(claim, VMStatuses.PANIC);
            }
        }

        // Warp ahead to the very first timestamp where a clock extension should be triggered.
        vm.warp(block.timestamp + halfGameDuration - (clockExtension + gameProxy.vm().oracle().challengePeriod()) + 1);

        // Execute a move that should cause a clock extension.
        bond = _getRequiredBond(gameProxy.maxGameDepth() - 2);
        (,,,, disputed,,) = gameProxy.claimData(gameProxy.maxGameDepth() - 2);
        gameProxy.attack{ value: bond }(disputed, gameProxy.maxGameDepth() - 2, claim);
        (,,,,,, clock) = gameProxy.claimData(gameProxy.maxGameDepth() - 1);

        // The clock should have been pushed back to the clock extension time.
        assertEq(
            clock.duration().raw(), halfGameDuration - (clockExtension + gameProxy.vm().oracle().challengePeriod())
        );
    }

    /// @dev Tests that an identical claim cannot be made twice. The duplicate claim attempt should
    ///      revert with the `ClaimAlreadyExists` error.
    function test_move_duplicateClaim_reverts() public {
        Claim claim = _dummyClaim();

        // Make the first move. This should succeed.
        uint256 bond = _getRequiredBond(0);
        (,,,, Claim disputed,,) = gameProxy.claimData(0);
        gameProxy.attack{ value: bond }(disputed, 0, claim);

        // Attempt to make the same move again.
        vm.expectRevert(ClaimAlreadyExists.selector);
        gameProxy.attack{ value: bond }(disputed, 0, claim);
    }

    /// @dev Static unit test asserting that identical claims at the same position can be made in different subgames.
    function test_move_duplicateClaimsDifferentSubgames_succeeds() public {
        Claim claimA = _dummyClaim();
        Claim claimB = _dummyClaim();

        // Make the first moves. This should succeed.
        uint256 bond = _getRequiredBond(0);
        (,,,, Claim disputed,,) = gameProxy.claimData(0);
        gameProxy.attack{ value: bond }(disputed, 0, claimA);
        gameProxy.attack{ value: bond }(disputed, 0, claimB);

        // Perform an attack at the same position with the same claim value in both subgames.
        // These both should succeed.
        bond = _getRequiredBond(1);
        (,,,, disputed,,) = gameProxy.claimData(1);
        gameProxy.attack{ value: bond }(disputed, 1, claimA);
        bond = _getRequiredBond(2);
        (,,,, disputed,,) = gameProxy.claimData(2);
        gameProxy.attack{ value: bond }(disputed, 2, claimA);
    }

    /// @dev Static unit test for the correctness of an opening attack.
    function test_move_simpleAttack_succeeds() public {
        // Warp ahead 5 seconds.
        vm.warp(block.timestamp + 5);

        Claim counter = _dummyClaim();

        // Perform the attack.
        uint256 reqBond = _getRequiredBond(0);
        vm.expectEmit(true, true, true, false);
        emit Move(0, counter, address(this));
        (,,,, Claim disputed,,) = gameProxy.claimData(0);
        gameProxy.attack{ value: reqBond }(disputed, 0, counter);

        // Grab the claim data of the attack.
        (
            uint32 parentIndex,
            address counteredBy,
            address claimant,
            uint128 bond,
            Claim claim,
            Position position,
            Clock clock
        ) = gameProxy.claimData(1);

        // Assert correctness of the attack claim's data.
        assertEq(parentIndex, 0);
        assertEq(counteredBy, address(0));
        assertEq(claimant, address(this));
        assertEq(bond, reqBond);
        assertEq(claim.raw(), counter.raw());
        assertEq(position.raw(), Position.wrap(1).move(true).raw());
        assertEq(clock.raw(), LibClock.wrap(Duration.wrap(5), Timestamp.wrap(uint64(block.timestamp))).raw());

        // Grab the claim data of the parent.
        (parentIndex, counteredBy, claimant, bond, claim, position, clock) = gameProxy.claimData(0);

        // Assert correctness of the parent claim's data.
        assertEq(parentIndex, type(uint32).max);
        assertEq(counteredBy, address(0));
        assertEq(claimant, address(this));
        assertEq(bond, 0);
        assertEq(claim.raw(), ROOT_CLAIM.raw());
        assertEq(position.raw(), 1);
        assertEq(clock.raw(), LibClock.wrap(Duration.wrap(0), Timestamp.wrap(uint64(block.timestamp - 5))).raw());
    }

    /// @dev Tests that making a claim at the execution trace bisection root level with an invalid status
    ///      byte reverts with the `UnexpectedRootClaim` error.
    function test_move_incorrectStatusExecRoot_reverts() public {
        Claim disputed;
        for (uint256 i; i < 4; i++) {
            (,,,, disputed,,) = gameProxy.claimData(i);
            gameProxy.attack{ value: _getRequiredBond(i) }(disputed, i, _dummyClaim());
        }

        uint256 bond = _getRequiredBond(4);
        (,,,, disputed,,) = gameProxy.claimData(4);
        vm.expectRevert(abi.encodeWithSelector(UnexpectedRootClaim.selector, bytes32(0)));
        gameProxy.attack{ value: bond }(disputed, 4, Claim.wrap(bytes32(0)));
    }

    /// @dev Tests that making a claim at the execution trace bisection root level with a valid status
    ///      byte succeeds.
    function test_move_correctStatusExecRoot_succeeds() public {
        Claim disputed;
        for (uint256 i; i < 4; i++) {
            uint256 bond = _getRequiredBond(i);
            (,,,, disputed,,) = gameProxy.claimData(i);
            gameProxy.attack{ value: bond }(disputed, i, _dummyClaim());
        }
        uint256 lastBond = _getRequiredBond(4);
        (,,,, disputed,,) = gameProxy.claimData(4);
        gameProxy.attack{ value: lastBond }(disputed, 4, _changeClaimStatus(_dummyClaim(), VMStatuses.PANIC));
    }

    /// @dev Static unit test asserting that a move reverts when the bonded amount is incorrect.
    function test_move_incorrectBondAmount_reverts() public {
        (,,,, Claim disputed,,) = gameProxy.claimData(0);
        vm.expectRevert(IncorrectBondAmount.selector);
        gameProxy.attack{ value: 0 }(disputed, 0, _dummyClaim());
    }

    /// @dev Static unit test asserting that a move reverts when the disputed claim does not match its index.
    function test_move_incorrectDisputedIndex_reverts() public {
        (,,,, Claim disputed,,) = gameProxy.claimData(0);
        gameProxy.attack{ value: _getRequiredBond(0) }(disputed, 0, _dummyClaim());
        uint256 bond = _getRequiredBond(1);
        vm.expectRevert(InvalidDisputedClaimIndex.selector);
        gameProxy.attack{ value: bond }(disputed, 1, _dummyClaim());
    }

    /// @dev Tests that challenging the root claim's L2 block number by providing the real preimage of the output root
    ///      succeeds.
    function testFuzz_challengeRootL2Block_succeeds(
        bytes32 _storageRoot,
        bytes32 _withdrawalRoot,
        uint256 _l2BlockNumber
    )
        public
    {
        _l2BlockNumber = bound(_l2BlockNumber, 0, type(uint256).max - 1);

        (Types.OutputRootProof memory outputRootProof, bytes32 outputRoot, bytes memory headerRLP) =
            _generateOutputRootProof(_storageRoot, _withdrawalRoot, abi.encodePacked(_l2BlockNumber));

        // Create the dispute game with the output root at the wrong L2 block number.
        IDisputeGame game = disputeGameFactory.create(GAME_TYPE, Claim.wrap(outputRoot), abi.encode(_l2BlockNumber + 1));

        // Challenge the L2 block number.
        IFaultDisputeGame fdg = IFaultDisputeGame(address(game));
        fdg.challengeRootL2Block(outputRootProof, headerRLP);

        // Ensure that a duplicate challenge reverts.
        vm.expectRevert(L2BlockNumberChallenged.selector);
        fdg.challengeRootL2Block(outputRootProof, headerRLP);

        // Warp past the clocks, resolve the game.
        vm.warp(block.timestamp + 3 days + 12 hours + 1);
        fdg.resolveClaim(0, 0);
        fdg.resolve();

        // Ensure the challenge was successful.
        assertEq(uint8(fdg.status()), uint8(GameStatus.CHALLENGER_WINS));
        assertTrue(fdg.l2BlockNumberChallenged());
    }

    /// @dev Tests that challenging the root claim's L2 block number by providing the real preimage of the output root
    ///      succeeds. Also, this claim should always receive the bond when there is another counter that is as far left
    ///      as possible.
    function testFuzz_challengeRootL2Block_receivesBond_succeeds(
        bytes32 _storageRoot,
        bytes32 _withdrawalRoot,
        uint256 _l2BlockNumber
    )
        public
    {
        vm.deal(address(0xb0b), 1 ether);
        _l2BlockNumber = bound(_l2BlockNumber, 0, type(uint256).max - 1);

        (Types.OutputRootProof memory outputRootProof, bytes32 outputRoot, bytes memory headerRLP) =
            _generateOutputRootProof(_storageRoot, _withdrawalRoot, abi.encodePacked(_l2BlockNumber));

        // Create the dispute game with the output root at the wrong L2 block number.
        disputeGameFactory.setInitBond(GAME_TYPE, 0.1 ether);
        uint256 balanceBefore = address(this).balance;
        IDisputeGame game = disputeGameFactory.create{ value: 0.1 ether }(
            GAME_TYPE, Claim.wrap(outputRoot), abi.encode(_l2BlockNumber + 1)
        );
        IFaultDisputeGame fdg = IFaultDisputeGame(address(game));

        // Attack the root as 0xb0b
        uint256 bond = _getRequiredBond(0);
        (,,,, Claim disputed,,) = fdg.claimData(0);
        vm.prank(address(0xb0b));
        fdg.attack{ value: bond }(disputed, 0, Claim.wrap(0));

        // Challenge the L2 block number as 0xace. This claim should receive the root claim's bond.
        vm.prank(address(0xace));
        fdg.challengeRootL2Block(outputRootProof, headerRLP);

        // Warp past the clocks, resolve the game.
        vm.warp(block.timestamp + 3 days + 12 hours + 1);
        fdg.resolveClaim(1, 0);
        fdg.resolveClaim(0, 0);
        fdg.resolve();

        // Ensure the challenge was successful.
        assertEq(uint8(fdg.status()), uint8(GameStatus.CHALLENGER_WINS));

        // Wait for the withdrawal delay.
        vm.warp(block.timestamp + delayedWeth.delay() + 1 seconds);

        // Claim credit
        vm.expectRevert(NoCreditToClaim.selector);
        fdg.claimCredit(address(this));
        fdg.claimCredit(address(0xb0b));
        fdg.claimCredit(address(0xace));

        // Ensure that the party who challenged the L2 block number with the special move received the bond.
        // - Root claim loses their bond
        // - 0xace receives the root claim's bond
        // - 0xb0b receives their bond back
        assertEq(address(this).balance, balanceBefore - 0.1 ether);
        assertEq(address(0xb0b).balance, 1 ether);
        assertEq(address(0xace).balance, 0.1 ether);
    }

    /// @dev Tests that challenging the root claim's L2 block number by providing the real preimage of the output root
    ///      never succeeds.
    function testFuzz_challengeRootL2Block_rightBlockNumber_reverts(
        bytes32 _storageRoot,
        bytes32 _withdrawalRoot,
        uint256 _l2BlockNumber
    )
        public
    {
        _l2BlockNumber = bound(_l2BlockNumber, 1, type(uint256).max);

        (Types.OutputRootProof memory outputRootProof, bytes32 outputRoot, bytes memory headerRLP) =
            _generateOutputRootProof(_storageRoot, _withdrawalRoot, abi.encodePacked(_l2BlockNumber));

        // Create the dispute game with the output root at the wrong L2 block number.
        IDisputeGame game = disputeGameFactory.create(GAME_TYPE, Claim.wrap(outputRoot), abi.encode(_l2BlockNumber));

        // Challenge the L2 block number.
        IFaultDisputeGame fdg = IFaultDisputeGame(address(game));
        vm.expectRevert(BlockNumberMatches.selector);
        fdg.challengeRootL2Block(outputRootProof, headerRLP);

        // Warp past the clocks, resolve the game.
        vm.warp(block.timestamp + 3 days + 12 hours + 1);
        fdg.resolveClaim(0, 0);
        fdg.resolve();

        // Ensure the challenge was successful.
        assertEq(uint8(fdg.status()), uint8(GameStatus.DEFENDER_WINS));
    }

    /// @dev Tests that challenging the root claim's L2 block number with a bad output root proof reverts.
    function test_challengeRootL2Block_badProof_reverts() public {
        Types.OutputRootProof memory outputRootProof =
            Types.OutputRootProof({ version: 0, stateRoot: 0, messagePasserStorageRoot: 0, latestBlockhash: 0 });

        vm.expectRevert(InvalidOutputRootProof.selector);
        gameProxy.challengeRootL2Block(outputRootProof, hex"");
    }

    /// @dev Tests that challenging the root claim's L2 block number with a bad output root proof reverts.
    function test_challengeRootL2Block_badHeaderRLP_reverts() public {
        Types.OutputRootProof memory outputRootProof =
            Types.OutputRootProof({ version: 0, stateRoot: 0, messagePasserStorageRoot: 0, latestBlockhash: 0 });
        bytes32 outputRoot = Hashing.hashOutputRootProof(outputRootProof);

        // Create the dispute game with the output root at the wrong L2 block number.
        IDisputeGame game = disputeGameFactory.create(GAME_TYPE, Claim.wrap(outputRoot), abi.encode(1));
        IFaultDisputeGame fdg = IFaultDisputeGame(address(game));

        vm.expectRevert(InvalidHeaderRLP.selector);
        fdg.challengeRootL2Block(outputRootProof, hex"");
    }

    /// @dev Tests that challenging the root claim's L2 block number with a bad output root proof reverts.
    function test_challengeRootL2Block_badHeaderRLPBlockNumberLength_reverts() public {
        (Types.OutputRootProof memory outputRootProof, bytes32 outputRoot,) =
            _generateOutputRootProof(0, 0, new bytes(64));

        // Create the dispute game with the output root at the wrong L2 block number.
        IDisputeGame game = disputeGameFactory.create(GAME_TYPE, Claim.wrap(outputRoot), abi.encode(1));
        IFaultDisputeGame fdg = IFaultDisputeGame(address(game));

        vm.expectRevert(InvalidHeaderRLP.selector);
        fdg.challengeRootL2Block(outputRootProof, hex"");
    }

    /// @dev Tests that a claim cannot be stepped against twice.
    function test_step_duplicateStep_reverts() public {
        // Give the test contract some ether
        vm.deal(address(this), 1000 ether);

        // Make claims all the way down the tree.
        (,,,, Claim disputed,,) = gameProxy.claimData(0);
        gameProxy.attack{ value: _getRequiredBond(0) }(disputed, 0, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(1);
        gameProxy.attack{ value: _getRequiredBond(1) }(disputed, 1, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(2);
        gameProxy.attack{ value: _getRequiredBond(2) }(disputed, 2, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(3);
        gameProxy.attack{ value: _getRequiredBond(3) }(disputed, 3, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(4);
        gameProxy.attack{ value: _getRequiredBond(4) }(disputed, 4, _changeClaimStatus(_dummyClaim(), VMStatuses.PANIC));
        (,,,, disputed,,) = gameProxy.claimData(5);
        gameProxy.attack{ value: _getRequiredBond(5) }(disputed, 5, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(6);
        gameProxy.attack{ value: _getRequiredBond(6) }(disputed, 6, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(7);
        gameProxy.attack{ value: _getRequiredBond(7) }(disputed, 7, _dummyClaim());
        gameProxy.addLocalData(LocalPreimageKey.DISPUTED_L2_BLOCK_NUMBER, 8, 0);
        gameProxy.step(8, true, absolutePrestateData, hex"");

        vm.expectRevert(DuplicateStep.selector);
        gameProxy.step(8, true, absolutePrestateData, hex"");
    }

    /// @dev Tests that successfully step with true attacking claim when there is a true defend claim(claim5) in the
    /// middle of the dispute game.
    function test_stepAttackDummyClaim_defendTrueClaimInTheMiddle_succeeds() public {
        // Give the test contract some ether
        vm.deal(address(this), 1000 ether);

        // Make claims all the way down the tree.
        (,,,, Claim disputed,,) = gameProxy.claimData(0);
        gameProxy.attack{ value: _getRequiredBond(0) }(disputed, 0, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(1);
        gameProxy.attack{ value: _getRequiredBond(1) }(disputed, 1, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(2);
        gameProxy.attack{ value: _getRequiredBond(2) }(disputed, 2, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(3);
        gameProxy.attack{ value: _getRequiredBond(3) }(disputed, 3, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(4);
        gameProxy.attack{ value: _getRequiredBond(4) }(disputed, 4, _changeClaimStatus(_dummyClaim(), VMStatuses.PANIC));
        bytes memory claimData5 = abi.encode(5, 5);
        Claim claim5 = Claim.wrap(keccak256(claimData5));
        (,,,, disputed,,) = gameProxy.claimData(5);
        gameProxy.attack{ value: _getRequiredBond(5) }(disputed, 5, claim5);
        (,,,, disputed,,) = gameProxy.claimData(6);
        gameProxy.defend{ value: _getRequiredBond(6) }(disputed, 6, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(7);
        gameProxy.attack{ value: _getRequiredBond(7) }(disputed, 7, _dummyClaim());
        gameProxy.addLocalData(LocalPreimageKey.DISPUTED_L2_BLOCK_NUMBER, 8, 0);
        gameProxy.step(8, true, claimData5, hex"");
    }

    /// @dev Tests that successfully step with true defend claim when there is a true defend claim(claim7) in the
    /// middle of the dispute game.
    function test_stepDefendDummyClaim_defendTrueClaimInTheMiddle_succeeds() public {
        // Give the test contract some ether
        vm.deal(address(this), 1000 ether);

        // Make claims all the way down the tree.
        (,,,, Claim disputed,,) = gameProxy.claimData(0);
        gameProxy.attack{ value: _getRequiredBond(0) }(disputed, 0, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(1);
        gameProxy.attack{ value: _getRequiredBond(1) }(disputed, 1, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(2);
        gameProxy.attack{ value: _getRequiredBond(2) }(disputed, 2, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(3);
        gameProxy.attack{ value: _getRequiredBond(3) }(disputed, 3, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(4);
        gameProxy.attack{ value: _getRequiredBond(4) }(disputed, 4, _changeClaimStatus(_dummyClaim(), VMStatuses.PANIC));

        bytes memory claimData7 = abi.encode(7, 7);
        Claim postState_ = Claim.wrap(gameImpl.vm().step(claimData7, hex"", bytes32(0)));

        (,,,, disputed,,) = gameProxy.claimData(5);
        gameProxy.attack{ value: _getRequiredBond(5) }(disputed, 5, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(6);
        gameProxy.defend{ value: _getRequiredBond(6) }(disputed, 6, postState_);
        (,,,, disputed,,) = gameProxy.claimData(7);

        gameProxy.attack{ value: _getRequiredBond(7) }(disputed, 7, Claim.wrap(keccak256(claimData7)));
        gameProxy.addLocalData(LocalPreimageKey.DISPUTED_L2_BLOCK_NUMBER, 8, 0);
        gameProxy.step(8, false, claimData7, hex"");
    }

    /// @dev Tests that step reverts with false attacking claim when there is a true defend claim(claim5) in the middle
    /// of the dispute game.
    function test_stepAttackTrueClaim_defendTrueClaimInTheMiddle_reverts() public {
        // Give the test contract some ether
        vm.deal(address(this), 1000 ether);

        // Make claims all the way down the tree.
        (,,,, Claim disputed,,) = gameProxy.claimData(0);
        gameProxy.attack{ value: _getRequiredBond(0) }(disputed, 0, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(1);
        gameProxy.attack{ value: _getRequiredBond(1) }(disputed, 1, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(2);
        gameProxy.attack{ value: _getRequiredBond(2) }(disputed, 2, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(3);
        gameProxy.attack{ value: _getRequiredBond(3) }(disputed, 3, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(4);
        gameProxy.attack{ value: _getRequiredBond(4) }(disputed, 4, _changeClaimStatus(_dummyClaim(), VMStatuses.PANIC));
        bytes memory claimData5 = abi.encode(5, 5);
        Claim claim5 = Claim.wrap(keccak256(claimData5));
        (,,,, disputed,,) = gameProxy.claimData(5);
        gameProxy.attack{ value: _getRequiredBond(5) }(disputed, 5, claim5);
        (,,,, disputed,,) = gameProxy.claimData(6);
        gameProxy.defend{ value: _getRequiredBond(6) }(disputed, 6, _dummyClaim());
        Claim postState_ = Claim.wrap(gameImpl.vm().step(claimData5, hex"", bytes32(0)));
        (,,,, disputed,,) = gameProxy.claimData(7);
        gameProxy.attack{ value: _getRequiredBond(7) }(disputed, 7, postState_);
        gameProxy.addLocalData(LocalPreimageKey.DISPUTED_L2_BLOCK_NUMBER, 8, 0);

        vm.expectRevert(ValidStep.selector);
        gameProxy.step(8, true, claimData5, hex"");
    }

    /// @dev Tests that step reverts with false defending claim when there is a true defend claim(postState_) in the
    /// middle of the dispute game.
    function test_stepDefendDummyClaim_defendTrueClaimInTheMiddle_reverts() public {
        // Give the test contract some ether
        vm.deal(address(this), 1000 ether);

        // Make claims all the way down the tree.
        (,,,, Claim disputed,,) = gameProxy.claimData(0);
        gameProxy.attack{ value: _getRequiredBond(0) }(disputed, 0, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(1);
        gameProxy.attack{ value: _getRequiredBond(1) }(disputed, 1, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(2);
        gameProxy.attack{ value: _getRequiredBond(2) }(disputed, 2, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(3);
        gameProxy.attack{ value: _getRequiredBond(3) }(disputed, 3, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(4);
        gameProxy.attack{ value: _getRequiredBond(4) }(disputed, 4, _changeClaimStatus(_dummyClaim(), VMStatuses.PANIC));

        bytes memory claimData7 = abi.encode(5, 5);
        Claim postState_ = Claim.wrap(gameImpl.vm().step(claimData7, hex"", bytes32(0)));

        (,,,, disputed,,) = gameProxy.claimData(5);
        gameProxy.attack{ value: _getRequiredBond(5) }(disputed, 5, postState_);
        (,,,, disputed,,) = gameProxy.claimData(6);
        gameProxy.defend{ value: _getRequiredBond(6) }(disputed, 6, _dummyClaim());

        bytes memory _dummyClaimData = abi.encode(gasleft(), gasleft());
        Claim dummyClaim7 = Claim.wrap(keccak256(_dummyClaimData));
        (,,,, disputed,,) = gameProxy.claimData(7);
        gameProxy.attack{ value: _getRequiredBond(7) }(disputed, 7, dummyClaim7);
        gameProxy.addLocalData(LocalPreimageKey.DISPUTED_L2_BLOCK_NUMBER, 8, 0);
        vm.expectRevert(ValidStep.selector);
        gameProxy.step(8, false, _dummyClaimData, hex"");
    }

    /// @dev Tests that step reverts with true defending claim when there is a true defend claim(postState_) in the
    /// middle of the dispute game.
    function test_stepDefendTrueClaim_defendTrueClaimInTheMiddle_reverts() public {
        // Give the test contract some ether
        vm.deal(address(this), 1000 ether);

        // Make claims all the way down the tree.
        (,,,, Claim disputed,,) = gameProxy.claimData(0);
        gameProxy.attack{ value: _getRequiredBond(0) }(disputed, 0, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(1);
        gameProxy.attack{ value: _getRequiredBond(1) }(disputed, 1, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(2);
        gameProxy.attack{ value: _getRequiredBond(2) }(disputed, 2, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(3);
        gameProxy.attack{ value: _getRequiredBond(3) }(disputed, 3, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(4);
        gameProxy.attack{ value: _getRequiredBond(4) }(disputed, 4, _changeClaimStatus(_dummyClaim(), VMStatuses.PANIC));

        bytes memory claimData7 = abi.encode(5, 5);
        Claim claim7 = Claim.wrap(keccak256(claimData7));
        Claim postState_ = Claim.wrap(gameImpl.vm().step(claimData7, hex"", bytes32(0)));

        (,,,, disputed,,) = gameProxy.claimData(5);
        gameProxy.attack{ value: _getRequiredBond(5) }(disputed, 5, postState_);
        (,,,, disputed,,) = gameProxy.claimData(6);
        gameProxy.defend{ value: _getRequiredBond(6) }(disputed, 6, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(7);
        gameProxy.attack{ value: _getRequiredBond(7) }(disputed, 7, claim7);
        gameProxy.addLocalData(LocalPreimageKey.DISPUTED_L2_BLOCK_NUMBER, 8, 0);

        vm.expectRevert(ValidStep.selector);
        gameProxy.step(8, false, claimData7, hex"");
    }

    /// @dev Static unit test for the correctness an uncontested root resolution.
    function test_resolve_rootUncontested_succeeds() public {
        vm.warp(block.timestamp + 3 days + 12 hours);
        gameProxy.resolveClaim(0, 0);
        assertEq(uint8(gameProxy.resolve()), uint8(GameStatus.DEFENDER_WINS));
    }

    /// @dev Static unit test for the correctness an uncontested root resolution.
    function test_resolve_rootUncontestedClockNotExpired_succeeds() public {
        vm.warp(block.timestamp + 3 days + 12 hours - 1 seconds);
        vm.expectRevert(ClockNotExpired.selector);
        gameProxy.resolveClaim(0, 0);
    }

    /// @dev Static unit test for the correctness of a multi-part resolution of a single claim.
    function test_resolve_multiPart_succeeds() public {
        vm.deal(address(this), 10_000 ether);

        uint256 bond = _getRequiredBond(0);
        for (uint256 i = 0; i < 2048; i++) {
            (,,,, Claim disputed,,) = gameProxy.claimData(0);
            gameProxy.attack{ value: bond }(disputed, 0, Claim.wrap(bytes32(i)));
        }

        // Warp past the clock period.
        vm.warp(block.timestamp + 3 days + 12 hours + 1 seconds);

        // Resolve all children of the root subgame. Every single one of these will be uncontested.
        for (uint256 i = 1; i <= 2048; i++) {
            gameProxy.resolveClaim(i, 0);
        }

        // Resolve the first half of the root claim subgame.
        gameProxy.resolveClaim(0, 1024);

        // Fetch the resolution checkpoint for the root subgame and assert correctness.
        (bool initCheckpoint, uint32 subgameIndex, Position leftmostPosition, address counteredBy) =
            gameProxy.resolutionCheckpoints(0);
        assertTrue(initCheckpoint);
        assertEq(subgameIndex, 1024);
        assertEq(leftmostPosition.raw(), Position.wrap(1).move(true).raw());
        assertEq(counteredBy, address(this));

        // The root subgame should not be resolved.
        assertFalse(gameProxy.resolvedSubgames(0));
        vm.expectRevert(OutOfOrderResolution.selector);
        gameProxy.resolve();

        // Resolve the second half of the root claim subgame.
        uint256 numToResolve = gameProxy.getNumToResolve(0);
        assertEq(numToResolve, 1024);
        gameProxy.resolveClaim(0, numToResolve);

        // Fetch the resolution checkpoint for the root subgame and assert correctness.
        (initCheckpoint, subgameIndex, leftmostPosition, counteredBy) = gameProxy.resolutionCheckpoints(0);
        assertTrue(initCheckpoint);
        assertEq(subgameIndex, 2048);
        assertEq(leftmostPosition.raw(), Position.wrap(1).move(true).raw());
        assertEq(counteredBy, address(this));

        // The root subgame should now be resolved
        assertTrue(gameProxy.resolvedSubgames(0));
        assertEq(uint8(gameProxy.resolve()), uint8(GameStatus.CHALLENGER_WINS));
    }

    /// @dev Static unit test asserting that resolve reverts when the absolute root
    ///      subgame has not been resolved.
    function test_resolve_rootUncontestedButUnresolved_reverts() public {
        vm.warp(block.timestamp + 3 days + 12 hours);
        vm.expectRevert(OutOfOrderResolution.selector);
        gameProxy.resolve();
    }

    /// @dev Static unit test asserting that resolve reverts when the game state is
    ///      not in progress.
    function test_resolve_notInProgress_reverts() public {
        uint256 chalWins = uint256(GameStatus.CHALLENGER_WINS);

        // Replace the game status in storage. It exists in slot 0 at offset 16.
        uint256 slot = uint256(vm.load(address(gameProxy), bytes32(0)));
        uint256 offset = 16 << 3;
        uint256 mask = 0xFF << offset;
        // Replace the byte in the slot value with the challenger wins status.
        slot = (slot & ~mask) | (chalWins << offset);

        vm.store(address(gameProxy), bytes32(uint256(0)), bytes32(slot));
        vm.expectRevert(GameNotInProgress.selector);
        gameProxy.resolveClaim(0, 0);
    }

    /// @dev Static unit test for the correctness of resolving a single attack game state.
    function test_resolve_rootContested_succeeds() public {
        (,,,, Claim disputed,,) = gameProxy.claimData(0);
        gameProxy.attack{ value: _getRequiredBond(0) }(disputed, 0, _dummyClaim());

        vm.warp(block.timestamp + 3 days + 12 hours);

        gameProxy.resolveClaim(1, 0);
        gameProxy.resolveClaim(0, 0);
        assertEq(uint8(gameProxy.resolve()), uint8(GameStatus.CHALLENGER_WINS));
    }

    /// @dev Static unit test for the correctness of resolving a game with a contested challenge claim.
    function test_resolve_challengeContested_succeeds() public {
        (,,,, Claim disputed,,) = gameProxy.claimData(0);
        gameProxy.attack{ value: _getRequiredBond(0) }(disputed, 0, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(1);
        gameProxy.defend{ value: _getRequiredBond(1) }(disputed, 1, _dummyClaim());

        vm.warp(block.timestamp + 3 days + 12 hours);

        gameProxy.resolveClaim(2, 0);
        gameProxy.resolveClaim(1, 0);
        gameProxy.resolveClaim(0, 0);
        assertEq(uint8(gameProxy.resolve()), uint8(GameStatus.DEFENDER_WINS));
    }

    /// @dev Static unit test for the correctness of resolving a game with multiplayer moves.
    function test_resolve_teamDeathmatch_succeeds() public {
        (,,,, Claim disputed,,) = gameProxy.claimData(0);
        gameProxy.attack{ value: _getRequiredBond(0) }(disputed, 0, _dummyClaim());
        gameProxy.attack{ value: _getRequiredBond(0) }(disputed, 0, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(1);
        gameProxy.defend{ value: _getRequiredBond(1) }(disputed, 1, _dummyClaim());
        gameProxy.defend{ value: _getRequiredBond(1) }(disputed, 1, _dummyClaim());

        vm.warp(block.timestamp + 3 days + 12 hours);

        gameProxy.resolveClaim(4, 0);
        gameProxy.resolveClaim(3, 0);
        gameProxy.resolveClaim(2, 0);
        gameProxy.resolveClaim(1, 0);
        gameProxy.resolveClaim(0, 0);
        assertEq(uint8(gameProxy.resolve()), uint8(GameStatus.CHALLENGER_WINS));
    }

    /// @dev Static unit test for the correctness of resolving a game that reaches max game depth.
    function test_resolve_stepReached_succeeds() public {
        Claim claim = _dummyClaim();
        for (uint256 i; i < gameProxy.splitDepth(); i++) {
            (,,,, Claim disputed,,) = gameProxy.claimData(i);
            gameProxy.attack{ value: _getRequiredBond(i) }(disputed, i, claim);
        }

        claim = _changeClaimStatus(claim, VMStatuses.PANIC);
        for (uint256 i = gameProxy.claimDataLen() - 1; i < gameProxy.maxGameDepth(); i++) {
            (,,,, Claim disputed,,) = gameProxy.claimData(i);
            gameProxy.attack{ value: _getRequiredBond(i) }(disputed, i, claim);
        }

        vm.warp(block.timestamp + 3 days + 12 hours);

        for (uint256 i = 9; i > 0; i--) {
            gameProxy.resolveClaim(i - 1, 0);
        }
        assertEq(uint8(gameProxy.resolve()), uint8(GameStatus.DEFENDER_WINS));
    }

    /// @dev Static unit test asserting that resolve reverts when attempting to resolve a subgame multiple times
    function test_resolve_claimAlreadyResolved_reverts() public {
        Claim claim = _dummyClaim();
        uint256 firstBond = _getRequiredBond(0);
        vm.deal(address(this), firstBond);
        (,,,, Claim disputed,,) = gameProxy.claimData(0);
        gameProxy.attack{ value: firstBond }(disputed, 0, claim);
        uint256 secondBond = _getRequiredBond(1);
        vm.deal(address(this), secondBond);
        (,,,, disputed,,) = gameProxy.claimData(1);
        gameProxy.attack{ value: secondBond }(disputed, 1, claim);

        vm.warp(block.timestamp + 3 days + 12 hours);

        assertEq(address(this).balance, 0);
        gameProxy.resolveClaim(2, 0);
        gameProxy.resolveClaim(1, 0);

        // Wait for the withdrawal delay.
        vm.warp(block.timestamp + delayedWeth.delay() + 1 seconds);

        gameProxy.claimCredit(address(this));
        assertEq(address(this).balance, firstBond + secondBond);

        vm.expectRevert(ClaimAlreadyResolved.selector);
        gameProxy.resolveClaim(1, 0);
        assertEq(address(this).balance, firstBond + secondBond);
    }

    /// @dev Static unit test asserting that resolve reverts when attempting to resolve a subgame at max depth
    function test_resolve_claimAtMaxDepthAlreadyResolved_reverts() public {
        Claim claim = _dummyClaim();
        for (uint256 i; i < gameProxy.splitDepth(); i++) {
            (,,,, Claim disputed,,) = gameProxy.claimData(i);
            gameProxy.attack{ value: _getRequiredBond(i) }(disputed, i, claim);
        }

        vm.deal(address(this), 10000 ether);
        claim = _changeClaimStatus(claim, VMStatuses.PANIC);
        for (uint256 i = gameProxy.claimDataLen() - 1; i < gameProxy.maxGameDepth(); i++) {
            (,,,, Claim disputed,,) = gameProxy.claimData(i);
            gameProxy.attack{ value: _getRequiredBond(i) }(disputed, i, claim);
        }

        vm.warp(block.timestamp + 3 days + 12 hours);

        // Resolve to claim bond
        uint256 balanceBefore = address(this).balance;
        gameProxy.resolveClaim(8, 0);

        // Wait for the withdrawal delay.
        vm.warp(block.timestamp + delayedWeth.delay() + 1 seconds);

        gameProxy.claimCredit(address(this));
        assertEq(address(this).balance, balanceBefore + _getRequiredBond(7));

        vm.expectRevert(ClaimAlreadyResolved.selector);
        gameProxy.resolveClaim(8, 0);
    }

    /// @dev Static unit test asserting that resolve reverts when attempting to resolve subgames out of order
    function test_resolve_outOfOrderResolution_reverts() public {
        (,,,, Claim disputed,,) = gameProxy.claimData(0);
        gameProxy.attack{ value: _getRequiredBond(0) }(disputed, 0, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(1);
        gameProxy.attack{ value: _getRequiredBond(1) }(disputed, 1, _dummyClaim());

        vm.warp(block.timestamp + 3 days + 12 hours);

        vm.expectRevert(OutOfOrderResolution.selector);
        gameProxy.resolveClaim(0, 0);
    }

    /// @dev Static unit test asserting that resolve pays out bonds on step, output bisection, and execution trace
    /// moves.
    function test_resolve_bondPayouts_succeeds() public {
        // Give the test contract some ether
        uint256 bal = 1000 ether;
        vm.deal(address(this), bal);

        // Make claims all the way down the tree.
        uint256 bond = _getRequiredBond(0);
        uint256 totalBonded = bond;
        (,,,, Claim disputed,,) = gameProxy.claimData(0);
        gameProxy.attack{ value: bond }(disputed, 0, _dummyClaim());
        bond = _getRequiredBond(1);
        totalBonded += bond;
        (,,,, disputed,,) = gameProxy.claimData(1);
        gameProxy.attack{ value: bond }(disputed, 1, _dummyClaim());
        bond = _getRequiredBond(2);
        totalBonded += bond;
        (,,,, disputed,,) = gameProxy.claimData(2);
        gameProxy.attack{ value: bond }(disputed, 2, _dummyClaim());
        bond = _getRequiredBond(3);
        totalBonded += bond;
        (,,,, disputed,,) = gameProxy.claimData(3);
        gameProxy.attack{ value: bond }(disputed, 3, _dummyClaim());
        bond = _getRequiredBond(4);
        totalBonded += bond;
        (,,,, disputed,,) = gameProxy.claimData(4);
        gameProxy.attack{ value: bond }(disputed, 4, _changeClaimStatus(_dummyClaim(), VMStatuses.PANIC));
        bond = _getRequiredBond(5);
        totalBonded += bond;
        (,,,, disputed,,) = gameProxy.claimData(5);
        gameProxy.attack{ value: bond }(disputed, 5, _dummyClaim());
        bond = _getRequiredBond(6);
        totalBonded += bond;
        (,,,, disputed,,) = gameProxy.claimData(6);
        gameProxy.attack{ value: bond }(disputed, 6, _dummyClaim());
        bond = _getRequiredBond(7);
        totalBonded += bond;
        (,,,, disputed,,) = gameProxy.claimData(7);
        gameProxy.attack{ value: bond }(disputed, 7, _dummyClaim());
        gameProxy.addLocalData(LocalPreimageKey.DISPUTED_L2_BLOCK_NUMBER, 8, 0);
        gameProxy.step(8, true, absolutePrestateData, hex"");

        // Ensure that the step successfully countered the leaf claim.
        (, address counteredBy,,,,,) = gameProxy.claimData(8);
        assertEq(counteredBy, address(this));

        // Ensure we bonded the correct amounts
        assertEq(address(this).balance, bal - totalBonded);
        assertEq(address(gameProxy).balance, 0);
        assertEq(delayedWeth.balanceOf(address(gameProxy)), totalBonded);

        // Resolve all claims
        vm.warp(block.timestamp + 3 days + 12 hours);
        for (uint256 i = gameProxy.claimDataLen(); i > 0; i--) {
            (bool success,) = address(gameProxy).call(abi.encodeCall(gameProxy.resolveClaim, (i - 1, 0)));
            assertTrue(success);
        }
        gameProxy.resolve();

        // Wait for the withdrawal delay.
        vm.warp(block.timestamp + delayedWeth.delay() + 1 seconds);

        gameProxy.claimCredit(address(this));

        // Ensure that bonds were paid out correctly.
        assertEq(address(this).balance, bal);
        assertEq(address(gameProxy).balance, 0);
        assertEq(delayedWeth.balanceOf(address(gameProxy)), 0);

        // Ensure that the init bond for the game is 0, in case we change it in the test suite in the future.
        assertEq(disputeGameFactory.initBonds(GAME_TYPE), 0);
    }

    /// @dev Static unit test asserting that resolve pays out bonds on step, output bisection, and execution trace
    /// moves with 2 actors and a dishonest root claim.
    function test_resolve_bondPayoutsSeveralActors_succeeds() public {
        // Give the test contract and bob some ether
        // We use the "1000 ether" literal for `bal`, the initial balance, to avoid stack too deep
        //uint256 bal = 1000 ether;
        address bob = address(0xb0b);
        vm.deal(address(this), 1000 ether);
        vm.deal(bob, 1000 ether);

        // Make claims all the way down the tree, trading off between bob and the test contract.
        uint256 firstBond = _getRequiredBond(0);
        uint256 thisBonded = firstBond;
        (,,,, Claim disputed,,) = gameProxy.claimData(0);
        gameProxy.attack{ value: firstBond }(disputed, 0, _dummyClaim());

        uint256 secondBond = _getRequiredBond(1);
        uint256 bobBonded = secondBond;
        (,,,, disputed,,) = gameProxy.claimData(1);
        vm.prank(bob);
        gameProxy.attack{ value: secondBond }(disputed, 1, _dummyClaim());

        uint256 thirdBond = _getRequiredBond(2);
        thisBonded += thirdBond;
        (,,,, disputed,,) = gameProxy.claimData(2);
        gameProxy.attack{ value: thirdBond }(disputed, 2, _dummyClaim());

        uint256 fourthBond = _getRequiredBond(3);
        bobBonded += fourthBond;
        (,,,, disputed,,) = gameProxy.claimData(3);
        vm.prank(bob);
        gameProxy.attack{ value: fourthBond }(disputed, 3, _dummyClaim());

        uint256 fifthBond = _getRequiredBond(4);
        thisBonded += fifthBond;
        (,,,, disputed,,) = gameProxy.claimData(4);
        gameProxy.attack{ value: fifthBond }(disputed, 4, _changeClaimStatus(_dummyClaim(), VMStatuses.PANIC));

        uint256 sixthBond = _getRequiredBond(5);
        bobBonded += sixthBond;
        (,,,, disputed,,) = gameProxy.claimData(5);
        vm.prank(bob);
        gameProxy.attack{ value: sixthBond }(disputed, 5, _dummyClaim());

        uint256 seventhBond = _getRequiredBond(6);
        thisBonded += seventhBond;
        (,,,, disputed,,) = gameProxy.claimData(6);
        gameProxy.attack{ value: seventhBond }(disputed, 6, _dummyClaim());

        uint256 eighthBond = _getRequiredBond(7);
        bobBonded += eighthBond;
        (,,,, disputed,,) = gameProxy.claimData(7);
        vm.prank(bob);
        gameProxy.attack{ value: eighthBond }(disputed, 7, _dummyClaim());

        gameProxy.addLocalData(LocalPreimageKey.DISPUTED_L2_BLOCK_NUMBER, 8, 0);
        gameProxy.step(8, true, absolutePrestateData, hex"");

        // Ensure that the step successfully countered the leaf claim.
        (, address counteredBy,,,,,) = gameProxy.claimData(8);
        assertEq(counteredBy, address(this));

        // Ensure we bonded the correct amounts
        assertEq(address(this).balance, 1000 ether - thisBonded);
        assertEq(bob.balance, 1000 ether - bobBonded);
        assertEq(address(gameProxy).balance, 0);
        assertEq(delayedWeth.balanceOf(address(gameProxy)), thisBonded + bobBonded);

        // Resolve all claims
        vm.warp(block.timestamp + 3 days + 12 hours);
        for (uint256 i = gameProxy.claimDataLen(); i > 0; i--) {
            (bool success,) = address(gameProxy).call(abi.encodeCall(gameProxy.resolveClaim, (i - 1, 0)));
            assertTrue(success);
        }
        gameProxy.resolve();

        // Wait for the withdrawal delay.
        vm.warp(block.timestamp + delayedWeth.delay() + 1 seconds);

        gameProxy.claimCredit(address(this));

        // Bob's claim should revert since it's value is 0
        vm.expectRevert(NoCreditToClaim.selector);
        gameProxy.claimCredit(bob);

        // Ensure that bonds were paid out correctly.
        assertEq(address(this).balance, 1000 ether + bobBonded);
        assertEq(bob.balance, 1000 ether - bobBonded);
        assertEq(address(gameProxy).balance, 0);
        assertEq(delayedWeth.balanceOf(address(gameProxy)), 0);

        // Ensure that the init bond for the game is 0, in case we change it in the test suite in the future.
        assertEq(disputeGameFactory.initBonds(GAME_TYPE), 0);
    }

    /// @dev Static unit test asserting that resolve pays out bonds on moves to the leftmost actor
    /// in subgames containing successful counters.
    function test_resolve_leftmostBondPayout_succeeds() public {
        uint256 bal = 1000 ether;
        address alice = address(0xa11ce);
        address bob = address(0xb0b);
        address charlie = address(0xc0c);
        vm.deal(address(this), bal);
        vm.deal(alice, bal);
        vm.deal(bob, bal);
        vm.deal(charlie, bal);

        // Make claims with bob, charlie and the test contract on defense, and alice as the challenger
        // charlie is successfully countered by alice
        // alice is successfully countered by both bob and the test contract
        uint256 firstBond = _getRequiredBond(0);
        (,,,, Claim disputed,,) = gameProxy.claimData(0);
        vm.prank(alice);
        gameProxy.attack{ value: firstBond }(disputed, 0, _dummyClaim());

        uint256 secondBond = _getRequiredBond(1);
        (,,,, disputed,,) = gameProxy.claimData(1);
        vm.prank(bob);
        gameProxy.defend{ value: secondBond }(disputed, 1, _dummyClaim());
        vm.prank(charlie);
        gameProxy.attack{ value: secondBond }(disputed, 1, _dummyClaim());
        gameProxy.attack{ value: secondBond }(disputed, 1, _dummyClaim());

        uint256 thirdBond = _getRequiredBond(3);
        (,,,, disputed,,) = gameProxy.claimData(3);
        vm.prank(alice);
        gameProxy.attack{ value: thirdBond }(disputed, 3, _dummyClaim());

        // Resolve all claims
        vm.warp(block.timestamp + 3 days + 12 hours);
        for (uint256 i = gameProxy.claimDataLen(); i > 0; i--) {
            (bool success,) = address(gameProxy).call(abi.encodeCall(gameProxy.resolveClaim, (i - 1, 0)));
            assertTrue(success);
        }
        gameProxy.resolve();

        // Wait for the withdrawal delay.
        vm.warp(block.timestamp + delayedWeth.delay() + 1 seconds);

        gameProxy.claimCredit(address(this));
        gameProxy.claimCredit(alice);
        gameProxy.claimCredit(bob);

        // Charlie's claim should revert since it's value is 0
        vm.expectRevert(NoCreditToClaim.selector);
        gameProxy.claimCredit(charlie);

        // Ensure that bonds were paid out correctly.
        uint256 aliceLosses = firstBond;
        uint256 charlieLosses = secondBond;
        assertEq(address(this).balance, bal + aliceLosses, "incorrect this balance");
        assertEq(alice.balance, bal - aliceLosses + charlieLosses, "incorrect alice balance");
        assertEq(bob.balance, bal, "incorrect bob balance");
        assertEq(charlie.balance, bal - charlieLosses, "incorrect charlie balance");
        assertEq(address(gameProxy).balance, 0);

        // Ensure that the init bond for the game is 0, in case we change it in the test suite in the future.
        assertEq(disputeGameFactory.initBonds(GAME_TYPE), 0);
    }

    /// @dev Static unit test asserting that the anchor state updates when the game resolves in
    /// favor of the defender and the anchor state is older than the game state.
    function test_resolve_validNewerStateUpdatesAnchor_succeeds() public {
        // Confirm that the anchor state is older than the game state.
        (Hash root, uint256 l2BlockNumber) = anchorStateRegistry.anchors(gameProxy.gameType());
        assert(l2BlockNumber < gameProxy.l2BlockNumber());

        // Resolve the game.
        vm.warp(block.timestamp + 3 days + 12 hours);
        gameProxy.resolveClaim(0, 0);
        assertEq(uint8(gameProxy.resolve()), uint8(GameStatus.DEFENDER_WINS));

        // Confirm that the anchor state is now the same as the game state.
        (root, l2BlockNumber) = anchorStateRegistry.anchors(gameProxy.gameType());
        assertEq(l2BlockNumber, gameProxy.l2BlockNumber());
        assertEq(root.raw(), gameProxy.rootClaim().raw());
    }

    /// @dev Static unit test asserting that the anchor state does not change when the game
    /// resolves in favor of the defender but the game state is not newer than the anchor state.
    function test_resolve_validOlderStateSameAnchor_succeeds() public {
        // Mock the game block to be older than the game state.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.l2BlockNumber, ()), abi.encode(0));

        // Confirm that the anchor state is newer than the game state.
        (Hash root, uint256 l2BlockNumber) = anchorStateRegistry.anchors(gameProxy.gameType());
        assert(l2BlockNumber >= gameProxy.l2BlockNumber());

        // Resolve the game.
        vm.mockCall(address(gameProxy), abi.encodeCall(gameProxy.l2BlockNumber, ()), abi.encode(0));
        vm.warp(block.timestamp + 3 days + 12 hours);
        gameProxy.resolveClaim(0, 0);
        assertEq(uint8(gameProxy.resolve()), uint8(GameStatus.DEFENDER_WINS));

        // Confirm that the anchor state is the same as the initial anchor state.
        (Hash updatedRoot, uint256 updatedL2BlockNumber) = anchorStateRegistry.anchors(gameProxy.gameType());
        assertEq(updatedL2BlockNumber, l2BlockNumber);
        assertEq(updatedRoot.raw(), root.raw());
    }

    /// @dev Static unit test asserting that the anchor state does not change when the game
    /// resolves in favor of the challenger, even if the game state is newer than the anchor.
    function test_resolve_invalidStateSameAnchor_succeeds() public {
        // Confirm that the anchor state is older than the game state.
        (Hash root, uint256 l2BlockNumber) = anchorStateRegistry.anchors(gameProxy.gameType());
        assert(l2BlockNumber < gameProxy.l2BlockNumber());

        // Challenge the claim and resolve it.
        (,,,, Claim disputed,,) = gameProxy.claimData(0);
        gameProxy.attack{ value: _getRequiredBond(0) }(disputed, 0, _dummyClaim());
        vm.warp(block.timestamp + 3 days + 12 hours);
        gameProxy.resolveClaim(1, 0);
        gameProxy.resolveClaim(0, 0);
        assertEq(uint8(gameProxy.resolve()), uint8(GameStatus.CHALLENGER_WINS));

        // Confirm that the anchor state is the same as the initial anchor state.
        (Hash updatedRoot, uint256 updatedL2BlockNumber) = anchorStateRegistry.anchors(gameProxy.gameType());
        assertEq(updatedL2BlockNumber, l2BlockNumber);
        assertEq(updatedRoot.raw(), root.raw());
    }

    /// @dev Static unit test asserting that credit may not be drained past allowance through reentrancy.
    function test_claimCredit_claimAlreadyResolved_reverts() public {
        ClaimCreditReenter reenter = new ClaimCreditReenter(gameProxy, vm);
        vm.startPrank(address(reenter));

        // Give the game proxy 1 extra ether, unregistered.
        vm.deal(address(gameProxy), 1 ether);

        // Perform a bonded move.
        Claim claim = _dummyClaim();
        uint256 firstBond = _getRequiredBond(0);
        vm.deal(address(reenter), firstBond);
        (,,,, Claim disputed,,) = gameProxy.claimData(0);
        gameProxy.attack{ value: firstBond }(disputed, 0, claim);
        uint256 secondBond = _getRequiredBond(1);
        vm.deal(address(reenter), secondBond);
        (,,,, disputed,,) = gameProxy.claimData(1);
        gameProxy.attack{ value: secondBond }(disputed, 1, claim);
        uint256 reenterBond = firstBond + secondBond;

        // Warp past the finalization period
        vm.warp(block.timestamp + 3 days + 12 hours);

        // Ensure that we bonded all the test contract's ETH
        assertEq(address(reenter).balance, 0);
        // Ensure the game proxy has 1 ether in it.
        assertEq(address(gameProxy).balance, 1 ether);
        // Ensure the game has a balance of reenterBond in the delayedWeth contract.
        assertEq(delayedWeth.balanceOf(address(gameProxy)), reenterBond);

        // Resolve the claim at index 2 first so that index 1 can be resolved.
        gameProxy.resolveClaim(2, 0);

        // Resolve the claim at index 1 and claim the reenter contract's credit.
        gameProxy.resolveClaim(1, 0);

        // Ensure that the game registered the `reenter` contract's credit.
        assertEq(gameProxy.credit(address(reenter)), reenterBond);

        // Wait for the withdrawal delay.
        vm.warp(block.timestamp + delayedWeth.delay() + 1 seconds);

        // Initiate the reentrant credit claim.
        reenter.claimCredit(address(reenter));

        // The reenter contract should have performed 2 calls to `claimCredit`.
        // Once all the credit is claimed, all subsequent calls will revert since there is 0 credit left to claim.
        // The claimant must only have received the amount bonded for the gindex 1 subgame.
        // The root claim bond and the unregistered ETH should still exist in the game proxy.
        assertEq(reenter.numCalls(), 2);
        assertEq(address(reenter).balance, reenterBond);
        assertEq(address(gameProxy).balance, 1 ether);
        assertEq(delayedWeth.balanceOf(address(gameProxy)), 0);

        vm.stopPrank();
    }

    /// @dev Tests that adding local data with an out of bounds identifier reverts.
    function testFuzz_addLocalData_oob_reverts(uint256 _ident) public {
        Claim disputed;
        // Get a claim below the split depth so that we can add local data for an execution trace subgame.
        for (uint256 i; i < 4; i++) {
            uint256 bond = _getRequiredBond(i);
            (,,,, disputed,,) = gameProxy.claimData(i);
            gameProxy.attack{ value: bond }(disputed, i, _dummyClaim());
        }
        uint256 lastBond = _getRequiredBond(4);
        (,,,, disputed,,) = gameProxy.claimData(4);
        gameProxy.attack{ value: lastBond }(disputed, 4, _changeClaimStatus(_dummyClaim(), VMStatuses.PANIC));

        // [1, 5] are valid local data identifiers.
        if (_ident <= 5) _ident = 0;

        vm.expectRevert(InvalidLocalIdent.selector);
        gameProxy.addLocalData(_ident, 5, 0);
    }

    /// @dev Tests that local data is loaded into the preimage oracle correctly in the subgame
    ///      that is disputing the transition from `GENESIS -> GENESIS + 1`
    function test_addLocalDataGenesisTransition_static_succeeds() public {
        IPreimageOracle oracle = IPreimageOracle(address(gameProxy.vm().oracle()));
        Claim disputed;

        // Get a claim below the split depth so that we can add local data for an execution trace subgame.
        for (uint256 i; i < 4; i++) {
            uint256 bond = _getRequiredBond(i);
            (,,,, disputed,,) = gameProxy.claimData(i);
            gameProxy.attack{ value: bond }(disputed, i, Claim.wrap(bytes32(i)));
        }
        uint256 lastBond = _getRequiredBond(4);
        (,,,, disputed,,) = gameProxy.claimData(4);
        gameProxy.attack{ value: lastBond }(disputed, 4, _changeClaimStatus(_dummyClaim(), VMStatuses.PANIC));

        // Expected start/disputed claims
        (Hash root,) = gameProxy.startingOutputRoot();
        bytes32 startingClaim = root.raw();
        bytes32 disputedClaim = bytes32(uint256(3));
        Position disputedPos = LibPosition.wrap(4, 0);

        // Expected local data
        bytes32[5] memory data = [
            gameProxy.l1Head().raw(),
            startingClaim,
            disputedClaim,
            bytes32(uint256(1) << 0xC0),
            bytes32(gameProxy.l2ChainId() << 0xC0)
        ];

        for (uint256 i = 1; i <= 5; i++) {
            uint256 expectedLen = i > 3 ? 8 : 32;
            bytes32 key = _getKey(i, keccak256(abi.encode(disputedClaim, disputedPos)));

            gameProxy.addLocalData(i, 5, 0);
            (bytes32 dat, uint256 datLen) = oracle.readPreimage(key, 0);
            assertEq(dat >> 0xC0, bytes32(expectedLen));
            // Account for the length prefix if i > 3 (the data stored
            // at identifiers i <= 3 are 32 bytes long, so the expected
            // length is already correct. If i > 3, the data is only 8
            // bytes long, so the length prefix + the data is 16 bytes
            // total.)
            assertEq(datLen, expectedLen + (i > 3 ? 8 : 0));

            gameProxy.addLocalData(i, 5, 8);
            (dat, datLen) = oracle.readPreimage(key, 8);
            assertEq(dat, data[i - 1]);
            assertEq(datLen, expectedLen);
        }
    }

    /// @dev Tests that local data is loaded into the preimage oracle correctly.
    function test_addLocalDataMiddle_static_succeeds() public {
        IPreimageOracle oracle = IPreimageOracle(address(gameProxy.vm().oracle()));
        Claim disputed;

        // Get a claim below the split depth so that we can add local data for an execution trace subgame.
        for (uint256 i; i < 4; i++) {
            uint256 bond = _getRequiredBond(i);
            (,,,, disputed,,) = gameProxy.claimData(i);
            gameProxy.attack{ value: bond }(disputed, i, Claim.wrap(bytes32(i)));
        }
        uint256 lastBond = _getRequiredBond(4);
        (,,,, disputed,,) = gameProxy.claimData(4);
        gameProxy.defend{ value: lastBond }(disputed, 4, _changeClaimStatus(ROOT_CLAIM, VMStatuses.VALID));

        // Expected start/disputed claims
        bytes32 startingClaim = bytes32(uint256(3));
        Position startingPos = LibPosition.wrap(4, 0);
        bytes32 disputedClaim = bytes32(uint256(2));
        Position disputedPos = LibPosition.wrap(3, 0);

        // Expected local data
        bytes32[5] memory data = [
            gameProxy.l1Head().raw(),
            startingClaim,
            disputedClaim,
            bytes32(uint256(2) << 0xC0),
            bytes32(gameProxy.l2ChainId() << 0xC0)
        ];

        for (uint256 i = 1; i <= 5; i++) {
            uint256 expectedLen = i > 3 ? 8 : 32;
            bytes32 key = _getKey(i, keccak256(abi.encode(startingClaim, startingPos, disputedClaim, disputedPos)));

            gameProxy.addLocalData(i, 5, 0);
            (bytes32 dat, uint256 datLen) = oracle.readPreimage(key, 0);
            assertEq(dat >> 0xC0, bytes32(expectedLen));
            // Account for the length prefix if i > 3 (the data stored
            // at identifiers i <= 3 are 32 bytes long, so the expected
            // length is already correct. If i > 3, the data is only 8
            // bytes long, so the length prefix + the data is 16 bytes
            // total.)
            assertEq(datLen, expectedLen + (i > 3 ? 8 : 0));

            gameProxy.addLocalData(i, 5, 8);
            (dat, datLen) = oracle.readPreimage(key, 8);
            assertEq(dat, data[i - 1]);
            assertEq(datLen, expectedLen);
        }
    }

    /// @dev Tests that the L2 block number claim is favored over the bisected-to block when adding data
    ///
    function test_addLocalData_l2BlockNumberExtension_succeeds() public {
        // Deploy a new dispute game with a L2 block number claim of 8. This is directly in the middle of
        // the leaves in our output bisection test tree, at SPLIT_DEPTH = 2 ** 2
        IFaultDisputeGame game = IFaultDisputeGame(
            address(disputeGameFactory.create(GAME_TYPE, Claim.wrap(bytes32(uint256(0xFF))), abi.encode(uint256(8))))
        );

        // Get a claim below the split depth so that we can add local data for an execution trace subgame.
        {
            Claim disputed;
            Position parent;
            Position pos;

            for (uint256 i; i < 4; i++) {
                (,,,,, parent,) = game.claimData(i);
                pos = parent.move(true);
                uint256 bond = game.getRequiredBond(pos);

                (,,,, disputed,,) = game.claimData(i);
                if (i == 0) {
                    game.attack{ value: bond }(disputed, i, Claim.wrap(bytes32(i)));
                } else {
                    game.defend{ value: bond }(disputed, i, Claim.wrap(bytes32(i)));
                }
            }
            (,,,,, parent,) = game.claimData(4);
            pos = parent.move(true);
            uint256 lastBond = game.getRequiredBond(pos);
            (,,,, disputed,,) = game.claimData(4);
            game.defend{ value: lastBond }(disputed, 4, _changeClaimStatus(ROOT_CLAIM, VMStatuses.INVALID));
        }

        // Expected start/disputed claims
        bytes32 startingClaim = bytes32(uint256(3));
        Position startingPos = LibPosition.wrap(4, 14);
        bytes32 disputedClaim = bytes32(uint256(0xFF));
        Position disputedPos = LibPosition.wrap(0, 0);

        // Expected local data. This should be `l2BlockNumber`, and not the actual bisected-to block,
        // as we choose the minimum between the two.
        bytes32 expectedNumber = bytes32(uint256(8 << 0xC0));
        uint256 expectedLen = 8;
        uint256 l2NumberIdent = LocalPreimageKey.DISPUTED_L2_BLOCK_NUMBER;

        // Compute the preimage key for the local data
        bytes32 localContext = keccak256(abi.encode(startingClaim, startingPos, disputedClaim, disputedPos));
        bytes32 rawKey = keccak256(abi.encode(l2NumberIdent | (1 << 248), address(game), localContext));
        bytes32 key = bytes32((uint256(rawKey) & ~uint256(0xFF << 248)) | (1 << 248));

        IPreimageOracle oracle = IPreimageOracle(address(game.vm().oracle()));
        game.addLocalData(l2NumberIdent, 5, 0);

        (bytes32 dat, uint256 datLen) = oracle.readPreimage(key, 0);
        assertEq(dat >> 0xC0, bytes32(expectedLen));
        assertEq(datLen, expectedLen + 8);

        game.addLocalData(l2NumberIdent, 5, 8);
        (dat, datLen) = oracle.readPreimage(key, 8);
        assertEq(dat, expectedNumber);
        assertEq(datLen, expectedLen);
    }

    /// @dev Static unit test asserting that resolveClaim isn't possible if there's time
    ///      left for a counter.
    function test_resolution_lastSecondDisputes_succeeds() public {
        // The honest proposer created an honest root claim during setup - node 0

        // Defender's turn
        vm.warp(block.timestamp + 3.5 days - 1 seconds);
        (,,,, Claim disputed,,) = gameProxy.claimData(0);
        gameProxy.attack{ value: _getRequiredBond(0) }(disputed, 0, _dummyClaim());
        // Chess clock time accumulated:
        assertEq(gameProxy.getChallengerDuration(0).raw(), 3.5 days - 1 seconds);
        assertEq(gameProxy.getChallengerDuration(1).raw(), 0);

        // Advance time by 1 second, so that the root claim challenger clock is expired.
        vm.warp(block.timestamp + 1 seconds);
        // Attempt a second attack against the root claim. This should revert since the challenger clock is expired.
        uint256 expectedBond = _getRequiredBond(0);
        vm.expectRevert(ClockTimeExceeded.selector);
        gameProxy.attack{ value: expectedBond }(disputed, 0, _dummyClaim());
        // Chess clock time accumulated:
        assertEq(gameProxy.getChallengerDuration(0).raw(), 3.5 days);
        assertEq(gameProxy.getChallengerDuration(1).raw(), 1 seconds);

        // Should not be able to resolve the root claim or second counter yet.
        vm.expectRevert(ClockNotExpired.selector);
        gameProxy.resolveClaim(1, 0);
        vm.expectRevert(OutOfOrderResolution.selector);
        gameProxy.resolveClaim(0, 0);

        // Warp to the last second of the root claim defender clock.
        vm.warp(block.timestamp + 3.5 days - 2 seconds);
        // Attack the challenge to the root claim. This should succeed, since the defender clock is not expired.
        (,,,, disputed,,) = gameProxy.claimData(1);
        gameProxy.attack{ value: _getRequiredBond(1) }(disputed, 1, _dummyClaim());
        // Chess clock time accumulated:
        assertEq(gameProxy.getChallengerDuration(0).raw(), 3.5 days);
        assertEq(gameProxy.getChallengerDuration(1).raw(), 3.5 days - 1 seconds);
        assertEq(gameProxy.getChallengerDuration(2).raw(), 3.5 days - gameProxy.clockExtension().raw());

        // Should not be able to resolve any claims yet.
        vm.expectRevert(ClockNotExpired.selector);
        gameProxy.resolveClaim(2, 0);
        vm.expectRevert(ClockNotExpired.selector);
        gameProxy.resolveClaim(1, 0);
        vm.expectRevert(OutOfOrderResolution.selector);
        gameProxy.resolveClaim(0, 0);

        vm.warp(block.timestamp + gameProxy.clockExtension().raw() - 1 seconds);

        // Should not be able to resolve any claims yet.
        vm.expectRevert(ClockNotExpired.selector);
        gameProxy.resolveClaim(2, 0);
        vm.expectRevert(OutOfOrderResolution.selector);
        gameProxy.resolveClaim(1, 0);
        vm.expectRevert(OutOfOrderResolution.selector);
        gameProxy.resolveClaim(0, 0);

        // Chess clock time accumulated:
        assertEq(gameProxy.getChallengerDuration(0).raw(), 3.5 days);
        assertEq(gameProxy.getChallengerDuration(1).raw(), 3.5 days);
        assertEq(gameProxy.getChallengerDuration(2).raw(), 3.5 days - 1 seconds);

        // Warp past the challenge period for the root claim defender. Defending the root claim should now revert.
        vm.warp(block.timestamp + 1 seconds);
        expectedBond = _getRequiredBond(1);
        vm.expectRevert(ClockTimeExceeded.selector); // no further move can be made
        gameProxy.attack{ value: expectedBond }(disputed, 1, _dummyClaim());
        expectedBond = _getRequiredBond(2);
        (,,,, disputed,,) = gameProxy.claimData(2);
        vm.expectRevert(ClockTimeExceeded.selector); // no further move can be made
        gameProxy.attack{ value: expectedBond }(disputed, 2, _dummyClaim());
        // Chess clock time accumulated:
        assertEq(gameProxy.getChallengerDuration(0).raw(), 3.5 days);
        assertEq(gameProxy.getChallengerDuration(1).raw(), 3.5 days);
        assertEq(gameProxy.getChallengerDuration(2).raw(), 3.5 days);

        vm.expectRevert(OutOfOrderResolution.selector);
        gameProxy.resolveClaim(1, 0);
        vm.expectRevert(OutOfOrderResolution.selector);
        gameProxy.resolveClaim(0, 0);

        // All clocks are expired. Resolve the game.
        gameProxy.resolveClaim(2, 0); // Node 2 is resolved as UNCOUNTERED by default since it has no children
        gameProxy.resolveClaim(1, 0); // Node 1 is resolved as COUNTERED since it has an UNCOUNTERED child
        gameProxy.resolveClaim(0, 0); // Node 0 is resolved as UNCOUNTERED since it has no UNCOUNTERED children

        // Defender wins game since the root claim is uncountered
        assertEq(uint8(gameProxy.resolve()), uint8(GameStatus.DEFENDER_WINS));
    }

    /// @dev Helper to generate a mock RLP encoded header (with only a real block number) & an output root proof.
    function _generateOutputRootProof(
        bytes32 _storageRoot,
        bytes32 _withdrawalRoot,
        bytes memory _l2BlockNumber
    )
        internal
        pure
        returns (Types.OutputRootProof memory proof_, bytes32 root_, bytes memory rlp_)
    {
        // L2 Block header
        bytes[] memory rawHeaderRLP = new bytes[](9);
        rawHeaderRLP[0] = hex"83FACADE";
        rawHeaderRLP[1] = hex"83FACADE";
        rawHeaderRLP[2] = hex"83FACADE";
        rawHeaderRLP[3] = hex"83FACADE";
        rawHeaderRLP[4] = hex"83FACADE";
        rawHeaderRLP[5] = hex"83FACADE";
        rawHeaderRLP[6] = hex"83FACADE";
        rawHeaderRLP[7] = hex"83FACADE";
        rawHeaderRLP[8] = RLPWriter.writeBytes(_l2BlockNumber);
        rlp_ = RLPWriter.writeList(rawHeaderRLP);

        // Output root
        proof_ = Types.OutputRootProof({
            version: 0,
            stateRoot: _storageRoot,
            messagePasserStorageRoot: _withdrawalRoot,
            latestBlockhash: keccak256(rlp_)
        });
        root_ = Hashing.hashOutputRootProof(proof_);
    }

    /// @dev Helper to get the required bond for the given claim index.
    function _getRequiredBond(uint256 _claimIndex) internal view returns (uint256 bond_) {
        (,,,,, Position parent,) = gameProxy.claimData(_claimIndex);
        Position pos = parent.move(true);
        bond_ = gameProxy.getRequiredBond(pos);
    }

    /// @dev Helper to return a pseudo-random claim
    function _dummyClaim() internal view returns (Claim) {
        return Claim.wrap(keccak256(abi.encode(gasleft())));
    }

    /// @dev Helper to get the localized key for an identifier in the context of the game proxy.
    function _getKey(uint256 _ident, bytes32 _localContext) internal view returns (bytes32) {
        bytes32 h = keccak256(abi.encode(_ident | (1 << 248), address(gameProxy), _localContext));
        return bytes32((uint256(h) & ~uint256(0xFF << 248)) | (1 << 248));
    }
}

contract FaultDispute_1v1_Actors_Test is FaultDisputeGame_Init {
    /// @dev The honest actor
    DisputeActor internal honest;
    /// @dev The dishonest actor
    DisputeActor internal dishonest;

    function setUp() public override {
        // Setup the `FaultDisputeGame`
        super.setUp();
    }

    /// @notice Fuzz test for a 1v1 output bisection dispute.
    /// @dev The alphabet game has a constant status byte, and is not safe from someone being dishonest in
    ///      output bisection and then posting a correct execution trace bisection root claim. This test
    ///      does not cover this case (i.e. root claim of output bisection is dishonest, root claim of
    ///      execution trace bisection is made by the dishonest actor but is honest, honest actor cannot
    ///      attack it without risk of losing).
    function testFuzz_outputBisection1v1honestRoot_succeeds(uint8 _divergeOutput, uint8 _divergeStep) public {
        uint256[] memory honestL2Outputs = new uint256[](16);
        for (uint256 i; i < honestL2Outputs.length; i++) {
            honestL2Outputs[i] = i + 1;
        }
        bytes memory honestTrace = new bytes(256);
        for (uint256 i; i < honestTrace.length; i++) {
            honestTrace[i] = bytes1(uint8(i));
        }

        uint256 divergeAtOutput = bound(_divergeOutput, 0, 15);
        uint256 divergeAtStep = bound(_divergeStep, 0, 7);
        uint256 divergeStepOffset = (divergeAtOutput << 4) + divergeAtStep;

        uint256[] memory dishonestL2Outputs = new uint256[](16);
        for (uint256 i; i < dishonestL2Outputs.length; i++) {
            dishonestL2Outputs[i] = i >= divergeAtOutput ? 0xFF : i + 1;
        }
        bytes memory dishonestTrace = new bytes(256);
        for (uint256 i; i < dishonestTrace.length; i++) {
            dishonestTrace[i] = i >= divergeStepOffset ? bytes1(uint8(0xFF)) : bytes1(uint8(i));
        }

        // Run the actor test
        _actorTest({
            _rootClaim: 16,
            _absolutePrestateData: 0,
            _honestTrace: honestTrace,
            _honestL2Outputs: honestL2Outputs,
            _dishonestTrace: dishonestTrace,
            _dishonestL2Outputs: dishonestL2Outputs,
            _expectedStatus: GameStatus.DEFENDER_WINS
        });
    }

    /// @notice Static unit test for a 1v1 output bisection dispute.
    function test_static_1v1honestRootGenesisAbsolutePrestate_succeeds() public {
        // The honest l2 outputs are from [1, 16] in this game.
        uint256[] memory honestL2Outputs = new uint256[](16);
        for (uint256 i; i < honestL2Outputs.length; i++) {
            honestL2Outputs[i] = i + 1;
        }
        // The honest trace covers all block -> block + 1 transitions, and is 256 bytes long, consisting
        // of bytes [0, 255].
        bytes memory honestTrace = new bytes(256);
        for (uint256 i; i < honestTrace.length; i++) {
            honestTrace[i] = bytes1(uint8(i));
        }

        // The dishonest l2 outputs are from [2, 17] in this game.
        uint256[] memory dishonestL2Outputs = new uint256[](16);
        for (uint256 i; i < dishonestL2Outputs.length; i++) {
            dishonestL2Outputs[i] = i + 2;
        }
        // The dishonest trace covers all block -> block + 1 transitions, and is 256 bytes long, consisting
        // of all set bits.
        bytes memory dishonestTrace = new bytes(256);
        for (uint256 i; i < dishonestTrace.length; i++) {
            dishonestTrace[i] = bytes1(0xFF);
        }

        // Run the actor test
        _actorTest({
            _rootClaim: 16,
            _absolutePrestateData: 0,
            _honestTrace: honestTrace,
            _honestL2Outputs: honestL2Outputs,
            _dishonestTrace: dishonestTrace,
            _dishonestL2Outputs: dishonestL2Outputs,
            _expectedStatus: GameStatus.DEFENDER_WINS
        });
    }

    /// @notice Static unit test for a 1v1 output bisection dispute.
    function test_static_1v1dishonestRootGenesisAbsolutePrestate_succeeds() public {
        // The honest l2 outputs are from [1, 16] in this game.
        uint256[] memory honestL2Outputs = new uint256[](16);
        for (uint256 i; i < honestL2Outputs.length; i++) {
            honestL2Outputs[i] = i + 1;
        }
        // The honest trace covers all block -> block + 1 transitions, and is 256 bytes long, consisting
        // of bytes [0, 255].
        bytes memory honestTrace = new bytes(256);
        for (uint256 i; i < honestTrace.length; i++) {
            honestTrace[i] = bytes1(uint8(i));
        }

        // The dishonest l2 outputs are from [2, 17] in this game.
        uint256[] memory dishonestL2Outputs = new uint256[](16);
        for (uint256 i; i < dishonestL2Outputs.length; i++) {
            dishonestL2Outputs[i] = i + 2;
        }
        // The dishonest trace covers all block -> block + 1 transitions, and is 256 bytes long, consisting
        // of all set bits.
        bytes memory dishonestTrace = new bytes(256);
        for (uint256 i; i < dishonestTrace.length; i++) {
            dishonestTrace[i] = bytes1(0xFF);
        }

        // Run the actor test
        _actorTest({
            _rootClaim: 17,
            _absolutePrestateData: 0,
            _honestTrace: honestTrace,
            _honestL2Outputs: honestL2Outputs,
            _dishonestTrace: dishonestTrace,
            _dishonestL2Outputs: dishonestL2Outputs,
            _expectedStatus: GameStatus.CHALLENGER_WINS
        });
    }

    /// @notice Static unit test for a 1v1 output bisection dispute.
    function test_static_1v1honestRoot_succeeds() public {
        // The honest l2 outputs are from [1, 16] in this game.
        uint256[] memory honestL2Outputs = new uint256[](16);
        for (uint256 i; i < honestL2Outputs.length; i++) {
            honestL2Outputs[i] = i + 1;
        }
        // The honest trace covers all block -> block + 1 transitions, and is 256 bytes long, consisting
        // of bytes [0, 255].
        bytes memory honestTrace = new bytes(256);
        for (uint256 i; i < honestTrace.length; i++) {
            honestTrace[i] = bytes1(uint8(i));
        }

        // The dishonest l2 outputs are from [2, 17] in this game.
        uint256[] memory dishonestL2Outputs = new uint256[](16);
        for (uint256 i; i < dishonestL2Outputs.length; i++) {
            dishonestL2Outputs[i] = i + 2;
        }
        // The dishonest trace covers all block -> block + 1 transitions, and is 256 bytes long, consisting
        // of all zeros.
        bytes memory dishonestTrace = new bytes(256);

        // Run the actor test
        _actorTest({
            _rootClaim: 16,
            _absolutePrestateData: 0,
            _honestTrace: honestTrace,
            _honestL2Outputs: honestL2Outputs,
            _dishonestTrace: dishonestTrace,
            _dishonestL2Outputs: dishonestL2Outputs,
            _expectedStatus: GameStatus.DEFENDER_WINS
        });
    }

    /// @notice Static unit test for a 1v1 output bisection dispute.
    function test_static_1v1dishonestRoot_succeeds() public {
        // The honest l2 outputs are from [1, 16] in this game.
        uint256[] memory honestL2Outputs = new uint256[](16);
        for (uint256 i; i < honestL2Outputs.length; i++) {
            honestL2Outputs[i] = i + 1;
        }
        // The honest trace covers all block -> block + 1 transitions, and is 256 bytes long, consisting
        // of bytes [0, 255].
        bytes memory honestTrace = new bytes(256);
        for (uint256 i; i < honestTrace.length; i++) {
            honestTrace[i] = bytes1(uint8(i));
        }

        // The dishonest l2 outputs are from [2, 17] in this game.
        uint256[] memory dishonestL2Outputs = new uint256[](16);
        for (uint256 i; i < dishonestL2Outputs.length; i++) {
            dishonestL2Outputs[i] = i + 2;
        }
        // The dishonest trace covers all block -> block + 1 transitions, and is 256 bytes long, consisting
        // of all zeros.
        bytes memory dishonestTrace = new bytes(256);

        // Run the actor test
        _actorTest({
            _rootClaim: 17,
            _absolutePrestateData: 0,
            _honestTrace: honestTrace,
            _honestL2Outputs: honestL2Outputs,
            _dishonestTrace: dishonestTrace,
            _dishonestL2Outputs: dishonestL2Outputs,
            _expectedStatus: GameStatus.CHALLENGER_WINS
        });
    }

    /// @notice Static unit test for a 1v1 output bisection dispute.
    function test_static_1v1correctRootHalfWay_succeeds() public {
        // The honest l2 outputs are from [1, 16] in this game.
        uint256[] memory honestL2Outputs = new uint256[](16);
        for (uint256 i; i < honestL2Outputs.length; i++) {
            honestL2Outputs[i] = i + 1;
        }
        // The honest trace covers all block -> block + 1 transitions, and is 256 bytes long, consisting
        // of bytes [0, 255].
        bytes memory honestTrace = new bytes(256);
        for (uint256 i; i < honestTrace.length; i++) {
            honestTrace[i] = bytes1(uint8(i));
        }

        // The dishonest l2 outputs are half correct, half incorrect.
        uint256[] memory dishonestL2Outputs = new uint256[](16);
        for (uint256 i; i < dishonestL2Outputs.length; i++) {
            dishonestL2Outputs[i] = i > 7 ? 0xFF : i + 1;
        }
        // The dishonest trace is half correct, half incorrect.
        bytes memory dishonestTrace = new bytes(256);
        for (uint256 i; i < dishonestTrace.length; i++) {
            dishonestTrace[i] = i > (127 + 4) ? bytes1(0xFF) : bytes1(uint8(i));
        }

        // Run the actor test
        _actorTest({
            _rootClaim: 16,
            _absolutePrestateData: 0,
            _honestTrace: honestTrace,
            _honestL2Outputs: honestL2Outputs,
            _dishonestTrace: dishonestTrace,
            _dishonestL2Outputs: dishonestL2Outputs,
            _expectedStatus: GameStatus.DEFENDER_WINS
        });
    }

    /// @notice Static unit test for a 1v1 output bisection dispute.
    function test_static_1v1dishonestRootHalfWay_succeeds() public {
        // The honest l2 outputs are from [1, 16] in this game.
        uint256[] memory honestL2Outputs = new uint256[](16);
        for (uint256 i; i < honestL2Outputs.length; i++) {
            honestL2Outputs[i] = i + 1;
        }
        // The honest trace covers all block -> block + 1 transitions, and is 256 bytes long, consisting
        // of bytes [0, 255].
        bytes memory honestTrace = new bytes(256);
        for (uint256 i; i < honestTrace.length; i++) {
            honestTrace[i] = bytes1(uint8(i));
        }

        // The dishonest l2 outputs are half correct, half incorrect.
        uint256[] memory dishonestL2Outputs = new uint256[](16);
        for (uint256 i; i < dishonestL2Outputs.length; i++) {
            dishonestL2Outputs[i] = i > 7 ? 0xFF : i + 1;
        }
        // The dishonest trace is half correct, half incorrect.
        bytes memory dishonestTrace = new bytes(256);
        for (uint256 i; i < dishonestTrace.length; i++) {
            dishonestTrace[i] = i > (127 + 4) ? bytes1(0xFF) : bytes1(uint8(i));
        }

        // Run the actor test
        _actorTest({
            _rootClaim: 0xFF,
            _absolutePrestateData: 0,
            _honestTrace: honestTrace,
            _honestL2Outputs: honestL2Outputs,
            _dishonestTrace: dishonestTrace,
            _dishonestL2Outputs: dishonestL2Outputs,
            _expectedStatus: GameStatus.CHALLENGER_WINS
        });
    }

    /// @notice Static unit test for a 1v1 output bisection dispute.
    function test_static_1v1correctAbsolutePrestate_succeeds() public {
        // The honest l2 outputs are from [1, 16] in this game.
        uint256[] memory honestL2Outputs = new uint256[](16);
        for (uint256 i; i < honestL2Outputs.length; i++) {
            honestL2Outputs[i] = i + 1;
        }
        // The honest trace covers all block -> block + 1 transitions, and is 256 bytes long, consisting
        // of bytes [0, 255].
        bytes memory honestTrace = new bytes(256);
        for (uint256 i; i < honestTrace.length; i++) {
            honestTrace[i] = bytes1(uint8(i));
        }

        // The dishonest l2 outputs are half correct, half incorrect.
        uint256[] memory dishonestL2Outputs = new uint256[](16);
        for (uint256 i; i < dishonestL2Outputs.length; i++) {
            dishonestL2Outputs[i] = i > 7 ? 0xFF : i + 1;
        }
        // The dishonest trace correct is half correct, half incorrect.
        bytes memory dishonestTrace = new bytes(256);
        for (uint256 i; i < dishonestTrace.length; i++) {
            dishonestTrace[i] = i > 127 ? bytes1(0xFF) : bytes1(uint8(i));
        }

        // Run the actor test
        _actorTest({
            _rootClaim: 16,
            _absolutePrestateData: 0,
            _honestTrace: honestTrace,
            _honestL2Outputs: honestL2Outputs,
            _dishonestTrace: dishonestTrace,
            _dishonestL2Outputs: dishonestL2Outputs,
            _expectedStatus: GameStatus.DEFENDER_WINS
        });
    }

    /// @notice Static unit test for a 1v1 output bisection dispute.
    function test_static_1v1dishonestAbsolutePrestate_succeeds() public {
        // The honest l2 outputs are from [1, 16] in this game.
        uint256[] memory honestL2Outputs = new uint256[](16);
        for (uint256 i; i < honestL2Outputs.length; i++) {
            honestL2Outputs[i] = i + 1;
        }
        // The honest trace covers all block -> block + 1 transitions, and is 256 bytes long, consisting
        // of bytes [0, 255].
        bytes memory honestTrace = new bytes(256);
        for (uint256 i; i < honestTrace.length; i++) {
            honestTrace[i] = bytes1(uint8(i));
        }

        // The dishonest l2 outputs are half correct, half incorrect.
        uint256[] memory dishonestL2Outputs = new uint256[](16);
        for (uint256 i; i < dishonestL2Outputs.length; i++) {
            dishonestL2Outputs[i] = i > 7 ? 0xFF : i + 1;
        }
        // The dishonest trace correct is half correct, half incorrect.
        bytes memory dishonestTrace = new bytes(256);
        for (uint256 i; i < dishonestTrace.length; i++) {
            dishonestTrace[i] = i > 127 ? bytes1(0xFF) : bytes1(uint8(i));
        }

        // Run the actor test
        _actorTest({
            _rootClaim: 0xFF,
            _absolutePrestateData: 0,
            _honestTrace: honestTrace,
            _honestL2Outputs: honestL2Outputs,
            _dishonestTrace: dishonestTrace,
            _dishonestL2Outputs: dishonestL2Outputs,
            _expectedStatus: GameStatus.CHALLENGER_WINS
        });
    }

    /// @notice Static unit test for a 1v1 output bisection dispute.
    function test_static_1v1honestRootFinalInstruction_succeeds() public {
        // The honest l2 outputs are from [1, 16] in this game.
        uint256[] memory honestL2Outputs = new uint256[](16);
        for (uint256 i; i < honestL2Outputs.length; i++) {
            honestL2Outputs[i] = i + 1;
        }
        // The honest trace covers all block -> block + 1 transitions, and is 256 bytes long, consisting
        // of bytes [0, 255].
        bytes memory honestTrace = new bytes(256);
        for (uint256 i; i < honestTrace.length; i++) {
            honestTrace[i] = bytes1(uint8(i));
        }

        // The dishonest l2 outputs are half correct, half incorrect.
        uint256[] memory dishonestL2Outputs = new uint256[](16);
        for (uint256 i; i < dishonestL2Outputs.length; i++) {
            dishonestL2Outputs[i] = i > 7 ? 0xFF : i + 1;
        }
        // The dishonest trace is half correct, and correct all the way up to the final instruction of the exec
        // subgame.
        bytes memory dishonestTrace = new bytes(256);
        for (uint256 i; i < dishonestTrace.length; i++) {
            dishonestTrace[i] = i > (127 + 7) ? bytes1(0xFF) : bytes1(uint8(i));
        }

        // Run the actor test
        _actorTest({
            _rootClaim: 16,
            _absolutePrestateData: 0,
            _honestTrace: honestTrace,
            _honestL2Outputs: honestL2Outputs,
            _dishonestTrace: dishonestTrace,
            _dishonestL2Outputs: dishonestL2Outputs,
            _expectedStatus: GameStatus.DEFENDER_WINS
        });
    }

    /// @notice Static unit test for a 1v1 output bisection dispute.
    function test_static_1v1dishonestRootFinalInstruction_succeeds() public {
        // The honest l2 outputs are from [1, 16] in this game.
        uint256[] memory honestL2Outputs = new uint256[](16);
        for (uint256 i; i < honestL2Outputs.length; i++) {
            honestL2Outputs[i] = i + 1;
        }
        // The honest trace covers all block -> block + 1 transitions, and is 256 bytes long, consisting
        // of bytes [0, 255].
        bytes memory honestTrace = new bytes(256);
        for (uint256 i; i < honestTrace.length; i++) {
            honestTrace[i] = bytes1(uint8(i));
        }

        // The dishonest l2 outputs are half correct, half incorrect.
        uint256[] memory dishonestL2Outputs = new uint256[](16);
        for (uint256 i; i < dishonestL2Outputs.length; i++) {
            dishonestL2Outputs[i] = i > 7 ? 0xFF : i + 1;
        }
        // The dishonest trace is half correct, and correct all the way up to the final instruction of the exec
        // subgame.
        bytes memory dishonestTrace = new bytes(256);
        for (uint256 i; i < dishonestTrace.length; i++) {
            dishonestTrace[i] = i > (127 + 7) ? bytes1(0xFF) : bytes1(uint8(i));
        }

        // Run the actor test
        _actorTest({
            _rootClaim: 0xFF,
            _absolutePrestateData: 0,
            _honestTrace: honestTrace,
            _honestL2Outputs: honestL2Outputs,
            _dishonestTrace: dishonestTrace,
            _dishonestL2Outputs: dishonestL2Outputs,
            _expectedStatus: GameStatus.CHALLENGER_WINS
        });
    }

    ////////////////////////////////////////////////////////////////
    //                          HELPERS                           //
    ////////////////////////////////////////////////////////////////

    /// @dev Helper to run a 1v1 actor test
    function _actorTest(
        uint256 _rootClaim,
        uint256 _absolutePrestateData,
        bytes memory _honestTrace,
        uint256[] memory _honestL2Outputs,
        bytes memory _dishonestTrace,
        uint256[] memory _dishonestL2Outputs,
        GameStatus _expectedStatus
    )
        internal
    {
        // Setup the environment
        bytes memory absolutePrestateData =
            _setup({ _absolutePrestateData: _absolutePrestateData, _rootClaim: _rootClaim });

        // Create actors
        _createActors({
            _honestTrace: _honestTrace,
            _honestPreStateData: absolutePrestateData,
            _honestL2Outputs: _honestL2Outputs,
            _dishonestTrace: _dishonestTrace,
            _dishonestPreStateData: absolutePrestateData,
            _dishonestL2Outputs: _dishonestL2Outputs
        });

        // Exhaust all moves from both actors
        _exhaustMoves();

        // Resolve the game and assert that the defender won
        _warpAndResolve();
        assertEq(uint8(gameProxy.status()), uint8(_expectedStatus));
    }

    /// @dev Helper to setup the 1v1 test
    function _setup(
        uint256 _absolutePrestateData,
        uint256 _rootClaim
    )
        internal
        returns (bytes memory absolutePrestateData_)
    {
        absolutePrestateData_ = abi.encode(_absolutePrestateData);
        Claim absolutePrestateExec =
            _changeClaimStatus(Claim.wrap(keccak256(absolutePrestateData_)), VMStatuses.UNFINISHED);
        Claim rootClaim = Claim.wrap(bytes32(uint256(_rootClaim)));
        super.init({ rootClaim: rootClaim, absolutePrestate: absolutePrestateExec, l2BlockNumber: _rootClaim });
    }

    /// @dev Helper to create actors for the 1v1 dispute.
    function _createActors(
        bytes memory _honestTrace,
        bytes memory _honestPreStateData,
        uint256[] memory _honestL2Outputs,
        bytes memory _dishonestTrace,
        bytes memory _dishonestPreStateData,
        uint256[] memory _dishonestL2Outputs
    )
        internal
    {
        honest = new HonestDisputeActor({
            _gameProxy: gameProxy,
            _l2Outputs: _honestL2Outputs,
            _trace: _honestTrace,
            _preStateData: _honestPreStateData
        });
        dishonest = new HonestDisputeActor({
            _gameProxy: gameProxy,
            _l2Outputs: _dishonestL2Outputs,
            _trace: _dishonestTrace,
            _preStateData: _dishonestPreStateData
        });

        vm.deal(address(honest), 100 ether);
        vm.deal(address(dishonest), 100 ether);
        vm.label(address(honest), "HonestActor");
        vm.label(address(dishonest), "DishonestActor");
    }

    /// @dev Helper to exhaust all moves from both actors.
    function _exhaustMoves() internal {
        while (true) {
            // Allow the dishonest actor to make their moves, and then the honest actor.
            (uint256 numMovesA,) = dishonest.move();
            (uint256 numMovesB, bool success) = honest.move();

            require(success, "FaultDispute_1v1_Actors_Test: Honest actor's moves should always be successful");

            // If both actors have run out of moves, we're done.
            if (numMovesA == 0 && numMovesB == 0) break;
        }
    }

    /// @dev Helper to warp past the chess clock and resolve all claims within the dispute game.
    function _warpAndResolve() internal {
        // Warp past the chess clock
        vm.warp(block.timestamp + 3 days + 12 hours);

        // Resolve all claims in reverse order. We allow `resolveClaim` calls to fail due to
        // the check that prevents claims with no subgames attached from being passed to
        // `resolveClaim`. There's also a check in `resolve` to ensure all children have been
        // resolved before global resolution, which catches any unresolved subgames here.
        for (uint256 i = gameProxy.claimDataLen(); i > 0; i--) {
            (bool success,) = address(gameProxy).call(abi.encodeCall(gameProxy.resolveClaim, (i - 1, 0)));
            assertTrue(success);
        }
        gameProxy.resolve();
    }
}

contract ClaimCreditReenter {
    Vm internal immutable vm;
    IFaultDisputeGame internal immutable GAME;
    uint256 public numCalls;

    constructor(IFaultDisputeGame _gameProxy, Vm _vm) {
        GAME = _gameProxy;
        vm = _vm;
    }

    function claimCredit(address _recipient) public {
        numCalls += 1;
        if (numCalls > 1) {
            vm.expectRevert(NoCreditToClaim.selector);
        }
        GAME.claimCredit(_recipient);
    }

    receive() external payable {
        if (numCalls == 5) {
            return;
        }
        claimCredit(address(this));
    }
}

/// @dev Helper to change the VM status byte of a claim.
function _changeClaimStatus(Claim _claim, VMStatus _status) pure returns (Claim out_) {
    assembly {
        out_ := or(and(not(shl(248, 0xFF)), _claim), shl(248, _status))
    }
}
