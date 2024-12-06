// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

// Testing
import { CommonBase } from "forge-std/Base.sol";

// Libraries
import "src/dispute/lib/Types.sol";

// Interfaces
import { IFaultDisputeGame } from "interfaces/dispute/IFaultDisputeGame.sol";

/// @title GameSolver
/// @notice The `GameSolver` contract is a contract that can produce an array of available
///         moves for a given `FaultDisputeGame` contract, from the eyes of an honest
///         actor. The `GameSolver` does not implement functionality for acting on the `Move`s
///         it suggests.
abstract contract GameSolver is CommonBase {
    /// @notice The `FaultDisputeGame` proxy that the `GameSolver` will be solving.
    IFaultDisputeGame public immutable GAME;
    /// @notice The split depth of the game
    uint256 internal immutable SPLIT_DEPTH;
    /// @notice The max depth of the game
    uint256 internal immutable MAX_DEPTH;
    /// @notice The maximum L2 block number that the output bisection portion of the position tree
    ///         can handle.
    uint256 internal immutable MAX_L2_BLOCK_NUMBER;

    /// @notice The L2 outputs that the `GameSolver` will be representing, keyed by L2 block number - 1.
    uint256[] public l2Outputs;
    /// @notice The execution trace that the `GameSolver` will be representing.
    bytes public trace;
    /// @notice The raw absolute prestate data.
    bytes public absolutePrestateData;
    /// @notice The offset of previously processed claims in the `GAME` contract's `claimData` array.
    ///         Starts at 0 and increments by 1 for each claim processed.
    uint256 public processedBuf;
    /// @notice Signals whether or not the `GameSolver` agrees with the root claim of the
    ///         `GAME` contract.
    bool public agreeWithRoot;

    /// @notice The `MoveKind` enum represents a kind of interaction with the `FaultDisputeGame` contract.
    enum MoveKind {
        Attack,
        Defend,
        Step,
        AddLocalData
    }

    /// @notice The `Move` struct represents a move in the game, and contains information
    ///         about the kind of move, the sender of the move, and the calldata to be sent
    ///         to the `FaultDisputeGame` contract by a consumer of this contract.
    struct Move {
        MoveKind kind;
        bytes data;
        uint256 value;
    }

    constructor(
        IFaultDisputeGame _gameProxy,
        uint256[] memory _l2Outputs,
        bytes memory _trace,
        bytes memory _preStateData
    ) {
        GAME = _gameProxy;
        SPLIT_DEPTH = GAME.splitDepth();
        MAX_DEPTH = GAME.maxGameDepth();
        MAX_L2_BLOCK_NUMBER = 2 ** (MAX_DEPTH - SPLIT_DEPTH);

        l2Outputs = _l2Outputs;
        trace = _trace;
        absolutePrestateData = _preStateData;
    }

    /// @notice Returns an array of `Move`s that can be taken from the perspective of an honest
    ///         actor in the `FaultDisputeGame` contract.
    function solveGame() external virtual returns (Move[] memory moves_);
}

/// @title HonestGameSolver
/// @notice The `HonestGameSolver` is an implementation of `GameSolver` which responds accordingly depending
///         on the state of the `FaultDisputeGame` contract in relation to their local opinion of the correct
///         order of output roots and the execution trace between each block `n` -> `n + 1` state transition.
contract HonestGameSolver is GameSolver {
    /// @notice The `Direction` enum represents the direction of a proposed move in the game,
    ///         or a lack thereof.
    enum Direction {
        Defend,
        Attack,
        Noop
    }

    constructor(
        IFaultDisputeGame _gameProxy,
        uint256[] memory _l2Outputs,
        bytes memory _trace,
        bytes memory _preStateData
    )
        GameSolver(_gameProxy, _l2Outputs, _trace, _preStateData)
    {
        // Mark agreement with the root claim if the local opinion of the root claim is the same as the
        // observed root claim.
        agreeWithRoot = Claim.unwrap(outputAt(MAX_L2_BLOCK_NUMBER)) == Claim.unwrap(_gameProxy.rootClaim());
    }

    ////////////////////////////////////////////////////////////////
    //                          EXTERNAL                          //
    ////////////////////////////////////////////////////////////////

    /// @notice Returns an array of `Move`s that can be taken from the perspective of an honest
    ///         actor in the `FaultDisputeGame` contract.
    function solveGame() external override returns (Move[] memory moves_) {
        uint256 numClaims = GAME.claimDataLen();

        // Pre-allocate the `moves_` array to the maximum possible length. Test environment, so
        // over-allocation is fine, and more easy to read than making a linked list in asm.
        moves_ = new Move[](numClaims - processedBuf);

        uint256 numMoves = 0;
        for (uint256 i = processedBuf; i < numClaims; i++) {
            // Grab the observed claim.
            IFaultDisputeGame.ClaimData memory observed = getClaimData(i);

            // Determine the direction of the next move to be taken.
            (Direction moveDirection, Position movePos) = determineDirection(observed);

            // Continue if there is no move to be taken against the observed claim.
            if (moveDirection == Direction.Noop) continue;

            if (movePos.depth() <= MAX_DEPTH) {
                // bisection
                moves_[numMoves++] = handleBisectionMove(moveDirection, movePos, i);
            } else {
                // instruction step
                moves_[numMoves++] = handleStepMove(moveDirection, observed.position, movePos, i);
            }
        }

        // Update the length of the `moves_` array to the number of moves that were added. This is
        // always a no-op or a truncation operation.
        assembly {
            mstore(moves_, numMoves)
        }

        // Increment `processedBuf` by the number of claims processed, so that next time around,
        // we don't attempt to process the same claims again.
        processedBuf += numClaims - processedBuf;
    }

    ////////////////////////////////////////////////////////////////
    //                          INTERNAL                          //
    ////////////////////////////////////////////////////////////////

    /// @dev Helper function to determine the direction of the next move to be taken.
    function determineDirection(IFaultDisputeGame.ClaimData memory _claimData)
        internal
        view
        returns (Direction direction_, Position movePos_)
    {
        bool rightLevel = isRightLevel(_claimData.position);
        bool localAgree = Claim.unwrap(claimAt(_claimData.position)) == Claim.unwrap(_claimData.claim);
        if (_claimData.parentIndex == type(uint32).max) {
            // If we agree with the parent claim and it is on a level we agree with, ignore it.
            if (localAgree && rightLevel) {
                return (Direction.Noop, Position.wrap(0));
            }

            // The parent claim is the root claim. We must attack if we disagree per the game rules.
            direction_ = Direction.Attack;
            movePos_ = _claimData.position.move(true);
        } else {
            // Never attempt to defend an execution trace subgame root. Only attack if we disagree with it,
            // otherwise do nothing.
            // NOTE: This is not correct behavior in the context of the honest actor; The alphabet game has
            //       a constant status byte, and is not safe from someone being dishonest in output bisection
            //       and then posting a correct execution trace bisection root claim.
            if (_claimData.position.depth() == SPLIT_DEPTH + 1 && localAgree) {
                return (Direction.Noop, Position.wrap(0));
            }

            // If the parent claim is not the root claim, first check if the observed claim is on a level that
            // agrees with the local view of the root claim. If it is, noop. If it is not, perform an attack or
            // defense depending on the local view of the observed claim.
            if (rightLevel) {
                // Never move against a claim on the right level. Even if it's wrong, if it's uncountered, it furthers
                // our goals.
                return (Direction.Noop, Position.wrap(0));
            } else {
                // Fetch the local opinion of the parent claim.
                Claim localParent = claimAt(_claimData.position);

                // NOTE: Poison not handled.
                if (Claim.unwrap(localParent) != Claim.unwrap(_claimData.claim)) {
                    // If we disagree with the observed claim, we must attack it.
                    movePos_ = _claimData.position.move(true);
                    direction_ = Direction.Attack;
                } else {
                    // If we agree with the observed claim, we must defend the observed claim.
                    movePos_ = _claimData.position.move(false);
                    direction_ = Direction.Defend;
                }
            }
        }
    }

    /// @notice Returns a `Move` struct that represents an attack or defense move in the bisection portion
    ///         of the game.
    ///
    /// @dev Note: This function assumes that the `movePos` and `challengeIndex` are valid within the
    ///            output bisection context. This is enforced by the `solveGame` function.
    function handleBisectionMove(
        Direction _direction,
        Position _movePos,
        uint256 _challengeIndex
    )
        internal
        view
        returns (Move memory move_)
    {
        bool isAttack = _direction == Direction.Attack;

        uint256 bond = GAME.getRequiredBond(_movePos);
        (,,,, Claim disputed,,) = GAME.claimData(_challengeIndex);

        move_ = Move({
            kind: isAttack ? MoveKind.Attack : MoveKind.Defend,
            value: bond,
            data: abi.encodeCall(IFaultDisputeGame.move, (disputed, _challengeIndex, claimAt(_movePos), isAttack))
        });
    }

    /// @notice Returns a `Move` struct that represents a step move in the execution trace
    ///         bisection portion of the dispute game.
    /// @dev Note: This function assumes that the `movePos` and `challengeIndex` are valid within the
    ///            execution trace bisection context. This is enforced by the `solveGame` function.
    function handleStepMove(
        Direction _direction,
        Position _parentPos,
        Position _movePos,
        uint256 _challengeIndex
    )
        internal
        view
        returns (Move memory move_)
    {
        bool isAttack = _direction == Direction.Attack;
        bytes memory preStateTrace;

        // First, we need to find the pre/post state index depending on whether we
        // are making an attack step or a defense step. If the relative index at depth of the
        // move position is 0, the prestate is the absolute prestate and we need to
        // do nothing.
        if ((_movePos.indexAtDepth() % (2 ** (MAX_DEPTH - SPLIT_DEPTH))) != 0) {
            // Grab the trace up to the prestate's trace index.
            if (isAttack) {
                Position leafPos = Position.wrap(Position.unwrap(_parentPos) - 1);
                preStateTrace = abi.encode(leafPos.traceIndex(MAX_DEPTH), stateAt(leafPos));
            } else {
                preStateTrace = abi.encode(_parentPos.traceIndex(MAX_DEPTH), stateAt(_parentPos));
            }
        } else {
            preStateTrace = absolutePrestateData;
        }

        move_ = Move({
            kind: MoveKind.Step,
            value: 0,
            data: abi.encodeCall(IFaultDisputeGame.step, (_challengeIndex, isAttack, preStateTrace, hex""))
        });
    }

    ////////////////////////////////////////////////////////////////
    //                          HELPERS                           //
    ////////////////////////////////////////////////////////////////

    /// @dev Helper function to get the `ClaimData` struct at a given index in the `GAME` contract's
    ///      `claimData` array.
    function getClaimData(uint256 _claimIndex) internal view returns (IFaultDisputeGame.ClaimData memory claimData_) {
        // thanks, solc
        (
            uint32 parentIndex,
            address countered,
            address claimant,
            uint128 bond,
            Claim claim,
            Position position,
            Clock clock
        ) = GAME.claimData(_claimIndex);
        claimData_ = IFaultDisputeGame.ClaimData({
            parentIndex: parentIndex,
            counteredBy: countered,
            claimant: claimant,
            bond: bond,
            claim: claim,
            position: position,
            clock: clock
        });
    }

    /// @notice Returns the player's claim that commits to a given position, swapping between
    ///         output bisection claims and execution trace bisection claims depending on the depth.
    /// @dev Prefer this function over `outputAt` or `statehashAt` directly.
    function claimAt(Position _position) internal view returns (Claim claim_) {
        return _position.depth() > SPLIT_DEPTH ? statehashAt(_position) : outputAt(_position);
    }

    /// @notice Returns the mock output at the given position.
    function outputAt(Position _position) internal view returns (Claim claim_) {
        // Don't allow for positions that are deeper than the split depth.
        if (_position.depth() > SPLIT_DEPTH) {
            revert("GameSolver: invalid position depth");
        }

        return outputAt(_position.traceIndex(SPLIT_DEPTH) + 1);
    }

    /// @notice Returns the mock output at the given L2 block number.
    function outputAt(uint256 _l2BlockNumber) internal view returns (Claim claim_) {
        return Claim.wrap(bytes32(l2Outputs[_l2BlockNumber - 1]));
    }

    /// @notice Returns the player's claim that commits to a given trace index.
    function statehashAt(uint256 _traceIndex) internal view returns (Claim claim_) {
        bytes32 hash =
            keccak256(abi.encode(_traceIndex >= trace.length ? trace.length - 1 : _traceIndex, stateAt(_traceIndex)));
        assembly {
            claim_ := or(and(hash, not(shl(248, 0xFF))), shl(248, 1))
        }
    }

    /// @notice Returns the player's claim that commits to a given trace index.
    function statehashAt(Position _position) internal view returns (Claim claim_) {
        return statehashAt(_position.traceIndex(MAX_DEPTH));
    }

    /// @notice Returns the state at the trace index within the player's trace.
    function stateAt(Position _position) internal view returns (uint256 state_) {
        return stateAt(_position.traceIndex(MAX_DEPTH));
    }

    /// @notice Returns the state at the trace index within the player's trace.
    function stateAt(uint256 _traceIndex) internal view returns (uint256 state_) {
        return uint256(uint8(_traceIndex >= trace.length ? trace[trace.length - 1] : trace[_traceIndex]));
    }

    /// @notice Returns whether or not the position is on a level which opposes the local opinion of the
    ///         root claim.
    function isRightLevel(Position _position) internal view returns (bool isRightLevel_) {
        isRightLevel_ = agreeWithRoot == (_position.depth() % 2 == 0);
    }
}

/// @title DisputeActor
/// @notice The `DisputeActor` contract is an abstract contract that represents an actor
///         that consumes the suggested moves from a `GameSolver` contract.
abstract contract DisputeActor {
    /// @notice The `GameSolver` contract used to determine the moves to be taken.
    GameSolver public solver;

    /// @notice Performs all available moves deemed by the attached solver.
    /// @return numMoves_ The number of moves that the actor took.
    /// @return success_ True if all moves were successful, false otherwise.
    function move() external virtual returns (uint256 numMoves_, bool success_);
}

/// @title HonestDisputeActor
/// @notice An actor that consumes the suggested moves from an `HonestGameSolver` contract. Note
///         that this actor *can* be dishonest if the trace is faulty, but it will always follow
///         the rules of the honest actor.
contract HonestDisputeActor is DisputeActor {
    IFaultDisputeGame public immutable GAME;

    constructor(
        IFaultDisputeGame _gameProxy,
        uint256[] memory _l2Outputs,
        bytes memory _trace,
        bytes memory _preStateData
    ) {
        GAME = _gameProxy;
        solver = GameSolver(new HonestGameSolver(_gameProxy, _l2Outputs, _trace, _preStateData));
    }

    /// @inheritdoc DisputeActor
    function move() external override returns (uint256 numMoves_, bool success_) {
        GameSolver.Move[] memory moves = solver.solveGame();
        numMoves_ = moves.length;

        // Optimistically assume success, will be set to false if any move fails.
        success_ = true;

        // Perform all available moves given to the actor by the solver.
        for (uint256 i = 0; i < moves.length; i++) {
            GameSolver.Move memory localMove = moves[i];

            // If the move is a step, we first need to add the starting L2 block number to the `PreimageOracle`
            // via the `FaultDisputeGame` contract.
            // TODO: This is leaky. Could be another move kind.
            if (localMove.kind == GameSolver.MoveKind.Step) {
                bytes memory moveData = localMove.data;
                uint256 challengeIndex;
                assembly {
                    challengeIndex := mload(add(moveData, 0x24))
                }
                GAME.addLocalData({
                    _ident: LocalPreimageKey.DISPUTED_L2_BLOCK_NUMBER,
                    _execLeafIdx: challengeIndex,
                    _partOffset: 0
                });
            }

            (bool innerSuccess,) = address(GAME).call{ value: localMove.value }(localMove.data);
            assembly {
                success_ := and(success_, innerSuccess)
            }
        }
    }

    fallback() external payable { }

    receive() external payable { }
}
